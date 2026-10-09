package markdown

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	gast "github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

type converter struct {
	ctx       context.Context
	src       []byte
	res       *ast.Resources
	warns     rd.Warnings
	footnotes map[int][]ast.Block
	// imageCaptions keeps rich alt text so implicit figures get formatted
	// captions.
	imageCaptions map[*ast.Image][]ast.Inline
}

func newConverter(ctx context.Context, src []byte, res *ast.Resources) *converter {
	return &converter{ctx: ctx, src: src, res: res, footnotes: map[int][]ast.Block{}}
}

func (c *converter) document(root gast.Node) []ast.Block {
	// Footnote definitions are collected first so references can embed them.
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if list, ok := n.(*east.FootnoteList); ok {
			for f := list.FirstChild(); f != nil; f = f.NextSibling() {
				if fn, ok := f.(*east.Footnote); ok {
					c.footnotes[fn.Index] = c.blocks(fn)
				}
			}
		}
	}
	return c.blocks(root)
}

func (c *converter) blocks(parent gast.Node) []ast.Block {
	var out []ast.Block
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		if c.ctx.Err() != nil {
			return out
		}
		out = append(out, c.block(n)...)
	}
	return attachCaptions(out)
}

func (c *converter) block(n gast.Node) []ast.Block {
	switch n := n.(type) {
	case *gast.Paragraph:
		if c.isPageBreak(n) {
			return []ast.Block{&ast.PageBreak{}}
		}
		return c.paragraph(c.inlines(n), false)
	case *gast.TextBlock:
		return c.paragraph(c.inlines(n), true)
	case *gast.Heading:
		h := &ast.Heading{Level: n.Level, Inlines: ast.TrimInlines(c.inlines(n))}
		h.Attr = c.attributes(n)
		if h.Attr.HasClass("unnumbered") || h.Attr.HasClass("unlisted") {
			h.Unnumbered = true
		}
		// "Title {-}" is Pandoc shorthand for an unnumbered heading.
		if len(h.Inlines) > 0 {
			if t, ok := h.Inlines[len(h.Inlines)-1].(*ast.Text); ok && strings.HasSuffix(t.Value, "{-}") && !isEscapedAt(t.Value, len(t.Value)-3) {
				t.Value = strings.TrimSpace(strings.TrimSuffix(t.Value, "{-}"))
				h.Unnumbered = true
				// The automatic identifier was derived from "Title {-}".
				h.Attr.ID = strings.TrimRight(h.Attr.ID, "-")
			}
		}
		return []ast.Block{h}
	case *gast.ThematicBreak:
		return []ast.Block{&ast.HorizontalRule{}}
	case *gast.CodeBlock:
		return []ast.Block{&ast.CodeBlock{Text: c.lines(n)}}
	case *gast.FencedCodeBlock:
		return c.fencedCode(n)
	case *gast.Blockquote:
		return []ast.Block{c.blockquote(n)}
	case *gast.List:
		return []ast.Block{c.list(n)}
	case *gast.HTMLBlock:
		return c.htmlBlock(n)
	case *east.Table:
		return []ast.Block{c.table(n)}
	case *east.DefinitionList:
		return []ast.Block{c.definitionList(n)}
	case *east.FootnoteList:
		return nil
	case *divNode:
		if n.Attr.ID == "refs" {
			return []ast.Block{&ast.Bibliography{}} // Pandoc's reference list placeholder
		}
		return []ast.Block{c.div(n)}
	case *gast.Document:
		return c.blocks(n)
	}
	if n.Type() == gast.TypeBlock {
		return c.blocks(n)
	}
	return nil
}

