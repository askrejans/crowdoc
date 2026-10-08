package typst

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

type placedCell struct {
	cell     ast.Cell
	row, col int
}

// gridLayout places cells on a grid, honouring row and column spans, and
// returns the column count.
func gridLayout(rows []ast.Row) ([][]placedCell, int) {
	occupied := map[[2]int]bool{}
	placed := make([][]placedCell, len(rows))
	cols := 0
	for r, row := range rows {
		c := 0
		for _, cell := range row.Cells {
			for occupied[[2]int{r, c}] {
				c++
			}
			cs, rs := max(cell.ColSpan, 1), max(cell.RowSpan, 1)
			if r+rs > len(rows) {
				rs = len(rows) - r
				cell.RowSpan = rs
			}
			for dr := 0; dr < rs; dr++ {
				for dc := 0; dc < cs; dc++ {
					occupied[[2]int{r + dr, c + dc}] = true
				}
			}
			placed[r] = append(placed[r], placedCell{cell: cell, row: r, col: c})
			c += cs
		}
		if c > cols {
			cols = c
		}
	}
	// Account for row spans reaching past the last placed cell of a row.
	for k := range occupied {
		if k[1]+1 > cols {
			cols = k[1] + 1
		}
	}
	return placed, cols
}

func (w *writer) table(t *ast.Table) string {
	all := make([]ast.Row, 0, len(t.Head)+len(t.Body)+len(t.Foot))
	all = append(append(append(all, t.Head...), t.Body...), t.Foot...)
	if len(all) == 0 {
		return ""
	}
	placed, ncols := gridLayout(all)
	if ncols == 0 {
		return ""
	}

	stats := columnStats(placed, ncols, len(t.Head))
	colSpec := make([]string, ncols)
	aligns := make([]string, ncols)
	explicit := len(t.Cols) == ncols
	sumWidth := 0.0
	for _, c := range t.Cols {
		sumWidth += c.Width
	}
	totalMax := 0
	for _, s := range stats {
		totalMax += s.maxLen
	}
	for i := 0; i < ncols; i++ {
		s := stats[i]
		switch {
		case explicit && sumWidth > 0.5 && t.Cols[i].Width > 0:
			colSpec[i] = ftoa(t.Cols[i].Width*100) + "fr"
		case totalMax <= 70 || s.maxLen <= 18:
			colSpec[i] = "auto"
		default:
			weight := float64(min(s.avgLen, 60)) + 4
			colSpec[i] = ftoa(weight) + "fr"
		}
		a := ast.AlignDefault
		if explicit {
			a = t.Cols[i].Align
		}
		if a == ast.AlignDefault && s.numeric {
			a = ast.AlignRight
		}
		aligns[i] = alignName(a)
	}

	var cells []string
	headRows := len(t.Head)
	bodyEnd := len(all) - len(t.Foot)
	rowCells := func(from, to int) []string {
		var out []string
		for r := from; r < to; r++ {
			for _, pc := range placed[r] {
				out = append(out, w.cell(pc))
			}
		}
		return out
	}
	if headRows > 0 {
		cells = append(cells, "table.header("+strings.Join(rowCells(0, headRows), ", ")+")")
	}
	cells = append(cells, rowCells(headRows, bodyEnd)...)
	if len(t.Foot) > 0 {
		cells = append(cells, "table.footer(repeat: false, "+strings.Join(rowCells(bodyEnd, len(all)), ", ")+")")
	}

	var sb strings.Builder
	sb.WriteString("table(\n  columns: (")
	sb.WriteString(strings.Join(colSpec, ", "))
	if ncols == 1 {
		sb.WriteString(",")
	}
	sb.WriteString("),\n  align: (")
	sb.WriteString(strings.Join(aligns, ", "))
	if ncols == 1 {
		sb.WriteString(",")
	}
	sb.WriteString("),\n  ")
	sb.WriteString(strings.Join(cells, ",\n  "))
	sb.WriteString(",\n)")
	tbl := sb.String()

	// Many columns: shrink the type so the table still fits.
	if ncols >= 9 {
		size := "0.82em"
		if ncols >= 13 {
			size = "0.7em"
		}
		tbl = "text(size: " + size + ", " + tbl + ")"
	}
	if t.Caption != nil || t.Attr.ID != "" {
		cap := "none"
		if t.Caption != nil {
			cap = content(w.inlinesAtStart(t.Caption))
		}
		return "#figure(" + tbl + ", caption: " + cap + ", kind: table)" + labelSuffix(t.Attr.ID)
	}
	return "#align(center, " + tbl + ")"
}

