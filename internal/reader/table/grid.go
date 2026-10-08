// Package table reads tabular formats — CSV, TSV, XLSX and ODS — into the
// crowdoc AST. Spreadsheets become one section per sheet holding tables,
// chart data tables and images; text-heavy sheets become paragraphs.
package table

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// cellKind is the value type of a cell, used for alignment.
type cellKind uint8

const (
	kText cellKind = iota
	kNumber
	kDate
	kBool
	kError
)

// sheetCell is one non-empty (or merge-anchoring) cell of a sheet.
type sheetCell struct {
	row, col         int // 0-based
	text             string
	kind             cellKind
	align            ast.Align
	link             string
	rowSpan, colSpan int
}

// sheet is the reader-independent content of one worksheet.
type sheet struct {
	name       string
	cells      []sheetCell
	headerRows int         // rows the source marks as header (0 = first row)
	extras     []ast.Block // charts and images, placed after the content
}

// gridCell is a cell of the dense grid built from a sheet.
type gridCell struct {
	text             string
	link             string
	kind             cellKind
	align            ast.Align
	covered          bool  // inside another cell's span
	anchor           int32 // grid row of the covering cell
	rowSpan, colSpan int32
}

type grid struct {
	cells   [][]gridCell
	origRow []int // source row number of each grid row (gap detection)
}

// buildGrid compacts a sheet into a dense grid: rows and columns without
// any content are dropped (spacer columns, stray formatting), merged cells
// keep their span over the remaining rows and columns.
func buildGrid(cells []sheetCell, maxCells int) (*grid, bool) {
	var rows, cols []int
	for _, c := range cells {
		if c.text != "" {
			rows = append(rows, c.row)
			cols = append(cols, c.col)
		}
	}
	rows, cols = sortedUnique(rows), sortedUnique(cols)
	if len(rows) == 0 || len(cols) == 0 {
		return nil, false
	}
	truncated := false
	if maxCells > 0 && len(rows)*len(cols) > maxCells {
		keep := max(maxCells/len(cols), 1)
		rows = rows[:min(keep, len(rows))]
		truncated = true
	}
	g := &grid{cells: make([][]gridCell, len(rows)), origRow: rows}
	for i := range g.cells {
		g.cells[i] = make([]gridCell, len(cols))
	}
	for _, c := range cells {
		ri, ok1 := slices.BinarySearch(rows, c.row)
		ci, ok2 := slices.BinarySearch(cols, c.col)
		if !ok1 || !ok2 {
			continue
		}
		gc := &g.cells[ri][ci]
		if gc.covered || gc.text != "" {
			continue
		}
		gc.text, gc.kind, gc.align, gc.link = c.text, c.kind, c.align, c.link
		rs, cs := int32(1), int32(1)
		if c.rowSpan > 1 {
			end, _ := slices.BinarySearch(rows, c.row+c.rowSpan)
			rs = int32(max(end-ri, 1))
		}
		if c.colSpan > 1 {
			end, _ := slices.BinarySearch(cols, c.col+c.colSpan)
			cs = int32(max(end-ci, 1))
		}
		gc.rowSpan, gc.colSpan = rs, cs
		for r := ri; r < ri+int(rs); r++ {
			for k := ci; k < ci+int(cs); k++ {
				if r == ri && k == ci {
					continue
				}
				cov := &g.cells[r][k]
				*cov = gridCell{covered: true, anchor: int32(ri)}
			}
		}
	}
	return g, truncated
}

func sortedUnique(xs []int) []int {
	slices.Sort(xs)
	return slices.Compact(xs)
}

// rowInfo summarises one grid row for layout decisions.
type rowInfo struct {
	filled int    // non-empty anchor cells
	text   string // the text when filled == 1
	wide   bool   // the single cell spans at least half the columns
}

func (g *grid) info(i int) rowInfo {
	var ri rowInfo
	for _, c := range g.cells[i] {
		if c.covered || c.text == "" {
			continue
		}
		ri.filled++
		ri.text = c.text
		ri.wide = int(c.colSpan)*2 >= len(g.cells[i]) && len(g.cells[i]) > 2
	}
	return ri
}

const longText = 80

