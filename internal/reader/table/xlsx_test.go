package table

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

const (
	nsMain = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	nsRel  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
)

var pngBytes = func() []byte {
	b, _ := hex.DecodeString("89504e470d0a1a0a0000000d4948445200000001000000010806000000" +
		"1f15c4890000000d49444154789c6360000002000154a24f5d0000000049454e44ae426082")
	return b
}()

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(files[n])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type xsheet struct {
	name, state, body, rels string
	chartsheet              bool
}

// xlsx assembles a workbook: sheets get worksheet parts and relationships;
// extra parts (styles, shared strings, charts, media) are added as given.
func xlsx(t *testing.T, sheets []xsheet, extra map[string]string) []byte {
	t.Helper()
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="` + nsRel + `officeDocument" Target="xl/workbook.xml"/></Relationships>`,
	}
	var wb, rels strings.Builder
	wb.WriteString(`<workbook ` + nsMain + `>`)
	if v, ok := extra["workbookPr"]; ok {
		wb.WriteString(v)
		delete(extra, "workbookPr")
	}
	wb.WriteString(`<sheets>`)
	rels.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i, s := range sheets {
		state := ""
		if s.state != "" {
			state = ` state="` + s.state + `"`
		}
		fmt.Fprintf(&wb, `<sheet name="%s" sheetId="%d" r:id="rId%d"%s/>`, s.name, i+1, i+1, state)
		kind, dir, root := "worksheet", "worksheets", "worksheet"
		if s.chartsheet {
			kind, dir, root = "chartsheet", "chartsheets", "chartsheet"
		}
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="%s%s" Target="%s/sheet%d.xml"/>`, i+1, nsRel, kind, dir, i+1)
		files[fmt.Sprintf("xl/%s/sheet%d.xml", dir, i+1)] = `<` + root + ` ` + nsMain + `>` + s.body + `</` + root + `>`
		if s.rels != "" {
			files[fmt.Sprintf("xl/%s/_rels/sheet%d.xml.rels", dir, i+1)] = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + s.rels + `</Relationships>`
		}
	}
	wb.WriteString(`</sheets></workbook>`)
	if _, ok := extra["xl/styles.xml"]; ok {
		rels.WriteString(`<Relationship Id="rIdS" Type="` + nsRel + `styles" Target="styles.xml"/>`)
	}
	if _, ok := extra["xl/sharedStrings.xml"]; ok {
		rels.WriteString(`<Relationship Id="rIdT" Type="` + nsRel + `sharedStrings" Target="/xl/sharedStrings.xml"/>`)
	}
	rels.WriteString(`</Relationships>`)
	files["xl/workbook.xml"] = wb.String()
	files["xl/_rels/workbook.xml.rels"] = rels.String()
	for k, v := range extra {
		files[k] = v
	}
	return zipBytes(t, files)
}

// row builds <row> XML from inline-string cells; "" leaves a cell out and
// "#n" writes the number n.
func row(r int, vals ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<row r="%d">`, r)
	for i, v := range vals {
		if v == "" {
			continue
		}
		ref := string(rune('A'+i)) + fmt.Sprint(r)
		if n, ok := strings.CutPrefix(v, "#"); ok {
			fmt.Fprintf(&b, `<c r="%s"><v>%s</v></c>`, ref, n)
		} else {
			fmt.Fprintf(&b, `<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, v)
		}
	}
	b.WriteString(`</row>`)
	return b.String()
}

func readXLSX(t *testing.T, data []byte) (*ast.Document, []string) {
	t.Helper()
	doc, warns, err := ReadXLSX(context.Background(), data, rd.Options{Name: "book.xlsx"})
	if err != nil {
		t.Fatalf("ReadXLSX: %v", err)
	}
	if doc.Resources == nil {
		t.Fatal("Resources not initialised")
	}
	return doc, warns
}

