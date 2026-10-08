package html

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
)

func (c *conv) inlineChildren(n *html.Node) []ast.Inline {
	if c.depth > maxDepth {
		return nil
	}
	c.depth++
	defer func() { c.depth-- }()
	var out []ast.Inline
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if c.skip(ch) {
			continue
		}
		out = append(out, c.inline(ch)...)
	}
	return out
}

func (c *conv) inline(n *html.Node) []ast.Inline {
	switch n.Type {
	case html.TextNode:
		if s := collapseWS(n.Data); s != "" {
			return []ast.Inline{&ast.Text{Value: s}}
		}
		return nil
	case html.ElementNode:
	default:
		return nil
	}
	ins := c.inlineElem(n)
	if id := c.keptID(n); id != "" {
		ins = append([]ast.Inline{&ast.Span{Attr: ast.Attr{ID: id}}}, ins...)
	}
	return ins
}

func (c *conv) inlineElem(n *html.Node) []ast.Inline {
	switch n.Data {
	case "br":
		return []ast.Inline{&ast.LineBreak{}}
	case "wbr":
		return nil
	case "strong", "b":
		if w := styleProp(n, "font-weight"); w == "normal" || w == "400" {
			return c.inlineChildren(n)
		}
		return wrap(&ast.Strong{Inlines: c.inlineChildren(n)})
	case "em", "i", "cite", "dfn", "var":
		if styleProp(n, "font-style") == "normal" {
			return c.inlineChildren(n)
		}
		return wrap(&ast.Emph{Inlines: c.inlineChildren(n)})
	case "u", "ins":
		return wrap(&ast.Underline{Inlines: c.inlineChildren(n)})
	case "s", "del", "strike":
		return wrap(&ast.Strike{Inlines: c.inlineChildren(n)})
	case "sup":
		return c.sup(n)
	case "sub":
		return wrap(&ast.Subscript{Inlines: c.inlineChildren(n)})
	case "mark":
		return wrap(&ast.Highlight{Inlines: c.inlineChildren(n)})
	case "code", "tt", "samp":
		return c.code(n)
	case "kbd":
		if hasDescendant(n, "kbd") {
			return c.inlineChildren(n)
		}
		if t := collapseWS(textContent(n)); strings.TrimSpace(t) != "" {
			return []ast.Inline{&ast.Span{Attr: ast.Attr{Classes: []string{"kbd"}}, Inlines: []ast.Inline{&ast.Text{Value: strings.TrimSpace(t)}}}}
		}
		return nil
	case "q":
		ins := []ast.Inline{&ast.Text{Value: "“"}}
		ins = append(ins, c.inlineChildren(n)...)
		return append(ins, &ast.Text{Value: "”"})
	case "a":
		return c.link(n)
	case "img":
		return c.image(n, nil)
	case "picture":
		return c.picture(n)
	case "video", "audio":
		return c.media(n)
	case "math":
		return c.mathML(n)
	case "script":
		return c.scriptMath(n)
	case "mjx-container":
		return c.mathContainer(n)
	case "rt":
		if hasSibling(n, "rp") {
			return c.inlineChildren(n)
		}
		ins := []ast.Inline{&ast.Text{Value: "("}}
		return append(append(ins, c.inlineChildren(n)...), &ast.Text{Value: ")"})
	case "span", "font", "div":
		if ins, ok := c.specialSpan(n); ok {
			return ins
		}
		return c.styled(n, c.inlineChildren(n))
	}
	if blockTags[n.Data] {
		// Block content in an inline-only context (heading, caption, term):
		// keep the text, separated by spaces.
		ins := []ast.Inline{&ast.Text{Value: " "}}
		ins = append(ins, c.inlineChildren(n)...)
		return append(ins, &ast.Text{Value: " "})
	}
	return c.inlineChildren(n)
}

func wrap(in ast.Inline) []ast.Inline {
	if len(ast.InlineChildren(in)) == 0 {
		return nil
	}
	return []ast.Inline{in}
}

func hasDescendant(n *html.Node, tag string) bool {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, tag) || hasDescendant(ch, tag) {
			return true
		}
	}
	return false
}