func (c *converter) lines(n gast.Node) string {
	var sb strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		sb.Write(seg.Value(c.src))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (c *converter) attributes(n gast.Node) ast.Attr {
	var a ast.Attr
	for _, attr := range n.Attributes() {
		name := string(attr.Name)
		var val string
		switch v := attr.Value.(type) {
		case []byte:
			val = string(v)
		case string:
			val = v
		default:
			continue
		}
		switch name {
		case "id":
			a.ID = val
		case "class":
			a.Classes = append(a.Classes, strings.Fields(val)...)
		default:
			if a.KV == nil {
				a.KV = map[string]string{}
			}
			a.KV[name] = val
		}
	}
	return a
}

// ---------------------------------------------------------------------------
// Blocks
// ---------------------------------------------------------------------------

var rawFormatRe = regexp.MustCompile(`^\{=([A-Za-z0-9_-]+)\}$`)

func (c *converter) fencedCode(n *gast.FencedCodeBlock) []ast.Block {
	info := ""
	if n.Info != nil {
		info = strings.TrimSpace(string(n.Info.Segment.Value(c.src)))
	}
	text := c.lines(n)
	if m := rawFormatRe.FindStringSubmatch(info); m != nil {
		format := strings.ToLower(m[1])
		if format == "html" {
			return c.htmlString(text)
		}
		return []ast.Block{&ast.RawBlock{Format: format, Text: text}}
	}
	cb := &ast.CodeBlock{Text: text}
	switch {
	case strings.HasPrefix(info, "{"):
		end := strings.LastIndexByte(info, '}')
		if end < 0 {
			end = len(info)
		}
		cb.Attr = parseAttrString(info[1:end])
		// The first class names the language, unless it describes the
		// block's role ({.output}, {.numberLines}).
		for i, cl := range cb.Attr.Classes {
			if !codeRoleClasses[cl] {
				cb.Lang = cl
				cb.Attr.Classes = append(cb.Attr.Classes[:i:i], cb.Attr.Classes[i+1:]...)
				break
			}
		}
	default:
		lang, rest, _ := strings.Cut(info, " ")
		cb.Lang = lang
		if rest = strings.TrimSpace(rest); strings.HasPrefix(rest, "{") {
			end := strings.LastIndexByte(rest, '}')
			if end > 0 {
				cb.Attr = parseAttrString(rest[1:end])
			}
		}
	}
	cb.Lang = strings.ToLower(strings.Trim(cb.Lang, "{}."))
	if cap := cb.Attr.Get("caption"); cap != "" {
		cb.Caption = ast.Str(cap)
	}
	return []ast.Block{cb}
}

// codeRoleClasses are code block classes that are not languages.
var codeRoleClasses = map[string]bool{
	"input": true, "output": true, "error": true, "stderr": true,
	"numberLines": true, "number-lines": true, "numberlines": true,
}

var alertRe = regexp.MustCompile(`^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION|INFO|DANGER|SUCCESS|EXAMPLE|QUOTE|ABSTRACT|SUMMARY|BUG|QUESTION|FAILURE|ERROR)\][+-]?[ \t]*`)

func (c *converter) blockquote(n *gast.Blockquote) ast.Block {
	blocks := c.blocks(n)
	// GitHub/Obsidian alerts: > [!WARNING] Optional title
	if len(blocks) > 0 {
		if p, ok := blocks[0].(*ast.Para); ok && len(p.Inlines) > 0 {
			if t, ok := p.Inlines[0].(*ast.Text); ok {
				if m := alertRe.FindStringSubmatch(t.Value); m != nil {
					kind := strings.ToLower(m[1])
					switch kind {
					case "error", "failure", "bug":
						kind = "danger"
					case "question":
						kind = "info"
					case "summary":
						kind = "abstract"
					}
					rest := t.Value[len(m[0]):]
					inl := append([]ast.Inline{&ast.Text{Value: rest}}, p.Inlines[1:]...)
					div := &ast.Div{Attr: ast.Attr{Classes: []string{kind}}}
					// Text on the marker line is the title; the rest is the body.
					title, body := splitAtFirstBreak(inl)
					if title = ast.TrimInlines(title); len(title) > 0 {
						div.Title = title
					}
					if body = ast.TrimInlines(body); len(body) > 0 {
						div.Blocks = append(div.Blocks, &ast.Para{Inlines: body})
					}
					div.Blocks = append(div.Blocks, blocks[1:]...)
					return div
				}
			}
		}
	}
	return &ast.BlockQuote{Blocks: blocks}
}

func splitAtFirstBreak(ins []ast.Inline) ([]ast.Inline, []ast.Inline) {
	for i, in := range ins {
		switch in.(type) {
		case *ast.SoftBreak, *ast.LineBreak:
			return ins[:i], ins[i+1:]
		}
	}
	return ins, nil
}

func (c *converter) list(n *gast.List) ast.Block {
	l := &ast.List{Ordered: n.IsOrdered(), Tight: n.IsTight}
	if l.Ordered {
		l.Start = n.Start
	}
	for item := n.FirstChild(); item != nil; item = item.NextSibling() {
		li := ast.ListItem{}
		if first := item.FirstChild(); first != nil {
			if box, ok := first.FirstChild().(*east.TaskCheckBox); ok {
				li.Task = ast.TaskOpen
				if box.IsChecked {
					li.Task = ast.TaskDone
				}
			}
		}
		li.Blocks = c.blocks(item)
		l.Items = append(l.Items, li)
	}
	return l
}

func (c *converter) table(n *east.Table) ast.Block {
	t := &ast.Table{}
	for _, a := range n.Alignments {
		t.Cols = append(t.Cols, ast.ColSpec{Align: align(a)})
	}
	for r := n.FirstChild(); r != nil; r = r.NextSibling() {
		row := ast.Row{}
		for cell := r.FirstChild(); cell != nil; cell = cell.NextSibling() {
			inl := ast.TrimInlines(c.inlines(cell))
			unescapeCellMath(inl)
			var blocks []ast.Block
			if len(inl) > 0 {
				blocks = []ast.Block{&ast.Plain{Inlines: inl}}
			}
			row.Cells = append(row.Cells, ast.Cell{Blocks: blocks})
		}
		if _, ok := r.(*east.TableHeader); ok {
			// A header row of empty cells means "no header" (pipe
			// tables always need one) when body rows follow.
			if !emptyCells(row) || r.NextSibling() == nil {
				t.Head = append(t.Head, row)
			}
		} else {
			t.Body = append(t.Body, row)
		}
	}
	return t
}

// unescapeCellMath turns "\|" in table-cell math into "|": as in code
// spans, a pipe inside a pipe-table cell must be escaped.
func unescapeCellMath(ins []ast.Inline) {
	for _, in := range ins {
		if m, ok := in.(*ast.Math); ok {
			m.TeX = strings.ReplaceAll(m.TeX, `\|`, "|")
			continue
		}
		if _, ok := in.(*ast.Note); !ok {
			unescapeCellMath(ast.InlineChildren(in))
		}
	}
}

func emptyCells(r ast.Row) bool {
	for _, c := range r.Cells {
		if len(c.Blocks) > 0 {
			return false
		}
	}
	return true
}

func align(a east.Alignment) ast.Align {
	switch a {
	case east.AlignLeft:
		return ast.AlignLeft
	case east.AlignCenter:
		return ast.AlignCenter
	case east.AlignRight:
		return ast.AlignRight
	}
	return ast.AlignDefault
}

func (c *converter) definitionList(n *east.DefinitionList) ast.Block {
	dl := &ast.DefinitionList{}
	for x := n.FirstChild(); x != nil; x = x.NextSibling() {
		switch x := x.(type) {
		case *east.DefinitionTerm:
			dl.Items = append(dl.Items, ast.DefinitionItem{Term: ast.TrimInlines(c.inlines(x))})
		case *east.DefinitionDescription:
			if len(dl.Items) == 0 {
				dl.Items = append(dl.Items, ast.DefinitionItem{})
			}
			last := &dl.Items[len(dl.Items)-1]
			last.Definitions = append(last.Definitions, c.blocks(x))
		}
	}
	return dl
}

var calloutAliases = map[string]string{
	"note": "note", "notes": "note", "callout-note": "note", "admonition": "note", "seealso": "note",
	"tip": "tip", "hint": "tip", "callout-tip": "tip",
	"info": "info", "information": "info", "callout-info": "info",
	"important": "important", "callout-important": "important",
	"warning": "warning", "warn": "warning", "attention": "warning", "callout-warning": "warning",
	"caution": "caution", "callout-caution": "caution",
	"danger": "danger", "error": "danger", "callout-danger": "danger",
	"success": "success", "check": "success", "done": "success",
	"example": "example", "abstract": "abstract", "summary": "abstract",
	"theorem": "theorem", "lemma": "lemma", "corollary": "corollary", "proposition": "proposition",
	"definition": "definition", "remark": "remark", "proof": "proof", "exercise": "exercise", "solution": "solution",
	"appendix": "appendix", "center": "center", "landscape": "landscape",
}

func (c *converter) div(n *divNode) ast.Block {
	d := &ast.Div{Attr: n.Attr}
	if n.Title != "" {
		d.Title = ast.Str(n.Title)
	}
	for i, cl := range d.Attr.Classes {
		if alias, ok := calloutAliases[strings.ToLower(cl)]; ok {
			d.Attr.Classes[i] = alias
		}
	}
	d.Blocks = c.blocks(n)
	// Quarto puts the callout title in a leading heading (written as the
	// first thing inside the div).
	if _, ok := n.FirstChild().(*gast.Heading); ok && len(d.Title) == 0 && len(d.Blocks) > 0 && isCallout(d.Attr) {
		if h, ok := d.Blocks[0].(*ast.Heading); ok {
			d.Title = h.Inlines
			d.Blocks = d.Blocks[1:]
		}
	}
	return d
}

func isCallout(a ast.Attr) bool {
	for _, cl := range a.Classes {
		switch cl {
		case "note", "tip", "info", "important", "warning", "caution", "danger", "success", "example":
			return true
		}
	}
	return false
}

func (c *converter) htmlBlock(n *gast.HTMLBlock) []ast.Block {
	var sb strings.Builder
	sb.WriteString(c.lines(n))
	if n.HasClosure() {
		sb.WriteByte('\n')
		sb.Write(n.ClosureLine.Value(c.src))
	}
	return c.htmlString(sb.String())
}

func (c *converter) htmlString(src string) []ast.Block {
	trimmed := strings.TrimSpace(src)
	if pageBreakCommentRe.MatchString(trimmed) {
		return []ast.Block{&ast.PageBreak{}}
	}
	if trimmed == "" || (strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->")) {
		return nil
	}
	if HTMLFragment != nil {
		blocks, w := HTMLFragment(src, c.res)
		for _, x := range w {
			c.warns.Addf("%s", x)
		}
		return blocks
	}
	text := strings.TrimSpace(htmlText(src))
	if text == "" {
		return nil
	}
	return []ast.Block{&ast.Para{Inlines: ast.Str(text)}}
}

// pageBreakCommentRe matches the HTML comments used as page breaks
// (<!-- pagebreak -->, <!-- newpage -->).
var pageBreakCommentRe = regexp.MustCompile(`(?i)^<!--\s*(?:pagebreak|newpage|page-break)\s*-->$`)

// isPageBreak reports whether a paragraph is Pandoc's page-break
// convention: a line holding only \newpage or \pagebreak.
func (c *converter) isPageBreak(n *gast.Paragraph) bool {
	if n.Lines().Len() != 1 {
		return false
	}
	seg := n.Lines().At(0)
	switch strings.TrimSpace(string(seg.Value(c.src))) {
	case `\newpage`, `\pagebreak`:
		return true
	}
	return false
}

func htmlText(src string) string {
	z := html.NewTokenizer(strings.NewReader(src))
	var sb strings.Builder
	for {
		switch z.Next() {
		case html.ErrorToken:
			return rd.CleanText(sb.String())
		case html.TextToken:
			sb.Write(z.Text())
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			sb.WriteByte(' ')
		}
	}
}

// ---------------------------------------------------------------------------
// Paragraph post-processing
// ---------------------------------------------------------------------------

const displayMathFormat = "x-display-math"

func (c *converter) paragraph(ins []ast.Inline, plain bool) []ast.Block {
	ins = ast.TrimInlines(ins)
	if len(ins) == 0 {
		return nil
	}
	// Line block: every line starts with "| ".
	if lb := lineBlock(ins); lb != nil {
		return []ast.Block{lb}
	}
	// Display math splits the paragraph.
	var out []ast.Block
	var cur []ast.Inline
	flush := func() {
		cur = ast.TrimInlines(cur)
		if len(cur) == 0 {
			cur = nil
			return
		}
		out = append(out, c.textBlock(cur, plain)...)
		cur = nil
	}
	for i := 0; i < len(ins); i++ {
		if raw, ok := ins[i].(*ast.RawInline); ok && raw.Format == displayMathFormat {
			flush()
			mb := &ast.MathBlock{TeX: raw.Text}
			// $$…$$ {#eq:label}
			if i+1 < len(ins) {
				if t, ok := ins[i+1].(*ast.Text); ok {
					if attr, rest, ok := leadingAttr(t.Value); ok {
						mb.Label = attr.ID
						ins[i+1] = &ast.Text{Value: rest}
					}
				}
			}
			out = append(out, mb)
			continue
		}
		cur = append(cur, ins[i])
	}
	flush()
	return out
}

func (c *converter) textBlock(ins []ast.Inline, plain bool) []ast.Block {
	// A paragraph holding only an image is a figure (Pandoc implicit_figures).
	if len(ins) == 1 {
		if img, ok := ins[0].(*ast.Image); ok && !plain {
			fig := &ast.Figure{Image: img, Attr: ast.Attr{ID: img.Attr.ID}}
			img.Attr.ID = ""
			if cap, ok := c.imageCaptions[img]; ok {
				fig.Caption = cap
			}
			return []ast.Block{fig}
		}
	}
	if plain {
		return []ast.Block{&ast.Plain{Inlines: ins}}
	}
	return []ast.Block{&ast.Para{Inlines: ins}}
}

func lineBlock(ins []ast.Inline) ast.Block {
	t, ok := ins[0].(*ast.Text)
	if !ok || !strings.HasPrefix(t.Value, "| ") && t.Value != "|" {
		return nil
	}
	var lines [][]ast.Inline
	var cur []ast.Inline
	atStart := true
	for _, in := range ins {
		if _, ok := in.(*ast.SoftBreak); ok {
			lines = append(lines, cur)
			cur, atStart = nil, true
			continue
		}
		if atStart {
			tx, ok := in.(*ast.Text)
			if !ok || (!strings.HasPrefix(tx.Value, "| ") && tx.Value != "|") {
				return nil
			}
			rest := strings.TrimPrefix(tx.Value[1:], " ")
			// Keep indentation: leading spaces become no-break spaces.
			trimmed := strings.TrimLeft(rest, " ")
			atStart = false
			v := strings.Repeat("\u00a0", len(rest)-len(trimmed)) + trimmed
			if v == "" {
				continue // an empty line
			}
			in = &ast.Text{Value: v}
		}
		cur = append(cur, in)
	}
	lines = append(lines, cur)
	if len(lines) < 2 {
		return nil
	}
	return &ast.LineBlock{Lines: lines}
}

var leadingAttrRe = regexp.MustCompile(`^\s*\{([^{}]*)\}`)

// Escaped braces inside {…} are protected by same-length (four-byte)
// stand-ins while the attribute block is located.
var (
	protectBraces = strings.NewReplacer(string(escapeMark)+"{", "\U000F0001", string(escapeMark)+"}", "\U000F0002")
)

func leadingAttr(s string) (ast.Attr, string, bool) {
	p := protectBraces.Replace(s)
	m := leadingAttrRe.FindStringSubmatchIndex(p)
	if m == nil {
		return ast.Attr{}, s, false
	}
	// The attributes were part of a text run: escaped braces and quotes
	// are literal.
	inner := strings.NewReplacer("\U000F0001", string(escapeMark)+"{", "\U000F0002", string(escapeMark)+"}").Replace(p[m[2]:m[3]])
	return parseAttrString(inner), s[m[1]:], true
}

// attachCaptions moves "Table: caption" / ": caption" paragraphs adjacent to
// tables into the table, and "Listing: caption" into code blocks.
func attachCaptions(blocks []ast.Block) []ast.Block {
	out := blocks[:0]
	for i := 0; i < len(blocks); i++ {
		b := blocks[i]
		if cap, attr, ok := captionPara(b, "Table:"); ok {
			if i+1 < len(blocks) {
				if t, ok := blocks[i+1].(*ast.Table); ok && t.Caption == nil {
					t.Caption, t.Attr.ID = cap, firstNonEmpty(t.Attr.ID, attr.ID)
					continue
				}
			}
			if len(out) > 0 {
				if t, ok := out[len(out)-1].(*ast.Table); ok && t.Caption == nil {
					t.Caption, t.Attr.ID = cap, firstNonEmpty(t.Attr.ID, attr.ID)
					continue
				}
			}
		}
		if cap, attr, ok := captionPara(b, "Listing:"); ok && i+1 < len(blocks) {
			if cb, ok := blocks[i+1].(*ast.CodeBlock); ok && cb.Caption == nil {
				cb.Caption = cap
				if attr.ID != "" {
					cb.Attr.ID = attr.ID
				}
				continue
			}
		}
		out = append(out, b)
	}
	return out
}

func captionPara(b ast.Block, prefix string) ([]ast.Inline, ast.Attr, bool) {
	var inlines []ast.Inline
	switch p := b.(type) {
	case *ast.Para:
		inlines = p.Inlines
	case *ast.Plain:
		inlines = p.Inlines // a caption in a tight list item
	}
	if len(inlines) == 0 {
		return nil, ast.Attr{}, false
	}
	p := &ast.Para{Inlines: inlines}
	t, ok := p.Inlines[0].(*ast.Text)
	if !ok {
		return nil, ast.Attr{}, false
	}
	var rest string
	switch {
	case strings.HasPrefix(t.Value, prefix):
		rest = t.Value[len(prefix):]
	case prefix == "Table:" && strings.HasPrefix(t.Value, ": "):
		rest = t.Value[2:]
	default:
		return nil, ast.Attr{}, false
	}
	ins := append([]ast.Inline{&ast.Text{Value: strings.TrimLeft(rest, " ")}}, p.Inlines[1:]...)
	var attr ast.Attr
	// Trailing {#tbl:id}
	if last, ok := ins[len(ins)-1].(*ast.Text); ok {
		v := strings.TrimSpace(last.Value)
		if i := strings.LastIndex(last.Value, "{"); i >= 0 && !isEscapedAt(last.Value, i) && strings.HasSuffix(v, "}") && !isEscapedAt(v, len(v)-1) {
			attr = parseAttrString(strings.TrimSuffix(strings.TrimSpace(last.Value[i+1:]), "}"))
			ins[len(ins)-1] = &ast.Text{Value: strings.TrimRight(last.Value[:i], " ")}
		}
	}
	return ast.TrimInlines(ins), attr, true
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// Inlines
// ---------------------------------------------------------------------------

func (c *converter) inlines(parent gast.Node) []ast.Inline {
	var out []ast.Inline
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		out = append(out, c.inline(n)...)
	}
	out = ast.MergeText(c.pairHTML(out))
	return c.finishInlines(out, 0)
}

// finishInlines applies image attributes and smart punctuation, also
// inside formatting built from inline HTML tags.
func (c *converter) finishInlines(out []ast.Inline, depth int) []ast.Inline {
	out = c.imageAttributes(ast.MergeText(out))
	out = smartenText(out)
	if depth < 32 {
		for _, in := range out {
			switch n := in.(type) {
			case *ast.Emph, *ast.Strong, *ast.Strike, *ast.Underline, *ast.Superscript,
				*ast.Subscript, *ast.SmallCaps, *ast.Highlight, *ast.Span, *ast.Link:
				setInlineChildren(n, c.finishInlines(ast.InlineChildren(n), depth+1))
			}
		}
	}
	return out
}

func (c *converter) inline(n gast.Node) []ast.Inline {
	switch n := n.(type) {
	case *gast.Text:
		var v string
		if n.IsRaw() {
			v = string(n.Segment.Value(c.src))
		} else {
			v = unescapeText(n.Segment.Value(c.src))
		}
		out := []ast.Inline{&ast.Text{Value: v}}
		switch {
		case n.HardLineBreak():
			out = append(out, &ast.LineBreak{})
		case n.SoftLineBreak():
			out = append(out, &ast.SoftBreak{})
		}
		return out
	case *gast.String:
		v := string(n.Value)
		if !n.IsRaw() && !n.IsCode() {
			v = unescapeText(n.Value)
		}
		return []ast.Inline{&ast.Text{Value: v}}
	case *gast.CodeSpan:
		var sb strings.Builder
		for x := n.FirstChild(); x != nil; x = x.NextSibling() {
			switch t := x.(type) {
			case *gast.Text:
				sb.Write(t.Segment.Value(c.src))
			case *gast.String:
				sb.Write(t.Value)
			}
		}
		code := strings.ReplaceAll(sb.String(), "\n", " ")
		return []ast.Inline{&ast.Code{Text: code}}
	case *gast.Emphasis:
		ch := c.inlines(n)
		if n.Level >= 2 {
			return []ast.Inline{&ast.Strong{Inlines: ch}}
		}
		return []ast.Inline{&ast.Emph{Inlines: ch}}
	case *gast.Link:
		dest := unescape(n.Destination)
		return []ast.Inline{&ast.Link{URL: dest, Title: unescape(n.Title), Inlines: uncite(c.inlines(n))}}
	case *gast.Image:
		img := &ast.Image{Src: unescape(n.Destination), Title: unescape(n.Title)}
		caption := c.inlines(n)
		img.Alt = ast.PlainText(caption)
		c.rememberCaption(img, caption)
		c.storeDataImage(img)
		return []ast.Inline{img}
	case *gast.AutoLink:
		u := string(n.URL(c.src))
		label := string(n.Label(c.src))
		if n.AutoLinkType == gast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(u), "mailto:") {
			u = "mailto:" + u
		} else if strings.HasPrefix(strings.ToLower(u), "www.") {
			u = "https://" + u
		}
		return []ast.Inline{&ast.Link{URL: u, Inlines: []ast.Inline{&ast.Text{Value: label}}}}
	case *gast.RawHTML:
		var sb strings.Builder
		for i := 0; i < n.Segments.Len(); i++ {
			seg := n.Segments.At(i)
			sb.Write(seg.Value(c.src))
		}
		return []ast.Inline{&ast.RawInline{Format: "html", Text: sb.String()}}
	case *east.Strikethrough:
		return []ast.Inline{&ast.Strike{Inlines: c.inlines(n)}}
	case *east.TaskCheckBox:
		return nil
	case *east.FootnoteLink:
		return []ast.Inline{&ast.Note{Blocks: c.footnotes[n.Index]}}
	case *east.FootnoteBacklink:
		return nil
	case *mathNode:
		if n.Display {
			return []ast.Inline{&ast.RawInline{Format: displayMathFormat, Text: n.TeX}}
		}
		return []ast.Inline{&ast.Math{TeX: strings.TrimSpace(n.TeX)}}
	case *citeNode:
		cite := n.Cite
		cite.Fallback = []ast.Inline{&ast.Text{Value: n.Raw}}
		return []ast.Inline{&cite}
	case *highlightNode:
		return []ast.Inline{&ast.Highlight{Inlines: c.inlines(n)}}
	case *superNode:
		return []ast.Inline{&ast.Superscript{Inlines: c.inlines(n)}}
	case *subNode:
		return []ast.Inline{&ast.Subscript{Inlines: []ast.Inline{&ast.Text{Value: unescapeText([]byte(n.Value))}}}}
	case *inlineNoteNode:
		blocks, w := convertSource(c.ctx, n.Raw, c.res)
		for _, x := range w {
			c.warns.Addf("%s", x)
		}
		return []ast.Inline{&ast.Note{Blocks: blocks}}
	}
	return c.inlines(n)
}

