package markdown

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/askrejans/crowdoc/v2/ast"
)

// ---------------------------------------------------------------------------
// Custom goldmark nodes
// ---------------------------------------------------------------------------

var (
	kindMath      = gast.NewNodeKind("CDMath")
	kindCite      = gast.NewNodeKind("CDCite")
	kindDiv       = gast.NewNodeKind("CDDiv")
	kindHighlight = gast.NewNodeKind("CDHighlight")
	kindSuper     = gast.NewNodeKind("CDSuperscript")
)

type mathNode struct {
	gast.BaseInline
	TeX     string
	Display bool
}

func (n *mathNode) Kind() gast.NodeKind                { return kindMath }
func (n *mathNode) Dump(source []byte, level int)      { gast.DumpHelper(n, source, level, nil, nil) }
func (n *citeNode) Kind() gast.NodeKind                { return kindCite }
func (n *citeNode) Dump(source []byte, level int)      { gast.DumpHelper(n, source, level, nil, nil) }
func (n *divNode) Kind() gast.NodeKind                 { return kindDiv }
func (n *divNode) Dump(source []byte, level int)       { gast.DumpHelper(n, source, level, nil, nil) }
func (n *highlightNode) Kind() gast.NodeKind           { return kindHighlight }
func (n *highlightNode) Dump(source []byte, level int) { gast.DumpHelper(n, source, level, nil, nil) }
func (n *superNode) Kind() gast.NodeKind               { return kindSuper }
func (n *superNode) Dump(source []byte, level int)     { gast.DumpHelper(n, source, level, nil, nil) }

type citeNode struct {
	gast.BaseInline
	Cite ast.Cite
	Raw  string
}

type divNode struct {
	gast.BaseBlock
	Attr  ast.Attr
	Title string
}

type highlightNode struct{ gast.BaseInline }

// subNode is Pandoc's ~subscript~ (single tildes, no spaces inside).
type subNode struct {
	gast.BaseInline
	Value string
}

var kindSub = gast.NewNodeKind("CDSubscript")

func (n *subNode) Kind() gast.NodeKind           { return kindSub }
func (n *subNode) Dump(source []byte, level int) { gast.DumpHelper(n, source, level, nil, nil) }

type subParser struct{}

func (subParser) Trigger() []byte { return []byte{'~'} }

func (subParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, _ := block.PeekLine()
	if len(line) < 3 || line[1] == '~' || block.PrecendingCharacter() == '~' {
		return nil
	}
	for i := 1; i < len(line); i++ {
		switch line[i] {
		case ' ', '\t', '\n', '\r':
			return nil
		case '~':
			if i == 1 || (i+1 < len(line) && line[i+1] == '~') {
				return nil
			}
			block.Advance(i + 1)
			return &subNode{Value: string(line[1:i])}
		}
	}
	return nil
}

type inlineNoteNode struct {
	gast.BaseInline
	Raw string
}

var kindInlineNote = gast.NewNodeKind("CDInlineNote")

func (n *inlineNoteNode) Kind() gast.NodeKind           { return kindInlineNote }
func (n *inlineNoteNode) Dump(source []byte, level int) { gast.DumpHelper(n, source, level, nil, nil) }

type superNode struct{ gast.BaseInline }

// ---------------------------------------------------------------------------
// Math: $…$, $$…$$, \(…\), \[…\]
// ---------------------------------------------------------------------------

type mathParser struct{}

func (mathParser) Trigger() []byte { return []byte{'$', '\\'} }

func (mathParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, _ := block.PeekLine()
	if len(line) < 2 {
		return nil
	}
	switch {
	case line[0] == '$' && line[1] == '$':
		tex, ok := scanUntil(block, 2, "$$", false)
		if !ok || strings.TrimSpace(tex) == "" {
			return nil
		}
		return &mathNode{TeX: strings.TrimSpace(tex), Display: true}
	case line[0] == '$':
		if isSpaceByte(line[1]) {
			return nil
		}
		tex, ok := scanInlineDollar(block)
		if !ok {
			return nil
		}
		return &mathNode{TeX: tex}
	case line[0] == '\\' && (line[1] == '(' || line[1] == '['):
		closer, display := `\)`, false
		if line[1] == '[' {
			closer, display = `\]`, true
		}
		save, seg := block.Position()
		tex, ok := scanUntil(block, 2, closer, true)
		// "\[1\]" is far more often an escaped bracket than mathematics, so
		// only accept spans that contain something math-like.
		if !ok || !looksLikeMath(tex) {
			block.SetPosition(save, seg)
			return nil
		}
		return &mathNode{TeX: strings.TrimSpace(tex), Display: display}
	}
	return nil
}

