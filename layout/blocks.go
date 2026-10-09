package layout

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

type pkind int

const (
	pPara pkind = iota
	pHeading
	pList
	pCode
	pTable
	pFigure
	pMath
)

// proto is a block recognised in a flow, before cross-flow merging and
// conversion to the AST.
type proto struct {
	kind  pkind
	lines []*line
	flow  *flow

	quote    bool
	indented bool
	hanging  bool

	numDepth int
	level    int // heading level given by the producer, or 0
	hstyle   headingStyle

	list    *listNode
	table   *ast.Table
	img     *pageImage
	caption []*line

	firstText, lastText bool
}

func (p *proto) isFloat() bool { return p.kind == pFigure || p.kind == pTable }

type headingStyle struct {
	size float64
	bold bool
}

// processFlow classifies the items of one flow into blocks.
func (d *doc) processFlow(f *flow) []*proto {
	var out []*proto
	items := f.items
	tables := d.findTables(f)
	for i := 0; i < len(items); {
		it := items[i]
		if it.img != nil {
			p := &proto{kind: pFigure, img: it.img, flow: f}
			next := i + 1
			if next < len(items) && items[next].l != nil && d.captionNear(items[next].l, it.img.y1, "figure") {
				p.caption, next = d.collectCaption(items, next, f)
			} else if n := len(out); n > 0 && out[n-1].kind == pPara && d.captionAbove(out[n-1], it.img.y0, "figure") {
				p.caption = out[n-1].lines
				out = out[:n-1]
			}
			out = append(out, p)
			i = next
			continue
		}
		if r, ok := tables[i]; ok {
			p := &proto{kind: pTable, flow: f, table: d.buildTable(items[r.start:r.end], r.seps)}
			for _, x := range items[r.start:r.end] {
				p.lines = append(p.lines, x.l)
			}
			next := r.end
			top, bottom := items[r.start].l.y0, items[r.end-1].l.y1
			if n := len(out); n > 0 && out[n-1].kind == pPara && d.captionAbove(out[n-1], top, "table") {
				p.caption = out[n-1].lines
				out = out[:n-1]
			} else if next < len(items) && items[next].l != nil && d.captionNear(items[next].l, bottom, "table") {
				p.caption, next = d.collectCaption(items, next, f)
			}
			out = append(out, p)
			i = next
			continue
		}
		l := it.l
		switch {
		case d.isMathLine(l, f):
			p, next := d.collectMath(items, i, f)
			out = append(out, p)
			i = next
		case d.isCode(l):
			p, next := d.collectCode(items, i, f)
			out = append(out, p)
			i = next
		case d.plainHeading(items, i):
			p := &proto{kind: pHeading, flow: f, lines: []*line{l}, hstyle: headingStyle{size: roundTo(l.size, 0.5)}}
			if len(l.words) > 1 {
				p.numDepth = numberingDepth(l.words[0].text)
			}
			out = append(out, p)
			i++
		case d.isHeadingLine(l):
			if p, next, ok := d.collectHeading(items, i, f); ok {
				out = append(out, p)
				i = next
				continue
			}
			p, next := d.collectPara(items, i, f, tables)
			out = append(out, p)
			i = next
		default:
			if _, ok := d.lineMarker(l, f, prevLine(out)); ok {
				if p, next, ok := d.collectList(items, i, f, tables); ok {
					out = append(out, p)
					i = next
					continue
				}
			}
			p, next := d.collectPara(items, i, f, tables)
			out = append(out, p)
			i = next
		}
	}
	first, last := -1, -1
	for k, p := range out {
		if !p.isFloat() {
			if first < 0 {
				first = k
			}
			last = k
		}
	}
	if first >= 0 {
		out[first].firstText = true
		out[last].lastText = true
	}
	return out
}

// prevLine returns the last line of the last block, or nil.
func prevLine(ps []*proto) *line {
	if n := len(ps); n > 0 && len(ps[n-1].lines) > 0 && !ps[n-1].isFloat() {
		return ps[n-1].lines[len(ps[n-1].lines)-1]
	}
	return nil
}

// isCode reports whether a line is set entirely in a monospaced font in a
// document whose body text is not.
func (d *doc) isCode(l *line) bool {
	switch l.role() {
	case RoleCode:
		return true
	case RoleAuto, RoleParagraph:
	default:
		return false
	}
	if d.bodyMono {
		return false
	}
	_, _, m := l.styleFrac()
	return m >= 0.95
}

