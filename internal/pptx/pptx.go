// Package pptx builds a minimal, editable PowerPoint (.pptx) deck from a set of
// text slides. Each slide uses explicit text boxes (title + bullets) so it does
// not depend on placeholder inheritance from the master/layout, which keeps the
// generated file robust across PowerPoint, Keynote and Google Slides.
package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Slide is one slide's content.
type Slide struct {
	Title   string
	Bullets []string
}

// Build returns the bytes of a .pptx deck. The first slide is a title slide for
// deckTitle; the rest follow.
func Build(deckTitle string, slides []Slide) ([]byte, error) {
	all := make([]Slide, 0, len(slides)+1)
	all = append(all, Slide{Title: strings.TrimSpace(deckTitle)})
	all = append(all, slides...)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, data string) error {
		w, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("pptx %s: %w", name, err)
		}
		_, err = w.Write([]byte(data))
		return err
	}

	if err := write("[Content_Types].xml", contentTypes(len(all))); err != nil {
		return nil, err
	}
	for name, data := range map[string]string{
		"_rels/.rels":                                  rootRels,
		"ppt/presentation.xml":                         presentationXML(len(all)),
		"ppt/_rels/presentation.xml.rels":              presentationRels(len(all)),
		"ppt/theme/theme1.xml":                         themeXML,
		"ppt/slideMasters/slideMaster1.xml":            slideMasterXML,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": slideMasterRels,
		"ppt/slideLayouts/slideLayout1.xml":            slideLayoutXML,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": slideLayoutRels,
	} {
		if err := write(name, data); err != nil {
			return nil, err
		}
	}
	for i, s := range all {
		n := strconv.Itoa(i + 1)
		if err := write("ppt/slides/slide"+n+".xml", slideXML(s, i == 0)); err != nil {
			return nil, err
		}
		if err := write("ppt/slides/_rels/slide"+n+".xml.rels", slideRels); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("pptx close: %w", err)
	}
	return buf.Bytes(), nil
}

func contentTypes(nSlides int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>` +
		`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`)
	for i := 1; i <= nSlides; i++ {
		b.WriteString(`<Override PartName="/ppt/slides/slide` + strconv.Itoa(i) +
			`.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`)
	}
	b.WriteString(`</Types>`)
	return b.String()
}

const rootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
	`</Relationships>`

func presentationXML(nSlides int) string {
	var ids strings.Builder
	for i := 1; i <= nSlides; i++ {
		// slide rels are rId2.. (rId1 is the master); slide ids start at 256.
		ids.WriteString(`<p:sldId id="` + strconv.Itoa(255+i) + `" r:id="rId` + strconv.Itoa(i+1) + `"/>`)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
		`<p:sldIdLst>` + ids.String() + `</p:sldIdLst>` +
		`<p:sldSz cx="12192000" cy="6858000" type="screen16x9"/>` +
		`<p:notesSz cx="6858000" cy="9144000"/>` +
		`</p:presentation>`
}

func presentationRels(nSlides int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	for i := 1; i <= nSlides; i++ {
		b.WriteString(`<Relationship Id="rId` + strconv.Itoa(i+1) +
			`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide` + strconv.Itoa(i) + `.xml"/>`)
	}
	b.WriteString(`<Relationship Id="rId` + strconv.Itoa(nSlides+2) +
		`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`)
	b.WriteString(`</Relationships>`)
	return b.String()
}

// slideXML renders one slide with explicit title + body text boxes.
func slideXML(s Slide, isTitle bool) string {
	titleSize, titleY := "3600", "2057400"
	if !isTitle {
		titleSize, titleY = "3200", "457200"
	}
	var body strings.Builder
	if !isTitle {
		body.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr/></p:nvSpPr>` +
			`<p:spPr><a:xfrm><a:off x="685800" y="1714500"/><a:ext cx="10820400" cy="4572000"/></a:xfrm>` +
			`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
			`<p:txBody><a:bodyPr/><a:lstStyle/>`)
		if len(s.Bullets) == 0 {
			body.WriteString(`<a:p/>`)
		}
		for _, bl := range s.Bullets {
			body.WriteString(para("• "+bl, "2000", false, dirRTL(bl)))
		}
		body.WriteString(`</p:txBody></p:sp>`)
	}

	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr/></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="685800" y="` + titleY + `"/><a:ext cx="10820400" cy="1143000"/></a:xfrm>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/>` + para(s.Title, titleSize, true, dirRTL(s.Title)) + `</p:txBody></p:sp>` +
		body.String() +
		`</p:spTree></p:cSld></p:sld>`
}

// para builds one <a:p> with alignment/direction for the given text.
func para(text, size string, bold bool, rtl bool) string {
	algn := "l"
	rtlAttr := ""
	if rtl {
		algn = "r"
		rtlAttr = ` rtl="1"`
	}
	b := ""
	if bold {
		b = ` b="1"`
	}
	return `<a:p><a:pPr algn="` + algn + `"` + rtlAttr + `/><a:r><a:rPr lang="ar-IQ" sz="` + size + `"` + b + `/><a:t>` + escapeXML(text) + `</a:t></a:r></a:p>`
}

// dirRTL reports whether text leads with an Arabic/Hebrew (RTL) character.
func dirRTL(s string) bool {
	for _, r := range s {
		switch {
		case (r >= 0x0590 && r <= 0x05FF), (r >= 0x0600 && r <= 0x06FF),
			(r >= 0x0750 && r <= 0x077F), (r >= 0x08A0 && r <= 0x08FF),
			(r >= 0xFB50 && r <= 0xFDFF), (r >= 0xFE70 && r <= 0xFEFF):
			return true
		case (r >= 'A' && r <= 'Z'), (r >= 'a' && r <= 'z'):
			return false
		}
	}
	return true // default RTL (Arabic-first bot)
}

func escapeXML(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
