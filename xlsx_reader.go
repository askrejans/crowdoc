package main

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// parseXLSXFile reads an XLSX file and converts it to a Document via markdown.
// Each sheet becomes a section. Supports tables, text blocks, charts, and images.
// Parsed using stdlib only (archive/zip + encoding/xml).
func parseXLSXFile(path, inputDir string) (Document, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Document{}, fmt.Errorf("failed to open XLSX file: %w", err)
	}
	defer zr.Close()

	// Parse shared strings
	sharedStrings, err := parseSharedStrings(zr)
	if err != nil {
		return Document{}, err
	}

	// Parse workbook for sheet names and relationship IDs
	sheets, err := parseWorkbook(zr)
	if err != nil {
		return Document{}, err
	}

	// Parse workbook relationships to map rId -> file path
	rels, err := parseWorkbookRels(zr)
	if err != nil {
		// Fallback: try default sheet paths
		rels = make(map[string]string)
	}

	title := titleFromFilename(path)

	var md strings.Builder
	md.WriteString(fmt.Sprintf("---\ntitle: %s\nstyle: report\n---\n\n", title))

	sheetsProcessed := 0

	for _, sheet := range sheets {
		// Resolve sheet file path from relationships
		sheetPath := ""
		if target, ok := rels[sheet.RID]; ok {
			sheetPath = "xl/" + target
		} else {
			// Fallback: try common path patterns
			sheetPath = fmt.Sprintf("xl/worksheets/sheet%d.xml", sheet.Index)
		}

		rows, err := parseWorksheet(zr, sheetPath, sharedStrings)
		if err != nil {
			continue // skip unreadable sheets
		}

		if len(rows) == 0 {
			continue
		}

		sheetsProcessed++
		md.WriteString(fmt.Sprintf("## %s\n\n", sheet.Name))

		// Render sheet content based on layout: text-heavy or tabular
		renderSheetContent(&md, rows)
	}

	// Extract chart data from xl/charts/*.xml
	charts := parseCharts(zr)
	for _, chart := range charts {
		sheetsProcessed++
		md.WriteString(fmt.Sprintf("## %s\n\n", chart.Title))
		if chart.ChartType != "" {
			md.WriteString(fmt.Sprintf("*Chart type: %s*\n\n", chart.ChartType))
		}
		if len(chart.Series) > 0 {
			renderChartData(&md, chart)
		}
	}

	// Extract embedded images from xl/media/*
	images := extractImages(zr, inputDir)
	if len(images) > 0 {
		for _, img := range images {
			md.WriteString(fmt.Sprintf("![%s](%s)\n\n", img.name, img.path))
		}
	}

	if sheetsProcessed == 0 {
		return Document{}, fmt.Errorf("XLSX file has no readable data")
	}

	return parseMarkdown(md.String(), inputDir), nil
}

// renderSheetContent decides whether to render rows as a table or text paragraphs.
// A row is "text" if it uses only column A and the cell is 80+ characters.
func renderSheetContent(md *strings.Builder, rows [][]string) {
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	if maxCols == 0 {
		md.WriteString("*Empty sheet*\n\n")
		return
	}

	// Detect text-heavy sheets: single column where most cells are long paragraphs
	if isTextSheet(rows, maxCols) {
		renderTextSheet(md, rows)
		return
	}

	// Mixed mode: check each row — long single-cell rows become paragraphs,
	// short multi-cell rows become table rows
	if hasMixedContent(rows, maxCols) {
		renderMixedSheet(md, rows, maxCols)
		return
	}

	// Standard table rendering
	renderTableSheet(md, rows, maxCols)
}

// isTextSheet returns true if the sheet is primarily text paragraphs in column A.
func isTextSheet(rows [][]string, maxCols int) bool {
	if maxCols > 2 {
		return false
	}
	textRows := 0
	for _, row := range rows {
		nonEmpty := countNonEmpty(row)
		if nonEmpty == 1 && len(strings.TrimSpace(row[0])) >= 80 {
			textRows++
		}
	}
	return textRows > len(rows)/2
}

