package markdown

import (
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// htmlTable writes a table Markdown tables cannot express (spans, several
// header rows, footers, cells with several blocks) as an HTML block, which
// the reader converts back. The block must not contain blank lines.
func (w *writer) htmlTable(t *ast.Table) string {
	h := &htmlWriter{w: w}
	h.table(t, true)
	out := h.sb.String()
	if len(h.notes) > 0 {
		out += "\n" + strings.Join(h.notes, "\n")
	}
	return out
}

type htmlWriter struct {
	w     *writer
	sb    strings.Builder
	notes []string
	depth int
}

func (h *htmlWriter) table(t *ast.Table, top bool) {
	h.sb.WriteString("<table")
	if id := safeID(t.Attr.ID); id != "" {
		h.sb.WriteString(` id="` + htmlEscape(id) + `"`)
	}
	ncols := len(t.Cols)
	for _, rows := range [][]ast.Row{t.Head, t.Body, t.Foot} {
		for _, r := range rows {
			n := 0
			for _, c := range r.Cells {
				n += max(c.ColSpan, 1)
			}
			ncols = max(ncols, n)
		}
	}
	nrows := len(t.Head) + len(t.Body) + len(t.Foot)
	if ncols <= 1 || nrows <= 1 {
		// The HTML reader treats one-cell and one-column tables as layout.
		h.sb.WriteString(` role="table"`)
	}
	h.sb.WriteString(">\n")
	if len(t.Caption) > 0 {
		h.sb.WriteString("<caption>" + h.inlines(t.Caption) + "</caption>\n")
	}
	section := func(tag string, rows []ast.Row, cellTag string) {
		if len(rows) == 0 {
			return
		}
		h.sb.WriteString("<" + tag + ">\n")
		for _, r := range rows {
			h.sb.WriteString("<tr>\n")
			col := 0
			for _, c := range r.Cells {
				h.sb.WriteString("<" + cellTag)
				if c.ColSpan > 1 {
					h.sb.WriteString(` colspan="` + strconv.Itoa(c.ColSpan) + `"`)
				}
				if c.RowSpan > 1 {
					h.sb.WriteString(` rowspan="` + strconv.Itoa(c.RowSpan) + `"`)
				}
				a := c.Align
				if a == ast.AlignDefault && col < len(t.Cols) {
					a = t.Cols[col].Align
				}
				switch a {
				case ast.AlignLeft:
					h.sb.WriteString(` align="left"`)
				case ast.AlignCenter:
					h.sb.WriteString(` align="center"`)
				case ast.AlignRight:
					h.sb.WriteString(` align="right"`)
				}
				h.sb.WriteString(">")
				h.cellBlocks(c.Blocks)
				h.sb.WriteString("</" + cellTag + ">\n")
				col += max(c.ColSpan, 1)
			}
			h.sb.WriteString("</tr>\n")
		}
		h.sb.WriteString("</" + tag + ">\n")
	}
	section("thead", t.Head, "th")
	section("tbody", t.Body, "td")
	section("tfoot", t.Foot, "td")
	h.sb.WriteString("</table>")
	if top {
		return
	}
	h.sb.WriteString("\n")
}

func (h *htmlWriter) cellBlocks(blocks []ast.Block) {
	if len(blocks) == 1 && isText(blocks[0]) {
		h.sb.WriteString(h.inlines(blockInlines(blocks[0])))
		return
	}
	for _, b := range blocks {
		h.block(b)
	}
}

func (h *htmlWriter) blocks(blocks []ast.Block) {
	for _, b := range blocks {
		h.block(b)
	}
}

func (h *htmlWriter) block(b ast.Block) {
	if h.depth > maxDepth {
		return
	}
	h.depth++
	defer func() { h.depth-- }()
	switch n := b.(type) {
	case *ast.Para:
		h.sb.WriteString("<p>" + h.inlines(n.Inlines) + "</p>\n")
	case *ast.Plain:
		h.sb.WriteString("<p>" + h.inlines(n.Inlines) + "</p>\n")
	case *ast.Heading:
		// Headings inside cells make the reader treat a table as layout.
		h.sb.WriteString("<p><strong>" + h.inlines(n.Inlines) + "</strong></p>\n")
		h.w.warn("heading inside a table cell written as bold text")
	case *ast.CodeBlock:
		h.sb.WriteString("<pre><code")
		if lang := safeWord(n.Lang); lang != "" {
			h.sb.WriteString(` class="language-` + htmlEscape(lang) + `"`)
		}
		h.sb.WriteString(">" + strings.ReplaceAll(htmlEscape(cleanCode(n.Text)), "\n", "&#10;") + "</code></pre>\n")
	case *ast.MathBlock:
		h.sb.WriteString(`<p><span class="math display">\[` + htmlEscape(strings.Join(strings.Fields(n.TeX), " ")) + `\]</span></p>` + "\n")
	case *ast.RawBlock:
		if strings.EqualFold(n.Format, "html") {
			h.sb.WriteString(noBlankLines(n.Text) + "\n")
		} else {
			h.w.warn("raw %s block inside a table cell dropped", n.Format)
		}
	case *ast.BlockQuote:
		h.sb.WriteString("<blockquote>\n")
		h.blocks(n.Blocks)
		h.sb.WriteString("</blockquote>\n")
	case *ast.List:
		tag := "ul"
		if n.Ordered {
			tag = "ol"
		}
		h.sb.WriteString("<" + tag)
		if n.Ordered && n.Start > 1 {
			h.sb.WriteString(` start="` + strconv.Itoa(n.Start) + `"`)
		}
		h.sb.WriteString(">\n")
		for _, it := range n.Items {
			h.sb.WriteString("<li>")
			switch it.Task {
			case ast.TaskOpen:
				h.sb.WriteString(`<input type="checkbox"> `)
			case ast.TaskDone:
				h.sb.WriteString(`<input type="checkbox" checked> `)
			}
			if len(it.Blocks) == 1 && isText(it.Blocks[0]) {
				h.sb.WriteString(h.inlines(blockInlines(it.Blocks[0])))
			} else {
				h.sb.WriteString("\n")
				h.blocks(it.Blocks)
			}
			h.sb.WriteString("</li>\n")
		}
		h.sb.WriteString("</" + tag + ">\n")
	case *ast.DefinitionList:
		h.sb.WriteString("<dl>\n")
		for _, it := range n.Items {
			h.sb.WriteString("<dt>" + h.inlines(it.Term) + "</dt>\n")
			for _, d := range it.Definitions {
				h.sb.WriteString("<dd>\n")
				h.blocks(d)
				h.sb.WriteString("</dd>\n")
			}
		}
		h.sb.WriteString("</dl>\n")
	case *ast.Table:
		h.w.warn("table nested in a table cell written as text")
		for _, cb := range ast.Children(n) {
			h.block(cb)
		}
	case *ast.Figure:
		h.sb.WriteString("<figure>")
		if n.Image != nil {
			h.sb.WriteString(h.img(n.Image))
		}
		if len(n.Caption) > 0 {
			h.sb.WriteString("<figcaption>" + h.inlines(n.Caption) + "</figcaption>")
		}
		h.sb.WriteString("</figure>\n")
	case *ast.HorizontalRule:
		h.sb.WriteString("<hr>\n")
	case *ast.PageBreak:
		h.w.warn("page break inside a table cell dropped")
	case *ast.Div:
		h.sb.WriteString("<div" + htmlAttrs(n.Attr) + ">\n")
		if len(n.Title) > 0 {
			h.sb.WriteString("<p><strong>" + h.inlines(n.Title) + "</strong></p>\n")
		}
		h.blocks(n.Blocks)
		h.sb.WriteString("</div>\n")
	case *ast.LineBlock:
		h.sb.WriteString("<p>")
		for i, l := range n.Lines {
			if i > 0 {
				h.sb.WriteString("<br>")
			}
			h.sb.WriteString(h.inlines(l))
		}
		h.sb.WriteString("</p>\n")
	case *ast.ReferenceList:
		for _, e := range n.Entries {
			h.sb.WriteString("<p>")
			if e.Label != "" {
				h.sb.WriteString("[" + htmlEscape(e.Label) + "] ")
			}
			h.sb.WriteString(h.inlines(e.Inlines) + "</p>\n")
		}
	case *ast.Bibliography:
		h.w.warn("reference list position inside a table cell dropped")
	}
}

func noBlankLines(s string) string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func (h *htmlWriter) inlines(ins []ast.Inline) string {
	var sb strings.Builder
	h.inl(&sb, ins)
	return sb.String()
}

func (h *htmlWriter) inl(sb *strings.Builder, ins []ast.Inline) {
	if h.depth > maxDepth {
		return
	}
	h.depth++
	defer func() { h.depth-- }()
	wrap := func(tag, attrs string, kids []ast.Inline) {
		sb.WriteString("<" + tag + attrs + ">")
		h.inl(sb, kids)
		sb.WriteString("</" + tag + ">")
	}
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			sb.WriteString(htmlEscape(cleanText(n.Value)))
		case *ast.Emph:
			wrap("em", "", n.Inlines)
		case *ast.Strong:
			wrap("strong", "", n.Inlines)
		case *ast.Strike:
			wrap("del", "", n.Inlines)
		case *ast.Underline:
			wrap("u", "", n.Inlines)
		case *ast.Superscript:
			wrap("sup", "", n.Inlines)
		case *ast.Subscript:
			wrap("sub", "", n.Inlines)
		case *ast.SmallCaps:
			wrap("span", ` class="smallcaps"`, n.Inlines)
		case *ast.Highlight:
			wrap("mark", "", n.Inlines)
		case *ast.Span:
			if n.Attr.ID == "" && len(n.Attr.KV) == 0 && len(n.Attr.Classes) == 1 && n.Attr.Classes[0] == "kbd" {
				wrap("kbd", "", n.Inlines)
			} else {
				wrap("span", htmlAttrs(n.Attr), n.Inlines)
			}
		case *ast.Code:
			sb.WriteString("<code>" + htmlEscape(cleanText(n.Text)) + "</code>")
		case *ast.Math:
			sb.WriteString(`<span class="math inline">\(` + htmlEscape(strings.Join(strings.Fields(n.TeX), " ")) + `\)</span>`)
		case *ast.Link:
			sb.WriteString(`<a href="` + htmlEscape(n.URL) + `"`)
			if n.Title != "" {
				sb.WriteString(` title="` + htmlEscape(n.Title) + `"`)
			}
			sb.WriteString(">")
			h.inl(sb, n.Inlines)
			sb.WriteString("</a>")
		case *ast.Image:
			sb.WriteString(h.img(n))
		case *ast.LineBreak:
			sb.WriteString("<br>")
		case *ast.SoftBreak:
			sb.WriteByte(' ')
		case *ast.Note:
			h.w.htmlNotes++
			id := "cd-note-" + strconv.Itoa(h.w.htmlNotes)
			sb.WriteString(`<a href="#` + id + `" role="doc-noteref">` + strconv.Itoa(h.w.htmlNotes) + `</a>`)
			sub := &htmlWriter{w: h.w, depth: h.depth}
			sub.blocks(n.Blocks)
			h.notes = append(h.notes, `<aside id="`+id+`" role="doc-footnote">`+"\n"+strings.TrimSuffix(sub.sb.String(), "\n")+"\n</aside>")
			h.notes = append(h.notes, sub.notes...)
		case *ast.Cite:
			h.w.warn("citation inside a complex table written as text")
			if n.Rendered != nil {
				h.inl(sb, n.Rendered)
			} else {
				h.inl(sb, n.Fallback)
			}
		case *ast.Ref:
			h.w.warn("cross-reference inside a complex table written as a link")
			sb.WriteString(`<a href="#` + htmlEscape(n.Target) + `">` + htmlEscape(n.Target) + `</a>`)
		case *ast.RawInline:
			if strings.EqualFold(n.Format, "html") {
				sb.WriteString(strings.Join(strings.Fields(n.Text), " "))
			}
		}
	}
}

func (h *htmlWriter) img(n *ast.Image) string {
	src := h.w.media.resolve(h.w, n.Src)
	var sb strings.Builder
	sb.WriteString(`<img src="` + htmlEscape(src) + `" alt="` + htmlEscape(n.Alt) + `"`)
	if n.Title != "" {
		sb.WriteString(` title="` + htmlEscape(n.Title) + `"`)
	}
	if n.Width != "" {
		sb.WriteString(` width="` + htmlEscape(n.Width) + `"`)
	}
	if n.Height != "" {
		sb.WriteString(` height="` + htmlEscape(n.Height) + `"`)
	}
	sb.WriteString(">")
	return sb.String()
}
