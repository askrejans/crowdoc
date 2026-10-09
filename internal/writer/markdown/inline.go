package markdown

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// ictx describes where inline content is written.
type ictx struct {
	lineStart bool // the content starts a line (block syntax could start)
	paraStart bool // the content starts a paragraph
	heading   bool
	oneLine   bool // headings, terms, descriptions: no line breaks
	caption   bool // table or listing caption: "{" would start attributes
	braces    bool // escape "{" and "}" (attribute values)
	quotes    bool // escape quotes (quoted attribute values)
	inLink    bool // link text: no nested links
	alt       bool // image description
	plainAlt  bool // the description of an inline image (plain text)
	table     bool // pipe-table cell
	lineBlock bool
	html      bool // inside an HTML container tag
	// bracketEntities writes "[" as an entity: "\[" would start display
	// math in a block that also contains "\]".
	bracketEntities bool
	// forbid lists delimiter characters used by enclosing spans.
	forbid string
	depth  int
}

// endRune stands for "end of the block" (a line end).
const endRune = '\n'

// inlines renders inline content for a leaf block.
func (w *writer) inlines(ins []ast.Inline, c ictx) string {
	ins = prepareInlines(ins, c)
	return w.seq(ins, c, initialPrev(c), endRune)
}

func initialPrev(c ictx) rune {
	if c.lineStart {
		return endRune
	}
	return ' '
}

// prepareInlines merges text, turns newlines in text into spaces, moves
// white space at the edges of formatting outside of it and trims the
// content of a leaf block.
func prepareInlines(ins []ast.Inline, c ictx) []ast.Inline {
	ins = hoist(ins, 0)
	return ast.TrimInlines(ins)
}

// hoist normalises an inline sequence (recursively): adjacent text is
// merged, control characters are removed, empty formatting is dropped and
// leading/trailing white space inside formatting moves outside, where
// Markdown can express it.
func hoist(ins []ast.Inline, depth int) []ast.Inline {
	out := make([]ast.Inline, 0, len(ins))
	for _, in := range ins {
		switch n := in.(type) {
		case nil:
			continue
		case *ast.Text:
			v := cleanText(n.Value)
			if v != "" {
				out = append(out, &ast.Text{Value: v})
			}
		case *ast.Emph, *ast.Strong, *ast.Strike, *ast.Underline, *ast.Superscript,
			*ast.Subscript, *ast.SmallCaps, *ast.Highlight, *ast.Span:
			kids := ast.InlineChildren(in)
			if depth < maxDepth {
				kids = hoist(kids, depth+1)
			}
			lead, core, trail := splitEdges(kids)
			out = append(out, lead...)
			if sp, ok := in.(*ast.Span); ok && len(core) == 0 && sp.Attr.ID != "" {
				out = append(out, &ast.Span{Attr: sp.Attr})
			} else if len(core) > 0 {
				out = append(out, withChildren(in, core))
			}
			out = append(out, trail...)
		case *ast.Code:
			if cleanText(n.Text) != "" {
				out = append(out, n)
			}
		case *ast.Math:
			if strings.TrimSpace(n.TeX) != "" {
				out = append(out, n)
			}
		case *ast.Link:
			kids := n.Inlines
			if depth < maxDepth {
				kids = hoist(kids, depth+1)
			}
			l := *n
			l.Inlines = kids
			out = append(out, &l)
		default:
			out = append(out, in)
		}
	}
	return ast.MergeText(out)
}

const maxDepth = 64

// cleanText removes characters Markdown cannot carry: control characters
// (newlines become spaces).
func cleanText(s string) string {
	ok := true
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 && s[i] != '\t' || s[i] == 0x7f {
			ok = false
			break
		}
	}
	if ok && utf8.ValidString(s) {
		return s
	}
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r':
			sb.WriteByte(' ')
		case r == utf8.RuneError:
			sb.WriteRune(r)
		case r < 0x20 && r != '\t' || r == 0x7f:
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// splitEdges separates leading and trailing white space and breaks.
func splitEdges(ins []ast.Inline) (lead, core, trail []ast.Inline) {
	core = ins
	for len(core) > 0 {
		switch n := core[0].(type) {
		case *ast.SoftBreak, *ast.LineBreak:
			lead = append(lead, n)
			core = core[1:]
			continue
		case *ast.Text:
			t := strings.TrimLeftFunc(n.Value, unicode.IsSpace)
			if t != n.Value {
				lead = append(lead, &ast.Text{Value: n.Value[:len(n.Value)-len(t)]})
				if t == "" {
					core = core[1:]
					continue
				}
				core = append([]ast.Inline{&ast.Text{Value: t}}, core[1:]...)
			}
		}
		break
	}
	for len(core) > 0 {
		last := len(core) - 1
		switch n := core[last].(type) {
		case *ast.SoftBreak, *ast.LineBreak:
			trail = append([]ast.Inline{n}, trail...)
			core = core[:last]
			continue
		case *ast.Text:
			t := strings.TrimRightFunc(n.Value, unicode.IsSpace)
			if t != n.Value {
				trail = append([]ast.Inline{&ast.Text{Value: n.Value[len(t):]}}, trail...)
				if t == "" {
					core = core[:last]
					continue
				}
				core = append(append([]ast.Inline{}, core[:last]...), &ast.Text{Value: t})
			}
		}
		break
	}
	return lead, core, trail
}

