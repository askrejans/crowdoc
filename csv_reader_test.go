package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── detectCSVDelimiter ─────────────────────────────────────────────────────

func TestDetectCSVDelimiter_Comma(t *testing.T) {
	got := detectCSVDelimiter("a,b,c\n1,2,3")
	if got != ',' {
		t.Errorf("expected comma, got %q", got)
	}
}

func TestDetectCSVDelimiter_Semicolon(t *testing.T) {
	got := detectCSVDelimiter("a;b;c\n1;2;3")
	if got != ';' {
		t.Errorf("expected semicolon, got %q", got)
	}
}

func TestDetectCSVDelimiter_Tab(t *testing.T) {
	got := detectCSVDelimiter("a\tb\tc\n1\t2\t3")
	if got != '\t' {
		t.Errorf("expected tab, got %q", got)
	}
}

func TestDetectCSVDelimiter_DefaultsToComma(t *testing.T) {
	got := detectCSVDelimiter("hello world")
	if got != ',' {
		t.Errorf("expected comma default, got %q", got)
	}
}

// ─── csvToMarkdown ──────────────────────────────────────────────────────────

func TestCsvToMarkdown_BasicTable(t *testing.T) {
	records := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "Riga"},
		{"Bob", "25", "Liepaja"},
	}
	md := csvToMarkdown(records, "test-data.csv")

	if !strings.Contains(md, "title: Test Data") {
		t.Error("should derive title from filename")
	}
	if !strings.Contains(md, "| Name | Age | City |") {
		t.Error("should contain header row")
	}
	if !strings.Contains(md, "| --- | --- | --- |") {
		t.Error("should contain separator row")
	}
	if !strings.Contains(md, "| Alice | 30 | Riga |") {
		t.Error("should contain data rows")
	}
	if !strings.Contains(md, "*2 rows, 3 columns*") {
		t.Error("should contain row/column count")
	}
}

func TestCsvToMarkdown_SingleColumn(t *testing.T) {
	records := [][]string{{"Items"}, {"Apple"}, {"Banana"}}
	md := csvToMarkdown(records, "items.csv")

	if !strings.Contains(md, "| Items |") {
		t.Error("should handle single column")
	}
}

func TestCsvToMarkdown_VaryingRowLengths(t *testing.T) {
	records := [][]string{
		{"A", "B", "C"},
		{"1"},
		{"x", "y", "z", "extra"},
	}
	md := csvToMarkdown(records, "data.csv")

	// Should pad short rows and truncate long rows
	if !strings.Contains(md, "| --- | --- | --- |") {
		t.Error("should use max columns from all rows")
	}
}

func TestCsvToMarkdown_EmptyRecords(t *testing.T) {
	records := [][]string{{}}
	md := csvToMarkdown(records, "empty.csv")

	if !strings.Contains(md, "No data") {
		t.Error("should handle empty records gracefully")
	}
}

// ─── parseCSVFile ───────────────────────────────────────────────────────────

func TestParseCSVFile_CommaSeparated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	os.WriteFile(path, []byte("Name,Score\nAlice,95\nBob,87\n"), 0644)

	doc, err := parseCSVFile(path, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "Test" {
		t.Errorf("expected title 'Test', got %q", doc.Title)
	}
	if doc.Style != "report" {
		t.Errorf("expected style 'report', got %q", doc.Style)
	}
	if len(doc.Sections) == 0 {
		t.Error("should have at least one section")
	}
}

func TestParseCSVFile_SemicolonDelimited(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "european.csv")
	os.WriteFile(path, []byte("Name;Price;Qty\nWidget;1,50;100\nGadget;2,75;50\n"), 0644)

	doc, err := parseCSVFile(path, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Sections) == 0 {
		t.Error("should parse semicolon-delimited CSV")
	}
}

func TestParseCSVFile_TabDelimited(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.csv")
	os.WriteFile(path, []byte("A\tB\tC\n1\t2\t3\n"), 0644)

	_, err := parseCSVFile(path, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCSVFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.csv")
	os.WriteFile(path, []byte(""), 0644)

	_, err := parseCSVFile(path, dir)
	if err == nil {
		t.Error("should return error for empty CSV")
	}
}

func TestParseCSVFile_NonExistent(t *testing.T) {
	_, err := parseCSVFile("/nonexistent/file.csv", "/tmp")
	if err == nil {
		t.Error("should return error for non-existent file")
	}
}

// ─── titleFromFilename ──────────────────────────────────────────────────────

func TestTitleFromFilename_Dashes(t *testing.T) {
	got := titleFromFilename("quarterly-sales-report.csv")
	if got != "Quarterly Sales Report" {
		t.Errorf("got %q", got)
	}
}

func TestTitleFromFilename_Underscores(t *testing.T) {
	got := titleFromFilename("employee_data.xlsx")
	if got != "Employee Data" {
		t.Errorf("got %q", got)
	}
}

func TestTitleFromFilename_WithPath(t *testing.T) {
	got := titleFromFilename("/some/path/my-file.txt")
	if got != "My File" {
		t.Errorf("got %q", got)
	}
}

// ─── padRow ─────────────────────────────────────────────────────────────────

func TestPadRow_ShortRow(t *testing.T) {
	row := padRow([]string{"a"}, 3)
	if len(row) != 3 {
		t.Errorf("expected 3 cols, got %d", len(row))
	}
	if row[0] != "a" || row[1] != "" || row[2] != "" {
		t.Errorf("unexpected padding: %v", row)
	}
}

func TestPadRow_ExactLength(t *testing.T) {
	row := padRow([]string{"a", "b"}, 2)
	if len(row) != 2 {
		t.Errorf("expected 2 cols, got %d", len(row))
	}
}

func TestPadRow_LongRow(t *testing.T) {
	row := padRow([]string{"a", "b", "c", "d"}, 2)
	if len(row) != 2 {
		t.Errorf("expected 2 cols (truncated), got %d", len(row))
	}
}