func isSpaceByte(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

func looksLikeMath(s string) bool {
	return strings.ContainsAny(s, `\^_=+{}<>|`) || (len(strings.TrimSpace(s)) <= 3 && strings.TrimSpace(s) != "" && !isDigits(strings.TrimSpace(s)))
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// scanUntil consumes the opener (skip bytes) and everything up to and
// including closer, possibly across lines of the current block. Backslash
// escapes inside the span are kept verbatim (they are TeX).
func scanUntil(block text.Reader, skip int, closer string, allowEscapedCloser bool) (string, bool) {
	saveLine, saveSeg := block.Position()
	block.Advance(skip)
	var sb strings.Builder
	for {
		line, _ := block.PeekLine()
		if line == nil {
			block.SetPosition(saveLine, saveSeg)
			return "", false
		}
		for i := 0; i < len(line); i++ {
			if line[i] == '\\' && !allowEscapedCloser && i+1 < len(line) {
				// \$ inside $$…$$ is a literal dollar in TeX.
				sb.WriteByte(line[i])
				sb.WriteByte(line[i+1])
				i++
				continue
			}
			if bytes.HasPrefix(line[i:], []byte(closer)) {
				block.Advance(i + len(closer))
				return sb.String(), true
			}
			if allowEscapedCloser && line[i] == '\\' && i+1 < len(line) && !bytes.HasPrefix(line[i:], []byte(closer)) {
				sb.WriteByte(line[i])
				sb.WriteByte(line[i+1])
				i++
				continue
			}
			sb.WriteByte(line[i])
		}
		block.AdvanceLine()
	}
}

// scanInlineDollar implements Pandoc's tex_math_dollars rules: the closing
// $ must follow a non-space character and must not be followed by a digit.
func scanInlineDollar(block text.Reader) (string, bool) {
	saveLine, saveSeg := block.Position()
	block.Advance(1)
	var sb strings.Builder
	for {
		line, _ := block.PeekLine()
		if line == nil {
			block.SetPosition(saveLine, saveSeg)
			return "", false
		}
		for i := 0; i < len(line); i++ {
			c := line[i]
			if c == '\\' && i+1 < len(line) {
				sb.WriteByte(c)
				sb.WriteByte(line[i+1])
				i++
				continue
			}
			if c == '$' {
				prev := byte(0)
				if sb.Len() > 0 {
					prev = sb.String()[sb.Len()-1]
				}
				next := byte(0)
				if i+1 < len(line) {
					next = line[i+1]
				}
				if sb.Len() == 0 || isSpaceByte(prev) || (next >= '0' && next <= '9') {
					block.SetPosition(saveLine, saveSeg)
					return "", false
				}
				block.Advance(i + 1)
				return sb.String(), true
			}
			sb.WriteByte(c)
		}
		block.AdvanceLine()
	}
}

// ---------------------------------------------------------------------------
// Citations: [see @doe2020, p. 33; @roe2019], @doe2020, @doe2020 [p. 4]
// ---------------------------------------------------------------------------

var (
	citeKeyRe   = regexp.MustCompile(`^[\p{L}\p{N}_](?:[\p{L}\p{N}_:.#$%&+?<>~/-]*[\p{L}\p{N}_])?`)
	locLabelMap = map[string]string{
		"p.": "page", "pp.": "page", "page": "page", "pages": "page", "lpp.": "page", "s.": "page",
		"chap.": "chapter", "chapter": "chapter", "chapters": "chapter", "ch.": "chapter",
		"sec.": "section", "section": "section", "sections": "section", "§": "section", "§§": "section",
		"fig.": "figure", "figure": "figure", "figures": "figure",
		"tab.": "table", "table": "table", "tables": "table",
		"para.": "paragraph", "paragraph": "paragraph", "paragraphs": "paragraph", "¶": "paragraph",
		"l.": "line", "ll.": "line", "line": "line", "lines": "line",
		"vol.": "volume", "vols.": "volume", "volume": "volume", "volumes": "volume",
		"n.": "note", "nn.": "note", "note": "note", "notes": "note", "nr.": "number", "no.": "number",
	}
)

type citeParser struct{}

func (citeParser) Trigger() []byte { return []byte{'[', '@'} }

func (citeParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, _ := block.PeekLine()
	if len(line) == 0 {
		return nil
	}
	if line[0] == '@' {
		return parseNarrativeCite(block)
	}
	return parseBracketCite(block)
}

func parseNarrativeCite(block text.Reader) gast.Node {
	prev := block.PrecendingCharacter()
	if prev != '\n' && prev != utf8.RuneError && (unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '_' || prev == '@' || prev == '/' || prev == '.') {
		return nil
	}
	line, _ := block.PeekLine()
	key := citeKeyRe.Find(line[1:])
	if key == nil {
		return nil
	}
	n := 1 + len(key)
	item := ast.CiteItem{Key: string(key)}
	raw := string(line[:n])
	// "@key [p. 33]": an adjacent bracket holds the locator.
	rest := line[n:]
	if len(rest) > 1 && rest[0] == ' ' && rest[1] == '[' {
		if end := bytes.IndexByte(rest, ']'); end > 2 {
			after := rest[end+1:]
			inner := string(rest[2:end])
			if (len(after) == 0 || (after[0] != '(' && after[0] != '[')) && !strings.Contains(inner, "@") {
				applySuffix(&item, inner)
				n += end + 1
				raw = string(line[:n])
			}
		}
	}
	block.Advance(n)
	return &citeNode{Cite: ast.Cite{Mode: ast.CiteNarrative, Items: []ast.CiteItem{item}}, Raw: raw}
}

func parseBracketCite(block text.Reader) gast.Node {
	line, _ := block.PeekLine()
	end := matchingBracket(line)
	if end < 0 {
		return nil
	}
	if end+1 < len(line) && (line[end+1] == '(' || line[end+1] == '[' || line[end+1] == ':') {
		return nil // a link, reference link or link definition
	}
	inner := string(line[1:end])
	if !strings.Contains(inner, "@") {
		return nil
	}
	var items []ast.CiteItem
	for _, part := range strings.Split(inner, ";") {
		item, ok := parseCiteItem(part)
		if !ok {
			return nil
		}
		items = append(items, item)
	}
	block.Advance(end + 1)
	return &citeNode{Cite: ast.Cite{Mode: ast.CiteParenthetical, Items: items}, Raw: string(line[:end+1])}
}

func matchingBracket(line []byte) int {
	depth := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case '`':
			// Brackets inside a code span do not count.
			i = skipCodeSpan(line, i) - 1
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		case '\n':
			return -1
		}
	}
	return -1
}