func withChildren(in ast.Inline, kids []ast.Inline) ast.Inline {
	switch n := in.(type) {
	case *ast.Emph:
		return &ast.Emph{Inlines: kids}
	case *ast.Strong:
		return &ast.Strong{Inlines: kids}
	case *ast.Strike:
		return &ast.Strike{Inlines: kids}
	case *ast.Underline:
		return &ast.Underline{Inlines: kids}
	case *ast.Superscript:
		return &ast.Superscript{Inlines: kids}
	case *ast.Subscript:
		return &ast.Subscript{Inlines: kids}
	case *ast.SmallCaps:
		return &ast.SmallCaps{Inlines: kids}
	case *ast.Highlight:
		return &ast.Highlight{Inlines: kids}
	case *ast.Span:
		return &ast.Span{Attr: n.Attr, Inlines: kids}
	}
	return in
}

// ---------------------------------------------------------------------------
// Sequences
// ---------------------------------------------------------------------------

// seqState is the rendering state of one inline sequence.
type seqState struct {
	sb   strings.Builder
	prev rune // the character before the sequence
	// after describes the piece just written.
	afterMath, afterImage, afterBracket, afterCite, afterCode bool
}

func (s *seqState) last() rune {
	if s.sb.Len() == 0 {
		return s.prev
	}
	r, _ := utf8.DecodeLastRuneInString(s.sb.String())
	return r
}

func (s *seqState) atLineStart(c ictx) bool {
	if s.sb.Len() == 0 {
		return c.lineStart
	}
	return strings.HasSuffix(s.sb.String(), "\n")
}

// seq renders ins; prev is the character before it and end the character
// that follows it.
func (w *writer) seq(ins []ast.Inline, c ictx, prev, end rune) string {
	st := &seqState{prev: prev}
	for i, in := range ins {
		next := end
		if i+1 < len(ins) {
			next = firstRune(ins[i+1])
		}
		w.inline(st, in, c, next, i == 0)
		if (st.afterBracket || st.afterCite) && i+1 < len(ins) {
			sep := true
			switch ins[i+1].(type) {
			case *ast.Text, *ast.SoftBreak:
				sep = false
			case *ast.LineBreak:
				// "<br>" in a table cell would extend a key.
				sep = c.table && st.afterCite
			}
			if sep {
				st.sb.WriteString("<!-- -->")
			}
		}
	}
	// A closing "_", "~" or HTML tag would extend a citation key.
	if st.afterCite && (isKeyRune(end) || end == fmtRune || strings.ContainsRune(":.#$%&+?<>~/-", end)) {
		st.sb.WriteString("<!-- -->")
	}
	return st.sb.String()
}

// firstRune approximates the first character an inline is written with.
// Only its class (space, punctuation, other) and a few exact characters
// matter to the escaping rules.
func firstRune(in ast.Inline) rune {
	switch n := in.(type) {
	case *ast.Text:
		r, _ := utf8.DecodeRuneInString(n.Value)
		return r
	case *ast.SoftBreak:
		return ' '
	case *ast.LineBreak:
		return '\\'
	case *ast.RawInline:
		r, _ := utf8.DecodeRuneInString(n.Text)
		if r == utf8.RuneError {
			return '<'
		}
		return r
	case *ast.Link, *ast.Note:
		return '['
	case *ast.Image:
		return '!'
	case *ast.Code:
		return '`'
	case *ast.Math:
		return '$'
	case *ast.Cite, *ast.Ref:
		return '[' // or "@" for narrative citations
	}
	return fmtRune // formatting: a delimiter or an HTML tag
}

// fmtRune stands for the unknown first character of formatting: a
// delimiter ("*", "_", "~", "=", "^") or an HTML tag. It counts as
// punctuation and as a possible citation-key character.
const fmtRune = '\uE000'

