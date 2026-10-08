package rtf

import (
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// inlines converts runs into nested inline nodes; flags in drop are
// removed first (e.g. bold in headings).
func (p *parser) inlines(items []item, drop uint16) []ast.Inline {
	runs := collapseSpaces(items)
	for i := range runs {
		runs[i].f.flags &^= drop
	}
	// Neutral items (notes, anchors) adopt the formatting of the preceding
	// run so they do not split a bold or linked span in two.
	for i := range runs {
		if !runs[i].neutral {
			continue
		}
		if i > 0 {
			runs[i].f = runs[i-1].f
		} else if j := nextNonNeutral(runs, i); j >= 0 {
			runs[i].f = runs[j].f
		}
	}
	var out []ast.Inline
	for i := 0; i < len(runs); {
		j := i + 1
		for j < len(runs) && runs[j].f.link == runs[i].f.link {
			j++
		}
		seg := runs[i:j]
		if id := runs[i].f.link; id > 0 && id <= len(p.links) {
			for k := range seg {
				seg[k].f.flags &^= fUnderline
			}
			li := p.links[id-1]
			lead, inner, trail := splitSpaces(wrapFlags(seg, 0))
			out = append(out, lead...)
			if len(inner) > 0 {
				out = append(out, &ast.Link{URL: li.url, Title: li.title, Inlines: inner})
			}
			out = append(out, trail...)
		} else {
			out = append(out, wrapFlags(seg, 0)...)
		}
		i = j
	}
	return ast.TrimInlines(ast.MergeText(out))
}

func nextNonNeutral(runs []item, i int) int {
	for j := i + 1; j < len(runs); j++ {
		if !runs[j].neutral {
			return j
		}
	}
	return -1
}

// collapseSpaces normalises whitespace across runs the way a word processor
// renders it: tabs become spaces and runs of spaces collapse to one.
func collapseSpaces(items []item) []item {
	out := make([]item, 0, len(items))
	space := true // drop leading spaces
	for _, it := range items {
		if it.in != nil {
			switch {
			case isLineBreak(it.in):
				trimTrailingSpace(out)
				space = true
			case !it.neutral || isNote(it.in):
				space = false // a visible inline separates words
			}
			out = append(out, it)
			continue
		}
		var b []byte
		for _, r := range string(it.text) {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f' {
				if !space {
					b = append(b, ' ')
					space = true
				}
				continue
			}
			space = false
			b = utf8.AppendRune(b, r)
		}
		if len(b) > 0 {
			it.text = b
			out = append(out, it)
		}
	}
	return out
}

func isLineBreak(in ast.Inline) bool { _, ok := in.(*ast.LineBreak); return ok }
func isNote(in ast.Inline) bool      { _, ok := in.(*ast.Note); return ok }

func trimTrailingSpace(out []item) {
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].in != nil {
			if out[i].neutral {
				continue
			}
			return
		}
		out[i].text = []byte(strings.TrimRight(string(out[i].text), " "))
		return
	}
}

func wrapFlags(seg []item, k int) []ast.Inline {
	if k == len(wrapOrder) {
		return leaves(seg)
	}
	flag := wrapOrder[k]
	var out []ast.Inline
	for i := 0; i < len(seg); {
		on := seg[i].f.flags&flag != 0
		j := i + 1
		for j < len(seg) && (seg[j].f.flags&flag != 0) == on {
			j++
		}
		inner := wrapFlags(seg[i:j], k+1)
		if on {
			lead, mid, trail := splitSpaces(inner)
			out = append(out, lead...)
			if len(mid) > 0 {
				out = append(out, wrap(flag, mid))
			}
			out = append(out, trail...)
		} else {
			out = append(out, inner...)
		}
		i = j
	}
	return out
}

func wrap(flag uint16, ins []ast.Inline) ast.Inline {
	switch flag {
	case fBold:
		return &ast.Strong{Inlines: ins}
	case fItalic:
		return &ast.Emph{Inlines: ins}
	case fUnderline:
		return &ast.Underline{Inlines: ins}
	case fStrike:
		return &ast.Strike{Inlines: ins}
	case fSuper:
		return &ast.Superscript{Inlines: ins}
	case fSub:
		return &ast.Subscript{Inlines: ins}
	case fSmallCaps:
		return &ast.SmallCaps{Inlines: ins}
	}
	return &ast.Highlight{Inlines: ins}
}

// leaves turns a run sequence with identical wrapper flags into Text, Code
// and ready-made inlines.
func leaves(seg []item) []ast.Inline {
	var out []ast.Inline
	var buf []byte
	code := false
	flush := func() {
		if len(buf) == 0 {
			return
		}
		if code {
			s := string(buf)
			t := strings.TrimSpace(s)
			if t != "" {
				if strings.HasPrefix(s, " ") {
					out = append(out, &ast.Text{Value: " "})
				}
				out = append(out, &ast.Code{Text: t})
				if strings.HasSuffix(s, " ") {
					out = append(out, &ast.Text{Value: " "})
				}
			} else {
				out = append(out, &ast.Text{Value: " "})
			}
		} else {
			out = append(out, &ast.Text{Value: string(buf)})
		}
		buf = buf[:0]
	}
	for _, it := range seg {
		if it.in != nil {
			flush()
			out = append(out, it.in)
			continue
		}
		if c := it.f.flags&fCode != 0; c != code {
			flush()
			code = c
		}
		buf = append(buf, it.text...)
	}
	flush()
	return out
}

// splitSpaces moves leading and trailing spaces out of a wrapper's content.
func splitSpaces(ins []ast.Inline) (lead, mid, trail []ast.Inline) {
	mid = ins
	if len(mid) > 0 {
		if t, ok := mid[0].(*ast.Text); ok && strings.HasPrefix(t.Value, " ") {
			lead = []ast.Inline{&ast.Text{Value: " "}}
			if v := strings.TrimLeft(t.Value, " "); v != "" {
				mid = append([]ast.Inline{&ast.Text{Value: v}}, mid[1:]...)
			} else {
				mid = mid[1:]
			}
		}
	}
	if n := len(mid); n > 0 {
		if t, ok := mid[n-1].(*ast.Text); ok && strings.HasSuffix(t.Value, " ") {
			trail = []ast.Inline{&ast.Text{Value: " "}}
			if v := strings.TrimRight(t.Value, " "); v != "" {
				mid = append(append([]ast.Inline{}, mid[:n-1]...), &ast.Text{Value: v})
			} else {
				mid = mid[:n-1]
			}
		}
	}
	return lead, mid, trail
}