// hasMixedContent returns true if a sheet has both text paragraphs and table rows.
func hasMixedContent(rows [][]string, maxCols int) bool {
	hasText := false
	hasTable := false
	for _, row := range rows {
		nonEmpty := countNonEmpty(row)
		if nonEmpty == 1 && len(strings.TrimSpace(row[0])) >= 80 {
			hasText = true
		} else if nonEmpty > 1 {
			hasTable = true
		}
	}
	return hasText && hasTable
}

func countNonEmpty(row []string) int {
	n := 0
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			n++
		}
	}
	return n
}

// renderTextSheet renders single-column text rows as paragraphs.
func renderTextSheet(md *strings.Builder, rows [][]string) {
	for _, row := range rows {
		text := ""
		if len(row) > 0 {
			text = strings.TrimSpace(row[0])
		}
		if text == "" {
			continue
		}
		// Short bold lines might be sub-headings
		if len(text) < 60 && !strings.Contains(text, " ") || isAllCapsHeading(text) {
			md.WriteString("### " + text + "\n\n")
		} else {
			md.WriteString(text + "\n\n")
		}
	}
}

// renderMixedSheet renders a sheet with both text blocks and table segments.
func renderMixedSheet(md *strings.Builder, rows [][]string, maxCols int) {
	inTable := false
	tableStart := 0

	for i, row := range rows {
		nonEmpty := countNonEmpty(row)
		isText := nonEmpty <= 1 && len(row) > 0 && len(strings.TrimSpace(row[0])) >= 80

		if isText {
			// Flush any pending table
			if inTable {
				renderTableSheet(md, rows[tableStart:i], maxCols)
				inTable = false
			}
			text := strings.TrimSpace(row[0])
			md.WriteString(text + "\n\n")
		} else if nonEmpty > 0 {
			if !inTable {
				tableStart = i
				inTable = true
			}
		}
	}
	// Flush remaining table
	if inTable {
		renderTableSheet(md, rows[tableStart:], maxCols)
	}
}

// renderTableSheet renders rows as a markdown table.
func renderTableSheet(md *strings.Builder, rows [][]string, maxCols int) {
	if len(rows) == 0 {
		return
	}

	// Header row
	header := padRow(rows[0], maxCols)
	md.WriteString("|")
	for _, cell := range header {
		md.WriteString(" " + cell + " |")
	}
	md.WriteString("\n")

	// Separator
	md.WriteString("|")
	for range header {
		md.WriteString(" --- |")
	}
	md.WriteString("\n")

	// Data rows
	for _, row := range rows[1:] {
		padded := padRow(row, maxCols)
		md.WriteString("|")
		for _, cell := range padded {
			md.WriteString(" " + cell + " |")
		}
		md.WriteString("\n")
	}

	md.WriteString(fmt.Sprintf("\n*%d rows, %d columns*\n\n", len(rows)-1, maxCols))
}

// --- Chart extraction ---

type xlsxChartInfo struct {
	Title     string
	ChartType string
	Series    []xlsxChartSeries
}

type xlsxChartSeries struct {
	Name       string
	Categories []string
	Values     []string
}

// parseCharts extracts chart data from xl/charts/*.xml files.
func parseCharts(zr *zip.ReadCloser) []xlsxChartInfo {
	var charts []xlsxChartInfo

	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "xl/charts/chart") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		chart := parseChartXML(data)
		if chart.Title != "" || len(chart.Series) > 0 {
			if chart.Title == "" {
				chart.Title = "Chart"
			}
			charts = append(charts, chart)
		}
	}

	return charts
}

// parseChartXML extracts title, type, and series data from chart XML.
func parseChartXML(data []byte) xlsxChartInfo {
	content := string(data)
	info := xlsxChartInfo{}

	// Extract chart title — look for <c:t> inside <c:title>
	info.Title = extractChartTitle(content)

	// Detect chart type
	info.ChartType = detectChartType(content)

	// Extract series data
	info.Series = extractChartSeries(content)

	return info
}

