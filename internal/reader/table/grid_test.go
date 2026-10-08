package table

import (
	"strings"
	"testing"
)

// cells builds sheet cells from rows of strings ("" = empty).
func cells(rows ...[]string) []sheetCell {
	var out []sheetCell
	for r, row := range rows {
		for c, v := range row {
			if v != "" {
				out = append(out, sheetCell{row: r, col: c, text: v})
			}
		}
	}
	return out
}

func layout(s *sheet, level int) string {
	g, _ := buildGrid(s.cells, 1<<20)
	if g == nil {
		return ""
	}
	return dump(sheetBlocks(s, g, level))
}

func TestSheetLayout(t *testing.T) {
	long := "This is a long explanatory sentence that somebody typed into a single cell between two tables."
	tests := []struct {
		name  string
		cells []sheetCell
		want  string
	}{
		{"plain table", cells([]string{"a", "b"}, []string{"1", "2"}), "Table[RR]{H[a|b][1|2]}"},
		{"title row and gap", cells([]string{"Inventory"}, nil, []string{"Item", "Qty"}, []string{"Nails", "100"}),
			"H1 Inventory\nTable[-R]{H[Item|Qty][Nails|100]}"},
		{"category rows stay in the table", cells([]string{"Item", "Qty"}, []string{"Fruit", ""}, []string{"Apple", "3"}, []string{"Pear", "4"}),
			"Table[-R]{H[Item|Qty][Fruit|][Apple|3][Pear|4]}"},
		{"long text splits tables", cells([]string{"a", "b"}, []string{"1", "2"}, []string{long}, []string{"c", "d"}, []string{"3", "4"}),
			"Table[RR]{H[a|b][1|2]}\nP " + long + "\nTable[RR]{H[c|d][3|4]}"},
		{"trailing note after gap", cells([]string{"a", "b"}, []string{"1", "2"}, nil, []string{"Source: survey"}),
			"Table[RR]{H[a|b][1|2]}\nP Source: survey"},
		{"single column list", cells([]string{"Names"}, []string{"Anna"}, []string{"Juris"}), "Table[-]{H[Names][Anna][Juris]}"},
		{"merged section row stays in the table", []sheetCell{
			{row: 0, col: 0, text: "Item"}, {row: 0, col: 1, text: "Qty"}, {row: 0, col: 2, text: "Price"},
			{row: 1, col: 0, text: "Fruit", colSpan: 3},
			{row: 2, col: 0, text: "Apple"}, {row: 2, col: 1, text: "3"}, {row: 2, col: 2, text: "0.5"},
		}, "Table[-RR]{H[Item|Qty|Price][Fruit<c3>][Apple|3|0.5]}"},
		{"sentence title is a paragraph", cells([]string{"Figures are in thousands."}, nil, []string{"a", "b"}, []string{"1", "2"}),
			"P Figures are in thousands.\nTable[RR]{H[a|b][1|2]}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := layout(&sheet{cells: tc.cells}, 1); got != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

func TestMultiRowHeader(t *testing.T) {
	// "Region" spans two header rows; "Sales" spans two columns.
	s := &sheet{cells: []sheetCell{
		{row: 0, col: 0, text: "Region", rowSpan: 2}, {row: 0, col: 1, text: "Sales", colSpan: 2},
		{row: 1, col: 1, text: "Q1"}, {row: 1, col: 2, text: "Q2"},
		{row: 2, col: 0, text: "North"}, {row: 2, col: 1, text: "1"}, {row: 2, col: 2, text: "2"},
	}}
	want := "Table[-RR]{H[Region<r2>|Sales<c2>]H[Q1|Q2][North|1|2]}"
	if got := layout(s, 1); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestSpanAcrossHeaderAndBody(t *testing.T) {
	// A header cell merged down into the body is clamped; the body keeps an
	// empty cell in its place.
	s := &sheet{headerRows: 1, cells: []sheetCell{
		{row: 0, col: 0, text: "A"}, {row: 0, col: 1, text: "B", rowSpan: 2},
		{row: 1, col: 0, text: "x"},
		{row: 2, col: 0, text: "y"}, {row: 2, col: 1, text: "z"},
	}}
	want := "Table[--]{H[A|B][x|][y|z]}"
	if got := layout(s, 1); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestTextSheetHeadings(t *testing.T) {
	long := strings.Repeat("word ", 20)
	s := &sheet{cells: cells([]string{"SUMMARY"}, []string{long}, []string{"Details"}, []string{long + "again"})}
	want := "H2 SUMMARY\nP " + strings.TrimSpace(long) + "\nH2 Details\nP " + long + "again"
	if got := layout(s, 2); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBuildGridTruncates(t *testing.T) {
	var cs []sheetCell
	for r := range 100 {
		cs = append(cs, sheetCell{row: r, col: 0, text: "x"}, sheetCell{row: r, col: 5000, text: "y"})
	}
	g, truncated := buildGrid(cs, 50)
	if !truncated || len(g.cells) != 25 || len(g.cells[0]) != 2 {
		t.Fatalf("truncated=%v rows=%d cols=%d", truncated, len(g.cells), len(g.cells[0]))
	}
}
