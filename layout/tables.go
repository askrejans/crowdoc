package layout

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/askrejans/crowdoc/v2/ast"
)

// tableRegion is a run of flow items [start, end) laid out as a grid.
type tableRegion struct {
	start, end int
	seps       []float64 // x positions separating the columns
}

// findTables looks for runs of at least three lines whose words fall into
// two or more consistent columns: the x-projection of all words in the run
// leaves vertical gaps.
func (d *doc) findTables(f *flow) map[int]tableRegion {
	out := map[int]tableRegion{}
	items := f.items
	for i := 0; i < len(items); {
		l := items[i].l
		if l == nil || !d.tableCandidate(l, f) || !rowLike(l) && !tableRole(l) {
			i++
			continue
		}
		j := d.growTable(items, i, f)
		var lines []*line
		for k := i; k < j; k++ {
			lines = append(lines, items[k].l)
		}
		minGap := 0.6 * l.size
		// A header row with wide centred labels may hide gaps that the
		// body rows show.
		seps := projectionSeps(lines, minGap)
		if len(lines) >= 3 {
			if body := projectionSeps(lines[1:], minGap); len(body) > len(seps) {
				seps = body
			}
		}
		rows := j - i
		if len(seps) > 0 && (rows >= 3 || rows >= 2 && tableRole(l)) && d.validTable(items[i:j], seps) {
			out[i] = tableRegion{start: i, end: j, seps: seps}
			i = j
			continue
		}
		i++
	}
	return out
}

// growTable extends a table starting at item i while the following lines
// keep its column structure, size and row spacing.
func (d *doc) growTable(items []flowItem, i int, f *flow) int {
	first := items[i].l
	minGap := 0.6 * first.size
	tagged := tableRole(first)
	lines := []*line{first}
	var pitches []float64
	j := i + 1
	for ; j < len(items); j++ {
		n := items[j].l
		if n == nil || !d.tableCandidate(n, f) {
			break
		}
		if tagged {
			// The producer marked the cells: follow the tags.
			if !tableRole(n) {
				break
			}
			lines = append(lines, n)
			continue
		}
		prev := items[j-1].l
		pitch := n.base - prev.base
		if math.Abs(n.size-first.size) > 0.15*first.size || pitch <= 0 || n.y0-prev.y1 > 2*max(d.body, n.size) {
			break
		}
		if len(pitches) >= 2 && pitch > 1.9*median(pitches) {
			break
		}
		if d.isHeadingLine(n) && !rowLike(n) {
			break
		}
		// The new line must not cover the gaps between the columns found
		// so far (a paragraph line below the table does).
		body := lines
		if len(lines) >= 2 {
			body = lines[1:]
		}
		before := projectionSeps(body, minGap)
		after := projectionSeps(append(append([]*line{}, body...), n), minGap)
		if len(after) == 0 || len(lines) >= 2 && len(after) < len(before) {
			break
		}
		lines = append(lines, n)
		pitches = append(pitches, pitch)
	}
	return j
}

// tableRole reports whether the producer tagged the line as table content.
func tableRole(l *line) bool {
	r := l.role()
	return r == RoleTableCell || r == RoleTableHeader
}

// tableCandidate excludes lines that belong to other structures.
func (d *doc) tableCandidate(l *line, f *flow) bool {
	if tableRole(l) {
		return true
	}
	if r := l.role(); r != RoleAuto {
		return false
	}
	if d.isCode(l) || l.size > d.body*1.3 {
		return false
	}
	if _, ok := d.rawMarker(l, f); ok {
		return false
	}
	return true
}

// rowLike reports whether a line has a gap wide enough to separate cells.
func rowLike(l *line) bool {
	for k := 1; k < len(l.words); k++ {
		if l.words[k].x0-l.words[k-1].x1 >= 1.0*l.size {
			return true
		}
	}
	return false
}

