package rtf

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// flow assembles finished paragraphs into blocks: consecutive code
// paragraphs become one code block, list paragraphs nest into lists, quote
// paragraphs merge into one block quote and captions attach to figures and
// tables.
type flow struct {
	p       *parser
	body    bool
	entries []entry

	code      *ast.CodeBlock
	codeText  strings.Builder
	codeBlank int
	quote     *ast.BlockQuote
	center    *ast.Div
	lists     []*openList

	// cell statistics (alignment and boldness of the content)
	align      ast.Align
	alignSet   bool
	alignMixed bool
	textLen    int
	boldLen    int
}

type entry struct {
	block   ast.Block
	caption bool
	rec     *paraRec // size-based heading candidate
}

type openList struct {
	list   *ast.List
	level  int
	key    string
	indent int
	loose  bool
}

func newFlow(p *parser, body bool) *flow { return &flow{p: p, body: body} }

func (f *flow) push(e entry) { f.entries = append(f.entries, e) }

func (f *flow) add(r *paraRec) {
	if r.kind != rEmpty {
		f.textLen += r.textLen
		f.boldLen += r.boldLen
		if !f.alignSet {
			f.align, f.alignSet = r.align, true
		} else if f.align != r.align {
			f.alignMixed = true
		}
	}
	if r.pageBreakBefore && f.body && r.kind != rEmpty {
		f.addBlock(&ast.PageBreak{})
	}
	switch r.kind {
	case rEmpty:
		if r.codeBlank && f.code != nil {
			f.codeBlank++
		}
		return
	case rCode:
		f.closeLists()
		f.closeQuote()
		f.closeCenter()
		if f.code == nil {
			f.code = &ast.CodeBlock{}
			f.push(entry{block: f.code})
			f.codeText.Reset()
		} else {
			for range f.codeBlank + 1 {
				f.codeText.WriteByte('\n')
			}
		}
		f.codeText.WriteString(r.code)
		f.codeBlank = 0
		return
	}
	f.closeCode()
	if r.list != nil {
		f.closeQuote()
		f.closeCenter()
		f.addItem(r)
		return
	}
	if len(f.lists) > 0 {
		if r.kind == rNormal && f.continuation(r) {
			return
		}
		f.closeLists()
	}
	switch r.kind {
	case rQuote:
		f.closeCenter()
		if f.quote == nil {
			f.quote = &ast.BlockQuote{}
			f.push(entry{block: f.quote})
		}
		f.quote.Blocks = append(f.quote.Blocks, &ast.Para{Inlines: r.inlines})
		return
	case rNormal:
		f.closeQuote()
		if r.align == ast.AlignCenter && f.body {
			if f.center == nil {
				f.center = &ast.Div{Attr: ast.Attr{Classes: []string{"center"}}}
				f.push(entry{block: f.center})
			}
			f.center.Blocks = append(f.center.Blocks, &ast.Para{Inlines: r.inlines})
			return
		}
		f.closeCenter()
		e := entry{block: &ast.Para{Inlines: r.inlines}}
		if r.candidate {
			e.rec = r
		}
		f.push(e)
		return
	}
	f.closeQuote()
	f.closeCenter()
	switch r.kind {
	case rHeading:
		f.push(entry{block: &ast.Heading{Level: r.level, Inlines: r.inlines, Attr: ast.Attr{ID: r.id}}})
	case rTitle, rSubtitle:
		f.setTitle(r)
	case rCaption:
		f.push(entry{block: &ast.Para{Inlines: r.inlines}, caption: true})
	case rFigure:
		f.push(entry{block: &ast.Figure{Image: r.image}})
	}
}

func (f *flow) setTitle(r *paraRec) {
	text := ast.PlainText(r.inlines)
	if text == "" {
		return
	}
	m := &f.p.doc.Meta
	if r.kind == rSubtitle {
		m.Subtitle = joinSpace(m.Subtitle, text)
		return
	}
	if f.p.titleFromBody {
		m.Title = joinSpace(m.Title, text)
	} else {
		m.Title = text
		f.p.titleFromBody = true
	}
}

func joinSpace(a, b string) string {
	if a == "" {
		return b
	}
	return a + " " + b
}

// addBlock appends a ready block (table, page break, text box content).
func (f *flow) addBlock(b ast.Block) {
	if _, ok := b.(*ast.PageBreak); ok && !f.body {
		return
	}
	f.closeAll()
	f.push(entry{block: b})
}

func (f *flow) closeAll() {
	f.closeCode()
	f.closeLists()
	f.closeQuote()
	f.closeCenter()
}

func (f *flow) closeCode() {
	if f.code != nil {
		f.code.Text = f.codeText.String()
	}
	f.code, f.codeBlank = nil, 0
}
func (f *flow) closeQuote()  { f.quote = nil }
func (f *flow) closeCenter() { f.center = nil }

func (f *flow) top() *openList { return f.lists[len(f.lists)-1] }