func (w *writer) inline(st *seqState, in ast.Inline, c ictx, next rune, first bool) {
	afterMath, afterImage, afterBracket, afterCite, afterCode := st.afterMath, st.afterImage, st.afterBracket, st.afterCite, st.afterCode
	st.afterMath, st.afterImage, st.afterBracket, st.afterCite, st.afterCode = false, false, false, false, false
	switch n := in.(type) {
	case *ast.Text:
		s := w.escapeText(n.Value, st, c, next, textFlags{afterMath: afterMath, afterImage: afterImage, afterBracket: afterBracket, afterCite: afterCite})
		if afterCite && strings.HasPrefix(s, " ") && strings.TrimLeft(s, " ") == "" && next == '[' {
			// "@key [" would read the bracket as the citation's locator.
			s = "&#32;" + s[1:]
		}
		if afterCite {
			if r, _ := utf8.DecodeRuneInString(s); isKeyRune(r) || strings.ContainsRune(":.#$%&+?<>~/-", r) {
				st.sb.WriteString("<!-- -->") // the text would extend the citation key
			}
		}
		st.sb.WriteString(s)
	case *ast.SoftBreak:
		if st.sb.Len() > 0 && st.atLineStart(c) || afterCite && next == '[' {
			if afterCite {
				st.sb.WriteString("<!-- -->") // "@key&#32;" would extend the key
			}
			st.sb.WriteString("&#32;") // a space starting a line would be dropped
		} else {
			st.sb.WriteByte(' ')
		}
	case *ast.LineBreak:
		switch {
		case c.heading || c.oneLine || c.plainAlt:
			st.sb.WriteByte(' ') // one-line contexts and plain image descriptions
		case c.table || strings.HasSuffix(st.sb.String(), `\`) || st.sb.Len() == 0:
			st.sb.WriteString("<br>")
		default:
			st.sb.WriteString("\\\n")
		}
	case *ast.Emph:
		w.delimited(st, n.Inlines, []string{"*", "_"}, "em", "", c, next)
	case *ast.Strong:
		w.delimited(st, n.Inlines, []string{"**", "__"}, "strong", "", c, next)
	case *ast.Strike:
		w.delimited(st, n.Inlines, []string{"~~"}, "del", "", c, next)
	case *ast.Highlight:
		w.delimited(st, n.Inlines, []string{"=="}, "mark", "", c, next)
	case *ast.Superscript:
		w.delimited(st, n.Inlines, []string{"^"}, "sup", "", c, next)
	case *ast.Subscript:
		if s, ok := simpleSub(n.Inlines); ok && st.last() != '~' && next != '~' && !strings.Contains(c.forbid, "~") {
			st.sb.WriteString("~" + s + "~")
			return
		}
		w.delimited(st, n.Inlines, nil, "sub", "", c, next)
	case *ast.Underline:
		w.delimited(st, n.Inlines, nil, "u", "", c, next)
	case *ast.SmallCaps:
		w.delimited(st, n.Inlines, nil, "span", ` class="smallcaps"`, c, next)
	case *ast.Span:
		w.span(st, n, c, next)
	case *ast.Code:
		code := codeSpan(n.Text, c.table)
		if strings.Contains(n.Text, "@") && isEmailRune(st.last()) || afterCode {
			// "x`a@b.c" would read as an e-mail address, two code spans
			// in a row as one fence.
			st.sb.WriteString("<!-- -->") // "x`a@b.c" would read as an e-mail address
		}
		st.sb.WriteString(code)
		st.afterCode = code != ""
	case *ast.Math:
		if strings.Contains(n.TeX, "@") && isEmailRune(st.last()) {
			st.sb.WriteString("<!-- -->")
		}
		w.math(st, n, c)
		st.afterMath = true
	case *ast.Link:
		w.link(st, n, c)
	case *ast.Image:
		st.sb.WriteString(w.image(n, nil, false, c))
		st.afterImage = true
	case *ast.Note:
		w.note(st, n, c)
		st.afterBracket = true
	case *ast.Cite:
		bracket := w.cite(st, n, c)
		st.afterBracket, st.afterCite = bracket || strings.HasSuffix(st.sb.String(), "]"), !bracket
	case *ast.Ref:
		bracket := w.ref(st, n, c)
		st.afterBracket, st.afterCite = bracket, !bracket
	case *ast.RawInline:
		if strings.EqualFold(n.Format, "html") {
			raw := strings.Join(strings.Fields(n.Text), " ")
			if c.table {
				raw = strings.ReplaceAll(raw, "|", "&#124;")
			}
			st.sb.WriteString(raw)
			return
		}
		w.warn("raw %s inline content dropped", n.Format)
	default:
		w.warn("unsupported inline %T dropped", in)
	}
}

// isEmailRune reports whether r may belong to the local part of an
// e-mail address.
func isEmailRune(r rune) bool {
	return r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(".!#$%&'*+/=?^_`{|}~-", r))
}

// isKeyRune reports whether r continues a citation key.
func isKeyRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

// delimited writes formatting with the first usable delimiter, or as an
// HTML element when no delimiter would read back correctly.
func (w *writer) delimited(st *seqState, kids []ast.Inline, delims []string, tag, tagAttrs string, c ictx, next rune) {
	prev := st.last()
	choice := ""
	if c.depth < maxDepth {
		first, last := firstRune(kids[0]), lastRune(kids[len(kids)-1])
		for _, d := range delims {
			if delimOK(d, prev, first, last, next, c.forbid) {
				choice = d
				break
			}
		}
	}
	inner := c
	inner.depth++
	inner.lineStart, inner.paraStart = false, false
	open, close := choice, choice
	if choice == "" {
		inner.html = true
		open, close = "<"+tag+tagAttrs+">", "</"+tag+">"
	} else {
		inner.forbid += choice[:1]
	}
	body := w.seq(kids, inner, rune(open[len(open)-1]), rune(close[0]))
	if body == "" {
		return
	}
	if choice != "" {
		f, _ := utf8.DecodeRuneInString(body)
		l, _ := utf8.DecodeLastRuneInString(body)
		if !delimOK(choice, prev, f, l, next, "") || strings.HasSuffix(body, "\\") && choice != "" {
			open, close = "<"+tag+tagAttrs+">", "</"+tag+">"
		}
	}
	st.sb.WriteString(open)
	st.sb.WriteString(body)
	st.sb.WriteString(close)
}

