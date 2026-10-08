package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func TestMetadata(t *testing.T) {
	doc, _ := read(t, fixture{
		core: `<dc:title>Stored title</dc:title><dc:creator>Anna Bērziņa; John Smith</dc:creator>` +
			`<dc:subject>Testing</dc:subject><cp:keywords>alpha, beta; gamma</cp:keywords>` +
			`<dc:description>A short summary.</dc:description><dc:language>lv-LV</dc:language>`,
		body: p("Title", run("Visible "), run("title")) + p("Subtitle", run("The subtitle")) + p("", run("Body.")),
	})
	m := doc.Meta
	if m.Title != "Visible title" || m.Subtitle != "The subtitle" {
		t.Errorf("title/subtitle = %q / %q", m.Title, m.Subtitle)
	}
	if got := strings.Join(m.AuthorNames(), "|"); got != "Anna Bērziņa|John Smith" {
		t.Errorf("authors = %q", got)
	}
	if m.Subject != "Testing" || m.Summary != "A short summary." || m.Lang != "lv-LV" {
		t.Errorf("subject/summary/lang = %q %q %q", m.Subject, m.Summary, m.Lang)
	}
	if got := strings.Join(m.Keywords, "|"); got != "alpha|beta|gamma" {
		t.Errorf("keywords = %q", got)
	}
	expectDump(t, doc, `P("Body.")`)
}

func TestLanguageFromRuns(t *testing.T) {
	// No core language: the language of most text wins over the default.
	doc, _ := read(t, fixture{
		docLang: "en-US",
		body: p("", runPr(`<w:lang w:val="lv-LV"/>`, "Šis ir garš teksts latviešu valodā.")) +
			p("", run("Short.")),
	})
	if doc.Meta.Lang != "lv-LV" {
		t.Errorf("Lang = %q", doc.Meta.Lang)
	}
	doc, _ = read(t, fixture{docLang: "de-DE", body: p("", run("Hallo"))})
	if doc.Meta.Lang != "de-DE" {
		t.Errorf("default Lang = %q", doc.Meta.Lang)
	}
}

func TestHeadings(t *testing.T) {
	doc, _ := read(t, fixture{
		styles: `<w:style w:type="paragraph" w:styleId="Virsraksts2"><w:name w:val="Virsraksts 2"/><w:basedOn w:val="Normal"/></w:style>` +
			`<w:style w:type="paragraph" w:styleId="berschrift3"><w:name w:val="Überschrift 3"/></w:style>` +
			`<w:style w:type="paragraph" w:styleId="MyChapter"><w:name w:val="My Chapter"/><w:basedOn w:val="Heading1"/></w:style>`,
		numbering: `<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`,
		body: p("TOCHeading", run("Contents")) +
			p("TOC1", run("Introduction 1")) +
			pPr(`<w:pStyle w:val="Heading1"/>`+numPr("1", "0"), `<w:bookmarkStart w:id="1" w:name="_Toc123"/>`, run("Introduction"), `<w:bookmarkEnd w:id="1"/>`) +
			p("Virsraksts2", `<w:bookmarkStart w:id="2" w:name="Metodes un rīki"/>`, run("Metodes")) +
			p("berschrift3", run("Details")) +
			pPr(`<w:outlineLvl w:val="3"/>`, run("Direct outline")) +
			pPr(`<w:pStyle w:val="Heading2"/><w:outlineLvl w:val="9"/>`, run("Body text level")) +
			p("MyChapter", run("Custom")) +
			p("Heading1") +
			p("Heading2", runPr(`<w:b/>`, "All "), runPr(`<w:b/><w:i/>`, "bold")),
	})
	expectDump(t, doc, strings.Join([]string{
		`H1("Introduction")`,
		`H2#metodes-un-riki("Metodes")`,
		`H3("Details")`,
		`H4("Direct outline")`,
		`P("Body text level")`,
		`H1("Custom")`,
		`H2("All " I("bold"))`,
	}, "\n"))
}

func TestInlineFormatting(t *testing.T) {
	doc, _ := read(t, fixture{
		styles: `<w:style w:type="character" w:styleId="CodeChar"><w:name w:val="Inline Code"/></w:style>`,
		body: p("",
			runPr(`<w:b/>`, "bold"), runPr(`<w:b/>`, " still"), run(" "),
			runPr(`<w:i/>`, "it"), run(" "),
			runPr(`<w:u w:val="single"/>`, "under"), runPr(`<w:u w:val="none"/>`, " no"), run(" "),
			runPr(`<w:strike/>`, "gone"), runPr(`<w:dstrike/>`, "twice"), run(" "),
			runPr(`<w:vertAlign w:val="superscript"/>`, "2"), runPr(`<w:vertAlign w:val="subscript"/>`, "i"), run(" "),
			runPr(`<w:smallCaps/>`, "Small"), runPr(`<w:caps/>`, " Caps"), run(" "),
			runPr(`<w:highlight w:val="yellow"/>`, "mark"), runPr(`<w:shd w:val="clear" w:color="auto" w:fill="FFFF00"/>`, "shade"),
			runPr(`<w:shd w:val="clear" w:color="auto" w:fill="auto"/>`, " plain")) +
			p("", runPr(`<w:b/>`, "outer "), runPr(`<w:b/><w:i/>`, "both"), runPr(`<w:b/>`, " end")) +
			p("", runPr(`<w:i/>`, "x "), runPr(`<w:b/><w:i/>`, "y"), runPr(`<w:i/>`, " z")) +
			p("", run("Call "), runPr(`<w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/>`, "fmt."), runPr(`<w:rFonts w:ascii="Consolas"/>`, "Println"),
				run(" or "), runPr(`<w:rStyle w:val="VerbatimChar"/>`, "x := 1"), run(" and "), runPr(`<w:rStyle w:val="CodeChar"/>`, "y")) +
			p("", runPr(`<w:rStyle w:val="Strong"/>`, "strong"), run(" "), runPr(`<w:rStyle w:val="Emphasis"/>`, "emph"),
				run(" "), runPr(`<w:b w:val="0"/>`, "notbold"), runPr(`<w:vanish/>`, "HIDDEN")) +
			p("", run("a\tb"), `<w:r><w:tab/><w:t>c</w:t><w:br/><w:t>d</w:t><w:cr/><w:t>e</w:t><w:noBreakHyphen/><w:t>f</w:t><w:softHyphen/><w:t>g</w:t></w:r>`) +
			p("", run("  many    spaces  "), run("  here ")),
	})
	expectDump(t, doc, strings.Join([]string{
		`P(B("bold still") " " I("it") " " U("under") " no " S("gonetwice") " " Sup("2") Sub("i") " " SC("Small") " Caps " Hl("markshade") " plain")`,
		`P(B("outer " I("both") " end"))`,
		`P(I("x " B("y") " z"))`,
		`P("Call " Code("fmt.Println") " or " Code("x := 1") " and " Code("y"))`,
		`P(B("strong") " " I("emph") " notbold")`,
		`P("a b c" BR "d" BR "e‑fg")`,
		`P("many spaces here")`,
	}, "\n"))
}

func TestSymbols(t *testing.T) {
	doc, warns := read(t, fixture{
		body: p("",
			`<w:r><w:sym w:font="Symbol" w:char="F0B7"/><w:t xml:space="preserve"> item </w:t><w:sym w:font="Wingdings" w:char="F0FC"/></w:r>`,
			runPr(`<w:rFonts w:ascii="Symbol" w:hAnsi="Symbol"/>`, "a+b"),
			`<w:r><w:sym w:font="Webdings" w:char="F021"/></w:r>`),
	})
	expectDump(t, doc, `P("• item ✓α+β")`)
	if !hasWarning(warns, "Webdings") {
		t.Errorf("expected a warning for the unmapped symbol, got %v", warns)
	}
	if n := len([]rune(symbolASCII)); n != 0x7F-0x20 {
		t.Errorf("symbolASCII has %d runes", n)
	}
}