func extractChartTitle(content string) string {
	// Look for text inside <c:title>...<a:t>Title</a:t>...</c:title>
	// or <c:title>...<c:v>Title</c:v>...</c:title>
	titleStart := strings.Index(content, "<c:title>")
	if titleStart < 0 {
		return ""
	}
	titleEnd := strings.Index(content[titleStart:], "</c:title>")
	if titleEnd < 0 {
		return ""
	}
	titleBlock := content[titleStart : titleStart+titleEnd]

	// Try <a:t> first (rich text title)
	if idx := strings.Index(titleBlock, "<a:t>"); idx >= 0 {
		end := strings.Index(titleBlock[idx:], "</a:t>")
		if end > 5 {
			return titleBlock[idx+5 : idx+end]
		}
	}
	// Try <c:v> (reference title)
	if idx := strings.Index(titleBlock, "<c:v>"); idx >= 0 {
		end := strings.Index(titleBlock[idx:], "</c:v>")
		if end > 5 {
			return titleBlock[idx+5 : idx+end]
		}
	}
	return ""
}

func detectChartType(content string) string {
	types := []struct {
		tag  string
		name string
	}{
		{"<c:barChart>", "Bar Chart"},
		{"<c:bar3DChart>", "3D Bar Chart"},
		{"<c:lineChart>", "Line Chart"},
		{"<c:line3DChart>", "3D Line Chart"},
		{"<c:pieChart>", "Pie Chart"},
		{"<c:pie3DChart>", "3D Pie Chart"},
		{"<c:areaChart>", "Area Chart"},
		{"<c:scatterChart>", "Scatter Chart"},
		{"<c:doughnutChart>", "Doughnut Chart"},
		{"<c:radarChart>", "Radar Chart"},
	}
	for _, t := range types {
		if strings.Contains(content, t.tag) {
			return t.name
		}
	}
	return ""
}

func extractChartSeries(content string) []xlsxChartSeries {
	var series []xlsxChartSeries

	// Find all <c:ser>...</c:ser> blocks
	remaining := content
	for {
		start := strings.Index(remaining, "<c:ser>")
		if start < 0 {
			break
		}
		end := strings.Index(remaining[start:], "</c:ser>")
		if end < 0 {
			break
		}
		serBlock := remaining[start : start+end+len("</c:ser>")]
		remaining = remaining[start+end+len("</c:ser>"):]

		s := xlsxChartSeries{}

		// Series name from <c:tx>...<c:v>Name</c:v>
		s.Name = extractCacheValues(serBlock, "c:tx", 1)[0]

		// Categories from <c:cat>
		cats := extractCacheValues(serBlock, "c:cat", 20)
		for _, c := range cats {
			if c != "" {
				s.Categories = append(s.Categories, c)
			}
		}

		// Values from <c:val>
		vals := extractCacheValues(serBlock, "c:val", 20)
		for _, v := range vals {
			if v != "" {
				s.Values = append(s.Values, v)
			}
		}

		if s.Name != "" || len(s.Categories) > 0 || len(s.Values) > 0 {
			series = append(series, s)
		}
	}

	return series
}

