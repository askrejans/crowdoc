package docx

import (
	"sort"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/mathconv/omml"
)

// blockCtx tells converters where content ends up: headings, figures and
// page breaks only make sense in the main flow, not in cells or notes.
type blockCtx struct {
	top    bool
	inCell bool
	inNote bool
}

type itemKind int

const (
	kPara itemKind = iota
	kHeading
	kTitle
	kSubtitle
	kBlock
)

// item is a converted paragraph or block before list, code, quote and
// caption grouping.
type item struct {
	kind      itemKind
	block     ast.Block
	ins       []ast.Inline
	level     int
	p         *paraProps
	code      bool
	codeText  string
	task      ast.TaskState
	bookmarks []string

	// cont marks the later parts of a paragraph split by a block-level
	// marker (display math, page break); in a list they stay in the item.
	cont bool

	capKind  capKind
	figure   *ast.Figure
	table    *ast.Table
	chart    bool
	consumed bool

	// Inputs of the heading heuristic for documents without heading styles.
	allBold  bool
	size     int
	plainLen int
}

// blockItems converts block-level elements.
func (r *reader) blockItems(s *story, kids []*node, bc blockCtx) []item {
	var out []item
	for _, n := range kids {
		if r.err != nil {
			return out
		}
		if err := r.ctx.Err(); err != nil {
			r.err = err
			return out
		}
		switch n.ns {
		case "w":
			switch n.name {
			case "p":
				out = append(out, r.dropCapParagraph(s, n, bc)...)
			case "tbl":
				if it, ok := r.table(s, n, bc); ok {
					out = append(out, it)
				}
			case "sdt":
				if content := r.sdtContent(n); content != nil {
					out = append(out, r.blockItems(s, content.kids, bc)...)
				}
			case "bookmarkStart":
				if name := n.attr("name"); name != "" {
					s.bookmarks = append(s.bookmarks, name)
				}
			case "altChunk":
				r.warn.Addf("embedded alternative-format content (altChunk) was dropped")
			case "del", "moveFrom", "sectPr":
			default:
				if !strings.HasSuffix(n.name, "Pr") {
					out = append(out, r.blockItems(s, n.kids, bc)...)
				}
			}
		case "m":
			if n.name == "oMathPara" || n.name == "oMath" {
				if tex, _ := omml.ToLaTeX(toOMML(n, 0)); strings.TrimSpace(tex) != "" {
					out = append(out, item{kind: kBlock, block: &ast.MathBlock{TeX: tex}})
				}
			}
		case "mc":
			if n.name == "AlternateContent" {
				if alt := chooseAlternate(n); alt != nil {
					out = append(out, r.blockItems(s, alt.kids, bc)...)
				}
			}
		}
	}
	return out
}

// dropCapParagraph converts a paragraph, joining a drop-cap initial kept
// in its own framed paragraph to the text that follows it.
func (r *reader) dropCapParagraph(s *story, p *node, bc blockCtx) []item {
	items := r.paragraph(s, p, bc)
	if len(items) == 1 && items[0].kind == kPara && !items[0].code && items[0].p.dropCap {
		s.dropCap = append(s.dropCap, items[0].ins...)
		return nil
	}
	if len(s.dropCap) > 0 {
		for k := range items {
			if items[k].kind != kBlock && !items[k].code {
				items[k].ins = ast.MergeText(append(s.dropCap, items[k].ins...))
				s.dropCap = nil
				break
			}
		}
	}
	return items
}