// skipCodeSpan returns the offset after the code span opening at i (or
// after the backtick run when it is not closed on the line).
func skipCodeSpan(line []byte, i int) int {
	n := 0
	for i+n < len(line) && line[i+n] == '`' {
		n++
	}
	for j := i + n; j < len(line); {
		if line[j] != '`' {
			j++
			continue
		}
		k := j
		for k < len(line) && line[k] == '`' {
			k++
		}
		if k-j == n {
			return k
		}
		j = k
	}
	return i + n
}

func parseCiteItem(part string) (ast.CiteItem, bool) {
	at := strings.IndexByte(part, '@')
	if at < 0 {
		return ast.CiteItem{}, false
	}
	prefix := part[:at]
	item := ast.CiteItem{}
	if strings.HasSuffix(prefix, "-") {
		item.SuppressAuthor = true
		prefix = prefix[:len(prefix)-1]
	}
	if at > 0 {
		r, _ := utf8.DecodeLastRuneInString(part[:at])
		if r != '-' && !unicode.IsSpace(r) && !unicode.IsPunct(r) || r == '\\' {
			return ast.CiteItem{}, false // e-mail address or escaped "@", not a citation
		}
	}
	item.Prefix = strings.TrimSpace(prefix)
	key := citeKeyRe.FindString(part[at+1:])
	if key == "" {
		return ast.CiteItem{}, false
	}
	item.Key = key
	applySuffix(&item, part[at+1+len(key):])
	return item, true
}

