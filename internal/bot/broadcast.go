package bot

import (
	"context"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// SendRich delivers a broadcast message to one user: an optional photo (by URL)
// with the text as caption, plus an optional inline URL button. It implements
// the web.Notifier rich-send method used by the admin broadcast tool.
func (a *App) SendRich(userID int64, text, imageURL, buttonLabel, buttonURL string) error {
	var markup models.ReplyMarkup
	if buttonURL != "" {
		label := buttonLabel
		if label == "" {
			label = "🔗 فتح"
		}
		markup = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: label, URL: buttonURL}},
		}}
	}
	ctx := context.Background()
	text = cleanMarkdown(text)

	if imageURL != "" {
		_, err := a.bot.SendPhoto(ctx, &tg.SendPhotoParams{
			ChatID:      userID,
			Photo:       &models.InputFileString{Data: imageURL},
			Caption:     text,
			ReplyMarkup: markup,
		})
		return err
	}
	_, err := a.bot.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:      userID,
		Text:        text,
		ReplyMarkup: markup,
	})
	return err
}