func lastRune(in ast.Inline) rune {
	switch n := in.(type) {
	case *ast.Text:
		r, _ := utf8.DecodeLastRuneInString(n.Value)
		return r
	case *ast.SoftBreak:
		return ' '
	case *ast.LineBreak:
		return '\n'
	case *ast.Link, *ast.Image:
		return ')'
	case *ast.Note, *ast.Cite, *ast.Ref:
		return ']'
	case *ast.Code:
		return '`'
	case *ast.Math:
		return '$'
	}
	return '>'
}

// delimOK reports whether delimiter d around content starting with first
// and ending with last, between prev and next, reads back as formatting.
func delimOK(d string, prev, first, last, next rune, forbid string) bool {
	ch := rune(d[0])
	if strings.ContainsRune(forbid, ch) || prev == ch || next == ch || first == ch || last == ch {
		return false
	}
	if isSpace(first) || isSpace(last) {
		return false
	}
	if ch == '^' && (first == '[' || prev == '[' || next == '[') {
		return false // ^[ starts an inline footnote, [^ a footnote reference
	}
	openLeft, openRight := flanking(prev, first)
	closeLeft, closeRight := flanking(last, next)
	switch ch {
	case '_':
		canOpen := openLeft && (!openRight || isPunct(prev))
		canClose := closeRight && (!closeLeft || isPunct(next))
		return canOpen && canClose
	default:
		return openLeft && closeRight
	}
}

// flanking computes CommonMark's left- and right-flanking properties of a
// delimiter run between before and after.
func flanking(before, after rune) (left, right bool) {
	bs, bp := isSpace(before), isPunct(before)
	as, ap := isSpace(after), isPunct(after)
	left = !as && (!ap || bs || bp)
	right = !bs && (!bp || as || ap)
	return left, right
}

func isSpace(r rune) bool { return r == 0 || unicode.IsSpace(r) }

// isASCIISpace matches the white space the reader's own inline parsers
// check for.
func isASCIISpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func isPunct(r rune) bool { return r == fmtRune || unicode.IsPunct(r) || unicode.IsSymbol(r) }

// simpleSub returns the text of a subscript written as ~text~: one text
// run without white space or tildes.
func simpleSub(ins []ast.Inline) (string, bool) {
	if len(ins) != 1 {
		return "", false
	}
	t, ok := ins[0].(*ast.Text)
	if !ok || t.Value == "" || strings.ContainsAny(t.Value, "~") || strings.IndexFunc(t.Value, unicode.IsSpace) >= 0 || strings.ContainsRune(t.Value, escapeMarkRune) {
		return "", false
	}
	// The content is read verbatim and unescaped once; escaping all
	// punctuation keeps automatic links from reaching into it.
	var sb strings.Builder
	for i := 0; i < len(t.Value); i++ {
		ch := t.Value[i]
		switch {
		case ch == '&':
			sb.WriteString("&amp;")
		case ch < utf8.RuneSelf && isPunct(rune(ch)) && !unicode.IsLetter(rune(ch)):
			sb.WriteByte('\\')
			sb.WriteByte(ch)
		default:
			sb.WriteByte(ch)
		}
	}
	return sb.String(), true
}

func (w *writer) span(st *seqState, sp *ast.Span, c ictx, next rune) {
	a := sp.Attr
	if len(sp.Inlines) == 0 && a.ID == "" {
		return
	}
	if a.ID == "" && len(a.KV) == 0 && len(a.Classes) == 0 {
		st.sb.WriteString(w.seq(sp.Inlines, c, st.last(), next))
		return
	}
	if a.ID == "" && len(a.KV) == 0 && len(a.Classes) == 1 && a.Classes[0] == "kbd" {
		w.delimited(st, sp.Inlines, nil, "kbd", "", c, next)
		return
	}
	attrs := htmlAttrs(a)
	if len(sp.Inlines) == 0 {
		st.sb.WriteString("<span" + attrs + "></span>")
		return
	}
	w.delimited(st, sp.Inlines, nil, "span", attrs, c, next)
}

// htmlAttrs renders attributes for an HTML start tag.
func htmlAttrs(a ast.Attr) string {
	var sb strings.Builder
	if a.ID != "" {
		sb.WriteString(` id="` + htmlEscape(a.ID) + `"`)
	}
	if len(a.Classes) > 0 {
		sb.WriteString(` class="` + htmlEscape(strings.Join(a.Classes, " ")) + `"`)
	}
	for _, k := range sortedKeys(a.KV) {
		if !validAttrName(k) {
			continue
		}
		sb.WriteString(" " + k + `="` + htmlEscape(a.KV[k]) + `"`)
	}
	return sb.String()
}

var attrNameRe = regexp.MustCompile(`^[A-Za-z_:][A-Za-z0-9_:.-]*$`)

func validAttrName(k string) bool { return attrNameRe.MatchString(k) }