const listNumbering = `<w:abstractNum w:abstractNumId="0">` +
	`<w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>` +
	`<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%2)"/></w:lvl>` +
	`<w:lvl w:ilvl="2"><w:start w:val="1"/><w:numFmt w:val="upperRoman"/><w:lvlText w:val="%3."/></w:lvl></w:abstractNum>` +
	`<w:abstractNum w:abstractNumId="1">` +
	`<w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/></w:lvl>` +
	`<w:lvl w:ilvl="1"><w:numFmt w:val="bullet"/><w:lvlText w:val="o"/></w:lvl></w:abstractNum>` +
	`<w:abstractNum w:abstractNumId="2"><w:lvl w:ilvl="0"><w:start w:val="5"/><w:numFmt w:val="upperLetter"/></w:lvl></w:abstractNum>` +
	`<w:abstractNum w:abstractNumId="3"><w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/><w:lvlText w:val="☐"/></w:lvl></w:abstractNum>` +
	`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>` +
	`<w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>` +
	`<w:num w:numId="3"><w:abstractNumId w:val="0"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride></w:num>` +
	`<w:num w:numId="4"><w:abstractNumId w:val="2"/></w:num>` +
	`<w:num w:numId="5"><w:abstractNumId w:val="3"/></w:num>`

func TestLists(t *testing.T) {
	doc, _ := read(t, fixture{
		numbering: listNumbering,
		body: listPara("1", "0", "One") +
			listPara("1", "1", "One-a") +
			listPara("1", "2", "Deep") +
			listPara("1", "1", "One-b") +
			listPara("1", "0", "Two") +
			p("", run("Interruption.")) +
			listPara("1", "0", "Three") +
			listPara("2", "0", "Bullet") +
			listPara("2", "1", "Sub bullet") +
			p("", run("Break.")) +
			listPara("3", "0", "Restarted") +
			listPara("4", "0", "Letter E") +
			listPara("5", "0", "Todo"),
	})
	expectDump(t, doc, strings.Join([]string{
		`OL(1,0)[Item[Plain("One") OL(1,1)[Item[Plain("One-a") OL(1,4)[Item[Plain("Deep")]]] Item[Plain("One-b")]]] Item[Plain("Two")]]`,
		`P("Interruption.")`,
		`OL(3,0)[Item[Plain("Three")]]`,
		`UL[Item[Plain("Bullet") UL[Item[Plain("Sub bullet")]]]]`,
		`P("Break.")`,
		`OL(1,0)[Item[Plain("Restarted")]]`,
		`OL(5,2)[Item[Plain("Letter E")]]`,
		`UL[Item[ ][Plain("Todo")]]`,
	}, "\n"))
}

func TestListContinuationParagraph(t *testing.T) {
	doc, _ := read(t, fixture{
		numbering: listNumbering,
		body: listPara("2", "0", "First item") +
			p("ListParagraph", run("More about the first item.")) +
			listPara("2", "0", "Second") +
			p("", run("After.")),
	})
	expectDump(t, doc, strings.Join([]string{
		`ULloose[Item[P("First item") P("More about the first item.")] Item[P("Second")]]`,
		`P("After.")`,
	}, "\n"))
}

func TestNumberingViaStyle(t *testing.T) {
	// The numbering lives in the paragraph style, and levels link back to
	// styles through w:pStyle.
	doc, _ := read(t, fixture{
		styles: `<w:style w:type="paragraph" w:styleId="ListNumber"><w:name w:val="List Number"/><w:pPr><w:numPr><w:numId w:val="7"/></w:numPr></w:pPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="ListNumber2"><w:name w:val="List Number 2"/><w:pPr><w:numPr><w:numId w:val="7"/></w:numPr></w:pPr></w:style>`,
		numbering: `<w:abstractNum w:abstractNumId="9"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:pStyle w:val="ListNumber"/></w:lvl>` +
			`<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="lowerRoman"/><w:pStyle w:val="ListNumber2"/></w:lvl></w:abstractNum>` +
			`<w:num w:numId="7"><w:abstractNumId w:val="9"/></w:num>`,
		body: p("ListNumber", run("a")) + p("ListNumber2", run("b")) + p("ListNumber", run("c")) +
			pPr(`<w:pStyle w:val="ListNumber"/><w:numPr><w:numId w:val="0"/></w:numPr>`, run("not a list")),
	})
	expectDump(t, doc, strings.Join([]string{
		`OL(1,0)[Item[Plain("a") OL(1,3)[Item[Plain("b")]]] Item[Plain("c")]]`,
		`P("not a list")`,
	}, "\n"))
}

func TestTable(t *testing.T) {
	cell := func(tcPr, content string) string {
		if tcPr != "" {
			tcPr = `<w:tcPr>` + tcPr + `</w:tcPr>`
		}
		return `<w:tc>` + tcPr + content + `</w:tc>`
	}
	right := func(text string) string { return pPr(`<w:jc w:val="right"/>`, run(text)) }
	tbl := `<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/></w:tblPr><w:tblGrid><w:gridCol w:w="2000"/><w:gridCol w:w="1000"/><w:gridCol w:w="1000"/></w:tblGrid>` +
		`<w:tr><w:trPr><w:tblHeader/></w:trPr>` + cell("", p("", runPr(`<w:b/>`, "Name"))) + cell(`<w:gridSpan w:val="2"/>`, p("", run("Values"))) + `</w:tr>` +
		`<w:tr>` + cell(`<w:vMerge w:val="restart"/>`, p("", run("Merged"))) + cell("", right("1")) + cell("", right("2")) + `</w:tr>` +
		`<w:tr>` + cell(`<w:vMerge/>`, p("")) + cell("", right("3")) + cell("", p("")) + `</w:tr>` +
		`<w:tr><w:trPr><w:del w:id="9" w:author="x"/></w:trPr>` + cell("", p("", run("deleted row"))) + `</w:tr>` +
		`<w:tr>` + cell("", p("", run("Nested"))+`<w:tbl><w:tr>`+cell("", p("", run("inner")))+`</w:tr></w:tbl>`) + cell("", right("4")) + cell("", p("", run("x"), `<w:r><w:br/></w:r>`, run("y"))) + `</w:tr>` +
		`</w:tbl>`
	doc, _ := read(t, fixture{
		body: p("Caption", run("Table "), field(run("1"), " SEQ Table \\* ARABIC "), run(": Results of "), runPr(`<w:b/>`, "tests")) + tbl,
	})
	expectDump(t, doc, `Table{"Results of " B("tests")}(head:[Plain("Name")|c2:Plain("Values")] `+
		`body:[r2:Plain("Merged")|Plain("1")|a3:Plain("2")][Plain("3")|Plain()][P("Nested") Table(body:[Plain("inner")])|Plain("4")|Plain("x" BR "y")])`)
	tb := doc.Blocks[0].(*ast.Table)
	if len(tb.Cols) != 3 || tb.Cols[0].Width != 0.5 || tb.Cols[1].Width != 0.25 {
		t.Errorf("cols = %+v", tb.Cols)
	}
	if tb.Cols[1].Align != ast.AlignRight {
		t.Errorf("column 2 alignment = %v", tb.Cols[1].Align)
	}
}

func TestTableHeaderHeuristics(t *testing.T) {
	row := func(cells ...string) string {
		var sb strings.Builder
		sb.WriteString("<w:tr>")
		for _, c := range cells {
			sb.WriteString(`<w:tc>` + c + `</w:tc>`)
		}
		sb.WriteString("</w:tr>")
		return sb.String()
	}
	boldFirst := `<w:tbl>` + row(p("", runPr(`<w:b/>`, "A")), p("", runPr(`<w:b/>`, "B")), p("")) + row(p("", run("1")), p("", run("2")), p("", run("3"))) + `</w:tbl>`
	styled := `<w:tbl><w:tblPr><w:tblStyle w:val="GridTable4"/><w:tblLook w:val="04A0" w:firstRow="1" w:lastRow="0"/></w:tblPr>` +
		row(p("", run("H"))) + row(p("", run("v"))) + `</w:tbl>`
	notHead := `<w:tbl>` + row(p("", runPr(`<w:b/>`, "A")), p("", run("B"))) + row(p("", run("1")), p("", run("2"))) + `</w:tbl>`
	doc, _ := read(t, fixture{body: boldFirst + p("", run("-")) + styled + p("", run("-")) + notHead})
	expectDump(t, doc, strings.Join([]string{
		`Table(head:[Plain("A")|Plain("B")|Plain()] body:[Plain("1")|Plain("2")|Plain("3")])`,
		`P("-")`,
		`Table(head:[Plain("H")] body:[Plain("v")])`,
		`P("-")`,
		`Table(body:[Plain(B("A"))|Plain("B")][Plain("1")|Plain("2")])`,
	}, "\n"))
}

