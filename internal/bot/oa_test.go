package bot

import (
	"strings"
	"testing"

	"github.com/erticaz/manhal/internal/scholar"
)

func TestOAResultScreen_OA(t *testing.T) {
	res := &scholar.OAResult{
		Title: "An Open Paper", IsOA: true,
		Locations: []scholar.OALocation{
			{URL: "https://repo/paper.pdf", IsPDF: true, HostType: "repository", Version: "publishedVersion"},
			{URL: "https://pub/article", HostType: "publisher", Version: "acceptedVersion"},
		},
	}
	scr := oaResultScreen(res)
	for _, want := range []string{"An Open Paper", "https://repo/paper.pdf", "مستودع", "النسخة المنشورة", "https://pub/article", "Unpaywall"} {
		if !strings.Contains(scr.Text, want) {
			t.Errorf("OA screen missing %q in:\n%s", want, scr.Text)
		}
	}
}

func TestOAResultScreen_Closed(t *testing.T) {
	scr := oaResultScreen(&scholar.OAResult{IsOA: false})
	if !strings.Contains(scr.Text, "ما لقيت نسخة مجانية") {
		t.Errorf("closed screen wrong:\n%s", scr.Text)
	}
	if !strings.Contains(scr.Text, "المؤلف") {
		t.Error("closed screen should suggest contacting the author")
	}
}

func TestOAResultScreen_DedupesURLs(t *testing.T) {
	res := &scholar.OAResult{
		IsOA: true,
		Locations: []scholar.OALocation{
			{URL: "https://same/pdf", IsPDF: true, HostType: "repository"},
			{URL: "https://same/pdf", IsPDF: true, HostType: "publisher"},
		},
	}
	got := strings.Count(oaResultScreen(res).Text, "https://same/pdf")
	if got != 1 {
		t.Errorf("duplicate URL shown %d times, want 1", got)
	}
}

func TestOAErrorScreen(t *testing.T) {
	cases := []struct {
		err  error
		frag string
	}{
		{scholar.ErrInvalidDOI, "DOI صحيح"},
		{scholar.ErrNotFound, "ما لقيت بيانات"},
		{scholar.ErrNotConfigured, "غير مفعّلة"},
	}
	for _, c := range cases {
		if got := oaErrorScreen(c.err).Text; !strings.Contains(got, c.frag) {
			t.Errorf("error for %v = %q, missing %q", c.err, got, c.frag)
		}
	}
}

func TestOALocationLabel(t *testing.T) {
	pdf := oaLocationLabel(scholar.OALocation{IsPDF: true, HostType: "repository", Version: "publishedVersion"})
	if !strings.HasPrefix(pdf, "📄") || !strings.Contains(pdf, "مستودع") {
		t.Errorf("pdf label = %q", pdf)
	}
	link := oaLocationLabel(scholar.OALocation{HostType: "publisher", Version: "submittedVersion"})
	if !strings.HasPrefix(link, "🔗") || !strings.Contains(link, "preprint") {
		t.Errorf("link label = %q", link)
	}
}