const testStyles = `<styleSheet ` + nsMain + `>
<numFmts count="3"><numFmt numFmtId="164" formatCode="dd/mm/yyyy"/><numFmt numFmtId="165" formatCode="&quot;€&quot;#,##0.00"/><numFmt numFmtId="166" formatCode="0.0%"/></numFmts>
<cellStyleXfs count="1"><xf numFmtId="0"/></cellStyleXfs>
<cellXfs count="7"><xf numFmtId="0"/><xf numFmtId="164" applyNumberFormat="1"/><xf numFmtId="14"/><xf numFmtId="165"/><xf numFmtId="166"/><xf numFmtId="20"/><xf numFmtId="0"><alignment horizontal="center"/></xf></cellXfs>
</styleSheet>`

func TestXLSXValues(t *testing.T) {
	sst := `<sst ` + nsMain + ` count="3" uniqueCount="3"><si><t>Name</t></si><si><r><rPr><b/></rPr><t>Rich</t></r><r><t xml:space="preserve"> text</t></r><rPh sb="0" eb="1"><t>ルビ</t></rPh></si><si><t>line_x000D_
two</t></si></sst>`
	body := `<sheetData>` +
		`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="inlineStr"><is><t>Kind</t></is></c><c r="C1" t="inlineStr"><is><t>Value</t></is></c></row>` +
		`<row r="2"><c t="s"><v>1</v></c><c t="inlineStr"><is><t>bool</t></is></c><c t="b"><v>1</v></c></row>` +
		`<row r="3"><c t="s"><v>2</v></c><c t="inlineStr"><is><t>error</t></is></c><c t="e"><v>#DIV/0!</v></c></row>` +
		`<row r="4"><c r="A4" t="inlineStr"><is><t>d</t></is></c><c r="B4" t="inlineStr"><is><t>custom date</t></is></c><c r="C4" s="1"><v>45306</v></c></row>` +
		`<row r="5"><c r="A5" t="inlineStr"><is><t>e</t></is></c><c r="B5" t="inlineStr"><is><t>builtin date+time</t></is></c><c r="C5" s="2"><v>45306.75</v></c></row>` +
		`<row r="6"><c r="A6" t="inlineStr"><is><t>f</t></is></c><c r="B6" t="inlineStr"><is><t>currency</t></is></c><c r="C6" s="3"><v>-1300.255</v></c></row>` +
		`<row r="7"><c r="A7" t="inlineStr"><is><t>g</t></is></c><c r="B7" t="inlineStr"><is><t>percent</t></is></c><c r="C7" s="4"><v>0.125</v></c></row>` +
		`<row r="8"><c r="A8" t="inlineStr"><is><t>h</t></is></c><c r="B8" t="inlineStr"><is><t>time</t></is></c><c r="C8" s="5"><v>0.5</v></c></row>` +
		`<row r="9"><c r="A9" t="inlineStr"><is><t>i</t></is></c><c r="B9" t="inlineStr"><is><t>general</t></is></c><c r="C9"><v>0.30000000000000004</v></c></row>` +
		`<row r="10"><c r="A10" t="inlineStr"><is><t>j</t></is></c><c r="B10" t="str"><v>formula text</v></c><c r="C10" t="d"><v>2024-02-29T08:15:00</v></c></row>` +
		`<row r="11"><c r="A11" t="inlineStr"><is><t>k</t></is></c><c r="B11" t="inlineStr"><is><t>no cached value</t></is></c><c r="C11"><f>SUM(1,2)</f></c></row>` +
		`</sheetData>`
	doc, warns := readXLSX(t, xlsx(t, []xsheet{{name: "Data", body: body}}, map[string]string{"xl/styles.xml": testStyles, "xl/sharedStrings.xml": sst}))
	checkDump(t, doc.Blocks, "Table[---]{H[Name|Kind|Value]"+
		"[Rich text|bool|TRUE][line↵two|error|#DIV/0!][d|custom date|2024-01-15][e|builtin date+time|2024-01-15 18:00]"+
		"[f|currency|-€1,300.26][g|percent|12.5%][h|time|12:00][i|general|0.3][j|formula text|2024-02-29 08:15][k|no cached value|]}")
	if len(warns) != 0 {
		t.Errorf("warnings = %q", warns)
	}
}

