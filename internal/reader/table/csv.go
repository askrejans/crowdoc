package table

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	stdunicode "unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// ReadCSV parses comma-, semicolon-, tab- or pipe-separated values; the
// delimiter is detected from the data. The first row is the header.
func ReadCSV(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	return readDelimited(ctx, data, o, 0)
}

// ReadTSV parses tab-separated values. The first row is the header.
func ReadTSV(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	return readDelimited(ctx, data, o, '\t')
}

func readDelimited(ctx context.Context, data []byte, o rd.Options, delim rune) (*ast.Document, []string, error) {
	doc := &ast.Document{Resources: ast.NewResources()}
	var warn rd.Warnings
	text := decodeText(data)
	if !strings.Contains(text, "\n") && strings.Contains(text, "\r") {
		text = strings.ReplaceAll(text, "\r", "\n") // classic Mac line endings
	}
	// A leading "sep=;" hint line names the delimiter.
	if line, rest, ok := strings.Cut(text, "\n"); ok || line != "" {
		if l := strings.TrimSpace(line); strings.HasPrefix(strings.ToLower(l), "sep=") && utf8.RuneCountInString(l) == 5 {
			r, _ := utf8.DecodeLastRuneInString(l)
			if delim == 0 && validDelimiter(r) {
				delim = r
			}
			text = rest
		}
	}
	if delim == 0 {
		delim = detectDelimiter(text)
	}
	records, err := parseRecords(ctx, text, delim, o.Limits.Normalized().MaxTableCells, &warn)
	if err != nil {
		return nil, nil, err
	}
	records = trimRecords(records)
	if len(records) == 0 {
		warn.Addf("the file contains no data")
		doc.Blocks = []ast.Block{&ast.Para{}} // nothing to show; the warning explains
		return doc, warn.List(), nil
	}
	doc.Blocks = []ast.Block{denseTable(records)}
	return doc, warn.List(), nil
}

// decodeText converts raw bytes to UTF-8: BOMs are honoured, UTF-16 is
// detected, and legacy 8-bit text is decoded as Windows-1257 (Baltic),
// Windows-1251 (Cyrillic) or Windows-1252.
func decodeText(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		b = b[3:]
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return decodeUTF16(b, unicode.LittleEndian)
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return decodeUTF16(b, unicode.BigEndian)
	default:
		if e, ok := utf16Guess(b); ok {
			return decodeUTF16(b, e)
		}
	}
	if utf8.Valid(b) {
		return string(b)
	}
	cm := charmap.Windows1252
	switch legacyCodePage(b) {
	case 1257:
		cm = charmap.Windows1257
	case 1251:
		cm = charmap.Windows1251
	}
	out, err := cm.NewDecoder().Bytes(b)
	if err != nil {
		return strings.ToValidUTF8(string(b), "\ufffd")
	}
	return string(out)
}

func decodeUTF16(b []byte, e unicode.Endianness) string {
	out, err := unicode.UTF16(e, unicode.UseBOM).NewDecoder().Bytes(b)
	if err != nil {
		return strings.ToValidUTF8(string(b), "\ufffd")
	}
	return string(out)
}

// utf16Guess detects BOM-less UTF-16 from the NUL bytes ASCII text leaves
// in every other position.
func utf16Guess(b []byte) (unicode.Endianness, bool) {
	n := min(len(b), 4096) &^ 1
	if n < 4 {
		return unicode.LittleEndian, false
	}
	even, odd := 0, 0
	for i := 0; i < n; i += 2 {
		if b[i] == 0 {
			even++
		}
		if b[i+1] == 0 {
			odd++
		}
	}
	pairs := n / 2
	switch {
	case odd*10 >= pairs*7 && even*10 < pairs:
		return unicode.LittleEndian, true
	case even*10 >= pairs*7 && odd*10 < pairs:
		return unicode.BigEndian, true
	}
	return unicode.LittleEndian, false
}

