package text

import (
	"context"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

type parser struct {
	ctx   context.Context
	lines int
}

// blocks parses a page of lines into blocks.
func (p *parser) blocks(lines []line) ([]ast.Block, error) {
	var out []ast.Block
	for i := 0; i < len(lines); {
		if p.lines += 1; p.lines%1024 == 0 {
			if err := p.ctx.Err(); err != nil {
				return nil, err
			}
		}
		l := lines[i]
		if l.blank {
			i++
			continue
		}
		prevBlank := i == 0 || lines[i-1].blank
		nextBlank := i+1 >= len(lines) || lines[i+1].blank
		if lvl, rest := atxHeading(l.text); lvl > 0 && l.indent < 4 {
			out = append(out, heading(lvl, rest))
			i++
			continue
		}
		if l.indent < 4 && i+1 < len(lines) && !lines[i+1].blank && underlineLevel(lines[i+1].text) > 0 && headingText(l) {
			out = append(out, heading(underlineLevel(lines[i+1].text), l.text))
			i += 2
			continue
		}
		if prevBlank && isRule(l.text) {
			out = append(out, &ast.HorizontalRule{})
			i++
			continue
		}
		if l.indent >= 4 && prevBlank {
			b, n := p.indented(lines[i:])
			out = append(out, b...)
			i += n
			continue
		}
		if prevBlank {
			if lvl, title, ok := numberedHeading(lines, i); ok {
				out = append(out, heading(lvl, title))
				i++
				continue
			}
			if isAllCapsHeading(l.text) && !isListLine(l) && (nextBlank || !isAllCapsHeading(lines[i+1].text)) {
				out = append(out, heading(1, l.text))
				i++
				continue
			}
		}
		if m, ok := listMarker(l.text); ok && startsList(lines, i, m) {
			lst, n := p.list(lines[i:])
			out = append(out, lst)
			i += n
			continue
		}
		b, n := p.paragraph(lines[i:])
		out = append(out, b)
		i += n
	}
	return out, nil
}

func heading(level int, text string) ast.Block {
	return &ast.Heading{Level: min(max(level, 1), 6), Inlines: linkify(collapseSpaces(text))}
}

// atxHeading recognises "# Heading" lines.
func atxHeading(s string) (int, string) {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(s) || s[n] != ' ' {
		return 0, ""
	}
	rest := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s[n:]), "#"))
	if rest == "" {
		return 0, ""
	}
	return n, rest
}

// underlineLevel returns 1, 2 or 3 for "===", "---" and "~~~" underlines.
func underlineLevel(s string) int {
	if len(s) < 3 {
		return 0
	}
	c := s[0]
	if strings.Trim(s, string(c)) != "" {
		return 0
	}
	switch c {
	case '=':
		return 1
	case '-':
		return 2
	case '~':
		return 3
	}
	return 0
}

func headingText(l line) bool {
	if _, ok := listMarker(l.text); ok {
		return false
	}
	return utf8.RuneCountInString(l.text) <= 100 && underlineLevel(l.text) == 0
}

func isRule(s string) bool {
	s = strings.ReplaceAll(s, " ", "")
	if len(s) < 3 {
		return false
	}
	switch s[0] {
	case '-', '*', '_', '=', '~':
		return strings.Trim(s, string(s[0])) == ""
	}
	return false
}

func isListLine(l line) bool {
	_, ok := listMarker(l.text)
	return ok
}

