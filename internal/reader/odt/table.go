package odt

import (
	"strconv"

	"github.com/askrejans/crowdoc/v2/ast"
)

const (
	maxRepeat = 1000 // cap for number-rows/columns-repeated
	maxSpan   = 1000
)

type tableRow struct {
	head  bool
	cells []ast.Cell
	empty bool
}

func (r *reader) table(t *node) []ast.Block {
	var rows []tableRow
	truncated := false
	var addRows func(n *node, head bool, depth int)
	addRows = func(n *node, head bool, depth int) {
		if depth > 8 {
			return
		}
		for _, k := range n.kids {
			if r.ctx.Err() != nil || truncated {
				return
			}
			switch k.name {
			case "table:table-header-rows":
				addRows(k, true, depth+1)
			case "table:table-rows", "table:table-row-group":
				addRows(k, head, depth+1)
			case "table:table-row":
				row := r.tableRow(k)
				row.head = head
				repeat, err := strconv.Atoi(k.attr("table:number-rows-repeated"))
				if err != nil || repeat < 1 {
					repeat = 1
				}
				if row.empty {
					repeat = min(repeat, 1)
				}
				for i := 0; i < min(repeat, maxRepeat); i++ {
					if r.cells+len(row.cells) > r.limits.MaxTableCells {
						truncated = true
						return
					}
					r.cells += len(row.cells)
					rows = append(rows, row)
				}
			}
		}
	}
	addRows(t, false, 0)
	if truncated {
		r.warn.Addf("odt: table truncated at %d cells", r.limits.MaxTableCells)
	}
	// Spreadsheet-like tables end in repeated empty rows.
	for len(rows) > 0 && rows[len(rows)-1].empty {
		rows = rows[:len(rows)-1]
	}
	if len(rows) == 0 {
		return nil
	}
	trimEmptyColumns(rows)

	tbl := &ast.Table{}
	tbl.Attr.ID = r.targetID(t.attr("table:name"))
	for _, row := range rows {
		if row.head {
			tbl.Head = append(tbl.Head, ast.Row{Cells: row.cells})
		} else {
			tbl.Body = append(tbl.Body, ast.Row{Cells: row.cells})
		}
	}
	if len(tbl.Head) == 0 && len(tbl.Body) > 1 && allBold(tbl.Body[0]) {
		tbl.Head = []ast.Row{unbold(tbl.Body[0])}
		tbl.Body = tbl.Body[1:]
	}
	tbl.Cols = columns(tbl)
	return []ast.Block{tbl}
}

func (r *reader) tableRow(tr *node) tableRow {
	row := tableRow{empty: true}
	for _, k := range tr.kids {
		if k.name != "table:table-cell" {
			continue // covered cells are omitted
		}
		cell := r.tableCell(k)
		repeat, err := strconv.Atoi(k.attr("table:number-columns-repeated"))
		if err != nil || repeat < 1 {
			repeat = 1
		}
		if len(cell.Blocks) > 0 {
			row.empty = false
		}
		for i := 0; i < min(repeat, maxRepeat); i++ {
			row.cells = append(row.cells, cell)
		}
	}
	return row
}

func (r *reader) tableCell(td *node) ast.Cell {
	blocks := r.blocks(td)
	if len(blocks) == 1 {
		if p, ok := blocks[0].(*ast.Para); ok {
			blocks[0] = &ast.Plain{Inlines: p.Inlines}
		}
	}
	cell := ast.Cell{Blocks: blocks}
	if v, err := strconv.Atoi(td.attr("table:number-columns-spanned")); err == nil && v > 1 {
		cell.ColSpan = min(v, maxSpan)
	}
	if v, err := strconv.Atoi(td.attr("table:number-rows-spanned")); err == nil && v > 1 {
		cell.RowSpan = min(v, maxSpan)
	}
	// Alignment comes from the cell's paragraphs.
	for _, k := range td.kids {
		if k.name == "text:p" || k.name == "text:h" {
			cell.Align = r.styles.cellAlign(k.attr("text:style-name"))
			break
		}
	}
	return cell
}

// trimEmptyColumns drops trailing empty cells that only exist because of
// repeated column definitions.
func trimEmptyColumns(rows []tableRow) {
	last := 0
	for _, row := range rows {
		w := 0
		for _, c := range row.cells {
			span := max(c.ColSpan, 1)
			if len(c.Blocks) > 0 {
				last = max(last, w+span)
			}
			w += span
		}
	}
	for i := range rows {
		w := 0
		for j, c := range rows[i].cells {
			if w >= last {
				rows[i].cells = rows[i].cells[:j]
				break
			}
			w += max(c.ColSpan, 1)
		}
	}
}

func allBold(r ast.Row) bool {
	if len(r.Cells) == 0 {
		return false
	}
	nonEmpty := 0
	for _, cell := range r.Cells {
		if len(cell.Blocks) == 0 {
			continue
		}
		nonEmpty++
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
	return nonEmpty > 0
}

func unbold(r ast.Row) ast.Row {
	for i, cell := range r.Cells {
		if len(cell.Blocks) == 0 {
			continue
		}
		p := cell.Blocks[0].(*ast.Plain)
		r.Cells[i].Blocks = []ast.Block{&ast.Plain{Inlines: p.Inlines[0].(*ast.Strong).Inlines}}
	}
	return r
}

// columns sizes the table and sets a column alignment when every body cell
// in the column has the same explicit alignment.
func columns(t *ast.Table) []ast.ColSpec {
	width := 0
	type pos struct{ row, cell, col int }
	var body []pos
	for si, rows := range [][]ast.Row{t.Head, t.Body} {
		var carry []int
		for ri, row := range rows {
			col := 0
			for ci, cell := range row.Cells {
				for col < len(carry) && carry[col] > 0 {
					col++
				}
				if si == 1 && cell.ColSpan <= 1 && len(cell.Blocks) > 0 {
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
	for _, rows := range [][]ast.Row{t.Head, t.Body} {
		for i := range rows {
			for j := range rows[i].Cells {
				c := &rows[i].Cells[j]
				if c.Align == ast.AlignLeft {
					c.Align = ast.AlignDefault // the default for text
				}
			}
		}
	}
	for _, p := range body {
		c := &t.Body[p.row].Cells[p.cell]
		if cols[p.col].Align != ast.AlignDefault && c.Align == cols[p.col].Align {
			c.Align = ast.AlignDefault
		}
	}
	return cols
}
