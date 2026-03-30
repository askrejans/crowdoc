package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── parseCellRef ───────────────────────────────────────────────────────────

func TestParseCellRef_A1(t *testing.T) {
	col, row := parseCellRef("A1")
	if col != 1 || row != 1 {
		t.Errorf("A1: got col=%d row=%d, want col=1 row=1", col, row)
	}
}

func TestParseCellRef_B3(t *testing.T) {
	col, row := parseCellRef("B3")
	if col != 2 || row != 3 {
		t.Errorf("B3: got col=%d row=%d, want col=2 row=3", col, row)
	}
}

func TestParseCellRef_Z1(t *testing.T) {
	col, row := parseCellRef("Z1")
	if col != 26 || row != 1 {
		t.Errorf("Z1: got col=%d row=%d, want col=26 row=1", col, row)
	}
}

func TestParseCellRef_AA1(t *testing.T) {
	col, row := parseCellRef("AA1")
	if col != 27 || row != 1 {
		t.Errorf("AA1: got col=%d row=%d, want col=27 row=1", col, row)
	}
}

func TestParseCellRef_AZ10(t *testing.T) {
	col, row := parseCellRef("AZ10")
	if col != 52 || row != 10 {
		t.Errorf("AZ10: got col=%d row=%d, want col=52 row=10", col, row)
	}
}

// ─── colName ────────────────────────────────────────────────────────────────

func TestColName_A(t *testing.T) {
	if got := colName(1); got != "A" {
		t.Errorf("colName(1) = %q, want A", got)
	}
}

func TestColName_Z(t *testing.T) {
	if got := colName(26); got != "Z" {
		t.Errorf("colName(26) = %q, want Z", got)
	}
}

func TestColName_AA(t *testing.T) {
	if got := colName(27); got != "AA" {
		t.Errorf("colName(27) = %q, want AA", got)
	}
}

// ─── cellValue ──────────────────────────────────────────────────────────────

func TestCellValue_SharedString(t *testing.T) {
	ss := []string{"hello", "world"}
	cell := xlsxCell{T: "s", V: "1"}
	if got := cellValue(cell, ss); got != "world" {
		t.Errorf("got %q, want 'world'", got)
	}
}

func TestCellValue_SharedString_OutOfRange(t *testing.T) {
	ss := []string{"hello"}
	cell := xlsxCell{T: "s", V: "99"}
	got := cellValue(cell, ss)
	if got != "99" {
		t.Errorf("got %q, want fallback '99'", got)
	}
}

func TestCellValue_Number(t *testing.T) {
	cell := xlsxCell{V: "42.5"}
	if got := cellValue(cell, nil); got != "42.5" {
		t.Errorf("got %q, want '42.5'", got)
	}
}

func TestCellValue_Boolean_True(t *testing.T) {
	cell := xlsxCell{T: "b", V: "1"}
	if got := cellValue(cell, nil); got != "Yes" {
		t.Errorf("got %q, want 'Yes'", got)
	}
}

func TestCellValue_Boolean_False(t *testing.T) {
	cell := xlsxCell{T: "b", V: "0"}
	if got := cellValue(cell, nil); got != "No" {
		t.Errorf("got %q, want 'No'", got)
	}
}

// ─── createTestXLSX + parseXLSXFile ────────────────────────────────────────

func TestParseXLSXFile_SingleSheet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-report.xlsx")

	sheets := map[string][][]string{
		"Sales": {
			{"Product", "Revenue", "Units"},
			{"Widget A", "1500", "100"},
			{"Widget B", "2300", "150"},
		},
	}

	if err := createTestXLSX(path, sheets); err != nil {
		t.Fatalf("failed to create test XLSX: %v", err)
	}

	doc, err := parseXLSXFile(path, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.Title != "Test Report" {
		t.Errorf("title = %q, want 'Test Report'", doc.Title)
	}
	if doc.Style != "report" {
		t.Errorf("style = %q, want 'report'", doc.Style)
	}
	if len(doc.Sections) == 0 {
		t.Fatal("expected at least one section")
	}
	if doc.Sections[0].Title != "Sales" {
		t.Errorf("section title = %q, want 'Sales'", doc.Sections[0].Title)
	}
}