// numberedHeading recognises outline headings such as "2.3 Methods" or
// "IV. Results" standing on their own line before a blank line. A single
// level number ("1. Introduction") is a heading only when the next block
// is not another numbered item (then the lines form a loose list).
func numberedHeading(lines []line, i int) (int, string, bool) {
	l := lines[i]
	if l.indent >= 4 || i+1 < len(lines) && !lines[i+1].blank {
		return 0, "", false
	}
	num, title, ok := strings.Cut(l.text, " ")
	title = strings.TrimSpace(title)
	if !ok || title == "" || utf8.RuneCountInString(title) > 80 {
		return 0, "", false
	}
	if last, _ := utf8.DecodeLastRuneInString(title); strings.ContainsRune(".,;:!?", last) {
		return 0, "", false
	}
	if first, _ := utf8.DecodeRuneInString(title); !unicode.IsUpper(first) && !unicode.IsDigit(first) {
		return 0, "", false
	}
	bare := strings.TrimSuffix(num, ".")
	level := 1
	if romanValue(bare) == 0 || bare != strings.ToUpper(bare) || bare == num {
		parts := strings.Split(bare, ".")
		for _, part := range parts {
			if part == "" || len(part) > 3 || strings.Trim(part, "0123456789") != "" {
				return 0, "", false
			}
		}
		level = len(parts)
		if level == 1 && bare == num {
			return 0, "", false // "2 apples"
		}
	}
	if level == 1 {
		j := i + 1
		for j < len(lines) && lines[j].blank {
			j++
		}
		if j < len(lines) {
			// Another numbered item or an indented continuation means a list.
			if m, ok := listMarker(lines[j].text); ok && m.ordered || lines[j].indent > l.indent {
				return 0, "", false
			}
		}
	}
	return level, title, true
}

// ---------------------------------------------------------------------------
// Lists
// ---------------------------------------------------------------------------

type marker struct {
	ordered bool
	style   ast.NumberStyle
	num     int
	width   int    // bytes of marker and following space
	family  string // "bullet:-", "num", "alpha", "roman"
	letter  rune   // for alpha/roman markers, the marker text
	text    string // raw marker text for alpha/roman ambiguity
}

var bullets = []string{"-", "*", "+", "•", "◦", "▪", "‣", "–", "·", "○", "■", "□", "➢"}

// listMarker recognises a list marker at the start of s.
func listMarker(s string) (marker, bool) {
	for _, b := range bullets {
		if rest, ok := strings.CutPrefix(s, b); ok && (strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t")) && strings.TrimSpace(rest) != "" {
			return marker{width: len(b), family: "bullet:" + b}, true
		}
	}
	paren := strings.HasPrefix(s, "(")
	body := s
	if paren {
		body = s[1:]
	}
	end := 0
	for end < len(body) && end < 10 && (isASCIILetter(body[end]) || body[end] >= '0' && body[end] <= '9') {
		end++
	}
	if end == 0 || end >= len(body) {
		return marker{}, false
	}
	delim := body[end]
	if paren && delim != ')' || !paren && delim != '.' && delim != ')' {
		return marker{}, false
	}
	rest := body[end+1:]
	if !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t") || strings.TrimSpace(rest) == "" {
		return marker{}, false
	}
	tok := body[:end]
	m := marker{ordered: true, width: end + 1, text: tok}
	if paren {
		m.width++
	}
	switch {
	case strings.Trim(tok, "0123456789") == "":
		n, err := strconv.Atoi(tok)
		if err != nil || n >= 1000 || len(tok) > 3 {
			return marker{}, false // years such as "2024. gadā"
		}
		m.family, m.num = "num", n
	case len(tok) == 1 && isASCIILetter(tok[0]):
		m.family, m.letter = "alpha", rune(tok[0])
		m.num = int(unicode.ToLower(m.letter)-'a') + 1
		m.style = ast.NumberLowerAlpha
		if unicode.IsUpper(m.letter) {
			m.style = ast.NumberUpperAlpha
		}
		if v := romanValue(tok); v > 0 && (tok == "i" || tok == "I") {
			m.family, m.num = "roman", v
			m.style = romanStyle(tok)
		}
	case romanValue(tok) > 0:
		m.family, m.num, m.style = "roman", romanValue(tok), romanStyle(tok)
	default:
		return marker{}, false
	}
	return m, true
}

func isASCIILetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func romanStyle(tok string) ast.NumberStyle {
	if tok == strings.ToUpper(tok) {
		return ast.NumberUpperRoman
	}
	return ast.NumberLowerRoman
}

