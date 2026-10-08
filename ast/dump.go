package ast

import (
	"fmt"
	"sort"
	"strings"
)

// Dump renders blocks as a compact, deterministic S-expression. It is meant
// for tests and debugging.
func Dump(blocks []Block) string {
	var sb strings.Builder
	for _, b := range blocks {
		dumpBlock(&sb, b, 0)
	}
	return sb.String()
}

// DumpInlines renders inlines like Dump.
func DumpInlines(ins []Inline) string {
	var sb strings.Builder
	dumpInlines(&sb, ins)
	return sb.String()
}

func indent(sb *strings.Builder, n int) { sb.WriteString(strings.Repeat("  ", n)) }

func dumpAttr(a Attr) string {
	var parts []string
	if a.ID != "" {
		parts = append(parts, "#"+a.ID)
	}
	for _, c := range a.Classes {
		parts = append(parts, "."+c)
	}
	keys := make([]string, 0, len(a.KV))
	for k := range a.KV {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, k+"="+a.KV[k])
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, " ") + "}"
}

func dumpBlock(sb *strings.Builder, b Block, lvl int) {
	indent(sb, lvl)
	switch n := b.(type) {
	case *Para:
		sb.WriteString("Para ")
		dumpInlines(sb, n.Inlines)
	case *Plain:
		sb.WriteString("Plain ")
		dumpInlines(sb, n.Inlines)
	case *Heading:
		fmt.Fprintf(sb, "H%d%s", n.Level, dumpAttr(n.Attr))
		if n.Unnumbered {
			sb.WriteString("[-]")
		}
		sb.WriteByte(' ')
		dumpInlines(sb, n.Inlines)
	case *CodeBlock:
		fmt.Fprintf(sb, "Code[%s]%s %q", n.Lang, dumpAttr(n.Attr), n.Text)
		if n.Caption != nil {
			sb.WriteString(" cap=")
			dumpInlines(sb, n.Caption)
		}
	case *MathBlock:
		fmt.Fprintf(sb, "MathBlock %q", n.TeX)
		if n.Label != "" {
			sb.WriteString(" #" + n.Label)
		}
	case *RawBlock:
		fmt.Fprintf(sb, "Raw[%s] %q", n.Format, n.Text)
	case *BlockQuote:
		sb.WriteString("Quote\n")
		for _, c := range n.Blocks {
			dumpBlock(sb, c, lvl+1)
		}
		return
	case *List:
		kind := "Bullet"
		if n.Ordered {
			kind = fmt.Sprintf("Ordered(%d,%d)", n.Start, n.Style)
		}
		if n.Tight {
			kind += "-tight"
		}
		sb.WriteString(kind + "\n")
		for _, it := range n.Items {
			indent(sb, lvl+1)
			sb.WriteString("Item")
			switch it.Task {
			case TaskOpen:
				sb.WriteString("[ ]")
			case TaskDone:
				sb.WriteString("[x]")
			}
			sb.WriteByte('\n')
			for _, c := range it.Blocks {
				dumpBlock(sb, c, lvl+2)
			}
		}
		return
	case *DefinitionList:
		sb.WriteString("DefList\n")
		for _, it := range n.Items {
			indent(sb, lvl+1)
			sb.WriteString("Term ")
			dumpInlines(sb, it.Term)
			sb.WriteByte('\n')
			for _, d := range it.Definitions {
				for _, c := range d {
					dumpBlock(sb, c, lvl+2)
				}
			}
		}
		return
	case *Table:
		fmt.Fprintf(sb, "Table%s cols=%d", dumpAttr(n.Attr), len(n.Cols))
		if n.Caption != nil {
			sb.WriteString(" cap=")
			dumpInlines(sb, n.Caption)
		}
		sb.WriteByte('\n')
		for _, part := range []struct {
			name string
			rows []Row
		}{{"head", n.Head}, {"body", n.Body}, {"foot", n.Foot}} {
			for _, r := range part.rows {
				indent(sb, lvl+1)
				sb.WriteString(part.name + ":")
				for _, c := range r.Cells {
					sb.WriteString(" |")
					if c.ColSpan > 1 || c.RowSpan > 1 {
						fmt.Fprintf(sb, "(%dx%d)", c.ColSpan, c.RowSpan)
					}
					sb.WriteString(strings.TrimSpace(strings.ReplaceAll(BlocksText(c.Blocks), "\n", " / ")))
				}
				sb.WriteByte('\n')
			}
		}
		return
	case *Figure:
		fmt.Fprintf(sb, "Figure%s", dumpAttr(n.Attr))
		if n.Image != nil {
			fmt.Fprintf(sb, " src=%q", n.Image.Src)
			if n.Image.Width != "" {
				fmt.Fprintf(sb, " w=%s", n.Image.Width)
			}
		}
		if n.Caption != nil {
			sb.WriteString(" cap=")
			dumpInlines(sb, n.Caption)
		}
	case *HorizontalRule:
		sb.WriteString("HR")
	case *PageBreak:
		sb.WriteString("PageBreak")
	case *Div:
		fmt.Fprintf(sb, "Div%s", dumpAttr(n.Attr))
		if n.Title != nil {
			sb.WriteString(" title=")
			dumpInlines(sb, n.Title)
		}
		sb.WriteByte('\n')
		for _, c := range n.Blocks {
			dumpBlock(sb, c, lvl+1)
		}
		return
	case *LineBlock:
		sb.WriteString("LineBlock")
		for _, l := range n.Lines {
			sb.WriteString(" | ")
			dumpInlines(sb, l)
		}
	case *Bibliography:
		sb.WriteString("Bibliography")
	case *ReferenceList:
		sb.WriteString("ReferenceList\n")
		for _, e := range n.Entries {
			indent(sb, lvl+1)
			fmt.Fprintf(sb, "[%s]#%s ", e.Label, e.ID)
			dumpInlines(sb, e.Inlines)
			sb.WriteByte('\n')
		}
		return
	default:
		fmt.Fprintf(sb, "%T", b)
	}
	sb.WriteByte('\n')
}