// extractCacheValues pulls <c:v> values from inside a parent tag's cache.
func extractCacheValues(block, parentTag string, maxValues int) []string {
	result := make([]string, maxValues)

	start := strings.Index(block, "<"+parentTag+">")
	if start < 0 {
		// Also try self-nesting like <c:tx> which may contain <c:strRef>
		start = strings.Index(block, "<"+parentTag)
		if start < 0 {
			return result
		}
	}
	endTag := "</" + parentTag + ">"
	end := strings.Index(block[start:], endTag)
	if end < 0 {
		return result
	}
	section := block[start : start+end]

	// Extract all <c:v>...</c:v> values with their idx
	remaining := section
	for {
		ptIdx := strings.Index(remaining, "<c:pt ")
		if ptIdx < 0 {
			// Try plain <c:v> without <c:pt> wrapper
			vIdx := strings.Index(remaining, "<c:v>")
			if vIdx < 0 {
				break
			}
			vEnd := strings.Index(remaining[vIdx:], "</c:v>")
			if vEnd < 0 {
				break
			}
			val := remaining[vIdx+5 : vIdx+vEnd]
			// Put in first empty slot
			for i := range result {
				if result[i] == "" {
					result[i] = val
					break
				}
			}
			remaining = remaining[vIdx+vEnd+6:]
			continue
		}

		// Parse idx attribute
		idxAttr := ""
		attrStart := strings.Index(remaining[ptIdx:], `idx="`)
		if attrStart >= 0 {
			attrVal := remaining[ptIdx+attrStart+5:]
			attrEnd := strings.Index(attrVal, `"`)
			if attrEnd > 0 {
				idxAttr = attrVal[:attrEnd]
			}
		}

		// Get value
		vStart := strings.Index(remaining[ptIdx:], "<c:v>")
		if vStart < 0 {
			remaining = remaining[ptIdx+6:]
			continue
		}
		vEnd := strings.Index(remaining[ptIdx+vStart:], "</c:v>")
		if vEnd < 0 {
			remaining = remaining[ptIdx+6:]
			continue
		}
		val := remaining[ptIdx+vStart+5 : ptIdx+vStart+vEnd]

		idx, err := strconv.Atoi(idxAttr)
		if err == nil && idx >= 0 && idx < maxValues {
			result[idx] = val
		}

		remaining = remaining[ptIdx+vStart+vEnd+6:]
	}

	return result
}

// renderChartData renders chart series as a markdown table.
func renderChartData(md *strings.Builder, chart xlsxChartInfo) {
	// Build a table: first column = categories, then one column per series
	maxLen := 0
	for _, s := range chart.Series {
		if len(s.Categories) > maxLen {
			maxLen = len(s.Categories)
		}
		if len(s.Values) > maxLen {
			maxLen = len(s.Values)
		}
	}

	if maxLen == 0 {
		return
	}

	// Use categories from first series that has them
	var categories []string
	for _, s := range chart.Series {
		if len(s.Categories) > 0 {
			categories = s.Categories
			break
		}
	}

	// Header
	md.WriteString("| Category |")
	for _, s := range chart.Series {
		name := s.Name
		if name == "" {
			name = "Series"
		}
		md.WriteString(" " + name + " |")
	}
	md.WriteString("\n| --- |")
	for range chart.Series {
		md.WriteString(" --- |")
	}
	md.WriteString("\n")

	// Data rows
	for i := 0; i < maxLen; i++ {
		cat := ""
		if i < len(categories) {
			cat = categories[i]
		} else {
			cat = fmt.Sprintf("%d", i+1)
		}
		md.WriteString("| " + cat + " |")
		for _, s := range chart.Series {
			val := ""
			if i < len(s.Values) {
				val = s.Values[i]
			}
			md.WriteString(" " + val + " |")
		}
		md.WriteString("\n")
	}
	md.WriteString("\n")
}

// --- Image extraction ---

type xlsxImage struct {
	name string
	path string
}

// extractImages saves embedded images from xl/media/ to inputDir and returns paths.
func extractImages(zr *zip.ReadCloser, inputDir string) []xlsxImage {
	var images []xlsxImage

	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "xl/media/") {
			continue
		}

		// Only extract common image formats
		lower := strings.ToLower(f.Name)
		if !strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".jpg") &&
			!strings.HasSuffix(lower, ".jpeg") && !strings.HasSuffix(lower, ".gif") &&
			!strings.HasSuffix(lower, ".bmp") {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		// Save to inputDir
		baseName := f.Name[strings.LastIndex(f.Name, "/")+1:]
		outPath := inputDir + "/" + baseName
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			continue
		}

		name := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		images = append(images, xlsxImage{name: name, path: outPath})
	}

	return images
}

// --- XLSX XML types ---

type xlsxWorkbook struct {
	Sheets struct {
		Sheet []xlsxSheet `xml:"sheet"`
	} `xml:"sheets"`
}

