package bot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/domain"
	"github.com/erticaz/manhal/internal/plans"
	"github.com/erticaz/manhal/internal/store"
	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// premiumBenefits is the single source of truth for what a paid subscription
// unlocks. It is shown on the subscribe screen and on the premium gate so the
// value is always clear and consistent.
func premiumBenefits() []string {
	return []string{
		"حدود استخدام يومية أعلى للأدوات الذكية (أو بلا حدود)",
		"إعادة الكتابة البشرية لملفات Word كاملة (لا النص فقط)",
		"أولوية في الدعم الفني",
	}
}

// plansList renders the plan catalogue as bullet lines with price + duration.
func plansList(pl []plans.Plan) string {
	var b strings.Builder
	for _, p := range pl {
		b.WriteString("• " + p.Name + " — " + strconv.Itoa(p.Price) + " د.ع / " + p.DurationLabel() + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// benefitsList renders the benefits as bullet lines.
func benefitsList() string {
	var b strings.Builder
	for _, x := range premiumBenefits() {
		b.WriteString("• " + x + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// premiumGateScreen is shown when a free user reaches a premium-only feature.
func premiumGateScreen(reason string) Screen {
	text := "💎 هذه ميزة مخصّصة للمشتركين."
	if strings.TrimSpace(reason) != "" {
		text += "\n" + reason
	}
	text += "\n\n✨ مزايا الاشتراك:\n" + benefitsList()
	return Screen{
		Text: text,
		Keyboard: &Keyboard{Rows: [][]Button{
			{{Text: "💎 الاشتراك / الترقية", Data: "menu:subscribe"}},
			{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
		}},
	}
}

// subscribeScreen shows the premium benefits and plans plus the manual payment
// details the admin configured, with a button to register a paid request.
func subscribeScreen(bs config.BotSettings, pl []plans.Plan) Screen {
	text := "💎 الاشتراك المميّز في منهل\n\n✨ ماذا تكسب:\n" + benefitsList()
	if len(pl) > 0 {
		text += "\n\n📦 الباقات المتاحة:\n" + plansList(pl)
	}
	if info := strings.TrimSpace(bs.PremiumInfo); info != "" {
		text += "\n\n" + info
	}
	if pay := strings.TrimSpace(bs.PaymentDetails); pay != "" {
		text += "\n\n💳 طريقة الدفع:\n" + pay
	}
	if len(pl) > 0 {
		text += "\n\n👇 اختر الباقة التي دفعت قيمتها لتفعيلها:"
	} else {
		text += "\n\nبعد إتمام التحويل اضغط الزر أدناه ليصل طلبك، وسنفعّل اشتراكك بعد تأكيد الدفع."
	}

	var rows [][]Button
	if link := strings.TrimSpace(bs.PaymentLink); link != "" {
		rows = append(rows, []Button{{Text: "💳 ادفع عبر التطبيق", URL: link}})
	}
	// One button per plan; tapping it starts the pay-proof flow for that plan.
	for _, p := range pl {
		rows = append(rows, []Button{{
			Text: "📦 " + p.Name + " — " + strconv.Itoa(p.Price) + " د.ع",
			Data: "premium:plan:" + p.ID,
		}})
	}
	if len(pl) == 0 {
		rows = append(rows, []Button{{Text: "✅ طلبت الاشتراك / دفعت", Data: "premium:request"}})
	}
	rows = append(rows,
		[]Button{{Text: "🎁 لديّ كود هدية", Data: "premium:code"}},
		[]Button{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	)
	return Screen{Text: text, Keyboard: &Keyboard{Rows: rows}}
}

// handlePremium routes the "premium:" callbacks: gift-code entry, plan choice,
// or the legacy generic request (when no plans are configured).
func (a *App) handlePremium(ctx context.Context, b *tg.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil {
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	data := strings.TrimPrefix(cq.Data, "premium:")

	switch {
	case data == "code":
		a.sessions.set(cq.From.ID, stateAwaitGiftCode)
		a.send(ctx, cq.From.ID, Screen{
			Text:     "🎁 أرسل كود الهدية الذي حصلت عليه:",
			Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ رجوع", Data: "menu:home"}}}},
		})
	case strings.HasPrefix(data, "plan:"):
		a.startPlanPayment(ctx, cq.From.ID, &cq.From, strings.TrimPrefix(data, "plan:"))
	default:
		a.legacyPremiumRequest(ctx, &cq.From)
	}
}

// startPlanPayment shows the payment instructions for a chosen plan and awaits
// the user's payment proof.
func (a *App) startPlanPayment(ctx context.Context, userID int64, from *models.User, planID string) {
	p, ok := a.plans.Get(planID)
	if !ok {
		a.send(ctx, userID, subscribeScreen(a.settings.Get(), a.plans.List()))
		return
	}
	bs := a.settings.Get()
	text := "📦 باقة: " + p.Name + " — " + strconv.Itoa(p.Price) + " د.ع / " + p.DurationLabel() + "\n\n"
	if pay := strings.TrimSpace(bs.PaymentDetails); pay != "" {
		text += "💳 حوّل المبلغ عبر:\n" + pay + "\n\n"
	}
	text += "ثم أرسل إثبات الدفع هنا: صورة الوصل، أو رقم العملية واسم المُرسِل."
	a.sessions.startPayProof(userID, planID)
	a.send(ctx, userID, Screen{
		Text:     text,
		Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ إلغاء", Data: "menu:home"}}}},
	})
}

// handlePayProof captures the user's payment proof (text and/or a photo) and
// files a pending subscription request tagged with the chosen plan.
func (a *App) handlePayProof(ctx context.Context, msg *models.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	planID := a.sessions.payPlan(userID)
	p, ok := a.plans.Get(planID)
	if !ok {
		a.sessions.clear(userID)
		a.send(ctx, chatID, subscribeScreen(a.settings.Get(), a.plans.List()))
		return
	}

	proof := strings.TrimSpace(msg.Text)
	if proof == "" {
		proof = strings.TrimSpace(msg.Caption)
	}
	var fileID string
	if len(msg.Photo) > 0 {
		fileID = msg.Photo[len(msg.Photo)-1].FileID // largest size
	}
	if proof == "" && fileID == "" {
		a.send(ctx, chatID, Screen{
			Text:     "أرسل إثبات الدفع: صورة الوصل، أو رقم العملية واسم المُرسِل.",
			Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ إلغاء", Data: "menu:home"}}}},
		})
		return // stay in the proof state
	}

	// One open request at a time keeps the admin queue clean.
	if pending, _ := a.store.ListSubscriptionRequests(ctx, domain.SubReqPending); pending != nil {
		for _, r := range pending {
			if r.UserID == userID {
				a.sessions.clear(userID)
				a.send(ctx, chatID, Screen{
					Text:     "🕓 لديك طلب اشتراك قيد المراجعة بالفعل. سنفعّله فور تأكيد الدفع. شكراً لصبرك 🌟",
					Keyboard: supportNav(),
				})
				return
			}
		}
	}

	req := domain.SubscriptionRequest{
		ID:          subReqID(userID),
		UserID:      userID,
		UserName:    userDisplayName(msg.From),
		PlanID:      p.ID,
		PlanName:    p.Name,
		Months:      p.Months,
		Tier:        p.Tier,
		Price:       p.Price,
		Proof:       proof,
		ProofFileID: fileID,
		Status:      domain.SubReqPending,
	}
	a.sessions.clear(userID)
	if err := a.store.AddSubscriptionRequest(ctx, req); err != nil {
		a.logf("add subscription request: %v", err)
		a.send(ctx, chatID, Screen{Text: "⚠️ تعذّر تسجيل طلبك الآن، حاول لاحقاً.", Keyboard: supportNav()})
		return
	}
	a.send(ctx, chatID, Screen{
		Text: "✅ وصل طلب اشتراكك في باقة «" + p.Name + "»!\n" +
			"سنراجع الدفع ونفعّل اشتراكك قريباً، وسيصلك إشعار فور التفعيل. شكراً لك 🌟",
		Keyboard: supportNav(),
	})
}

// ProofImage fetches a payment-proof photo by Telegram file id, implementing
// web.ProofImages so the admin queue can preview receipts. Telegram photos are
// JPEG.
func (a *App) ProofImage(ctx context.Context, fileID string) ([]byte, string, error) {
	data, err := a.downloadFile(ctx, fileID)
	if err != nil {
		return nil, "", err
	}
	return data, "image/jpeg", nil
}

// subReqID derives a short stable id for a subscription request.
func subReqID(userID int64) string {
	seed := "subreq|" + strconv.FormatInt(userID, 10) + "|" + strconv.FormatInt(time.Now().UnixNano(), 10)
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:])[:10]
}

// legacyPremiumRequest files a generic support ticket when no plans exist.
func (a *App) legacyPremiumRequest(ctx context.Context, from *models.User) {
	const msg = "💎 طلب اشتراك بريميم — يرجى تأكيد الدفع وتفعيل الحساب."
	ticket := domain.Ticket{
		ID:       supportID(from.ID, msg),
		UserID:   from.ID,
		UserName: userDisplayName(from),
		Message:  msg,
		Status:   domain.TicketOpen,
	}
	if err := a.store.AddTicket(ctx, ticket); err != nil {
		a.logf("premium request add: %v", err)
	}
	a.send(ctx, from.ID, Screen{
		Text:     "✅ وصل طلب اشتراكك!\nسيتواصل معك فريقنا لتأكيد الدفع وتفعيل المزايا المميّزة قريباً. شكراً لك 🌟",
		Keyboard: supportNav(),
	})
}

// handleGiftCode redeems a gift code the user sent and grants its premium tier.
func (a *App) handleGiftCode(ctx context.Context, msg *models.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	a.sessions.clear(userID)

	code := strings.ToUpper(strings.TrimSpace(msg.Text))
	if code == "" {
		a.send(ctx, chatID, Screen{Text: "أرسل كوداً صحيحاً.", Keyboard: giftNav()})
		return
	}

	g, err := a.store.RedeemGiftCode(ctx, code, userID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		a.send(ctx, chatID, Screen{Text: "❌ كود غير صحيح. تأكّد منه وحاول مجدداً.", Keyboard: giftNav()})
		return
	case errors.Is(err, store.ErrCodeUsed):
		a.send(ctx, chatID, Screen{Text: "❌ هذا الكود مُستخدَم مسبقاً.", Keyboard: giftNav()})
		return
	case err != nil:
		a.logf("redeem gift: %v", err)
		a.send(ctx, chatID, Screen{Text: "⚠️ تعذّر تفعيل الكود الآن، جرّب لاحقاً.", Keyboard: giftNav()})
		return
	}

	u, gerr := a.store.GetUser(ctx, userID)
	if gerr != nil || u == nil {
		u = &domain.User{TelegramID: userID, Name: userDisplayName(msg.From)}
	}
	u.Tier = g.Tier
	if g.Days > 0 {
		until := time.Now().AddDate(0, 0, g.Days)
		u.PremiumUntil = &until
	} else {
		u.PremiumUntil = nil
	}
	if err := a.store.SaveUser(ctx, u); err != nil {
		a.logf("save user after gift: %v", err)
	}

	dur := "بشكل دائم"
	if g.Days > 0 {
		dur = "لمدة " + strconv.Itoa(g.Days) + " يوم"
	}
	a.send(ctx, chatID, Screen{
		Text:     "🎉 تم تفعيل اشتراكك المميّز " + dur + "! استمتع بكامل الميزات 🌟",
		Keyboard: &Keyboard{Rows: [][]Button{{{Text: "⬅️ القائمة الرئيسية", Data: "menu:home"}}}},
	})
}

func giftNav() *Keyboard {
	return &Keyboard{Rows: [][]Button{
		{{Text: "🎁 جرّب كوداً آخر", Data: "premium:code"}},
		{{Text: "⬅️ رجوع للقائمة", Data: "menu:home"}},
	}}
}
