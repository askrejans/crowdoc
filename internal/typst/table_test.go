package typst

import (
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

func TestGridLayoutKeepsMergedCellsDisjoint(t *testing.T) {
	rows := []ast.Row{
		{Cells: []ast.Cell{{}, {RowSpan: 2}, {}}},
		{Cells: []ast.Cell{{ColSpan: 2}, {}}},
	}
	placed, _ := gridLayout(rows)
	occupied := map[[2]int]bool{}
	for _, row := range placed {
		for _, cell := range row {
			for y := 0; y < max(cell.cell.RowSpan, 1); y++ {
				for x := 0; x < max(cell.cell.ColSpan, 1); x++ {
					at := [2]int{cell.row + y, cell.col + x}
					if occupied[at] {
						t.Fatalf("merged cells overlap at row %d, column %d", at[0], at[1])
					}
					occupied[at] = true
				}
			}
		}
	}
}

func TestMergedTableKeepsSourceRowsInTypst(t *testing.T) {
	cell := func(text string, colspan, rowspan int) ast.Cell {
		return ast.Cell{Blocks: []ast.Block{&ast.Para{Inlines: ast.Str(text)}}, ColSpan: colspan, RowSpan: rowspan}
	}
	table := &ast.Table{Body: []ast.Row{
		{Cells: []ast.Cell{cell("Alpha", 1, 1), cell("Bravo", 1, 2), cell("Charlie", 1, 1)}},
		{Cells: []ast.Cell{cell("Delta", 2, 1), cell("Echo", 1, 1)}},
	}}
	got := (&writer{}).table(table)
	for _, want := range []string{
		"table.cell(x: 1, y: 0, rowspan: 2)[Bravo]",
		"table.cell(x: 2, y: 1, colspan: 2)[Delta]",
		"table.cell(x: 4, y: 1)[Echo]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("source cell placement missing %s in %s", want, got)
		}
	}
}
