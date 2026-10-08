package odt

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/html"
)

// run is a piece of paragraph content with uniform formatting: text or an
// atomic inline (note, image, math, citation, anchor, line break).
type run struct {
	f    flags
	link string
	text string
	hard bool // text:s / text:tab content, kept verbatim in code
	inl  ast.Inline
}

// content is the result of walking a paragraph's inline content.
type content struct {
	runs   []run
	floats []ast.Block // frames anchored outside the text flow
	seq    int         // index in runs of a caption number field, or -1
	ids    []string    // bookmark ids found in the paragraph
}

func (r *reader) inlineContent(p *node, base flags) *content {
	c := &content{seq: -1}
	r.walkInline(p, base, "", c, 0)
	return c
}

func (r *reader) walkInline(n *node, f flags, link string, c *content, depth int) {
	if depth > maxDepth {
		return
	}
	for _, k := range n.kids {
		switch k.name {
		case "":
			if t := collapseODF(k.text); t != "" {
				c.runs = append(c.runs, run{f: f, link: link, text: t})
			}
		case "text:span":
			r.walkInline(k, r.styles.text(k.attr("text:style-name")).over(f), link, c, depth+1)
		case "text:a":
			href := r.linkURL(k.attr("xlink:href"))
			if href == "" {
				href = link
			}
			r.walkInline(k, f, href, c, depth+1)
		case "draw:a":
			r.walkInline(k, f, r.linkURL(k.attr("xlink:href")), c, depth+1)
		case "text:s":
			n, err := strconv.Atoi(k.attr("text:c"))
			if err != nil || n < 1 {
				n = 1
			}
			c.runs = append(c.runs, run{f: f, link: link, text: strings.Repeat(" ", min(n, 200)), hard: true})
		case "text:tab":
			c.runs = append(c.runs, run{f: f, link: link, text: "\t", hard: true})
		case "text:line-break":
			c.runs = append(c.runs, run{f: f, link: link, inl: &ast.LineBreak{}})
		case "text:bookmark", "text:bookmark-start", "text:reference-mark", "text:reference-mark-start":
			if id := html.SanitizeID(k.attr("text:name")); id != "" && r.targets[id] {
				c.ids = append(c.ids, id)
				c.runs = append(c.runs, run{link: link, inl: &ast.Span{Attr: ast.Attr{ID: id}}})
			}
		case "text:note":
			if note := r.note(k); note != nil {
				c.runs = append(c.runs, run{link: link, inl: note})
			}
		case "text:sequence":
			c.seq = len(c.runs)
		case "text:number":
			// rendered list or heading numbers; the writer numbers itself
		case "text:bibliography-mark":
			c.runs = append(c.runs, run{f: f, link: link, inl: r.citation(k)})
		case "draw:frame":
			inl, blocks := r.frame(k)
			if inl != nil {
				if k.attr("text:anchor-type") == "as-char" || isMath(inl) {
					c.runs = append(c.runs, run{link: link, inl: inl})
				} else {
					c.floats = append(c.floats, &ast.Figure{Image: inl.(*ast.Image)})
				}
			}
			c.floats = append(c.floats, blocks...)
		case "math:math":
			if k.tex != "" {
				c.runs = append(c.runs, run{link: link, inl: &ast.Math{TeX: k.tex}})
			}
		case "text:ruby":
			r.walkInline(k.child("text:ruby-base"), f, link, c, depth+1)
			if rt := strings.TrimSpace(k.child("text:ruby-text").textContent()); rt != "" {
				c.runs = append(c.runs, run{f: f, link: link, text: "(" + rt + ")"})
			}
		case "text:hidden-text", "text:hidden-paragraph":
			if k.attr("text:is-hidden") != "true" {
				r.walkInline(k, f, link, c, depth+1)
			}
		case "office:annotation", "office:annotation-end", "text:soft-page-break",
			"text:change", "text:change-start", "text:change-end", "text:bookmark-end",
			"text:reference-mark-end", "text:toc-mark", "text:toc-mark-start", "text:toc-mark-end",
			"text:alphabetical-index-mark", "text:alphabetical-index-mark-start",
			"text:alphabetical-index-mark-end", "text:user-index-mark", "text:user-index-mark-start",
			"text:user-index-mark-end", "text:tracked-changes", "office:event-listeners":
		case "draw:custom-shape", "draw:rect", "draw:ellipse", "draw:polygon", "draw:path",
			"draw:line", "draw:polyline", "draw:connector", "draw:g", "draw:circle", "draw:regular-polygon":
			if blocks := r.shapeText(k); len(blocks) > 0 {
				c.floats = append(c.floats, blocks...)
			} else {
				r.warn.Addf("odt: drawing shape dropped")
			}
		default:
			// Fields (dates, page numbers, references, metadata) show their
			// current value as text content.
			r.walkInline(k, f, link, c, depth+1)
		}
	}
}