func TestXLSXLayout(t *testing.T) {
	report := `<cols><col min="3" max="3" hidden="1" width="5"/></cols><sheetData>` +
		row(1, "Sales report 2024") +
		row(3, "Region", "Q1", "secret", "Q2") +
		row(4, "North", "#10", "x", "#20") +
		`<row r="5" hidden="1"><c r="A5" t="inlineStr"><is><t>Hidden</t></is></c><c r="B5"><v>1</v></c></row>` +
		row(6, "South", "#30", "y", "#40") +
		row(8, "These figures are preliminary and will be revised after the annual audit of all the regional offices.") +
		`</sheetData><mergeCells count="1"><mergeCell ref="A1:D1"/></mergeCells>`
	merged := `<sheetData>` + row(1, "Item", "Detail", "") + row(2, "A", "one", "") + row(3, "", "two", "") + row(4, "Wide", "", "") + `</sheetData>` +
		`<mergeCells count="2"><mergeCell ref="A2:A3"/><mergeCell ref="A4:B4"/></mergeCells>`
	text := `<sheetData>` + row(1, "OVERVIEW") +
		row(2, "This worksheet holds prose: long paragraphs that someone typed into single cells because the spreadsheet was at hand.") +
		row(3, "A second paragraph follows, again well over eighty characters long so the sheet reads as a document.") + `</sheetData>`
	data := xlsx(t, []xsheet{
		{name: "Report", body: report},
		{name: "Hidden", state: "hidden", body: `<sheetData>` + row(1, "secret") + `</sheetData>`},
		{name: "Merged", body: merged},
		{name: "Notes", body: text},
		{name: "Empty", body: `<sheetData/>`},
	}, nil)
	doc, warns := readXLSX(t, data)
	checkDump(t, doc.Blocks, strings.Join([]string{
		"H1 Report",
		"H2 Sales report 2024",
		"Table[-RR]{H[Region|Q1|Q2][North|10|20][South|30|40]}",
		"P These figures are preliminary and will be revised after the annual audit of all the regional offices.",
		"H1 Merged",
		"Table[--]{H[Item|Detail][A<r2>|one][two][Wide<c2>]}",
		"H1 Notes",
		"H2 OVERVIEW",
		"P This worksheet holds prose: long paragraphs that someone typed into single cells because the spreadsheet was at hand.",
		"P A second paragraph follows, again well over eighty characters long so the sheet reads as a document.",
	}, "\n"))
	if !slices.Contains(warns, `sheet "Hidden" is hidden and was skipped`) {
		t.Errorf("warnings = %q", warns)
	}
}

func TestXLSXSingleSheetHasNoHeading(t *testing.T) {
	doc, _ := readXLSX(t, xlsx(t, []xsheet{{name: "Only", body: `<sheetData>` + row(1, "a", "b") + row(2, "#1", "#2") + `</sheetData>`}}, nil))
	checkDump(t, doc.Blocks, "Table[RR]{H[a|b][1|2]}")
}

func TestXLSXSpacerColumnsAndRowsWithoutRefs(t *testing.T) {
	body := `<sheetData><row><c t="inlineStr"><is><t>a</t></is></c><c/><c t="inlineStr"><is><t>b</t></is></c></row>` +
		`<row><c><v>1</v></c><c/><c><v>2</v></c></row></sheetData>`
	doc, _ := readXLSX(t, xlsx(t, []xsheet{{name: "S", body: body}}, nil))
	checkDump(t, doc.Blocks, "Table[RR]{H[a|b][1|2]}")
}

