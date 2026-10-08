package docx

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// node is a compact DOM element. ns is a short namespace code ("w", "m",
// "r", "a", ...) for the namespaces the reader understands, the namespace
// URI otherwise, or the bare prefix when a producer forgot to declare it.
// Attribute Name.Space fields carry the same codes.
type node struct {
	ns    string
	name  string
	attrs []xml.Attr
	kids  []*node
	text  string
}

// maxDepth bounds element nesting; deeper subtrees are dropped so hostile
// input cannot exhaust the stack in the recursive converters.
const maxDepth = 512

// nsCodes maps transitional and strict OOXML namespace URIs to short codes.
var nsCodes = map[string]string{
	"http://schemas.openxmlformats.org/wordprocessingml/2006/main":            "w",
	"http://purl.oclc.org/ooxml/wordprocessingml/main":                        "w",
	"http://schemas.openxmlformats.org/officeDocument/2006/math":              "m",
	"http://purl.oclc.org/ooxml/officeDocument/math":                          "m",
	"http://schemas.openxmlformats.org/officeDocument/2006/relationships":     "r",
	"http://purl.oclc.org/ooxml/officeDocument/relationships":                 "r",
	"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing":  "wp",
	"http://purl.oclc.org/ooxml/drawingml/wordprocessingDrawing":              "wp",
	"http://schemas.openxmlformats.org/drawingml/2006/main":                   "a",
	"http://purl.oclc.org/ooxml/drawingml/main":                               "a",
	"http://schemas.openxmlformats.org/drawingml/2006/picture":                "pic",
	"http://purl.oclc.org/ooxml/drawingml/picture":                            "pic",
	"http://schemas.openxmlformats.org/drawingml/2006/chart":                  "c",
	"http://purl.oclc.org/ooxml/drawingml/chart":                              "c",
	"http://schemas.openxmlformats.org/drawingml/2006/diagram":                "dgm",
	"http://purl.oclc.org/ooxml/drawingml/diagram":                            "dgm",
	"http://schemas.openxmlformats.org/markup-compatibility/2006":             "mc",
	"urn:schemas-microsoft-com:vml":                                           "v",
	"urn:schemas-microsoft-com:office:office":                                 "o",
	"urn:schemas-microsoft-com:office:word":                                   "w10",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingShape":       "wps",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingGroup":       "wpg",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas":      "wpc",
	"http://schemas.microsoft.com/office/word/2010/wordml":                    "w14",
	"http://schemas.microsoft.com/office/word/2012/wordml":                    "w15",
	"http://schemas.openxmlformats.org/package/2006/relationships":            "pr",
	"http://schemas.openxmlformats.org/package/2006/metadata/core-properties": "cp",
	"http://purl.org/dc/elements/1.1/":                                        "dc",
	"http://purl.org/dc/terms/":                                               "dcterms",
	"http://schemas.openxmlformats.org/officeDocument/2006/bibliography":      "b",
	"http://purl.oclc.org/ooxml/officeDocument/bibliography":                  "b",
}

func nsCode(uri string) string {
	if c, ok := nsCodes[uri]; ok {
		return c
	}
	return uri
}

func newNode(t xml.StartElement) *node {
	n := &node{ns: nsCode(t.Name.Space), name: t.Name.Local}
	if len(t.Attr) > 0 {
		n.attrs = make([]xml.Attr, 0, len(t.Attr))
		for _, a := range t.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				continue
			}
			a.Name.Space = nsCode(a.Name.Space)
			n.attrs = append(n.attrs, a)
		}
	}
	return n
}

// newDecoder returns a lenient decoder for an OOXML part. UTF-16 parts
// (some producers write custom XML that way) are converted up front.
func newDecoder(data []byte) *xml.Decoder {
	dec := xml.NewDecoder(bytes.NewReader(toUTF8(data)))
	dec.Strict = false
	dec.CharsetReader = charsetReader
	return dec
}

// readElement builds the subtree rooted at start.
func readElement(dec *xml.Decoder, start xml.StartElement) (*node, error) {
	root := newNode(start)
	stack := []*node{root}
	skip := 0
	for len(stack) > 0 {
		tok, err := dec.Token()
		if err != nil {
			return root, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if skip > 0 || len(stack) >= maxDepth {
				skip++
				continue
			}
			n := newNode(t)
			p := stack[len(stack)-1]
			p.kids = append(p.kids, n)
			stack = append(stack, n)
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if skip == 0 {
				top := stack[len(stack)-1]
				top.text += string(t)
			}
		}
	}
	return root, nil
}

// parseXML parses a whole part and returns its root element.
func parseXML(data []byte) (*node, error) {
	dec := newDecoder(data)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			n, err := readElement(dec, se)
			if err != nil && err != io.EOF {
				return n, err
			}
			return n, nil
		}
	}
}