var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")

func drawing(rid, kind string, extra string) string {
	return `<w:r><w:drawing><wp:` + kind + `><wp:extent cx="1828800" cy="914400"/><wp:docPr id="1" name="Picture 1" descr="A cat" title="Cat"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:blipFill>` +
		`<a:blip ` + extra + `r:embed="` + rid + `"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:` + kind + `></w:drawing></w:r>`
}

func TestImages(t *testing.T) {
	doc, warns := read(t, fixture{
		rels: []string{
			relXML("rIdImg", "image", "media/image1.png", false),
			relXML("rIdOdd", "image", "media/blob.bin", false),
			relXML("rIdExt", "image", "https://example.org/pic.jpg", true),
			relXML("rIdLocal", "image", "file:///etc/secret.png", true),
			relXML("rIdVml", "image", "media/image1.png", false),
		},
		files: map[string][]byte{"word/media/image1.png": pngBytes, "word/media/blob.bin": []byte("not an image")},
		body: p("", drawing("rIdImg", "inline", "")) +
			p("Caption", run("Figure "), field(run("1"), " SEQ Figure \\* ARABIC "), run(": A sleepy cat")) +
			p("", run("Inline "), drawing("rIdImg", "inline", ""), run(" icon")) +
			p("", run("Floating"), drawing("rIdImg", "anchor", "")) +
			p("", `<w:r><w:drawing><wp:inline><wp:extent cx="0" cy="0"/><wp:docPr id="2" name="x"/><a:graphic><a:graphicData><pic:pic><pic:blipFill><a:blip r:link="rIdExt"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`) +
			p("", drawing("rIdOdd", "inline", "")) +
			p("", `<w:r><w:drawing><wp:inline><wp:docPr id="3" name="y"/><a:graphic><a:graphicData><pic:pic><pic:blipFill><a:blip r:link="rIdLocal"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`) +
			p("", `<w:r><w:pict><v:shape style="width:72pt;height:1in"><v:imagedata r:id="rIdVml" o:title="Legacy"/></v:shape></w:pict></w:r>`, run(" legacy")),
	})
	expectDump(t, doc, strings.Join([]string{
		`Figure(Img(res:media/image1.png 144ptx72pt alt=A cat);"A sleepy cat")`,
		`P("Inline " Img(res:media/image1.png 144ptx72pt alt=A cat) " icon")`,
		`P("Floating")`,
		`Figure(Img(res:media/image1.png 144ptx72pt alt=A cat);)`,
		`Figure(Img(https://example.org/pic.jpg);)`,
		// Linked local files are passed on; the caller's access rules decide.
		`Figure(Img(file:///etc/secret.png);)`,
		`P(Img(res:media/image1.png 72ptx72pt alt=Legacy) " legacy")`,
	}, "\n"))
	res, ok := doc.Resources.Get("res:media/image1.png")
	if !ok || res.MediaType != "image/png" || !bytes.Equal(res.Data, pngBytes) {
		t.Errorf("resource = %+v ok=%v", res, ok)
	}
	if doc.Resources.Len() != 1 {
		t.Errorf("resources = %v", doc.Resources.Names())
	}
	if !hasWarning(warns, "unknown format") {
		t.Errorf("warnings = %v", warns)
	}
}

func TestFootnotesAndEndnotes(t *testing.T) {
	doc, warns := read(t, fixture{
		footnotes: `<w:footnote w:id="1"><w:p><w:pPr><w:pStyle w:val="FootnoteText"/></w:pPr><w:r><w:rPr><w:rStyle w:val="FootnoteReference"/></w:rPr><w:footnoteRef/></w:r>` +
			`<w:r><w:t xml:space="preserve"> See </w:t></w:r><w:r><w:rPr><w:i/></w:rPr><w:t>the source</w:t></w:r><w:r><w:t>.</w:t></w:r></w:p>` +
			`<w:p><w:r><w:t>Second paragraph.</w:t></w:r></w:p></w:footnote>`,
		endnotes: `<w:endnote w:id="2"><w:p><w:r><w:endnoteRef/></w:r><w:r><w:t>An endnote.</w:t></w:r></w:p></w:endnote>`,
		body: p("", run("Text"), `<w:r><w:rPr><w:rStyle w:val="FootnoteReference"/></w:rPr><w:footnoteReference w:id="1"/></w:r>`,
			run(" and more"), `<w:r><w:rPr><w:vertAlign w:val="superscript"/></w:rPr><w:endnoteReference w:id="2"/></w:r>`,
			`<w:r><w:footnoteReference w:id="99"/></w:r>`),
	})
	expectDump(t, doc, `P("Text" Note[P("See " I("the source") ".") P("Second paragraph.")] " and more" Note[P("An endnote.")])`)
	if !hasWarning(warns, "footnote 99 is missing") {
		t.Errorf("warnings = %v", warns)
	}
}

func TestMath(t *testing.T) {
	frac := `<m:f><m:num><m:r><m:t>a</m:t></m:r></m:num><m:den><m:r><m:t>b</m:t></m:r></m:den></m:f>`
	doc, _ := read(t, fixture{
		body: p("", run("Inline "), `<m:oMath>`+frac+`</m:oMath>`, run(" math.")) +
			p("", `<m:oMath><m:r><m:t>x=1</m:t></m:r></m:oMath>`) +
			p("", run("Before"), `<m:oMathPara><m:oMath>`+frac+`</m:oMath></m:oMathPara>`, run("after")) +
			p("", `<m:oMathPara><m:oMath><m:r><m:t>E=m</m:t></m:r><m:sSup><m:e><m:r><m:t>c</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup></m:oMath></m:oMathPara>`),
	})
	expectDump(t, doc, strings.Join([]string{
		`P("Inline " Math(\frac{a}{b}) " math.")`,
		`MathBlock(x=1)`,
		`P("Before")`,
		`MathBlock(\frac{a}{b})`,
		`P("after")`,
		`MathBlock(E=mc^{2})`,
	}, "\n"))
}

func TestHyperlinksAndBookmarks(t *testing.T) {
	doc, _ := read(t, fixture{
		rels: []string{relXML("rIdLink", "hyperlink", "https://example.org/a?b=1", true), relXML("rIdJs", "hyperlink", "javascript:alert(1)", true)},
		body: p("Heading1", `<w:bookmarkStart w:id="1" w:name="_Ref100"/><w:bookmarkStart w:id="2" w:name="Intro"/>`, run("Introduction"), `<w:bookmarkEnd w:id="2"/><w:bookmarkEnd w:id="1"/>`) +
			p("Heading2", `<w:bookmarkStart w:id="3" w:name="_Toc555"/>`, run("Unreferenced"), `<w:bookmarkEnd w:id="3"/>`) +
			p("",
				`<w:hyperlink r:id="rIdLink" w:history="1"><w:r><w:rPr><w:rStyle w:val="Hyperlink"/></w:rPr><w:t>external</w:t></w:r></w:hyperlink>`, run(", "),
				`<w:hyperlink w:anchor="Intro"><w:r><w:rPr><w:u w:val="single"/><w:b/></w:rPr><w:t>internal</w:t></w:r></w:hyperlink>`, run(", "),
				field(runPr(`<w:rStyle w:val="Hyperlink"/>`, "field link"), ` HYPERLINK "https://example.com/x" `), run(", "),
				field(run("see Introduction"), ` REF _Ref100 \h `), run(", "),
				field(run("anchor field"), ` HYPERLINK \l "_Ref100" `), run(", "),
				`<w:hyperlink r:id="rIdJs"><w:r><w:t>unsafe</w:t></w:r></w:hyperlink>`, run(", "),
				field(run("plain ref"), ` REF _Ref100 \r `)) +
			p("", `<w:bookmarkStart w:id="4" w:name="_Ref200"/>`, run("Anchored paragraph"), `<w:bookmarkEnd w:id="4"/>`) +
			p("", field(run("back"), ` REF _Ref200 \h `)),
	})
	expectDump(t, doc, strings.Join([]string{
		`H1#intro("Introduction")`,
		`H2("Unreferenced")`,
		`P(Link(https://example.org/a?b=1;"external") ", " B(Link(#intro;"internal")) ", " Link(https://example.com/x;"field link") ", " Link(#intro;"see Introduction") ", " Link(#intro;"anchor field") ", unsafe, plain ref")`,
		`P(Span#_ref200() "Anchored paragraph")`,
		`P(Link(#_ref200;"back"))`,
	}, "\n"))
}

