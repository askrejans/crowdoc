package odt

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html/charset"

	"github.com/askrejans/crowdoc/v2/internal/mathconv/mathml"
)

// node is a compact element tree of an OpenDocument XML part. Names are
// canonical "prefix:local" pairs independent of the prefixes the file uses.
type node struct {
	name  string // "" for text nodes
	attrs []attr
	text  string
	kids  []*node

	// MathML elements are converted while parsing.
	tex     string
	display bool
}

type attr struct{ name, val string }

func (n *node) attr(name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.attrs {
		if a.name == name {
			return a.val
		}
	}
	return ""
}

func (n *node) child(name string) *node {
	if n == nil {
		return nil
	}
	for _, k := range n.kids {
		if k.name == name {
			return k
		}
	}
	return nil
}

// find returns the first descendant with the given name (depth-first).
func (n *node) find(name string) *node {
	if n == nil {
		return nil
	}
	for _, k := range n.kids {
		if k.name == name {
			return k
		}
		if f := k.find(name); f != nil {
			return f
		}
	}
	return nil
}

// textContent returns the text below n.
func (n *node) textContent() string {
	if n == nil {
		return ""
	}
	if n.name == "" {
		return n.text
	}
	var sb strings.Builder
	var walk func(*node)
	walk = func(x *node) {
		for _, k := range x.kids {
			switch k.name {
			case "":
				sb.WriteString(k.text)
			case "text:s":
				sb.WriteByte(' ')
			case "text:tab":
				sb.WriteByte('\t')
			case "text:line-break":
				sb.WriteByte('\n')
			case "office:annotation", "text:note-citation":
			default:
				walk(k)
			}
		}
	}
	walk(n)
	return sb.String()
}

var namespaces = map[string]string{
	"urn:oasis:names:tc:opendocument:xmlns:office:1.0":                     "office",
	"urn:oasis:names:tc:opendocument:xmlns:style:1.0":                      "style",
	"urn:oasis:names:tc:opendocument:xmlns:text:1.0":                       "text",
	"urn:oasis:names:tc:opendocument:xmlns:table:1.0":                      "table",
	"urn:oasis:names:tc:opendocument:xmlns:drawing:1.0":                    "draw",
	"urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0":          "fo",
	"http://www.w3.org/1999/xlink":                                         "xlink",
	"http://purl.org/dc/elements/1.1/":                                     "dc",
	"urn:oasis:names:tc:opendocument:xmlns:meta:1.0":                       "meta",
	"urn:oasis:names:tc:opendocument:xmlns:datastyle:1.0":                  "number",
	"urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0":             "svg",
	"urn:oasis:names:tc:opendocument:xmlns:chart:1.0":                      "chart",
	"http://www.w3.org/XML/1998/namespace":                                 "xml",
	"http://www.w3.org/1998/Math/MathML":                                   "math",
	"urn:org:documentfoundation:names:experimental:office:xmlns:loext:1.0": "loext",
	"http://openoffice.org/2009/office":                                    "officeooo",
}

const mathNS = "http://www.w3.org/1998/Math/MathML"

func canonical(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	if p, ok := namespaces[n.Space]; ok {
		return p + ":" + n.Local
	}
	// Undeclared prefixes are left in Space by the decoder.
	if !strings.Contains(n.Space, ":") && !strings.Contains(n.Space, "/") {
		return n.Space + ":" + n.Local
	}
	return "x:" + n.Local
}

const maxDepth = 400

// parseXML builds the element tree of an XML part.
func parseXML(data []byte) (*node, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charset.NewReaderLabel
	root := &node{name: "#document"}
	if err := buildTree(d, root, 0); err != nil && len(root.kids) == 0 {
		return nil, err
	}
	return root, nil
}

func buildTree(d *xml.Decoder, parent *node, depth int) error {
	for {
		tok, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == mathNS && t.Name.Local == "math" {
				tex, display, _ := mathml.ConvertElement(d, t)
				parent.kids = append(parent.kids, &node{name: "math:math", tex: tex, display: display})
				continue
			}
			if depth >= maxDepth {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			n := &node{name: canonical(t.Name)}
			if len(t.Attr) > 0 {
				n.attrs = make([]attr, 0, len(t.Attr))
				for _, a := range t.Attr {
					if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" && a.Name.Space == "" {
						continue
					}
					n.attrs = append(n.attrs, attr{canonical(a.Name), a.Value})
				}
			}
			parent.kids = append(parent.kids, n)
			if err := buildTree(d, n, depth+1); err != nil {
				return err
			}
		case xml.CharData:
			parent.kids = append(parent.kids, &node{text: string(t)})
		case xml.EndElement:
			return nil
		}
	}
}

// rootName returns the canonical name of the first element in data.
func rootName(data []byte) string {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.CharsetReader = charset.NewReaderLabel
	for {
		tok, err := d.Token()
		if err != nil {
			return ""
		}
		if se, ok := tok.(xml.StartElement); ok {
			return canonical(se.Name)
		}
	}
}
