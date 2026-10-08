// Package asttest renders AST fragments in a compact notation so reader
// tests can compare whole documents in one assertion.
package asttest

import (
	"fmt"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// Dump renders blocks one per line. Notation: P[..] paragraph, Pl[..]
// plain, H2#id[..] heading, Code(lang)"..", $$..$$ display math, BQ{..},
// UL{a | b} / OL(start,style){..} (~ marks loose lists), DL{term: defs},
// T{cap; cols; head; body; foot}, Fig(img)[caption], Div.class(title){..},
// LB{line / line}; inlines: *[strong] _[emph] u[..] ~[strike] ^[sup]
// ,[sub] sc[..] =[mark] `code` $math$ <url>[text] !img(src ..) ^note{..}
// @cite(keys)[fallback] \n line break, span#id.class[..].
func Dump(blocks []ast.Block) string {
	var sb strings.Builder
	for i, b := range blocks {
		if i > 0 {
			sb.WriteString("\n")
		}
		dumpBlock(&sb, b)
	}
	return sb.String()
}

func dumpBlocksInline(sb *strings.Builder, blocks []ast.Block) {
	for i, b := range blocks {
		if i > 0 {
			sb.WriteString(" ")
		}
		dumpBlock(sb, b)
	}
}

func dumpBlock(sb *strings.Builder, b ast.Block) {
	switch n := b.(type) {
	case *ast.Para:
		sb.WriteString("P[" + dumpIns(n.Inlines) + "]")
	case *ast.Plain:
		sb.WriteString("Pl[" + dumpIns(n.Inlines) + "]")
	case *ast.Heading:
		fmt.Fprintf(sb, "H%d", n.Level)
		if n.Attr.ID != "" {
			sb.WriteString("#" + n.Attr.ID)
		}
		sb.WriteString("[" + dumpIns(n.Inlines) + "]")
	case *ast.CodeBlock:
		fmt.Fprintf(sb, "Code(%s)%q", n.Lang, n.Text)
		if len(n.Caption) > 0 {
			sb.WriteString("[" + dumpIns(n.Caption) + "]")
		}
	case *ast.MathBlock:
		sb.WriteString("$$" + n.TeX + "$$")
	case *ast.BlockQuote:
		sb.WriteString("BQ{")
		dumpBlocksInline(sb, n.Blocks)
		sb.WriteString("}")
	case *ast.List:
		if n.Ordered {
			fmt.Fprintf(sb, "OL(%d,%d)", n.Start, n.Style)
		} else {
			sb.WriteString("UL")
		}
		if !n.Tight {
			sb.WriteString("~")
		}
		sb.WriteString("{")
		for i, it := range n.Items {
			if i > 0 {
				sb.WriteString(" | ")
			}
			switch it.Task {
			case ast.TaskOpen:
				sb.WriteString("[ ] ")
			case ast.TaskDone:
				sb.WriteString("[x] ")
			}
			dumpBlocksInline(sb, it.Blocks)
		}
		sb.WriteString("}")
	case *ast.DefinitionList:
		sb.WriteString("DL{")
		for i, it := range n.Items {
			if i > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString(dumpIns(it.Term) + ":")
			for _, d := range it.Definitions {
				sb.WriteString(" ")
				dumpBlocksInline(sb, d)
			}
		}
		sb.WriteString("}")
	case *ast.Table:
		sb.WriteString("T")
		if n.Attr.ID != "" {
			sb.WriteString("#" + n.Attr.ID)
		}
		sb.WriteString("{")
		if len(n.Caption) > 0 {
			sb.WriteString("cap:" + dumpIns(n.Caption) + "; ")
		}
		var al []string
		for _, c := range n.Cols {
			al = append(al, fmt.Sprint(int(c.Align)))
		}
		sb.WriteString("cols:" + strings.Join(al, ","))
		dumpRows(sb, "head", n.Head)
		dumpRows(sb, "body", n.Body)
		dumpRows(sb, "foot", n.Foot)
		sb.WriteString("}")
	case *ast.Figure:
		sb.WriteString("Fig")
		if n.Attr.ID != "" {
			sb.WriteString("#" + n.Attr.ID)
		}
		sb.WriteString("(" + dumpIns([]ast.Inline{n.Image}) + ")")
		if len(n.Caption) > 0 {
			sb.WriteString("[" + dumpIns(n.Caption) + "]")
		}
	case *ast.HorizontalRule:
		sb.WriteString("HR")
	case *ast.PageBreak:
		sb.WriteString("PB")
	case *ast.Div:
		sb.WriteString("Div")
		for _, c := range n.Attr.Classes {
			sb.WriteString("." + c)
		}
		if n.Attr.ID != "" {
			sb.WriteString("#" + n.Attr.ID)
		}
		if len(n.Title) > 0 {
			sb.WriteString("(" + dumpIns(n.Title) + ")")
		}
		sb.WriteString("{")
		dumpBlocksInline(sb, n.Blocks)
		sb.WriteString("}")
	case *ast.LineBlock:
		var lines []string
		for _, l := range n.Lines {
			lines = append(lines, dumpIns(l))
		}
		sb.WriteString("LB{" + strings.Join(lines, " / ") + "}")
	case *ast.Bibliography:
		sb.WriteString("Bib")
	default:
		fmt.Fprintf(sb, "%T", b)
	}
}

func dumpRows(sb *strings.Builder, name string, rows []ast.Row) {
	if len(rows) == 0 {
		return
	}
	sb.WriteString("; " + name + ":")
	for i, r := range rows {
		if i > 0 {
			sb.WriteString(" /")
		}
		for _, c := range r.Cells {
			sb.WriteString(" (")
			if c.ColSpan > 1 {
				fmt.Fprintf(sb, "c%d ", c.ColSpan)
			}
			if c.RowSpan > 1 {
				fmt.Fprintf(sb, "r%d ", c.RowSpan)
			}
			if c.Align != ast.AlignDefault {
				fmt.Fprintf(sb, "a%d ", c.Align)
			}
			dumpBlocksInline(sb, c.Blocks)
			sb.WriteString(")")
		}
	}
}

// DumpInlines renders inlines in the notation of Dump.
func DumpInlines(ins []ast.Inline) string { return dumpIns(ins) }

func dumpIns(ins []ast.Inline) string {
	var sb strings.Builder
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			sb.WriteString(n.Value)
		case *ast.Strong:
			sb.WriteString("*[" + dumpIns(n.Inlines) + "]")
		case *ast.Emph:
			sb.WriteString("_[" + dumpIns(n.Inlines) + "]")
		case *ast.Underline:
			sb.WriteString("u[" + dumpIns(n.Inlines) + "]")
		case *ast.Strike:
			sb.WriteString("~[" + dumpIns(n.Inlines) + "]")
		case *ast.Superscript:
			sb.WriteString("^[" + dumpIns(n.Inlines) + "]")
		case *ast.Subscript:
			sb.WriteString(",[" + dumpIns(n.Inlines) + "]")
		case *ast.SmallCaps:
			sb.WriteString("sc[" + dumpIns(n.Inlines) + "]")
		case *ast.Highlight:
			sb.WriteString("=[" + dumpIns(n.Inlines) + "]")
		case *ast.Code:
			sb.WriteString("`" + n.Text + "`")
		case *ast.Math:
			sb.WriteString("$" + n.TeX + "$")
		case *ast.Link:
			sb.WriteString("<" + n.URL + ">[" + dumpIns(n.Inlines) + "]")
		case *ast.Image:
			s := "!img(" + n.Src
			if n.Alt != "" {
				s += " alt=" + n.Alt
			}
			if n.Width != "" {
				s += " w=" + n.Width
			}
			if n.Height != "" {
				s += " h=" + n.Height
			}
			sb.WriteString(s + ")")
		case *ast.Note:
			var nb strings.Builder
			dumpBlocksInline(&nb, n.Blocks)
			sb.WriteString("^note{" + nb.String() + "}")
		case *ast.Cite:
			var keys []string
			for _, it := range n.Items {
				keys = append(keys, it.Key)
			}
			sb.WriteString("@cite(" + strings.Join(keys, ";") + ")[" + dumpIns(n.Fallback) + "]")
		case *ast.LineBreak:
			sb.WriteString("\\n")
		case *ast.SoftBreak:
			sb.WriteString("~")
		case *ast.Span:
			s := "span"
			if n.Attr.ID != "" {
				s += "#" + n.Attr.ID
			}
			for _, c := range n.Attr.Classes {
				s += "." + c
			}
			sb.WriteString(s + "[" + dumpIns(n.Inlines) + "]")
		default:
			fmt.Fprintf(&sb, "%T", in)
		}
	}
	return sb.String()
}