func romanValue(s string) int {
	vals := map[byte]int{'i': 1, 'v': 5, 'x': 10, 'l': 50, 'c': 100, 'd': 500, 'm': 1000}
	if s == "" || s != strings.ToLower(s) && s != strings.ToUpper(s) {
		return 0
	}
	lower := strings.ToLower(s)
	total, prev := 0, 0
	for i := len(lower) - 1; i >= 0; i-- {
		v, ok := vals[lower[i]]
		if !ok {
			return 0
		}
		if v < prev {
			total -= v
		} else {
			total += v
			prev = v
		}
	}
	return total
}

// sameFamily reports whether marker m continues a list started by first.
func sameFamily(first, m marker) bool {
	switch {
	case first.family == m.family:
		return first.family != "alpha" && first.family != "roman" || isUpper(first.text) == isUpper(m.text)
	case first.family == "roman" && m.family == "alpha":
		// "v" after "iv" is roman; letters that are roman numerals parse as
		// alpha in isolation.
		return romanValue(m.text) > 0 && isUpper(first.text) == isUpper(m.text)
	case first.family == "alpha" && m.family == "roman":
		return len(m.text) == 1 && isUpper(first.text) == isUpper(m.text)
	}
	return false
}

func isUpper(s string) bool { return s != "" && s == strings.ToUpper(s) }

// startsList guards against prose that merely begins like a list item:
// letter and roman markers ("A. Smith wrote") need a second item, and a
// lone numbered line must be item 1.
func startsList(lines []line, i int, m marker) bool {
	if !m.ordered {
		return true
	}
	items := 1
	for j := i + 1; j < len(lines) && items < 2; j++ {
		if lines[j].blank {
			continue
		}
		if n, ok := listMarker(lines[j].text); ok && sameFamily(m, n) && lines[j].indent == lines[i].indent {
			items++
			continue
		}
		if lines[j].indent <= lines[i].indent && j > i+1 && lines[j-1].blank {
			break
		}
	}
	switch m.family {
	case "alpha", "roman":
		return items >= 2
	}
	return items >= 2 || m.num == 1
}

type listItem struct {
	blocks []ast.Block
	para   []string
}

func (it *listItem) flush() {
	if len(it.para) > 0 {
		it.blocks = append(it.blocks, &ast.Plain{Inlines: joinLines(it.para)})
		it.para = nil
	}
}

// list parses a list starting at lines[0] and returns it with the number
// of lines consumed. Items continue over wrapped lines, indented
// paragraphs and nested lists.
func (p *parser) list(lines []line) (*ast.List, int) {
	first, _ := listMarker(lines[0].text)
	base := lines[0].indent
	lst := &ast.List{Ordered: first.ordered, Tight: true}
	if first.ordered {
		lst.Start, lst.Style = first.num, first.style
	}
	sibling := func(l line) (marker, bool) {
		m, ok := listMarker(l.text)
		return m, ok && sameFamily(first, m) && l.indent >= base-1 && l.indent <= base+1
	}
	var items []*listItem
	var cur *listItem
	content := 0
	i := 0
	for i < len(lines) {
		l := lines[i]
		if m, ok := sibling(l); ok {
			if cur != nil {
				cur.flush()
			}
			cur = &listItem{para: []string{strings.TrimSpace(l.text[m.width:])}}
			items = append(items, cur)
			content = l.indent + m.width + 1
			i++
			continue
		}
		if cur == nil {
			break
		}
		if l.blank {
			j := i + 1
			for j < len(lines) && lines[j].blank {
				j++
			}
			if j == len(lines) {
				i = j
				break
			}
			next := lines[j]
			_, isMarker := listMarker(next.text)
			if _, ok := sibling(next); ok {
				lst.Tight = false
			} else if !isMarker && next.indent >= content {
				cur.flush() // an indented paragraph of the same item
				lst.Tight = false
			} else if !isMarker || next.indent <= base {
				break
			}
			i = j
			continue
		}
		if _, isMarker := listMarker(l.text); isMarker {
			if l.indent <= base {
				break // a list of another kind starts here
			}
			cur.flush()
			sub, n := p.list(lines[i:])
			cur.blocks = append(cur.blocks, sub)
			i += n
			continue
		}
		// Wrapped text of the current item (lazy continuation).
		cur.para = append(cur.para, l.text)
		i++
	}
	for _, it := range items {
		it.flush()
		if !lst.Tight {
			for k, b := range it.blocks {
				if pl, ok := b.(*ast.Plain); ok {
					it.blocks[k] = &ast.Para{Inlines: pl.Inlines}
				}
			}
		}
		lst.Items = append(lst.Items, ast.ListItem{Blocks: it.blocks})
	}
	return lst, i
}