func isMath(in ast.Inline) bool {
	_, ok := in.(*ast.Math)
	return ok
}

// collapseODF normalises whitespace as ODF requires: runs of white space
// in text content count as one space.
func collapseODF(s string) string {
	if !strings.ContainsAny(s, "\t\n\r") && !strings.Contains(s, "  ") {
		return s
	}
	var sb strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !space {
				sb.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		sb.WriteRune(r)
	}
	return sb.String()
}

// linkURL maps an xlink:href to a link target.
func (r *reader) linkURL(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "#") {
		name, _ := splitTarget(href[1:])
		if id := html.SanitizeID(name); id != "" {
			return "#" + id
		}
		return ""
	}
	l := strings.ToLower(href)
	if strings.HasPrefix(l, "javascript:") || strings.HasPrefix(l, "vbscript:") || strings.HasPrefix(l, "data:") {
		return ""
	}
	return href
}

// splitTarget splits an internal link target "Name|outline" into the name
// and its kind.
func splitTarget(t string) (name, kind string) {
	if u, err := url.PathUnescape(t); err == nil {
		t = u
	}
	if i := strings.LastIndexByte(t, '|'); i >= 0 {
		return t[:i], t[i+1:]
	}
	return t, ""
}

// inlines turns runs into an inline tree: whitespace collapsed across runs
// and identical formatting merged, so word-processor run splits vanish.
func inlines(runs []run) []ast.Inline {
	return build(tidy(runs))
}

// tidy collapses whitespace across runs, trims the ends and line-break
// edges, and moves formatting off whitespace-only runs and atoms.
func tidy(runs []run) []run {
	out := make([]run, 0, len(runs))
	space := true // at start: drop leading spaces
	for _, rn := range runs {
		if rn.inl != nil {
			switch rn.inl.(type) {
			case *ast.LineBreak:
				out = trimRight(out)
				space = true
				out = append(out, rn)
			case *ast.Span, *ast.Note:
				rn.f = 0
				out = append(out, rn)
			default:
				rn.f &^= fCode
				out = append(out, rn)
				space = false
			}
			continue
		}
		t := strings.ReplaceAll(rn.text, "\t", " ")
		if space {
			t = strings.TrimLeft(t, " ")
		}
		if t == "" {
			continue
		}
		var b strings.Builder
		prev := space
		for _, ch := range t {
			if ch == ' ' {
				if prev {
					continue
				}
				prev = true
			} else {
				prev = false
			}
			b.WriteRune(ch)
		}
		t = b.String()
		space = strings.HasSuffix(t, " ")
		rn.text, rn.hard = t, false
		out = append(out, rn)
	}
	out = trimRight(out)
	// A space between runs takes the formatting both neighbours share, so
	// "bold" " " "text" stays one container and stray formatting on a lone
	// space disappears.
	for i := range out {
		if out[i].inl != nil || strings.TrimSpace(out[i].text) != "" {
			continue
		}
		prev, next := textFlags(out, i, -1), textFlags(out, i, 1)
		out[i].f = prev & next
	}
	return out
}

func textFlags(runs []run, i, step int) flags {
	for j := i + step; j >= 0 && j < len(runs); j += step {
		if runs[j].inl == nil && strings.TrimSpace(runs[j].text) != "" {
			return runs[j].f
		}
	}
	return 0
}