func (d *doc) collectCode(items []flowItem, i int, f *flow) (*proto, int) {
	first := items[i].l
	p := &proto{kind: pCode, flow: f, lines: []*line{first}}
	blk := 0
	if first.role() == RoleCode {
		blk = first.block()
	}
	j := i + 1
	for j < len(items) {
		n := items[j].l
		if n == nil || !d.isCode(n) {
			break
		}
		if blk != 0 && n.block() != blk {
			break
		}
		prev := p.lines[len(p.lines)-1]
		if blk == 0 && n.base-prev.base > 4*d.pitchFor(prev.size) {
			break
		}
		p.lines = append(p.lines, n)
		j++
	}
	return p, j
}

// codeText reconstructs preformatted text from monospaced words: the
// character width turns gaps back into spaces and indentation.
func (d *doc) codeText(lines []*line) string {
	var widths []float64
	minX := math.Inf(1)
	for _, l := range lines {
		minX = min(minX, l.x0)
		for _, w := range l.words {
			if n := utf8.RuneCountInString(w.text); n > 0 {
				widths = append(widths, w.w()/float64(n))
			}
		}
	}
	cw := median(widths)
	if cw <= 0 {
		cw = 0.6 * d.body
	}
	pitch := d.pitchFor(lines[0].size)
	var sb strings.Builder
	for li, l := range lines {
		if li > 0 {
			sb.WriteByte('\n')
			gap := l.base - lines[li-1].base
			for k := 1; k < 4 && gap > pitch*(float64(k)+0.6); k++ {
				sb.WriteByte('\n')
			}
		}
		sb.WriteString(strings.Repeat(" ", int(math.Round((l.x0-minX)/cw))))
		for wi, w := range l.words {
			if wi > 0 {
				n := int(math.Round((w.x0 - l.words[wi-1].x1) / cw))
				if n < 1 && spaceBefore(l.words[wi-1], w) {
					n = 1
				}
				sb.WriteString(strings.Repeat(" ", max(0, n)))
			}
			sb.WriteString(w.text)
		}
	}
	return sb.String()
}

// plainHeading recognises the headings of a document set in one
// monospaced font (a plain-text printout): a short line in capitals or
// with a section number, with blank lines above and below.
func (d *doc) plainHeading(items []flowItem, i int) bool {
	l := items[i].l
	if !d.bodyMono || l.role() != RoleAuto || len(l.words) > 8 || l.chars() > 60 {
		return false
	}
	text := l.text()
	switch lastRune(text) {
	case '.', ',', ';', '!', '?':
		return false
	}
	numbered := len(l.words) > 1 && numberingDepth(l.words[0].text) > 0
	if !numbered && (strings.ToUpper(text) != text || !hasLetter(text)) {
		return false
	}
	blank := 1.6 * d.pitchFor(l.size)
	if i > 0 && (items[i-1].l == nil || l.base-items[i-1].l.base < blank) {
		return false
	}
	return i+1 < len(items) && items[i+1].l != nil && items[i+1].l.base-l.base >= blank
}

// isHeadingLine reports whether a line looks like a heading on its own:
// larger than body text, or bold and short, or numbered and emphasised.
func (d *doc) isHeadingLine(l *line) bool {
	switch r := l.role(); {
	case r.headingLevel() > 0 || r == RoleTitle:
		return true
	case r == RoleParagraph && !d.taggedHeadings, r == RoleAuto:
	default:
		// The producer says what the line is, and it is not a heading.
		return false
	}
	t := l.text()
	if !hasLetter(t) || l.chars() > 160 || len(l.words) > 25 {
		return false
	}
	if kind, _ := captionLabel(t); kind != "" {
		return false
	}
	b, it, _ := l.styleFrac()
	numbered := len(l.words) > 1 && numberingDepth(l.words[0].text) > 0
	switch {
	case l.size >= d.body*1.15:
		return true
	case b >= 0.85 && len(l.words) <= 14 && l.size >= d.body*0.9:
		return !(strings.HasSuffix(t, ".") && len(l.words) > 6)
	case numbered && (b >= 0.5 || it >= 0.9) && len(l.words) <= 14:
		return true
	}
	return false
}

