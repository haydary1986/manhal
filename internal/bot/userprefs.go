package bot

import (
	"context"
	"time"

	"github.com/erticaz/manhal/internal/config"
	"github.com/erticaz/manhal/internal/domain"
)

// userField returns the user's saved discipline tag, or "" if none.
func (a *App) userField(ctx context.Context, userID int64) string {
	u, err := a.store.GetUser(ctx, userID)
	if err != nil {
		return ""
	}
	return u.Field
}

// setUserField persists the chosen discipline on the user record, creating the
// user if it does not exist yet.
func (a *App) setUserField(ctx context.Context, userID int64, field string) {
	u, err := a.store.GetUser(ctx, userID)
	if err != nil {
		u = &domain.User{TelegramID: userID, Tier: domain.TierFree, CreatedAt: time.Now()}
	}
	updated := *u // copy: never mutate the value returned by the store
	updated.Field = field
	if err := a.store.SaveUser(ctx, &updated); err != nil {
		a.logf("setUserField: %v", err)
	}
}

// fieldLabel renders a discipline tag for display ("" => all disciplines).
func (a *App) fieldLabel(field string) string {
	if field == "" {
		return "كل التخصصات"
	}
	return config.DisciplineLabel(a.disciplines, field)
}
