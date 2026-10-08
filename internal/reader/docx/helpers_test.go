package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

const nsAll = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" ` +
	`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
	`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" ` +
	`xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" ` +
	`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
	`xmlns:v="urn:schemas-microsoft-com:vml" ` +
	`xmlns:o="urn:schemas-microsoft-com:office:office" ` +
	`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" ` +
	`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"`

// fixture assembles a .docx package in memory.
type fixture struct {
	body      string
	styles    string // extra w:style elements (defaults are always present)
	numbering string // w:abstractNum / w:num elements
	footnotes string // w:footnote elements
	endnotes  string
	rels      []string // extra Relationship elements for document.xml
	core      string   // core properties children
	files     map[string][]byte
	docLang   string
}

func (f fixture) build(t testing.TB) []byte {
	t.Helper()
	files := map[string][]byte{}
	put := func(name, s string) { files[name] = []byte(s) }
	put("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`+
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`+
		`<Default Extension="xml" ContentType="application/xml"/>`+
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	put("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>`+
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/></Relationships>`)
	put("word/document.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document `+nsAll+`><w:body>`+f.body+`<w:sectPr/></w:body></w:document>`)
	lang := f.docLang
	if lang == "" {
		lang = "en-US"
	}
	put("word/styles.xml", `<?xml version="1.0" encoding="UTF-8"?><w:styles `+nsAll+`>`+
		`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="22"/><w:lang w:val="`+lang+`"/></w:rPr></w:rPrDefault></w:docDefaults>`+
		defaultStyles+f.styles+`</w:styles>`)
	rels := `<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`
	if f.numbering != "" {
		put("word/numbering.xml", `<?xml version="1.0" encoding="UTF-8"?><w:numbering `+nsAll+`>`+f.numbering+`</w:numbering>`)
		rels += `<Relationship Id="rIdNum" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`
	}
	if f.footnotes != "" {
		put("word/footnotes.xml", `<?xml version="1.0" encoding="UTF-8"?><w:footnotes `+nsAll+`>`+
			`<w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote>`+
			`<w:footnote w:type="continuationSeparator" w:id="0"><w:p><w:r><w:continuationSeparator/></w:r></w:p></w:footnote>`+
			f.footnotes+`</w:footnotes>`)
		rels += `<Relationship Id="rIdFn" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/>`
	}
	if f.endnotes != "" {
		put("word/endnotes.xml", `<?xml version="1.0" encoding="UTF-8"?><w:endnotes `+nsAll+`>`+f.endnotes+`</w:endnotes>`)
		rels += `<Relationship Id="rIdEn" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes" Target="endnotes.xml"/>`
	}
	rels += strings.Join(f.rels, "")
	put("word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+rels+`</Relationships>`)
	if f.core != "" {
		put("docProps/core.xml", `<?xml version="1.0" encoding="UTF-8"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" `+
			`xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">`+f.core+`</cp:coreProperties>`)
	}
	for name, data := range f.files {
		files[name] = data
	}
	return zipFiles(t, files)
}

func zipFiles(t testing.TB, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(files[n]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const defaultStyles = `<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="32"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:pPr><w:outlineLvl w:val="1"/></w:pPr><w:rPr><w:b/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:pPr><w:outlineLvl w:val="2"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:rPr><w:sz w:val="56"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Subtitle"><w:name w:val="Subtitle"/><w:basedOn w:val="Normal"/></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:rPr><w:i/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="IntenseQuote"><w:name w:val="Intense Quote"/><w:basedOn w:val="Normal"/></w:style>` +
	`<w:style w:type="paragraph" w:styleId="Caption"><w:name w:val="caption"/><w:basedOn w:val="Normal"/><w:rPr><w:i/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="HTMLPreformatted"><w:name w:val="HTML Preformatted"/><w:basedOn w:val="Normal"/><w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/></w:rPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="720"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="TOCHeading"><w:name w:val="TOC Heading"/><w:basedOn w:val="Heading1"/><w:pPr><w:outlineLvl w:val="9"/></w:pPr></w:style>` +
	`<w:style w:type="paragraph" w:styleId="TOC1"><w:name w:val="toc 1"/><w:basedOn w:val="Normal"/></w:style>` +
	`<w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/></w:style>` +
	`<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:rPr><w:color w:val="0563C1"/><w:u w:val="single"/></w:rPr></w:style>` +
	`<w:style w:type="character" w:styleId="Strong"><w:name w:val="Strong"/><w:rPr><w:b/></w:rPr></w:style>` +
	`<w:style w:type="character" w:styleId="Emphasis"><w:name w:val="Emphasis"/><w:rPr><w:i/></w:rPr></w:style>` +
	`<w:style w:type="character" w:styleId="VerbatimChar"><w:name w:val="Verbatim Char"/><w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/></w:rPr></w:style>` +
	`<w:style w:type="character" w:styleId="FootnoteReference"><w:name w:val="footnote reference"/><w:rPr><w:vertAlign w:val="superscript"/></w:rPr></w:style>` +
	`<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/></w:style>` +
	`<w:style w:type="table" w:styleId="GridTable4"><w:name w:val="Grid Table 4"/><w:tblStylePr w:type="firstRow"><w:rPr><w:b/><w:bCs/></w:rPr></w:tblStylePr></w:style>`

// Small XML builders.

func p(style string, content ...string) string {
	pr := ""
	if style != "" {
		pr = `<w:pPr><w:pStyle w:val="` + style + `"/></w:pPr>`
	}
	return `<w:p>` + pr + strings.Join(content, "") + `</w:p>`
}

func pPr(ppr string, content ...string) string {
	return `<w:p><w:pPr>` + ppr + `</w:pPr>` + strings.Join(content, "") + `</w:p>`
}

func run(text string) string { return `<w:r><w:t xml:space="preserve">` + text + `</w:t></w:r>` }

func runPr(rpr, text string) string {
	return `<w:r><w:rPr>` + rpr + `</w:rPr><w:t xml:space="preserve">` + text + `</w:t></w:r>`
}

func numPr(numID, ilvl string) string {
	return `<w:numPr><w:ilvl w:val="` + ilvl + `"/><w:numId w:val="` + numID + `"/></w:numPr>`
}

func listPara(numID, ilvl, text string) string {
	return pPr(`<w:pStyle w:val="ListParagraph"/>`+numPr(numID, ilvl), run(text))
}

// field builds a complex field; instr may be split into several runs.
func field(result string, instr ...string) string {
	var sb strings.Builder
	sb.WriteString(`<w:r><w:fldChar w:fldCharType="begin"/></w:r>`)
	for _, in := range instr {
		sb.WriteString(`<w:r><w:instrText xml:space="preserve">` + in + `</w:instrText></w:r>`)
	}
	sb.WriteString(`<w:r><w:fldChar w:fldCharType="separate"/></w:r>`)
	sb.WriteString(result)
	sb.WriteString(`<w:r><w:fldChar w:fldCharType="end"/></w:r>`)
	return sb.String()
}

func relXML(id, typ, target string, external bool) string {
	mode := ""
	if external {
		mode = ` TargetMode="External"`
	}
	return `<Relationship Id="` + id + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/` + typ + `" Target="` + target + `"` + mode + `/>`
}

func read(t testing.TB, f fixture) (*ast.Document, []string) {
	t.Helper()
	doc, warns, err := Read(context.Background(), f.build(t), rd.Options{Name: "test.docx"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc.Resources == nil {
		t.Fatal("Resources not initialised")
	}
	return doc, warns
}

// ---------------------------------------------------------------------------
// AST dump: a compact, readable rendering for assertions.

func dump(blocks []ast.Block) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		parts = append(parts, dumpBlock(b))
	}
	return strings.Join(parts, "\n")
}

func dumpBlock(b ast.Block) string {
	switch n := b.(type) {
	case *ast.Para:
		return "P(" + dumpInlines(n.Inlines) + ")"
	case *ast.Plain:
		return "Plain(" + dumpInlines(n.Inlines) + ")"
	case *ast.Heading:
		id := ""
		if n.Attr.ID != "" {
			id = "#" + n.Attr.ID
		}
		return fmt.Sprintf("H%d%s(%s)", n.Level, id, dumpInlines(n.Inlines))
	case *ast.CodeBlock:
		return fmt.Sprintf("Code(%q)", n.Text)
	case *ast.MathBlock:
		return "MathBlock(" + n.TeX + ")"
	case *ast.BlockQuote:
		return "Quote[" + dumpList(n.Blocks) + "]"
	case *ast.List:
		kind := "UL"
		if n.Ordered {
			kind = fmt.Sprintf("OL(%d,%d)", n.Start, n.Style)
		}
		if !n.Tight {
			kind += "loose"
		}
		var items []string
		for _, it := range n.Items {
			task := ""
			switch it.Task {
			case ast.TaskOpen:
				task = "[ ]"
			case ast.TaskDone:
				task = "[x]"
			}
			items = append(items, "Item"+task+"["+dumpList(it.Blocks)+"]")
		}
		return kind + "[" + strings.Join(items, " ") + "]"
	case *ast.Table:
		var sb strings.Builder
		sb.WriteString("Table")
		if n.Attr.ID != "" {
			sb.WriteString("#" + n.Attr.ID)
		}
		if n.Caption != nil {
			sb.WriteString("{" + dumpInlines(n.Caption) + "}")
		}
		sb.WriteString("(")
		for _, rows := range []struct {
			name string
			rows []ast.Row
		}{{"head", n.Head}, {"body", n.Body}} {
			if len(rows.rows) == 0 {
				continue
			}
			sb.WriteString(rows.name + ":")
			for _, row := range rows.rows {
				sb.WriteString("[")
				for i, c := range row.Cells {
					if i > 0 {
						sb.WriteString("|")
					}
					if c.ColSpan > 1 {
						fmt.Fprintf(&sb, "c%d:", c.ColSpan)
					}
					if c.RowSpan > 1 {
						fmt.Fprintf(&sb, "r%d:", c.RowSpan)
					}
					if c.Align != ast.AlignDefault {
						fmt.Fprintf(&sb, "a%d:", c.Align)
					}
					sb.WriteString(dumpList(c.Blocks))
				}
				sb.WriteString("]")
			}
			sb.WriteString(" ")
		}
		return strings.TrimSpace(sb.String()) + ")"
	case *ast.Figure:
		s := "Figure"
		if n.Attr.ID != "" {
			s += "#" + n.Attr.ID
		}
		return s + "(" + dumpInline(n.Image) + ";" + dumpInlines(n.Caption) + ")"
	case *ast.PageBreak:
		return "PageBreak"
	case *ast.HorizontalRule:
		return "HR"
	case *ast.Bibliography:
		return "Bibliography"
	case *ast.Div:
		return "Div[" + dumpList(n.Blocks) + "]"
	}
	return fmt.Sprintf("%T", b)
}

func dumpList(blocks []ast.Block) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		parts = append(parts, dumpBlock(b))
	}
	return strings.Join(parts, " ")
}

func dumpInlines(ins []ast.Inline) string {
	parts := make([]string, 0, len(ins))
	for _, in := range ins {
		parts = append(parts, dumpInline(in))
	}
	return strings.Join(parts, " ")
}

func dumpInline(in ast.Inline) string {
	wrap := func(name string, kids []ast.Inline) string { return name + "(" + dumpInlines(kids) + ")" }
	switch n := in.(type) {
	case *ast.Text:
		return fmt.Sprintf("%q", n.Value)
	case *ast.Strong:
		return wrap("B", n.Inlines)
	case *ast.Emph:
		return wrap("I", n.Inlines)
	case *ast.Underline:
		return wrap("U", n.Inlines)
	case *ast.Strike:
		return wrap("S", n.Inlines)
	case *ast.Superscript:
		return wrap("Sup", n.Inlines)
	case *ast.Subscript:
		return wrap("Sub", n.Inlines)
	case *ast.SmallCaps:
		return wrap("SC", n.Inlines)
	case *ast.Highlight:
		return wrap("Hl", n.Inlines)
	case *ast.Code:
		return fmt.Sprintf("Code(%q)", n.Text)
	case *ast.Math:
		return "Math(" + n.TeX + ")"
	case *ast.Link:
		return "Link(" + n.URL + ";" + dumpInlines(n.Inlines) + ")"
	case *ast.Image:
		s := "Img(" + n.Src
		if n.Width != "" || n.Height != "" {
			s += " " + n.Width + "x" + n.Height
		}
		if n.Alt != "" {
			s += " alt=" + n.Alt
		}
		return s + ")"
	case *ast.Note:
		return "Note[" + dumpList(n.Blocks) + "]"
	case *ast.Cite:
		var keys []string
		for _, it := range n.Items {
			k := it.Key
			if it.Prefix != "" {
				k = it.Prefix + " " + k
			}
			if it.Locator != "" {
				k += "@" + it.LocatorLabel + ":" + it.Locator
			}
			if it.Suffix != "" {
				k += " " + it.Suffix
			}
			if it.SuppressAuthor {
				k = "-" + k
			}
			keys = append(keys, k)
		}
		return "Cite(" + strings.Join(keys, ",") + ";" + dumpInlines(n.Fallback) + ")"
	case *ast.LineBreak:
		return "BR"
	case *ast.SoftBreak:
		return "SB"
	case *ast.Span:
		return "Span#" + n.Attr.ID + "(" + dumpInlines(n.Inlines) + ")"
	}
	return fmt.Sprintf("%T", in)
}

func expectDump(t *testing.T, doc *ast.Document, want string) {
	t.Helper()
	got := dump(doc.Blocks)
	if got != want {
		t.Errorf("blocks mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func hasWarning(warns []string, substr string) bool {
	for _, w := range warns {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}
