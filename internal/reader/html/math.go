package html

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/mathconv/mathml"
)

func isMathScript(n *html.Node) bool {
	t := strings.ToLower(strings.TrimSpace(attr(n, "type")))
	return strings.HasPrefix(t, "math/tex") || strings.HasPrefix(t, "math/latex")
}

// mathInline builds a Math inline from TeX of untrusted origin; unsafe TeX
// is kept as code so nothing reaches the engine unchecked.
func (c *conv) mathInline(tex string, display bool) []ast.Inline {
	tex = strings.TrimSpace(tex)
	if tex == "" {
		return nil
	}
	if !mathml.SafeTeX(tex) {
		c.warn.Addf("html: math with unsupported TeX commands kept as text")
		return []ast.Inline{&ast.Code{Text: tex}}
	}
	m := &ast.Math{TeX: tex}
	if display {
		c.display[m] = true
	}
	return []ast.Inline{m}
}

func (c *conv) mathML(n *html.Node) []ast.Inline {
	display := strings.EqualFold(attr(n, "display"), "block") || strings.EqualFold(attr(n, "mode"), "display")
	tex := mathml.Convert(n)
	if tex == "" {
		if t := strings.TrimSpace(collapseWS(textContent(n))); t != "" {
			return []ast.Inline{&ast.Text{Value: t}}
		}
		return nil
	}
	return c.mathInline(tex, display)
}

func (c *conv) mathInlineFromMathML(m *html.Node, display bool) []ast.Inline {
	if tex := mathml.Convert(m); tex != "" {
		return c.mathInline(tex, display)
	}
	return nil
}

func findClassPrefix(n *html.Node, prefix string, depth int) bool {
	if depth > 4 {
		return false
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && (hasClassPrefix(ch, prefix) || findClassPrefix(ch, prefix, depth+1)) {
			return true
		}
	}
	return false
}

// renderedMath reads the MathML (with its TeX annotation) that client-side
// math renderers keep next to their HTML output.
func (c *conv) renderedMath(n *html.Node, display bool) []ast.Inline {
	if m := findElem(n, "math", 0); m != nil {
		if tex := mathml.Convert(m); tex != "" {
			return c.mathInline(tex, display)
		}
	}
	return c.inlineChildren(n)
}

// mathContainer handles <mjx-container> output: data-latex when present,
// otherwise the assistive MathML.
func (c *conv) mathContainer(n *html.Node) []ast.Inline {
	d := strings.ToLower(attr(n, "display"))
	display := d == "true" || d == "block"
	if tex := attr(n, "data-latex"); tex != "" {
		return c.mathInline(tex, display)
	}
	if el := findAttr(n, "data-latex", 0); el != nil {
		return c.mathInline(attr(el, "data-latex"), display)
	}
	if m := findElem(n, "math", 0); m != nil {
		if tex := mathml.Convert(m); tex != "" {
			return c.mathInline(tex, display)
		}
	}
	return nil
}

// scriptMath handles <script type="math/tex; mode=display"> sources.
func (c *conv) scriptMath(n *html.Node) []ast.Inline {
	if !isMathScript(n) {
		return nil
	}
	display := strings.Contains(strings.ToLower(attr(n, "type")), "mode=display")
	var sb strings.Builder
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.TextNode {
			sb.WriteString(ch.Data)
		}
	}
	return c.mathInline(sb.String(), display)
}

// texSpan handles <span class="math inline">\(x\)</span> as written by
// document converters.
func (c *conv) texSpan(n *html.Node, display bool) []ast.Inline {
	tex := strings.TrimSpace(textContent(n))
	for _, d := range [][2]string{{`\(`, `\)`}, {`\[`, `\]`}, {"$$", "$$"}, {"$", "$"}} {
		if strings.HasPrefix(tex, d[0]) && strings.HasSuffix(tex, d[1]) && len(tex) >= len(d[0])+len(d[1]) {
			tex = tex[len(d[0]) : len(tex)-len(d[1])]
			break
		}
	}
	return c.mathInline(tex, display)
}

func findElem(n *html.Node, tag string, depth int) *html.Node {
	if depth > maxDepth {
		return nil
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, tag) {
			return ch
		}
		if f := findElem(ch, tag, depth+1); f != nil {
			return f
		}
	}
	return nil
}

func findAttr(n *html.Node, key string, depth int) *html.Node {
	if depth > maxDepth {
		return nil
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && attr(ch, key) != "" {
			return ch
		}
		if f := findAttr(ch, key, depth+1); f != nil {
			return f
		}
	}
	return nil
}
