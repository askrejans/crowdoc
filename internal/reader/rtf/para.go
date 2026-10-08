package rtf

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// rtfCtx is an independent text flow: the document body, a footnote or a
// text box. Each has its own pending paragraph and open tables.
type rtfCtx struct {
	flow   *flow
	para   paraBuf
	tables []*tableBuilder
	after  []ast.Block // blocks (text boxes) emitted after the current paragraph
}

type paraBuf struct {
	items       []item
	listText    string
	hasListText bool
	pn          *pnInfo
}

func (b *paraBuf) empty() bool { return len(b.items) == 0 && !b.hasListText && b.pn == nil }

// Formatting flags of a run.
const (
	fBold uint16 = 1 << iota
	fItalic
	fUnderline
	fStrike
	fSuper
	fSub
	fSmallCaps
	fHighlight
	fCode
)

// wrapOrder is the nesting order of formatting wrappers (outermost first).
var wrapOrder = []uint16{fBold, fItalic, fUnderline, fStrike, fSuper, fSub, fSmallCaps, fHighlight}

type fmtKey struct {
	flags uint16
	link  int
	size  int
}

// item is one run of a paragraph: text with formatting, or a ready inline
// (line break, footnote, image, anchor).
type item struct {
	text    []byte
	f       fmtKey
	in      ast.Inline
	neutral bool // takes the formatting of its neighbours (notes, anchors)
}

func (p *parser) fmtKey() fmtKey {
	c := &p.st().chr
	var fl uint16
	if c.bold {
		fl |= fBold
	}
	if c.italic {
		fl |= fItalic
	}
	if c.underline {
		fl |= fUnderline
	}
	if c.strike {
		fl |= fStrike
	}
	switch c.vert {
	case 1:
		fl |= fSuper
	case -1:
		fl |= fSub
	}
	if c.smallcaps {
		fl |= fSmallCaps
	}
	if c.highlight {
		fl |= fHighlight
	}
	if c.codeStyle || p.fontMono(c.font) {
		fl |= fCode
	}
	return fmtKey{flags: fl, link: c.link, size: c.size}
}

func (p *parser) fontMono(id int) bool {
	f := p.fonts[id]
	return f != nil && f.mono
}

// paraText appends decoded text to the current paragraph.
func (p *parser) paraText(s string) {
	if p.st().chr.hidden {
		return
	}
	clean := true
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 && c != '\t' || c == 0xE2 || c == 0xEF {
			clean = false
			break
		}
	}
	if clean {
		p.appendText(s)
		return
	}
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		special := r < 0x20 && r != '\t' || r == '\u2028' || r == '\u2029' || r == '\ufffc' || r == '\ufeff'
		if !special {
			i += size
			continue
		}
		if start < i {
			p.appendText(s[start:i])
		}
		switch r {
		case '\u2028':
			p.paraInline(&ast.LineBreak{})
		case '\u2029':
			p.endParagraph()
		}
		i += size
		start = i
	}
	if start < len(s) {
		p.appendText(s[start:])
	}
}

func (p *parser) appendText(s string) {
	if s == "" {
		return
	}
	c := p.cur()
	f := p.fmtKey()
	items := c.para.items
	if n := len(items); n > 0 && items[n-1].in == nil && items[n-1].f == f {
		items[n-1].text = append(items[n-1].text, s...)
	} else {
		c.para.items = append(items, item{text: []byte(s), f: f})
	}
	p.langChars[p.st().chr.lang] += len(s)
}

// paraInline appends a ready-made inline to the current paragraph.
func (p *parser) paraInline(in ast.Inline) {
	it := item{in: in}
	switch in.(type) {
	case *ast.LineBreak:
		it.f = p.fmtKey()
	case *ast.Image:
		it.f.link = p.st().chr.link
	default:
		it.neutral = true
	}
	c := p.cur()
	c.para.items = append(c.para.items, it)
}

func noteInline(blocks []ast.Block) ast.Inline { return &ast.Note{Blocks: blocks} }

func anchor(id string) ast.Inline { return &ast.Span{Attr: ast.Attr{ID: id}} }

func anchorID(in ast.Inline) (string, bool) {
	s, ok := in.(*ast.Span)
	if !ok || s.Attr.ID == "" || len(s.Inlines) > 0 || len(s.Attr.Classes) > 0 {
		return "", false
	}
	return s.Attr.ID, true
}

// sanitizeID turns a bookmark name into an identifier made of lowercase
// ASCII letters, digits, '-', '_', ':' and '.'.
func sanitizeID(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 'a' - 'A')
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == ':', r == '.':
			b.WriteRune(r)
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = true
			continue
		}
		dash = false
	}
	return strings.TrimRight(b.String(), "-")
}

// ---------------------------------------------------------------------------
// Contexts, paragraphs and tables
// ---------------------------------------------------------------------------