func TestTrackChanges(t *testing.T) {
	doc, _ := read(t, fixture{
		body: p("",
			run("Keep "),
			`<w:ins w:id="1" w:author="a"><w:r><w:t xml:space="preserve">inserted </w:t></w:r></w:ins>`,
			`<w:del w:id="2" w:author="a"><w:r><w:delText>deleted </w:delText></w:r></w:del>`,
			`<w:moveFrom w:id="3"><w:r><w:t>moved away </w:t></w:r></w:moveFrom>`,
			`<w:moveTo w:id="4"><w:r><w:t xml:space="preserve">moved here </w:t></w:r></w:moveTo>`,
			`<w:commentRangeStart w:id="5"/>`, run("text"), `<w:commentRangeEnd w:id="5"/><w:r><w:commentReference w:id="5"/></w:r>`,
			`<w:smartTag w:element="x"><w:r><w:t xml:space="preserve"> tagged</w:t></w:r></w:smartTag>`,
			`<w:customXml w:element="y"><w:r><w:t xml:space="preserve"> custom</w:t></w:r></w:customXml>`) +
			`<w:del w:id="6"><w:p><w:r><w:delText>whole paragraph</w:delText></w:r></w:p></w:del>`,
	})
	expectDump(t, doc, `P("Keep inserted moved here text tagged custom")`)
}

func cslInstr(id string, extra string) string {
	return ` ADDIN ZOTERO_ITEM CSL_CITATION {"citationID":"x1","properties":{"formattedCitation":"(Smith 2020)","plainCitation":"(Smith 2020)","noteIndex":0},` +
		`"citationItems":[{"id":` + id + `,"uris":["http://example.org/users/1/items/ABCD1234"],"itemData":{"id":` + id + `,"type":"article-journal",` +
		`"title":"A study of &lt;i&gt;things&lt;/i&gt;","container-title":"Journal of Tests","volume":12,"issue":"3","page":"45-67",` +
		`"DOI":"10.1000/xyz","author":[{"family":"Smith","given":"Jane"},{"family":"Beethoven","given":"Ludwig","non-dropping-particle":"van"},{"literal":"Test Consortium"}],` +
		`"issued":{"date-parts":[["2020","3",15]]}}` + extra + `}],"schema":"https://github.com/citation-style-language/schema/raw/master/csl-citation.json"} `
}

func TestCSLCitationsAndBibliography(t *testing.T) {
	second := ` ADDIN CSL_CITATION {"citationItems":[{"id":"ITEM-1","itemData":{"id":"ITEM-1","type":"book","title":"Other Book",` +
		`"author":[{"family":"Doe","given":"J."}],"issued":{"raw":"2019-05"},"publisher":"Pub"},"uris":["http://example.com/documents/?uuid=1234-abcd"]},` +
		`{"id":"ITEM-2","itemData":{"type":"webpage","title":"Site","issued":{"literal":"n.d."}}}]} `
	doc, _ := read(t, fixture{
		body: p("",
			run("As shown "),
			// Instruction split across runs, as word processors write long JSON.
			field(run("(Smith 2020)"), cslInstr("42", `,"locator":"12","label":"page","prefix":"see","suppress-author":true`)[:120], cslInstr("42", `,"locator":"12","label":"page","prefix":"see","suppress-author":true`)[120:]),
			run(" and again "),
			field(run("[1]"), cslInstr("42", "")),
			run(" and "),
			field(run("(Doe 2019; Site n.d.)"), second),
			run(".")) +
			p("Heading1", run("References")) +
			// The bibliography field spans paragraphs: begin here, end below.
			p("", `<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> ADDIN ZOTERO_BIBL {"uncited":[],"omitted":[],"custom":[]} CSL_BIBLIOGRAPHY </w:instrText></w:r>`+
				`<w:r><w:fldChar w:fldCharType="separate"/></w:r>`, run("Smith, J. (2020). A study of things.")) +
			p("", run("Doe, J. (2019). Other Book."), `<w:r><w:fldChar w:fldCharType="end"/></w:r>`) +
			p("", run("After bibliography.")),
	})
	_ = doc
	expectDump(t, doc, strings.Join([]string{
		`P("As shown " Cite(-see 42@page:12;"(Smith 2020)") " and again " Cite(42;"[1]") " and " Cite(1234-abcd,site;"(Doe 2019; Site n.d.)") ".")`,
		`Bibliography`,
		`P("After bibliography.")`,
	}, "\n"))
	if doc.Meta.ReferenceSectionTitle != "References" {
		t.Errorf("ReferenceSectionTitle = %q", doc.Meta.ReferenceSectionTitle)
	}
	if len(doc.References) != 3 {
		t.Fatalf("references = %+v", doc.References)
	}
	r := doc.References[0]
	if r.ID != "42" || r.Type != "article-journal" || r.Title != "A study of things" || r.ContainerTitle != "Journal of Tests" ||
		r.Volume != "12" || r.Issue != "3" || r.Page != "45-67" || r.DOI != "10.1000/xyz" {
		t.Errorf("reference = %+v", r)
	}
	if r.Issued != (ast.Date{Year: 2020, Month: 3, Day: 15}) {
		t.Errorf("issued = %+v", r.Issued)
	}
	wantNames := []ast.Name{{Family: "Smith", Given: "Jane"}, {Family: "Beethoven", Given: "Ludwig", Particle: "van"}, {Literal: "Test Consortium"}}
	if len(r.Author) != 3 || r.Author[0] != wantNames[0] || r.Author[1] != wantNames[1] || r.Author[2] != wantNames[2] {
		t.Errorf("authors = %+v", r.Author)
	}
	if b := doc.References[1]; b.ID != "1234-abcd" || b.Type != "book" || b.Issued != (ast.Date{Year: 2019, Month: 5}) || b.Publisher != "Pub" {
		t.Errorf("second reference = %+v", b)
	}
	if c := doc.References[2]; c.ID != "site" || c.Issued.Literal != "n.d." || c.Type != "webpage" {
		t.Errorf("third reference = %+v", c)
	}
}

