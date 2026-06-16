package announce

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func ptr(t time.Time) *time.Time { return &t }

var now = date(2026, time.June, 16)

func fixture() *Repo {
	return NewRepo([]Announcement{
		{
			ID: "conf-ai", Kind: KindConference, Title: "مؤتمر الذكاء الاصطناعي",
			Disciplines: []string{"cs"}, Deadline: ptr(date(2026, time.August, 15)),
			PostedAt: date(2026, time.June, 10),
		},
		{
			ID: "grant-med", Kind: KindGrant, Title: "منحة طبية",
			Disciplines: []string{"med"}, Deadline: ptr(date(2026, time.July, 1)),
			PostedAt: date(2026, time.June, 14),
		},
		{
			ID: "job-general", Kind: KindJob, Title: "وظيفة بحثية",
			Disciplines: nil, PostedAt: date(2026, time.June, 12),
		},
		{
			ID: "conf-expired", Kind: KindConference, Title: "مؤتمر منتهٍ",
			Disciplines: []string{"cs"}, Deadline: ptr(date(2026, time.May, 1)),
			PostedAt: date(2026, time.April, 1),
		},
	})
}

func TestList_ExcludesExpiredAndSortsNewestFirst(t *testing.T) {
	got := fixture().List(now, Filter{})
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (expired excluded)", len(got))
	}
	wantOrder := []string{"grant-med", "job-general", "conf-ai"}
	for i, id := range wantOrder {
		if got[i].ID != id {
			t.Errorf("position %d = %q, want %q", i, got[i].ID, id)
		}
	}
}

func TestList_IncludeExpired(t *testing.T) {
	got := fixture().List(now, Filter{IncludeExpired: true})
	if len(got) != 4 {
		t.Errorf("len = %d, want 4 with expired", len(got))
	}
}

func TestList_FilterByKind(t *testing.T) {
	got := fixture().List(now, Filter{Kinds: []Kind{KindConference, KindCFP}})
	if len(got) != 1 || got[0].ID != "conf-ai" {
		t.Errorf("kind filter = %+v, want only conf-ai", ids(got))
	}
}

func TestList_FilterByDiscipline(t *testing.T) {
	got := fixture().List(now, Filter{Discipline: "cs"})
	// cs items: conf-ai (matches), plus job-general (no tags => general).
	if got2 := ids(got); len(got) != 2 {
		t.Fatalf("discipline cs = %v, want 2 (conf-ai + general job)", got2)
	}
	for _, a := range got {
		if a.ID == "grant-med" {
			t.Error("med grant should not match cs discipline")
		}
	}
}

func TestDaysLeft(t *testing.T) {
	a := Announcement{Deadline: ptr(date(2026, time.June, 26))}
	if d, ok := a.DaysLeft(now); !ok || d != 10 {
		t.Errorf("DaysLeft = (%d, %v), want (10, true)", d, ok)
	}
	past := Announcement{Deadline: ptr(date(2026, time.June, 1))}
	if d, ok := past.DaysLeft(now); !ok || d != 0 {
		t.Errorf("past DaysLeft = (%d, %v), want (0, true)", d, ok)
	}
	none := Announcement{}
	if _, ok := none.DaysLeft(now); ok {
		t.Error("DaysLeft without deadline should report ok=false")
	}
}

func TestExpiredAndMatchesDiscipline(t *testing.T) {
	exp := Announcement{Deadline: ptr(date(2026, time.May, 1))}
	if !exp.Expired(now) {
		t.Error("should be expired")
	}
	gen := Announcement{}
	if !gen.MatchesDiscipline("anything") {
		t.Error("untagged item should match any discipline")
	}
	cs := Announcement{Disciplines: []string{"cs", "ai"}}
	if !cs.MatchesDiscipline("AI") || cs.MatchesDiscipline("med") {
		t.Error("discipline matching (case-insensitive) is wrong")
	}
	if !cs.MatchesDiscipline("") {
		t.Error("empty field should match everything")
	}
}

func ids(as []Announcement) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.ID
	}
	return out
}