func trimRight(runs []run) []run {
	for len(runs) > 0 {
		last := &runs[len(runs)-1]
		if last.inl != nil {
			if _, ok := last.inl.(*ast.LineBreak); ok {
				runs = runs[:len(runs)-1]
				continue
			}
			return runs
		}
		last.text = strings.TrimRight(last.text, " ")
		if last.text != "" {
			return runs
		}
		runs = runs[:len(runs)-1]
	}
	return runs
}

// build nests runs into formatting containers, choosing at each point the
// flag shared by the longest stretch of runs.
func build(runs []run) []ast.Inline {
	var out []ast.Inline
	for i := 0; i < len(runs); {
		rn := runs[i]
		if rn.link != "" {
			j := i
			for j < len(runs) && runs[j].link == rn.link {
				j++
			}
			inner := make([]run, j-i)
			copy(inner, runs[i:j])
			for k := range inner {
				inner[k].link = ""
				inner[k].f &^= fUnderline // links are not underlined in print
			}
			out = append(out, &ast.Link{URL: rn.link, Inlines: build(inner)})
			i = j
			continue
		}
		if flag, j := longestFlag(runs, i); flag != 0 {
			inner := make([]run, j-i)
			copy(inner, runs[i:j])
			for k := range inner {
				inner[k].f &^= flag
			}
			if kids := build(inner); len(kids) > 0 {
				out = append(out, wrap(flag, kids))
			}
			i = j
			continue
		}
		if rn.f&fCode != 0 && rn.inl == nil {
			var sb strings.Builder
			j := i
			for j < len(runs) && runs[j].link == "" && runs[j].f == fCode && runs[j].inl == nil {
				sb.WriteString(runs[j].text)
				j++
			}
			out = append(out, &ast.Code{Text: sb.String()})
			i = j
			continue
		}
		if rn.inl != nil {
			out = append(out, rn.inl)
		} else if n := len(out); n > 0 {
			if t, ok := out[n-1].(*ast.Text); ok {
				out[n-1] = &ast.Text{Value: t.Value + rn.text}
			} else {
				out = append(out, &ast.Text{Value: rn.text})
			}
		} else {
			out = append(out, &ast.Text{Value: rn.text})
		}
		i++
	}
	return out
}

// longestFlag returns the container flag of runs[i] that covers the most
// consecutive runs, and the end of that stretch.
func longestFlag(runs []run, i int) (flags, int) {
	best, bestEnd := flags(0), i
	for _, fl := range wrapOrder {
		if runs[i].f&fl == 0 {
			continue
		}
		j := i
		for j < len(runs) && runs[j].link == "" && runs[j].f&fl != 0 {
			j++
		}
		if j > bestEnd {
			best, bestEnd = fl, j
		}
	}
	return best, bestEnd
}

func wrap(f flags, kids []ast.Inline) ast.Inline {
	switch f {
	case fBold:
		return &ast.Strong{Inlines: kids}
	case fItalic:
		return &ast.Emph{Inlines: kids}
	case fUnderline:
		return &ast.Underline{Inlines: kids}
	case fStrike:
		return &ast.Strike{Inlines: kids}
	case fSuper:
		return &ast.Superscript{Inlines: kids}
	case fSub:
		return &ast.Subscript{Inlines: kids}
	case fSmallCaps:
		return &ast.SmallCaps{Inlines: kids}
	}
	return &ast.Highlight{Inlines: kids}
}

// plainText renders runs verbatim for code blocks.
func plainText(runs []run) string {
	var sb strings.Builder
	for _, rn := range runs {
		if rn.inl != nil {
			if _, ok := rn.inl.(*ast.LineBreak); ok {
				sb.WriteByte('\n')
			}
			continue
		}
		sb.WriteString(rn.text)
	}
	return sb.String()
}

// allCode reports whether every visible run is monospace.
func allCode(runs []run) bool {
	seen := false
	for _, rn := range runs {
		if rn.inl != nil {
			if _, ok := rn.inl.(*ast.LineBreak); ok {
				continue
			}
			if _, ok := rn.inl.(*ast.Span); ok {
				continue
			}
			return false
		}
		if strings.TrimSpace(rn.text) == "" {
			continue
		}
		if rn.f&fCode == 0 {
			return false
		}
		seen = true
	}
	return seen
}