// legacyCodePage guesses the code page of non-UTF-8 text. Cyrillic words
// are runs of high bytes; Baltic text is recognised by š/ž, which sit where
// Windows-1252 has the rarely used Icelandic ð/þ.
func legacyCodePage(b []byte) int {
	high, adjacent, baltic := 0, 0, 0
	for i, c := range b {
		if c < 0x80 {
			continue
		}
		high++
		if i > 0 && b[i-1] >= 0x80 || i+1 < len(b) && b[i+1] >= 0x80 {
			adjacent++
		}
		switch c {
		case 0xF0, 0xFE, 0xD0, 0xDE:
			baltic++
		}
	}
	switch {
	case high == 0:
		return 1252
	case adjacent*10 >= high*6:
		return 1251
	case baltic > 0 && baltic*10 >= high:
		return 1257
	}
	return 1252
}

var delimiters = []rune{',', ';', '\t', '|'}

func validDelimiter(r rune) bool {
	return r != '"' && r != '\r' && r != '\n' && r != utf8.RuneError && r != 0 && !stdunicode.IsLetter(r) && !stdunicode.IsDigit(r)
}

// detectDelimiter picks the delimiter that splits the first records into
// the most consistent number of fields.
func detectDelimiter(text string) rune {
	sample := text
	cut := false
	if len(sample) > 64<<10 {
		sample, cut = sample[:64<<10], true
	}
	best, bestScore, bestFields := ',', -1.0, 0
	for _, d := range delimiters {
		r := newCSVReader(sample, d)
		counts := map[int]int{}
		total := 0
		var last int
		for total < 50 {
			rec, err := r.Read()
			if err != nil {
				break
			}
			counts[len(rec)]++
			last = len(rec)
			total++
		}
		if cut && total > 1 {
			counts[last]-- // the sample may end mid-record
			total--
		}
		mode, freq := 0, 0
		for n, c := range counts {
			if c > freq || c == freq && n > mode {
				mode, freq = n, c
			}
		}
		if total == 0 || mode < 2 {
			continue
		}
		score := float64(freq) / float64(total)
		if score > bestScore || score == bestScore && mode > bestFields {
			best, bestScore, bestFields = d, score, mode
		}
	}
	return best
}

func newCSVReader(text string, delim rune) *csv.Reader {
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	// Leading-space trimming would also swallow empty tab-separated fields.
	r.TrimLeadingSpace = delim != '\t'
	return r
}

func parseRecords(ctx context.Context, text string, delim rune, maxCells int, warn *rd.Warnings) ([][]string, error) {
	r := newCSVReader(text, delim)
	var records [][]string
	cells := 0
	for {
		if len(records)%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			line := 0
			if pe := (*csv.ParseError)(nil); errors.As(err, &pe) {
				line = pe.Line
			}
			warn.Addf("the data could not be parsed after line %d; the rest was dropped", line)
			break
		}
		if cells += len(rec); cells > maxCells {
			warn.Addf("the table has more than %d cells; it was truncated to %d rows", maxCells, len(records))
			break
		}
		records = append(records, rec)
	}
	return records, nil
}

// trimRecords trims cell whitespace, pads ragged rows and drops trailing
// empty rows and columns.
func trimRecords(records [][]string) [][]string {
	width := 0
	for _, rec := range records {
		for j := range rec {
			rec[j] = strings.TrimSpace(rec[j])
			if rec[j] != "" {
				width = max(width, j+1)
			}
		}
	}
	last := -1
	for i, rec := range records {
		for _, c := range rec {
			if c != "" {
				last = i
				break
			}
		}
	}
	records = records[:last+1]
	for i, rec := range records {
		switch {
		case len(rec) > width:
			records[i] = rec[:width]
		case len(rec) < width:
			records[i] = append(rec, make([]string, width-len(rec))...)
		}
	}
	return records
}

// denseTable builds a table whose first row is the header.
func denseTable(records [][]string) *ast.Table {
	g := &grid{cells: make([][]gridCell, len(records)), origRow: make([]int, len(records))}
	for i, rec := range records {
		g.origRow[i] = i
		row := make([]gridCell, len(rec))
		for j, v := range rec {
			row[j] = gridCell{text: v, rowSpan: 1, colSpan: 1}
		}
		g.cells[i] = row
	}
	tbl := g.table(0, len(records), 1)
	if len(records) == 1 {
		tbl.Head, tbl.Body = tbl.Body, nil
	}
	return tbl
}