// applySuffix splits ", p. 33, emphasis added" into locator and suffix,
// following Pandoc: a known label or a leading number starts a locator.
func applySuffix(item *ast.CiteItem, s string) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), ","))
	if s == "" {
		return
	}
	label, rest := "", s
	lower := strings.ToLower(s)
	for _, l := range locLabels {
		if !strings.HasPrefix(lower, l) {
			continue
		}
		after := lower[len(l):]
		if after == "" || after[0] == ' ' || (after[0] >= '0' && after[0] <= '9') || strings.HasSuffix(l, ".") || strings.HasSuffix(l, "§") {
			label, rest = locLabelMap[l], strings.TrimSpace(s[len(l):])
			break
		}
	}
	loc, tail := splitLocator(rest)
	if loc == "" {
		item.Suffix = s
		return
	}
	if label == "" {
		label = "page"
	}
	item.Locator, item.LocatorLabel = loc, label
	item.Suffix = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tail), ","))
}

var locLabels = func() []string {
	out := make([]string, 0, len(locLabelMap))
	for k := range locLabelMap {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		return len(out[i]) > len(out[j]) || (len(out[i]) == len(out[j]) && out[i] < out[j])
	})
	return out
}()

// splitLocator returns the leading locator ("33", "33-35", "iv", "12, 14",
// "33f") and the remaining text.
func splitLocator(s string) (string, string) {
	end := 0
	roman := isRomanToken(s)
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '-' || r == '–' || r == '—' || r == '.' || r == ':':
		case roman && strings.ContainsRune("ivxlcdmIVXLCDM", r):
		case (r == 'f') && i > 0:
		case r == ',' || r == ' ':
			// continue only when more numbers follow ("12, 14")
			restTrim := strings.TrimLeft(s[i:], ", ")
			if restTrim == "" || restTrim[0] < '0' || restTrim[0] > '9' {
				return strings.TrimRight(s[:end], ".:-–— "), s[i:]
			}
		default:
			return strings.TrimRight(s[:end], ".:-–— "), s[i:]
		}
		end = i + utf8.RuneLen(r)
	}
	return strings.TrimRight(s[:end], ".:-–— "), s[end:]
}

func isRomanToken(s string) bool {
	tok := s
	if i := strings.IndexAny(s, " ,;"); i >= 0 {
		tok = s[:i]
	}
	tok = strings.ToLower(strings.TrimRight(tok, "."))
	if tok == "" {
		return false
	}
	for _, r := range tok {
		if !strings.ContainsRune("ivxlcdm", r) {
			return false
		}
	}
	return len(tok) <= 6
}

// ---------------------------------------------------------------------------
// Fenced divs: ::: warning / ::: {.note #id title="…"} / :::tip Title
// ---------------------------------------------------------------------------

type divParser struct{}

func (divParser) Trigger() []byte { return []byte{':'} }

func (divParser) Open(parent gast.Node, reader text.Reader, pc parser.Context) (gast.Node, parser.State) {
	line, seg := reader.PeekLine()
	trimmed := bytes.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return nil, parser.NoChildren
	}
	colons := 0
	for colons < len(trimmed) && trimmed[colons] == ':' {
		colons++
	}
	if colons < 3 {
		return nil, parser.NoChildren
	}
	// Closing colons on the opening line ("::: note :::") are dropped; a
	// title may still end in a colon ("::: note Read this:").
	spec := strings.TrimSpace(fenceTailRe.ReplaceAllString(strings.TrimSpace(string(trimmed[colons:])), ""))
	if spec == "" {
		return nil, parser.NoChildren // a closing fence without an open div
	}
	node := &divNode{}
	if strings.HasPrefix(spec, "{") {
		end := strings.LastIndexByte(spec, '}')
		if end < 0 {
			return nil, parser.NoChildren
		}
		node.Attr = parseAttrString(spec[1:end])
		node.Title = strings.TrimSpace(spec[end+1:])
	} else {
		word, title, _ := strings.Cut(spec, " ")
		node.Attr.Classes = []string{strings.ToLower(strings.Trim(word, "{}."))}
		node.Title = strings.TrimSpace(title)
	}
	if t := node.Attr.Get("title"); t != "" && node.Title == "" {
		node.Title = t
		delete(node.Attr.KV, "title")
		if len(node.Attr.KV) == 0 {
			node.Attr.KV = nil
		}
	}
	reader.Advance(seg.Len() - 1)
	return node, parser.HasChildren
}