func dumpInlines(sb *strings.Builder, ins []Inline) {
	for _, in := range ins {
		switch n := in.(type) {
		case *Text:
			sb.WriteString(n.Value)
		case *SoftBreak:
			sb.WriteString("↵")
		case *LineBreak:
			sb.WriteString("⏎")
		case *Emph:
			sb.WriteString("_")
			dumpInlines(sb, n.Inlines)
			sb.WriteString("_")
		case *Strong:
			sb.WriteString("**")
			dumpInlines(sb, n.Inlines)
			sb.WriteString("**")
		case *Code:
			sb.WriteString("`" + n.Text + "`")
		case *Math:
			sb.WriteString("$" + n.TeX + "$")
		case *Link:
			sb.WriteString("[")
			dumpInlines(sb, n.Inlines)
			sb.WriteString("](" + n.URL + ")")
		case *Image:
			fmt.Fprintf(sb, "![%s](%s)", n.Alt, n.Src)
		case *Note:
			sb.WriteString("^[")
			sb.WriteString(strings.TrimSpace(strings.ReplaceAll(Dump(n.Blocks), "\n", " ")))
			sb.WriteString("]")
		case *Cite:
			sb.WriteString("{cite")
			if n.Mode == CiteNarrative {
				sb.WriteString("-n")
			}
			for _, it := range n.Items {
				sb.WriteString(" ")
				if it.SuppressAuthor {
					sb.WriteString("-")
				}
				sb.WriteString("@" + it.Key)
				if it.Locator != "" {
					sb.WriteString("," + it.LocatorLabel + "=" + it.Locator)
				}
				if it.Prefix != "" {
					sb.WriteString(" pre=" + it.Prefix)
				}
				if it.Suffix != "" {
					sb.WriteString(" suf=" + it.Suffix)
				}
			}
			sb.WriteString("}")
		case *Ref:
			sb.WriteString("{ref " + n.Target + "}")
		case *RawInline:
			fmt.Fprintf(sb, "{raw-%s %q}", n.Format, n.Text)
		default:
			name := strings.TrimPrefix(fmt.Sprintf("%T", in), "*ast.")
			sb.WriteString(name + "(")
			dumpInlines(sb, InlineChildren(in))
			sb.WriteString(")")
		}
	}
}