func (d *doc) collectHeading(items []flowItem, i int, f *flow) (*proto, int, bool) {
	first := items[i].l
	p := &proto{kind: pHeading, flow: f, lines: []*line{first}}
	b, _, _ := first.styleFrac()
	p.hstyle = headingStyle{size: roundTo(first.size, 0.5), bold: b >= 0.5}
	if lvl := first.role().headingLevel(); lvl > 0 && first.block() != 0 {
		// A tagged heading: its lines share the block.
		j := i + 1
		for j < len(items) && items[j].l != nil && items[j].l.block() == first.block() {
			p.lines = append(p.lines, items[j].l)
			j++
		}
		p.level = lvl
		if len(first.words) > 1 {
			p.numDepth = numberingDepth(first.words[0].text)
		}
		return p, j, true
	}
	j := i + 1
	for j < len(items) {
		n := items[j].l
		if n == nil || !sameStyle(n, first) || !d.isHeadingLine(n) {
			break
		}
		prev := p.lines[len(p.lines)-1]
		if n.y0-prev.y1 > 0.7*n.size {
			break
		}
		p.lines = append(p.lines, n)
		j++
	}
	limit := 3
	if first.size < d.body*1.15 {
		limit = 2
	}
	words := 0
	for _, l := range p.lines {
		words += len(l.words)
	}
	if len(p.lines) > limit || words > 30 {
		return nil, i, false
	}
	// A bold line directly followed by body text on the next line with no
	// extra space is a bold run-in paragraph start rather than a heading.
	if first.size < d.body*1.15 && j < len(items) && items[j].l != nil {
		n, prev := items[j].l, p.lines[len(p.lines)-1]
		if n.base-prev.base <= d.pitchFor(prev.size)*1.15 && !endsSentence(prev.text()) && startsLower(n.text()) {
			return nil, i, false
		}
	}
	p.numDepth = numberingDepth(first.words[0].text)
	if len(first.words) < 2 {
		p.numDepth = 0
	}
	return p, j, true
}

// collectPara gathers the lines of one paragraph.
func (d *doc) collectPara(items []flowItem, i int, f *flow, tables map[int]tableRegion) (*proto, int) {
	l := items[i].l
	p := &proto{kind: pPara, flow: f, lines: []*line{l}, indented: d.indented(l, f)}
	j := i + 1
	for j < len(items) {
		if _, ok := tables[j]; ok {
			break
		}
		n := items[j].l
		if n == nil || d.isCode(n) || d.isHeadingLine(n) {
			break
		}
		if _, ok := d.lineMarker(n, f, p.lines[len(p.lines)-1]); ok {
			break
		}
		if kind, _ := captionLabel(n.text()); kind != "" {
			break
		}
		if d.paraBreak(p, n, f) {
			break
		}
		p.lines = append(p.lines, n)
		j++
	}
	d.markQuote(p, f)
	return p, j
}

func (d *doc) indented(l *line, f *flow) bool {
	ind := l.x0 - f.left
	return ind > 0.6*l.size && ind < 8*l.size
}

// short reports whether a line ends well before the right edge of its flow.
func short(l *line, f *flow) bool {
	width := f.right - f.left
	slack := max(2.5*l.size, 0.12*width)
	if f.ragged {
		slack = max(slack, 0.35*width)
	}
	return l.x1 < f.right-slack
}

func centered(l *line, f *flow) bool {
	lm, rm := l.x0-f.left, f.right-l.x1
	return lm > 2*l.size && rm > 2*l.size && math.Abs(lm-rm) < 1.5*l.size
}