func (p *parser) pushCtx() {
	p.ctxs = append(p.ctxs, &rtfCtx{flow: newFlow(p, false)})
}

// popCtxWith finishes the innermost context using props for its last
// paragraph and returns its blocks.
func (p *parser) popCtxWith(props paraState) []ast.Block {
	if len(p.ctxs) <= 1 {
		return nil
	}
	c := p.cur()
	if !c.para.empty() {
		p.finishPara(c, props, 0)
	}
	p.syncTables(c, 0)
	p.ctxs = p.ctxs[:len(p.ctxs)-1]
	return c.flow.finish()
}

func (p *parser) popCtx() {
	blocks := p.popCtxWith(defaultPara())
	if len(blocks) > 0 {
		p.paraInline(noteInline(blocks))
	}
}

func (p *parser) endParagraph() {
	p.finishPara(p.cur(), p.st().para, 0)
}

// finishPara ends the pending paragraph of c and places it in the body or
// in the current cell of the table at the paragraph's nesting depth.
func (p *parser) finishPara(c *rtfCtx, props paraState, minDepth int) {
	pb := c.para
	c.para = paraBuf{}
	depth := max(props.depth(), minDepth)
	p.syncTables(c, depth)
	rec := p.record(&pb, props, c == p.ctxs[0] && depth == 0)
	target := c.flow
	if depth > 0 && rec != nil {
		target = c.tables[depth-1].cell(p)
	}
	if rec != nil {
		target.add(rec)
	}
	if len(c.after) > 0 {
		if depth > 0 {
			target = c.tables[depth-1].cell(p)
		}
		for _, b := range c.after {
			target.addBlock(b)
		}
		c.after = nil
	}
}

// syncTables opens or closes tables so that exactly depth are open.
func (p *parser) syncTables(c *rtfCtx, depth int) {
	for len(c.tables) > depth {
		t := c.tables[len(c.tables)-1]
		c.tables = c.tables[:len(c.tables)-1]
		tbl := t.build()
		if tbl == nil {
			continue
		}
		if len(c.tables) > 0 {
			c.tables[len(c.tables)-1].cell(p).addBlock(tbl)
		} else {
			c.flow.addBlock(tbl)
		}
	}
	for len(c.tables) < depth {
		c.tables = append(c.tables, &tableBuilder{})
	}
}

func (p *parser) cellEnd(level int) {
	c := p.cur()
	p.finishPara(c, p.st().para, level)
	p.syncTables(c, level)
	c.tables[level-1].endCell()
}

func (p *parser) rowEnd(level int) {
	c := p.cur()
	if !c.para.empty() {
		p.finishPara(c, p.st().para, level)
	} else {
		c.para = paraBuf{}
	}
	p.syncTables(c, level)
	c.tables[level-1].endRow(p.rowDef(level))
}

func (p *parser) pageBreak() {
	c := p.cur()
	if p.st().para.depth() > 0 || c != p.ctxs[0] {
		return // page breaks inside tables and notes are meaningless here
	}
	if !c.para.empty() {
		p.endParagraph()
	}
	p.syncTables(c, 0)
	c.flow.addBlock(&ast.PageBreak{})
}

// ---------------------------------------------------------------------------
// Paragraph classification
// ---------------------------------------------------------------------------

type recKind uint8

const (
	rNormal recKind = iota
	rEmpty
	rHeading
	rCode
	rQuote
	rCaption
	rTitle
	rSubtitle
	rFigure
)

// paraRec is a finished paragraph ready for the flow builder.
type paraRec struct {
	kind            recKind
	level           int
	inlines         []ast.Inline
	items           []item // kept for size-based heading candidates
	code            string
	id              string
	list            *listRef
	align           ast.Align
	indent          int
	pageBreakBefore bool
	codeBlank       bool
	image           *ast.Image
	textLen         int
	boldLen         int
	minSize         int
	candidate       bool
	boldHead        bool // short all-bold line: heading candidate without styles
}

type listRef struct {
	key     string
	level   int
	ordered bool
	style   ast.NumberStyle
	start   int
}

