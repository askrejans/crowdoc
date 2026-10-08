package docx

import (
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// fmtFlags is the inline formatting of a text segment.
type fmtFlags uint16

const (
	fBold fmtFlags = 1 << iota
	fItalic
	fUnder
	fStrike
	fSmall
	fHigh
	fSup
	fSub
	fCode
)

// wrapOrder breaks ties when choosing the outermost wrapper.
var wrapOrder = [...]fmtFlags{fBold, fItalic, fUnder, fStrike, fSmall, fHigh, fSup, fSub}

// seg is one piece of paragraph content: formatted text, an inline atom
// (image, note, math, link, citation, line break) or a block-level marker
// (page break, display math, bibliography) that splits the paragraph.
type seg struct {
	f    fmtFlags
	text string
	node ast.Inline
	mark ast.Block
}

func isSpaceText(s seg) bool {
	if s.node != nil || s.mark != nil {
		return false
	}
	return strings.TrimLeft(s.text, " \t\r\n") == ""
}

// commonFlags returns the formatting shared by every non-blank text segment.
func commonFlags(segs []seg) fmtFlags {
	var common fmtFlags
	first := true
	for _, s := range segs {
		if s.node != nil || s.mark != nil || isSpaceText(s) {
			continue
		}
		if first {
			common, first = s.f, false
			continue
		}
		common &= s.f
	}
	return common
}

func clearFlags(segs []seg, f fmtFlags) {
	for i := range segs {
		segs[i].f &^= f
	}
}

// peelSpace splits leading and trailing whitespace text off segs so it can
// be placed outside a wrapping link or citation.
func peelSpace(segs []seg) (lead string, inner []seg, trail string) {
	inner = segs
	for len(inner) > 0 && inner[0].node == nil && inner[0].mark == nil {
		t := inner[0].text
		v := strings.TrimLeft(t, " \t\r\n")
		lead += t[:len(t)-len(v)]
		if v != "" {
			inner = append([]seg{{f: inner[0].f, text: v}}, inner[1:]...)
			break
		}
		inner = inner[1:]
	}
	for len(inner) > 0 {
		last := inner[len(inner)-1]
		if last.node != nil || last.mark != nil {
			break
		}
		v := strings.TrimRight(last.text, " \t\r\n")
		trail = last.text[len(v):] + trail
		if v != "" {
			inner = append(append([]seg{}, inner[:len(inner)-1]...), seg{f: last.f, text: v})
			break
		}
		inner = inner[:len(inner)-1]
	}
	return lead, inner, trail
}

// buildInlines turns segments into a nested inline tree. Whitespace is
// collapsed the way a word processor lays text out; trim also removes it
// at the edges (paragraph level).
func buildInlines(segs []seg, trim bool) []ast.Inline {
	return group(collapse(segs, trim))
}

func collapse(segs []seg, trim bool) []seg {
	out := make([]seg, 0, len(segs))
	prevSpace := trim
	var b strings.Builder
	for _, s := range segs {
		if s.mark != nil {
			continue
		}
		if s.node != nil {
			if _, ok := s.node.(*ast.LineBreak); ok {
				out = trimTrailingSpace(out)
				prevSpace = true
			} else {
				prevSpace = false
			}
			out = append(out, s)
			continue
		}
		b.Reset()
		for _, r := range s.text {
			switch {
			case r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v':
				if !prevSpace {
					b.WriteByte(' ')
					prevSpace = true
				}
				continue
			case r < 0x20 || r == 0x7f || r == utf8.RuneError || r == '\u200b' || r == '\ufeff':
				continue
			}
			b.WriteRune(r)
			prevSpace = false
		}
		if b.Len() > 0 {
			out = append(out, seg{f: s.f, text: b.String()})
		}
	}
	if trim {
		out = trimTrailingSpace(out)
	}
	return out
}

func trimTrailingSpace(out []seg) []seg {
	for len(out) > 0 {
		last := &out[len(out)-1]
		if last.node != nil {
			break
		}
		last.text = strings.TrimRight(last.text, " ")
		if last.text != "" {
			break
		}
		out = out[:len(out)-1]
	}
	return out
}

// group nests formatted segments, opening at each position the wrapper
// whose formatting extends furthest so `**a *b* c**` stays one Strong.
func group(segs []seg) []ast.Inline {
	var out []ast.Inline
	for i := 0; i < len(segs); {
		s := segs[i]
		wrap := s.f &^ fCode
		if wrap == 0 || isSpaceText(s) {
			if s.f&fCode != 0 && s.node == nil {
				j := i
				var sb strings.Builder
				for j < len(segs) && segs[j].node == nil && segs[j].f == s.f {
					sb.WriteString(segs[j].text)
					j++
				}
				if s.f == fCode {
					out = append(out, &ast.Code{Text: sb.String()})
					i = j
					continue
				}
			}
			if s.node != nil {
				out = append(out, s.node)
			} else {
				out = append(out, &ast.Text{Value: s.text})
			}
			i++
			continue
		}
		best, bestLen := fmtFlags(0), 0
		for _, f := range wrapOrder {
			if wrap&f == 0 {
				continue
			}
			n := runLength(segs[i:], f)
			if n > bestLen {
				best, bestLen = f, n
			}
		}
		inner := make([]seg, bestLen)
		copy(inner, segs[i:i+bestLen])
		clearFlags(inner, best)
		out = appendWrapped(out, best, group(inner))
		i += bestLen
	}
	return ast.MergeText(out)
}

// runLength counts the segments carrying f from the start, bridging
// unformatted whitespace between two formatted segments.
func runLength(segs []seg, f fmtFlags) int {
	n := 0
	for j := 0; j < len(segs); j++ {
		if segs[j].f&f != 0 {
			n = j + 1
			continue
		}
		if isSpaceText(segs[j]) && segs[j].f&fCode == 0 {
			continue
		}
		break
	}
	return n
}

// appendWrapped wraps children in the formatting node for f, keeping edge
// whitespace outside the wrapper.
func appendWrapped(out []ast.Inline, f fmtFlags, children []ast.Inline) []ast.Inline {
	var lead, trail string
	if len(children) > 0 {
		if t, ok := children[0].(*ast.Text); ok {
			v := strings.TrimLeft(t.Value, " ")
			lead = t.Value[:len(t.Value)-len(v)]
			if v == "" {
				children = children[1:]
			} else if lead != "" {
				children = append([]ast.Inline{&ast.Text{Value: v}}, children[1:]...)
			}
		}
	}
	if len(children) > 0 {
		if t, ok := children[len(children)-1].(*ast.Text); ok {
			v := strings.TrimRight(t.Value, " ")
			trail = t.Value[len(v):]
			if v == "" {
				children = children[:len(children)-1]
			} else if trail != "" {
				children = append(append([]ast.Inline{}, children[:len(children)-1]...), &ast.Text{Value: v})
			}
		}
	}
	if lead != "" {
		out = append(out, &ast.Text{Value: lead})
	}
	if len(children) > 0 {
		out = append(out, wrapper(f, children))
	}
	if trail != "" {
		out = append(out, &ast.Text{Value: trail})
	}
	return out
}

func wrapper(f fmtFlags, ins []ast.Inline) ast.Inline {
	switch f {
	case fBold:
		return &ast.Strong{Inlines: ins}
	case fItalic:
		return &ast.Emph{Inlines: ins}
	case fUnder:
		return &ast.Underline{Inlines: ins}
	case fStrike:
		return &ast.Strike{Inlines: ins}
	case fSmall:
		return &ast.SmallCaps{Inlines: ins}
	case fHigh:
		return &ast.Highlight{Inlines: ins}
	case fSup:
		return &ast.Superscript{Inlines: ins}
	case fSub:
		return &ast.Subscript{Inlines: ins}
	}
	return &ast.Span{Inlines: ins}
}

// segPlain returns the visible text of a segment.
func segPlain(s seg) string {
	switch n := s.node.(type) {
	case nil:
		return s.text
	case *ast.LineBreak:
		return "\n"
	case *ast.Link:
		return ast.PlainText(n.Inlines)
	case *ast.Cite:
		return ast.PlainText(n.Fallback)
	case *ast.Code:
		return n.Text
	}
	return ""
}

// leadingText concatenates the text at the start of ins up to the first
// non-text node, descending into formatting wrappers.
func leadingText(ins []ast.Inline) (string, bool) {
	var sb strings.Builder
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			sb.WriteString(n.Value)
		case *ast.Code:
			sb.WriteString(n.Text)
		case *ast.Strong, *ast.Emph, *ast.Underline, *ast.Strike, *ast.SmallCaps,
			*ast.Highlight, *ast.Superscript, *ast.Subscript, *ast.Span:
			s, complete := leadingText(ast.InlineChildren(in))
			sb.WriteString(s)
			if !complete {
				return sb.String(), false
			}
		case *ast.Note:
		default:
			return sb.String(), false
		}
	}
	return sb.String(), true
}

