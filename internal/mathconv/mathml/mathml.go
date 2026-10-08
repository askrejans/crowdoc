// Package mathml converts Presentation MathML to LaTeX math-mode source.
//
// Two front ends share one converter: Convert takes a <math> tree parsed by
// golang.org/x/net/html (HTML5, EPUB), ConvertXML and ConvertElement take
// XML MathML (OpenDocument formula objects, flat ODT, XHTML). When the
// MathML carries its TeX source in a <semantics> annotation (client-side
// math renderers and wiki software do) that source is returned as is,
// provided it passes SafeTeX.
package mathml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// maxDepth bounds the nesting depth of the element tree; deeper content is
// dropped so hostile input cannot exhaust the stack.
const maxDepth = 200

// node is a minimal MathML element tree shared by both front ends.
type node struct {
	name  string // local element name in lower case; "" for text
	attrs map[string]string
	text  string
	kids  []*node
}

func (n *node) attr(key string) string {
	if n == nil || n.attrs == nil {
		return ""
	}
	return n.attrs[key]
}

// elems returns the element children of n.
func (n *node) elems() []*node {
	var out []*node
	for _, k := range n.kids {
		if k.name != "" {
			out = append(out, k)
		}
	}
	return out
}

// textContent concatenates all descendant text.
func (n *node) textContent() string {
	if n.name == "" {
		return n.text
	}
	var sb strings.Builder
	var walk func(*node)
	walk = func(x *node) {
		for _, k := range x.kids {
			if k.name == "" {
				sb.WriteString(k.text)
			} else {
				walk(k)
			}
		}
	}
	walk(n)
	return sb.String()
}

func localName(s string) string {
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}

// Convert returns the LaTeX for a MathML element parsed by the HTML parser
// (normally a <math> element; any ancestor of one also works). It returns
// "" when nothing convertible is found.
func Convert(n *html.Node) string {
	if n == nil {
		return ""
	}
	tex, _ := convertRoot(fromHTML(n, 0))
	return tex
}

func fromHTML(h *html.Node, depth int) *node {
	n := &node{name: "mrow"}
	switch h.Type {
	case html.TextNode:
		return &node{text: h.Data}
	case html.ElementNode:
		n.name = localName(h.Data)
		if len(h.Attr) > 0 {
			n.attrs = make(map[string]string, len(h.Attr))
			for _, a := range h.Attr {
				n.attrs[localName(a.Key)] = a.Val
			}
		}
	case html.DocumentNode:
	default:
		return nil
	}
	if depth >= maxDepth {
		return n
	}
	for c := h.FirstChild; c != nil; c = c.NextSibling {
		if k := fromHTML(c, depth+1); k != nil {
			n.kids = append(n.kids, k)
		}
	}
	return n
}

// ConvertXML converts an XML MathML document (for example the content.xml
// of an embedded OpenDocument formula). display reports display="block".
func ConvertXML(data []byte) (tex string, display bool, err error) {
	d := newDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", false, errors.New("mathml: no MathML element found")
			}
			return "", false, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return ConvertElement(d, se)
		}
	}
}

func newDecoder(r io.Reader) *xml.Decoder {
	d := xml.NewDecoder(r)
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charset.NewReaderLabel
	return d
}

// ConvertElement converts the MathML element whose start tag was just read
// from d, consuming tokens up to and including its end tag. It lets
// streaming XML readers convert inline <math:math> elements in place.
func ConvertElement(d *xml.Decoder, start xml.StartElement) (tex string, display bool, err error) {
	root, err := fromXML(d, start, 0)
	if err != nil && root == nil {
		return "", false, err
	}
	tex, display = convertRoot(root)
	if tex == "" && err != nil {
		return "", display, err
	}
	return tex, display, nil
}