func TestParseXLSXFile_MultipleSheets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.xlsx")

	sheets := map[string][][]string{
		"Employees": {
			{"Name", "Department"},
			{"Alice", "Engineering"},
		},
		"Revenue": {
			{"Month", "Amount"},
			{"Jan", "50000"},
			{"Feb", "62000"},
		},
	}

	if err := createTestXLSX(path, sheets); err != nil {
		t.Fatalf("failed to create test XLSX: %v", err)
	}

	doc, err := parseXLSXFile(path, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Sections) < 2 {
		t.Errorf("expected 2 sections, got %d", len(doc.Sections))
	}
}

func TestParseXLSXFile_NonExistent(t *testing.T) {
	_, err := parseXLSXFile("/nonexistent.xlsx", "/tmp")
	if err == nil {
		t.Error("should return error for non-existent file")
	}
}

func TestParseXLSXFile_InvalidZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.xlsx")
	os.WriteFile(path, []byte("not a zip file"), 0644)

	_, err := parseXLSXFile(path, dir)
	if err == nil {
		t.Error("should return error for invalid ZIP")
	}
}

func TestParseXLSXFile_EmptySpreadsheet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.xlsx")

	// Create a minimal XLSX with no data
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	writeZipEntry(zw, "[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`)
	writeZipEntry(zw, "xl/workbook.xml", `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/></sheets></workbook>`)
	writeZipEntry(zw, "xl/_rels/workbook.xml.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`)
	writeZipEntry(zw, "xl/worksheets/sheet1.xml", `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData></sheetData></worksheet>`)
	zw.Close()
	f.Close()

	_, err := parseXLSXFile(path, dir)
	if err == nil {
		t.Error("should return error for empty spreadsheet")
	}
}

// ─── Text detection ─────────────────────────────────────────────────────────

func TestIsTextSheet_LongSingleColumn(t *testing.T) {
	rows := [][]string{
		{"This is a very long paragraph of text that exceeds eighty characters and should be detected as text content rather than tabular data."},
		{"Another long paragraph that also exceeds the threshold and should be classified as a text block for proper rendering in the output."},
	}
	if !isTextSheet(rows, 1) {
		t.Error("single-column long-text sheet should be detected as text")
	}
}

func TestIsTextSheet_TabularData(t *testing.T) {
	rows := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "Berlin"},
	}
	if isTextSheet(rows, 3) {
		t.Error("multi-column short data should NOT be detected as text")
	}
}

func TestHasMixedContent(t *testing.T) {
	rows := [][]string{
		{"This is a long paragraph of text content that exceeds eighty characters and should be rendered as a text block, not a table row."},
		{"Name", "Score", "Grade"},
		{"Alice", "95", "A"},
	}
	if !hasMixedContent(rows, 3) {
		t.Error("sheet with both text and table rows should be detected as mixed")
	}
}

