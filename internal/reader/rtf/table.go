package rtf

import (
	"slices"

	"github.com/askrejans/crowdoc/v2/ast"
)

// cellDef is one \cellx column definition with its merge flags.
type cellDef struct {
	right          int
	hFirst, hMerge bool
	vFirst, vMerge bool
}

// rowDef is a \trowd row definition.
type rowDef struct {
	cells  []cellDef
	header bool
	left   int
}

func (p *parser) rowDef(level int) *rowDef {
	d := p.rowDefs[level]
	if d == nil {
		d = &rowDef{}
		p.rowDefs[level] = d
	}
	return d
}

// tableWord handles row and cell definition words.
func (p *parser) tableWord(t token, st *state) bool {
	level := 1
	switch {
	case st.nestProps:
		level = max(2, len(p.cur().tables))
	case st.para.itap >= 2:
		level = st.para.itap
	}
	switch t.name {
	case "trowd":
		*p.rowDef(level) = rowDef{}
		p.cellPending[level] = cellDef{}
	case "trhdr":
		p.rowDef(level).header = true
	case "trleft":
		p.rowDef(level).left = t.param
	case "clmgf":
		c := p.cellPending[level]
		c.hFirst = true
		p.cellPending[level] = c
	case "clmrg":
		c := p.cellPending[level]
		c.hMerge = true
		p.cellPending[level] = c
	case "clvmgf":
		c := p.cellPending[level]
		c.vFirst = true
		p.cellPending[level] = c
	case "clvmrg":
		c := p.cellPending[level]
		c.vMerge = true
		p.cellPending[level] = c
	case "cellx":
		d := p.rowDef(level)
		c := p.cellPending[level]
		c.right = t.param
		d.cells = append(d.cells, c)
		p.cellPending[level] = cellDef{}
	default:
		return false
	}
	return true
}

// tableBuilder collects the rows of one table.
type tableBuilder struct {
	rows  []rowData
	cells []cellData
	cur   *flow
}

type rowData struct {
	cells []cellData
	def   rowDef
}

type cellData struct {
	blocks []ast.Block
	align  ast.Align
	bold   bool // all text is bold
	empty  bool
}

func (t *tableBuilder) cell(p *parser) *flow {
	if t.cur == nil {
		t.cur = newFlow(p, false)
	}
	return t.cur
}

func (t *tableBuilder) endCell() {
	cd := cellData{empty: true}
	if f := t.cur; f != nil {
		cd.blocks = f.finish()
		cd.empty = len(cd.blocks) == 0
		if !f.alignMixed {
			cd.align = f.align
		}
		cd.bold = f.textLen > 0 && f.boldLen == f.textLen
	}
	t.cells = append(t.cells, cd)
	t.cur = nil
}

func (t *tableBuilder) endRow(def *rowDef) {
	if t.cur != nil {
		t.endCell()
	}
	if len(t.cells) == 0 {
		return
	}
	d := *def
	d.cells = slices.Clone(def.cells)
	t.rows = append(t.rows, rowData{cells: t.cells, def: d})
	t.cells = nil
}

// gridCell is a cell placed on the column grid.
type gridCell struct {
	data      cellData
	col, span int
	vFirst    bool
	vMerge    bool
}

// build converts the collected rows into a table. Columns come from the
// union of all \cellx boundaries, so cells that span several columns of
// other rows get a ColSpan.
func (t *tableBuilder) build() *ast.Table {
	if t.cur != nil || len(t.cells) > 0 {
		var last rowDef
		if len(t.rows) > 0 {
			last = t.rows[len(t.rows)-1].def
		}
		t.endRow(&last)
	}
	if len(t.rows) == 0 {
		return nil
	}
	rows := make([][]gridCell, len(t.rows))
	for i, r := range t.rows {
		rows[i] = placeRow(r)
	}
	bounds, ok := gridBounds(rows, t.rows)
	ncols := 0
	if ok {
		ncols = len(bounds)
		for i := range rows {
			for j := range rows[i] {
				c := &rows[i][j]
				lo := 0
				if j > 0 {
					lo = rows[i][j-1].col + rows[i][j-1].span
				}
				hi := slices.Index(bounds, snap(bounds, c.span))
				c.col = lo
				c.span = max(hi-lo+1, 1)
			}
		}
	} else {
		for i := range rows {
			for j := range rows[i] {
				rows[i][j].col, rows[i][j].span = j, 1
			}
			ncols = max(ncols, len(rows[i]))
		}
	}

	tbl := &ast.Table{Cols: make([]ast.ColSpec, ncols)}
	if ok {
		setWidths(tbl.Cols, bounds, t.rows)
	}
	active := make([]*ast.Cell, ncols)
	out := make([]ast.Row, 0, len(rows))
	for _, gr := range rows {
		var cells []ast.Cell
		var starts []int // grid column of each emitted cell that starts a vertical merge, else -1
		for _, gc := range gr {
			if gc.col >= ncols {
				continue
			}
			if gc.vMerge && active[gc.col] != nil {
				active[gc.col].RowSpan = max(active[gc.col].RowSpan, 1) + 1
				continue
			}
			c := ast.Cell{Blocks: cellBlocks(gc.data.blocks), Align: gc.data.align}
			if gc.span > 1 {
				c.ColSpan = gc.span
			}
			for k := gc.col; k < min(gc.col+gc.span, ncols); k++ {
				active[k] = nil
			}
			cells = append(cells, c)
			if gc.vFirst {
				starts = append(starts, gc.col)
			} else {
				starts = append(starts, -1)
			}
		}
		// Pointers are taken only once the row's cell slice is final.
		for i, col := range starts {
			if col >= 0 {
				active[col] = &cells[i]
			}
		}
		out = append(out, ast.Row{Cells: cells})
	}

	head := 0
	for head < len(t.rows) && t.rows[head].def.header {
		head++
	}
	if head == len(t.rows) {
		head = min(head, 1)
	}
	if head == 0 && len(t.rows) > 1 && allBold(t.rows[0].cells) {
		head = 1
	}
	if head > 0 {
		tbl.Head = out[:head]
	}
	tbl.Body = out[head:]
	columnAlign(tbl, rows[head:])
	return tbl
}