func (f *flow) addItem(r *paraRec) {
	l := r.list
	if l.key == "txt" {
		// Typed bullets nest by indentation only.
		for len(f.lists) > 0 && f.top().key == "txt" && f.top().indent > r.indent {
			f.popList()
		}
		if len(f.lists) > 0 && f.top().key == "txt" && f.top().indent < r.indent {
			l.level = f.top().level + 1
		} else if len(f.lists) > 0 {
			l.level = f.top().level
		}
	}
	for len(f.lists) > 0 && f.top().level > l.level {
		f.popList()
	}
	if len(f.lists) > 0 {
		if t := f.top(); t.level == l.level && (t.key != l.key || t.list.Ordered != l.ordered) {
			f.popList()
		}
	}
	if len(f.lists) == 0 || f.top().level < l.level {
		nl := &ast.List{Ordered: l.ordered, Tight: true}
		if l.ordered {
			nl.Style, nl.Start = l.style, l.start
		}
		if len(f.lists) == 0 {
			f.push(entry{block: nl})
		} else {
			parent := f.top().list
			if len(parent.Items) == 0 {
				parent.Items = append(parent.Items, ast.ListItem{})
			}
			it := &parent.Items[len(parent.Items)-1]
			it.Blocks = append(it.Blocks, nl)
		}
		f.lists = append(f.lists, &openList{list: nl, level: l.level, key: l.key})
	}
	t := f.top()
	t.indent = r.indent
	t.list.Items = append(t.list.Items, ast.ListItem{Blocks: []ast.Block{&ast.Plain{Inlines: r.inlines}}})
}

// continuation attaches an indented non-list paragraph to the open list
// item it lines up with.
func (f *flow) continuation(r *paraRec) bool {
	if r.indent <= 0 || f.lists[0].indent <= 0 || r.indent < f.lists[0].indent {
		return false
	}
	for len(f.lists) > 1 && f.top().indent > r.indent {
		f.popList()
	}
	t := f.top()
	if len(t.list.Items) == 0 {
		return false
	}
	it := &t.list.Items[len(t.list.Items)-1]
	it.Blocks = append(it.Blocks, &ast.Para{Inlines: r.inlines})
	t.loose = true
	return true
}

func (f *flow) popList() {
	t := f.top()
	f.lists = f.lists[:len(f.lists)-1]
	if !t.loose {
		return
	}
	t.list.Tight = false
	for i := range t.list.Items {
		for j, b := range t.list.Items[i].Blocks {
			if pl, ok := b.(*ast.Plain); ok {
				t.list.Items[i].Blocks[j] = &ast.Para{Inlines: pl.Inlines}
			}
		}
	}
}

func (f *flow) closeLists() {
	for len(f.lists) > 0 {
		f.popList()
	}
}

func (f *flow) finish() []ast.Block {
	f.closeAll()
	f.attachCaptions()
	if f.body {
		f.sizeHeadings()
	}
	out := make([]ast.Block, 0, len(f.entries))
	for _, e := range f.entries {
		if e.block == nil {
			continue
		}
		if _, ok := e.block.(*ast.PageBreak); ok {
			if len(out) == 0 {
				continue
			}
			if _, prev := out[len(out)-1].(*ast.PageBreak); prev {
				continue
			}
		}
		out = append(out, e.block)
	}
	for len(out) > 0 {
		if _, ok := out[len(out)-1].(*ast.PageBreak); !ok {
			break
		}
		out = out[:len(out)-1]
	}
	return out
}

var (
	tableWords  = []string{"table", "tabula", "tab.", "tabelle", "tabla", "tableau", "tabel", "tabell", "taulukko", "lentelė", "таблица", "табл."}
	figureWords = []string{"figure", "fig.", "image", "illustration", "attēls", "att.", "abbildung", "abb.", "figura", "figur", "kuva", "paveikslas", "рисунок", "рис."}
)

// attachCaptions moves caption-styled paragraphs onto the adjacent figure
// or table.
func (f *flow) attachCaptions() {
	for i := range f.entries {
		e := &f.entries[i]
		if !e.caption || e.block == nil {
			continue
		}
		para := e.block.(*ast.Para)
		text := ast.PlainText(para.Inlines)
		isTable := labelLen(text, tableWords) > 0
		isFigure := labelLen(text, figureWords) > 0
		order := []int{i - 1, i + 1}
		if isTable {
			order = []int{i + 1, i - 1}
		}
		for _, j := range order {
			if j < 0 || j >= len(f.entries) || f.entries[j].block == nil {
				continue
			}
			switch n := f.entries[j].block.(type) {
			case *ast.Table:
				if n.Caption == nil && !isFigure {
					n.Caption = stripLabel(para.Inlines, tableWords)
					e.block = nil
				}
			case *ast.Figure:
				if n.Caption == nil && !isTable {
					n.Caption = stripLabel(para.Inlines, figureWords)
					e.block = nil
				}
			}
			if e.block == nil {
				break
			}
		}
	}
}

