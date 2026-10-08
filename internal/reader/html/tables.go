package html

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
)

type section int

const (
	secHead section = iota
	secBody
	secFoot
)

type rowNode struct {
	tr  *html.Node
	sec section
}

// tableRows lists the rows of t (not of nested tables) with their section.
func tableRows(t *html.Node) []rowNode {
	var rows []rowNode
	var visit func(p *html.Node, sec section)
	visit = func(p *html.Node, sec section) {
		for ch := p.FirstChild; ch != nil; ch = ch.NextSibling {
			switch {
			case isElem(ch, "tr"):
				rows = append(rows, rowNode{ch, sec})
			case isElem(ch, "thead"):
				visit(ch, secHead)
			case isElem(ch, "tbody"):
				visit(ch, secBody)
			case isElem(ch, "tfoot"):
				visit(ch, secFoot)
			}
		}
	}
	visit(t, secBody)
	// Browsers render tfoot last wherever it appears.
	var out []rowNode
	for _, s := range []section{secHead, secBody, secFoot} {
		for _, r := range rows {
			if r.sec == s {
				out = append(out, r)
			}
		}
	}
	return out
}

func (c *conv) rowCells(tr *html.Node) []*html.Node {
	var cells []*html.Node
	for ch := tr.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, "td", "th") && !c.skip(ch) {
			cells = append(cells, ch)
		}
	}
	return cells
}

func (c *conv) table(t *html.Node) []ast.Block {
	rows := tableRows(t)
	var caption []ast.Inline
	for ch := t.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, "caption") && !c.skip(ch) {
			caption = stripLabel(normalize(c.inlineChildren(ch)))
			break
		}
	}
	if c.isLayoutTable(t, rows) {
		return c.unwrapLayout(rows, caption)
	}
	tbl := &ast.Table{Caption: caption}
	tbl.Attr.ID = c.keptID(t)

	var head, body, foot [][]*html.Node
	for _, r := range rows {
		cells := c.rowCells(r.tr)
		if len(cells) == 0 {
			continue
		}
		switch r.sec {
		case secHead:
			head = append(head, cells)
		case secFoot:
			foot = append(foot, cells)
		default:
			body = append(body, cells)
		}
	}
	// Without <thead>, leading rows made only of <th> are header rows.
	if len(head) == 0 {
		for len(body) > 1 && allTH(body[0]) {
			head = append(head, body[0])
			body = body[1:]
		}
	}
	truncated := false
	convert := func(rows [][]*html.Node) []ast.Row {
		var out []ast.Row
		for i, cells := range rows {
			if c.ctx.Err() != nil || truncated {
				break
			}
			if c.cells+len(cells) > c.opts.Limits.MaxTableCells {
				truncated = true
				break
			}
			c.cells += len(cells)
			row := ast.Row{Cells: make([]ast.Cell, 0, len(cells))}
			trAlign := alignOf(cells[0].Parent)
			for _, td := range cells {
				row.Cells = append(row.Cells, c.cell(td, len(rows)-i, trAlign))
			}
			out = append(out, row)
		}
		return out
	}
	tbl.Head = convert(head)
	tbl.Body = convert(body)
	tbl.Foot = convert(foot)
	if truncated {
		c.warn.Addf("html: table truncated at %d cells", c.opts.Limits.MaxTableCells)
	}
	for len(tbl.Body) > 0 && emptyRow(tbl.Body[len(tbl.Body)-1]) {
		tbl.Body = tbl.Body[:len(tbl.Body)-1]
	}
	if len(tbl.Head) == 0 && len(tbl.Body) > 1 && allBold(tbl.Body[0]) {
		tbl.Head = []ast.Row{unbold(tbl.Body[0])}
		tbl.Body = tbl.Body[1:]
	}
	if len(tbl.Head)+len(tbl.Body)+len(tbl.Foot) == 0 {
		if len(caption) > 0 {
			return []ast.Block{&ast.Para{Inlines: caption}}
		}
		return nil
	}
	tbl.Cols = columns(tbl)
	return []ast.Block{tbl}
}

func emptyRow(r ast.Row) bool {
	for _, c := range r.Cells {
		if len(c.Blocks) > 0 || c.RowSpan > 1 {
			return false
		}
	}
	return true
}

func allTH(cells []*html.Node) bool {
	for _, td := range cells {
		if td.Data != "th" {
			return false
		}
	}
	return len(cells) > 0
}

func (c *conv) cell(td *html.Node, rowsLeft int, rowAlign ast.Align) ast.Cell {
	blocks := c.blocks(td, true)
	if len(blocks) == 1 {
		if p, ok := blocks[0].(*ast.Para); ok {
			blocks[0] = &ast.Plain{Inlines: p.Inlines}
		}
	}
	if id := c.keptID(td); id != "" {
		blocks = attachID(blocks, id)
	}
	cell := ast.Cell{Blocks: blocks, Align: alignOf(td)}
	if cell.Align == ast.AlignDefault {
		cell.Align = rowAlign
	}
	if v := spanAttr(td, "colspan", 1000); v > 1 {
		cell.ColSpan = v
	}
	rs := spanAttr(td, "rowspan", 65534)
	if strings.TrimSpace(attr(td, "rowspan")) == "0" {
		rs = rowsLeft // rowspan=0 spans the rest of the section
	}
	if rs > rowsLeft {
		rs = rowsLeft
	}
	if rs > 1 {
		cell.RowSpan = rs
	}
	return cell
}

func spanAttr(n *html.Node, key string, max int) int {
	v, err := strconv.Atoi(strings.TrimSpace(attr(n, key)))
	if err != nil || v < 1 {
		return 1
	}
	if v > max {
		return max
	}
	return v
}