const sourcesXML = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>` +
	`<b:Sources SelectedStyle="\APA.XSL" xmlns:b="http://schemas.openxmlformats.org/officeDocument/2006/bibliography" xmlns="http://schemas.openxmlformats.org/officeDocument/2006/bibliography">` +
	`<b:Source><b:Tag>Ber21</b:Tag><b:SourceType>Book</b:SourceType><b:Title>Latvijas vēsture</b:Title><b:Year>2021</b:Year><b:City>Rīga</b:City><b:Publisher>Zvaigzne</b:Publisher>` +
	`<b:Author><b:Author><b:NameList><b:Person><b:Last>Bērziņa</b:Last><b:First>Anna</b:First><b:Middle>M.</b:Middle></b:Person></b:NameList></b:Author>` +
	`<b:Editor><b:NameList><b:Person><b:Last>Kalniņš</b:Last><b:First>Jānis</b:First></b:Person></b:NameList></b:Editor></b:Author><b:StandardNumber>978-9934-0-1234-5</b:StandardNumber></b:Source>` +
	`<b:Source><b:Tag>WHO20</b:Tag><b:SourceType>JournalArticle</b:SourceType><b:Title>Global report</b:Title><b:JournalName>Health Journal</b:JournalName>` +
	`<b:Year>2020</b:Year><b:Month>March</b:Month><b:Day>4</b:Day><b:Pages>1-9</b:Pages><b:Volume>7</b:Volume><b:Issue>2</b:Issue>` +
	`<b:Author><b:Author><b:Corporate>World Health Organization</b:Corporate></b:Author></b:Author></b:Source>` +
	`</b:Sources>`

func TestWordCitations(t *testing.T) {
	doc, warns := read(t, fixture{
		rels:  []string{relXML("rIdCx", "customXml", "../customXml/item1.xml", false)},
		files: map[string][]byte{"customXml/item1.xml": []byte(sourcesXML)},
		body: p("",
			run("Known "),
			`<w:sdt><w:sdtPr><w:id w:val="1"/><w:citation/></w:sdtPr><w:sdtContent>`+
				field(run("(Bērziņa, 2021, p. 23)"), ` CITATION Ber21 \p 23 \l 1062 `)+`</w:sdtContent></w:sdt>`,
			run(", multiple "),
			`<w:fldSimple w:instr=" CITATION WHO20 \l 1033 \m Ber21 \n"><w:r><w:t>(World Health Organization, 2020; 2021)</w:t></w:r></w:fldSimple>`,
			run(", unknown "),
			field(run("(Nobody)"), ` CITATION Nob99 \l 1033 `)) +
			`<w:sdt><w:sdtPr><w:docPartObj><w:docPartGallery w:val="Bibliographies"/><w:docPartUnique/></w:docPartObj></w:sdtPr><w:sdtContent>` +
			p("Heading1", run("Bibliography")) +
			p("", field(run("Bērziņa, A. M. (2021). Latvijas vēsture."), ` BIBLIOGRAPHY `)) +
			`</w:sdtContent></w:sdt>`,
	})
	expectDump(t, doc, strings.Join([]string{
		`P("Known " Cite(Ber21@:23;"(Bērziņa, 2021, p. 23)") ", multiple " Cite(WHO20,-Ber21;"(World Health Organization, 2020; 2021)") ", unknown " Cite(Nob99;"(Nobody)"))`,
		`Bibliography`,
	}, "\n"))
	if doc.Meta.ReferenceSectionTitle != "Bibliography" {
		t.Errorf("ReferenceSectionTitle = %q", doc.Meta.ReferenceSectionTitle)
	}
	if len(doc.References) != 2 {
		t.Fatalf("references = %+v", doc.References)
	}
	b := doc.References[0]
	if b.ID != "Ber21" || b.Type != "book" || b.Title != "Latvijas vēsture" || b.Publisher != "Zvaigzne" || b.PublisherPlace != "Rīga" ||
		b.ISBN != "978-9934-0-1234-5" || b.Issued.Year != 2021 {
		t.Errorf("book = %+v", b)
	}
	if len(b.Author) != 1 || b.Author[0] != (ast.Name{Family: "Bērziņa", Given: "Anna M."}) || len(b.Editor) != 1 || b.Editor[0].Family != "Kalniņš" {
		t.Errorf("book names = %+v / %+v", b.Author, b.Editor)
	}
	w := doc.References[1]
	if w.ID != "WHO20" || w.Type != "article-journal" || w.ContainerTitle != "Health Journal" || w.Issued != (ast.Date{Year: 2020, Month: 3, Day: 4}) ||
		len(w.Author) != 1 || w.Author[0].Literal != "World Health Organization" || w.Page != "1-9" {
		t.Errorf("article = %+v", w)
	}
	if !hasWarning(warns, "Nob99") {
		t.Errorf("warnings = %v", warns)
	}
}

func TestRecordCitation(t *testing.T) {
	instr := ` ADDIN EN.CITE &lt;EndNote&gt;&lt;Cite&gt;&lt;Author&gt;Smith&lt;/Author&gt;&lt;Year&gt;2018&lt;/Year&gt;&lt;RecNum&gt;17&lt;/RecNum&gt;` +
		`&lt;Pages&gt;5&lt;/Pages&gt;&lt;DisplayText&gt;(Smith, 2018)&lt;/DisplayText&gt;&lt;record&gt;&lt;rec-number&gt;17&lt;/rec-number&gt;` +
		`&lt;ref-type name="Journal Article"&gt;17&lt;/ref-type&gt;&lt;contributors&gt;&lt;authors&gt;&lt;author&gt;&lt;style face="normal"&gt;Smith, John&lt;/style&gt;&lt;/author&gt;` +
		`&lt;author&gt;Acme Institute,&lt;/author&gt;&lt;/authors&gt;&lt;/contributors&gt;&lt;titles&gt;&lt;title&gt;Record title&lt;/title&gt;` +
		`&lt;secondary-title&gt;Some Journal&lt;/secondary-title&gt;&lt;/titles&gt;&lt;volume&gt;4&lt;/volume&gt;&lt;dates&gt;&lt;year&gt;2018&lt;/year&gt;&lt;/dates&gt;` +
		`&lt;/record&gt;&lt;/Cite&gt;&lt;/EndNote&gt; `
	doc, _ := read(t, fixture{body: p("", run("See "), field(run("(Smith, 2018)"), instr), run("."))})
	expectDump(t, doc, `P("See " Cite(rec-17@:5;"(Smith, 2018)") ".")`)
	if len(doc.References) != 1 {
		t.Fatalf("references = %+v", doc.References)
	}
	r := doc.References[0]
	if r.Type != "article-journal" || r.Title != "Record title" || r.ContainerTitle != "Some Journal" || r.Issued.Year != 2018 || r.Volume != "4" ||
		len(r.Author) != 2 || r.Author[0] != (ast.Name{Family: "Smith", Given: "John"}) || r.Author[1].Literal != "Acme Institute" {
		t.Errorf("reference = %+v", r)
	}
}

func TestCodeAndQuotes(t *testing.T) {
	mono := `<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/>`
	doc, _ := read(t, fixture{
		body: p("HTMLPreformatted", run("func main() {")) +
			p("HTMLPreformatted", run("\tfmt.Println(\"hi\")   ")) +
			p("HTMLPreformatted") +
			p("HTMLPreformatted", run("}")) +
			p("", run("Between.")) +
			p("", runPr(mono, "$ go test"), runPr(mono, " ./...")) +
			p("", runPr(mono, "$ go vet")) +
			p("Quote", run("To be or not to be.")) +
			p("IntenseQuote", run("That is the question.")) +
			p("", run("Done.")),
	})
	expectDump(t, doc, strings.Join([]string{
		`Code("func main() {\n\tfmt.Println(\"hi\")\n\n}")`,
		`P("Between.")`,
		`Code("$ go test ./...\n$ go vet")`,
		`Quote[P("To be or not to be.") P("That is the question.")]`,
		`P("Done.")`,
	}, "\n"))
}

func TestFields(t *testing.T) {
	doc, _ := read(t, fixture{
		body: `<w:sdt><w:sdtPr><w:docPartObj><w:docPartGallery w:val="Table of Contents"/></w:docPartObj></w:sdtPr><w:sdtContent>` +
			p("TOCHeading", run("Contents")) + p("TOC1", field(run("Intro\t1"), ` TOC \o "1-3" \h \z \u `)) + `</w:sdtContent></w:sdt>` +
			// A bare TOC field spanning several paragraphs.
			p("", `<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> TOC \o "1-3" </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r>`, run("Entry one")) +
			p("", run("Entry two"), `<w:r><w:fldChar w:fldCharType="end"/></w:r>`) +
			p("", run("Page "), field(run("3"), ` PAGE `), run(" of "), `<w:fldSimple w:instr=" NUMPAGES "><w:r><w:t>9</w:t></w:r></w:fldSimple>`) +
			p("", run("Equation "), field(run("(1)"), ` SEQ Equation \* ARABIC `), run(" holds.")) +
			p("", run("Index"), field("", ` XE "term" `), run(" entry.")) +
			p("", run("Box "), `<w:r><w:fldChar w:fldCharType="begin"><w:ffData><w:checkBox><w:default w:val="1"/></w:checkBox></w:ffData></w:fldChar></w:r><w:r><w:instrText> FORMCHECKBOX </w:instrText></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>`) +
			p("", `<w:sdt><w:sdtPr><w:showingPlcHdr/></w:sdtPr><w:sdtContent><w:r><w:t>Click or tap here to enter text.</w:t></w:r></w:sdtContent></w:sdt>`) +
			p("", `<w:sdt><w:sdtPr><w:alias w:val="Name"/></w:sdtPr><w:sdtContent><w:r><w:t>Filled in</w:t></w:r></w:sdtContent></w:sdt>`),
	})
	expectDump(t, doc, strings.Join([]string{
		`P("Page 3 of 9")`,
		`P("Equation (1) holds.")`,
		`P("Index entry.")`,
		`P("Box ☒")`,
		`P("Filled in")`,
	}, "\n"))
}

func TestTextBoxesAndAlternateContent(t *testing.T) {
	box := `<w:txbxContent>` + p("", run("Boxed text")) + `</w:txbxContent>`
	doc, _ := read(t, fixture{
		body: p("", run("Anchor paragraph"),
			`<w:r><mc:AlternateContent><mc:Choice Requires="wps"><w:drawing><wp:anchor><wp:docPr id="5" name="Text Box 5"/><a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"><wps:wsp><wps:txbx>`+box+`</wps:txbx></wps:wsp></a:graphicData></a:graphic></wp:anchor></w:drawing></mc:Choice>`+
				`<mc:Fallback><w:pict><v:shape><v:textbox>`+box+`</v:textbox></v:shape></w:pict></mc:Fallback></mc:AlternateContent></w:r>`) +
			p("", `<w:r><mc:AlternateContent><mc:Choice Requires="w16se"><w16se:symEx xmlns:w16se="http://schemas.microsoft.com/office/word/2015/wordml/symex" w16se:font="Segoe UI Emoji" w16se:char="1F600"/></mc:Choice><mc:Fallback><w:t>😀</w:t></mc:Fallback></mc:AlternateContent></w:r>`) +
			p("", `<w:r><w:pict><v:rect o:hr="t" style="width:0;height:1.5pt"/></w:pict></w:r>`) +
			pPr(`<w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="auto"/></w:pBdr>`),
	})
	expectDump(t, doc, strings.Join([]string{
		`P("Anchor paragraph")`,
		`P("Boxed text")`,
		`P("😀")`,
		`HR`,
		`HR`,
	}, "\n"))
}