// assemble groups items into the final blocks of one container.
func (r *reader) assemble(items []item, bc blockCtx) []ast.Block {
	r.attachCaptions(items, bc)
	var out []ast.Block
	for i := 0; i < len(items); {
		if r.err != nil {
			return out
		}
		it := &items[i]
		if bc.top && len(r.titles)+len(r.subtitles) > 0 && closesTitle(it) {
			r.titleClosed = true
		}
		switch {
		case it.consumed:
			i++
		case it.kind == kPara && it.code:
			var cb *ast.CodeBlock
			cb, i = codeBlock(items, i)
			if cb != nil {
				out = append(out, cb)
			}
		case it.kind == kPara && it.p != nil && it.p.list != nil:
			var l *ast.List
			l, i = r.list(items, i)
			out = append(out, l)
		case it.kind == kPara && it.p != nil && it.p.kind == kindQuote:
			q := &ast.BlockQuote{}
			for i < len(items) && items[i].kind == kPara && !items[i].code && items[i].p != nil &&
				items[i].p.kind == kindQuote && items[i].p.list == nil && !items[i].consumed {
				q.Blocks = append(q.Blocks, r.paraBlock(&items[i], bc))
				i++
			}
			out = append(out, q)
		case it.kind == kTitle && r.titleClosed:
			h := &ast.Heading{Level: 1, Inlines: it.ins}
			h.Attr.ID = r.anchorID(it.bookmarks, true)
			out = append(out, h)
			i++
		case it.kind == kSubtitle && r.titleClosed:
			out = append(out, r.paraBlock(it, bc))
			i++
		case it.kind == kTitle:
			r.titles = append(r.titles, ast.PlainText(it.ins))
			i++
		case it.kind == kSubtitle:
			r.subtitles = append(r.subtitles, ast.PlainText(it.ins))
			i++
		case it.kind == kHeading:
			h := &ast.Heading{Level: it.level, Inlines: it.ins}
			h.Attr.ID = r.anchorID(it.bookmarks, true)
			out = append(out, h)
			i++
		case it.kind == kPara:
			out = append(out, r.paraBlock(it, bc))
			i++
		default:
			if it.figure != nil && it.figure.Attr.ID == "" {
				it.figure.Attr.ID = r.anchorID(it.bookmarks, false)
			}
			if it.block != nil {
				out = append(out, it.block)
			}
			i++
		}
	}
	if !bc.top {
		out = dropPageBreaks(out)
	}
	return out
}

// closesTitle reports whether it is body content ending the run of title
// paragraphs at the top of a document.
func closesTitle(it *item) bool {
	if it.consumed || it.kind == kTitle || it.kind == kSubtitle {
		return false
	}
	_, pageBreak := it.block.(*ast.PageBreak)
	return !pageBreak
}

// paraBlock converts a paragraph item, anchoring referenced bookmarks.
func (r *reader) paraBlock(it *item, bc blockCtx) ast.Block {
	ins := it.ins
	if id := r.anchorID(it.bookmarks, false); id != "" {
		ins = append([]ast.Inline{&ast.Span{Attr: ast.Attr{ID: id}}}, ins...)
	}
	if bc.inCell {
		return &ast.Plain{Inlines: ins}
	}
	return &ast.Para{Inlines: ins}
}

func codeBlock(items []item, i int) (*ast.CodeBlock, int) {
	var lines []string
	for i < len(items) && items[i].kind == kPara && items[i].code && !items[i].consumed {
		lines = append(lines, items[i].codeText)
		i++
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil, i
	}
	return &ast.CodeBlock{Text: strings.Join(lines, "\n")}, i
}

// openList is a list being built at one nesting level.
type openList struct {
	list    *ast.List
	level   int
	numID   string
	ordered bool
	style   ast.NumberStyle
}