// projectionSeps returns the column separators of a set of lines: centres
// of the vertical gaps in the union of their word extents.
func projectionSeps(lines []*line, minGap float64) []float64 {
	var iv [][2]float64
	for _, l := range lines {
		for _, w := range l.words {
			iv = append(iv, [2]float64{w.x0, w.x1})
		}
	}
	if len(iv) == 0 {
		return nil
	}
	sort.Slice(iv, func(i, j int) bool { return iv[i][0] < iv[j][0] })
	var seps []float64
	end := iv[0][1]
	for _, x := range iv[1:] {
		if x[0]-end >= minGap {
			seps = append(seps, (x[0]+end)/2)
		}
		end = max(end, x[1])
	}
	return seps
}

// validTable checks that most rows populate at least two columns.
func (d *doc) validTable(items []flowItem, seps []float64) bool {
	multi := 0
	for _, it := range items {
		cols := map[int]bool{}
		for _, w := range it.l.words {
			cols[colIndex(seps, w.cx())] = true
		}
		if len(cols) >= 2 {
			multi++
		}
	}
	return multi*10 >= len(items)*6
}

func colIndex(seps []float64, x float64) int {
	c := 0
	for c < len(seps) && x > seps[c] {
		c++
	}
	return c
}

// tableCell collects the lines of one cell.
type tableCell struct{ lines []*line }

// buildTable converts a table region into an ast.Table.
func (d *doc) buildTable(items []flowItem, seps []float64) *ast.Table {
	ncol := len(seps) + 1
	var rows [][]tableCell
	var rowLines [][]*line
	rowPitch := d.rowPitch(items)
	var prev *line
	var prevBlocks map[int]bool
	for _, it := range items {
		l := it.l
		parts := make([][]*word, ncol)
		blocks := map[int]bool{}
		for _, w := range l.words {
			c := colIndex(seps, w.cx())
			parts[c] = append(parts[c], w)
			if w.block != 0 {
				blocks[w.block] = true
			}
		}
		if prev == nil || !continuesRow(l, prev, parts, blocks, prevBlocks, rowPitch) {
			rows = append(rows, make([]tableCell, ncol))
			rowLines = append(rowLines, nil)
		}
		prev, prevBlocks = l, blocks
		r := rows[len(rows)-1]
		for c, ws := range parts {
			if len(ws) > 0 {
				r[c].lines = append(r[c].lines, l.subLine(ws))
			}
		}
		rowLines[len(rowLines)-1] = append(rowLines[len(rowLines)-1], l)
	}
	t := &ast.Table{Cols: make([]ast.ColSpec, ncol)}
	head := headerRows(rows, rowLines)
	for ri, r := range rows {
		row := ast.Row{}
		for _, c := range r {
			var blocks []ast.Block
			if len(c.lines) > 0 {
				opts := inlineOpts{noBold: ri < head}
				if ins := d.inlines(c.lines, opts); len(ins) > 0 {
					blocks = []ast.Block{&ast.Plain{Inlines: ins}}
				}
			}
			row.Cells = append(row.Cells, ast.Cell{Blocks: blocks})
		}
		if ri < head {
			t.Head = append(t.Head, row)
		} else {
			t.Body = append(t.Body, row)
		}
	}
	for c := 0; c < ncol; c++ {
		t.Cols[c].Align = columnAlign(rows[head:], c, func(r []tableCell, c int) []*line { return r[c].lines })
	}
	return t
}

// rowPitch is the usual baseline distance between consecutive lines of a
// table region.
func (d *doc) rowPitch(items []flowItem) float64 {
	var v []float64
	for k := 1; k < len(items); k++ {
		if p := items[k].l.base - items[k-1].l.base; p > 0 {
			v = append(v, p)
		}
	}
	return median(v)
}

