package text

import (
	"fmt"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// dump renders blocks in a compact Markdown-like notation so tests can
// compare whole structures as strings.
func dump(blocks []ast.Block) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		parts = append(parts, dumpBlock(b))
	}
	return strings.Join(parts, "\n")
}

func dumpBlock(b ast.Block) string {
	switch n := b.(type) {
	case *ast.Para:
		return "P " + dumpInlines(n.Inlines)
	case *ast.Plain:
		return dumpInlines(n.Inlines)
	case *ast.Heading:
		id := ""
		if n.Attr.ID != "" {
			id = "#" + n.Attr.ID
		}
		return fmt.Sprintf("H%d%s %s", n.Level, id, dumpInlines(n.Inlines))
	case *ast.CodeBlock:
		return fmt.Sprintf("Code(%s%s) %q", n.Lang, classes(n.Attr), n.Text)
	case *ast.MathBlock:
		return "Math " + n.TeX
	case *ast.RawBlock:
		return fmt.Sprintf("Raw(%s) %q", n.Format, n.Text)
	case *ast.BlockQuote:
		return "Quote{" + strings.ReplaceAll(dump(n.Blocks), "\n", "; ") + "}"
	case *ast.List:
		var items []string
		for _, it := range n.Items {
			items = append(items, strings.ReplaceAll(dump(it.Blocks), "\n", "; "))
		}
		kind := "UL"
		if n.Ordered {
			kind = fmt.Sprintf("OL(%d,%d)", n.Start, n.Style)
		}
		if !n.Tight {
			kind += "loose"
		}
		return kind + "{" + strings.Join(items, " | ") + "}"
	case *ast.Table:
		var sb strings.Builder
		sb.WriteString("Table")
		if n.Caption != nil {
			sb.WriteString("(" + dumpInlines(n.Caption) + ")")
		}
		var aligns []string
		for _, c := range n.Cols {
			aligns = append(aligns, "LCR"[max(int(c.Align)-1, 0):max(int(c.Align)-1, 0)+1])
			if c.Align == ast.AlignDefault {
				aligns[len(aligns)-1] = "-"
			}
		}
		sb.WriteString("[" + strings.Join(aligns, "") + "]{")
		rows := func(prefix string, rs []ast.Row) {
			for _, r := range rs {
				var cells []string
				for _, c := range r.Cells {
					s := strings.ReplaceAll(dump(c.Blocks), "\n", "; ")
					if c.ColSpan > 1 {
						s += fmt.Sprintf("<c%d>", c.ColSpan)
					}
					if c.RowSpan > 1 {
						s += fmt.Sprintf("<r%d>", c.RowSpan)
					}
					cells = append(cells, s)
				}
				sb.WriteString(prefix + strings.Join(cells, "|") + "]")
			}
		}
		rows("H[", n.Head)
		rows("[", n.Body)
		sb.WriteString("}")
		return sb.String()
	case *ast.Figure:
		s := "Fig " + dumpInlines([]ast.Inline{n.Image})
		if n.Caption != nil {
			s += " cap=" + dumpInlines(n.Caption)
		}
		return s
	case *ast.PageBreak:
		return "PB"
	case *ast.HorizontalRule:
		return "HR"
	case *ast.Div:
		return "Div" + classes(n.Attr) + "{" + strings.ReplaceAll(dump(n.Blocks), "\n", "; ") + "}"
	case *ast.LineBlock:
		var lines []string
		for _, l := range n.Lines {
			lines = append(lines, dumpInlines(l))
		}
		return "Lines{" + strings.Join(lines, " / ") + "}"
	}
	return fmt.Sprintf("%T", b)
}

func classes(a ast.Attr) string {
	s := ""
	for _, c := range a.Classes {
		s += "." + c
	}
	for k, v := range a.KV {
		s += " " + k + "=" + v
	}
	return s
}

func dumpInlines(ins []ast.Inline) string {
	var sb strings.Builder
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			sb.WriteString(n.Value)
		case *ast.Strong:
			sb.WriteString("**" + dumpInlines(n.Inlines) + "**")
		case *ast.Emph:
			sb.WriteString("*" + dumpInlines(n.Inlines) + "*")
		case *ast.Underline:
			sb.WriteString("_{" + dumpInlines(n.Inlines) + "}")
		case *ast.Strike:
			sb.WriteString("~~" + dumpInlines(n.Inlines) + "~~")
		case *ast.Superscript:
			sb.WriteString("^{" + dumpInlines(n.Inlines) + "}")
		case *ast.Subscript:
			sb.WriteString("~{" + dumpInlines(n.Inlines) + "}")
		case *ast.SmallCaps:
			sb.WriteString("sc{" + dumpInlines(n.Inlines) + "}")
		case *ast.Highlight:
			sb.WriteString("=={" + dumpInlines(n.Inlines) + "}")
		case *ast.Code:
			sb.WriteString("`" + n.Text + "`")
		case *ast.Math:
			sb.WriteString("$" + n.TeX + "$")
		case *ast.Link:
			sb.WriteString("[" + dumpInlines(n.Inlines) + "](" + n.URL + ")")
		case *ast.Image:
			sb.WriteString("![" + n.Alt + "](" + n.Src)
			if n.Width != "" {
				sb.WriteString(" " + n.Width + "×" + n.Height)
			}
			sb.WriteString(")")
		case *ast.Note:
			sb.WriteString("^[" + strings.ReplaceAll(dump(n.Blocks), "\n", "; ") + "]")
		case *ast.LineBreak:
			sb.WriteString("↵")
		case *ast.SoftBreak:
			sb.WriteString("⏎")
		case *ast.Span:
			if n.Attr.ID != "" && len(n.Inlines) == 0 {
				sb.WriteString("{#" + n.Attr.ID + "}")
			} else {
				sb.WriteString("[" + dumpInlines(n.Inlines) + "]{" + n.Attr.ID + classes(n.Attr) + "}")
			}
		default:
			sb.WriteString(fmt.Sprintf("<%T>", in))
		}
	}
	return sb.String()
}