// ---------------------------------------------------------------------------
// Paragraphs and indented blocks
// ---------------------------------------------------------------------------

// paragraph reads consecutive non-blank lines. Three or more short lines
// (addresses, verse, contact blocks) keep their line breaks.
func (p *parser) paragraph(lines []line) (ast.Block, int) {
	var texts []string
	n := 0
	for n < len(lines) && !lines[n].blank {
		l := lines[n]
		if n > 0 {
			if _, ok := listMarker(l.text); ok {
				break
			}
			if lvl, _ := atxHeading(l.text); lvl > 0 {
				break
			}
			if n+1 < len(lines) && !lines[n+1].blank && underlineLevel(lines[n+1].text) > 0 && headingText(l) {
				break
			}
		}
		texts = append(texts, l.text)
		n++
	}
	if len(texts) >= 3 {
		short := true
		for _, t := range texts {
			if utf8.RuneCountInString(t) >= 45 {
				short = false
				break
			}
		}
		if short {
			lb := &ast.LineBlock{}
			for _, t := range texts {
				lb.Lines = append(lb.Lines, linkify(collapseSpaces(t)))
			}
			return lb, n
		}
	}
	return &ast.Para{Inlines: joinLines(texts)}, n
}

// joinLines joins hard-wrapped lines with soft breaks.
func joinLines(texts []string) []ast.Inline {
	var out []ast.Inline
	for i, t := range texts {
		if i > 0 {
			out = append(out, &ast.SoftBreak{})
		}
		out = append(out, linkify(collapseSpaces(t))...)
	}
	return out
}

// indented handles a block indented by four or more columns: a first-line
// indent opens an ordinary paragraph, indented prose is a quotation and
// anything else is preformatted code.
func (p *parser) indented(lines []line) ([]ast.Block, int) {
	if len(lines) > 1 && !lines[1].blank && lines[1].indent < 4 {
		b, n := p.paragraph(lines)
		return []ast.Block{b}, n
	}
	n := 0
	for n < len(lines) {
		if lines[n].blank {
			j := n
			for j < len(lines) && lines[j].blank {
				j++
			}
			if j < len(lines) && lines[j].indent >= 4 {
				n = j
				continue
			}
			break
		}
		if lines[n].indent < 4 {
			break
		}
		n++
	}
	block := lines[:n]
	if isProse(block) {
		var inner []line
		for _, l := range block {
			l.indent -= 4
			inner = append(inner, l)
		}
		blocks, _ := p.blocks(inner)
		return []ast.Block{&ast.BlockQuote{Blocks: blocks}}, n
	}
	min := -1
	for _, l := range block {
		if !l.blank && (min < 0 || l.indent < min) {
			min = l.indent
		}
	}
	var b strings.Builder
	for k, l := range block {
		if k > 0 {
			b.WriteByte('\n')
		}
		if !l.blank {
			b.WriteString(dropIndent(l.raw, min))
		}
	}
	return []ast.Block{&ast.CodeBlock{Text: b.String()}}, n
}

// isProse reports whether indented lines read as running text rather than
// code or aligned columns.
func isProse(lines []line) bool {
	letters, total := 0, 0
	for _, l := range lines {
		if l.blank {
			continue
		}
		if strings.ContainsAny(l.text, "{}[]();=<>$\\|`") || strings.Contains(l.text, "   ") || strings.Contains(l.text, "\t") {
			return false
		}
		for _, r := range l.text {
			total++
			if unicode.IsLetter(r) || r == ' ' {
				letters++
			}
		}
	}
	return total > 0 && letters*100 >= total*85
}