// dropPrefix removes the first n bytes of leading text from ins.
func dropPrefix(ins []ast.Inline, n int) []ast.Inline {
	out := make([]ast.Inline, 0, len(ins))
	for i, in := range ins {
		if n <= 0 {
			out = append(out, ins[i:]...)
			break
		}
		switch t := in.(type) {
		case *ast.Text:
			if len(t.Value) <= n {
				n -= len(t.Value)
				continue
			}
			out = append(out, &ast.Text{Value: t.Value[n:]})
			n = 0
		case *ast.Code:
			if len(t.Text) <= n {
				n -= len(t.Text)
				continue
			}
			out = append(out, &ast.Code{Text: t.Text[n:]})
			n = 0
		case *ast.Note:
			out = append(out, in)
		default:
			kids := ast.InlineChildren(in)
			s, _ := leadingText(kids)
			if len(s) <= n && isWrapper(in) {
				n -= len(s)
				if rest := dropPrefix(kids, len(s)); len(rest) > 0 {
					out = append(out, rewrap(in, rest))
				}
				continue
			}
			if isWrapper(in) {
				out = append(out, rewrap(in, dropPrefix(kids, n)))
			} else {
				out = append(out, in)
			}
			n = 0
		}
	}
	return out
}

func isWrapper(in ast.Inline) bool {
	switch in.(type) {
	case *ast.Strong, *ast.Emph, *ast.Underline, *ast.Strike, *ast.SmallCaps,
		*ast.Highlight, *ast.Superscript, *ast.Subscript, *ast.Span:
		return true
	}
	return false
}

func rewrap(in ast.Inline, kids []ast.Inline) ast.Inline {
	switch n := in.(type) {
	case *ast.Strong:
		return &ast.Strong{Inlines: kids}
	case *ast.Emph:
		return &ast.Emph{Inlines: kids}
	case *ast.Underline:
		return &ast.Underline{Inlines: kids}
	case *ast.Strike:
		return &ast.Strike{Inlines: kids}
	case *ast.SmallCaps:
		return &ast.SmallCaps{Inlines: kids}
	case *ast.Highlight:
		return &ast.Highlight{Inlines: kids}
	case *ast.Superscript:
		return &ast.Superscript{Inlines: kids}
	case *ast.Subscript:
		return &ast.Subscript{Inlines: kids}
	case *ast.Span:
		return &ast.Span{Attr: n.Attr, Inlines: kids}
	}
	return in
}

// unwrapStrong removes a Strong that wraps an entire inline list.
func unwrapStrong(ins []ast.Inline) []ast.Inline {
	trimmed := ast.TrimInlines(ins)
	if len(trimmed) == 1 {
		if s, ok := trimmed[0].(*ast.Strong); ok {
			return s.Inlines
		}
	}
	return ins
}