// paraBreak decides whether line n starts a new paragraph after p.
func (d *doc) paraBreak(p *proto, n *line, f *flow) bool {
	prev := p.lines[len(p.lines)-1]
	// Sizes estimated from OCR boxes are noisy.
	sizeTol := 0.1
	if prev.ocr {
		sizeTol = 0.25
	}
	if math.Abs(n.size-prev.size) > sizeTol*max(n.size, prev.size) {
		return true
	}
	if pb, nb := prev.block(), n.block(); pb != 0 && nb != 0 {
		// The producer grouped the lines: trust it. OCR engines often
		// report each line of a skewed or curved page as a paragraph of its
		// own, so for OCR a split is only a hint and the geometry decides.
		if pb == nb || !prev.ocr {
			return pb != nb
		}
	}
	pitch := n.base - prev.base
	exp := d.pitchFor(prev.size)
	if pitch > exp*1.3+0.5 || pitch < 0.5*prev.size {
		return true
	}
	ind := n.x0 - f.left
	pind := prev.x0 - f.left
	if len(p.lines) == 1 && !p.indented && ind > 0.6*n.size && ind < 8*n.size && pind <= 0.3*n.size && !centered(n, f) {
		if d.indentDoc && (short(prev, f) || endsSentence(prev.text())) {
			return true
		}
		p.hanging = true
		return false
	}
	if p.hanging && ind <= 0.3*n.size {
		return true
	}
	if !p.hanging && ind > 0.6*n.size && ind < 8*n.size && pind <= 0.3*n.size && !centered(n, f) &&
		(d.indentDoc || short(prev, f) || endsSentence(prev.text())) {
		return true
	}
	if short(prev, f) && endsSentence(prev.text()) && !startsLower(n.text()) && !centered(prev, f) {
		return true
	}
	if centered(prev, f) != centered(n, f) && len(p.lines) == 1 && short(prev, f) {
		return true
	}
	// A switch between upright and italic text after a line that could end
	// a paragraph (a caption or quotation followed by body text).
	if styleFlip(prev, n) && (short(prev, f) || endsSentence(prev.text()) || pitch > exp*1.12) {
		return true
	}
	// A bold run-in label ("Keywords:", "Note.") after a line that ended
	// early starts a new paragraph.
	if (short(prev, f) || endsSentence(prev.text())) && runInLabel(n) || keywordLabelRe.MatchString(n.text()) {
		return true
	}
	return false
}

// styleFlip reports whether one line is set (almost) entirely in italics
// and the other not.
func styleFlip(a, b *line) bool {
	_, ia, _ := a.styleFrac()
	_, ib, _ := b.styleFrac()
	return ia >= 0.9 && ib <= 0.1 || ib >= 0.9 && ia <= 0.1
}

// runInLabel reports whether a line starts with a short bold label that
// ends with a colon or period, followed by regular text.
func runInLabel(l *line) bool {
	n := 0
	for i, w := range l.words {
		if !w.bold {
			return i > 0 && i <= 4 && n > 0
		}
		if r := lastRune(w.text); r == ':' || r == '.' {
			n++
		}
	}
	return false
}

// markQuote flags paragraphs indented as a block (every line starts at
// the same indented position) as block quotations when they are italic or
// indented on both sides, and a single indented italic line too.
func (d *doc) markQuote(p *proto, f *flow) {
	if r := p.lines[0].role(); r != RoleAuto {
		p.quote = r == RoleQuote
		return
	}
	minX, maxX := math.Inf(1), math.Inf(-1)
	sameStart := true
	for k, l := range p.lines {
		minX = min(minX, l.x0)
		if k > 0 && math.Abs(l.x0-p.lines[0].x0) > 1.5 {
			sameStart = false
		}
		if k < len(p.lines)-1 || len(p.lines) == 1 {
			maxX = max(maxX, l.x1)
		}
	}
	size := p.lines[0].size
	ql, qr := minX-f.left, f.right-maxX
	if ql < 0.9*size || ql > 0.3*(f.right-f.left) {
		return
	}
	_, it, _ := styleFracLines(p.lines)
	if len(p.lines) == 1 {
		p.quote = it >= 0.7 && !(d.indentDoc && ql < 2*size)
		return
	}
	if !sameStart {
		// First-line indents, or centred lines.
		return
	}
	p.quote = it >= 0.5 || qr >= 0.9*size && ql >= 1.5*size
	if p.quote {
		p.indented = false
	}
}

func styleFracLines(ls []*line) (b, i, m float64) {
	n := 0
	for _, l := range ls {
		lb, li, lm := l.styleFrac()
		c := float64(l.chars())
		b += lb * c
		i += li * c
		m += lm * c
		n += l.chars()
	}
	if n == 0 {
		return 0, 0, 0
	}
	return b / float64(n), i / float64(n), m / float64(n)
}

// captionNear reports whether l is a caption of the given kind right below
// (or above) y.
func (d *doc) captionNear(l *line, y float64, kind string) bool {
	k, _ := captionLabel(l.text())
	if l.role() == RoleCaption && (k == "" || k == kind) {
		k = kind
	}
	return k == kind && math.Abs(l.y0-y) < 3*max(d.body, l.size)
}

func (d *doc) captionAbove(p *proto, y float64, kind string) bool {
	if len(p.lines) == 0 || len(p.lines) > 4 {
		return false
	}
	k, _ := captionLabel(p.lines[0].text())
	if p.lines[0].role() == RoleCaption && (k == "" || k == kind) {
		k = kind
	}
	last := p.lines[len(p.lines)-1]
	return k == kind && y-last.y1 < 3*max(d.body, last.size) && y-last.y1 > -last.size
}