// labelLen returns the byte length of a leading caption label such as
// "Figure 3:", "Tab. 2.1 –" or "Table:" in s, or 0 when s has none.
func labelLen(s string, words []string) int {
	i := len(s) - len(strings.TrimLeft(s, " \t"))
	for _, w := range words {
		rest := s[i:]
		n := 0
		for range utf8.RuneCountInString(w) {
			_, size := utf8.DecodeRuneInString(rest[n:])
			if size == 0 {
				n = -1
				break
			}
			n += size
		}
		if n <= 0 || !strings.EqualFold(rest[:n], w) {
			continue
		}
		j := i + n
		if j < len(s) && !strings.HasSuffix(w, ".") {
			if r, _ := utf8.DecodeRuneInString(s[j:]); !(r == ' ' || r == ':' || r == '.' || r >= '0' && r <= '9') {
				continue
			}
		}
		j += len(s[j:]) - len(strings.TrimLeft(s[j:], " "))
		numbered := false
		k := j
		for k < len(s) && (isDigit(s[k]) || isLetter(s[k]) || s[k] == '.' || s[k] == '-') {
			k++
		}
		if tok := strings.TrimRight(s[j:k], ".-"); tok != "" && (isNumberToken(tok) || romanValue(tok) > 0) {
			numbered = true
			j = k
		}
		j += len(s[j:]) - len(strings.TrimLeft(s[j:], " "))
		sep := false
		if r, size := utf8.DecodeRuneInString(s[j:]); r == ':' || r == '.' || r == '-' || r == '–' || r == '—' {
			sep = true
			j += size
		}
		if !numbered && !sep {
			continue
		}
		return j + len(s[j:]) - len(strings.TrimLeft(s[j:], " "))
	}
	return 0
}

func isNumberToken(s string) bool {
	digit := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case isDigit(c):
			digit = true
		case c == '.' || c == '-':
		default:
			return false
		}
	}
	return digit
}

// stripLabel removes a leading "Figure 3:" style label (numbering is
// generated by the writer).
func stripLabel(ins []ast.Inline, words []string) []ast.Inline {
	ins = slices.Clone(ins)
	// Labels are often split over a few runs ("Figure ", "3", ": text").
	var prefix strings.Builder
	n := 0
	for n < len(ins) {
		t, ok := ins[n].(*ast.Text)
		if !ok {
			break
		}
		prefix.WriteString(t.Value)
		n++
	}
	if n == 0 {
		// A bold label: drop the whole wrapper when it is only the label.
		if c := ast.InlineChildren(ins[0]); c != nil {
			label := ast.PlainText(c)
			if l := labelLen(label+" ", words); l > 0 && l >= len(strings.TrimSpace(label)) {
				return ast.TrimInlines(stripLabelRest(ins[1:]))
			}
		}
		return ins
	}
	l := labelLen(prefix.String(), words)
	if l == 0 {
		return ins
	}
	out := []ast.Inline{}
	if rest := prefix.String()[l:]; rest != "" {
		out = append(out, &ast.Text{Value: rest})
	}
	out = append(out, ins[n:]...)
	return ast.TrimInlines(out)
}

// stripLabelRest removes the separator left over after a label wrapper.
func stripLabelRest(ins []ast.Inline) []ast.Inline {
	if len(ins) > 0 {
		if t, ok := ins[0].(*ast.Text); ok {
			v := strings.TrimLeft(t.Value, " :.-–—")
			if v == "" {
				return ins[1:]
			}
			return append([]ast.Inline{&ast.Text{Value: v}}, ins[1:]...)
		}
	}
	return ins
}

// sizeHeadings promotes short, large-type paragraphs to headings in
// documents without heading styles (simple editors).
func (f *flow) sizeHeadings() {
	p := f.p
	if p.styleHeads {
		return
	}
	body, bodyN := 0, 0
	for size, n := range p.sizeChars {
		if n > bodyN || n == bodyN && size < body {
			body, bodyN = size, n
		}
	}
	if body == 0 {
		return
	}
	large := func(r *paraRec) bool { return r.minSize*5 >= body*6 }
	paras, cands := 0, 0
	sizes := map[int]bool{}
	for _, e := range f.entries {
		if _, ok := e.block.(*ast.Para); ok {
			paras++
		}
		if e.rec == nil {
			continue
		}
		if large(e.rec) {
			cands++
			sizes[e.rec.minSize] = true
		} else if e.rec.boldHead && e.rec.minSize >= body {
			cands++
		}
	}
	if cands == 0 || cands*2 > paras {
		return
	}
	order := make([]int, 0, len(sizes))
	for s := range sizes {
		order = append(order, s)
	}
	slices.SortFunc(order, func(a, b int) int { return b - a })
	for i := range f.entries {
		e := &f.entries[i]
		if e.rec == nil {
			continue
		}
		level := len(order) + 1 // bold body-size lines rank below sized ones
		switch {
		case large(e.rec):
			level = slices.Index(order, e.rec.minSize) + 1
		case !e.rec.boldHead || e.rec.minSize < body:
			continue
		}
		items, id := extractAnchors(e.rec.items)
		e.block = &ast.Heading{Level: min(level, 6), Inlines: p.inlines(items, fBold|uniformFlags(items)), Attr: ast.Attr{ID: id}}
	}
}
