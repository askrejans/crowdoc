package html

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
)

// calloutKinds maps class names that mark an admonition on their own.
var calloutKinds = map[string]string{
	"note": "note", "warning": "warning", "tip": "tip", "info": "info",
	"important": "important", "caution": "caution", "danger": "danger",
	"hint": "tip", "attention": "warning", "seealso": "note", "warn": "warning",
}

// calloutVariants are only recognised next to a callout marker class
// ("alert alert-success").
var calloutVariants = map[string]string{
	"error": "danger", "success": "tip", "primary": "info", "secondary": "info",
	"notice": "note", "information": "info", "question": "info", "abstract": "note",
	"example": "note", "failure": "danger", "bug": "danger", "quote": "note",
	"todo": "note", "light": "info", "dark": "info",
}

var calloutMarkers = map[string]bool{
	"admonition": true, "callout": true, "alert": true, "markdown-alert": true,
	"theme-admonition": true, "admonitionblock": true, "notification": true,
}

var calloutPrefixes = []string{"markdown-alert-", "theme-admonition-", "admonition-", "callout-", "alert--", "alert-"}

// calloutKind returns the admonition kind of a container, or "".
func calloutKind(n *html.Node) string {
	if !isElem(n, "div", "aside", "section", "blockquote", "details", "p") {
		return ""
	}
	cls := classes(n)
	marker := false
	for _, cl := range cls {
		if k, ok := calloutKinds[cl]; ok {
			return k
		}
		if calloutMarkers[cl] {
			marker = true
		}
	}
	if !marker {
		return ""
	}
	for _, cl := range cls {
		for _, p := range calloutPrefixes {
			if !strings.HasPrefix(cl, p) {
				continue
			}
			v := cl[len(p):]
			if k, ok := calloutKinds[v]; ok {
				return k
			}
			if k, ok := calloutVariants[v]; ok {
				return k
			}
		}
		if k, ok := calloutVariants[cl]; ok {
			return k
		}
	}
	return "note"
}

var calloutTitleClasses = []string{
	"admonition-title", "markdown-alert-title", "callout-title-container", "callout-title",
	"alert-heading", "admonition-heading", "admonitionheading", "title",
}

func isCalloutTitle(n *html.Node) bool {
	for _, cl := range classes(n) {
		for _, t := range calloutTitleClasses {
			if cl == t || strings.HasPrefix(cl, t+"_") {
				return true
			}
		}
	}
	return false
}

// calloutTitle finds the title element among the first descendants.
func calloutTitle(n *html.Node, depth int) *html.Node {
	if depth > 3 {
		return nil
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.TextNode && strings.TrimSpace(ch.Data) != "" {
			return nil
		}
		if ch.Type != html.ElementNode {
			continue
		}
		if isCalloutTitle(ch) {
			return ch
		}
		if t := calloutTitle(ch, depth+1); t != nil {
			return t
		}
		if !isElem(ch, "div", "header", "span") {
			return nil
		}
	}
	return nil
}

func (c *conv) callout(n *html.Node, kind string) []ast.Block {
	div := &ast.Div{Attr: ast.Attr{Classes: []string{kind}}}
	if t := calloutTitle(n, 0); t != nil {
		div.Title = normalize(c.inlineChildren(t))
		c.skipped[t] = true
		defer delete(c.skipped, t)
	}
	if n.Data == "p" {
		div.Blocks = c.paragraphs(c.inlineChildren(n), false)
	} else {
		div.Blocks = c.blocks(n, false)
	}
	if len(div.Blocks) == 0 && len(div.Title) == 0 {
		return nil
	}
	return []ast.Block{div}
}
