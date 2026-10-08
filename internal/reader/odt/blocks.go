package odt

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// blockBuilder accumulates blocks, merging consecutive quotation
// paragraphs into one block quote and code paragraphs into one code block.
type blockBuilder struct {
	out      []ast.Block
	quote    *ast.BlockQuote
	code     []string
	codeLang string
	blank    int // pending empty code-styled lines
}

func (b *blockBuilder) flushCode() {
	if len(b.code) > 0 {
		b.out = append(b.out, &ast.CodeBlock{Lang: b.codeLang, Text: strings.Join(b.code, "\n")})
	}
	b.code, b.blank = nil, 0
}

func (b *blockBuilder) flush() {
	b.flushCode()
	b.quote = nil
}

func (b *blockBuilder) add(blocks ...ast.Block) {
	if len(blocks) == 0 {
		return
	}
	b.flush()
	b.out = append(b.out, blocks...)
}

func (b *blockBuilder) addQuote(blocks ...ast.Block) {
	b.flushCode()
	if b.quote == nil {
		b.quote = &ast.BlockQuote{}
		b.out = append(b.out, b.quote)
	}
	b.quote.Blocks = append(b.quote.Blocks, blocks...)
}

func (b *blockBuilder) addCode(line string) {
	b.quote = nil
	for ; b.blank > 0; b.blank-- {
		b.code = append(b.code, "")
	}
	b.code = append(b.code, line)
}

func (b *blockBuilder) pageBreak() {
	b.flush()
	if len(b.out) == 0 {
		return
	}
	if _, ok := b.out[len(b.out)-1].(*ast.PageBreak); ok {
		return
	}
	b.out = append(b.out, &ast.PageBreak{})
}

var droppedBlocks = map[string]bool{
	"text:table-of-content": true, "text:illustration-index": true, "text:table-index": true,
	"text:object-index": true, "text:user-index": true, "text:alphabetical-index": true,
	"text:tracked-changes": true, "text:sequence-decls": true, "text:variable-decls": true,
	"text:user-field-decls": true, "text:dde-connection-decls": true, "office:forms": true,
	"text:soft-page-break": true, "office:annotation": true, "office:annotation-end": true,
	"table:calculation-settings": true, "table:content-validations": true,
	"table:label-ranges": true, "text:alphabetical-index-auto-mark-file": true,
	"table:table-column": true, "table:table-columns": true, "table:table-column-group": true,
	"text:change": true, "text:change-start": true, "text:change-end": true,
	"text:bookmark-end": true, "office:scripts": true, "office:font-face-decls": true,
	"text:page-sequence": true,
}

// blocks converts the block-level children of parent.
func (r *reader) blocks(parent *node) []ast.Block {
	b := &blockBuilder{}
	r.blocksInto(b, parent, 0)
	b.flush()
	return r.attachCaptions(b.out)
}

func (r *reader) blocksInto(b *blockBuilder, parent *node, depth int) {
	if depth > maxDepth {
		return
	}
	for _, k := range parent.kids {
		if r.ctx.Err() != nil {
			return
		}
		if k.name == "" || droppedBlocks[k.name] {
			continue
		}
		switch k.name {
		case "text:p":
			r.para(b, k)
		case "text:h":
			r.heading(b, k, 0)
		case "text:list":
			b.add(r.list(k, 1, "")...)
		case "table:table":
			b.add(r.table(k)...)
		case "text:section":
			if k.attr("text:display") == "none" {
				continue
			}
			b.flush()
			start := len(b.out)
			r.blocksInto(b, k, depth+1)
			b.flush()
			if id := r.targetID(k.attr("text:name")); id != "" && len(b.out) > start {
				rest := attachID(b.out[start:], id)
				b.out = append(b.out[:start], rest...)
			}
		case "text:bibliography":
			b.add(&ast.Bibliography{})
		case "text:numbered-paragraph":
			r.blocksInto(b, k, depth+1)
		case "math:math":
			if k.tex != "" {
				b.add(&ast.MathBlock{TeX: k.tex})
			}
		case "draw:frame", "draw:a", "draw:custom-shape", "draw:rect", "draw:g":
			c := &content{seq: -1}
			r.walkInline(&node{kids: []*node{k}}, 0, "", c, 0)
			b.add(c.floats...)
			if ins := inlines(c.runs); len(ins) > 0 {
				b.add(r.paraOrFigure(ins))
			}
		default:
			r.blocksInto(b, k, depth+1)
		}
	}
}

