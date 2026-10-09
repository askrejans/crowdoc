package markdown

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/width"

	"github.com/askrejans/crowdoc/v2/ast"
)

func (w *writer) table(t *ast.Table) string {
	if !pipeTableOK(t) {
		return w.htmlTable(t)
	}
	ncols := len(t.Cols)
	for _, rows := range [][]ast.Row{t.Head, t.Body} {
		for _, r := range rows {
			ncols = max(ncols, len(r.Cells))
		}
	}
	if ncols == 0 {
		return ""
	}
	if len(t.Attr.Classes) > 0 || len(t.Attr.KV) > 0 {
		w.warn("table classes and attributes dropped")
	}
	cell := func(c ast.Cell) string {
		if len(c.Blocks) == 0 {
			return ""
		}
		return w.leaf(blockInlines(c.Blocks[0]), ictx{table: true})
	}
	var rows [][]string
	head := make([]string, ncols)
	if len(t.Head) == 1 {
		for i, c := range t.Head[0].Cells {
			head[i] = cell(c)
		}
	}
	rows = append(rows, head)
	for _, r := range t.Body {
		row := make([]string, ncols)
		for i, c := range r.Cells {
			row[i] = cell(c)
		}
		rows = append(rows, row)
	}
	widths := make([]int, ncols)
	for _, r := range rows {
		for i, s := range r {
			widths[i] = max(widths[i], displayWidth(s))
		}
	}
	for i := range widths {
		widths[i] = max(widths[i], 3)
		if widths[i] > maxPadWidth {
			widths[i] = 0 // do not pad very wide columns
		}
	}
	align := func(i int) ast.Align {
		if i < len(t.Cols) {
			return t.Cols[i].Align
		}
		return ast.AlignDefault
	}
	var sb strings.Builder
	if caption := w.leaf(t.Caption, ictx{caption: true}); caption != "" {
		sb.WriteString("Table: " + caption)
		if id := safeID(t.Attr.ID); id != "" {
			sb.WriteString(" {#" + id + "}")
		}
		sb.WriteString("\n\n")
	} else if id := safeID(t.Attr.ID); id != "" {
		sb.WriteString("Table: {#" + id + "}\n\n")
	}
	writeRow := func(r []string) {
		sb.WriteString("|")
		for i, s := range r {
			sb.WriteString(" ")
			sb.WriteString(pad(s, widths[i], align(i)))
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}
	writeRow(rows[0])
	sb.WriteString("|")
	for i := range widths {
		wd := max(widths[i], 3)
		dashes := strings.Repeat("-", wd)
		switch align(i) {
		case ast.AlignLeft:
			dashes = ":" + dashes[1:]
		case ast.AlignRight:
			dashes = dashes[1:] + ":"
		case ast.AlignCenter:
			dashes = ":" + dashes[2:] + ":"
		}
		sb.WriteString(" " + dashes + " |")
	}
	sb.WriteString("\n")
	for _, r := range rows[1:] {
		writeRow(r)
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

const maxPadWidth = 40

// pipeTableOK reports whether a table fits GFM pipe-table syntax: at most
// one header row, no footer, no spans, cells holding at most one
// paragraph.
func pipeTableOK(t *ast.Table) bool {
	if len(t.Head) > 1 || len(t.Foot) > 0 || len(t.Head)+len(t.Body) == 0 || len(t.Head) == 1 && len(t.Body) == 0 && emptyRow(t.Head[0]) {
		return false
	}
	for _, rows := range [][]ast.Row{t.Head, t.Body} {
		for _, r := range rows {
			for _, c := range r.Cells {
				if c.ColSpan > 1 || c.RowSpan > 1 || len(c.Blocks) > 1 {
					return false
				}
				if len(c.Blocks) == 1 && !isText(c.Blocks[0]) {
					return false
				}
			}
		}
	}
	return true
}

func emptyRow(r ast.Row) bool {
	for _, c := range r.Cells {
		if len(c.Blocks) > 0 {
			return false
		}
	}
	return true
}

func blockInlines(b ast.Block) []ast.Inline {
	switch n := b.(type) {
	case *ast.Para:
		return n.Inlines
	case *ast.Plain:
		return n.Inlines
	}
	return nil
}

func pad(s string, wd int, a ast.Align) string {
	gap := wd - displayWidth(s)
	if gap <= 0 {
		return s
	}
	switch a {
	case ast.AlignRight:
		return strings.Repeat(" ", gap) + s
	case ast.AlignCenter:
		l := gap / 2
		return strings.Repeat(" ", l) + s + strings.Repeat(" ", gap-l)
	}
	return s + strings.Repeat(" ", gap)
}

// displayWidth estimates the columns s occupies in a monospaced editor:
// wide and full-width characters (CJK, most emoji) count two, combining
// marks and zero-width characters none.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case r < 0x20:
		case r < utf8.RuneSelf:
			n++
		case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) || r == 0xFE0F:
		default:
			switch width.LookupRune(r).Kind() {
			case width.EastAsianWide, width.EastAsianFullwidth:
				n += 2
			default:
				if r >= 0x1F300 && r <= 0x1FAFF {
					n += 2
				} else {
					n++
				}
			}
		}
	}
	return n
}
