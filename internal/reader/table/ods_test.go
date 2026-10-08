package table

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

const odsNS = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" xmlns:chart="urn:oasis:names:tc:opendocument:xmlns:chart:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0"`

func ods(t *testing.T, styles, body string, extra map[string]string) []byte {
	t.Helper()
	files := map[string]string{
		"mimetype":    "application/vnd.oasis.opendocument.spreadsheet",
		"content.xml": `<?xml version="1.0" encoding="UTF-8"?><office:document-content ` + odsNS + `><office:automatic-styles>` + styles + `</office:automatic-styles><office:body><office:spreadsheet>` + body + `</office:spreadsheet></office:body></office:document-content>`,
	}
	for k, v := range extra {
		files[k] = v
	}
	return zipBytes(t, files)
}

func readODS(t *testing.T, data []byte) (*ast.Document, []string) {
	t.Helper()
	doc, warns, err := ReadODS(context.Background(), data, rd.Options{Name: "book.ods"})
	if err != nil {
		t.Fatalf("ReadODS: %v", err)
	}
	if doc.Resources == nil {
		t.Fatal("Resources not initialised")
	}
	return doc, warns
}

func TestODSValuesAndRepeats(t *testing.T) {
	body := `<table:table table:name="Data">
<table:table-column table:number-columns-repeated="2"/><table:table-column table:visibility="collapse"/><table:table-column table:number-columns-repeated="1021"/>
<table:table-header-rows><table:table-row>
  <table:table-cell office:value-type="string"><text:p>Item</text:p></table:table-cell>
  <table:table-cell office:value-type="string"><text:p>Amount</text:p></table:table-cell>
  <table:table-cell office:value-type="string"><text:p>Hidden col</text:p></table:table-cell>
  <table:table-cell office:value-type="string"><text:p>Typed</text:p></table:table-cell>
  <table:table-cell table:number-columns-repeated="1020"/>
</table:table-row></table:table-header-rows>
<table:table-row>
  <table:table-cell office:value-type="string"><text:p>two<text:s text:c="3"/>spaces</text:p><office:annotation><text:p>a comment</text:p></office:annotation></table:table-cell>
  <table:table-cell office:value-type="float" office:value="1234.5"><text:p>1 234,50</text:p></table:table-cell>
  <table:table-cell office:value-type="string"><text:p>x</text:p></table:table-cell>
  <table:table-cell office:value-type="percentage" office:value="0.125"/>
</table:table-row>
<table:table-row table:visibility="collapse"><table:table-cell office:value-type="string"><text:p>hidden row</text:p></table:table-cell></table:table-row>
<table:table-row>
  <table:table-cell office:value-type="string"><text:p>line</text:p><text:p>break<text:line-break/>here</text:p></table:table-cell>
  <table:table-cell office:value-type="currency" office:value="-5"><text:p>-5,00 €</text:p></table:table-cell>
  <table:table-cell/>
  <table:table-cell office:value-type="date" office:date-value="2024-03-05T10:30:00"/>
</table:table-row>
<table:table-row>
  <table:table-cell><text:p><text:a xlink:href="https://example.org/">site</text:a></text:p></table:table-cell>
  <table:table-cell office:value-type="float" office:value="7"/>
  <table:table-cell/>
  <table:table-cell office:value-type="time" office:time-value="PT08H05M00S"/>
</table:table-row>
<table:table-row>
  <table:table-cell office:value-type="string"><text:p>yes</text:p></table:table-cell>
  <table:table-cell office:value-type="float" office:value="1"><text:p>1</text:p></table:table-cell>
  <table:table-cell/>
  <table:table-cell office:value-type="boolean" office:boolean-value="true"/>
</table:table-row>
<table:table-row table:number-rows-repeated="2"><table:table-cell office:value-type="string"><text:p>rep</text:p></table:table-cell><table:table-cell office:value-type="float" office:value="2" table:number-columns-repeated="1"/></table:table-row>
<table:table-row table:number-rows-repeated="1048570"><table:table-cell table:number-columns-repeated="1024"/></table:table-row>
</table:table>`
	doc, warns := readODS(t, ods(t, "", body, nil))
	checkDump(t, doc.Blocks, "Table[-R-]{H[Item|Amount|Typed][two   spaces|1 234,50|12.5%][line↵break↵here|-5,00 €|2024-03-05 10:30]"+
		"[[site](https://example.org/)|7|8:05][yes|1|TRUE][rep|2|][rep|2|]}")
	if len(warns) != 0 {
		t.Errorf("warnings = %q", warns)
	}
}