// attachID makes id point at the first of blocks.
func attachID(blocks []ast.Block, id string) []ast.Block {
	switch first := blocks[0].(type) {
	case *ast.Heading:
		if first.Attr.ID == "" {
			first.Attr.ID = id
			return blocks
		}
	case *ast.Para:
		first.Inlines = append([]ast.Inline{&ast.Span{Attr: ast.Attr{ID: id}}}, first.Inlines...)
		return blocks
	case *ast.Table:
		if first.Attr.ID == "" {
			first.Attr.ID = id
			return blocks
		}
	case *ast.Figure:
		if first.Attr.ID == "" {
			first.Attr.ID = id
			return blocks
		}
	}
	return []ast.Block{&ast.Div{Attr: ast.Attr{ID: id}, Blocks: blocks}}
}

func (r *reader) paraOrFigure(ins []ast.Inline) ast.Block {
	if len(ins) == 1 {
		switch n := ins[0].(type) {
		case *ast.Image:
			return &ast.Figure{Image: n}
		case *ast.Math:
			return &ast.MathBlock{TeX: n.TeX}
		}
	}
	return &ast.Para{Inlines: ins}
}

func (r *reader) para(b *blockBuilder, n *node) {
	st := r.styles.para(n.attr("text:style-name"))
	if st.breakBefore || st.masterPage != "" {
		b.pageBreak()
	}
	defer func() {
		if st.breakAfter {
			b.pageBreak()
		}
	}()
	switch st.role {
	case roleHeading:
		r.heading(b, n, st.level)
		return
	case roleCode:
		c := r.inlineContent(n, 0)
		b.add(c.floats...)
		text := plainText(c.runs)
		if strings.TrimSpace(text) == "" {
			if len(b.code) > 0 {
				b.blank++
			}
			return
		}
		b.addCode(strings.TrimRight(text, " "))
		return
	}

	base := flags(0)
	if st.role == roleNormal {
		base = st.text.over(0)
	}
	c := r.inlineContent(n, base)
	if st.role == roleNormal && allCode(c.runs) && len(c.floats) == 0 {
		b.addCode(strings.TrimRight(plainText(c.runs), " "))
		return
	}
	ins := inlines(c.runs)
	switch st.role {
	case roleTitle:
		b.add(c.floats...)
		if t := ast.PlainText(ins); t != "" {
			r.title = append(r.title, t)
		}
		return
	case roleSubtitle:
		b.add(c.floats...)
		if t := ast.PlainText(ins); t != "" {
			r.subtitle = append(r.subtitle, t)
		}
		return
	case roleCaption:
		r.caption(b, c, st.caption)
		return
	}
	var blocks []ast.Block
	blocks = append(blocks, c.floats...)
	if len(ins) > 0 {
		blocks = append(blocks, r.paraOrFigure(ins))
	}
	if st.role == roleQuote {
		if len(blocks) > 0 {
			b.addQuote(blocks...)
		}
		return
	}
	b.add(blocks...)
}

var captionLabel = regexp.MustCompile(`^(?i)(figure|fig\.|illustration|image|drawing|table|tab\.|listing|text|abbildung|abb\.|tabelle|attēls|tabula|zīmējums|рис\.|рисунок|таблица|figura|tabla|tableau|ilustración)\s*[0-9]+(\.[0-9]+)*\s*[:.–—-]?\s*`)

// caption handles a caption-styled paragraph outside a frame: the figure or
// table it belongs to is found by attachCaptions.
func (r *reader) caption(b *blockBuilder, c *content, kind string) {
	ins := captionInlines(c)
	if len(c.floats) > 0 {
		// A caption paragraph holding the image itself.
		if f, ok := c.floats[0].(*ast.Figure); ok && len(c.floats) == 1 && len(f.Caption) == 0 {
			f.Caption = ins
			b.add(f)
			return
		}
		b.add(c.floats...)
	}
	if len(ins) == 0 {
		return
	}
	if kind == "figure" && captionLabel.MatchString(ast.PlainText(inlines(c.runs))) {
		lower := strings.ToLower(ast.PlainText(inlines(c.runs)))
		if strings.HasPrefix(lower, "tab") {
			kind = "table"
		}
	}
	p := &ast.Para{Inlines: ins}
	r.captions[p] = kind
	b.add(p)
}