func alignName(a ast.Align) string {
	switch a {
	case ast.AlignCenter:
		return "center"
	case ast.AlignRight:
		return "right"
	}
	return "left"
}

func (w *writer) cell(pc placedCell) string {
	body := w.cellContent(pc.cell.Blocks)
	var args []string
	if pc.cell.ColSpan > 1 {
		args = append(args, "colspan: "+strconv.Itoa(pc.cell.ColSpan))
	}
	if pc.cell.RowSpan > 1 {
		args = append(args, "rowspan: "+strconv.Itoa(pc.cell.RowSpan))
	}
	if pc.cell.Align != ast.AlignDefault {
		args = append(args, "align: "+alignName(pc.cell.Align))
	}
	if len(args) == 0 {
		return content(body)
	}
	return "table.cell(" + strings.Join(args, ", ") + ")" + content(body)
}

func (w *writer) cellContent(blocks []ast.Block) string {
	if len(blocks) == 1 {
		switch b := blocks[0].(type) {
		case *ast.Plain:
			return w.inlinesAtStart(b.Inlines)
		case *ast.Para:
			return w.inlinesAtStart(b.Inlines)
		}
	}
	return w.blocks(blocks)
}

type colStat struct {
	maxLen, avgLen int
	numeric        bool
}

func columnStats(placed [][]placedCell, ncols, headRows int) []colStat {
	stats := make([]colStat, ncols)
	sums := make([]int, ncols)
	counts := make([]int, ncols)
	nums := make([]int, ncols)
	nonEmpty := make([]int, ncols)
	for r, row := range placed {
		for _, pc := range row {
			if pc.cell.ColSpan > 1 {
				continue
			}
			text := strings.TrimSpace(ast.BlocksText(pc.cell.Blocks))
			l := longestLine(text)
			i := pc.col
			if l > stats[i].maxLen {
				stats[i].maxLen = l
			}
			sums[i] += l
			counts[i]++
			if r >= headRows && text != "" {
				nonEmpty[i]++
				if looksNumeric(text) {
					nums[i]++
				}
			}
		}
	}
	for i := range stats {
		if counts[i] > 0 {
			stats[i].avgLen = sums[i] / counts[i]
		}
		stats[i].numeric = nonEmpty[i] > 0 && nums[i]*10 >= nonEmpty[i]*8
	}
	return stats
}

func longestLine(s string) int {
	max := 0
	for _, line := range strings.Split(s, "\n") {
		if n := utf8.RuneCountInString(line); n > max {
			max = n
		}
	}
	return max
}

// looksNumeric accepts numbers with sign, grouping, decimal comma/point,
// currency symbols, percent and accounting parentheses.
func looksNumeric(s string) bool {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "()")
	s = strings.TrimSpace(strings.Trim(s, "€$£¥₹%+-−–"))
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(s, "EUR"), "USD"))
	if s == "" {
		return false
	}
	digits := 0
	for _, r := range s {
		switch {
		case unicode.IsDigit(r):
			digits++
		case r == '.' || r == ',' || r == ' ' || r == ' ' || r == ' ' || r == '\'':
		default:
			return false
		}
	}
	return digits > 0
}
