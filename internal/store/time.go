package store

import "time"

// baghdad is the timezone used for all time-of-day / weekday analytics. Iraq
// observes UTC+3 year-round (no DST since 2008); a fixed zone is the fallback
// if the tzdata database is unavailable in the runtime image.
var baghdad = loadBaghdad()

func loadBaghdad() *time.Location {
	if loc, err := time.LoadLocation("Asia/Baghdad"); err == nil {
		return loc
	}
	return time.FixedZone("Asia/Baghdad", 3*60*60)
}