// captionInlines drops the "Figure 1:" label in front of a caption.
func captionInlines(c *content) []ast.Inline {
	runs := c.runs
	if c.seq >= 0 && c.seq <= len(runs) {
		// Everything before the number field is the category label; the
		// separator follows the number.
		runs = append([]run(nil), runs[c.seq:]...)
		for len(runs) > 0 && runs[0].inl == nil {
			t := strings.TrimLeft(runs[0].text, " :.–—-")
			if t != "" {
				runs[0].text = t
				break
			}
			runs = runs[1:]
		}
		return inlines(runs)
	}
	ins := inlines(runs)
	if len(ins) > 0 {
		if t, ok := ins[0].(*ast.Text); ok {
			if loc := captionLabel.FindStringIndex(t.Value); loc != nil {
				rest := t.Value[loc[1]:]
				out := append([]ast.Inline(nil), ins[1:]...)
				if rest != "" {
					out = append([]ast.Inline{&ast.Text{Value: rest}}, out...)
				}
				return out
			}
		}
	}
	return ins
}

// attachCaptions moves caption paragraphs onto the adjacent figure or
// table; captions without a partner stay as paragraphs.
func (r *reader) attachCaptions(blocks []ast.Block) []ast.Block {
	out := blocks[:0:0]
	for i := 0; i < len(blocks); i++ {
		p, ok := blocks[i].(*ast.Para)
		kind, isCap := r.captions[p]
		if !ok || !isCap {
			out = append(out, blocks[i])
			continue
		}
		if n := len(out); n > 0 {
			switch prev := out[n-1].(type) {
			case *ast.Figure:
				if kind != "table" && len(prev.Caption) == 0 {
					prev.Caption = p.Inlines
					continue
				}
			case *ast.Table:
				if kind == "table" && len(prev.Caption) == 0 {
					prev.Caption = p.Inlines
					continue
				}
			}
		}
		if i+1 < len(blocks) {
			switch next := blocks[i+1].(type) {
			case *ast.Table:
				if kind == "table" && len(next.Caption) == 0 {
					next.Caption = p.Inlines
					continue
				}
			case *ast.Figure:
				if kind != "table" && len(next.Caption) == 0 {
					next.Caption = p.Inlines
					continue
				}
			}
		}
		out = append(out, p)
	}
	return out
}

func (r *reader) heading(b *blockBuilder, n *node, level int) {
	if level == 0 {
		level = 1
		if v, err := strconv.Atoi(n.attr("text:outline-level")); err == nil && v > 0 {
			level = min(v, 10)
		}
	}
	st := r.styles.para(n.attr("text:style-name"))
	if n.name == "text:h" && st.breakBefore {
		b.pageBreak()
	}
	c := r.inlineContent(n, 0)
	ins := inlines(c.runs)
	id := ""
	if len(c.ids) > 0 {
		id = c.ids[0]
		ins = dropAnchor(ins, id)
	}
	if id == "" {
		id = r.targetID(ast.PlainText(ins))
	}
	b.add(c.floats...)
	if len(ins) == 0 {
		return
	}
	b.add(&ast.Heading{Level: level, Inlines: ins, Attr: ast.Attr{ID: id}})
	if n.name == "text:h" && st.breakAfter {
		b.pageBreak()
	}
}

// dropAnchor removes the anchor span carrying id from heading inlines.
func dropAnchor(ins []ast.Inline, id string) []ast.Inline {
	out := ins[:0:0]
	for _, in := range ins {
		if sp, ok := in.(*ast.Span); ok && sp.Attr.ID == id && len(sp.Inlines) == 0 {
			continue
		}
		out = append(out, in)
	}
	return out
}

