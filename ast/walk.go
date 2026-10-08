package ast

import "strings"

// Children returns the direct child blocks of b (nil for leaf blocks).
// List items, definitions and table cells are flattened in document order.
func Children(b Block) []Block {
	switch n := b.(type) {
	case *BlockQuote:
		return n.Blocks
	case *Div:
		return n.Blocks
	case *List:
		var out []Block
		for _, it := range n.Items {
			out = append(out, it.Blocks...)
		}
		return out
	case *DefinitionList:
		var out []Block
		for _, it := range n.Items {
			for _, d := range it.Definitions {
				out = append(out, d...)
			}
		}
		return out
	case *Table:
		var out []Block
		for _, rows := range [][]Row{n.Head, n.Body, n.Foot} {
			for _, r := range rows {
				for _, c := range r.Cells {
					out = append(out, c.Blocks...)
				}
			}
		}
		return out
	}
	return nil
}

// InlinesOf returns the inline content held directly by block b.
func InlinesOf(b Block) [][]Inline {
	switch n := b.(type) {
	case *Para:
		return [][]Inline{n.Inlines}
	case *Plain:
		return [][]Inline{n.Inlines}
	case *Heading:
		return [][]Inline{n.Inlines}
	case *CodeBlock:
		return [][]Inline{n.Caption}
	case *Table:
		return [][]Inline{n.Caption}
	case *Figure:
		out := [][]Inline{n.Caption}
		if n.Image != nil {
			out = append(out, []Inline{n.Image})
		}
		return out
	case *Div:
		return [][]Inline{n.Title}
	case *DefinitionList:
		var out [][]Inline
		for _, it := range n.Items {
			out = append(out, it.Term)
		}
		return out
	case *LineBlock:
		return n.Lines
	case *ReferenceList:
		var out [][]Inline
		for _, e := range n.Entries {
			out = append(out, e.Inlines)
		}
		return out
	}
	return nil
}

// InlineChildren returns the inline children of a container inline.
func InlineChildren(in Inline) []Inline {
	switch n := in.(type) {
	case *Emph:
		return n.Inlines
	case *Strong:
		return n.Inlines
	case *Strike:
		return n.Inlines
	case *Underline:
		return n.Inlines
	case *Superscript:
		return n.Inlines
	case *Subscript:
		return n.Inlines
	case *SmallCaps:
		return n.Inlines
	case *Highlight:
		return n.Inlines
	case *Link:
		return n.Inlines
	case *Span:
		return n.Inlines
	case *Cite:
		if n.Rendered != nil {
			return n.Rendered
		}
		return n.Fallback
	}
	return nil
}

// WalkBlocks calls fn for every block in depth-first document order,
// including blocks nested in footnotes. Returning false skips the block's
// children.
func WalkBlocks(blocks []Block, fn func(Block) bool) {
	for _, b := range blocks {
		if !fn(b) {
			continue
		}
		WalkBlocks(Children(b), fn)
		for _, ins := range InlinesOf(b) {
			walkNotes(ins, fn)
		}
	}
}

func walkNotes(ins []Inline, fn func(Block) bool) {
	for _, in := range ins {
		if n, ok := in.(*Note); ok {
			WalkBlocks(n.Blocks, fn)
			continue
		}
		walkNotes(InlineChildren(in), fn)
	}
}

// WalkInlines calls fn for every inline in the document in order, including
// inlines inside footnotes, captions and table cells.
func WalkInlines(blocks []Block, fn func(Inline)) {
	var visit func([]Inline)
	visit = func(ins []Inline) {
		for _, in := range ins {
			fn(in)
			if n, ok := in.(*Note); ok {
				WalkInlines(n.Blocks, fn)
				continue
			}
			visit(InlineChildren(in))
		}
	}
	for _, b := range blocks {
		for _, ins := range InlinesOf(b) {
			visit(ins)
		}
		WalkInlines(Children(b), fn)
	}
}

// PlainText renders inlines as plain text (formatting removed, notes
// dropped, math kept as its TeX source).
func PlainText(ins []Inline) string {
	var sb strings.Builder
	writePlain(&sb, ins)
	return strings.TrimSpace(sb.String())
}

func writePlain(sb *strings.Builder, ins []Inline) {
	for _, in := range ins {
		switch n := in.(type) {
		case *Text:
			sb.WriteString(n.Value)
		case *Code:
			sb.WriteString(n.Text)
		case *Math:
			sb.WriteString(n.TeX)
		case *SoftBreak, *LineBreak:
			sb.WriteByte(' ')
		case *Image:
			sb.WriteString(n.Alt)
		case *Note:
		case *Ref:
			sb.WriteString(n.Target)
		case *RawInline:
		default:
			writePlain(sb, InlineChildren(in))
		}
	}
}

// BlocksText renders blocks as plain text, one paragraph per line.
func BlocksText(blocks []Block) string {
	var parts []string
	WalkBlocks(blocks, func(b Block) bool {
		switch n := b.(type) {
		case *Para:
			parts = append(parts, PlainText(n.Inlines))
		case *Plain:
			parts = append(parts, PlainText(n.Inlines))
		case *Heading:
			parts = append(parts, PlainText(n.Inlines))
		case *CodeBlock:
			parts = append(parts, n.Text)
		}
		return true
	})
	return strings.Join(parts, "\n")
}

// Str converts plain text into inlines, turning newlines into soft breaks.
func Str(s string) []Inline {
	if s == "" {
		return nil
	}
	if !strings.Contains(s, "\n") {
		return []Inline{&Text{Value: s}}
	}
	var out []Inline
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			out = append(out, &SoftBreak{})
		}
		if line != "" {
			out = append(out, &Text{Value: line})
		}
	}
	return out
}

// TrimInlines removes leading and trailing whitespace and breaks.
func TrimInlines(ins []Inline) []Inline {
	for len(ins) > 0 {
		switch n := ins[0].(type) {
		case *SoftBreak, *LineBreak:
			ins = ins[1:]
			continue
		case *Text:
			v := strings.TrimLeft(n.Value, " \t\r\n ")
			if v == "" {
				ins = ins[1:]
				continue
			}
			if v != n.Value {
				ins = append([]Inline{&Text{Value: v}}, ins[1:]...)
			}
		}
		break
	}
	for len(ins) > 0 {
		last := len(ins) - 1
		switch n := ins[last].(type) {
		case *SoftBreak, *LineBreak:
			ins = ins[:last]
			continue
		case *Text:
			v := strings.TrimRight(n.Value, " \t\r\n ")
			if v == "" {
				ins = ins[:last]
				continue
			}
			if v != n.Value {
				ins = append(append([]Inline{}, ins[:last]...), &Text{Value: v})
			}
		}
		break
	}
	return ins
}

// MergeText joins adjacent Text nodes (recursively) to keep trees compact.
func MergeText(ins []Inline) []Inline {
	out := ins[:0:0]
	for _, in := range ins {
		if t, ok := in.(*Text); ok {
			if t.Value == "" {
				continue
			}
			if len(out) > 0 {
				if prev, ok := out[len(out)-1].(*Text); ok {
					out[len(out)-1] = &Text{Value: prev.Value + t.Value}
					continue
				}
			}
		}
		out = append(out, in)
	}
	return out
}