const barChart = `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><c:chart>
<c:title><c:tx><c:rich><a:p><a:r><a:t>Quarterly </a:t></a:r><a:r><a:t>revenue</a:t></a:r></a:p></c:rich></c:tx></c:title><c:autoTitleDeleted val="0"/>
<c:plotArea><c:barChart>
<c:ser><c:idx val="0"/><c:tx><c:strRef><c:f>Data!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>2023</c:v></c:pt></c:strCache></c:strRef></c:tx>
<c:dLbls><c:txPr><a:p><a:r><a:t>label text</a:t></a:r></a:p></c:txPr></c:dLbls>
<c:cat><c:strRef><c:f>Data!$A$2:$A$4</c:f><c:strCache><c:ptCount val="3"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt><c:pt idx="2"><c:v>Q3</c:v></c:pt></c:strCache></c:strRef></c:cat>
<c:val><c:numRef><c:f>Data!$B$2:$B$4</c:f><c:numCache><c:formatCode>#,##0</c:formatCode><c:ptCount val="3"/><c:pt idx="0"><c:v>1000</c:v></c:pt><c:pt idx="2"><c:v>3000.4</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>
<c:ser><c:idx val="1"/><c:tx><c:v>2024</c:v></c:tx><c:val><c:numRef><c:numCache><c:ptCount val="3"/><c:pt idx="0"><c:v>1500</c:v></c:pt><c:pt idx="1"><c:v>2500</c:v></c:pt><c:pt idx="2"><c:v>3500</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>
</c:barChart><c:catAx><c:title><c:tx><c:rich><a:p><a:r><a:t>Quarter</a:t></a:r></a:p></c:rich></c:tx></c:title></c:catAx></c:plotArea></c:chart></c:chartSpace>`

// A chart written without cached values (as some libraries do): data comes
// from the referenced cells.
const refChart = `<chartSpace xmlns="http://schemas.openxmlformats.org/drawingml/2006/chart"><chart><plotArea><pieChart>
<ser><tx><strRef><f>'My Data'!B1</f></strRef></tx><cat><numRef><f>'My Data'!$A$2:$A$4</f></numRef></cat><val><numRef><f>'My Data'!$B$2:$B$4</f></numRef></val></ser>
</pieChart></plotArea></chart></chartSpace>`

const scatterChart = `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:autoTitleDeleted val="1"/><c:plotArea><c:scatterChart>
<c:ser><c:tx><c:v>Only</c:v></c:tx><c:xVal><c:numRef><c:numCache><c:formatCode>General</c:formatCode><c:pt idx="0"><c:v>0.5</c:v></c:pt><c:pt idx="1"><c:v>1.5</c:v></c:pt></c:numCache></c:numRef></c:xVal>
<c:yVal><c:numRef><c:numCache><c:formatCode>0%</c:formatCode><c:pt idx="0"><c:v>0.25</c:v></c:pt><c:pt idx="1"><c:v>0.75</c:v></c:pt></c:numCache></c:numRef></c:yVal></c:ser>
</c:scatterChart></c:plotArea></c:chart></c:chartSpace>`

func drawingXML(anchors ...string) string {
	return `<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006">` +
		strings.Join(anchors, "") + `</xdr:wsDr>`
}

func chartAnchor(row int, rid string) string {
	return fmt.Sprintf(`<xdr:twoCellAnchor><xdr:from><xdr:col>4</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from><xdr:to><xdr:col>9</xdr:col><xdr:row>20</xdr:row></xdr:to>`+
		`<xdr:graphicFrame macro=""><xdr:nvGraphicFramePr><xdr:cNvPr id="2" name="Chart 1"/><xdr:cNvGraphicFramePr/></xdr:nvGraphicFramePr><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="%s"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:twoCellAnchor>`, row, rid)
}

