package odt

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/html/asttest"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

const nsDecl = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" xmlns:math="http://www.w3.org/1998/Math/MathML" xmlns:chart="urn:oasis:names:tc:opendocument:xmlns:chart:1.0"`

const pngBytes = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

const stylesXML = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles ` + nsDecl + ` office:version="1.3">
<office:font-face-decls>
 <style:font-face style:name="Liberation Mono" svg:font-family="'Liberation Mono'" style:font-family-generic="modern" style:font-pitch="fixed"/>
 <style:font-face style:name="Liberation Serif" svg:font-family="'Liberation Serif'" style:font-pitch="variable"/>
</office:font-face-decls>
<office:styles>
 <style:default-style style:family="paragraph"><style:text-properties style:font-name="Liberation Serif" fo:language="lv" fo:country="LV"/></style:default-style>
 <style:style style:name="Standard" style:family="paragraph"/>
 <style:style style:name="Text_20_body" style:display-name="Text body" style:family="paragraph" style:parent-style-name="Standard"/>
 <style:style style:name="Heading" style:family="paragraph" style:parent-style-name="Standard"><style:text-properties fo:font-weight="bold"/></style:style>
 <style:style style:name="Heading_20_1" style:display-name="Heading 1" style:family="paragraph" style:parent-style-name="Heading"/>
 <style:style style:name="Title" style:family="paragraph" style:parent-style-name="Heading"><style:text-properties fo:font-weight="bold"/></style:style>
 <style:style style:name="Subtitle" style:family="paragraph" style:parent-style-name="Heading"><style:text-properties fo:font-style="italic"/></style:style>
 <style:style style:name="Quotations" style:family="paragraph" style:parent-style-name="Standard"><style:text-properties fo:font-style="italic"/></style:style>
 <style:style style:name="Preformatted_20_Text" style:display-name="Preformatted Text" style:family="paragraph" style:parent-style-name="Standard"><style:text-properties style:font-name="Liberation Mono"/></style:style>
 <style:style style:name="Table_20_Contents" style:display-name="Table Contents" style:family="paragraph" style:parent-style-name="Standard"/>
 <style:style style:name="Caption" style:family="paragraph" style:parent-style-name="Standard"><style:text-properties fo:font-style="italic"/></style:style>
 <style:style style:name="Illustration" style:family="paragraph" style:parent-style-name="Caption"/>
 <style:style style:name="Table" style:family="paragraph" style:parent-style-name="Caption"/>
 <style:style style:name="Footnote" style:family="paragraph" style:parent-style-name="Standard"/>
 <style:style style:name="Internet_20_link" style:display-name="Internet link" style:family="text"><style:text-properties style:text-underline-style="solid"/></style:style>
 <style:style style:name="Source_20_Text" style:display-name="Source Text" style:family="text"/>
</office:styles>
</office:document-styles>`

const metaXML = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-meta ` + nsDecl + `><office:meta>
 <dc:title>Stale Meta Title</dc:title>
 <meta:initial-creator>Anna Bērziņa</meta:initial-creator>
 <dc:creator>Last Editor</dc:creator>
 <dc:date>2024-03-05T10:11:12.123</dc:date>
 <dc:subject>Testing</dc:subject>
 <dc:description>A test document.</dc:description>
 <meta:keyword>alpha, beta</meta:keyword>
 <meta:keyword>gamma</meta:keyword>
 <dc:language>en-GB</dc:language>
</office:meta></office:document-meta>`