func TestPageBreaks(t *testing.T) {
	doc, _ := read(t, fixture{
		styles: `<w:style w:type="paragraph" w:styleId="Chapter"><w:name w:val="Chapter"/><w:basedOn w:val="Heading1"/><w:pPr><w:pageBreakBefore/></w:pPr></w:style>`,
		body: p("Chapter", run("First")) +
			p("", run("Before"), `<w:r><w:br w:type="page"/></w:r>`, run("After")) +
			p("", `<w:r><w:br w:type="page"/></w:r>`) +
			p("", `<w:r><w:lastRenderedPageBreak/><w:t>Rendered</w:t></w:r>`) +
			pPr(`<w:sectPr><w:type w:val="nextPage"/></w:sectPr>`, run("End of section")) +
			pPr(`<w:sectPr><w:type w:val="continuous"/></w:sectPr>`, run("Continuous")) +
			p("Chapter", run("Second")) +
			p("", `<w:r><w:br w:type="page"/></w:r>`),
	})
	expectDump(t, doc, strings.Join([]string{
		`H1("First")`,
		`P("Before")`,
		`PageBreak`,
		`P("After")`,
		`PageBreak`,
		`P("Rendered")`,
		`P("End of section")`,
		`PageBreak`,
		`P("Continuous")`,
		`PageBreak`,
		`H1("Second")`,
	}, "\n"))
}

func TestFakeHeadings(t *testing.T) {
	big := `<w:b/><w:sz w:val="36"/>`
	doc, _ := read(t, fixture{
		body: p("", runPr(big, "Project overview")) +
			p("", run("This document has no heading styles at all, only direct formatting.")) +
			p("", runPr(`<w:b/><w:sz w:val="28"/>`, "Background")) +
			p("", run("Some background text that is long enough to be body text.")) +
			p("", runPr(`<w:b/>`, "Details")) +
			p("", run("More body text follows here, with a final period.")) +
			p("", runPr(`<w:b/>`, "Not a heading.")) +
			p("", run("Even more body text to keep the ratio of headings low.")),
	})
	expectDump(t, doc, strings.Join([]string{
		`H1("Project overview")`,
		`P("This document has no heading styles at all, only direct formatting.")`,
		`H2("Background")`,
		`P("Some background text that is long enough to be body text.")`,
		`H3("Details")`,
		`P("More body text follows here, with a final period.")`,
		`P(B("Not a heading."))`,
		`P("Even more body text to keep the ratio of headings low.")`,
	}, "\n"))
}

func TestChartAsTable(t *testing.T) {
	chart := `<?xml version="1.0" encoding="UTF-8"?><c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<c:chart><c:title><c:tx><c:rich><a:p><a:r><a:t>Sales</a:t></a:r></a:p></c:rich></c:tx></c:title><c:plotArea><c:barChart>` +
		`<c:ser><c:tx><c:strRef><c:strCache><c:pt idx="0"><c:v>2024</c:v></c:pt></c:strCache></c:strRef></c:tx>` +
		`<c:cat><c:strRef><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt></c:strCache></c:strRef></c:cat>` +
		`<c:val><c:numRef><c:numCache><c:pt idx="0"><c:v>1.1000000000000001</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>` +
		`</c:barChart></c:plotArea></c:chart></c:chartSpace>`
	doc, warns := read(t, fixture{
		rels:  []string{relXML("rIdChart", "chart", "charts/chart1.xml", false)},
		files: map[string][]byte{"word/charts/chart1.xml": []byte(chart)},
		body:  p("", `<w:r><w:drawing><wp:inline><wp:extent cx="100" cy="100"/><wp:docPr id="1" name="Chart 1"/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rIdChart"/></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`),
	})
	expectDump(t, doc, `Table{"Sales"}(head:[Plain()|Plain("2024")] body:[Plain("Q1")|Plain("1.1")][Plain("Q2")|Plain("2")])`)
	if !hasWarning(warns, "chart") {
		t.Errorf("warnings = %v", warns)
	}
}

