package pptx

// Static, known-minimal OOXML parts shared by every deck: a slide master, one
// blank layout, and a theme with the full color/font/format schemes PowerPoint
// requires. Slides carry their own text boxes, so they don't depend on these
// for content — these just satisfy the package structure.

const slideMasterRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>` +
	`</Relationships>`

const slideLayoutRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>` +
	`</Relationships>`

// slideRels links a slide to the layout, plus its image when present (rId2).
func slideRels(slideNum string, hasImage bool) string {
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>`
	if hasImage {
		rels += `<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image` + slideNum + `.jpeg"/>`
	}
	return rels + `</Relationships>`
}

const emptySpTree = `<p:spTree>` +
	`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
	`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
	`</p:spTree>`

const slideMasterXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
	`<p:cSld>` + emptySpTree + `</p:cSld>` +
	`<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>` +
	`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>` +
	`<p:txStyles>` +
	`<p:titleStyle><a:lvl1pPr><a:defRPr sz="4400"/></a:lvl1pPr></p:titleStyle>` +
	`<p:bodyStyle><a:lvl1pPr><a:defRPr sz="2400"/></a:lvl1pPr></p:bodyStyle>` +
	`<p:otherStyle><a:lvl1pPr><a:defRPr sz="1800"/></a:lvl1pPr></p:otherStyle>` +
	`</p:txStyles>` +
	`</p:sldMaster>`

const slideLayoutXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">` +
	`<p:cSld name="Blank">` + emptySpTree + `</p:cSld>` +
	`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
	`</p:sldLayout>`

const themeXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Manhal">` +
	`<a:themeElements>` +
	`<a:clrScheme name="Manhal">` +
	`<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>` +
	`<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
	`<a:dk2><a:srgbClr val="1F2937"/></a:dk2>` +
	`<a:lt2><a:srgbClr val="EEF2FF"/></a:lt2>` +
	`<a:accent1><a:srgbClr val="4F46E5"/></a:accent1>` +
	`<a:accent2><a:srgbClr val="0EA5E9"/></a:accent2>` +
	`<a:accent3><a:srgbClr val="10B981"/></a:accent3>` +
	`<a:accent4><a:srgbClr val="F59E0B"/></a:accent4>` +
	`<a:accent5><a:srgbClr val="EF4444"/></a:accent5>` +
	`<a:accent6><a:srgbClr val="8B5CF6"/></a:accent6>` +
	`<a:hlink><a:srgbClr val="2563EB"/></a:hlink>` +
	`<a:folHlink><a:srgbClr val="7C3AED"/></a:folHlink>` +
	`</a:clrScheme>` +
	`<a:fontScheme name="Manhal">` +
	`<a:majorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>` +
	`<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont>` +
	`</a:fontScheme>` +
	`<a:fmtScheme name="Manhal">` +
	`<a:fillStyleLst>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`</a:fillStyleLst>` +
	`<a:lnStyleLst>` +
	`<a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>` +
	`<a:ln w="12700"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>` +
	`<a:ln w="19050"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>` +
	`</a:lnStyleLst>` +
	`<a:effectStyleLst>` +
	`<a:effectStyle><a:effectLst/></a:effectStyle>` +
	`<a:effectStyle><a:effectLst/></a:effectStyle>` +
	`<a:effectStyle><a:effectLst/></a:effectStyle>` +
	`</a:effectStyleLst>` +
	`<a:bgFillStyleLst>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
	`</a:bgFillStyleLst>` +
	`</a:fmtScheme>` +
	`</a:themeElements>` +
	`</a:theme>`