func (p *parser) record(pb *paraBuf, props paraState, body bool) *paraRec {
	kind, level := skNone, 0
	if sty := p.resolve(props.style); sty != nil {
		kind, level = sty.kind, sty.level
	}
	if props.outline >= 0 && props.outline <= 8 && kind != skTitle && kind != skSubtitle && kind != skTOC {
		kind, level = skHeading, props.outline+1
	}
	rec := &paraRec{align: props.align, indent: props.li, pageBreakBefore: props.pagebb}

	textLen, monoLen, boldLen, others, images, breaks := 0, 0, 0, 0, 0, 0
	minSize := 0
	var image *ast.Image
	for _, it := range pb.items {
		if it.in != nil {
			switch in := it.in.(type) {
			case *ast.Image:
				images++
				image = in
			case *ast.LineBreak:
				breaks++
			default:
				if _, ok := anchorID(in); !ok {
					others++
				}
			}
			continue
		}
		n := nonSpaceLen(it.text)
		if n == 0 {
			continue
		}
		textLen += n
		if it.f.flags&fCode != 0 {
			monoLen += n
		}
		if it.f.flags&fBold != 0 {
			boldLen += n
		}
		if minSize == 0 || it.f.size < minSize {
			minSize = it.f.size
		}
	}
	rec.textLen, rec.boldLen, rec.minSize = textLen, boldLen, minSize

	if textLen == 0 && images == 0 && others == 0 {
		rec.kind = rEmpty
		rec.codeBlank = kind == skCode || p.fontMono(p.st().chr.font)
		return rec
	}
	if kind == skTOC {
		return nil
	}
	if kind == skHeading {
		p.styleHeads = true
		if !body {
			kind = skNone // no sectioning inside table cells and notes
		}
	}

	switch {
	case (kind == skTitle || kind == skSubtitle) && body && images == 0:
		rec.kind = rTitle
		if kind == skSubtitle {
			rec.kind = rSubtitle
		}
		rec.inlines = p.inlines(pb.items, fBold|fItalic|fUnderline)
		return rec
	case kind == skHeading:
		rec.kind, rec.level = rHeading, min(max(level, 1), 9)
		items, id := extractAnchors(pb.items)
		rec.id = id
		rec.inlines = p.inlines(items, fBold|uniformFlags(items))
		return rec
	}
	rec.list = p.listRef(pb, props)
	if rec.list == nil && (kind == skCode || textLen > 0 && monoLen == textLen && images == 0 && others == 0) {
		rec.kind = rCode
		rec.code = rawText(pb.items)
		return rec
	}

	if body {
		for _, it := range pb.items {
			if it.in == nil {
				p.sizeChars[it.f.size] += nonSpaceLen(it.text)
			}
		}
	}
	rec.inlines = p.inlines(pb.items, 0)
	if len(rec.inlines) == 0 {
		return nil
	}
	if rec.list == nil && kind == skNone {
		if rest, ok := stripBullet(rec.inlines); ok {
			rec.inlines = rest
			rec.list = &listRef{key: "txt"}
		}
	}
	if rec.list != nil {
		return rec
	}
	switch {
	case images == 1 && textLen == 0 && others == 0 && body:
		rec.kind = rFigure
		rec.image = image
	case kind == skQuote:
		rec.kind = rQuote
	case kind == skCaption:
		rec.kind = rCaption
		rec.inlines = p.inlines(pb.items, uniformFlags(pb.items))
	default:
		rec.kind = rNormal
		if body && breaks == 0 && images == 0 && others == 0 && textLen <= 150 {
			plain := ast.PlainText(rec.inlines)
			last, _ := utf8.DecodeLastRuneInString(plain)
			if !strings.ContainsRune(".,;", last) {
				rec.candidate = true
				rec.items = pb.items
				rec.boldHead = boldLen == textLen && textLen <= 80 && !strings.ContainsRune(":!?", last)
			}
		}
	}
	return rec
}

// stripBullet recognises a paragraph typed with a literal bullet glyph
// ("• item") and returns its content without the marker.
func stripBullet(ins []ast.Inline) ([]ast.Inline, bool) {
	t, ok := ins[0].(*ast.Text)
	if !ok {
		return nil, false
	}
	r, size := utf8.DecodeRuneInString(t.Value)
	switch r {
	case '•', '◦', '▪', '·', '‣', '⁃', '●', '○', '■', '□', '➢', '✓':
	default:
		return nil, false
	}
	rest := t.Value[size:]
	if !strings.HasPrefix(rest, " ") {
		return nil, false
	}
	out := ast.TrimInlines(append([]ast.Inline{&ast.Text{Value: rest}}, ins[1:]...))
	return out, len(out) > 0
}