type xlsxSheet struct {
	Name  string `xml:"name,attr"`
	ID    int    `xml:"sheetId,attr"`
	RID   string `xml:"id,attr"`
	Index int    // 1-based order
}

type xlsxSST struct {
	SI []xlsxSI `xml:"si"`
}

type xlsxSI struct {
	T string   `xml:"t"`
	R []xlsxRun `xml:"r"`
}

type xlsxRun struct {
	T string `xml:"t"`
}

type xlsxWorksheet struct {
	SheetData struct {
		Row []xlsxRow `xml:"row"`
	} `xml:"sheetData"`
}

type xlsxRow struct {
	R    int        `xml:"r,attr"`
	C    []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	R  string `xml:"r,attr"`
	T  string `xml:"t,attr"`
	V  string `xml:"v"`
	IS struct {
		T string `xml:"t"`
		R []xlsxRun `xml:"r"`
	} `xml:"is"`
}

type xlsxRelationships struct {
	Rel []xlsxRel `xml:"Relationship"`
}

type xlsxRel struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

// --- Parsing functions ---

func readZipFile(zr *zip.ReadCloser, name string) ([]byte, error) {
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, name) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file not found in archive: %s", name)
}

func parseSharedStrings(zr *zip.ReadCloser) ([]string, error) {
	data, err := readZipFile(zr, "xl/sharedStrings.xml")
	if err != nil {
		// Not all XLSX files have shared strings
		return nil, nil
	}

	var sst xlsxSST
	if err := xml.Unmarshal(data, &sst); err != nil {
		return nil, fmt.Errorf("failed to parse shared strings: %w", err)
	}

	result := make([]string, len(sst.SI))
	for i, si := range sst.SI {
		if si.T != "" {
			result[i] = si.T
		} else if len(si.R) > 0 {
			// Rich text: concatenate runs
			var parts []string
			for _, r := range si.R {
				parts = append(parts, r.T)
			}
			result[i] = strings.Join(parts, "")
		}
	}
	return result, nil
}

func parseWorkbook(zr *zip.ReadCloser) ([]xlsxSheet, error) {
	data, err := readZipFile(zr, "xl/workbook.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to read workbook: %w", err)
	}

	var wb xlsxWorkbook
	if err := xml.Unmarshal(data, &wb); err != nil {
		return nil, fmt.Errorf("failed to parse workbook: %w", err)
	}

	sheets := wb.Sheets.Sheet
	for i := range sheets {
		sheets[i].Index = i + 1
	}
	return sheets, nil
}

func parseWorkbookRels(zr *zip.ReadCloser) (map[string]string, error) {
	data, err := readZipFile(zr, "xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}

	var rels xlsxRelationships
	if err := xml.Unmarshal(data, &rels); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, r := range rels.Rel {
		if strings.Contains(r.Type, "worksheet") {
			result[r.ID] = r.Target
		}
	}
	return result, nil
}

func parseWorksheet(zr *zip.ReadCloser, path string, sharedStrings []string) ([][]string, error) {
	data, err := readZipFile(zr, path)
	if err != nil {
		return nil, err
	}

	var ws xlsxWorksheet
	if err := xml.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("failed to parse worksheet: %w", err)
	}

	if len(ws.SheetData.Row) == 0 {
		return nil, nil
	}

	// Parse rows into a map (row number -> col number -> value)
	type cellPos struct {
		row, col int
	}
	cells := make(map[cellPos]string)
	maxRow, maxCol := 0, 0

	for _, row := range ws.SheetData.Row {
		for _, cell := range row.C {
			col, rowNum := parseCellRef(cell.R)
			if rowNum == 0 && row.R > 0 {
				rowNum = row.R
			}

			val := cellValue(cell, sharedStrings)
			if val == "" {
				continue
			}

			cells[cellPos{rowNum, col}] = val
			if rowNum > maxRow {
				maxRow = rowNum
			}
			if col > maxCol {
				maxCol = col
			}
		}
	}

	if maxRow == 0 || maxCol == 0 {
		return nil, nil
	}

	// Convert to 2D string array (skip fully empty rows)
	var rows [][]string
	for r := 1; r <= maxRow; r++ {
		row := make([]string, maxCol)
		hasData := false
		for c := 1; c <= maxCol; c++ {
			if v, ok := cells[cellPos{r, c}]; ok {
				row[c-1] = v
				hasData = true
			}
		}
		if hasData {
			rows = append(rows, row)
		}
	}

	return rows, nil
}

