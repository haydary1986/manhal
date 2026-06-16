package announce

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ParsesYAMLDates(t *testing.T) {
	dir := t.TempDir()
	yaml := `announcements:
  - id: conf-1
    kind: conference
    title: "مؤتمر تجريبي"
    disciplines: [cs, ai]
    deadline: 2026-08-15
    link: https://example.com/conf
    posted_at: 2026-06-10
  - id: job-1
    kind: job
    title: "وظيفة بدون موعد"
    posted_at: 2026-06-12
`
	if err := os.WriteFile(filepath.Join(dir, "announcements.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	repo, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if repo.Len() != 2 {
		t.Fatalf("Len = %d, want 2", repo.Len())
	}

	all := repo.List(date(2026, time.January, 1), Filter{})
	conf := all[len(all)-1] // oldest posted is conf-1 (posted_at 06-10)
	if conf.ID != "conf-1" {
		t.Fatalf("unexpected order: %v", ids(all))
	}
	if conf.Deadline == nil || !conf.Deadline.Equal(date(2026, time.August, 15)) {
		t.Errorf("deadline parsed wrong: %v", conf.Deadline)
	}
	if len(conf.Disciplines) != 2 || conf.Disciplines[0] != "cs" {
		t.Errorf("disciplines = %v", conf.Disciplines)
	}
	if conf.Link != "https://example.com/conf" {
		t.Errorf("link = %q", conf.Link)
	}
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	repo, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load on empty dir: %v", err)
	}
	if repo.Len() != 0 {
		t.Errorf("Len = %d, want 0", repo.Len())
	}
}