// placeRow pairs a row's cell contents with its definitions and applies
// old-style horizontal merges (\clmgf/\clmrg).
func placeRow(r rowData) []gridCell {
	n := max(len(r.cells), len(r.def.cells))
	var out []gridCell
	for i := 0; i < n; i++ {
		var gc gridCell
		if i < len(r.cells) {
			gc.data = r.cells[i]
		} else {
			gc.data = cellData{empty: true}
		}
		right := -1
		if i < len(r.def.cells) {
			d := r.def.cells[i]
			right = d.right
			gc.vFirst, gc.vMerge = d.vFirst, d.vMerge
			if d.hMerge && len(out) > 0 {
				prev := &out[len(out)-1]
				prev.span = right // extend the previous cell's right edge
				prev.data.blocks = append(prev.data.blocks, gc.data.blocks...)
				continue
			}
		}
		gc.span = right // temporarily the right boundary
		out = append(out, gc)
	}
	return out
}

// gridBounds returns the sorted column boundaries shared by all rows, or
// ok=false when the definitions are unusable (missing or not increasing).
func gridBounds(rows [][]gridCell, data []rowData) ([]int, bool) {
	var all []int
	for i, r := range rows {
		prev := data[i].def.left
		for _, c := range r {
			if c.span < 0 || c.span <= prev {
				return nil, false
			}
			prev = c.span
			all = append(all, c.span)
		}
	}
	slices.Sort(all)
	var bounds []int
	for _, b := range all {
		// Boundaries within ~1 mm are the same column edge.
		if len(bounds) > 0 && b-bounds[len(bounds)-1] < 60 {
			continue
		}
		bounds = append(bounds, b)
	}
	if len(bounds) == 0 || len(bounds) > 64 {
		return nil, false
	}
	return bounds, true
}

// snap returns the grid boundary that x belongs to.
func snap(bounds []int, x int) int {
	best := bounds[0]
	for _, b := range bounds {
		if b <= x {
			best = b
		}
	}
	return best
}

func setWidths(cols []ast.ColSpec, bounds []int, data []rowData) {
	left := bounds[0]
	for _, r := range data {
		left = min(left, r.def.left)
	}
	total := float64(bounds[len(bounds)-1] - left)
	if total <= 0 {
		return
	}
	prev := left
	for i, b := range bounds {
		cols[i].Width = float64(b-prev) / total
		prev = b
	}
}

func cellBlocks(blocks []ast.Block) []ast.Block {
	if len(blocks) == 1 {
		if p, ok := blocks[0].(*ast.Para); ok {
			return []ast.Block{&ast.Plain{Inlines: p.Inlines}}
		}
	}
	return blocks
}

func allBold(cells []cellData) bool {
	seen := false
	for _, c := range cells {
		if c.empty {
			continue
		}
		if !c.bold {
			return false
		}
		seen = true
	}
	return seen
}

// columnAlign sets a column's alignment when every non-empty body cell in
// it shares the same explicit alignment.
func columnAlign(tbl *ast.Table, rows [][]gridCell) {
	type colState struct {
		align        ast.Align
		set, uniform bool
	}
	st := make([]colState, len(tbl.Cols))
	for _, r := range rows {
		for _, c := range r {
			if c.col >= len(st) || c.span != 1 || c.data.empty || c.vMerge {
				continue
			}
			cs := &st[c.col]
			switch {
			case !cs.set:
				cs.align, cs.set, cs.uniform = c.data.align, true, true
			case cs.align != c.data.align:
				cs.uniform = false
			}
		}
	}
	for i, cs := range st {
		if cs.set && cs.uniform && cs.align != ast.AlignDefault {
			tbl.Cols[i].Align = cs.align
		}
	}
}