// parseCellRef converts "B3" -> (col=2, row=3). Returns 1-based indices.
func parseCellRef(ref string) (col, row int) {
	col = 0
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		col = col*26 + int(ref[i]-'A') + 1
		i++
	}
	row, _ = strconv.Atoi(ref[i:])
	return col, row
}

// cellValue extracts the display value from a cell.
func cellValue(cell xlsxCell, sharedStrings []string) string {
	switch cell.T {
	case "s": // shared string
		idx, err := strconv.Atoi(cell.V)
		if err != nil || idx < 0 || idx >= len(sharedStrings) {
			return cell.V
		}
		return sharedStrings[idx]
	case "inlineStr":
		if cell.IS.T != "" {
			return cell.IS.T
		}
		if len(cell.IS.R) > 0 {
			var parts []string
			for _, r := range cell.IS.R {
				parts = append(parts, r.T)
			}
			return strings.Join(parts, "")
		}
		return ""
	case "b": // boolean
		if cell.V == "1" {
			return "Yes"
		}
		return "No"
	default:
		return cell.V
	}
}

// xlsxTestChart defines chart data for createTestXLSX.
type xlsxTestChart struct {
	Title      string
	ChartType  string // "bar", "line", "pie"
	Categories []string
	Series     []struct {
		Name   string
		Values []string
	}
}

