package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestBuild(t *testing.T) {
	data, err := Build("عرضي التقديمي", []Slide{
		{Title: "المقدمة", Bullets: []string{"نقطة أولى", "نقطة ثانية فيها <رمز>"}},
		{Title: "Methods", Bullets: []string{"a point"}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(b)
	}

	// Title slide + 2 content slides = 3 slides.
	for _, want := range []string{
		"[Content_Types].xml", "_rels/.rels",
		"ppt/presentation.xml", "ppt/_rels/presentation.xml.rels",
		"ppt/theme/theme1.xml",
		"ppt/slideMasters/slideMaster1.xml", "ppt/slideMasters/_rels/slideMaster1.xml.rels",
		"ppt/slideLayouts/slideLayout1.xml", "ppt/slideLayouts/_rels/slideLayout1.xml.rels",
		"ppt/slides/slide1.xml", "ppt/slides/slide2.xml", "ppt/slides/slide3.xml",
		"ppt/slides/_rels/slide1.xml.rels",
	} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing part %q", want)
		}
	}
	if _, ok := files["ppt/slides/slide4.xml"]; ok {
		t.Error("unexpected 4th slide")
	}

	// The deck title is on slide 1; content + escaped text on slide 2.
	if !strings.Contains(files["ppt/slides/slide1.xml"], "عرضي التقديمي") {
		t.Error("title slide missing deck title")
	}
	if !strings.Contains(files["ppt/slides/slide2.xml"], "المقدمة") ||
		!strings.Contains(files["ppt/slides/slide2.xml"], "نقطة أولى") {
		t.Error("content slide missing title/bullet")
	}
	if !strings.Contains(files["ppt/slides/slide2.xml"], "&lt;رمز&gt;") {
		t.Error("XML special chars not escaped in bullet")
	}
	// Presentation lists 3 slide ids.
	if n := strings.Count(files["ppt/presentation.xml"], "<p:sldId "); n != 3 {
		t.Errorf("presentation lists %d slides, want 3", n)
	}
}
