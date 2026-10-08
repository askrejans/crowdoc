package docx

import (
	"strconv"

	"github.com/askrejans/crowdoc/v2/ast"
)

// tcell is a table cell before row spans are resolved.
type tcell struct {
	col, span int
	vmerge    string // "", "restart" or "continue"
	cell      ast.Cell
	bold      bool // every character is bold
	empty     bool
}

type trow struct {
	cells  []*tcell
	header bool
}

// table converts a w:tbl.
func (r *reader) table(s *story, tbl *node, bc blockCtx) (item, bool) {
	tblPr := tbl.child("tblPr")
	var widths []float64
	total := 0.0
	for _, gc := range tbl.child("tblGrid").childrenNamed("gridCol") {
		w, _ := strconv.ParseFloat(gc.attr("w"), 64)
		widths = append(widths, w)
		total += w
	}

	var rows []*trow
	for _, tr := range tableRows(tbl) {
		if r.err != nil {
			return item{}, false
		}
		if err := r.ctx.Err(); err != nil {
			r.err = err
			return item{}, false
		}
		if row := r.tableRow(s, tr); row != nil {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return item{}, false
	}

	ncols := len(widths)
	for _, row := range rows {
		if n := len(row.cells); n > 0 {
			last := row.cells[n-1]
			if end := last.col + last.span; end > ncols {
				ncols = end
			}
		}
	}
	t := &ast.Table{Cols: make([]ast.ColSpec, ncols)}
	if len(widths) == ncols && total > 0 {
		for i, w := range widths {
			if w <= 0 {
				for k := range t.Cols {
					t.Cols[k].Width = 0
				}
				break
			}
			t.Cols[i].Width = w / total
		}
	}

	resolveRowSpans(rows)
	nhead := 0
	for nhead < len(rows) && rows[nhead].header {
		nhead++
	}
	styleHead := false
	if nhead == 0 && len(rows) > 1 {
		if r.tableStyleBoldFirstRow(tblPr) || rowAllBold(rows[0]) {
			nhead = 1
			styleHead = true
		}
	}
	if nhead >= len(rows) {
		nhead = 0
	}
	columnAlignment(t, rows[nhead:])
	for i, row := range rows {
		out := ast.Row{}
		for _, c := range row.cells {
			if c.vmerge == "continue" {
				continue
			}
			if i < nhead {
				for k, b := range c.cell.Blocks {
					if p, ok := b.(*ast.Plain); ok && (styleHead || c.bold) {
						c.cell.Blocks[k] = &ast.Plain{Inlines: unwrapStrong(p.Inlines)}
					}
				}
			}
			out.Cells = append(out.Cells, c.cell)
		}
		if i < nhead {
			t.Head = append(t.Head, out)
		} else {
			t.Body = append(t.Body, out)
		}
	}
	return item{kind: kBlock, block: t, table: t}, true
}

// tableRows lists the w:tr elements of a table, looking through content
// controls and revision marks and skipping deleted rows.
func tableRows(tbl *node) []*node {
	var out []*node
	var visit func(n *node)
	visit = func(n *node) {
		for _, k := range n.kids {
			switch k.name {
			case "tr":
				if k.child("trPr").child("del") == nil {
					out = append(out, k)
				}
			case "sdt":
				visit(k.child("sdtContent"))
			case "customXml", "ins", "moveTo":
				visit(k)
			case "AlternateContent":
				if alt := chooseAlternate(k); alt != nil {
					visit(alt)
				}
			}
		}
	}
	visit(tbl)
	return out
}

func tableCells(tr *node) []*node {
	var out []*node
	var visit func(n *node)
	visit = func(n *node) {
		for _, k := range n.kids {
			switch k.name {
			case "tc":
				out = append(out, k)
			case "sdt":
				visit(k.child("sdtContent"))
			case "customXml", "ins", "moveTo":
				visit(k)
			}
		}
	}
	visit(tr)
	return out
}

func (r *reader) tableRow(s *story, tr *node) *trow {
	trPr := tr.child("trPr")
	row := &trow{header: onOff(trPr.child("tblHeader"))}
	col := 0
	if v, err := strconv.Atoi(trPr.val("gridBefore")); err == nil && v > 0 {
		row.cells = append(row.cells, &tcell{col: 0, span: v, cell: ast.Cell{Blocks: []ast.Block{&ast.Plain{}}, ColSpan: v}, empty: true})
		col = v
	}
	for _, tc := range tableCells(tr) {
		tcPr := tc.child("tcPr")
		span := 1
		if v, err := strconv.Atoi(tcPr.val("gridSpan")); err == nil && v > 1 {
			span = v
		}
		c := &tcell{col: col, span: span}
		if vm := tcPr.child("vMerge"); vm != nil {
			c.vmerge = "continue"
			if vm.attr("val") == "restart" {
				c.vmerge = "restart"
			}
		}
		if hm := tcPr.child("hMerge"); hm != nil && hm.attr("val") != "restart" && len(row.cells) > 0 {
			// Legacy horizontal merge: fold into the previous cell.
			prev := row.cells[len(row.cells)-1]
			prev.span += span
			prev.cell.ColSpan = prev.span
			col += span
			continue
		}
		blocks, info := r.cellBlocks(s, tc)
		c.cell = ast.Cell{Blocks: blocks, Align: info.align}
		if span > 1 {
			c.cell.ColSpan = span
		}
		c.bold, c.empty = info.allBold, info.empty
		row.cells = append(row.cells, c)
		col += span
	}
	if len(row.cells) == 0 {
		return nil
	}
	return row
}

type cellInfo struct {
	align   ast.Align
	allBold bool
	empty   bool
}

// cellBlocks converts the content of a w:tc.
func (r *reader) cellBlocks(s *story, tc *node) ([]ast.Block, cellInfo) {
	bc := blockCtx{inCell: true}
	items := r.blockItems(s, tc.kids, bc)
	info := cellInfo{allBold: true, empty: true}
	aligns := map[ast.Align]bool{}
	for _, it := range items {
		if it.p == nil {
			info.allBold, info.empty = false, false
			continue
		}
		if it.kind != kBlock && len(it.ins) > 0 || it.code {
			info.empty = false
			if !it.allBold {
				info.allBold = false
			}
		}
		aligns[jcAlign(it.p.jc)] = true
	}
	if len(aligns) == 1 {
		for a := range aligns {
			info.align = a
		}
	}
	blocks := r.assemble(items, bc)
	if len(blocks) == 0 {
		blocks = []ast.Block{&ast.Plain{}}
	}
	if len(blocks) > 1 {
		for i, b := range blocks {
			if p, ok := b.(*ast.Plain); ok {
				blocks[i] = &ast.Para{Inlines: p.Inlines}
			}
		}
	}
	if info.empty {
		info.allBold = false
	}
	return blocks, info
}

func jcAlign(jc string) ast.Align {
	switch jc {
	case "left", "start":
		return ast.AlignLeft
	case "center":
		return ast.AlignCenter
	case "right", "end":
		return ast.AlignRight
	}
	return ast.AlignDefault
}

// resolveRowSpans turns vMerge restart/continue chains into RowSpan.
func resolveRowSpans(rows []*trow) {
	for i, row := range rows {
		for _, c := range row.cells {
			if c.vmerge != "restart" {
				continue
			}
			span := 1
			for k := i + 1; k < len(rows); k++ {
				cont := cellAt(rows[k], c.col)
				if cont == nil || cont.vmerge != "continue" {
					break
				}
				span++
			}
			if span > 1 {
				c.cell.RowSpan = span
			}
		}
	}
	// A continuation with nothing above it is a producer error; keep its
	// content as an ordinary cell.
	for i, row := range rows {
		for _, c := range row.cells {
			if c.vmerge != "continue" {
				continue
			}
			if i == 0 {
				c.vmerge = ""
				continue
			}
			above := cellAt(rows[i-1], c.col)
			if above == nil || above.vmerge == "" {
				c.vmerge = ""
			}
		}
	}
}

func cellAt(row *trow, col int) *tcell {
	for _, c := range row.cells {
		if c.col == col {
			return c
		}
	}
	return nil
}

func rowAllBold(row *trow) bool {
	nonEmpty := 0
	for _, c := range row.cells {
		if c.empty {
			continue
		}
		if !c.bold {
			return false
		}
		nonEmpty++
	}
	return nonEmpty > 0
}

// tableStyleBoldFirstRow reports whether the table style formats the first
// row in bold and the table enables first-row formatting.
func (r *reader) tableStyleBoldFirstRow(tblPr *node) bool {
	id := tblPr.val("tblStyle")
	if id == "" {
		return false
	}
	look := tblPr.child("tblLook")
	if look == nil {
		return false
	}
	enabled := truthy(look.attr("firstRow"))
	if !look.hasAttr("firstRow") {
		if v, err := strconv.ParseUint(look.attr("val"), 16, 16); err == nil {
			enabled = v&0x0020 != 0
		}
	}
	if !enabled {
		return false
	}
	for _, s := range r.styles.chain(id) {
		if s.firstRowBold {
			return true
		}
	}
	return false
}

// columnAlignment hoists an alignment shared by every body cell of a
// column into the column spec.
func columnAlignment(t *ast.Table, body []*trow) {
	for col := range t.Cols {
		var align ast.Align
		consistent, seen := true, false
		for _, row := range body {
			c := cellAt(row, col)
			if c == nil || c.span != 1 || c.vmerge == "continue" || c.empty {
				continue
			}
			if !seen {
				align, seen = c.cell.Align, true
			} else if c.cell.Align != align {
				consistent = false
				break
			}
		}
		if !seen || !consistent || align == ast.AlignDefault {
			continue
		}
		t.Cols[col].Align = align
		for _, row := range body {
			if c := cellAt(row, col); c != nil && c.span == 1 {
				c.cell.Align = ast.AlignDefault
			}
		}
	}
}
