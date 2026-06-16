package bot

import (
	"strings"
	"testing"
)

func TestStatsMenuScreen_ListsTests(t *testing.T) {
	scr := statsMenuScreen()
	var testButtons int
	for _, row := range scr.Keyboard.Rows {
		for _, b := range row {
			if strings.HasPrefix(b.Data, "stats:test:") {
				testButtons++
			}
		}
	}
	if testButtons != len(statsTests) {
		t.Errorf("test buttons = %d, want %d", testButtons, len(statsTests))
	}
}

func TestParseStatsGroups(t *testing.T) {
	groups := parseStatsGroups("1 2 3\n4, 5, 6\n٧ ٨ ٩\nنص بلا أرقام")
	if len(groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(groups))
	}
	if groups[1][2] != 6 {
		t.Errorf("comma parsing failed: %v", groups[1])
	}
	if groups[2][0] != 7 { // Arabic-Indic digits
		t.Errorf("arabic digits failed: %v", groups[2])
	}
}

func TestRunStatsTest_Describe(t *testing.T) {
	out, err := runStatsTest("describe", [][]float64{{2, 4, 4, 5, 5, 7, 9}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "N = 7") || !strings.Contains(out, "M = 5.14") {
		t.Errorf("describe output wrong:\n%s", out)
	}
}

func TestRunStatsTest_TTest(t *testing.T) {
	out, err := runStatsTest("ttest", [][]float64{{5, 6, 7, 8, 9}, {3, 4, 5, 6, 7}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "t(8.0) = 2.00") {
		t.Errorf("t-test stat wrong:\n%s", out)
	}
	if !strings.Contains(out, "p = .08") {
		t.Errorf("t-test p wrong:\n%s", out)
	}
}

func TestRunStatsTest_ANOVA(t *testing.T) {
	out, err := runStatsTest("anova", [][]float64{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "F(2.0 ، 6.0) = 27.00") || !strings.Contains(out, "p = .001") {
		t.Errorf("anova output wrong:\n%s", out)
	}
	if !strings.Contains(out, "دالة إحصائياً") {
		t.Errorf("anova significance missing:\n%s", out)
	}
}

func TestRunStatsTest_CronbachAndInsufficient(t *testing.T) {
	out, err := runStatsTest("cronbach", [][]float64{{1, 2, 3}, {1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "α = 1.00") || !strings.Contains(out, "ممتاز") {
		t.Errorf("cronbach output wrong:\n%s", out)
	}

	if _, err := runStatsTest("ttest", [][]float64{{1, 2}}); err == nil {
		t.Error("t-test with one group should error")
	}
}

func TestFmtP(t *testing.T) {
	if fmtP(0.0005) != "p < .001" {
		t.Errorf("tiny p = %q", fmtP(0.0005))
	}
	if fmtP(0.045) != "p = .045" {
		t.Errorf("p = %q, want 'p = .045'", fmtP(0.045))
	}
}

func TestSessions_Stats(t *testing.T) {
	s := newSessions()
	s.startStats(3, "anova")
	if s.get(3) != stateAwaitStats {
		t.Error("startStats should set await-stats state")
	}
	if s.statsTest(3) != "anova" {
		t.Errorf("statsTest = %q, want anova", s.statsTest(3))
	}
}
