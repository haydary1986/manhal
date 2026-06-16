package domain

import "time"

// TicketStatus is the lifecycle state of a support request.
type TicketStatus string

const (
	TicketOpen     TicketStatus = "open"
	TicketAnswered TicketStatus = "answered"
)

// Ticket is a direct support request from a user, reviewed and answered by an
// admin from the web panel; the answer is pushed back to the user via the bot.
type Ticket struct {
	ID         string
	UserID     int64
	UserName   string
	Message    string
	Reply      string
	Status     TicketStatus
	CreatedAt  time.Time
	AnsweredAt *time.Time
}