// list builds a (nested) list from consecutive numbered paragraphs.
// Indented unnumbered paragraphs continue the preceding item.
func (r *reader) list(items []item, i int) (*ast.List, int) {
	var stack []*openList
	var all []*ast.List
	newList := func(lr *listRef) *openList {
		l := &ast.List{Ordered: lr.ordered, Style: lr.style, Tight: true}
		if lr.ordered {
			l.Start = lr.number
		}
		all = append(all, l)
		return &openList{list: l, level: lr.ilvl, numID: lr.numID, ordered: lr.ordered, style: lr.style}
	}
	lastItem := func() *ast.ListItem {
		top := stack[len(stack)-1].list
		return &top.Items[len(top.Items)-1]
	}
	attach := func(ol *openList) {
		parent := lastItem()
		parent.Blocks = append(parent.Blocks, ol.list)
		stack = append(stack, ol)
	}

	j := i
loop:
	for j < len(items) {
		it := &items[j]
		if it.consumed {
			j++
			continue
		}
		if it.cont && len(stack) > 0 {
			li := lastItem()
			switch {
			case it.kind == kBlock:
				if it.block != nil {
					li.Blocks = append(li.Blocks, it.block)
				}
			case it.code:
				li.Blocks = append(li.Blocks, &ast.CodeBlock{Text: it.codeText})
			default:
				li.Blocks = append(li.Blocks, &ast.Para{Inlines: it.ins})
			}
			j++
			continue
		}
		if it.kind == kPara && !it.code && it.p != nil && it.p.list != nil {
			lr := it.p.list
			if len(stack) == 0 {
				stack = append(stack, newList(lr))
			} else {
				root := stack[0]
				if lr.ilvl <= root.level && (lr.numID != root.numID || lr.ordered != root.ordered || lr.style != root.style) {
					break loop
				}
				if lr.ilvl < root.level {
					break loop
				}
				for len(stack) > 1 && stack[len(stack)-1].level > lr.ilvl {
					stack = stack[:len(stack)-1]
				}
				top := stack[len(stack)-1]
				switch {
				case lr.ilvl > top.level:
					attach(newList(lr))
				case top.ordered != lr.ordered || top.style != lr.style:
					if len(stack) == 1 {
						break loop
					}
					stack = stack[:len(stack)-1]
					attach(newList(lr))
				}
			}
			top := stack[len(stack)-1].list
			ins := it.ins
			if id := r.anchorID(it.bookmarks, false); id != "" {
				ins = append([]ast.Inline{&ast.Span{Attr: ast.Attr{ID: id}}}, ins...)
			}
			task := it.task
			if task == ast.TaskNone {
				task = lr.task
			}
			top.Items = append(top.Items, ast.ListItem{Blocks: []ast.Block{&ast.Para{Inlines: ins}}, Task: task})
			j++
			continue
		}
		if len(stack) > 0 && it.kind == kPara && it.p != nil && it.p.indent > 0 && it.p.kind == kindNone {
			li := lastItem()
			if it.code {
				if cb, next := codeBlock(items, j); cb != nil {
					li.Blocks = append(li.Blocks, cb)
					j = next
					continue
				}
			}
			li.Blocks = append(li.Blocks, r.paraBlock(it, blockCtx{}))
			j++
			continue
		}
		break
	}
	for _, l := range all {
		finishList(l)
	}
	if len(stack) == 0 {
		return &ast.List{}, j + 1
	}
	return stack[0].list, j
}

// finishList decides tightness: items holding a single paragraph (plus
// nested lists) become Plain.
func finishList(l *ast.List) {
	for _, it := range l.Items {
		paras := 0
		for _, b := range it.Blocks {
			if _, ok := b.(*ast.List); !ok {
				paras++
			}
		}
		if paras > 1 {
			l.Tight = false
			return
		}
	}
	for _, it := range l.Items {
		for k, b := range it.Blocks {
			if p, ok := b.(*ast.Para); ok {
				it.Blocks[k] = &ast.Plain{Inlines: p.Inlines}
			}
		}
	}
}

// attachCaptions joins caption paragraphs to the adjacent table or figure.
func (r *reader) attachCaptions(items []item, bc blockCtx) {
	if bc.inCell {
		return
	}
	candidate := func(j int, want capKind, allowOther bool) bool {
		if j < 0 || j >= len(items) {
			return false
		}
		c := &items[j]
		if c.consumed || c.kind != kPara || c.code || c.p == nil || c.p.list != nil || c.capKind == capNone {
			return false
		}
		k := c.capKind.base()
		return k == want || (allowOther && k == capOther)
	}
	take := func(j int) ([]ast.Inline, string) {
		c := &items[j]
		c.consumed = true
		return stripCaption(c.ins), r.anchorID(c.bookmarks, false)
	}
	for i := range items {
		it := &items[i]
		switch {
		case it.table != nil && it.table.Caption == nil:
			var j int
			switch {
			case candidate(i-1, capTable, true):
				j = i - 1
			case candidate(i+1, capTable, false):
				j = i + 1
			case it.chart && candidate(i+1, capFigure, true):
				j = i + 1
			default:
				continue
			}
			it.table.Caption, it.table.Attr.ID = take(j)
			if len(it.table.Caption) == 0 {
				it.table.Caption = nil
			}
		case it.figure != nil && it.figure.Caption == nil:
			var j int
			switch {
			case candidate(i+1, capFigure, true):
				j = i + 1
			case candidate(i-1, capFigure, false):
				j = i - 1
			default:
				continue
			}
			it.figure.Caption, it.figure.Attr.ID = take(j)
			if len(it.figure.Caption) == 0 {
				it.figure.Caption = nil
			}
		}
	}
}