func (c *converter) rememberCaption(img *ast.Image, caption []ast.Inline) {
	if c.imageCaptions == nil {
		c.imageCaptions = map[*ast.Image][]ast.Inline{}
	}
	if len(caption) > 0 {
		c.imageCaptions[img] = caption
	}
}

// storeDataImage moves data: URI images into resources.
func (c *converter) storeDataImage(img *ast.Image) {
	if !strings.HasPrefix(img.Src, "data:") {
		return
	}
	name, data, ok := decodeDataURI(img.Src)
	if !ok {
		c.warns.Addf("an embedded data: image could not be decoded")
		return
	}
	img.Src = c.res.Add(name, rd.MediaTypeFromName(name), data)
}

func decodeDataURI(s string) (string, []byte, bool) {
	meta, payload, ok := strings.Cut(strings.TrimPrefix(s, "data:"), ",")
	if !ok {
		return "", nil, false
	}
	mediaType := strings.Split(meta, ";")[0]
	var data []byte
	var err error
	if strings.HasSuffix(meta, ";base64") {
		data, err = base64.StdEncoding.DecodeString(strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, payload))
		if err != nil {
			data, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(payload, "="))
		}
	} else {
		var unescaped string
		unescaped, err = url.PathUnescape(payload)
		data = []byte(unescaped)
	}
	if err != nil || len(data) == 0 {
		return "", nil, false
	}
	ext := ".bin"
	switch mediaType {
	case "image/png":
		ext = ".png"
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	case "image/svg+xml":
		ext = ".svg"
	case "image/bmp":
		ext = ".bmp"
	case "image/tiff":
		ext = ".tif"
	}
	return "embedded/image" + ext, data, true
}

