package bot

import (
	"context"
	"strings"

	tg "github.com/go-telegram/bot"
)

// ApplyIdentity pushes the admin-configured bot name and description to Telegram.
// Called once at startup; empty values are left unchanged. Best-effort — a
// failure (e.g. Telegram rate limit) is logged but never blocks the bot.
func (a *App) ApplyIdentity(ctx context.Context) {
	bs := a.settings.Get()
	if name := strings.TrimSpace(bs.BotName); name != "" {
		if _, err := a.bot.SetMyName(ctx, &tg.SetMyNameParams{Name: name}); err != nil {
			a.logf("set bot name: %v", err)
		}
	}
	if desc := strings.TrimSpace(bs.BotDescription); desc != "" {
		if _, err := a.bot.SetMyDescription(ctx, &tg.SetMyDescriptionParams{Description: desc}); err != nil {
			a.logf("set bot description: %v", err)
		}
	}
}