// sheetBlocks lays out a sheet: text sheets become paragraphs, otherwise
// the content becomes tables with title and note rows as paragraphs.
// level is the heading level for headings derived from the sheet content.
func sheetBlocks(s *sheet, g *grid, level int) []ast.Block {
	n := len(g.cells)
	infos := make([]rowInfo, n)
	for i := range infos {
		infos[i] = g.info(i)
	}
	if isTextSheet(g, infos) {
		var out []ast.Block
		for _, ri := range infos {
			if ri.filled == 0 {
				continue
			}
			if isHeadingText(ri.text) {
				out = append(out, &ast.Heading{Level: level, Inlines: ast.Str(ri.text)})
			} else {
				out = append(out, textBlocks(ri.text)...)
			}
		}
		return out
	}
	first, last := -1, -1
	for i, ri := range infos {
		if ri.filled >= 2 {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		// A single column of values is still a table (a list with a header).
		return []ast.Block{g.table(0, n, s.headerRows)}
	}
	// Extend the table upwards over short single-cell rows that directly
	// precede it (a header cell or a category row); rows separated by a gap
	// or holding long or wide text are titles.
	for first > 0 && tableish(infos[first-1]) && g.origRow[first]-g.origRow[first-1] == 1 {
		first--
	}
	for last < n-1 && tableish(infos[last+1]) && g.origRow[last+1]-g.origRow[last] == 1 {
		last++
	}
	var out []ast.Block
	for i := 0; i < first; i++ {
		ri := infos[i]
		if i == 0 && len([]rune(ri.text)) <= 100 && !endsSentence(ri.text) {
			out = append(out, &ast.Heading{Level: level, Inlines: ast.Str(ri.text)})
			continue
		}
		out = append(out, textBlocks(ri.text)...)
	}
	// Long single-cell rows inside the table split it (notes between
	// tables, as written by people using a sheet as a document).
	start := first
	for i := first; i <= last; i++ {
		ri := infos[i]
		if ri.filled == 1 && len([]rune(ri.text)) >= longText {
			if start < i {
				out = append(out, g.table(start, i, headerFor(s, start, first)))
			}
			out = append(out, textBlocks(ri.text)...)
			start = i + 1
		}
	}
	if start <= last {
		out = append(out, g.table(start, last+1, headerFor(s, start, first)))
	}
	for i := last + 1; i < n; i++ {
		out = append(out, textBlocks(infos[i].text)...)
	}
	return out
}

func headerFor(s *sheet, start, first int) int {
	if start == first {
		return s.headerRows
	}
	return 0
}

func tableish(ri rowInfo) bool {
	return ri.filled == 1 && !ri.wide && len([]rune(ri.text)) < longText
}

// isTextSheet reports whether a sheet is mostly prose in one column (a
// sheet used as a document).
func isTextSheet(g *grid, infos []rowInfo) bool {
	if len(g.cells) == 0 || len(g.cells[0]) > 2 {
		return false
	}
	long, rows := 0, 0
	for _, ri := range infos {
		if ri.filled == 0 {
			continue
		}
		rows++
		if ri.filled == 1 && len([]rune(ri.text)) >= longText {
			long++
		}
	}
	return long > 0 && long*2 >= rows
}

func isHeadingText(s string) bool {
	if len([]rune(s)) < 60 && !strings.ContainsAny(s, " \n") && strings.IndexFunc(s, unicode.IsLetter) >= 0 {
		return true
	}
	return isAllCaps(s)
}

func isAllCaps(s string) bool {
	if len(s) < 3 || len(s) > 80 || strings.Contains(s, "\n") {
		return false
	}
	letters := 0
	for _, r := range s {
		if unicode.IsLower(r) {
			return false
		}
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters >= 3
}

func endsSentence(s string) bool {
	r, _ := utf8.DecodeLastRuneInString(strings.TrimSpace(s))
	return r == '.' || r == '!' || r == '?' || r == ','
}

// textBlocks turns cell text into paragraphs: blank lines separate
// paragraphs, single newlines are line breaks.
func textBlocks(s string) []ast.Block {
	var out []ast.Block
	for _, part := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n\n") {
		if ins := cellInlines(part, ""); len(ins) > 0 {
			out = append(out, &ast.Para{Inlines: ins})
		}
	}
	return out
}

// cellInlines converts cell text to inlines; newlines are line breaks.
func cellInlines(s, link string) []ast.Inline {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []ast.Inline
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			out = append(out, &ast.LineBreak{})
		}
		if line = strings.TrimRight(line, " \t\r"); line != "" {
			out = append(out, &ast.Text{Value: line})
		}
	}
	if link != "" {
		return []ast.Inline{&ast.Link{URL: link, Inlines: out}}
	}
	return out
}