func (divParser) Continue(node gast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, seg := reader.PeekLine()
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) >= 3 && len(bytes.Trim(trimmed, ":")) == 0 {
		// The innermost open div owns a closing fence; inside a fenced
		// code block the line is code.
		for _, b := range pc.OpenedBlocks() {
			if b.Node != node && (b.Node.Kind() == kindDiv || b.Node.Kind() == gast.KindFencedCodeBlock) && isDescendant(b.Node, node) {
				return parser.Continue | parser.HasChildren
			}
		}
		reader.Advance(seg.Len() - 1)
		return parser.Close
	}
	return parser.Continue | parser.HasChildren
}

func isDescendant(n, ancestor gast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p == ancestor {
			return true
		}
	}
	return false
}

func (divParser) Close(node gast.Node, reader text.Reader, pc parser.Context) {}
func (divParser) CanInterruptParagraph() bool                                 { return true }
func (divParser) CanAcceptIndentedLine() bool                                 { return false }

var fenceTailRe = regexp.MustCompile(`(?:\s+:+|:{3,})$`)

// parseAttrString parses Pandoc attributes: #id .class key=value
// key="v w". Quotes inside a quoted value are written with a backslash when
// the attributes come from a text run (marked by escapeMark).
func parseAttrString(s string) ast.Attr {
	var a ast.Attr
	for {
		s = strings.TrimLeft(s, " \t")
		if s == "" {
			break
		}
		tok, rest := nextAttrToken(s)
		s = rest
		switch {
		case tok == "-":
			a.Classes = append(a.Classes, "unnumbered")
		case strings.HasPrefix(tok, "#"):
			a.ID = dropMarks(tok[1:])
		case strings.HasPrefix(tok, "."):
			a.Classes = append(a.Classes, dropMarks(tok[1:]))
		case strings.Contains(tok, "="):
			k, v, _ := strings.Cut(tok, "=")
			if a.KV == nil {
				a.KV = map[string]string{}
			}
			a.KV[strings.ToLower(dropMarks(k))] = dropMarks(unquoteAttr(v))
		case tok != "":
			a.Classes = append(a.Classes, dropMarks(tok))
		}
	}
	return a
}

// nextAttrToken splits off one attribute; a quoted value may hold spaces.
func nextAttrToken(s string) (string, string) {
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' {
		if q := s[i]; (q == '"' || q == '\'') && i > 0 && s[i-1] == '=' {
			j := i + 1
			for j < len(s) && (s[j] != q || isEscapedAt(s, j)) {
				j++
			}
			i = min(j+1, len(s))
			continue
		}
		i++
	}
	return s[:i], s[i:]
}

// unquoteAttr removes the quotes around a value.
func unquoteAttr(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] && !isEscapedAt(v, len(v)-1) {
		return v[1 : len(v)-1]
	}
	return strings.Trim(v, `"'`)
}

func dropMarks(s string) string { return strings.ReplaceAll(s, string(escapeMark), "") }

// ---------------------------------------------------------------------------
// ==highlight== and ^superscript^
// ---------------------------------------------------------------------------

type charDelim struct {
	char   byte
	length int
	make   func() gast.Node
}

func (d *charDelim) IsDelimiter(b byte) bool { return b == d.char }
func (d *charDelim) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}
func (d *charDelim) OnMatch(consumes int) gast.Node { return d.make() }

var (
	highlightDelim = &charDelim{char: '=', length: 2, make: func() gast.Node { return &highlightNode{} }}
	superDelim     = &charDelim{char: '^', length: 1, make: func() gast.Node { return &superNode{} }}
)

type delimParser struct{ d *charDelim }

func (p delimParser) Trigger() []byte { return []byte{p.d.char} }

func (p delimParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	if p.d.char == '^' && len(line) > 1 && line[1] == '[' {
		// Pandoc inline note: ^[text]
		if end := matchingBracket(line[1:]); end > 0 {
			block.Advance(end + 2)
			return &inlineNoteNode{Raw: string(line[2 : end+1])}
		}
	}
	node := parser.ScanDelimiter(line, before, p.d.length, p.d)
	if node == nil || node.OriginalLength != p.d.length || before == rune(p.d.char) {
		return nil
	}
	// ^ never opens or closes next to whitespace (x ^ y stays literal).
	if p.d.char == '^' {
		next := byte(' ')
		if len(line) > 1 {
			next = line[1]
		}
		if isSpaceByte(next) && node.CanOpen && !node.CanClose {
			return nil
		}
	}
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (p delimParser) CloseBlock(parent gast.Node, pc parser.Context) {}

// Ensure util is referenced for priorities in markdown.go.
var _ = util.Prioritized