// ---------------------------------------------------------------------------
// Code and math
// ---------------------------------------------------------------------------

// codeSpan writes inline code with a backtick fence no run inside matches.
func codeSpan(text string, table bool) string {
	text = strings.Join(strings.Split(cleanText(text), "\n"), " ")
	if text == "" {
		return ""
	}
	if table {
		text = strings.ReplaceAll(text, "|", `\|`)
	}
	runs := map[int]bool{}
	cur := 0
	for i := 0; i <= len(text); i++ {
		if i < len(text) && text[i] == '`' {
			cur++
			continue
		}
		if cur > 0 {
			runs[cur] = true
		}
		cur = 0
	}
	n := 1
	for runs[n] {
		n++
	}
	fence := strings.Repeat("`", n)
	allSpace := strings.Trim(text, " ") == ""
	if strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") ||
		!allSpace && strings.HasPrefix(text, " ") && strings.HasSuffix(text, " ") {
		text = " " + text + " "
	}
	return fence + text + fence
}

func (w *writer) math(st *seqState, m *ast.Math, c ictx) {
	tex := strings.Join(strings.Fields(cleanText(m.TeX)), " ")
	if tex == "" {
		return
	}
	if c.table {
		tex = strings.ReplaceAll(tex, "|", `\|`)
	}
	if dollarSafe(tex) {
		st.sb.WriteString("$" + tex + "$")
		return
	}
	if !strings.Contains(tex, `\)`) && looksLikeMath(tex) {
		st.sb.WriteString(`\(` + tex + `\)`)
		return
	}
	w.warn("inline math %q may not read back as math", tex)
	st.sb.WriteString("$" + tex + "$")
}

// dollarSafe reports whether $tex$ reads back as tex: no unescaped dollar
// inside and no trailing backslash escaping the closing dollar.
func dollarSafe(tex string) bool {
	for i := 0; i < len(tex); i++ {
		switch tex[i] {
		case '\\':
			if i+1 == len(tex) {
				return false
			}
			i++
		case '$':
			return false
		}
	}
	return true
}