const contentXML = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content ` + nsDecl + ` office:version="1.3">
<office:automatic-styles>
 <style:style style:name="P0" style:family="paragraph" style:parent-style-name="Title" style:master-page-name="Standard"/>
 <style:style style:name="P2" style:family="paragraph" style:parent-style-name="Text_20_body"><style:paragraph-properties fo:break-before="page"/></style:style>
 <style:style style:name="P3" style:family="paragraph" style:parent-style-name="Table_20_Contents"><style:paragraph-properties fo:text-align="end"/></style:style>
 <style:style style:name="T1" style:family="text"><style:text-properties fo:font-weight="bold"/></style:style>
 <style:style style:name="T2" style:family="text"><style:text-properties fo:font-weight="bold" fo:font-style="italic"/></style:style>
 <style:style style:name="T3" style:family="text"><style:text-properties style:text-position="super 58%"/></style:style>
 <style:style style:name="T4" style:family="text"><style:text-properties style:text-position="-33% 58%"/></style:style>
 <style:style style:name="T5" style:family="text"><style:text-properties style:font-name="Liberation Mono"/></style:style>
 <style:style style:name="T6" style:family="text"><style:text-properties style:text-underline-style="solid"/></style:style>
 <style:style style:name="T7" style:family="text"><style:text-properties style:text-line-through-style="solid"/></style:style>
 <style:style style:name="T8" style:family="text"><style:text-properties fo:font-variant="small-caps"/></style:style>
 <style:style style:name="T9" style:family="text"><style:text-properties fo:background-color="#ffff00"/></style:style>
 <text:list-style style:name="L1"><text:list-level-style-number text:level="1" style:num-format="1"/><text:list-level-style-number text:level="2" style:num-format="a"/></text:list-style>
 <text:list-style style:name="L2"><text:list-level-style-bullet text:level="1" text:bullet-char="•"/><text:list-level-style-bullet text:level="2" text:bullet-char="◦"/></text:list-style>
 <text:list-style style:name="L3"><text:list-level-style-number text:level="1" style:num-format="I" text:start-value="3"/></text:list-style>