// uncite turns citation syntax inside link text back into text: "[@x](url)"
// is a link whose label happens to start with "@".
func uncite(ins []ast.Inline) []ast.Inline {
	for i, in := range ins {
		if c, ok := in.(*ast.Cite); ok {
			ins[i] = &ast.Text{Value: ast.PlainText(c.Fallback)}
		}
	}
	return ast.MergeText(ins)
}

// imageAttributes applies "{width=50% #fig:x .class}" written right after
// an image.
func (c *converter) imageAttributes(ins []ast.Inline) []ast.Inline {
	for i := 0; i+1 < len(ins); i++ {
		img, ok := ins[i].(*ast.Image)
		if !ok {
			continue
		}
		t, ok := ins[i+1].(*ast.Text)
		if !ok {
			continue
		}
		attr, rest, ok := leadingAttr(t.Value)
		if !ok {
			continue
		}
		img.Attr.ID = attr.ID
		img.Attr.Classes = attr.Classes
		img.Width = attr.Get("width")
		img.Height = attr.Get("height")
		// fig-alt (Pandoc) gives a figure alt text apart from its caption.
		if alt, ok := attr.KV["fig-alt"]; ok {
			img.Alt = alt
			delete(attr.KV, "fig-alt")
			if len(attr.KV) == 0 {
				attr.KV = nil
			}
		}
		img.Attr.KV = attr.KV
		if rest == "" {
			ins = append(ins[:i+1], ins[i+2:]...)
		} else {
			ins[i+1] = &ast.Text{Value: rest}
		}
	}
	return ins
}