func TestODSSpansSheetsAndMeta(t *testing.T) {
	styles := `<style:style style:name="ta1" style:family="table"><style:table-properties table:display="true"/></style:style>` +
		`<style:style style:name="ta2" style:family="table"><style:table-properties table:display="false"/></style:style>`
	body := `<table:table table:name="Merged" table:style-name="ta1">
<table:table-row><table:table-cell table:number-columns-spanned="2" office:value-type="string"><text:p>Group</text:p></table:table-cell><table:covered-table-cell/><table:table-cell office:value-type="string"><text:p>Other</text:p></table:table-cell></table:table-row>
<table:table-row><table:table-cell office:value-type="string"><text:p>a</text:p></table:table-cell><table:table-cell office:value-type="string"><text:p>b</text:p></table:table-cell><table:table-cell table:number-rows-spanned="2" office:value-type="string"><text:p>tall</text:p></table:table-cell></table:table-row>
<table:table-row><table:table-cell office:value-type="string"><text:p>c</text:p></table:table-cell><table:table-cell office:value-type="string"><text:p>d</text:p></table:table-cell><table:covered-table-cell/></table:table-row>
</table:table>
<table:table table:name="Secret" table:style-name="ta2"><table:table-row><table:table-cell><text:p>x</text:p></table:table-cell></table:table-row></table:table>
<table:table table:name="Blank"><table:table-row table:number-rows-repeated="5"><table:table-cell table:number-columns-repeated="5"/></table:table-row></table:table>`
	meta := `<office:document-meta ` + odsNS + `><office:meta><dc:title>Plan</dc:title><meta:initial-creator>Original Author</meta:initial-creator><dc:creator>Last Editor</dc:creator><meta:keyword>one</meta:keyword><meta:keyword>two</meta:keyword></office:meta></office:document-meta>`
	doc, warns := readODS(t, ods(t, styles, body, map[string]string{"meta.xml": meta}))
	checkDump(t, doc.Blocks, "H1 Merged\nTable[---]{H[Group<c2>|Other][a|b|tall<r2>][c|d]}")
	if !slices.Contains(warns, `sheet "Secret" is hidden and was skipped`) {
		t.Errorf("warnings = %q", warns)
	}
	m := doc.Meta
	if m.Title != "Plan" || len(m.Authors) != 1 || m.Authors[0].Name != "Original Author" || !slices.Equal(m.Keywords, []string{"one", "two"}) {
		t.Errorf("meta = %+v", m)
	}
}