func alignOf(n *html.Node) ast.Align {
	if n == nil {
		return ast.AlignDefault
	}
	a := strings.ToLower(strings.TrimSpace(attr(n, "align")))
	if a == "" {
		a = styleProp(n, "text-align")
	}
	switch a {
	case "left", "start":
		return ast.AlignLeft
	case "center", "middle":
		return ast.AlignCenter
	case "right", "end":
		return ast.AlignRight
	}
	return ast.AlignDefault
}

func allBold(r ast.Row) bool {
	if len(r.Cells) == 0 {
		return false
	}
	for _, cell := range r.Cells {
		if len(cell.Blocks) != 1 {
			return false
		}
		p, ok := cell.Blocks[0].(*ast.Plain)
		if !ok || len(p.Inlines) != 1 {
			return false
		}
		if _, ok := p.Inlines[0].(*ast.Strong); !ok {
			return false
		}
	}
	return true
}

func unbold(r ast.Row) ast.Row {
	for i, cell := range r.Cells {
		p := cell.Blocks[0].(*ast.Plain)
		r.Cells[i].Blocks = []ast.Block{&ast.Plain{Inlines: p.Inlines[0].(*ast.Strong).Inlines}}
	}
	return r
}

// columns computes the column count (honouring spans) and sets a column's
// alignment when every body cell in it has the same explicit alignment.
func columns(t *ast.Table) []ast.ColSpec {
	width := 0
	type pos struct{ row, cell, col int }
	var body []pos
	for si, rows := range [][]ast.Row{t.Head, t.Body, t.Foot} {
		var carry []int
		for ri, r := range rows {
			col := 0
			for ci, cell := range r.Cells {
				for col < len(carry) && carry[col] > 0 {
					col++
				}
				if si == 1 && cell.ColSpan <= 1 {
					body = append(body, pos{ri, ci, col})
				}
				span := max(cell.ColSpan, 1)
				for k := col; k < col+span; k++ {
					for len(carry) <= k {
						carry = append(carry, 0)
					}
					carry[k] = max(cell.RowSpan, 1)
				}
				col += span
			}
			width = max(width, len(carry))
			for k := range carry {
				if carry[k] > 0 {
					carry[k]--
				}
			}
		}
	}
	cols := make([]ast.ColSpec, width)
	aligns := make([]ast.Align, width)
	mixed := make([]bool, width)
	seen := make([]bool, width)
	for _, p := range body {
		a := t.Body[p.row].Cells[p.cell].Align
		switch {
		case !seen[p.col]:
			aligns[p.col], seen[p.col] = a, true
		case aligns[p.col] != a:
			mixed[p.col] = true
		}
	}
	for j := range cols {
		if seen[j] && !mixed[j] && aligns[j] != ast.AlignDefault {
			cols[j].Align = aligns[j]
		}
	}
	for _, p := range body {
		cell := &t.Body[p.row].Cells[p.cell]
		if cols[p.col].Align != ast.AlignDefault && cell.Align == cols[p.col].Align {
			cell.Align = ast.AlignDefault
		}
	}
	return cols
}

// isLayoutTable reports whether t only positions content (a single cell,
// role=presentation, code line-number gutters, or headings and nested
// tables inside cells).
func (c *conv) isLayoutTable(t *html.Node, rows []rowNode) bool {
	switch strings.ToLower(attr(t, "role")) {
	case "presentation", "none":
		return true
	}
	for _, cl := range classes(t) {
		switch cl {
		case "lntable", "rouge-table", "highlighttable", "code-table":
			return true
		}
	}
	nCells, maxCols, blockCells := 0, 0, 0
	for _, r := range rows {
		cells := c.rowCells(r.tr)
		nCells += len(cells)
		maxCols = max(maxCols, len(cells))
		for _, td := range cells {
			if hasDescendantOf(td, 0, "h1", "h2", "h3", "h4", "h5", "h6", "table", "article", "main") {
				return true
			}
			if isGutterCell(td) {
				return true
			}
			if c.containsBlock(td) {
				blockCells++
			}
		}
	}
	if nCells == 1 {
		return true
	}
	return maxCols == 1 && blockCells == nCells && nCells > 0
}

func hasDescendantOf(n *html.Node, depth int, tags ...string) bool {
	if depth > maxDepth {
		return false
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, tags...) || hasDescendantOf(ch, depth+1, tags...) {
			return true
		}
	}
	return false
}

// isGutterCell reports whether td holds only the line numbers of a code
// listing.
func isGutterCell(td *html.Node) bool {
	for _, cl := range classes(td) {
		switch cl {
		case "gutter", "rouge-gutter", "linenos", "lntd-gutter", "code-gutter", "blob-num":
			return true
		}
	}
	if findElem(td, "pre", 0) == nil {
		return false
	}
	t := strings.TrimSpace(textContent(td))
	if t == "" {
		return false
	}
	for _, r := range t {
		if !(r >= '0' && r <= '9' || r == ' ' || r == '\n' || r == '\r' || r == '\t') {
			return false
		}
	}
	return true
}

func (c *conv) unwrapLayout(rows []rowNode, caption []ast.Inline) []ast.Block {
	var out []ast.Block
	for _, r := range rows {
		if c.ctx.Err() != nil {
			break
		}
		for _, td := range c.rowCells(r.tr) {
			if isGutterCell(td) {
				continue
			}
			out = append(out, c.blocks(td, false)...)
		}
	}
	if len(caption) > 0 {
		out = append([]ast.Block{&ast.Para{Inlines: caption}}, out...)
	}
	return out
}