func looksLikeMath(s string) bool {
	t := strings.TrimSpace(s)
	if strings.ContainsAny(s, `\^_=+{}<>|`) {
		return true
	}
	if t == "" || len(t) > 3 {
		return false
	}
	for _, r := range t {
		if r < '0' || r > '9' {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Links and images
// ---------------------------------------------------------------------------

var schemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]{1,31}:[^\s<>]*$`)
var emailRe = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

func (w *writer) link(st *seqState, l *ast.Link, c ictx) {
	if c.inLink {
		// Links cannot nest: keep the text.
		st.sb.WriteString(w.seq(l.Inlines, c, st.last(), ']'))
		return
	}
	if t, ok := soleText(l.Inlines); ok && l.Title == "" && !c.table {
		switch {
		case t == l.URL && schemeRe.MatchString(t) && !strings.ContainsAny(t, "\\&") && !strings.HasPrefix(strings.ToLower(t), "mailto:"):
			st.sb.WriteString("<" + t + ">")
			return
		case "mailto:"+t == l.URL && emailRe.MatchString(t):
			st.sb.WriteString("<" + t + ">")
			return
		}
	}
	inner := c
	inner.inLink = true
	inner.lineStart, inner.paraStart = false, false
	text := w.seq(l.Inlines, inner, '[', ']')
	switch text {
	case " ", "x", "X":
		// "[x](…" at the start of a list item would read as a checkbox.
		text = "&#" + strconv.Itoa(int(text[0])) + ";"
	}
	st.sb.WriteString("[" + text + "](" + destination(l.URL, c.table) + linkTitle(l.Title, c.table) + ")")
}

func soleText(ins []ast.Inline) (string, bool) {
	if len(ins) != 1 {
		return "", false
	}
	t, ok := ins[0].(*ast.Text)
	if !ok {
		return "", false
	}
	return t.Value, true
}

// destination writes a link or image destination.
func destination(u string, table bool) string {
	u = cleanText(u)
	if u == "" {
		return "<>" // an empty destination (a title may follow)
	}
	var sb strings.Builder
	pointy := strings.ContainsAny(u, "<> \t") || !balancedParens(u)
	for i := 0; i < len(u); i++ {
		ch := u[i]
		switch {
		case ch == '\\':
			sb.WriteString(`\\`)
		case ch == '&' && entityAt(u, i):
			sb.WriteString("&amp;")
		case pointy && (ch == '<' || ch == '>'):
			sb.WriteByte('\\')
			sb.WriteByte(ch)
		case table && ch == '|':
			sb.WriteString(`\|`)
		default:
			sb.WriteByte(ch)
		}
	}
	if pointy {
		return "<" + sb.String() + ">"
	}
	return sb.String()
}

func balancedParens(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func linkTitle(t string, table bool) string {
	t = cleanText(t)
	if t == "" {
		return ""
	}
	var sb strings.Builder
	for i := 0; i < len(t); i++ {
		ch := t[i]
		switch {
		case ch == '\\' || ch == '"':
			sb.WriteByte('\\')
			sb.WriteByte(ch)
		case ch == '&' && entityAt(t, i):
			sb.WriteString("&amp;")
		case table && ch == '|':
			sb.WriteString(`\|`)
		default:
			sb.WriteByte(ch)
		}
	}
	return ` "` + sb.String() + `"`
}

// image writes ![description](src "title"){attributes}. For figures the
// description is the caption.
func (w *writer) image(img *ast.Image, caption []ast.Inline, figure bool, c ictx) string {
	src := w.media.resolve(w, img.Src)
	var desc string
	inner := c
	inner.alt = true
	inner.lineStart, inner.paraStart = false, false
	if figure {
		// A figure caption may hold links, notes and citations.
		desc = w.seq(hoist(caption, 0), inner, '[', ']')
	} else {
		inner.inLink, inner.plainAlt = true, true
		desc = w.seq([]ast.Inline{&ast.Text{Value: strings.Join(strings.Fields(img.Alt), " ")}}, inner, '[', ']')
	}
	s := "![" + desc + "](" + destination(src, c.table) + linkTitle(img.Title, c.table) + ")"
	a := img.Attr
	if img.Width != "" || img.Height != "" {
		a.KV = copyKV(a.KV)
		if img.Width != "" {
			a.KV["width"] = img.Width
		}
		if img.Height != "" {
			a.KV["height"] = img.Height
		}
	}
	if attrs := w.attrString(a, attrInline); attrs != "" {
		if c.table {
			attrs = strings.ReplaceAll(attrs, "|", `\|`)
		}
		s += attrs
	}
	return s
}

// ---------------------------------------------------------------------------
// Notes, citations and cross-references
// ---------------------------------------------------------------------------

func (w *writer) note(st *seqState, n *ast.Note, c ictx) {
	idx := len(w.notes)
	w.notes = append(w.notes, "")
	label := "[^" + strconv.Itoa(idx+1) + "]"
	body := w.blocks(n.Blocks, false)
	switch {
	case body == "":
		w.notes[idx] = label + ": &nbsp;"
	case len(n.Blocks) > 0 && isText(n.Blocks[0]):
		w.notes[idx] = label + ": " + indentRest(body, "    ")
	default:
		w.notes[idx] = label + ":\n" + indentRest("\n"+body, "    ")[1:]
	}
	st.sb.WriteString(label)
}

var locatorAbbrev = map[string]string{
	"": "p.", "page": "p.", "chapter": "chap.", "section": "sec.", "figure": "fig.", "table": "tab.",
	"paragraph": "para.", "line": "l.", "volume": "vol.", "note": "n.", "number": "no.",
}

var citeKeyRe = regexp.MustCompile(`^[\p{L}\p{N}_](?:[\p{L}\p{N}_:.#$%&+?<>~/-]*[\p{L}\p{N}_])?$`)

// cite writes a citation and reports whether it used the bracket form.
func (w *writer) cite(st *seqState, ci *ast.Cite, c ictx) bool {
	ok := len(ci.Items) > 0
	for _, it := range ci.Items {
		if !citeKeyRe.MatchString(it.Key) {
			ok = false
		}
	}
	if !ok || c.inLink {
		fallback := ci.Rendered
		if fallback == nil {
			fallback = ci.Fallback
		}
		w.warn("citation written as text")
		st.sb.WriteString(w.seq(hoist(fallback, 0), c, st.last(), ' '))
		return false
	}
	if ci.Mode == ast.CiteNarrative && len(ci.Items) == 1 {
		it := ci.Items[0]
		// "@key [locator, suffix]": the bracket ends at the first "]".
		loc := narrativeLocator(it)
		if citeText(it.Prefix) == "" && !it.SuppressAuthor && !strings.Contains(loc, "@") {
			if !narrativeOK(st.last()) {
				st.sb.WriteString("<!-- -->") // "x@key" is not a citation
			}
			st.sb.WriteString("@" + it.Key)
			if loc != "" {
				st.sb.WriteString(" [" + loc + "]")
			}
			return false
		}
	}
	if ci.Mode == ast.CiteNarrative {
		w.warn("narrative citation of @%s written in brackets", ci.Items[0].Key)
	}
	var parts []string
	for _, it := range ci.Items {
		var sb strings.Builder
		if p := citeText(it.Prefix); p != "" {
			sb.WriteString(p + " ")
		}
		if it.SuppressAuthor {
			sb.WriteByte('-')
		}
		sb.WriteString("@" + it.Key)
		if loc := citeLocator(it); loc != "" {
			sb.WriteString(", " + loc)
		} else if s := citeSuffix(it.Suffix); s != "" {
			sb.WriteString(", " + s)
		}
		parts = append(parts, sb.String())
	}
	s := "[" + strings.Join(parts, "; ") + "]"
	if c.table {
		s = strings.ReplaceAll(s, "|", `\|`)
	}
	st.sb.WriteString(s)
	return true
}

// narrativeOK reports whether "@key" after prev reads as a citation.
func narrativeOK(prev rune) bool {
	return !(unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '_' || prev == '@' || prev == '/' || prev == '.')
}

func citeLocator(it ast.CiteItem) string {
	if it.Locator == "" {
		return ""
	}
	label, ok := locatorAbbrev[it.LocatorLabel]
	if !ok {
		label = citeText(it.LocatorLabel)
	}
	s := label + " " + citeSuffix(it.Locator)
	if suf := citeSuffix(it.Suffix); suf != "" {
		s += ", " + suf
	}
	return s
}

func narrativeLocator(it ast.CiteItem) string {
	clean := func(s string) string {
		return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
			switch r {
			case ']', '\\', '`':
				return -1
			}
			return r
		}, cleanText(s))), " ")
	}
	var s string
	if it.Locator != "" {
		label, ok := locatorAbbrev[it.LocatorLabel]
		if !ok {
			label = clean(it.LocatorLabel)
		}
		s = label + " " + clean(it.Locator)
		if suf := clean(it.Suffix); suf != "" {
			s += ", " + suf
		}
		return s
	}
	return clean(it.Suffix)
}

