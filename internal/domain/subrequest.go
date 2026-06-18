package domain

import "time"

// SubReqStatus is the lifecycle state of a paid-subscription request.
type SubReqStatus string

const (
	SubReqPending  SubReqStatus = "pending"
	SubReqApproved SubReqStatus = "approved"
	SubReqRejected SubReqStatus = "rejected"
)

// SubscriptionRequest is a user's request to activate a paid plan after paying,
// reviewed by an admin from the queue. It carries the chosen plan (denormalized
// so the admin can activate in one click) and the user's payment proof.
type SubscriptionRequest struct {
	ID          string
	UserID      int64
	UserName    string
	PlanID      string
	PlanName    string
	Months      int  // duration to grant (0 = permanent)
	Tier        Tier // tier to grant
	Price       int
	Proof       string // free-text proof (transaction id / sender / amount)
	ProofFileID string // optional Telegram photo file id of a receipt
	Status      SubReqStatus
	Note        string // admin note, e.g. a rejection reason
	CreatedAt   time.Time
	DecidedAt   *time.Time
}