func TestODSPicturesAndCharts(t *testing.T) {
	body := `<table:table table:name="Sheet1">
<table:shapes><draw:frame svg:width="2.54cm" svg:height="1.27cm"><draw:image xlink:href="Pictures/logo.png"/><svg:title>Logo</svg:title></draw:frame></table:shapes>
<table:table-row><table:table-cell office:value-type="string"><text:p>Month</text:p></table:table-cell><table:table-cell office:value-type="string"><text:p>Sales</text:p>
<draw:frame svg:width="8cm" svg:height="5cm"><draw:object xlink:href="./Object 1"/><draw:image xlink:href="./ObjectReplacements/Object 1"/></draw:frame></table:table-cell></table:table-row>
<table:table-row><table:table-cell office:value-type="string"><text:p>Jan</text:p></table:table-cell><table:table-cell office:value-type="float" office:value="10"><text:p>10</text:p></table:table-cell></table:table-row>
</table:table>`
	chartContent := `<office:document-content ` + odsNS + `><office:body><office:chart><chart:chart>
<chart:title><text:p>Monthly sales</text:p></chart:title>
<chart:plot-area><chart:axis chart:dimension="x"><chart:title><text:p>Month axis</text:p></chart:title></chart:axis></chart:plot-area>
<table:table table:name="local-table"><table:table-header-columns><table:table-column/></table:table-header-columns><table:table-columns><table:table-column/></table:table-columns>
<table:table-header-rows><table:table-row><table:table-cell><text:p/></table:table-cell><table:table-cell office:value-type="string"><text:p>Sales</text:p></table:table-cell></table:table-row></table:table-header-rows>
<table:table-rows><table:table-row><table:table-cell office:value-type="string"><text:p>Jan</text:p></table:table-cell><table:table-cell office:value-type="float" office:value="10"><text:p>10</text:p></table:table-cell></table:table-row>
<table:table-row><table:table-cell office:value-type="string"><text:p>Feb</text:p></table:table-cell><table:table-cell office:value-type="float" office:value="12.5"><text:p>12.5</text:p></table:table-cell></table:table-row></table:table-rows>
</table:table></chart:chart></office:chart></office:body></office:document-content>`
	doc, warns := readODS(t, ods(t, "", body, map[string]string{
		"Pictures/logo.png":           string(pngBytes),
		"Object 1/content.xml":        chartContent,
		"ObjectReplacements/Object 1": "svm",
	}))
	checkDump(t, doc.Blocks, strings.Join([]string{
		"Table[-R]{H[Month|Sales][Jan|10]}",
		"Fig ![Logo](res:ods/logo.png 72.0pt×36.0pt)",
		"Table(Monthly sales)[-R]{H[|Sales][Jan|10][Feb|12.5]}",
	}, "\n"))
	if len(warns) != 0 {
		t.Errorf("warnings = %q", warns)
	}
}

func TestODSErrorsAndEmpty(t *testing.T) {
	if _, _, err := ReadODS(context.Background(), []byte("nope"), rd.Options{}); err == nil {
		t.Error("expected error for non-zip input")
	}
	if _, _, err := ReadODS(context.Background(), zipBytes(t, map[string]string{"mimetype": "x"}), rd.Options{}); err == nil {
		t.Error("expected error for missing content.xml")
	}
	doc, warns := readODS(t, ods(t, "", `<table:table table:name="E"><table:table-row><table:table-cell/></table:table-row></table:table>`, nil))
	checkDump(t, doc.Blocks, "P ")
	if len(warns) != 1 {
		t.Errorf("warnings = %q", warns)
	}
}

func TestODSLimits(t *testing.T) {
	body := `<table:table table:name="Big"><table:table-row table:number-rows-repeated="100000"><table:table-cell office:value-type="string" table:number-columns-repeated="100"><text:p>x</text:p></table:table-cell></table:table-row></table:table>`
	doc, warns, err := ReadODS(context.Background(), ods(t, "", body, nil), rd.Options{Limits: rd.Limits{MaxTableCells: 1000}})
	if err != nil {
		t.Fatal(err)
	}
	tbl := doc.Blocks[0].(*ast.Table)
	if n := len(tbl.Head) + len(tbl.Body); n != 10 || len(warns) == 0 {
		t.Errorf("rows = %d, warnings = %q", n, warns)
	}
}

func TestLengthPt(t *testing.T) {
	tests := map[string]string{"2.54cm": "72.0pt", "10mm": "28.3pt", "1in": "72.0pt", "12pt": "12.0pt", "1pc": "12.0pt", "96px": "72.0pt", "": "", "5em": "", "-1cm": ""}
	for in, want := range tests {
		if got := lengthPt(in); got != want {
			t.Errorf("lengthPt(%q) = %q, want %q", in, got, want)
		}
	}
}

func FuzzReadODS(f *testing.F) {
	f.Add([]byte("PK\x03\x04"))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, _, err := ReadODS(context.Background(), data, rd.Options{})
		if err == nil && doc == nil {
			t.Fatal("nil document without error")
		}
	})
}