func TestStrictNamespaces(t *testing.T) {
	strict := `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://purl.oclc.org/ooxml/wordprocessingml/main"><w:body>` +
		`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Strict</w:t></w:r></w:p><w:p><w:r><w:rPr><w:b/></w:rPr><w:t>bold</w:t></w:r></w:p></w:body></w:document>`
	data := zipFiles(t, map[string][]byte{
		"_rels/.rels":       []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r1" Type="http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument" Target="/word/document.xml"/></Relationships>`),
		"word/document.xml": []byte(strict),
	})
	doc, _, err := Read(context.Background(), data, rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	expectDump(t, doc, "H1(\"Strict\")\nP(B(\"bold\"))")
}

func TestUTF16CustomXML(t *testing.T) {
	// Some producers store bibliography sources as UTF-16 with a BOM.
	src := `<?xml version="1.0" encoding="UTF-16"?>` + sourcesXML[strings.Index(sourcesXML, "<b:Sources"):]
	utf16 := []byte{0xFF, 0xFE}
	for _, r := range src {
		if r > 0xFFFF {
			t.Fatal("test data outside the BMP")
		}
		utf16 = append(utf16, byte(r), byte(r>>8))
	}
	doc, _ := read(t, fixture{
		files: map[string][]byte{"customXml/item2.xml": utf16},
		body:  p("", field(run("(WHO)"), ` CITATION WHO20 `)),
	})
	if len(doc.References) != 1 || doc.References[0].Title != "Global report" {
		t.Errorf("references = %+v", doc.References)
	}
}

func TestErrors(t *testing.T) {
	ctx := context.Background()
	if _, _, err := Read(ctx, []byte("definitely not a zip"), rd.Options{}); err == nil {
		t.Error("expected error for non-zip input")
	}
	noDoc := zipFiles(t, map[string][]byte{"hello.txt": []byte("hi")})
	if _, _, err := Read(ctx, noDoc, rd.Options{}); err == nil || !strings.Contains(err.Error(), "word/document.xml") {
		t.Errorf("missing document error = %v", err)
	}
	good := fixture{body: p("", run(strings.Repeat("content ", 200)))}.build(t)
	for _, cut := range []int{len(good) / 3, len(good) / 2, len(good) - 30} {
		if _, _, err := Read(ctx, good[:cut], rd.Options{}); err == nil {
			t.Errorf("truncated archive (%d bytes) read without error", cut)
		}
	}
	// Corrupt the compressed document stream inside an otherwise valid zip.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("word/document.xml")
	_, _ = w.Write([]byte(`<w:document ` + nsAll + `><w:body>` + strings.Repeat(p("", run("text")), 100) + `</w:body></w:document>`))
	_ = zw.Close()
	corrupt := buf.Bytes()
	for i := 60; i < 120 && i < len(corrupt); i++ {
		corrupt[i] ^= 0xA5
	}
	if _, _, err := Read(ctx, corrupt, rd.Options{}); err == nil {
		t.Error("corrupt deflate stream read without error")
	}
	for _, bad := range []string{"this is not XML", `<w:document ` + nsAll + `><w:body`, `<w:document ` + nsAll + `/>`} {
		data := zipFiles(t, map[string][]byte{"word/document.xml": []byte(bad)})
		if _, _, err := Read(ctx, data, rd.Options{}); err == nil {
			t.Errorf("document.xml %q read without error", bad)
		}
	}
	limited := fixture{body: p("", run(strings.Repeat("big ", 5000)))}.build(t)
	if _, _, err := Read(ctx, limited, rd.Options{Limits: rd.Limits{MaxEntry: 1024}}); !errors.Is(err, rd.ErrLimit) {
		t.Errorf("limit error = %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data := fixture{body: strings.Repeat(p("", run("paragraph")), 50)}.build(t)
	if _, _, err := Read(ctx, data, rd.Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestTruncatedBodyKeepsContent(t *testing.T) {
	doc := `<w:document ` + nsAll + `><w:body>` + p("", run("first")) + p("", run("second")) + `<w:p><w:r><w:t>cut`
	data := zipFiles(t, map[string][]byte{"word/document.xml": []byte(doc)})
	got, warns, err := Read(context.Background(), data, rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Text up to the cut is kept; the reader warns about the truncation.
	if dump(got.Blocks) != "P(\"first\")\nP(\"second\")\nP(\"cut\")" || !hasWarning(warns, "truncated") {
		t.Errorf("blocks = %s, warnings = %v", dump(got.Blocks), warns)
	}
}

func TestHostileInputDoesNotPanic(t *testing.T) {
	deep := strings.Repeat(`<w:sdt><w:sdtContent>`, 3000) + p("", run("deep")) + strings.Repeat(`</w:sdtContent></w:sdt>`, 3000)
	cyclic := `<w:style w:type="paragraph" w:styleId="A"><w:name w:val="A"/><w:basedOn w:val="B"/></w:style>` +
		`<w:style w:type="paragraph" w:styleId="B"><w:name w:val="B"/><w:basedOn w:val="A"/></w:style>`
	selfNote := `<w:footnote w:id="1"><w:p><w:r><w:t>loop</w:t></w:r><w:r><w:footnoteReference w:id="1"/></w:r></w:p></w:footnote>`
	doc, _ := read(t, fixture{
		styles:    cyclic,
		footnotes: selfNote,
		numbering: `<w:abstractNum w:abstractNumId="1"><w:numStyleLink w:val="L"/></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="1"/></w:num>`,
		body: deep + p("A", run("cyclic style")) + p("", run("x"), `<w:r><w:footnoteReference w:id="1"/></w:r>`) +
			listPara("1", "12", "bad level") + listPara("99", "0", "unknown num") +
			p("", `<w:r><w:fldChar w:fldCharType="end"/></w:r>`, run("stray end")) +
			p("", `<w:r><w:fldChar w:fldCharType="begin"/></w:r>`, run("never closed")),
	})
	if len(doc.Blocks) == 0 {
		t.Fatal("no blocks")
	}
}

func TestCaptionLabel(t *testing.T) {
	tests := []struct {
		in   string
		kind capKind
		rest string
	}{
		{"Table 1: Results", capTable, "Results"},
		{"Table : Results", capTable, "Results"},
		{"Tabula 2. Rezultāti", capTable, "Rezultāti"},
		{"Tableau 3 – Données", capTable, "Données"},
		{"Figure 1-2: Overview", capFigure, "Overview"},
		{"Fig. 4. A plot", capFigure, "A plot"},
		{"1. attēls. Shēma", capFigure, "Shēma"},
		{"2.3. tabula – Dati", capTable, "Dati"},
		{"Abbildung 5: Aufbau", capFigure, "Aufbau"},
		{"Figure shows the trend", capNone, ""},
		{"Tables are useful", capNone, ""},
		{"Results", capNone, ""},
	}
	for _, tt := range tests {
		kind, n := captionLabel(tt.in)
		if kind != tt.kind {
			t.Errorf("captionLabel(%q) kind = %v, want %v", tt.in, kind, tt.kind)
			continue
		}
		if n > 0 && tt.in[n:] != tt.rest {
			t.Errorf("captionLabel(%q) rest = %q, want %q", tt.in, tt.in[n:], tt.rest)
		}
	}
}

func TestSanitizeID(t *testing.T) {
	tests := map[string]string{
		"Intro":           "intro",
		"_Ref123456":      "_ref123456",
		"Metodes un rīki": "metodes-un-riki",
		"Ūdens/Ķīmija":    "udens-kimija",
		"Größe & Maß":     "grose-mas",
		"a..b:c":          "a..b:c",
		"日本":              "",
		"  trailing  -- ": "trailing",
	}
	for in, want := range tests {
		if got := sanitizeID(in); got != want {
			t.Errorf("sanitizeID(%q) = %q, want %q", in, sanitizeID(in), want)
		}
	}
}

func TestParseInstr(t *testing.T) {
	in := parseInstr(` HYPERLINK "https://x.org/a b" \l "anchor one" \o "tip" `)
	if in.typ != "HYPERLINK" || in.switchArg(`\l`) != "anchor one" || strings.Join(in.args(), "|") != "https://x.org/a b" {
		t.Errorf("hyperlink instr = %+v args=%v", in, in.args())
	}
	in = parseInstr(` REF _Ref12 \h \r `)
	if in.typ != "REF" || !in.hasSwitch(`\h`) || strings.Join(in.args(), "|") != "_Ref12" {
		t.Errorf("ref instr = %+v", in)
	}
	in = parseInstr(` SEQ Figure \* ARABIC `)
	if strings.Join(in.args(), "|") != "Figure" {
		t.Errorf("seq args = %v", in.args())
	}
}

func TestSafeURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://example.org":    "https://example.org",
		"mailto:a@b.c":           "mailto:a@b.c",
		"javascript:alert(1)":    "",
		"JavaScript:alert(1)":    "",
		"data:text/html,x":       "",
		"docs/readme.html":       "docs/readme.html",
		"C:\\Users\\file.docx":   "C:\\Users\\file.docx",
		"https://x.org/\x00evil": "",
	} {
		if got := safeURL(in); got != want {
			t.Errorf("safeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCSLDates(t *testing.T) {
	tests := map[string]ast.Date{
		`{"date-parts":[[2020,1,2]]}`:          {Year: 2020, Month: 1, Day: 2},
		`{"date-parts":[["2021"]]}`:            {Year: 2021},
		`{"date-parts":[["2019-07-04"]]}`:      {Year: 2019, Month: 7, Day: 4},
		`{"date-parts":[[2020,13]]}`:           {Year: 2020},
		`{"raw":"04.05.2018"}`:                 {Year: 2018, Month: 5, Day: 4},
		`{"literal":"forthcoming"}`:            {Literal: "forthcoming"},
		`{"date-parts":[[1999]],"circa":true}`: {Year: 1999, Circa: true},
		`"2017-02"`:                            {Year: 2017, Month: 2},
		`{"year":2016,"month":"8"}`:            {Year: 2016, Month: 8},
	}
	for in, want := range tests {
		if got := cslDate([]byte(in)); got != want {
			t.Errorf("cslDate(%s) = %+v, want %+v", in, got, want)
		}
	}
}

func TestCleanMarkup(t *testing.T) {
	if got := cleanMarkup(`The <i>E. coli</i> &amp; <span class="nocase">pH</span>`); got != "The E. coli & pH" {
		t.Errorf("cleanMarkup = %q", got)
	}
}

func TestListItemWithDisplayMath(t *testing.T) {
	doc, _ := read(t, fixture{
		numbering: listNumbering,
		body: pPr(`<w:pStyle w:val="ListParagraph"/>`+numPr("1", "0"), run("Solve"),
			`<m:oMathPara><m:oMath><m:r><m:t>x=1</m:t></m:r></m:oMath></m:oMathPara>`, run("where x is real.")) +
			listPara("1", "0", "Next"),
	})
	expectDump(t, doc, `OL(1,0)loose[Item[P("Solve") MathBlock(x=1) P("where x is real.")] Item[P("Next")]]`)
}

func TestOutlineNumberedHeadings(t *testing.T) {
	// One multi-level list numbers both the headings (level 0) and the
	// body paragraphs below them (level 1), which restart under each heading.
	doc, _ := read(t, fixture{
		styles: `<w:style w:type="paragraph" w:styleId="NumHeading"><w:name w:val="heading 1"/><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr><w:outlineLvl w:val="0"/></w:pPr></w:style>`,
		numbering: `<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:pStyle w:val="NumHeading"/></w:lvl>` +
			`<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="decimal"/></w:lvl></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`,
		body: p("NumHeading", run("Scope")) + listPara("1", "1", "a") + listPara("1", "1", "b") +
			p("NumHeading", run("Terms")) + listPara("1", "1", "c"),
	})
	expectDump(t, doc, strings.Join([]string{
		`H1("Scope")`,
		`OL(1,0)[Item[Plain("a")] Item[Plain("b")]]`,
		`H1("Terms")`,
		`OL(1,0)[Item[Plain("c")]]`,
	}, "\n"))
}

func TestHeadingNameBeatsInheritedBodyLevel(t *testing.T) {
	doc, _ := read(t, fixture{
		styles: `<w:style w:type="paragraph" w:styleId="Base"><w:name w:val="Base Text"/><w:pPr><w:outlineLvl w:val="9"/></w:pPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="H2x"><w:name w:val="heading 2"/><w:basedOn w:val="Base"/></w:style>`,
		body: p("H2x", run("Named heading")) + p("Base", run("Body")),
	})
	expectDump(t, doc, "H2(\"Named heading\")\nP(\"Body\")")
}

func TestFigureBookmarksAndInlineCaptions(t *testing.T) {
	doc, _ := read(t, fixture{
		rels:  []string{relXML("rIdImg", "image", "media/photo.jpeg", false)},
		files: map[string][]byte{"word/media/photo.jpeg": {0xFF, 0xD8, 0xFF, 0xE0}},
		body: p("", `<w:bookmarkStart w:id="1" w:name="_Ref77"/>`, drawing("rIdImg", "inline", ""), `<w:bookmarkEnd w:id="1"/>`) +
			p("", run("As "), field(run("the figure"), ` REF _Ref77 \h `), run(" shows.")) +
			p("", drawing("rIdImg", "inline", ""), `<w:r><w:br/></w:r>`, run("Figure 2. Same photo, captioned inline")),
	})
	expectDump(t, doc, strings.Join([]string{
		`Figure#_ref77(Img(res:media/photo.jpeg 144ptx72pt alt=A cat);)`,
		`P("As " Link(#_ref77;"the figure") " shows.")`,
		`Figure(Img(res:media/photo.jpeg 144ptx72pt alt=A cat);"Same photo, captioned inline")`,
	}, "\n"))
}

// richBody exercises most constructs; the sweeps below cut and corrupt it.
func richBody() string {
	frac := `<m:f><m:num><m:r><m:t>a</m:t></m:r></m:num><m:den><m:r><m:t>b</m:t></m:r></m:den></m:f>`
	return p("Title", run("Doc")) +
		p("Heading1", `<w:bookmarkStart w:id="1" w:name="_Ref1"/>`, run("Intro"), `<w:bookmarkEnd w:id="1"/>`) +
		p("", run("Text"), `<w:r><w:footnoteReference w:id="1"/></w:r>`, `<w:hyperlink w:anchor="_Ref1"><w:r><w:t>link</w:t></w:r></w:hyperlink>`,
			field(run("(Smith 2020)"), cslInstr("7", "")), `<m:oMath>`+frac+`</m:oMath>`, field(run("ref"), ` REF _Ref1 \h `)) +
		p("", `<m:oMathPara><m:oMath><m:nary><m:naryPr><m:chr m:val="∑"/></m:naryPr><m:sub/><m:sup/><m:e>`+frac+`</m:e></m:nary></m:oMath></m:oMathPara>`) +
		listPara("1", "0", "One") + listPara("1", "1", "Two") +
		`<w:tbl><w:tblGrid><w:gridCol w:w="1"/><w:gridCol w:w="1"/></w:tblGrid><w:tr><w:tc><w:tcPr><w:vMerge w:val="restart"/><w:gridSpan w:val="2"/></w:tcPr>` + p("", run("c")) + `</w:tc></w:tr>` +
		`<w:tr><w:tc><w:tcPr><w:vMerge/><w:gridSpan w:val="2"/></w:tcPr>` + p("") + `</w:tc></w:tr></w:tbl>` +
		p("", drawing("rIdImg", "inline", "")) + p("Caption", run("Figure 1: x")) +
		p("", `<w:r><mc:AlternateContent><mc:Choice Requires="wps"><w:drawing><wp:anchor><wp:docPr id="5" name="t"/><a:graphic><a:graphicData><wps:wsp><wps:txbx><w:txbxContent>`+p("", run("box"))+`</w:txbxContent></wps:txbx></wps:wsp></a:graphicData></a:graphic></wp:anchor></w:drawing></mc:Choice><mc:Fallback/></mc:AlternateContent></w:r>`) +
		p("", field(run("bib"), ` BIBLIOGRAPHY `))
}

func richFixture(body string) fixture {
	return fixture{
		numbering: listNumbering,
		footnotes: `<w:footnote w:id="1"><w:p><w:r><w:t>note</w:t></w:r></w:p></w:footnote>`,
		rels:      []string{relXML("rIdImg", "image", "media/image1.png", false)},
		files:     map[string][]byte{"word/media/image1.png": pngBytes},
		body:      body,
	}
}

func TestTruncationAndCorruptionSweep(t *testing.T) {
	base := richFixture(richBody())
	data := base.build(t)
	doc, _, err := Read(context.Background(), data, rd.Options{})
	if err != nil || len(doc.Blocks) < 8 {
		t.Fatalf("baseline: %v, %d blocks", err, len(doc.Blocks))
	}
	full := `<?xml version="1.0" encoding="UTF-8"?><w:document ` + nsAll + `><w:body>` + richBody() + `</w:body></w:document>`
	files := map[string][]byte{}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		_, _ = b.ReadFrom(rc)
		rc.Close()
		files[f.Name] = b.Bytes()
	}
	try := func(xml []byte) {
		files["word/document.xml"] = xml
		// Errors are fine; panics and hangs are not.
		_, _, _ = Read(context.Background(), zipFiles(t, files), rd.Options{})
	}
	for cut := 0; cut < len(full); cut += 7 {
		try([]byte(full[:cut]))
	}
	seed := uint32(1)
	next := func() uint32 { seed = seed*1664525 + 1013904223; return seed }
	for i := 0; i < 300; i++ {
		b := []byte(full)
		for k := 0; k < 3; k++ {
			pos := int(next() % uint32(len(b)))
			const alphabet = "<>/\"=&;: \x00wmrpt"
			b[pos] = alphabet[next()%uint32(len(alphabet))]
		}
		try(b)
	}
}

func TestTitleOnlyFromFirstTitleRun(t *testing.T) {
	doc, _ := read(t, fixture{
		body: p("", run("Letterhead line")) + p("Title", run("Main")) + p("Title", run("Title")) + p("Subtitle", run("Sub")) +
			p("", run("Body.")) + p("Title", run("Appendix A")) + p("Subtitle", run("Later subtitle")),
	})
	if doc.Meta.Title != "Main Title" || doc.Meta.Subtitle != "Sub" {
		t.Errorf("title = %q, subtitle = %q", doc.Meta.Title, doc.Meta.Subtitle)
	}
	expectDump(t, doc, "P(\"Letterhead line\")\nP(\"Body.\")\nH1(\"Appendix A\")\nP(\"Later subtitle\")")
}

func TestValidLang(t *testing.T) {
	for tag, want := range map[string]bool{"lv": true, "en-US": true, "sr-Latn-RS": true, "x-none": false, "": false, "en_US": false, "toolongprimary": false} {
		if got := validLang(tag); got != want {
			t.Errorf("validLang(%q) = %v", tag, got)
		}
	}
}

func TestDropCap(t *testing.T) {
	doc, _ := read(t, fixture{
		body: pPr(`<w:framePr w:dropCap="drop" w:lines="3" w:wrap="around" w:vAnchor="text" w:hAnchor="text"/>`, runPr(`<w:sz w:val="96"/>`, "O")) +
			p("", run("nce upon a time.")) + p("", run("Next.")),
	})
	expectDump(t, doc, "P(\"Once upon a time.\")\nP(\"Next.\")")
}