func fromXML(d *xml.Decoder, start xml.StartElement, depth int) (*node, error) {
	n := &node{name: strings.ToLower(start.Name.Local)}
	if len(start.Attr) > 0 {
		n.attrs = make(map[string]string, len(start.Attr))
		for _, a := range start.Attr {
			n.attrs[strings.ToLower(a.Name.Local)] = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return n, nil
			}
			return n, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth >= maxDepth {
				if err := d.Skip(); err != nil {
					return n, err
				}
				continue
			}
			k, err := fromXML(d, t, depth+1)
			n.kids = append(n.kids, k)
			if err != nil {
				return n, err
			}
		case xml.CharData:
			n.kids = append(n.kids, &node{text: string(t)})
		case xml.EndElement:
			return n, nil
		}
	}
}

// convertRoot finds the <math> element in root and converts it.
func convertRoot(root *node) (string, bool) {
	if root == nil {
		return "", false
	}
	m := findMath(root, 0)
	if m == nil {
		m = root
	}
	display := m.attr("display") == "block" || m.attr("mode") == "display"
	if tex := annotation(m); tex != "" {
		return tex, display
	}
	c := &converter{}
	tex := strings.TrimSpace(c.row(m.kids, state{}))
	if tex == "" {
		if alt := strings.TrimSpace(m.attr("alttext")); alt != "" && SafeTeX(alt) && looksLikeTeX(alt) {
			tex = alt
		}
	}
	return tex, display
}

func findMath(n *node, depth int) *node {
	if n.name == "math" {
		return n
	}
	if depth > maxDepth {
		return nil
	}
	for _, k := range n.kids {
		if k.name == "" {
			continue
		}
		if m := findMath(k, depth+1); m != nil {
			return m
		}
	}
	return nil
}

// annotation returns the TeX source kept in a top-level <semantics>
// annotation, or "".
func annotation(m *node) string {
	n := m
	for i := 0; i < 4 && n != nil; i++ {
		if n.name == "semantics" {
			for _, k := range n.elems() {
				if k.name != "annotation" {
					continue
				}
				if !isTeXEncoding(k.attr("encoding")) {
					continue
				}
				if tex := strings.TrimSpace(k.textContent()); tex != "" && SafeTeX(tex) {
					return tex
				}
			}
			return ""
		}
		els := n.elems()
		if len(els) != 1 {
			return ""
		}
		n = els[0]
	}
	return ""
}

// isTeXEncoding recognises the annotation encodings used for TeX source
// ("application/x-tex", "TeX", "LaTeX", "text/x-latex", ...).
func isTeXEncoding(enc string) bool {
	enc, _, _ = strings.Cut(strings.ToLower(strings.TrimSpace(enc)), ";")
	if i := strings.LastIndexByte(enc, '/'); i >= 0 {
		enc = enc[i+1:]
	}
	enc = strings.TrimPrefix(enc, "x-")
	return enc == "tex" || enc == "latex"
}

// looksLikeTeX reports whether an alttext value is TeX rather than prose.
func looksLikeTeX(s string) bool {
	return strings.ContainsAny(s, `\^_{}=+`) || len([]rune(s)) <= 3
}

// SafeTeX reports whether tex, taken from untrusted input, is free of
// control sequences that could read or write files, run code or redefine
// macros, and has balanced braces.
func SafeTeX(tex string) bool {
	if strings.Contains(tex, "^^") {
		return false
	}
	depth := 0
	for i := 0; i < len(tex); i++ {
		switch tex[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		case '\\':
			j := i + 1
			for j < len(tex) && isASCIILetter(tex[j]) {
				j++
			}
			if j == i+1 {
				i++ // control symbol such as \{ or \\
				continue
			}
			word := tex[i+1 : j]
			if dangerous[word] || strings.HasPrefix(word, "pdf") {
				return false
			}
			if word == "begin" || word == "end" {
				env := envName(tex[j:])
				if dangerous[env] || strings.Contains(env, "luacode") || env == "filecontents*" {
					return false
				}
			}
			i = j - 1
		}
	}
	return depth == 0
}

func envName(s string) string {
	s = strings.TrimLeft(s, " ")
	if !strings.HasPrefix(s, "{") {
		return ""
	}
	if end := strings.IndexByte(s, '}'); end > 0 {
		return strings.TrimSpace(s[1:end])
	}
	return ""
}

func isASCIILetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