func TestCountNonEmpty(t *testing.T) {
	if got := countNonEmpty([]string{"a", "", "b", ""}); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
	if got := countNonEmpty([]string{"", "", ""}); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

// ─── Chart parsing ──────────────────────────────────────────────────────────

func TestParseXLSXFile_WithCharts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chart-test.xlsx")

	sheets := map[string][][]string{
		"Data": {{"Q", "Rev"}, {"Q1", "100"}, {"Q2", "200"}},
	}
	charts := []xlsxTestChart{
		{
			Title:      "Revenue by Quarter",
			ChartType:  "bar",
			Categories: []string{"Q1", "Q2", "Q3", "Q4"},
			Series: []struct {
				Name   string
				Values []string
			}{
				{Name: "Revenue", Values: []string{"142600", "157000", "179800", "193800"}},
			},
		},
	}

	if err := createTestXLSX(path, sheets, charts...); err != nil {
		t.Fatalf("failed to create XLSX with chart: %v", err)
	}

	doc, err := parseXLSXFile(path, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have Data section + chart section
	found := false
	for _, s := range doc.Sections {
		if s.Title == "Revenue by Quarter" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should have a section for the chart")
	}
}

func TestDetectChartType(t *testing.T) {
	tests := []struct {
		xml  string
		want string
	}{
		{"<c:barChart>...</c:barChart>", "Bar Chart"},
		{"<c:lineChart>...</c:lineChart>", "Line Chart"},
		{"<c:pieChart>...</c:pieChart>", "Pie Chart"},
		{"<c:scatterChart>...</c:scatterChart>", "Scatter Chart"},
		{"no chart here", ""},
	}
	for _, tt := range tests {
		if got := detectChartType(tt.xml); got != tt.want {
			t.Errorf("detectChartType(%q) = %q, want %q", tt.xml[:20], got, tt.want)
		}
	}
}

func TestExtractChartTitle(t *testing.T) {
	xml := `<c:title><c:tx><c:rich><a:p><a:r><a:t>Sales Growth</a:t></a:r></a:p></c:rich></c:tx></c:title>`
	if got := extractChartTitle(xml); got != "Sales Growth" {
		t.Errorf("got %q, want 'Sales Growth'", got)
	}
}

func TestExtractChartTitle_Empty(t *testing.T) {
	if got := extractChartTitle("<c:chart></c:chart>"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ─── renderTextSheet ────────────────────────────────────────────────────────

func TestRenderTextSheet(t *testing.T) {
	var md strings.Builder
	rows := [][]string{
		{"This is a long paragraph of text that should be rendered as a text block rather than a table row. It contains detailed information about the project status."},
		{"Another paragraph with details about upcoming milestones and team assignments. This paragraph is also long enough to be treated as text content."},
	}
	renderTextSheet(&md, rows)
	got := md.String()
	if !strings.Contains(got, "long paragraph") {
		t.Error("should render text content as paragraphs")
	}
	if strings.Contains(got, "|") {
		t.Error("should NOT contain table formatting")
	}
}

func TestRenderTextSheet_AllCapsHeading(t *testing.T) {
	var md strings.Builder
	rows := [][]string{
		{"OVERVIEW"},
		{"This is a long paragraph of text that should follow the heading. It provides details about the section and what the reader should expect."},
	}
	renderTextSheet(&md, rows)
	got := md.String()
	if !strings.Contains(got, "### OVERVIEW") {
		t.Error("ALL CAPS short text should become a heading")
	}
}

// ─── renderMixedSheet ───────────────────────────────────────────────────────

func TestRenderMixedSheet(t *testing.T) {
	var md strings.Builder
	rows := [][]string{
		{"This is a long paragraph that introduces a data table below it. It should be rendered as text, not as part of the table that follows."},
		{"Name", "Score", "Grade"},
		{"Alice", "95", "A"},
		{"Bob", "87", "B"},
	}
	renderMixedSheet(&md, rows, 3)
	got := md.String()
	if !strings.Contains(got, "long paragraph") {
		t.Error("should render text paragraphs")
	}
	if !strings.Contains(got, "| Name | Score | Grade |") {
		t.Error("should render table rows")
	}
}

func TestRenderMixedSheet_TextAfterTable(t *testing.T) {
	var md strings.Builder
	rows := [][]string{
		{"Name", "Value"},
		{"Item A", "100"},
		{"This is a concluding paragraph of text that appears after the table and summarizes the key findings from the data presented above in detail."},
	}
	renderMixedSheet(&md, rows, 2)
	got := md.String()
	if !strings.Contains(got, "| Name | Value |") {
		t.Error("should render table")
	}
	if !strings.Contains(got, "concluding paragraph") {
		t.Error("should render text after table")
	}
}

// ─── XLSX with text sheet ───────────────────────────────────────────────────

func TestParseXLSXFile_TextSheet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "text-doc.xlsx")

	sheets := map[string][][]string{
		"Summary": {
			{"This is a long executive summary paragraph that explains the key findings of the report. It contains enough text to exceed the threshold for text detection."},
			{"A second paragraph provides additional context about the methodology used and the scope of the analysis performed during the review period."},
		},
	}

	if err := createTestXLSX(path, sheets); err != nil {
		t.Fatalf("failed to create XLSX: %v", err)
	}

	doc, err := parseXLSXFile(path, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse text content
	if len(doc.Sections) == 0 {
		t.Fatal("should have at least one section")
	}
}

// ─── cellValue extended ─────────────────────────────────────────────────────

func TestCellValue_InlineString(t *testing.T) {
	cell := xlsxCell{T: "inlineStr"}
	cell.IS.T = "inline text"
	if got := cellValue(cell, nil); got != "inline text" {
		t.Errorf("got %q, want 'inline text'", got)
	}
}

func TestCellValue_InlineStringRichText(t *testing.T) {
	cell := xlsxCell{T: "inlineStr"}
	cell.IS.R = []xlsxRun{{T: "hello "}, {T: "world"}}
	if got := cellValue(cell, nil); got != "hello world" {
		t.Errorf("got %q, want 'hello world'", got)
	}
}

func TestCellValue_InlineStringEmpty(t *testing.T) {
	cell := xlsxCell{T: "inlineStr"}
	if got := cellValue(cell, nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestCellValue_EmptyType(t *testing.T) {
	cell := xlsxCell{V: "plain value"}
	if got := cellValue(cell, nil); got != "plain value" {
		t.Errorf("got %q", got)
	}
}

// ─── extractCacheValues ─────────────────────────────────────────────────────

func TestExtractCacheValues_WithPoints(t *testing.T) {
	block := `<c:cat><c:strRef><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt></c:strCache></c:strRef></c:cat>`
	vals := extractCacheValues(block, "c:cat", 5)
	if vals[0] != "Q1" || vals[1] != "Q2" {
		t.Errorf("got %v, want [Q1 Q2 ...]", vals)
	}
}

func TestExtractCacheValues_MissingTag(t *testing.T) {
	block := `<c:ser><c:idx val="0"/></c:ser>`
	vals := extractCacheValues(block, "c:cat", 5)
	allEmpty := true
	for _, v := range vals {
		if v != "" {
			allEmpty = false
		}
	}
	if !allEmpty {
		t.Error("should return empty values for missing tag")
	}
}

// ─── renderChartData edge ───────────────────────────────────────────────────

func TestRenderChartData_EmptySeries(t *testing.T) {
	var md strings.Builder
	chart := xlsxChartInfo{
		Title: "Empty",
		Series: []xlsxChartSeries{
			{Name: "A", Categories: nil, Values: nil},
		},
	}
	renderChartData(&md, chart)
	// Should not panic, should produce minimal output
}

func TestRenderChartData_MultipleSeries(t *testing.T) {
	var md strings.Builder
	chart := xlsxChartInfo{
		Title: "Multi",
		Series: []xlsxChartSeries{
			{Name: "Sales", Categories: []string{"Q1", "Q2"}, Values: []string{"100", "200"}},
			{Name: "Costs", Categories: []string{"Q1", "Q2"}, Values: []string{"80", "150"}},
		},
	}
	renderChartData(&md, chart)
	got := md.String()
	if !strings.Contains(got, "| Sales |") {
		t.Error("should include first series")
	}
	if !strings.Contains(got, "Costs") {
		t.Error("should include second series")
	}
}

// ─── xmlEscape ──────────────────────────────────────────────────────────────

func TestXmlEscape_SpecialChars(t *testing.T) {
	got := xmlEscape(`<tag attr="val">&`)
	if !strings.Contains(got, "&lt;") {
		t.Error("should escape <")
	}
	if !strings.Contains(got, "&gt;") {
		t.Error("should escape >")
	}
	if !strings.Contains(got, "&amp;") {
		t.Error("should escape &")
	}
	if !strings.Contains(got, "&quot;") {
		t.Error("should escape quotes")
	}
}

func TestXmlEscape_PlainText(t *testing.T) {
	got := xmlEscape("hello world")
	if got != "hello world" {
		t.Errorf("plain text should be unchanged, got %q", got)
	}
}