func hasSibling(n *html.Node, tag string) bool {
	if n.Parent == nil {
		return false
	}
	for ch := n.Parent.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, tag) {
			return true
		}
	}
	return false
}

// sup unwraps footnote references written as <sup><a href="#fn1">1</a></sup>.
func (c *conv) sup(n *html.Node) []ast.Inline {
	kids := c.inlineChildren(n)
	var notes []ast.Inline
	other := false
	for _, k := range kids {
		switch v := k.(type) {
		case *ast.Note:
			notes = append(notes, v)
		case *ast.Text:
			if strings.Trim(v.Value, " ,;[]()") != "" {
				other = true
			}
		case *ast.Span:
			if len(v.Inlines) > 0 {
				other = true
			}
		default:
			other = true
		}
	}
	if len(notes) > 0 && !other {
		return notes
	}
	return wrap(&ast.Superscript{Inlines: kids})
}

func (c *conv) code(n *html.Node) []ast.Inline {
	text := collapseWS(textContent(n))
	if strings.TrimSpace(text) == "" {
		return nil
	}
	code := &ast.Code{Text: text}
	// <code><a href>name</a></code>: keep the link around the code.
	link := soleChildLink(n)
	if link != nil {
		if u := c.opts.RewriteLink(attr(link, "href")); u != "" && safeURL(u) {
			return []ast.Inline{&ast.Link{URL: u, Title: attr(link, "title"), Inlines: []ast.Inline{code}}}
		}
	}
	return []ast.Inline{code}
}

func soleChildLink(n *html.Node) *html.Node {
	var link *html.Node
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.TextNode && strings.TrimSpace(ch.Data) == "" {
			continue
		}
		if isElem(ch, "a") && attr(ch, "href") != "" && link == nil {
			link = ch
			continue
		}
		return nil
	}
	return link
}

// safeURL rejects script and data URLs as link targets.
func safeURL(u string) bool {
	l := strings.ToLower(strings.TrimSpace(u))
	l = strings.Map(func(r rune) rune {
		if r < 0x20 || r == ' ' {
			return -1
		}
		return r
	}, l)
	return !strings.HasPrefix(l, "javascript:") && !strings.HasPrefix(l, "vbscript:") && !strings.HasPrefix(l, "data:")
}

var permalinkClasses = map[string]bool{
	"anchor": true, "headerlink": true, "permalink": true, "hash-link": true,
	"heading-anchor": true, "anchor-link": true, "header-anchor": true,
	"heading-link": true, "anchorjs-link": true, "deep-link": true,
}

func (c *conv) link(n *html.Node) []ast.Inline {
	href := strings.TrimSpace(attr(n, "href"))
	if href == "" || !safeURL(href) {
		return c.inlineChildren(n)
	}
	if isBackref(n) || c.idx.backrefs[n] {
		return nil
	}
	u := c.opts.RewriteLink(href)
	if u == "" {
		return c.inlineChildren(n)
	}
	if strings.HasPrefix(u, "#") {
		id := u[1:]
		if body, ok := c.idx.bodies[id]; ok && c.idx.refs[id] && !c.active[id] {
			if note := c.note(id, body); note != nil {
				return []ast.Inline{note}
			}
			return nil
		}
		if c.inHeading > 0 && isPermalink(n) {
			return nil
		}
	}
	for _, cl := range classes(n) {
		if permalinkClasses[cl] && isPermalink(n) {
			return nil
		}
	}
	c.inLink++
	kids := c.inlineChildren(n)
	c.inLink--
	if !hasContent(kids) {
		return nil
	}
	return []ast.Inline{&ast.Link{URL: u, Title: attr(n, "title"), Inlines: kids}}
}

// hasContent reports whether ins holds anything visible.
func hasContent(ins []ast.Inline) bool {
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			if strings.TrimSpace(n.Value) != "" {
				return true
			}
		case *ast.LineBreak, *ast.SoftBreak:
		case *ast.Span:
			if hasContent(n.Inlines) {
				return true
			}
		default:
			if !setChildren(in, nil) || hasContent(ast.InlineChildren(in)) {
				return true
			}
		}
	}
	return false
}