// citeText removes the characters that would end or split a citation:
// ";", "@", backslashes and unbalanced brackets.
func citeText(s string) string { return citePart(s, true) }

// citeSuffix is citeText for text after the key, where "@" is harmless.
func citeSuffix(s string) string { return citePart(s, false) }

func citePart(s string, prefix bool) string {
	balanced := balancedBrackets(s)
	s = strings.Map(func(r rune) rune {
		switch r {
		case '@':
			if prefix {
				return -1
			}
		case ';', '\n', '\r', '\\', '`':
			return -1
		case '[', ']':
			if !balanced {
				return -1
			}
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func balancedBrackets(s string) bool {
	depth := 0
	for _, r := range s {
		switch r {
		case '[':
			depth++
		case ']':
			if depth--; depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func (w *writer) ref(st *seqState, r *ast.Ref, c ictx) bool {
	if !citeKeyRe.MatchString(r.Target) || c.inLink {
		w.warn("cross-reference to %q written as text", r.Target)
		st.sb.WriteString(w.escapeText(r.Target, st, c, ' ', textFlags{}))
		return false
	}
	if !r.Bare {
		if !narrativeOK(st.last()) {
			st.sb.WriteString("<!-- -->")
		}
		st.sb.WriteString("@" + r.Target)
		return false
	}
	if r.Bare {
		st.sb.WriteString("[-@" + r.Target + "]")
	} else {
		st.sb.WriteString("[@" + r.Target + "]")
	}
	return true
}

// ---------------------------------------------------------------------------
// Text
// ---------------------------------------------------------------------------

type textFlags struct {
	afterMath, afterImage, afterBracket, afterCite, afterCode bool
}

const escapeMarkRune = '﷐'

// escapeText escapes plain text so the reader returns it unchanged.
func (w *writer) escapeText(s string, st *seqState, c ictx, next rune, f textFlags) string {
	if s == "" {
		return ""
	}
	rs := []rune(s)
	n := len(rs)
	esc := make([]string, n) // replacement per rune ("" = as is)
	prevOf := func(i int) rune {
		if i == 0 {
			return st.last()
		}
		return rs[i-1]
	}
	nextOf := func(i int) rune {
		if i+1 < n {
			return rs[i+1]
		}
		return next
	}
	bs := func(i int) { esc[i] = `\` + string(rs[i]) }
	lineStart := st.atLineStart(c)

	// Delimiter runs.
	for i := 0; i < n; {
		ch := rs[i]
		j := i
		for j < n && rs[j] == ch {
			j++
		}
		before, after := prevOf(i), nextOf(j-1)
		switch ch {
		case '*', '_', '~', '^':
			l, r := flanking(before, after)
			if ch == '~' && !isASCIISpace(after) {
				l = true // "~x~" is a subscript
			}
			open, close := l, r
			if ch == '_' {
				open = l && (!r || isPunct(before))
				close = r && (!l || isPunct(after))
			}
			if open || close || before == ch || after == ch || strings.ContainsRune(c.forbid, ch) && (l || r) {
				for k := i; k < j; k++ {
					bs(k)
				}
			}
		case '=':
			l, r := flanking(before, after)
			if j-i == 2 && (l || r) || before == '=' || after == '=' {
				for k := i; k < j; k++ {
					bs(k)
				}
			}
		case '-':
			// "--" and "---" become dashes.
			for k := i + 1; k < j; k++ {
				bs(k)
			}
		case '.':
			if j-i >= 3 {
				for k := i + 1; k < j; k++ {
					bs(k)
				}
			}
		}
		i = j
	}

	for i := 0; i < n; i++ {
		ch := rs[i]
		nx := nextOf(i)
		switch ch {
		case '\\', '`', '|':
			bs(i)
		case '[':
			if c.bracketEntities {
				esc[i] = "&#91;"
			} else {
				bs(i)
			}
		case ']':
			if c.inLink || c.alt {
				esc[i] = "&#93;"
			}
		case '<':
			if nx < 128 && (unicode.IsLetter(nx) || nx == '/' || nx == '!' || nx == '?') {
				bs(i)
			}
		case '&':
			if entityAt(string(rs[i:]), 0) {
				bs(i)
			}
		case '$':
			if !isASCIISpace(nx) {
				bs(i) // "$x" opens math
			}
		case '@':
			if isKeyRune(nx) || nx == fmtRune {
				bs(i)
			}
		case '!', '^':
			if nx == '[' {
				bs(i) // "![" opens an image, "^[" an inline note
			}
		case '{':
			if c.heading || c.caption || c.braces {
				bs(i)
			}
		case '}':
			if c.braces {
				bs(i)
			}
		case '"', '\'':
			if c.quotes {
				bs(i)
			}
		case escapeMarkRune:
			esc[i] = "&#xFDD0;"
		}
	}

	// Word starts that would become automatic links.
	if !c.inLink {
		for i := 0; i < n; i++ {
			// The reader looks for links after spaces, "*_~(" and at the
			// start of every text run.
			if i > 0 && !isLinkifyBoundary(rs[i-1]) {
				continue
			}
			rest := string(rs[i:])
			switch {
			case strings.HasPrefix(rest, "http:") || strings.HasPrefix(rest, "ftp:"):
				bs(i + strings.IndexByte(rest, ':'))
			case strings.HasPrefix(rest, "https:"):
				bs(i + 5)
			case strings.HasPrefix(rest, "www."):
				bs(i + 3)
			}
		}
	}

	// Heading text must not end in a closing sequence of "#".
	if c.heading && next == endRune && rs[n-1] == '#' {
		k := n - 1
		for k > 0 && rs[k-1] == '#' {
			k--
		}
		if isSpace(prevOf(k)) {
			bs(k)
		}
	}

	// Text right after an image would be read as its {attributes}.
	if f.afterImage {
		for i := 0; i < n; i++ {
			if rs[i] == '{' {
				bs(i)
			}
			if !unicode.IsSpace(rs[i]) {
				break
			}
		}
	}
	if f.afterBracket && n > 0 {
		switch rs[0] {
		case '(':
			esc[0] = "&#40;"
		case ':':
			bs(0)
		}
	}
	if f.afterCite && n > 0 && strings.ContainsRune(":.#$%&+?<>~/-", rs[0]) {
		bs(0)
	}
	if f.afterMath && n > 0 && rs[0] >= '0' && rs[0] <= '9' {
		esc[0] = "&#" + strconv.Itoa(int(rs[0])) + ";"
	}

	start := 0
	if lineStart {
		// Leading white space after a hard break would be dropped.
		for start < n && (rs[start] == ' ' || rs[start] == '\t') {
			esc[start] = "&#" + strconv.Itoa(int(rs[start])) + ";"
			start++
		}
		if start == 0 {
			w.lineStartEscapes(rs, esc, c)
		}
	}

	var sb strings.Builder
	for i, r := range rs {
		if esc[i] != "" {
			sb.WriteString(esc[i])
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func isLinkifyBoundary(r rune) bool {
	return r == ' ' || r == '*' || r == '_' || r == '~' || r == '(' || r == '\n' || r == 0 || isSpace(r)
}

// lineStartEscapes escapes characters that would start a block at the
// beginning of a line.
func (w *writer) lineStartEscapes(rs []rune, esc []string, c ictx) {
	n := len(rs)
	bs := func(i int) {
		if esc[i] == "" {
			esc[i] = `\` + string(rs[i])
		}
	}
	nextIs := func(i int, f func(rune) bool) bool {
		return i+1 >= n || f(rs[i+1])
	}
	ws := func(r rune) bool { return r == ' ' || r == '\t' }
	switch rs[0] {
	case '#', '>', ':', '%', '=', '+', '*', '_', '~':
		bs(0)
	case '-':
		if nextIs(0, ws) || n > 1 && rs[1] == '-' || strings.Trim(string(rs), "-:| \t") == "" {
			bs(0) // a list item, a rule, a setext underline or a table delimiter row
		}
	case '<':
		if n > 1 && (unicode.IsLetter(rs[1]) || rs[1] == '/' || rs[1] == '!' || rs[1] == '?') {
			bs(0)
		}
	default:
		if rs[0] >= '0' && rs[0] <= '9' {
			k := 0
			for k < n && k < 10 && rs[k] >= '0' && rs[k] <= '9' {
				k++
			}
			if k < n && (rs[k] == '.' || rs[k] == ')') && nextIs(k, ws) {
				bs(k)
			}
		}
	}
	if c.paraStart {
		s := string(rs)
		for _, p := range []string{"Table:", "Listing:"} {
			if strings.HasPrefix(s, p) {
				bs(utf8.RuneCountInString(p) - 1)
			}
		}
	}
}

// entityAt reports whether s[i:] starts with an HTML entity reference.
func entityAt(s string, i int) bool {
	return entityRe.MatchString(s[i:])
}

var entityRe = regexp.MustCompile(`^&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{0,31});`)

func htmlEscape(s string) string {
	return htmlEscaper.Replace(s)
}

// htmlEscaper also escapes "|", which would split a pipe-table cell.
// It also escapes brackets, which would end a link label or citation.
var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "|", "&#124;", "[", "&#91;", "]", "&#93;")

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

var _ = fmt.Sprint
