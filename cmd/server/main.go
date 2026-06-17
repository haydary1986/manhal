// Command server runs the Manhal Telegram bot.
package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erticaz/manhal/internal/ai"
	"github.com/erticaz/manhal/internal/alerts"
	"github.com/erticaz/manhal/internal/announce"
	"github.com/erticaz/manhal/internal/bot"
	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/embed"
	"github.com/erticaz/manhal/internal/journal"
	"github.com/erticaz/manhal/internal/logbuf"
	"github.com/erticaz/manhal/internal/menu"
	"github.com/erticaz/manhal/internal/predator"
	"github.com/erticaz/manhal/internal/promotion"
	"github.com/erticaz/manhal/internal/scholar"
	"github.com/erticaz/manhal/internal/store"
	"github.com/erticaz/manhal/internal/web"
)

func main() {
	// Mirror logs to an in-memory buffer so the admin "logs" page can show them.
	log.SetOutput(io.MultiWriter(os.Stderr, logbuf.Default))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	settings, err := config.LoadBotSettings(cfg.DataDir)
	if err != nil {
		log.Fatalf("settings: %v", err)
	}
	// Shared, persisted settings: the bot reads the gate live; the web edits it.
	settingsMgr := config.NewSettingsManager(cfg.DataDir, settings)
	// Seed the API key from the environment on first run; the admin can override
	// it later from the dashboard.
	settingsMgr.SeedDeepSeekKey(cfg.DeepSeekKey)

	disciplines, err := config.LoadDisciplines(cfg.DataDir)
	if err != nil {
		log.Fatalf("disciplines: %v", err)
	}
	disciplinesMgr := config.NewDisciplinesManager(cfg.DataDir, disciplines)

	announcements, err := announce.Load(cfg.DataDir)
	if err != nil {
		log.Fatalf("announcements: %v", err)
	}

	journals, err := journal.LoadCSV(cfg.DataDir)
	if err != nil {
		log.Fatalf("journals: %v", err)
	}

	predators, err := predator.Load(cfg.DataDir)
	if err != nil {
		log.Fatalf("predatory list: %v", err)
	}

	menuMgr, err := menu.Load(cfg.DataDir)
	if err != nil {
		log.Fatalf("menu: %v", err)
	}

	promotionRules, err := promotion.Load(cfg.DataDir)
	if err != nil {
		log.Fatalf("promotion: %v", err)
	}

	// Storage: Postgres when DATABASE_URL is set, else in-memory for development.
	var st store.Store
	if cfg.DatabaseURL != "" {
		connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		pg, perr := store.NewPostgres(connectCtx, cfg.DatabaseURL)
		cancel()
		if perr != nil {
			log.Fatalf("postgres: %v", perr)
		}
		defer pg.Close()
		st = pg
		log.Println("using Postgres store")
	} else {
		st = store.NewMemory()
		log.Println("using in-memory store (set DATABASE_URL for Postgres persistence)")
	}

	// One OpenAlex client serves both works search and author profiles.
	openAlex := scholar.NewOpenAlex(cfg.CrossrefMailto)

	// Local embeddings power semantic search (#22); nil when EMBED_MODEL is unset.
	var embedder embed.Provider
	if cfg.EmbedModel != "" {
		embedder = embed.NewOllama(cfg.OllamaURL, cfg.EmbedModel)
		log.Printf("embeddings enabled (%s via %s)", cfg.EmbedModel, cfg.OllamaURL)
	} else {
		log.Println("embeddings disabled (set EMBED_MODEL to enable semantic search)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// AI provider reads its key live from settings so the admin can change it.
	aiProvider := ai.NewDeepSeek(settingsMgr.DeepSeekKey())
	aiProvider.SetKeyFunc(settingsMgr.DeepSeekKey)

	app, err := bot.New(bot.Deps{
		Config:      cfg,
		Settings:    settingsMgr,
		Store:       st,
		AI:          aiProvider,
		Citations:   scholar.NewCrossref(cfg.CrossrefMailto),
		Search:      openAlex,
		Authors:     openAlex,
		AuthorWorks: openAlex,
		Trends:      openAlex,
		OA:          scholar.NewUnpaywall(cfg.UnpaywallEmail),
		Journals:    journals,
		Predators:   predators,
		Promotion:   promotionRules,
		Announce:    announcements,
		Disciplines: disciplinesMgr,
		Menu:        menuMgr,
		Embed:       embedder,
	})

	// The bot acts as the push notifier for admin support replies; nil when the
	// bot couldn't start (replies are saved and delivered once it is up).
	var notifier web.Notifier
	if err == nil {
		notifier = app
		app.ApplyIdentity(ctx) // push admin-set bot name/description to Telegram
	}

	// Admin web server. Disabled unless at least one account exists, so the panel
	// is never exposed without a password. It runs independently of the bot.
	accounts := cfg.WebAccounts()
	if len(accounts) > 0 {
		webSrv := web.NewServer(menuMgr, st, notifier, accounts, settingsMgr, announcements).
			WithEditors(disciplinesMgr)
		go func() {
			log.Printf("admin web listening on %s (%d admin account(s))", cfg.WebAddr, len(accounts))
			if rerr := webSrv.Run(ctx, cfg.WebAddr); rerr != nil {
				log.Printf("admin web error: %v", rerr)
			}
		}()
	} else {
		log.Println("admin web disabled (set ADMIN_WEB_TOKEN or ADMIN_WEB_USERS to enable)")
	}

	if err != nil {
		if len(accounts) > 0 {
			log.Printf("bot failed to start (%v); running admin web only", err)
			<-ctx.Done()
			return
		}
		log.Fatalf("bot: %v", err)
	}

	// Proactive alerts: poll followed topics and push new papers via the bot.
	interval := time.Duration(cfg.AlertIntervalM) * time.Minute
	go alerts.NewScheduler(st, openAlex, app, interval).Run(ctx)
	go alerts.NewDeadlineReminder(announcements, st, st, app, cfg.ReminderDays, interval).Run(ctx)
	go alerts.NewCitationWatcher(st, openAlex, app, interval).Run(ctx)
	weekday := time.Weekday(((cfg.DigestWeekday % 7) + 7) % 7)
	go alerts.NewWeeklyDigest(st, openAlex, announcements, app, weekday, interval).Run(ctx)

	app.Run(ctx)
	log.Println("منهل bot stopped")
}
