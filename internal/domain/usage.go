package domain

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