func (d *doc) collectCaption(items []flowItem, i int, f *flow) ([]*line, int) {
	lines := []*line{items[i].l}
	j := i + 1
	tmp := &proto{lines: lines}
	for j < len(items) && len(lines) < 8 {
		n := items[j].l
		prev := lines[len(lines)-1]
		if n == nil || d.isHeadingLine(n) || d.paraBreak(tmp, n, f) || short(prev, f) && (prev.block() == 0 || prev.ocr && prev.block() != n.block()) || styleFlip(prev, n) {
			break
		}
		if r := n.role(); r != RoleAuto && r != RoleCaption && prev.role() == RoleCaption {
			break
		}
		lines = append(lines, n)
		tmp.lines = lines
		j++
	}
	return lines, j
}

// captionInlines returns caption text without its "Figure 1:" label.
func (d *doc) captionInlines(lines []*line) []ast.Inline {
	if len(lines) == 0 {
		return nil
	}
	first := lines[0]
	_, n := captionLabel(first.text())
	rest := stripPrefix(first, n)
	ls := append([]*line{}, lines[1:]...)
	if rest != nil {
		ls = append([]*line{rest}, ls...)
	}
	_, it, _ := styleFracLines(ls)
	return d.inlines(ls, inlineOpts{noItalic: it > 0.9})
}

// stripPrefix returns the line without its first n bytes of text, or nil
// when nothing remains.
func stripPrefix(l *line, n int) *line {
	pos := 0
	for i, w := range l.words {
		if i > 0 && spaceBefore(l.words[i-1], w) {
			pos++
		}
		end := pos + len(w.text)
		if end <= n {
			pos = end
			continue
		}
		var ws []*word
		if pos < n {
			cut := *w
			cut.text = w.text[n-pos:]
			ws = append(ws, &cut)
		} else {
			ws = append(ws, w)
		}
		ws = append(ws, l.words[i+1:]...)
		return l.subLine(ws)
	}
	return nil
}

// assemble classifies all flows, merges paragraphs continuing across
// columns and pages and converts everything to blocks.
func (d *doc) assemble() []ast.Block {
	var ps []*proto
	for _, f := range d.flows {
		if d.ctx.Err() != nil {
			return nil
		}
		ps = append(ps, d.processFlow(f)...)
	}
	d.assignLevels(ps)
	d.matchNotes(ps)
	var out []ast.Block
	for i := 0; i < len(ps); {
		p := ps[i]
		if p.kind != pPara || !p.lastText {
			out = append(out, d.toBlocks(p)...)
			i++
			continue
		}
		var floats []*proto
		j := i + 1
		for j < len(ps) {
			k := j
			var fl []*proto
			for k < len(ps) && ps[k].isFloat() {
				fl = append(fl, ps[k])
				k++
			}
			if k >= len(ps) || ps[k].kind != pPara || !ps[k].firstText || !d.continues(p, ps[k]) {
				break
			}
			p.lines = append(p.lines, ps[k].lines...)
			p.lastText = ps[k].lastText
			floats = append(floats, fl...)
			j = k + 1
			if !p.lastText {
				break
			}
		}
		out = append(out, d.toBlocks(p)...)
		for _, fl := range floats {
			out = append(out, d.toBlocks(fl)...)
		}
		i = j
	}
	return out
}

// continues decides whether paragraph q (first in its column) continues
// paragraph p (last in the previous column).
func (d *doc) continues(p, q *proto) bool {
	last, first := p.lines[len(p.lines)-1], q.lines[0]
	if lb, fb := last.block(), first.block(); lb != 0 && fb != 0 && (lb == fb || !last.ocr) {
		return lb == fb
	}
	if p.quote != q.quote || math.Abs(last.size-first.size) > 0.12*max(last.size, first.size) {
		return false
	}
	lt, ft := last.text(), first.text()
	if q.indented && d.indentDoc && !startsLower(ft) {
		return false
	}
	if startsLower(ft) {
		return true
	}
	lf := last.fl
	if lf == nil {
		lf = p.flow
	}
	full := !short(last, lf)
	if endsSentence(lt) {
		return d.indentDoc && !q.indented && full
	}
	return full
}

