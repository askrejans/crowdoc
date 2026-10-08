package html

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// cssProps are the properties that carry meaning for conversion; layout
// properties are ignored.
var cssProps = map[string]bool{
	"font-weight": true, "font-style": true, "text-decoration": true,
	"text-decoration-line": true, "vertical-align": true, "font-variant": true,
	"font-family": true, "display": true, "visibility": true, "text-align": true,
	"top": true, "position": true,
}

// Stylesheet holds the class rules of CSS that matter for conversion.
type Stylesheet struct {
	classes map[string][][2]string
}

// ParseStylesheet extracts simple class rules (".c1", "span.c1") from CSS.
// Office suites and e-book converters express bold, italics and
// superscripts this way instead of with markup.
func ParseStylesheet(css string) *Stylesheet {
	s := &Stylesheet{classes: map[string][][2]string{}}
	s.Add(css)
	return s
}

// Add parses more CSS into the stylesheet.
func (s *Stylesheet) Add(css string) {
	css = stripComments(css)
	for len(css) > 0 {
		open := strings.IndexByte(css, '{')
		if open < 0 {
			return
		}
		sel := strings.TrimSpace(css[:open])
		end := matchBrace(css, open)
		if end < 0 {
			return
		}
		body := css[open+1 : end]
		css = css[end+1:]
		if strings.HasPrefix(sel, "@") {
			continue // media queries, font faces, pages
		}
		decls := parseDecls(body)
		if len(decls) == 0 {
			continue
		}
		for _, one := range strings.Split(sel, ",") {
			if cls := simpleClass(strings.TrimSpace(one)); cls != "" {
				s.classes[cls] = append(s.classes[cls], decls...)
			}
		}
	}
}

func stripComments(css string) string {
	for {
		i := strings.Index(css, "/*")
		if i < 0 {
			return css
		}
		j := strings.Index(css[i+2:], "*/")
		if j < 0 {
			return css[:i]
		}
		css = css[:i] + " " + css[i+2+j+2:]
	}
}

func matchBrace(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// simpleClass returns the class of a selector of the form ".c" or "tag.c".
func simpleClass(sel string) string {
	dot := strings.IndexByte(sel, '.')
	if dot < 0 || strings.IndexByte(sel[dot+1:], '.') >= 0 {
		return ""
	}
	for i := 0; i < dot; i++ {
		if !(sel[i] >= 'a' && sel[i] <= 'z' || sel[i] >= 'A' && sel[i] <= 'Z' || sel[i] >= '0' && sel[i] <= '9') {
			return ""
		}
	}
	cls := sel[dot+1:]
	for i := 0; i < len(cls); i++ {
		c := cls[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return ""
		}
	}
	return cls
}

func parseDecls(body string) [][2]string {
	var out [][2]string
	for _, d := range strings.Split(body, ";") {
		k, v, ok := strings.Cut(d, ":")
		if !ok {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		if cssProps[k] && v != "" {
			out = append(out, [2]string{k, v})
		}
	}
	return out
}

// Apply copies the class rules onto the elements below root as inline
// style (written after any existing inline style, which keeps priority).
func (s *Stylesheet) Apply(root *html.Node) {
	if s == nil || len(s.classes) == 0 {
		return
	}
	var walk func(*html.Node, int)
	walk = func(n *html.Node, depth int) {
		if depth > maxDepth {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if cls := attr(c, "class"); cls != "" {
				s.applyTo(c, strings.Fields(cls))
			}
			walk(c, depth+1)
		}
	}
	walk(root, 0)
}

func (s *Stylesheet) applyTo(n *html.Node, cls []string) {
	var decls [][2]string
	for _, c := range cls {
		decls = append(decls, s.classes[c]...)
	}
	if len(decls) == 0 {
		return
	}
	var sb strings.Builder
	relative, top := false, 0.0
	for _, d := range decls {
		switch d[0] {
		case "position":
			relative = d[1] == "relative"
		case "top":
			top, _ = strconv.ParseFloat(strings.TrimRight(d[1], "abcdefghijklmnopqrstuvwxyz%"), 64)
		default:
			sb.WriteString(d[0] + ":" + d[1] + ";")
		}
	}
	style := sb.String()
	// Raised or lowered text written as a relative offset; it overrides
	// the "vertical-align: baseline" that usually accompanies it.
	if relative && top < 0 {
		style = "vertical-align:super;" + style
	} else if relative && top > 0 {
		style = "vertical-align:sub;" + style
	}
	if style == "" {
		return
	}
	for i, a := range n.Attr {
		if a.Key == "style" {
			n.Attr[i].Val = a.Val + ";" + style
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: "style", Val: style})
}

// pageStyles collects the <style> elements of a page.
func pageStyles(root *html.Node) *Stylesheet {
	s := ParseStylesheet("")
	var walk func(*html.Node, int)
	walk = func(n *html.Node, depth int) {
		if depth > maxDepth {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if isElem(c, "style") {
				s.Add(textOf(c))
				continue
			}
			walk(c, depth+1)
		}
	}
	walk(root, 0)
	return s
}

func textOf(n *html.Node) string {
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
	}
	return sb.String()
}