func toUTF8(data []byte) []byte {
	switch {
	case len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF:
		return data[3:]
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE:
		return decodeUTF16(data[2:], false)
	case len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF:
		return decodeUTF16(data[2:], true)
	case len(data) >= 2 && data[0] == '<' && data[1] == 0:
		return decodeUTF16(data, false)
	case len(data) >= 2 && data[0] == 0 && data[1] == '<':
		return decodeUTF16(data, true)
	}
	return data
}

func decodeUTF16(b []byte, bigEndian bool) []byte {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if bigEndian {
			u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			u = append(u, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	runes := utf16.Decode(u)
	out := make([]byte, 0, len(runes))
	for _, r := range runes {
		out = utf8.AppendRune(out, r)
	}
	return out
}

// charsetReader accepts the encodings OOXML producers declare. UTF-16 was
// already converted by toUTF8; single-byte Latin encodings are widened.
func charsetReader(label string, r io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "iso-8859-1", "latin1", "latin-1", "windows-1252", "cp1252", "us-ascii", "ascii":
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 0, len(data))
		for _, c := range data {
			out = utf8.AppendRune(out, rune(c))
		}
		return bytes.NewReader(out), nil
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// node accessors (all nil-safe)

func (n *node) is(ns, name string) bool { return n != nil && n.name == name && n.ns == ns }

// child returns the first child with the given local name in any namespace.
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

// childNS returns the first child with the given namespace and local name.
func (n *node) childNS(ns, name string) *node {
	if n == nil {
		return nil
	}
	for _, k := range n.kids {
		if k.name == name && k.ns == ns {
			return k
		}
	}
	return nil
}

func (n *node) childrenNamed(name string) []*node {
	if n == nil {
		return nil
	}
	var out []*node
	for _, k := range n.kids {
		if k.name == name {
			out = append(out, k)
		}
	}
	return out
}

// attr returns the first attribute with the given local name.
func (n *node) attr(name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func (n *node) hasAttr(name string) bool {
	if n == nil {
		return false
	}
	for _, a := range n.attrs {
		if a.Name.Local == name {
			return true
		}
	}
	return false
}

// attrNS returns the attribute with the given namespace code and local name.
func (n *node) attrNS(ns, name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.attrs {
		if a.Name.Local == name && a.Name.Space == ns {
			return a.Value
		}
	}
	return ""
}

// val returns the val attribute of the named child (w:pStyle/@w:val, ...).
func (n *node) val(name string) string { return n.child(name).attr("val") }

// find returns the first descendant (depth-first) with the local name.
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

// textContent concatenates the character data of n and its descendants.
func (n *node) textContent() string {
	if n == nil {
		return ""
	}
	if len(n.kids) == 0 {
		return n.text
	}
	var sb strings.Builder
	n.writeText(&sb)
	return sb.String()
}

func (n *node) writeText(sb *strings.Builder) {
	sb.WriteString(n.text)
	for _, k := range n.kids {
		k.writeText(sb)
	}
}

// onOff evaluates an ST_OnOff element: present without a value means on.
func onOff(n *node) bool {
	if n == nil {
		return false
	}
	if !n.hasAttr("val") {
		return true
	}
	switch strings.ToLower(n.attr("val")) {
	case "1", "on", "true":
		return true
	}
	return false
}

// understoodRequires lists the markup-compatibility prefixes whose
// mc:Choice content this reader can interpret.
var understoodRequires = map[string]bool{
	"wps": true, "wpg": true, "wpc": true, "wp14": true, "w14": true, "w15": true,
	"a14": true, "v": true, "o": true, "w10": true, "m": true, "a": true,
	"pic": true, "r": true, "w": true, "wp": true,
}

// chooseAlternate picks the branch of an mc:AlternateContent element to
// process: the first understood mc:Choice, else mc:Fallback.
func chooseAlternate(ac *node) *node {
	for _, k := range ac.kids {
		if k.name != "Choice" {
			continue
		}
		ok := true
		for _, req := range strings.Fields(k.attr("Requires")) {
			if !understoodRequires[req] {
				ok = false
				break
			}
		}
		if ok {
			return k
		}
	}
	return ac.child("Fallback")
}

// walk visits the descendants of n depth-first, following only the chosen
// branch of mc:AlternateContent. fn returns false to skip a subtree.
func walk(n *node, fn func(*node) bool) {
	if n == nil {
		return
	}
	for _, k := range n.kids {
		if k.name == "AlternateContent" {
			if alt := chooseAlternate(k); alt != nil {
				walk(alt, fn)
			}
			continue
		}
		if fn(k) {
			walk(k, fn)
		}
	}
}