func picAnchor(row int, rid, descr string) string {
	return fmt.Sprintf(`<xdr:oneCellAnchor><xdr:from><xdr:col>0</xdr:col><xdr:row>%d</xdr:row></xdr:from><xdr:ext cx="914400" cy="457200"/>`+
		`<xdr:pic><xdr:nvPicPr><xdr:cNvPr id="3" name="Picture 2" descr="%s"/><xdr:cNvPicPr/></xdr:nvPicPr><xdr:blipFill><a:blip r:embed="%s"/></xdr:blipFill>`+
		`<xdr:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1270000" cy="635000"/></a:xfrm></xdr:spPr></xdr:pic><xdr:clientData/></xdr:oneCellAnchor>`, row, descr, rid)
}

func TestXLSXChartsAndPictures(t *testing.T) {
	rels := func(items ...string) string {
		return `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + strings.Join(items, "") + `</Relationships>`
	}
	rel := func(id, typ, target string) string {
		return `<Relationship Id="` + id + `" Type="` + nsRel + typ + `" Target="` + target + `"/>`
	}
	data := xlsx(t, []xsheet{
		{name: "My Data", body: `<sheetData>` + row(1, "Year", "Count") + row(2, "#2021", "#5") + row(3, "#2022", "#7") + row(4, "#2023", "#9") + `</sheetData><drawing r:id="rId1"/>`,
			rels: rel("rId1", "drawing", "../drawings/drawing1.xml")},
		{name: "Chart", chartsheet: true, body: `<sheetViews/><drawing r:id="rId1"/>`, rels: rel("rId1", "drawing", "../drawings/drawing2.xml")},
	}, map[string]string{
		"xl/drawings/drawing1.xml": drawingXML(picAnchor(30, "rId4", "Company logo"), chartAnchor(10, "rId2"), chartAnchor(2, "rId3"), chartAnchor(40, "rId5"),
			`<mc:AlternateContent><mc:Choice Requires="cx1"><xdr:twoCellAnchor><xdr:graphicFrame><a:graphic><a:graphicData><cx:chart xmlns:cx="http://example.com/drawing/chartex" r:id="rId6"/></a:graphicData></a:graphic></xdr:graphicFrame></xdr:twoCellAnchor></mc:Choice><mc:Fallback><xdr:twoCellAnchor><xdr:sp><xdr:txBody><a:p><a:r><a:t>This chart isn't available</a:t></a:r></a:p></xdr:txBody></xdr:sp></xdr:twoCellAnchor></mc:Fallback></mc:AlternateContent>`),
		"xl/drawings/_rels/drawing1.xml.rels": rels(rel("rId2", "chart", "../charts/chart1.xml"), rel("rId3", "chart", "../charts/chart2.xml"), rel("rId4", "image", "../media/image1.png"),
			rel("rId5", "chart", "../charts/chart3.xml"), `<Relationship Id="rId6" Type="http://example.com/relationships/chartEx" Target="../charts/chartEx1.xml"/>`),
		"xl/drawings/drawing2.xml":            drawingXML(chartAnchor(0, "rId1")),
		"xl/drawings/_rels/drawing2.xml.rels": rels(rel("rId1", "chart", "../charts/chart4.xml")),
		"xl/charts/chart1.xml":                barChart,
		"xl/charts/chart2.xml":                refChart,
		"xl/charts/chart3.xml":                `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea><c:lineChart><c:ser><c:val><c:numRef><c:f>Gone!A1:A3</c:f></c:numRef></c:val></c:ser></c:lineChart></c:plotArea></c:chart></c:chartSpace>`,
		"xl/charts/chart4.xml":                scatterChart,
		"xl/charts/chartEx1.xml":              `<cx:chartSpace xmlns:cx="http://example.com/drawing/chartex"/>`,
		"xl/media/image1.png":                 string(pngBytes),
	})
	doc, warns := readXLSX(t, data)
	checkDump(t, doc.Blocks, strings.Join([]string{
		"H1 My Data",
		"Table[RR]{H[Year|Count][2021|5][2022|7][2023|9]}",
		"Table(Count)[RR]{H[|Count][2021|5][2022|7][2023|9]}",
		"Table(Quarterly revenue)[-RR]{H[Quarter|2023|2024][Q1|1,000|1500][Q2||2500][Q3|3,000|3500]}",
		"Fig ![Company logo](res:xlsx/image1.png 100.0pt×50.0pt)",
		"H1 Chart",
		"Table[RR]{H[|Only][0.5|25%][1.5|75%]}",
	}, "\n"))
	if r, ok := doc.Resources.Get("res:xlsx/image1.png"); !ok || r.MediaType != "image/png" {
		t.Errorf("image resource = %+v", r)
	}
	wantWarns := []string{`chart "chart3.xml" has no cached data and was dropped`, "a chart of an unsupported type was dropped"}
	for _, w := range wantWarns {
		if !slices.Contains(warns, w) {
			t.Errorf("missing warning %q in %q", w, warns)
		}
	}
}