</office:automatic-styles>
<office:body><office:text>
 <text:tracked-changes><text:changed-region text:id="ct2"><text:deletion><office:change-info><dc:creator>X</dc:creator></office:change-info><text:p>DELETED TEXT</text:p></text:deletion></text:changed-region></text:tracked-changes>
 <text:sequence-decls><text:sequence-decl text:display-outline-level="0" text:name="Illustration"/></text:sequence-decls>
 <text:p text:style-name="P0">Report <text:span text:style-name="T1">Title</text:span></text:p>
 <text:p text:style-name="Subtitle">A subtitle</text:p>
 <text:table-of-content text:name="Table of Contents1"><text:index-body><text:p><text:a xlink:href="#__RefHeading___Toc1_1">Contents entry</text:a></text:p></text:index-body></text:table-of-content>
 <text:h text:style-name="Heading_20_1" text:outline-level="1"><text:bookmark text:name="intro"/><text:bookmark-start text:name="__RefHeading___Toc1_1"/>Introduction<text:bookmark-end text:name="__RefHeading___Toc1_1"/></text:h>
 <text:p text:style-name="Text_20_body">Plain <text:span text:style-name="T1">bo</text:span><text:span text:style-name="T1">ld</text:span> <text:span text:style-name="T2">both</text:span>  and x<text:span text:style-name="T3">2</text:span> H<text:span text:style-name="T4">2</text:span>O <text:span text:style-name="T5">code()</text:span> <text:span text:style-name="T6">under</text:span> <text:span text:style-name="T7">strike</text:span> <text:span text:style-name="T8">caps</text:span> <text:span text:style-name="T9">mark</text:span>.<text:s text:c="3"/>Spaces<text:tab/>tab<text:line-break/>next line<text:soft-page-break/> continues.</text:p>
 <text:p text:style-name="Text_20_body">See <text:a xlink:type="simple" xlink:href="#intro">the intro</text:a>, <text:a xlink:href="https://example.test/"><text:span text:style-name="Internet_20_link">a site</text:span></text:a> and a note<text:note text:id="ftn1" text:note-class="footnote"><text:note-citation>1</text:note-citation><text:note-body><text:p text:style-name="Footnote">Footnote <text:span text:style-name="T1">text</text:span>.</text:p></text:note-body></text:note>.<office:annotation><dc:creator>Rev</dc:creator><text:p>Comment text</text:p></office:annotation></text:p>
 <text:p text:style-name="Text_20_body">Tracked <text:change-start text:change-id="ct1"/>inserted<text:change-end text:change-id="ct1"/> text<text:change text:change-id="ct2"/>.</text:p>
 <text:p text:style-name="Quotations">Quoted one.</text:p>
 <text:p text:style-name="Quotations">Quoted two.</text:p>
 <text:p text:style-name="Preformatted_20_Text">func main() {</text:p>
 <text:p text:style-name="Preformatted_20_Text"><text:s text:c="4"/>run()</text:p>
 <text:p text:style-name="Preformatted_20_Text"/>
 <text:p text:style-name="Preformatted_20_Text">}</text:p>
 <text:p text:style-name="Text_20_body">Interlude.</text:p>
 <text:p text:style-name="Text_20_body"><text:span text:style-name="Source_20_Text">ls -la</text:span></text:p>
 <text:list xml:id="list1" text:style-name="L1">
  <text:list-item><text:p>First</text:p></text:list-item>
  <text:list-item><text:p>Second</text:p><text:list><text:list-item><text:p>Nested a</text:p></text:list-item><text:list-item><text:p>Nested b</text:p></text:list-item></text:list></text:list-item>
 </text:list>
 <text:p text:style-name="Text_20_body">Interruption.</text:p>
 <text:list text:continue-numbering="true" text:style-name="L1"><text:list-item><text:p>Third</text:p></text:list-item></text:list>
 <text:list text:style-name="L2"><text:list-item><text:p>Bullet</text:p></text:list-item><text:list-item><text:list><text:list-item><text:p>Deep bullet</text:p></text:list-item></text:list></text:list-item></text:list>
 <text:list text:style-name="L3"><text:list-item><text:p>Roman three</text:p></text:list-item></text:list>
 <table:table table:name="Table1">
  <table:table-column table:number-columns-repeated="3"/>
  <table:table-header-rows><table:table-row><table:table-cell office:value-type="string"><text:p>Name</text:p></table:table-cell><table:table-cell table:number-columns-spanned="2"><text:p>Values</text:p></table:table-cell><table:covered-table-cell/></table:table-row></table:table-header-rows>
  <table:table-row><table:table-cell table:number-rows-spanned="2"><text:p>a</text:p></table:table-cell><table:table-cell><text:p text:style-name="P3">1</text:p></table:table-cell><table:table-cell><text:p text:style-name="P3">2</text:p></table:table-cell></table:table-row>
  <table:table-row><table:covered-table-cell/><table:table-cell><text:p text:style-name="P3">3</text:p></table:table-cell><table:table-cell><text:p text:style-name="P3">4</text:p></table:table-cell></table:table-row>
  <table:table-row table:number-rows-repeated="1000"><table:table-cell table:number-columns-repeated="3"/></table:table-row>
 </table:table>
 <text:p text:style-name="Table">Table <text:sequence text:ref-name="refTable0" text:name="Table" text:formula="ooow:Table+1" style:num-format="1">1</text:sequence>: Results</text:p>
 <text:p text:style-name="Standard"><draw:frame draw:name="Frame1" text:anchor-type="as-char" svg:width="8cm" svg:height="4cm"><draw:text-box fo:min-height="4cm"><text:p text:style-name="Illustration"><draw:frame draw:name="Image1" text:anchor-type="paragraph" svg:width="8cm" svg:height="4cm"><draw:image xlink:href="Pictures/img1.png" xlink:type="simple" xlink:show="embed" draw:mime-type="image/png"/><svg:title>A chart</svg:title></draw:frame>Illustration <text:sequence text:ref-name="refIllustration0" text:name="Illustration" text:formula="ooow:Illustration+1" style:num-format="1">1</text:sequence>: The <text:span text:style-name="T1">caption</text:span></text:p></draw:text-box></draw:frame></text:p>
 <text:p text:style-name="Standard">Inline <draw:frame draw:name="Image2" text:anchor-type="as-char" svg:width="1in" svg:height="12pt"><draw:image xlink:href="Pictures/img1.png" draw:mime-type="image/png"/></draw:frame> icon.</text:p>
 <text:p text:style-name="Standard"><draw:frame draw:name="Object1" text:anchor-type="as-char" svg:width="2cm" svg:height="1cm"><draw:object xlink:href="./Object 1" xlink:type="simple"/><draw:image xlink:href="./ObjectReplacements/Object 1" xlink:type="simple"/></draw:frame></text:p>
 <text:p text:style-name="Standard">Energy <draw:frame text:anchor-type="as-char"><draw:object xlink:href="./Object 1"/></draw:frame> inline.</text:p>
 <text:p text:style-name="Standard"><draw:frame text:anchor-type="paragraph"><draw:object xlink:href="./Object 2"/><draw:image xlink:href="./ObjectReplacements/Object 2"/></draw:frame>Chart paragraph.</text:p>
 <text:p text:style-name="Standard">Cited <text:bibliography-mark text:identifier="Smith2020" text:bibliography-type="article" text:author="Smith, John; Doe, Jane" text:title="On Things" text:journal="Journal of Stuff" text:year="2020" text:month="mar" text:volume="3" text:number="2" text:pages="10-20">[Smith2020]</text:bibliography-mark> and <text:bibliography-mark text:identifier="Book1" text:bibliography-type="book" text:author="Ann Author and Bob Writer" text:title="A Book" text:publisher="Pub" text:address="Riga" text:year="2019">[Book1]</text:bibliography-mark> again <text:bibliography-mark text:identifier="Smith2020" text:bibliography-type="article">[Smith2020]</text:bibliography-mark>.</text:p>
 <text:section text:name="Section1"><text:p text:style-name="Text_20_body">In section.</text:p></text:section>
 <text:bibliography text:name="Bibliography1"><text:index-body><text:p>Smith, J. On Things.</text:p></text:index-body></text:bibliography>
 <text:p text:style-name="P2">After page break.</text:p>
 <text:p text:style-name="Text_20_body"/>
 <text:p text:style-name="Text_20_body"/>
 <text:p text:style-name="Text_20_body">End <text:bookmark-ref text:reference-format="text" text:ref-name="intro">Introduction</text:bookmark-ref>.</text:p>