// pairHTML turns inline HTML tags into AST formatting: <sup>x</sup>,
// <br>, <img>, <kbd>, <mark>, <u>, … Unknown tags are dropped, keeping
// their content.
func (c *converter) pairHTML(ins []ast.Inline) []ast.Inline {
	has := false
	for _, in := range ins {
		if r, ok := in.(*ast.RawInline); ok && r.Format == "html" {
			has = true
			break
		}
	}
	if !has {
		return ins
	}
	out, _ := c.pairFrom(ins, 0, "")
	return out
}

func (c *converter) pairFrom(ins []ast.Inline, i int, closing string) ([]ast.Inline, int) {
	var out []ast.Inline
	for ; i < len(ins); i++ {
		r, ok := ins[i].(*ast.RawInline)
		if !ok || r.Format != "html" {
			out = append(out, ins[i])
			continue
		}
		tag, attrs, kind := parseTag(r.Text)
		switch kind {
		case "comment":
			continue
		case "end":
			if tag == closing {
				return out, i
			}
			continue // stray closing tag
		case "self":
			out = append(out, c.voidTag(tag, attrs)...)
			continue
		}
		if isVoid(tag) {
			out = append(out, c.voidTag(tag, attrs)...)
			continue
		}
		children, j := c.pairFrom(ins, i+1, tag)
		i = j
		out = append(out, wrapTag(tag, attrs, children)...)
	}
	return out, i
}