func (r *reader) note(n *node) *ast.Note {
	body := n.child("text:note-body")
	if body == nil {
		return nil
	}
	blocks := r.blocks(body)
	if len(blocks) == 0 {
		return nil
	}
	return &ast.Note{Blocks: blocks}
}

// list converts text:list. level is the nesting depth (1 = outermost).
func (r *reader) list(n *node, level int, styleName string) []ast.Block {
	if sn := n.attr("text:style-name"); sn != "" {
		styleName = sn
	}
	if styleName == "" {
		styleName = r.firstParaListStyle(n)
	}
	lv := listLevel{start: 1}
	if levels := r.styles.listLevels(styleName); len(levels) > level {
		lv = levels[level]
	}
	l := &ast.List{Ordered: lv.ordered, Style: lv.style}
	start := lv.start
	if cont := n.attr("text:continue-list"); cont != "" {
		if c, ok := r.listCounts["id:"+cont]; ok {
			start = c + 1
		}
	} else if n.attr("text:continue-numbering") == "true" {
		if c, ok := r.listCounts["style:"+styleName]; ok && level == 1 {
			start = c + 1
		}
	}

	headingsOnly := true
	var headings []ast.Block
	for i, item := range n.kids {
		if item.name != "text:list-item" && item.name != "text:list-header" {
			continue
		}
		if i == 0 || len(l.Items) == 0 {
			if v, err := strconv.Atoi(item.attr("text:start-value")); err == nil && v > 0 {
				start = v
			}
		}
		blocks, onlyLists := r.itemBlocks(item, level, styleName)
		for _, b := range blocks {
			if _, ok := b.(*ast.Heading); !ok {
				headingsOnly = false
			}
		}
		headings = append(headings, blocks...)
		if len(blocks) == 0 {
			continue
		}
		if onlyLists && len(l.Items) > 0 {
			last := &l.Items[len(l.Items)-1]
			last.Blocks = append(last.Blocks, blocks...)
			continue
		}
		l.Items = append(l.Items, ast.ListItem{Blocks: blocks})
	}
	if headingsOnly && len(headings) > 0 {
		return headings // numbered headings are a list in ODF
	}
	if len(l.Items) == 0 {
		return nil
	}
	count := start + len(l.Items) - 1
	if level == 1 {
		r.listCounts["style:"+styleName] = count
	}
	if id := n.attr("xml:id"); id != "" {
		r.listCounts["id:"+id] = count
	}
	if l.Ordered && start != 1 {
		l.Start = start
	}
	l.Tight = true
	for i := range l.Items {
		paras := 0
		for _, b := range l.Items[i].Blocks {
			if _, ok := b.(*ast.Para); ok {
				paras++
			}
		}
		if paras > 1 {
			l.Tight = false
		}
	}
	if l.Tight {
		for i := range l.Items {
			for j, b := range l.Items[i].Blocks {
				if p, ok := b.(*ast.Para); ok {
					l.Items[i].Blocks[j] = &ast.Plain{Inlines: p.Inlines}
				}
			}
		}
	}
	return []ast.Block{l}
}

// firstParaListStyle finds a list style assigned through the paragraph
// style of the list's first paragraph.
func (r *reader) firstParaListStyle(n *node) string {
	if p := n.find("text:p"); p != nil {
		return r.styles.para(p.attr("text:style-name")).listStyle
	}
	if h := n.find("text:h"); h != nil {
		return r.styles.para(h.attr("text:style-name")).listStyle
	}
	return ""
}

// itemBlocks converts a list item; onlyLists reports an item that holds
// nothing but nested lists (how ODF writes deeper levels).
func (r *reader) itemBlocks(item *node, level int, styleName string) ([]ast.Block, bool) {
	b := &blockBuilder{}
	onlyLists := true
	for _, k := range item.kids {
		switch k.name {
		case "":
			continue
		case "text:list":
			b.add(r.list(k, level+1, styleName)...)
			continue
		case "text:number", "text:soft-page-break":
			continue
		}
		onlyLists = false
		r.blocksInto(b, &node{kids: []*node{k}}, 0)
	}
	b.flush()
	return r.attachCaptions(b.out), onlyLists
}