</office:text></office:body></office:document-content>`

const formulaXML = `<?xml version="1.0" encoding="UTF-8"?>
<math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><semantics><mfrac><mi>a</mi><mi>b</mi></mfrac><annotation encoding="text/plain">a over b</annotation></semantics></math>`

const chartXML = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content ` + nsDecl + `><office:body><office:chart><chart:chart/></office:chart></office:body></office:document-content>`

func buildODT(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := []string{"mimetype"}
	for name := range files {
		if name != "mimetype" {
			names = append(names, name)
		}
	}
	for _, name := range names {
		body, ok := files[name]
		if !ok {
			continue
		}
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func fullODT(t *testing.T) []byte {
	return buildODT(t, map[string]string{
		"mimetype":                    "application/vnd.oasis.opendocument.text",
		"content.xml":                 contentXML,
		"styles.xml":                  stylesXML,
		"meta.xml":                    metaXML,
		"Pictures/img1.png":           pngBytes,
		"Object 1/content.xml":        formulaXML,
		"ObjectReplacements/Object 1": "VCLMTF\x01\x00",
		"Object 2/content.xml":        chartXML,
		"ObjectReplacements/Object 2": "VCLMTF\x01\x00",
		"META-INF/manifest.xml":       `<manifest/>`,
	})
}

func TestReadODT(t *testing.T) {
	doc, warns, err := Read(context.Background(), fullODT(t), rd.Options{Name: "report.odt"})
	if err != nil {
		t.Fatal(err)
	}
	m := doc.Meta
	if m.Title != "Report Title" || m.Subtitle != "A subtitle" {
		t.Errorf("Title/Subtitle = %q/%q", m.Title, m.Subtitle)
	}
	if got := m.AuthorNames(); !reflect.DeepEqual(got, []string{"Anna Bērziņa"}) {
		t.Errorf("Authors = %q", got)
	}
	if m.Lang != "en-GB" || m.Date != "2024-03-05" || m.Subject != "Testing" || m.Summary != "A test document." {
		t.Errorf("meta = lang %q date %q subject %q summary %q", m.Lang, m.Date, m.Subject, m.Summary)
	}
	if !reflect.DeepEqual(m.Keywords, []string{"alpha", "beta", "gamma"}) {
		t.Errorf("Keywords = %q", m.Keywords)
	}
	want := strings.Join([]string{
		`H1#intro[Introduction]`,
		"P[Plain *[bold _[both]] and x^[2] H,[2]O `code()` u[under] ~[strike] sc[caps] =[mark]. Spaces tab\\nnext line continues.]",
		`P[See <#intro>[the intro], <https://example.test/>[a site] and a note^note{P[Footnote *[text].]}.]`,
		`P[Tracked inserted text.]`,
		`BQ{P[Quoted one.] P[Quoted two.]}`,
		`Code()"func main() {\n    run()\n\n}"`,
		`P[Interlude.]`,
		`Code()"ls -la"`,
		`OL(0,0){Pl[First] | Pl[Second] OL(0,1){Pl[Nested a] | Pl[Nested b]}}`,
		`P[Interruption.]`,
		`OL(3,0){Pl[Third]}`,
		`UL{Pl[Bullet] UL{Pl[Deep bullet]}}`,
		`OL(3,4){Pl[Roman three]}`,
		`T{cap:Results; cols:0,3,3; head: (Pl[Name]) (c2 Pl[Values]); body: (r2 Pl[a]) (Pl[1]) (Pl[2]) / (Pl[3]) (Pl[4])}`,
		`Fig(!img(res:Pictures/img1.png alt=A chart w=226.77pt h=113.39pt))[The *[caption]]`,
		`P[Inline !img(res:Pictures/img1.png w=72pt h=12pt) icon.]`,
		`$$\frac{a}{b}$$`,
		`P[Energy $\frac{a}{b}$ inline.]`,
		`P[Chart paragraph.]`,
		`P[Cited @cite(Smith2020)[[Smith2020]] and @cite(Book1)[[Book1]] again @cite(Smith2020)[[Smith2020]].]`,
		`P[In section.]`,
		`Bib`,
		`PB`,
		`P[After page break.]`,
		`P[End Introduction.]`,
	}, "\n")
	if got := asttest.Dump(doc.Blocks); got != want {
		t.Errorf("blocks:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(asttest.Dump(doc.Blocks), "DELETED") || strings.Contains(asttest.Dump(doc.Blocks), "Comment") {
		t.Error("deleted tracked text or comments leaked into the output")
	}

	wantRefs := []ast.Reference{
		{
			ID: "Smith2020", Type: "article-journal", Title: "On Things", ContainerTitle: "Journal of Stuff",
			Volume: "3", Issue: "2", Page: "10-20",
			Author: []ast.Name{{Family: "Smith", Given: "John"}, {Family: "Doe", Given: "Jane"}},
			Issued: ast.Date{Year: 2020, Month: 3},
		},
		{
			ID: "Book1", Type: "book", Title: "A Book", Publisher: "Pub", PublisherPlace: "Riga",
			Author: []ast.Name{{Family: "Author", Given: "Ann"}, {Family: "Writer", Given: "Bob"}},
			Issued: ast.Date{Year: 2019},
		},
	}
	if !reflect.DeepEqual(doc.References, wantRefs) {
		t.Errorf("References =\n%+v\nwant\n%+v", doc.References, wantRefs)
	}
	if names := doc.Resources.Names(); !reflect.DeepEqual(names, []string{"Pictures/img1.png"}) {
		t.Errorf("resources = %q", names)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "chart or OLE") {
		t.Errorf("warnings = %q", warns)
	}
}

func flatODT(body, extraStyles string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<office:document ` + nsDecl + ` office:version="1.3" office:mimetype="application/vnd.oasis.opendocument.text">
<office:meta><dc:title>Flat Title</dc:title><dc:creator>Flat Author</dc:creator></office:meta>
<office:font-face-decls><style:font-face style:name="Courier New" svg:font-family="'Courier New'"/></office:font-face-decls>
<office:styles>
 <style:default-style style:family="paragraph"><style:text-properties fo:language="lv" fo:country="LV"/></style:default-style>
 <style:style style:name="Standard" style:family="paragraph"/>
 <style:style style:name="Heading_20_2" style:display-name="Heading 2" style:family="paragraph"/>
 <style:style style:name="Quote" style:family="paragraph"/>
` + extraStyles + `
</office:styles>
<office:automatic-styles>
 <style:style style:name="T1" style:family="text"><style:text-properties fo:font-weight="700"/></style:style>
 <style:style style:name="T2" style:family="text"><style:text-properties style:font-name="Courier New"/></style:style>
 <text:list-style style:name="LH"><text:list-level-style-number text:level="1" style:num-format="1"/></text:list-style>
</office:automatic-styles>
<office:body><office:text>` + body + `</office:text></office:body></office:document>`
}

func TestReadFlatODT(t *testing.T) {
	png64 := base64.StdEncoding.EncodeToString([]byte(pngBytes))
	body := `
<text:list text:style-name="LH"><text:list-item><text:h text:outline-level="1"><text:number>1.</text:number>Numbered heading</text:h></text:list-item></text:list>
<text:p text:style-name="Heading_20_2">Styled heading</text:p>
<text:p>Inline math <draw:frame text:anchor-type="as-char"><draw:object><math xmlns="http://www.w3.org/1998/Math/MathML"><msup><mi>x</mi><mn>2</mn></msup></math></draw:object></draw:frame> here.</text:p>
<text:p><draw:frame draw:name="Pic" text:anchor-type="paragraph" svg:width="10mm" svg:height="5mm"><draw:image><office:binary-data>` + png64 + `</office:binary-data></draw:image><svg:desc>Embedded picture</svg:desc></draw:frame></text:p>
<text:p text:style-name="Quote">A quote<office:annotation><text:p>note to self</text:p></office:annotation>.</text:p>
<text:p><text:span text:style-name="T2">mono</text:span> and <text:span text:style-name="T1">bold</text:span><text:span text:style-name="T1"> </text:span><text:span text:style-name="T1">run</text:span>.</text:p>
<table:table table:name="Grid">
 <table:table-row><table:table-cell><text:p><text:span text:style-name="T1">Head A</text:span></text:p></table:table-cell><table:table-cell><text:p><text:span text:style-name="T1">Head B</text:span></text:p></table:table-cell><table:table-cell table:number-columns-repeated="20"/></table:table-row>
 <table:table-row><table:table-cell><text:p>1</text:p></table:table-cell><table:table-cell><text:p>2</text:p></table:table-cell><table:table-cell table:number-columns-repeated="20"/></table:table-row>
</table:table>
<text:p>Image link <draw:frame text:anchor-type="as-char"><draw:image xlink:href="Pictures/missing.png"/></draw:frame>.</text:p>
<text:p>See <text:a xlink:href="#Grid|table">the grid</text:a> and <text:a xlink:href="javascript:alert(1)">bad</text:a>.</text:p>`
	doc, warns, err := Read(context.Background(), []byte(flatODT(body, "")), rd.Options{Name: "flat.fodt"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Meta.Title != "Flat Title" || doc.Meta.Lang != "lv-LV" {
		t.Errorf("meta = %q %q", doc.Meta.Title, doc.Meta.Lang)
	}
	if got := doc.Meta.AuthorNames(); !reflect.DeepEqual(got, []string{"Flat Author"}) {
		t.Errorf("Authors = %q", got)
	}
	want := strings.Join([]string{
		`H1[Numbered heading]`,
		`H2[Styled heading]`,
		`P[Inline math $x^{2}$ here.]`,
		`Fig(!img(res:Pictures/image-1.png alt=Embedded picture w=28.35pt h=14.17pt))`,
		`BQ{P[A quote.]}`,
		"P[`mono` and *[bold run].]",
		`T#grid{cols:0,0; head: (Pl[Head A]) (Pl[Head B]); body: (Pl[1]) (Pl[2])}`,
		`P[Image link !img(Pictures/missing.png).]`,
		`P[See <#grid>[the grid] and bad.]`,
	}, "\n")
	if got := asttest.Dump(doc.Blocks); got != want {
		t.Errorf("blocks:\ngot:\n%s\nwant:\n%s", got, want)
	}
	// The linked picture is resolved (and reported if missing) by the
	// caller's image resolver, not by the reader.
	if len(warns) != 0 {
		t.Errorf("warnings = %q", warns)
	}
}

func TestReadODTErrors(t *testing.T) {
	ctx := context.Background()
	if _, _, err := Read(ctx, []byte("PK\x03\x04garbage"), rd.Options{}); err == nil {
		t.Error("expected error for a broken zip")
	}
	noContent := buildODT(t, map[string]string{"mimetype": "application/vnd.oasis.opendocument.text", "styles.xml": stylesXML})
	if _, _, err := Read(ctx, noContent, rd.Options{}); err == nil {
		t.Error("expected error without content.xml")
	}
	if _, _, err := Read(ctx, []byte(`<office:document `+nsDecl+`><office:body><office:spreadsheet/></office:body></office:document>`), rd.Options{}); err == nil {
		t.Error("expected error for a non-text document")
	}
	if _, _, err := Read(ctx, fullODT(t), rd.Options{Limits: rd.Limits{MaxEntry: 100}}); !errors.Is(err, rd.ErrLimit) {
		t.Errorf("limits: err = %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := Read(cancelled, fullODT(t), rd.Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: err = %v", err)
	}
	// Truncated XML must not panic and keeps what was parsed.
	truncated := strings.TrimSuffix(flatODT(`<text:p>Kept text</text:p>`, ""), "</office:text></office:body></office:document>")
	doc, _, err := Read(ctx, []byte(truncated), rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := asttest.Dump(doc.Blocks); got != "P[Kept text]" {
		t.Errorf("truncated: %s", got)
	}
}

func TestReadODTTableLimits(t *testing.T) {
	var rows strings.Builder
	for i := 0; i < 30; i++ {
		rows.WriteString(`<table:table-row><table:table-cell><text:p>a</text:p></table:table-cell><table:table-cell><text:p>b</text:p></table:table-cell></table:table-row>`)
	}
	rows.WriteString(`<table:table-row table:number-rows-repeated="1048000"><table:table-cell><text:p>x</text:p></table:table-cell></table:table-row>`)
	doc, warns, err := Read(context.Background(), []byte(flatODT(`<table:table>`+rows.String()+`</table:table>`, "")), rd.Options{Limits: rd.Limits{MaxTableCells: 100}})
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := doc.Blocks[0].(*ast.Table)
	if !ok {
		t.Fatalf("got %s", asttest.Dump(doc.Blocks))
	}
	cells := 0
	for _, r := range append(tbl.Head, tbl.Body...) {
		cells += len(r.Cells)
	}
	if cells > 100 || len(warns) == 0 {
		t.Errorf("cells = %d, warnings = %q", cells, warns)
	}
}

func TestTextPosition(t *testing.T) {
	tests := []struct {
		in       string
		sup, sub bool
	}{
		{"super 58%", true, false}, {"sub 58%", false, true}, {"33% 58%", true, false},
		{"-33% 58%", false, true}, {"0% 100%", false, false}, {"", false, false},
	}
	for _, tt := range tests {
		if sup, sub := textPosition(tt.in); sup != tt.sup || sub != tt.sub {
			t.Errorf("textPosition(%q) = %v,%v", tt.in, sup, sub)
		}
	}
}

func TestSplitNames(t *testing.T) {
	tests := []struct {
		in   string
		want []ast.Name
	}{
		{"Smith, John; Doe, Jane", []ast.Name{{Family: "Smith", Given: "John"}, {Family: "Doe", Given: "Jane"}}},
		{"John Ronald Tolkien and Plato", []ast.Name{{Family: "Tolkien", Given: "John Ronald"}, {Family: "Plato"}}},
		{"", nil},
	}
	for _, tt := range tests {
		if got := splitNames(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitNames(%q) = %+v", tt.in, got)
		}
	}
}

func TestLength(t *testing.T) {
	tests := map[string]string{"2.54cm": "72pt", "10mm": "28.35pt", "1in": "72pt", "12pt": "12pt", "2pc": "24pt", "100px": "75pt", "50%": "50%", "": "", "abc": "", "-1cm": ""}
	for in, want := range tests {
		if got := length(in); got != want {
			t.Errorf("length(%q) = %q, want %q", in, got, want)
		}
	}
}