func isVoid(tag string) bool {
	switch tag {
	case "br", "img", "hr", "wbr", "input":
		return true
	}
	return false
}

func (c *converter) voidTag(tag string, attrs map[string]string) []ast.Inline {
	switch tag {
	case "br":
		return []ast.Inline{&ast.LineBreak{}}
	case "wbr":
		return []ast.Inline{&ast.Text{Value: "\u200b"}}
	case "img":
		img := &ast.Image{Src: attrs["src"], Alt: attrs["alt"], Title: attrs["title"]}
		if w := attrs["width"]; w != "" {
			img.Width = cssLength(w)
		}
		if h := attrs["height"]; h != "" {
			img.Height = cssLength(h)
		}
		c.storeDataImage(img)
		return []ast.Inline{img}
	case "input":
		if attrs["type"] == "checkbox" {
			if _, ok := attrs["checked"]; ok {
				return []ast.Inline{&ast.Text{Value: "☒ "}}
			}
			return []ast.Inline{&ast.Text{Value: "☐ "}}
		}
	}
	return nil
}

func cssLength(v string) string {
	v = strings.TrimSpace(v)
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return v + "px"
	}
	return v
}

func wrapTag(tag string, attrs map[string]string, ch []ast.Inline) []ast.Inline {
	switch tag {
	case "sup":
		return []ast.Inline{&ast.Superscript{Inlines: ch}}
	case "sub":
		return []ast.Inline{&ast.Subscript{Inlines: ch}}
	case "b", "strong":
		return []ast.Inline{&ast.Strong{Inlines: ch}}
	case "i", "em", "cite", "dfn", "var":
		return []ast.Inline{&ast.Emph{Inlines: ch}}
	case "u", "ins":
		return []ast.Inline{&ast.Underline{Inlines: ch}}
	case "s", "del", "strike":
		return []ast.Inline{&ast.Strike{Inlines: ch}}
	case "mark":
		return []ast.Inline{&ast.Highlight{Inlines: ch}}
	case "small":
		return ch
	case "kbd":
		return []ast.Inline{&ast.Span{Attr: ast.Attr{Classes: []string{"kbd"}}, Inlines: ch}}
	case "code", "tt", "samp":
		return []ast.Inline{&ast.Code{Text: ast.PlainText(ch)}}
	case "a":
		if href := attrs["href"]; href != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(href)), "javascript:") {
			return []ast.Inline{&ast.Link{URL: href, Title: attrs["title"], Inlines: ch}}
		}
		return ch
	case "span":
		return spanTag(attrs, ch)
	case "q":
		return append(append([]ast.Inline{&ast.Text{Value: "\""}}, ch...), &ast.Text{Value: "\""})
	}
	return ch
}