// isPermalink reports whether a link is a heading self-link decoration.
func isPermalink(a *html.Node) bool {
	switch normSpace(textContent(a)) {
	case "", "#", "¶", "§", "🔗", "link", "∞", "⚓", "permalink":
		return true
	}
	return false
}

func (c *conv) note(id string, body *html.Node) *ast.Note {
	c.active[id] = true
	c.inNote++
	defer func() {
		delete(c.active, id)
		c.inNote--
	}()
	src := body
	if body.Data == "table" {
		// Footnotes written as tables: <tr><td class="label">…</td><td>text</td>
		var last *html.Node
		var find func(*html.Node)
		find = func(x *html.Node) {
			for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
				if isElem(ch, "td") {
					last = ch
				} else if ch.Type == html.ElementNode && ch.Data != "table" {
					find(ch)
				}
			}
		}
		find(body)
		if last != nil {
			src = last
		}
	}
	blocks := c.blocks(src, false)
	if len(blocks) == 0 {
		return nil
	}
	return &ast.Note{Blocks: blocks}
}

// specialSpan handles math renderings and semantic span classes.
func (c *conv) specialSpan(n *html.Node) ([]ast.Inline, bool) {
	cls := classes(n)
	has := func(x string) bool {
		for _, cl := range cls {
			if cl == x {
				return true
			}
		}
		return false
	}
	switch {
	case has("mwe-math-element"):
		// Wiki software hides the MathML and shows an image; the MathML
		// carries the TeX source.
		if m := findElem(n, "math", 0); m != nil {
			display := strings.EqualFold(attr(m, "display"), "block") || findClassPrefix(n, "mwe-math-mathml-display", 0)
			return c.mathInlineFromMathML(m, display), true
		}
		return nil, false
	case has("katex-display"):
		return c.renderedMath(n, true), true
	case has("katex"):
		return c.renderedMath(n, false), true
	case has("math") && (has("inline") || has("display")):
		return c.texSpan(n, has("display")), true
	case has("smallcaps") || styleProp(n, "font-variant") == "small-caps":
		return wrap(&ast.SmallCaps{Inlines: c.inlineChildren(n)}), true
	case has("underline"):
		return wrap(&ast.Underline{Inlines: c.inlineChildren(n)}), true
	case has("mark"):
		return wrap(&ast.Highlight{Inlines: c.inlineChildren(n)}), true
	}
	return nil, false
}

// styled applies inline CSS formatting as written by word processors and
// office suites exporting to HTML.
func (c *conv) styled(n *html.Node, ins []ast.Inline) []ast.Inline {
	if attr(n, "style") == "" || len(ins) == 0 {
		return ins
	}
	if fam := styleProp(n, "font-family"); fam != "" && isMonoFamily(fam) {
		if t := collapseWS(textContent(n)); strings.TrimSpace(t) != "" {
			return []ast.Inline{&ast.Code{Text: t}}
		}
	}
	switch w := styleProp(n, "font-weight"); w {
	case "bold", "bolder", "600", "700", "800", "900":
		ins = []ast.Inline{&ast.Strong{Inlines: ins}}
	}
	if s := styleProp(n, "font-style"); s == "italic" || s == "oblique" {
		ins = []ast.Inline{&ast.Emph{Inlines: ins}}
	}
	deco := styleProp(n, "text-decoration") + " " + styleProp(n, "text-decoration-line")
	if strings.Contains(deco, "underline") && c.inLink == 0 {
		ins = []ast.Inline{&ast.Underline{Inlines: ins}}
	}
	if strings.Contains(deco, "line-through") {
		ins = []ast.Inline{&ast.Strike{Inlines: ins}}
	}
	switch styleProp(n, "vertical-align") {
	case "super":
		ins = []ast.Inline{&ast.Superscript{Inlines: ins}}
	case "sub":
		ins = []ast.Inline{&ast.Subscript{Inlines: ins}}
	}
	return ins
}

func isMonoFamily(fam string) bool {
	first := strings.Trim(strings.TrimSpace(strings.Split(fam, ",")[0]), `"'`)
	for _, m := range []string{"monospace", "courier", "consolas", "menlo", "monaco", "mono", "source code", "fira code", "inconsolata"} {
		if strings.Contains(first, m) {
			return true
		}
	}
	return false
}