// createTestXLSX creates a minimal valid XLSX file for testing.
// This is used by tests only — it creates a proper XLSX (zip of XML files).
// Optional charts parameter adds chart XML files to the archive.
func createTestXLSX(path string, sheets map[string][][]string, charts ...xlsxTestChart) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Sort sheet names for deterministic output
	sheetNames := make([]string, 0, len(sheets))
	for name := range sheets {
		sheetNames = append(sheetNames, name)
	}
	sort.Strings(sheetNames)

	// [Content_Types].xml
	ctXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`
	for i := range sheetNames {
		ctXML += fmt.Sprintf(`
  <Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
	}
	ctXML += `
  <Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
</Types>`
	writeZipEntry(zw, "[Content_Types].xml", ctXML)

	// _rels/.rels
	writeZipEntry(zw, "_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`)

	// Collect all unique strings for shared strings table
	var allStrings []string
	stringIndex := make(map[string]int)

	for _, name := range sheetNames {
		for _, row := range sheets[name] {
			for _, cell := range row {
				if _, exists := stringIndex[cell]; !exists {
					stringIndex[cell] = len(allStrings)
					allStrings = append(allStrings, cell)
				}
			}
		}
	}

	// xl/sharedStrings.xml
	var sstXML strings.Builder
	sstXML.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sstXML.WriteString(fmt.Sprintf(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`, len(allStrings), len(allStrings)))
	for _, s := range allStrings {
		sstXML.WriteString("<si><t>")
		sstXML.WriteString(xmlEscape(s))
		sstXML.WriteString("</t></si>")
	}
	sstXML.WriteString("</sst>")
	writeZipEntry(zw, "xl/sharedStrings.xml", sstXML.String())

	// xl/workbook.xml
	var wbXML strings.Builder
	wbXML.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	wbXML.WriteString(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	wbXML.WriteString("<sheets>")
	for i, name := range sheetNames {
		wbXML.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, xmlEscape(name), i+1, i+1))
	}
	wbXML.WriteString("</sheets></workbook>")
	writeZipEntry(zw, "xl/workbook.xml", wbXML.String())

	// xl/_rels/workbook.xml.rels
	var relsXML strings.Builder
	relsXML.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	relsXML.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := range sheetNames {
		relsXML.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1))
	}
	relsXML.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>`, len(sheetNames)+1))
	relsXML.WriteString("</Relationships>")
	writeZipEntry(zw, "xl/_rels/workbook.xml.rels", relsXML.String())

	// Worksheets
	for i, name := range sheetNames {
		rows := sheets[name]
		var wsXML strings.Builder
		wsXML.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
		wsXML.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
		wsXML.WriteString("<sheetData>")
		for r, row := range rows {
			wsXML.WriteString(fmt.Sprintf(`<row r="%d">`, r+1))
			for c, cell := range row {
				ref := colName(c+1) + strconv.Itoa(r+1)
				idx := stringIndex[cell]
				wsXML.WriteString(fmt.Sprintf(`<c r="%s" t="s"><v>%d</v></c>`, ref, idx))
			}
			wsXML.WriteString("</row>")
		}
		wsXML.WriteString("</sheetData></worksheet>")
		writeZipEntry(zw, fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), wsXML.String())
	}

	// Charts (optional)
	for i, chart := range charts {
		chartTag := "c:barChart"
		switch chart.ChartType {
		case "line":
			chartTag = "c:lineChart"
		case "pie":
			chartTag = "c:pieChart"
		}

		var cXML strings.Builder
		cXML.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
		cXML.WriteString(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`)
		cXML.WriteString(`<c:chart>`)

		// Title
		if chart.Title != "" {
			cXML.WriteString(`<c:title><c:tx><c:rich><a:p><a:r><a:t>`)
			cXML.WriteString(xmlEscape(chart.Title))
			cXML.WriteString(`</a:t></a:r></a:p></c:rich></c:tx></c:title>`)
		}

		cXML.WriteString(`<c:plotArea>`)
		cXML.WriteString(fmt.Sprintf(`<%s>`, chartTag))

		// Series
		for si, ser := range chart.Series {
			cXML.WriteString(fmt.Sprintf(`<c:ser><c:idx val="%d"/>`, si))

			// Series name
			cXML.WriteString(`<c:tx><c:strRef><c:strCache>`)
			cXML.WriteString(`<c:ptCount val="1"/>`)
			cXML.WriteString(fmt.Sprintf(`<c:pt idx="0"><c:v>%s</c:v></c:pt>`, xmlEscape(ser.Name)))
			cXML.WriteString(`</c:strCache></c:strRef></c:tx>`)

			// Categories
			if len(chart.Categories) > 0 {
				cXML.WriteString(`<c:cat><c:strRef><c:strCache>`)
				cXML.WriteString(fmt.Sprintf(`<c:ptCount val="%d"/>`, len(chart.Categories)))
				for ci, cat := range chart.Categories {
					cXML.WriteString(fmt.Sprintf(`<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, ci, xmlEscape(cat)))
				}
				cXML.WriteString(`</c:strCache></c:strRef></c:cat>`)
			}

			// Values
			cXML.WriteString(`<c:val><c:numRef><c:numCache>`)
			cXML.WriteString(fmt.Sprintf(`<c:ptCount val="%d"/>`, len(ser.Values)))
			for vi, val := range ser.Values {
				cXML.WriteString(fmt.Sprintf(`<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, vi, val))
			}
			cXML.WriteString(`</c:numCache></c:numRef></c:val>`)

			cXML.WriteString(`</c:ser>`)
		}

		cXML.WriteString(fmt.Sprintf(`</%s>`, chartTag))
		cXML.WriteString(`</c:plotArea>`)
		cXML.WriteString(`</c:chart></c:chartSpace>`)

		writeZipEntry(zw, fmt.Sprintf("xl/charts/chart%d.xml", i+1), cXML.String())
	}

	return nil
}

func writeZipEntry(zw *zip.Writer, name, content string) {
	w, _ := zw.Create(name)
	w.Write([]byte(content))
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func colName(col int) string {
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+col%26)) + name
		col /= 26
	}
	return name
}