func setInlineChildren(in ast.Inline, kids []ast.Inline) {
	switch n := in.(type) {
	case *ast.Emph:
		n.Inlines = kids
	case *ast.Strong:
		n.Inlines = kids
	case *ast.Strike:
		n.Inlines = kids
	case *ast.Underline:
		n.Inlines = kids
	case *ast.Superscript:
		n.Inlines = kids
	case *ast.Subscript:
		n.Inlines = kids
	case *ast.SmallCaps:
		n.Inlines = kids
	case *ast.Highlight:
		n.Inlines = kids
	case *ast.Span:
		n.Inlines = kids
	case *ast.Link:
		n.Inlines = kids
	}
}

// spanTag converts <span>: the classes smallcaps, underline and mark
// become formatting (as in the HTML reader); other spans keep their id,
// classes and attributes.
func spanTag(attrs map[string]string, ch []ast.Inline) []ast.Inline {
	var a ast.Attr
	for k, v := range attrs {
		switch k {
		case "id":
			a.ID = v
		case "class":
			a.Classes = strings.Fields(v)
		default:
			if a.KV == nil {
				a.KV = map[string]string{}
			}
			a.KV[k] = v
		}
	}
	if a.ID == "" && len(a.KV) == 0 && len(a.Classes) == 1 {
		switch a.Classes[0] {
		case "smallcaps":
			return []ast.Inline{&ast.SmallCaps{Inlines: ch}}
		case "underline":
			return []ast.Inline{&ast.Underline{Inlines: ch}}
		case "mark":
			return []ast.Inline{&ast.Highlight{Inlines: ch}}
		}
	}
	if a.ID == "" && len(a.Classes) == 0 && len(a.KV) == 0 {
		return ch
	}
	return []ast.Inline{&ast.Span{Attr: a, Inlines: ch}}
}