// normalize collapses whitespace across inline boundaries, trims the ends
// and drops empty formatting containers.
func normalize(ins []ast.Inline) []ast.Inline {
	space := true
	out := collapseSpaces(ins, &space)
	return trimEnd(out)
}

func collapseSpaces(ins []ast.Inline, space *bool) []ast.Inline {
	out := make([]ast.Inline, 0, len(ins))
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			v := n.Value
			if *space {
				v = strings.TrimLeft(v, " ")
			}
			if v == "" {
				continue
			}
			*space = strings.HasSuffix(v, " ")
			if len(out) > 0 {
				if prev, ok := out[len(out)-1].(*ast.Text); ok {
					out[len(out)-1] = &ast.Text{Value: prev.Value + v}
					continue
				}
			}
			out = append(out, &ast.Text{Value: v})
		case *ast.LineBreak:
			out = trimEnd(out)
			out = append(out, n)
			*space = true
		case *ast.SoftBreak:
			if !*space {
				out = append(out, &ast.Text{Value: " "})
				*space = true
			}
		case *ast.Span:
			kids := collapseSpaces(n.Inlines, space)
			if len(kids) == 0 && n.Attr.ID == "" {
				continue
			}
			n.Inlines = kids
			out = append(out, n)
		case *ast.Note:
			out = append(out, n)
		default:
			if setChildren(in, nil) {
				kids := collapseSpaces(ast.InlineChildren(in), space)
				if !hasContent(kids) {
					// Formatting around nothing but a space: keep the space.
					if len(kids) > 0 && len(out) > 0 {
						if prev, ok := out[len(out)-1].(*ast.Text); ok {
							out[len(out)-1] = &ast.Text{Value: prev.Value + " "}
							continue
						}
					}
					if len(kids) > 0 {
						out = append(out, &ast.Text{Value: " "})
					}
					continue
				}
				setChildren(in, kids)
				out = append(out, in)
				continue
			}
			out = append(out, in)
			*space = false
		}
	}
	return out
}

// setChildren replaces the children of a container inline; with kids == nil
// it only reports whether in is a container.
func setChildren(in ast.Inline, kids []ast.Inline) bool {
	switch n := in.(type) {
	case *ast.Emph:
		if kids != nil {
			n.Inlines = kids
		}
	case *ast.Strong:
		if kids != nil {
			n.Inlines = kids
		}
	case *ast.Strike:
		if kids != nil {
			n.Inlines = kids
		}
	case *ast.Underline:
		if kids != nil {
			n.Inlines = kids
		}
	case *ast.Superscript:
		if kids != nil {
			n.Inlines = kids
		}
	case *ast.Subscript:
		if kids != nil {
			n.Inlines = kids
		}
	case *ast.SmallCaps:
		if kids != nil {
			n.Inlines = kids
		}
	case *ast.Highlight:
		if kids != nil {
			n.Inlines = kids
		}
	case *ast.Link:
		if kids != nil {
			n.Inlines = kids
		}
	default:
		return false
	}
	return true
}

// trimEnd removes trailing spaces and line breaks, descending into the last
// container.
func trimEnd(ins []ast.Inline) []ast.Inline {
	for len(ins) > 0 {
		last := len(ins) - 1
		switch n := ins[last].(type) {
		case *ast.LineBreak, *ast.SoftBreak:
			ins = ins[:last]
			continue
		case *ast.Text:
			v := strings.TrimRight(n.Value, " ")
			if v == "" {
				ins = ins[:last]
				continue
			}
			if v != n.Value {
				ins[last] = &ast.Text{Value: v}
			}
		case *ast.Span:
			if len(n.Inlines) > 0 {
				n.Inlines = trimEnd(n.Inlines)
				if len(n.Inlines) == 0 && n.Attr.ID == "" {
					ins = ins[:last]
					continue
				}
			}
		default:
			if setChildren(ins[last], nil) {
				kids := trimEnd(ast.InlineChildren(ins[last]))
				if len(kids) == 0 {
					ins = ins[:last]
					continue
				}
				setChildren(ins[last], kids)
			}
		}
		break
	}
	return ins
}
