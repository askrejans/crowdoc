// Package text reads plain-text documents. Structure is inferred from the
// conventions people use in text files: underlined and ALL-CAPS headings,
// numbered section titles, bullet and numbered lists, indented code, short
// address-like lines, URLs and e-mail addresses, and a leading block of
// "Key: Value" metadata.
package text

import (
	"bytes"
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	xunicode "golang.org/x/text/encoding/unicode"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// Read parses a plain-text document.
func Read(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	doc := &ast.Document{Resources: ast.NewResources()}
	var warn rd.Warnings
	text := normalize(decode(data))
	pages := strings.Split(text, "\f")
	p := &parser{ctx: ctx}
	for i, page := range pages {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		lines := splitLines(page)
		if i == 0 {
			lines = p.metadata(lines, &doc.Meta)
			lines = p.title(lines, &doc.Meta)
		}
		blocks, err := p.blocks(lines)
		if err != nil {
			return nil, nil, err
		}
		if len(blocks) == 0 {
			continue
		}
		if len(doc.Blocks) > 0 {
			doc.Blocks = append(doc.Blocks, &ast.PageBreak{})
		}
		doc.Blocks = append(doc.Blocks, blocks...)
	}
	if len(doc.Blocks) == 0 && doc.Meta.Title == "" {
		warn.Addf("the file contains no text")
	}
	return doc, warn.List(), nil
}

// decode converts raw bytes to UTF-8 text (BOMs, UTF-16 and legacy 8-bit
// code pages are recognised).
func decode(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		b = b[3:]
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return utf16(b, xunicode.LittleEndian)
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return utf16(b, xunicode.BigEndian)
	default:
		if e, ok := utf16Guess(b); ok {
			return utf16(b, e)
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

func utf16(b []byte, e xunicode.Endianness) string {
	out, err := xunicode.UTF16(e, xunicode.UseBOM).NewDecoder().Bytes(b)
	if err != nil {
		return strings.ToValidUTF8(string(b), "\ufffd")
	}
	return string(out)
}

// utf16Guess detects BOM-less UTF-16 from the NUL bytes ASCII text leaves
// in every other position.
func utf16Guess(b []byte) (xunicode.Endianness, bool) {
	n := min(len(b), 4096) &^ 1
	if n < 4 {
		return xunicode.LittleEndian, false
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
		return xunicode.LittleEndian, true
	case even*10 >= pairs*7 && odd*10 < pairs:
		return xunicode.BigEndian, true
	}
	return xunicode.LittleEndian, false
}

// legacyCodePage guesses the code page of non-UTF-8 text: Cyrillic words
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

// normalize unifies line endings and drops control characters other than
// tab, newline and form feed.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\f':
			return r
		case r < 0x20 || r == 0x7f || r == '\ufeff':
			return -1
		}
		return r
	}, s)
}

// line is one source line with its indentation measured in columns.
type line struct {
	raw    string // the line with the common indentation removed
	text   string // trimmed content
	indent int
	blank  bool
}

const tabWidth = 4

func splitLines(page string) []line {
	parts := strings.Split(page, "\n")
	lines := make([]line, len(parts))
	common := -1
	for i, raw := range parts {
		raw = strings.TrimRight(raw, " \t\u00a0")
		text := strings.TrimSpace(raw)
		lines[i] = line{raw: raw, text: text, indent: indentOf(raw), blank: text == ""}
		if !lines[i].blank && (common < 0 || lines[i].indent < common) {
			common = lines[i].indent
		}
	}
	// A uniformly indented file is not one big code block.
	if common > 0 {
		for i := range lines {
			if !lines[i].blank {
				lines[i].raw = dropIndent(lines[i].raw, common)
				lines[i].indent -= common
			}
		}
	}
	return lines
}

func indentOf(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += tabWidth - n%tabWidth
		default:
			return n
		}
	}
	return n
}

// dropIndent removes up to cols columns of leading whitespace.
func dropIndent(s string, cols int) string {
	n := 0
	for i, r := range s {
		if n >= cols {
			return s[i:]
		}
		switch r {
		case ' ':
			n++
		case '\t':
			n += tabWidth - n%tabWidth
		default:
			return s[i:]
		}
	}
	return ""
}