// table builds an ast.Table from grid rows [r0, r1). head is the number of
// header rows (0 = the first row, extended over its vertical merges); a
// single row has no header.
func (g *grid) table(r0, r1, head int) *ast.Table {
	used := g.usedCols(r0, r1)
	if head <= 0 {
		head = 1
		for _, c := range g.cells[r0] {
			if !c.covered && c.rowSpan > 1 {
				head = max(head, min(int(c.rowSpan), 3))
			}
		}
	}
	head = max(min(head, r1-r0-1), 0)
	tbl := &ast.Table{Cols: make([]ast.ColSpec, len(used))}
	region := func(from, to int) []ast.Row {
		var rows []ast.Row
		for r := from; r < to; r++ {
			var row ast.Row
			for k, col := range used {
				c := g.cells[r][col]
				if c.covered {
					// Covered by a cell of the same region: omitted. A span
					// reaching in from another region leaves an empty cell.
					if int(c.anchor) < from || int(c.anchor) >= to {
						row.Cells = append(row.Cells, ast.Cell{})
					}
					continue
				}
				cell := ast.Cell{}
				if ins := cellInlines(c.text, c.link); len(ins) > 0 {
					cell.Blocks = []ast.Block{&ast.Plain{Inlines: ins}}
				}
				if span := g.spanCols(r, col, used, k); span > 1 {
					cell.ColSpan = span
				}
				if rs := min(int(c.rowSpan), to-r); rs > 1 {
					cell.RowSpan = rs
				}
				row.Cells = append(row.Cells, cell)
			}
			rows = append(rows, row)
		}
		return rows
	}
	tbl.Head = region(r0, r0+head)
	tbl.Body = region(r0+head, r1)
	if len(tbl.Head) == 0 {
		tbl.Head = nil
	}
	g.alignColumns(tbl, r0+head, r1, used)
	return tbl
}

// usedCols lists the grid columns that hold content within rows [r0, r1).
func (g *grid) usedCols(r0, r1 int) []int {
	width := len(g.cells[r0])
	hit := make([]bool, width)
	for r := r0; r < r1; r++ {
		for k, c := range g.cells[r] {
			if c.text != "" {
				for j := k; j < min(k+int(max(c.colSpan, 1)), width); j++ {
					hit[j] = true
				}
			}
		}
	}
	var used []int
	for k, h := range hit {
		if h {
			used = append(used, k)
		}
	}
	return used
}

// spanCols counts the used columns covered by the cell at (r, col).
func (g *grid) spanCols(r, col int, used []int, k int) int {
	c := g.cells[r][col]
	if c.colSpan <= 1 {
		return 1
	}
	n := 0
	for j := k; j < len(used) && used[j] < col+int(c.colSpan); j++ {
		n++
	}
	return n
}

// alignColumns right-aligns numeric columns and applies an explicit
// alignment shared by every body cell of a column.
func (g *grid) alignColumns(tbl *ast.Table, r0, r1 int, used []int) {
	for k, col := range used {
		explicit, uniform, set := ast.AlignDefault, true, false
		filled, numeric := 0, 0
		for r := r0; r < r1; r++ {
			c := g.cells[r][col]
			if c.covered || c.text == "" || c.colSpan > 1 {
				continue
			}
			filled++
			if c.kind == kNumber || c.kind == kText && isNumeric(c.text) {
				numeric++
			}
			if !set {
				explicit, set = c.align, true
			} else if c.align != explicit {
				uniform = false
			}
		}
		switch {
		case set && uniform && explicit != ast.AlignDefault:
			tbl.Cols[k].Align = explicit
		case filled > 0 && numeric*5 >= filled*4:
			tbl.Cols[k].Align = ast.AlignRight
		}
	}
}

// isNumeric reports whether s reads as a number: thousands separators,
// decimal comma, currency symbols, percent, signs and accounting-style
// parentheses are accepted.
func isNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		s = s[1 : len(s)-1]
	}
	s = trimAffixes(s)
	if s == "" {
		return false
	}
	digits, lastSep := 0, false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r >= '0' && r <= '9':
			digits++
			lastSep = false
		case r == '.' || r == ',' || r == '\'' || r == ' ' || r == '\u00a0' || r == '\u202f':
			if lastSep || i == 0 && r != '.' && r != ',' {
				return false
			}
			lastSep = true
		case (r == 'e' || r == 'E') && digits > 0 && !lastSep:
			exp := strings.TrimLeft(s[i+1:], "+-")
			return exp != "" && strings.Trim(exp, "0123456789") == ""
		default:
			return false
		}
		i += size
	}
	return digits > 0 && !lastSep
}

var numAffixes = []string{"+", "-", "\u2212", "€", "$", "£", "¥", "₽", "₹", "%", "EUR", "USD", "GBP"}

// trimAffixes strips signs, currency symbols and percent signs on either
// side of a number.
func trimAffixes(s string) string {
	for range 4 {
		before := s
		s = strings.TrimSpace(s)
		for _, a := range numAffixes {
			s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, a), a))
		}
		if s == before {
			break
		}
	}
	return s
}