func TestXLSXHyperlinks(t *testing.T) {
	data := xlsx(t, []xsheet{{name: "S",
		body: `<sheetData>` + row(1, "Site", "Note") + row(2, "Example", "plain") + `</sheetData><hyperlinks><hyperlink ref="A2" r:id="rId1"/><hyperlink ref="B2" location="'Other'!A1"/></hyperlinks>`,
		rels: `<Relationship Id="rId1" Type="` + nsRel + `hyperlink" Target="https://example.com/x" TargetMode="External"/>`,
	}}, nil)
	doc, _ := readXLSX(t, data)
	checkDump(t, doc.Blocks, "Table[--]{H[Site|Note][[Example](https://example.com/x)|plain]}")
}

func TestXLSXAlignmentAndDate1904(t *testing.T) {
	body := `<sheetData>` + row(1, "Label", "When") +
		`<row r="2"><c r="A2" s="6" t="inlineStr"><is><t>mid</t></is></c><c r="B2" s="1"><v>0</v></c></row>` +
		`<row r="3"><c r="A3" s="6" t="inlineStr"><is><t>dle</t></is></c><c r="B3" s="1"><v>1</v></c></row></sheetData>`
	data := xlsx(t, []xsheet{{name: "S", body: body}}, map[string]string{"xl/styles.xml": testStyles, "workbookPr": `<workbookPr date1904="1"/>`})
	doc, _ := readXLSX(t, data)
	checkDump(t, doc.Blocks, "Table[C-]{H[Label|When][mid|1904-01-01][dle|1904-01-02]}")
}

func TestXLSXMetadata(t *testing.T) {
	core := `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">` +
		`<dc:title>Budget</dc:title><dc:creator>Jane Roe</dc:creator><dc:subject>Plans</dc:subject><cp:keywords>money; plans, 2024</cp:keywords><dc:description>Yearly budget</dc:description><dc:language>lv-LV</dc:language><cp:lastModifiedBy>Someone Else</cp:lastModifiedBy></cp:coreProperties>`
	doc, _ := readXLSX(t, xlsx(t, []xsheet{{name: "S", body: `<sheetData>` + row(1, "x", "y") + `</sheetData>`}}, map[string]string{"docProps/core.xml": core}))
	m := doc.Meta
	if m.Title != "Budget" || m.Subject != "Plans" || m.Summary != "Yearly budget" || m.Lang != "lv-LV" ||
		len(m.Authors) != 1 || m.Authors[0].Name != "Jane Roe" || !slices.Equal(m.Keywords, []string{"money", "plans", "2024"}) {
		t.Errorf("meta = %+v", m)
	}
}

func TestXLSXErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"legacy xls", append(append([]byte{}, oleMagic...), make([]byte, 100)...), "legacy .xls"},
		{"not a zip", []byte("hello"), "not a valid XLSX"},
		{"xlsb", zipBytes(t, map[string]string{"xl/workbook.bin": "x"}), ".xlsb"},
		{"no workbook", zipBytes(t, map[string]string{"foo.txt": "x"}), "not a valid XLSX"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ReadXLSX(context.Background(), tc.data, rd.Options{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestXLSXEmptyAndLimits(t *testing.T) {
	doc, warns := readXLSX(t, xlsx(t, []xsheet{{name: "S", body: `<sheetData><row r="1"><c r="A1" s="0"/></row></sheetData>`}}, nil))
	checkDump(t, doc.Blocks, "P ")
	if len(warns) != 1 || warns[0] != "the workbook contains no data" {
		t.Errorf("warnings = %q", warns)
	}

	var rows strings.Builder
	for r := 1; r <= 50; r++ {
		rows.WriteString(row(r, fmt.Sprintf("#%d", r), "#1"))
	}
	data := xlsx(t, []xsheet{{name: "S", body: `<sheetData>` + rows.String() + `</sheetData>`}}, nil)
	doc, warns, err := ReadXLSX(context.Background(), data, rd.Options{Limits: rd.Limits{MaxTableCells: 20}})
	if err != nil {
		t.Fatal(err)
	}
	if tbl := doc.Blocks[0].(*ast.Table); len(tbl.Body)+len(tbl.Head) != 10 || len(warns) == 0 {
		t.Errorf("rows = %d, warnings = %q", len(tbl.Body)+len(tbl.Head), warns)
	}

	// A stray cell far away must not blow up the grid.
	far := `<sheetData>` + row(1, "a", "b") + `<row r="1048576"><c r="XFD1048576"><v>1</v></c></row></sheetData>`
	doc, _ = readXLSX(t, xlsx(t, []xsheet{{name: "S", body: far}}, nil))
	if len(doc.Blocks) == 0 {
		t.Error("no blocks")
	}
}

func TestCellRef(t *testing.T) {
	tests := []struct {
		ref      string
		col, row int
		ok       bool
	}{
		{"A1", 0, 0, true}, {"B3", 1, 2, true}, {"Z1", 25, 0, true}, {"AA1", 26, 0, true},
		{"AZ10", 51, 9, true}, {"XFD1048576", 16383, 1048575, true}, {"$C$5", 0, 0, false},
		{"A0", 0, 0, false}, {"1A", 0, 0, false}, {"", 0, 0, false}, {"ABCD1", 0, 0, false},
	}
	for _, tc := range tests {
		c, r, ok := cellRef(tc.ref)
		if ok != tc.ok || ok && (c != tc.col || r != tc.row) {
			t.Errorf("cellRef(%q) = %d,%d,%v", tc.ref, c, r, ok)
		}
	}
	if c1, r1, c2, r2, ok := rangeRef("$B$2:$A$1"); !ok || c1 != 0 || r1 != 0 || c2 != 1 || r2 != 1 {
		t.Errorf("rangeRef = %d %d %d %d %v", c1, r1, c2, r2, ok)
	}
}

func FuzzReadXLSX(f *testing.F) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("xl/workbook.xml")
	w.Write([]byte(`<workbook><sheets><sheet name="a" r:id="rId1"/></sheets></workbook>`))
	w, _ = zw.Create("xl/_rels/workbook.xml.rels")
	w.Write([]byte(`<Relationships><Relationship Id="rId1" Type="x/worksheet" Target="s.xml"/></Relationships>`))
	w, _ = zw.Create("xl/s.xml")
	w.Write([]byte(`<worksheet><sheetData><row><c t="s"><v>9</v></c></row></sheetData><mergeCells><mergeCell ref="A1:Z9"/></mergeCells></worksheet>`))
	zw.Close()
	f.Add(buf.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, _, err := ReadXLSX(context.Background(), data, rd.Options{})
		if err == nil && doc == nil {
			t.Fatal("nil document without error")
		}
	})
}