func (d *doc) assignLevels(ps []*proto) {
	var hs []*proto
	styles := map[headingStyle]bool{}
	numbered := 0
	depthOf := map[headingStyle]map[int]int{}
	for _, p := range ps {
		if p.kind != pHeading {
			continue
		}
		hs = append(hs, p)
		styles[p.hstyle] = true
		if p.numDepth > 0 {
			numbered++
			if depthOf[p.hstyle] == nil {
				depthOf[p.hstyle] = map[int]int{}
			}
			depthOf[p.hstyle][p.numDepth]++
		}
	}
	if len(hs) == 0 {
		return
	}
	order := make([]headingStyle, 0, len(styles))
	for s := range styles {
		order = append(order, s)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].size != order[j].size {
			return order[i].size > order[j].size
		}
		return order[i].bold && !order[j].bold
	})
	rank := map[headingStyle]int{}
	for i, s := range order {
		rank[s] = min(i+1, 4)
	}
	// Levels given by the producer win; untagged headings in the same
	// style get the same level.
	tagged := map[headingStyle]map[int]int{}
	for _, p := range hs {
		if p.level > 0 {
			if tagged[p.hstyle] == nil {
				tagged[p.hstyle] = map[int]int{}
			}
			tagged[p.hstyle][p.level]++
		}
	}
	useNumbers := numbered >= 2 && numbered*2 >= len(hs)
	for _, p := range hs {
		lvl := rank[p.hstyle]
		switch {
		case p.level > 0:
			p.numDepth = min(p.level, 6)
			continue
		case tagged[p.hstyle] != nil:
			p.numDepth = mostCommon(tagged[p.hstyle])
			continue
		}
		if useNumbers {
			switch {
			case p.numDepth > 0:
				lvl = min(p.numDepth, 4)
			case depthOf[p.hstyle] != nil:
				lvl = mostCommon(depthOf[p.hstyle])
			}
		}
		p.numDepth = lvl // reused as the final level
	}
	if numbered >= 2 && numbered*10 >= len(hs)*6 {
		f := false
		d.meta.NumberSections = &f
	}
}

func mostCommon(m map[int]int) int {
	best, bestN := 0, -1
	for k, n := range m {
		if n > bestN || (n == bestN && k < best) {
			best, bestN = k, n
		}
	}
	return min(max(best, 1), 4)
}

// toBlocks converts a proto block to AST blocks.
func (d *doc) toBlocks(p *proto) []ast.Block {
	switch p.kind {
	case pHeading:
		_, it, _ := styleFracLines(p.lines)
		ins := d.inlines(p.lines, inlineOpts{noBold: true, noItalic: it > 0.9})
		if len(ins) == 0 {
			return nil
		}
		return []ast.Block{&ast.Heading{Level: max(p.numDepth, 1), Inlines: ins}}
	case pCode:
		return []ast.Block{&ast.CodeBlock{Text: d.codeText(p.lines)}}
	case pList:
		return []ast.Block{d.listBlock(p.list)}
	case pTable:
		if len(p.caption) > 0 {
			p.table.Caption = d.captionInlines(p.caption)
		}
		return []ast.Block{p.table}
	case pFigure:
		return []ast.Block{d.figure(p)}
	case pMath:
		if b := d.mathBlock(p); b != nil {
			return []ast.Block{b}
		}
		return nil
	}
	ins := d.inlines(p.lines, inlineOpts{flow: p.flow})
	if len(ins) == 0 {
		return nil
	}
	var b ast.Block = &ast.Para{Inlines: ins}
	if p.quote {
		_, it, _ := styleFracLines(p.lines)
		if it > 0.9 {
			ins = d.inlines(p.lines, inlineOpts{noItalic: true, flow: p.flow})
		}
		b = &ast.BlockQuote{Blocks: []ast.Block{&ast.Para{Inlines: ins}}}
	}
	return []ast.Block{b}
}

func (d *doc) figure(p *proto) ast.Block {
	im := p.img
	d.imgCount++
	ext := ".png"
	switch im.mediaType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	}
	src := d.res.Add(fmt.Sprintf("page%d-image%d%s", im.page+1, d.imgCount, ext), im.mediaType, im.data)
	img := &ast.Image{
		Src:    src,
		Width:  fmt.Sprintf("%.0fpt", im.x1-im.x0),
		Height: fmt.Sprintf("%.0fpt", im.y1-im.y0),
	}
	fig := &ast.Figure{Image: img}
	if len(p.caption) > 0 {
		fig.Caption = d.captionInlines(p.caption)
		img.Alt = plainText(fig.Caption)
	}
	return fig
}