// continuesRow reports whether table line l continues the row of the
// previous line (a cell wrapped onto a second line) instead of starting a
// new row.
func continuesRow(l, prev *line, parts [][]*word, blocks, prevBlocks map[int]bool, rowPitch float64) bool {
	if len(blocks) > 0 && len(prevBlocks) > 0 {
		for b := range blocks {
			if !prevBlocks[b] {
				return false
			}
		}
		return true
	}
	if len(parts[0]) > 0 {
		return false
	}
	gap := l.base - prev.base
	if rowPitch > 1.35*l.size {
		// Rows are spaced apart: a tight line belongs to the row above.
		return gap < 0.8*rowPitch
	}
	return gap <= 1.3*l.size
}

// headerRows returns the number of header rows: rows tagged as headers,
// or a first row that is bold over a non-bold body, or a first row of
// labels over numeric columns.
func headerRows(rows [][]tableCell, rowLines [][]*line) int {
	if len(rows) < 2 {
		return 0
	}
	tagged := 0
	for _, rl := range rowLines {
		if rl[0].role() != RoleTableHeader {
			break
		}
		tagged++
	}
	if tagged > 0 {
		return min(tagged, len(rows)-1)
	}
	if rowLines[0][0].role() == RoleTableCell {
		return 0
	}
	firstBold, restBold := rowBold(rowLines[0]), 0.0
	for _, rl := range rowLines[1:] {
		restBold += rowBold(rl)
	}
	restBold /= float64(len(rowLines) - 1)
	if firstBold >= 0.8 && restBold < 0.5 {
		return 1
	}
	if len(rows) < 3 {
		return 0
	}
	numericCols := 0
	for c := range rows[0] {
		numeric, filled := 0, 0
		for _, r := range rows[1:] {
			if text := cellText(r[c]); text != "" {
				filled++
				if isNumeric(text) {
					numeric++
				}
			}
		}
		if filled >= 2 && numeric*10 >= filled*7 {
			numericCols++
			if t := cellText(rows[0][c]); t == "" || isNumeric(t) {
				return 0
			}
		}
	}
	if numericCols == 0 {
		return 0
	}
	return 1
}

func cellText(c tableCell) string {
	var parts []string
	for _, l := range c.lines {
		parts = append(parts, l.text())
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// isNumeric reports whether a cell holds a number, amount or percentage
// ("1 200", "−3.5 %", "€12,00", "3–7").
func isNumeric(s string) bool {
	digits, other := 0, 0
	for _, r := range s {
		switch {
		case unicode.IsDigit(r):
			digits++
		case unicode.IsSpace(r) || strings.ContainsRune(".,%+-−–—/:()€$£¥×x", r):
		default:
			other++
		}
	}
	return digits > 0 && other <= 1
}

func rowBold(ls []*line) float64 {
	n, b := 0, 0.0
	for _, l := range ls {
		bf, _, _ := l.styleFrac()
		c := l.chars()
		n += c
		b += bf * float64(c)
	}
	if n == 0 {
		return 0
	}
	return b / float64(n)
}

// columnAlign infers the alignment of column c from its body cells: equal
// right edges mean right alignment, equal centres centring.
func columnAlign[R any](rows []R, c int, get func(R, int) []*line) ast.Align {
	var lefts, rights, centres []float64
	for _, r := range rows {
		for _, l := range get(r, c) {
			lefts = append(lefts, l.x0)
			rights = append(rights, l.x1)
			centres = append(centres, (l.x0+l.x1)/2)
		}
	}
	if len(lefts) < 2 {
		return ast.AlignDefault
	}
	spread := func(v []float64) float64 {
		lo, hi := v[0], v[0]
		for _, x := range v {
			lo, hi = math.Min(lo, x), math.Max(hi, x)
		}
		return hi - lo
	}
	switch {
	case spread(lefts) <= 1.5:
		return ast.AlignDefault
	case spread(rights) <= 1.5:
		return ast.AlignRight
	case spread(centres) <= 1.5:
		return ast.AlignCenter
	}
	return ast.AlignDefault
}
