package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// parseCSVFile reads a CSV file and converts it to a Document via markdown.
// The first row is treated as the header. Delimiter is auto-detected (comma, semicolon, tab).
func parseCSVFile(path, inputDir string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("failed to read CSV file: %w", err)
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return Document{}, fmt.Errorf("CSV file is empty")
	}

	delimiter := detectCSVDelimiter(content)

	reader := csv.NewReader(strings.NewReader(content))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // allow varying row lengths

	records, err := reader.ReadAll()
	if err != nil {
		return Document{}, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return Document{}, fmt.Errorf("CSV file has no data")
	}

	md := csvToMarkdown(records, path)
	return parseMarkdown(md, inputDir), nil
}

// detectCSVDelimiter guesses the delimiter by counting occurrences in the first line.
func detectCSVDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexByte(content, '\n'); idx >= 0 {
		firstLine = content[:idx]
	}

	commas := strings.Count(firstLine, ",")
	semicolons := strings.Count(firstLine, ";")
	tabs := strings.Count(firstLine, "\t")

	if tabs > commas && tabs > semicolons {
		return '\t'
	}
	if semicolons > commas {
		return ';'
	}
	return ','
}

// csvToMarkdown converts CSV records to a markdown string with a table.
func csvToMarkdown(records [][]string, path string) string {
	title := titleFromFilename(path)

	// Find max columns across all rows
	maxCols := 0
	for _, row := range records {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	if maxCols == 0 {
		return fmt.Sprintf("---\ntitle: %s\nstyle: report\n---\n\nNo data.\n", title)
	}

	var md strings.Builder
	md.WriteString(fmt.Sprintf("---\ntitle: %s\nstyle: report\n---\n\n", title))
	md.WriteString("## Data\n\n")

	// Header row
	header := padRow(records[0], maxCols)
	md.WriteString("|")
	for _, cell := range header {
		md.WriteString(" " + strings.TrimSpace(cell) + " |")
	}
	md.WriteString("\n")

	// Separator
	md.WriteString("|")
	for range header {
		md.WriteString(" --- |")
	}
	md.WriteString("\n")

	// Data rows
	for _, row := range records[1:] {
		padded := padRow(row, maxCols)
		md.WriteString("|")
		for _, cell := range padded {
			md.WriteString(" " + strings.TrimSpace(cell) + " |")
		}
		md.WriteString("\n")
	}

	md.WriteString(fmt.Sprintf("\n*%d rows, %d columns*\n", len(records)-1, maxCols))

	return md.String()
}

// padRow ensures a row has exactly n cells, padding with empty strings.
func padRow(row []string, n int) []string {
	if len(row) >= n {
		return row[:n]
	}
	padded := make([]string, n)
	copy(padded, row)
	return padded
}

// titleFromFilename derives a title from a file path.
func titleFromFilename(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Title case: capitalize first letter of each word
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			runes := []rune(w)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}