// parseTag inspects a single raw HTML inline token.
func parseTag(raw string) (tag string, attrs map[string]string, kind string) {
	if strings.HasPrefix(raw, "<!--") {
		return "", nil, "comment"
	}
	z := html.NewTokenizer(strings.NewReader(raw))
	tt := z.Next()
	name, hasAttr := z.TagName()
	tag = strings.ToLower(string(name))
	attrs = map[string]string{}
	for hasAttr {
		var k, v []byte
		k, v, hasAttr = z.TagAttr()
		attrs[strings.ToLower(string(k))] = string(v)
	}
	switch tt {
	case html.EndTagToken:
		return tag, attrs, "end"
	case html.SelfClosingTagToken:
		return tag, attrs, "self"
	case html.StartTagToken:
		return tag, attrs, "start"
	}
	return tag, attrs, "comment"
}

// unescape resolves Markdown backslash escapes and HTML entities.
func unescape(b []byte) string {
	v := util.UnescapePunctuations(b)
	v = util.ResolveNumericReferences(v)
	v = util.ResolveEntityNames(v)
	return string(v)
}

// escapeMark precedes every character that was written as a backslash
// escape or an entity while a text run is being converted. The checks that
// work on converted text (smart punctuation, line blocks, alerts, captions,
// "{-}" and "{…}" attribute suffixes) therefore never match escaped
// characters; stripEscapeMarks removes the marks afterwards.
const escapeMark = '\uFDD0'

// unescapeText resolves backslash escapes and entities like unescape, but
// marks each resolved character with escapeMark.
func unescapeText(b []byte) string {
	if bytes.IndexByte(b, '\\') < 0 && bytes.IndexByte(b, '&') < 0 {
		return string(b)
	}
	var sb strings.Builder
	for i := 0; i < len(b); {
		switch {
		case b[i] == '\\' && i+1 < len(b) && util.IsPunct(b[i+1]):
			sb.WriteRune(escapeMark)
			sb.WriteByte(b[i+1])
			i += 2
			continue
		case b[i] == '&':
			if m := entityRe.Find(b[i:]); m != nil {
				ent, end := m, len(m)-1
				if r := unescape(ent); r != string(ent) {
					sb.WriteRune(escapeMark)
					sb.WriteString(r)
					i += end + 1
					continue
				}
			}
		}
		sb.WriteByte(b[i])
		i++
	}
	return sb.String()
}

var entityRe = regexp.MustCompile(`^&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{0,31});`)

// isEscapedAt reports whether the character at byte offset i of s was
// escaped in the source.
func isEscapedAt(s string, i int) bool {
	return i >= 3 && s[i-3:i] == string(escapeMark)
}

// stripEscapeMarks removes escape marks from all text in blocks. A mark
// protects the character after it, which is kept even if it is itself the
// mark's code point (written as an entity).
func stripEscapeMarks(blocks []ast.Block) {
	strip := func(s string) string {
		if !strings.ContainsRune(s, escapeMark) {
			return s
		}
		var sb strings.Builder
		skip := false
		for _, r := range s {
			if r == escapeMark && !skip {
				skip = true
				continue
			}
			skip = false
			sb.WriteRune(r)
		}
		return sb.String()
	}
	ast.WalkInlines(blocks, func(in ast.Inline) {
		switch n := in.(type) {
		case *ast.Text:
			n.Value = strip(n.Value)
		case *ast.Image:
			n.Alt = strip(n.Alt)
		}
	})
}

// smartenText applies Pandoc's "smart" punctuation to text runs: --- → —,
// -- → –, ... → …. Quotes are left for the typesetter, which knows the
// document language.
func smartenText(ins []ast.Inline) []ast.Inline {
	for _, in := range ins {
		if t, ok := in.(*ast.Text); ok && strings.ContainsAny(t.Value, "-.") {
			t.Value = smartReplacer.Replace(t.Value)
		}
	}
	return ins
}

var smartReplacer = strings.NewReplacer("---", "—", "--", "–", "...", "…")
