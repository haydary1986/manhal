// Package promotion is a rule-based academic promotion-points calculator (P13),
// implementing تعليمات الترقيات العلمية رقم (١٠) لسنة ٢٠٢٥ (which repealed
// تعليمات ١٦٧/٢٠١٧). It is deterministic and free of AI cost.
//
// Each promotion has FOUR gates (المواد ١–٣):
//   - a total-points minimum,
//   - a minimum from Table 1 (research / scientific output),
//   - a minimum from Table 2 (activities & community service),
//   - a minimum service period in the current rank.
//
// Table-1 points depend on the journal class, the author's position, AND the
// target rank, so an activity carries per-rank point values. Table-2 items carry
// per-unit points and a cap. All values live in an editable data file
// (data/promotion.yaml) so they track ministry amendments without code changes.
// Results are advisory; the promotion committee decides.
package promotion

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Activity is a scored item. Table is 1 (research) or 2 (activities). Points map
// a current-rank key to the per-unit value; the key "*" is a fallback for all
// ranks (used by Table-2 items, whose value is rank-independent). Cap bounds the
// total contribution of this activity (0 = uncapped).
type Activity struct {
	Key    string             `yaml:"key"`
	Label  string             `yaml:"label"`
	Table  int                `yaml:"table"`
	Points map[string]float64 `yaml:"points"`
	Cap    float64            `yaml:"cap"`
}

// PointsFor returns the per-unit value for a current rank.
func (a Activity) PointsFor(rankKey string) float64 {
	if v, ok := a.Points[rankKey]; ok {
		return v
	}
	if v, ok := a.Points["*"]; ok {
		return v
	}
	return 0
}

// Rank is a current rank plus the gates required to be promoted from it.
type Rank struct {
	Key             string  `yaml:"key"`
	Label           string  `yaml:"label"`
	NextLabel       string  `yaml:"next_label"`
	RequiredTotal   float64 `yaml:"required_total"`
	RequiredTable1  float64 `yaml:"required_table1"`
	RequiredTable2  float64 `yaml:"required_table2"`
	MinServiceYears int     `yaml:"min_service_years"`
}

// Rules is the full editable rule set.
type Rules struct {
	Activities []Activity `yaml:"activities"`
	Ranks      []Rank     `yaml:"ranks"`
}

// FindRank returns the rank with the given key.
func (r *Rules) FindRank(key string) (Rank, bool) {
	for _, rk := range r.Ranks {
		if rk.Key == key {
			return rk, true
		}
	}
	return Rank{}, false
}

// ActivitiesByTable returns the activities belonging to a table, in order.
func (r *Rules) ActivitiesByTable(table int) []Activity {
	var out []Activity
	for _, a := range r.Activities {
		if a.Table == table {
			out = append(out, a)
		}
	}
	return out
}

// Input is one calculation request.
type Input struct {
	RankKey      string
	Counts       map[string]float64 // activity key -> count
	ServiceYears int
}

// LineItem is one row of the breakdown (after applying the per-activity cap).
type LineItem struct {
	Label  string
	Count  float64
	Points float64
	Capped bool
}

// Result is the computed outcome with all four gates evaluated.
type Result struct {
	Rank         Rank
	Table1       float64
	Table2       float64
	Total        float64
	Breakdown1   []LineItem
	Breakdown2   []LineItem
	ServiceYears int

	TotalMet   bool
	Table1Met  bool
	Table2Met  bool
	ServiceMet bool
	Eligible   bool
}

// Compute scores the input against the rules. Unknown activity keys are ignored.
func (r *Rules) Compute(in Input) (Result, bool) {
	rank, ok := r.FindRank(in.RankKey)
	if !ok {
		return Result{}, false
	}
	res := Result{Rank: rank, ServiceYears: in.ServiceYears}

	for _, act := range r.Activities {
		count := in.Counts[act.Key]
		if count <= 0 {
			continue
		}
		pts := count * act.PointsFor(rank.Key)
		capped := false
		if act.Cap > 0 && pts > act.Cap {
			pts = act.Cap
			capped = true
		}
		li := LineItem{Label: act.Label, Count: count, Points: pts, Capped: capped}
		if act.Table == 1 {
			res.Table1 += pts
			res.Breakdown1 = append(res.Breakdown1, li)
		} else {
			res.Table2 += pts
			res.Breakdown2 = append(res.Breakdown2, li)
		}
	}

	res.Total = res.Table1 + res.Table2
	res.TotalMet = res.Total >= rank.RequiredTotal
	res.Table1Met = res.Table1 >= rank.RequiredTable1
	res.Table2Met = res.Table2 >= rank.RequiredTable2
	res.ServiceMet = in.ServiceYears >= rank.MinServiceYears
	res.Eligible = res.TotalMet && res.Table1Met && res.Table2Met && res.ServiceMet
	return res, true
}

// ParseActivities parses lines of "key: number" into activity counts plus the
// service-years value (key "years"). Unknown lines are skipped; Arabic-Indic
// digits are accepted.
func ParseActivities(text string) (counts map[string]float64, serviceYears int) {
	counts = map[string]float64{}
	for _, line := range strings.Split(text, "\n") {
		key, valStr, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		val, err := strconv.ParseFloat(normalizeDigits(valStr), 64)
		if err != nil || val < 0 {
			continue
		}
		if key == "years" || key == "سنوات" {
			serviceYears = int(val)
			continue
		}
		counts[key] = val
	}
	return counts, serviceYears
}

// splitKeyValue splits a line on the first ':' (ASCII or Arabic fullwidth '：'),
// trimming both sides and lowercasing the key.
func splitKeyValue(line string) (key, val string, ok bool) {
	i := strings.IndexAny(line, ":：")
	if i < 0 {
		return "", "", false
	}
	r, _ := utf8.DecodeRuneInString(line[i:])
	key = strings.ToLower(strings.TrimSpace(line[:i]))
	val = strings.TrimSpace(line[i+utf8.RuneLen(r):])
	if key == "" || val == "" {
		return "", "", false
	}
	return key, val, true
}

// normalizeDigits converts Arabic-Indic digits to ASCII.
func normalizeDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '٠' && r <= '٩':
			b.WriteRune('0' + (r - '٠'))
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