// metaKeys are the "Key: Value" header names recognised at the top.
var metaKeys = map[string]bool{
	"title": true, "author": true, "authors": true, "date": true, "subject": true,
	"keywords": true, "version": true, "status": true,
}

// metadata consumes a leading block of at least two "Key: Value" lines.
func (p *parser) metadata(lines []line, m *ast.Meta) []line {
	start := 0
	for start < len(lines) && lines[start].blank {
		start++
	}
	end := start
	for end < len(lines) {
		k, _, ok := metaLine(lines[end].text)
		if !ok || lines[end].blank || !metaKeys[k] {
			break
		}
		end++
	}
	if end-start < 2 {
		return lines
	}
	for _, l := range lines[start:end] {
		k, v, _ := metaLine(l.text)
		switch k {
		case "title":
			m.Title = v
		case "author", "authors":
			for _, name := range splitAuthors(v) {
				m.Authors = append(m.Authors, ast.Author{Name: name})
			}
		case "date":
			m.Date = v
		case "subject":
			m.Subject = v
		case "keywords":
			for _, kw := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' }) {
				if kw = strings.TrimSpace(kw); kw != "" {
					m.Keywords = append(m.Keywords, kw)
				}
			}
		case "version":
			m.Version = v
		case "status":
			m.Status = v
		}
	}
	return lines[end:]
}

func metaLine(s string) (key, value string, ok bool) {
	k, v, found := strings.Cut(s, ":")
	k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
	if !found || k == "" || v == "" || strings.ContainsAny(k, " \t") {
		return "", "", false
	}
	return k, v, true
}

// splitAuthors splits an author list on ";", " and " / " & ", or on commas
// when every part is a full name (so "Smith, John" stays one author).
func splitAuthors(s string) []string {
	var parts []string
	switch {
	case strings.Contains(s, ";"):
		parts = strings.Split(s, ";")
	case strings.Contains(s, " and ") || strings.Contains(s, " & "):
		parts = strings.FieldsFunc(strings.ReplaceAll(s, " and ", " & "), func(r rune) bool { return r == '&' })
		var out []string
		for _, p := range parts {
			out = append(out, strings.Split(p, ",")...)
		}
		parts = out
	case strings.Contains(s, ","):
		parts = strings.Split(s, ",")
		for _, p := range parts {
			if len(strings.Fields(p)) < 2 {
				parts = []string{s}
				break
			}
		}
	default:
		parts = []string{s}
	}
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// title takes the first line as the document title when it is short, not
// a list item, stands alone (blank line or underline after it) and more
// content follows.
func (p *parser) title(lines []line, m *ast.Meta) []line {
	if m.Title != "" {
		return lines
	}
	i := 0
	for i < len(lines) && lines[i].blank {
		i++
	}
	if i >= len(lines) {
		return lines
	}
	l := lines[i]
	text := l.text
	if lvl, rest := atxHeading(text); lvl > 0 {
		text = rest
	}
	if utf8.RuneCountInString(text) > 100 || text == "" || l.indent >= 4 {
		return lines
	}
	if _, ok := listMarker(l.text); ok {
		return lines
	}
	if last, _ := utf8.DecodeLastRuneInString(text); last == ',' || last == ':' || last == ';' {
		return lines
	}
	next := i + 1
	switch {
	case next < len(lines) && !lines[next].blank && underlineLevel(lines[next].text) > 0:
		next++
	case next >= len(lines) || lines[next].blank:
	default:
		return lines
	}
	rest := lines[next:]
	hasMore := false
	for _, r := range rest {
		if !r.blank {
			hasMore = true
			break
		}
	}
	if !hasMore {
		return lines
	}
	m.Title = collapseSpaces(text)
	return rest
}

func collapseSpaces(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '\t' }), " ")
}

// isAllCapsHeading reports whether s is an ALL-CAPS heading line: at least
// three letters, almost all upper case, no sentence punctuation at the end.
func isAllCapsHeading(s string) bool {
	if utf8.RuneCountInString(s) > 80 {
		return false
	}
	letters, upper := 0, 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) || !unicode.IsLower(r) {
				upper++
			}
		}
	}
	if letters < 3 || upper*10 < letters*9 {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(s)
	return !strings.ContainsRune(".!?,;", last)
}
