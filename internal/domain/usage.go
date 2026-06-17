package domain

import "time"

// UsageEvent is a single timestamped feature invocation by a user, used for the
// per-user activity report (diagnosing where a user got stuck).
type UsageEvent struct {
	Action string
	At     time.Time
}

// FeatureCount is a usage tally for a single feature (menu action).
type FeatureCount struct {
	Action string
	Count  int
}

// UserUsage is a single user's total activity, used for "most active users".
type UserUsage struct {
	UserID int64
	Name   string
	Count  int
}
