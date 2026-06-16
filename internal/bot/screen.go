package bot

import "github.com/go-telegram/bot/models"

// Button is a single inline keyboard button. Exactly one of Data or URL is set.
type Button struct {
	Text string
	Data string
	URL  string
}

// Keyboard is a grid of inline buttons.
type Keyboard struct {
	Rows [][]Button
}

// Screen is a renderable bot message: text plus an optional keyboard.
type Screen struct {
	Text     string
	Keyboard *Keyboard
}

// markup converts the declarative keyboard into the Telegram markup type.
// Returns nil for a nil keyboard.
func (k *Keyboard) markup() *models.InlineKeyboardMarkup {
	if k == nil {
		return nil
	}
	rows := make([][]models.InlineKeyboardButton, 0, len(k.Rows))
	for _, row := range k.Rows {
		btns := make([]models.InlineKeyboardButton, 0, len(row))
		for _, b := range row {
			btn := models.InlineKeyboardButton{Text: b.Text}
			if b.URL != "" {
				btn.URL = b.URL
			} else {
				btn.CallbackData = b.Data
			}
			btns = append(btns, btn)
		}
		rows = append(rows, btns)
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}