func nonSpaceLen(b []byte) int {
	n := 0
	for _, r := range string(b) {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}

// uniformFlags returns the formatting flags shared by every text run, so
// headings drop formatting that merely styles the whole heading.
func uniformFlags(items []item) uint16 {
	common := ^uint16(0)
	seen := false
	for _, it := range items {
		if it.in != nil || nonSpaceLen(it.text) == 0 {
			continue
		}
		common &= it.f.flags
		seen = true
	}
	if !seen {
		return 0
	}
	return common &^ fCode
}

// extractAnchors removes bookmark anchors from a heading's runs and picks
// the most meaningful one as the heading id.
func extractAnchors(items []item) ([]item, string) {
	out := make([]item, 0, len(items))
	id := ""
	for _, it := range items {
		if it.in != nil {
			if a, ok := anchorID(it.in); ok {
				if id == "" || strings.HasPrefix(id, "_") && !strings.HasPrefix(a, "_") {
					id = a
				}
				continue
			}
		}
		out = append(out, it)
	}
	return out, id
}

func rawText(items []item) string {
	var b strings.Builder
	for _, it := range items {
		if it.in != nil {
			if _, ok := it.in.(*ast.LineBreak); ok {
				b.WriteByte('\n')
			}
			continue
		}
		b.Write(it.text)
	}
	return strings.TrimRight(b.String(), " \t")
}

// listRef determines list membership from the list table (\ls/\ilvl), old
// style \pn numbering or, failing both, the \listtext marker.
func (p *parser) listRef(pb *paraBuf, props paraState) *listRef {
	var ref *listRef
	if props.ls > 0 {
		if def := p.lists[p.overrides[props.ls]]; def != nil {
			lv := listLevelDef{nfc: 23, start: 1}
			if props.ilvl < len(def.levels) {
				lv = def.levels[props.ilvl]
			}
			if lv.nfc == 255 {
				return nil // a level without marker: plain indented text
			}
			ordered, style := nfcStyle(lv.nfc)
			ref = &listRef{key: "ls" + strconv.Itoa(props.ls), level: props.ilvl, ordered: ordered, style: style, start: max(lv.start, 0)}
		}
	}
	if ref == nil && pb.pn != nil && !pb.pn.cont {
		pn := pb.pn
		ref = &listRef{key: "pn", level: pn.level, ordered: !pn.bullet, style: pnStyle(pn.style), start: max(pn.start, 1)}
	}
	if ref == nil && pb.hasListText {
		// Old-style numbering puts \pn only on a list's first paragraph; later items
		// carry just the marker text, so both share one key.
		ordered, style, _, _ := parseListText(pb.listText)
		ref = &listRef{key: "pn", level: props.ilvl, ordered: ordered, style: style, start: 1}
	}
	if ref == nil {
		return nil
	}
	counters := p.listCounters[ref.key]
	if counters == nil {
		counters = new([9]int)
		p.listCounters[ref.key] = counters
	}
	lvl := min(ref.level, 8)
	if _, _, n, ok := parseListText(pb.listText); ok && ref.ordered {
		counters[lvl] = n
	} else if counters[lvl] == 0 {
		counters[lvl] = ref.start
	} else {
		counters[lvl]++
	}
	for i := lvl + 1; i < len(counters); i++ {
		counters[i] = 0
	}
	ref.start = counters[lvl]
	return ref
}

func nfcStyle(nfc int) (bool, ast.NumberStyle) {
	switch nfc {
	case 1:
		return true, ast.NumberUpperRoman
	case 2:
		return true, ast.NumberLowerRoman
	case 3:
		return true, ast.NumberUpperAlpha
	case 4:
		return true, ast.NumberLowerAlpha
	case 23, 255:
		return false, ast.NumberDecimal
	}
	return true, ast.NumberDecimal
}

func pnStyle(s int) ast.NumberStyle {
	switch s {
	case 1:
		return ast.NumberUpperRoman
	case 2:
		return ast.NumberLowerRoman
	case 3:
		return ast.NumberUpperAlpha
	case 4:
		return ast.NumberLowerAlpha
	}
	return ast.NumberDecimal
}

// parseListText interprets a rendered list marker such as "3.", "b)",
// "iv." or "•". num is the marker's value when ok.
func parseListText(s string) (ordered bool, style ast.NumberStyle, num int, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "(")
	s = strings.TrimRight(s, ".):")
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		s = s[i+1:] // "1.2.3" -> "3"
	}
	if s == "" {
		return false, ast.NumberDecimal, 0, false
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 0 {
		return true, ast.NumberDecimal, n, true
	}
	if n := romanValue(s); n > 0 && (len(s) > 1 || s == "i" || s == "I") {
		if s == strings.ToUpper(s) {
			return true, ast.NumberUpperRoman, n, true
		}
		return true, ast.NumberLowerRoman, n, true
	}
	if len(s) == 1 {
		switch c := s[0]; {
		case c >= 'a' && c <= 'z':
			return true, ast.NumberLowerAlpha, int(c-'a') + 1, true
		case c >= 'A' && c <= 'Z':
			return true, ast.NumberUpperAlpha, int(c-'A') + 1, true
		}
	}
	return false, ast.NumberDecimal, 0, false
}

func romanValue(s string) int {
	vals := map[byte]int{'i': 1, 'v': 5, 'x': 10, 'l': 50, 'c': 100, 'd': 500, 'm': 1000}
	lower := strings.ToLower(s)
	if lower != s && strings.ToUpper(s) != s {
		return 0
	}
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