func dropPageBreaks(blocks []ast.Block) []ast.Block {
	out := blocks[:0]
	for _, b := range blocks {
		if _, ok := b.(*ast.PageBreak); !ok {
			out = append(out, b)
		}
	}
	return out
}

// promoteFakeHeadings turns short, bold or enlarged one-line paragraphs
// into headings when the document uses no heading styles at all, as some
// producers export headings as direct formatting only.
func (r *reader) promoteFakeHeadings(items []item) {
	var sizes map[int]int
	body := map[int]int{}
	for _, it := range items {
		switch it.kind {
		case kHeading:
			return
		case kPara:
			if it.size > 0 {
				body[it.size] += it.plainLen
			}
		}
	}
	bodySize, best := 0, -1
	for sz, n := range body {
		if n > best || (n == best && sz < bodySize) {
			bodySize, best = sz, n
		}
	}
	if bodySize == 0 {
		return
	}
	// candidate reports a short single-line paragraph that is set larger
	// than body text, or entirely bold.
	candidate := func(it *item) (ok, bigger bool) {
		if it.kind != kPara || it.code || it.p == nil || it.p.list != nil || it.p.kind != kindNone ||
			it.plainLen == 0 || it.plainLen > 120 || it.consumed {
			return false, false
		}
		for _, in := range it.ins {
			switch in.(type) {
			case *ast.LineBreak, *ast.Image, *ast.Math, *ast.Note, *ast.Cite:
				return false, false
			}
		}
		text := strings.TrimSpace(ast.PlainText(it.ins))
		if text == "" || strings.ContainsAny(text[len(text)-1:], ".,;") {
			return false, false
		}
		bigger = it.size*10 >= bodySize*12
		return bigger || (it.allBold && it.size >= bodySize), bigger
	}
	sizes = map[int]int{}
	var cands []int
	anyBigger := false
	for i := range items {
		if ok, bigger := candidate(&items[i]); ok {
			cands = append(cands, i)
			sizes[items[i].size]++
			anyBigger = anyBigger || bigger
		}
	}
	// Bold lines alone are weak evidence; enlarged text shows the document
	// really formats headings by hand. Too many candidates means the
	// formatting is simply this document's body style.
	if !anyBigger || len(cands)*2 > len(items) {
		return
	}
	ranked := make([]int, 0, len(sizes))
	for sz := range sizes {
		ranked = append(ranked, sz)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ranked)))
	for _, i := range cands {
		level := 1
		for k, sz := range ranked {
			if sz == items[i].size {
				level = k + 1
			}
		}
		if level > 4 {
			level = 4
		}
		it := &items[i]
		it.kind, it.level = kHeading, level
		it.ins = unwrapStrong(it.ins)
	}
}

// finishBlocks applies document-level clean-ups to the top-level blocks.
func (r *reader) finishBlocks(blocks []ast.Block) []ast.Block {
	out := make([]ast.Block, 0, len(blocks))
	for _, b := range blocks {
		switch b.(type) {
		case *ast.PageBreak:
			if len(out) == 0 {
				continue
			}
			if _, prev := out[len(out)-1].(*ast.PageBreak); prev {
				continue
			}
		case *ast.Bibliography:
			// The heading right above a generated bibliography names the
			// reference list; the writer prints it with the list.
			if n := len(out); n > 0 {
				if h, ok := out[n-1].(*ast.Heading); ok {
					if r.doc.Meta.ReferenceSectionTitle == "" {
						r.doc.Meta.ReferenceSectionTitle = ast.PlainText(h.Inlines)
					}
					out = out[:n-1]
				}
			}
		}
		out = append(out, b)
	}
	for len(out) > 0 {
		if _, ok := out[len(out)-1].(*ast.PageBreak); !ok {
			break
		}
		out = out[:len(out)-1]
	}
	return out
}
