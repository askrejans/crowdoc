package odt

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/mathconv/mathml"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// frame converts draw:frame. It returns an inline (image or math) or, for
// text boxes, blocks; caption frames become a captioned figure.
func (r *reader) frame(n *node) (ast.Inline, []ast.Block) {
	if tb := n.child("draw:text-box"); tb != nil {
		return nil, r.textBox(tb, n)
	}
	w, h := length(n.attr("svg:width")), length(n.attr("svg:height"))
	alt := strings.TrimSpace(n.child("svg:title").textContent())
	if alt == "" {
		alt = strings.TrimSpace(n.child("svg:desc").textContent())
	}
	var images []*node
	var object *node
	for _, k := range n.kids {
		switch k.name {
		case "draw:image":
			images = append(images, k)
		case "draw:object", "draw:object-ole":
			if object == nil {
				object = k
			}
		case "draw:plugin", "draw:applet", "draw:floating-frame":
			r.warn.Addf("odt: embedded %s dropped", strings.TrimPrefix(k.name, "draw:"))
		}
	}
	if object != nil {
		if tex, display, ok := r.formula(object); ok {
			_ = display
			return &ast.Math{TeX: tex}, nil
		}
	}
	if len(images) == 0 {
		if object != nil {
			r.warn.Addf("odt: embedded object (chart or OLE) without a replacement image dropped")
		}
		return nil, nil
	}
	src := r.bestImage(images)
	if src == "" {
		if object != nil {
			r.warn.Addf("odt: embedded object (chart or OLE) with an unsupported replacement image dropped")
		}
		return nil, nil
	}
	img := &ast.Image{Src: src, Alt: alt, Width: w, Height: h}
	if id := r.targetID(n.attr("draw:name")); id != "" {
		img.Attr.ID = id
	}
	return img, nil
}

// bestImage resolves the alternative images of a frame, preferring formats
// every engine includes (office suites write SVG with a PNG fallback).
func (r *reader) bestImage(images []*node) string {
	rank := func(mt string) int {
		switch mt {
		case "image/png", "image/jpeg", "image/gif":
			return 0
		case "application/pdf":
			return 1
		case "image/svg+xml":
			return 2
		}
		return 3
	}
	best, bestRank := "", 99
	for _, im := range images {
		src, mt := r.imageData(im)
		if src == "" {
			continue
		}
		if k := rank(mt); k < bestRank {
			best, bestRank = src, k
		}
	}
	return best
}

// imageData stores the bytes of a draw:image and returns its reference.
func (r *reader) imageData(im *node) (src, mediaType string) {
	if bin := im.child("office:binary-data"); bin != nil {
		data, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(bin.textContent()), ""))
		if err != nil || len(data) == 0 {
			r.warn.Addf("odt: undecodable embedded image dropped")
			return "", ""
		}
		mt := sniff(data)
		if !usableImage(mt) {
			r.warn.Addf("odt: embedded image in an unsupported format dropped")
			return "", ""
		}
		r.imageN++
		return r.doc.Resources.Add("Pictures/image-"+strconv.Itoa(r.imageN)+extFor(mt), mt, data), mt
	}
	href := strings.TrimSpace(im.attr("xlink:href"))
	if href == "" {
		return "", ""
	}
	if u, err := url.Parse(href); err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "file") {
		return href, rd.MediaTypeFromName(u.Path)
	}
	// "../picture.png" points next to the document file; the caller resolves
	// it against the document's directory and applies its access rules.
	if strings.HasPrefix(href, "../") {
		return strings.TrimPrefix(href, "../"), rd.MediaTypeFromName(href)
	}
	if r.zip == nil {
		return href, rd.MediaTypeFromName(href)
	}
	p := href
	if u, err := url.PathUnescape(p); err == nil {
		p = u
	}
	p = rd.CleanZipPath(strings.TrimPrefix(p, "./"))
	if ref, ok := r.images[p]; ok {
		return ref, r.images["mt:"+p]
	}
	data, err := r.zip.Read(p)
	if err != nil {
		if errors.Is(err, rd.ErrLimit) {
			r.warn.Addf("odt: image %q exceeds the size limits and was dropped", p)
		} else {
			r.warn.Addf("odt: image %q not found in the document", p)
		}
		r.images[p] = ""
		return "", ""
	}
	mt := sniff(data)
	if mt == "" {
		mt = rd.MediaTypeFromName(p)
	}
	if !usableImage(mt) {
		if !strings.HasPrefix(p, "ObjectReplacements/") { // reported with the object
			r.warn.Addf("odt: image %q in an unsupported format dropped", p)
		}
		r.images[p] = ""
		return "", ""
	}
	ref := r.doc.Resources.Add(p, mt, data)
	r.images[p], r.images["mt:"+p] = ref, mt
	return ref, mt
}

func usableImage(mt string) bool {
	return mt != "" && mt != "image/x-svm"
}

// sniff identifies an image format from its magic bytes.
func sniff(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG")):
		return "image/png"
	case bytes.HasPrefix(b, []byte("\xff\xd8\xff")):
		return "image/jpeg"
	case bytes.HasPrefix(b, []byte("GIF8")):
		return "image/gif"
	case bytes.HasPrefix(b, []byte("%PDF")):
		return "application/pdf"
	case bytes.HasPrefix(b, []byte("BM")):
		return "image/bmp"
	case bytes.HasPrefix(b, []byte("II*\x00")), bytes.HasPrefix(b, []byte("MM\x00*")):
		return "image/tiff"
	case len(b) > 12 && bytes.HasPrefix(b, []byte("RIFF")) && string(b[8:12]) == "WEBP":
		return "image/webp"
	case bytes.HasPrefix(b, []byte("\xd7\xcd\xc6\x9a")):
		return "image/wmf"
	case len(b) > 44 && string(b[40:44]) == " EMF":
		return "image/emf"
	case bytes.HasPrefix(b, []byte("VCLMTF")):
		return "image/x-svm"
	}
	head := b[:min(len(b), 512)]
	if bytes.Contains(head, []byte("<svg")) {
		return "image/svg+xml"
	}
	return ""
}

func extFor(mt string) string {
	switch mt {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	case "application/pdf":
		return ".pdf"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tif"
	case "image/webp":
		return ".webp"
	case "image/wmf":
		return ".wmf"
	case "image/emf":
		return ".emf"
	}
	return ""
}

// formula returns the TeX of an embedded formula object.
func (r *reader) formula(obj *node) (string, bool, bool) {
	if m := obj.find("math:math"); m != nil {
		return m.tex, m.display, m.tex != ""
	}
	href := strings.TrimPrefix(strings.TrimSpace(obj.attr("xlink:href")), "./")
	if href == "" || r.zip == nil {
		return "", false, false
	}
	if u, err := url.PathUnescape(href); err == nil {
		href = u
	}
	p := rd.CleanZipPath(path.Join(href, "content.xml"))
	data, err := r.zip.Read(p)
	if err != nil || !strings.HasSuffix(rootName(data), ":math") && rootName(data) != "math" {
		return "", false, false
	}
	tex, display, err := mathml.ConvertXML(data)
	if err != nil || tex == "" {
		r.warn.Addf("odt: formula object %q could not be converted", href)
		return "", false, false
	}
	return tex, display, true
}

// textBox converts a frame holding a text box. Word processors write captioned
// images as a frame whose text box holds the image frame followed by the
// caption text.
func (r *reader) textBox(tb, outer *node) []ast.Block {
	var paras []*node
	for _, k := range tb.kids {
		if k.name != "" {
			paras = append(paras, k)
		}
	}
	if len(paras) == 1 && (paras[0].name == "text:p" || paras[0].name == "text:h") {
		c := r.inlineContent(paras[0], 0)
		var img ast.Inline
		var rest []run
		restSeq := -1
		for i, rn := range c.runs {
			if i == c.seq {
				restSeq = len(rest)
			}
			if im, ok := rn.inl.(*ast.Image); ok && img == nil {
				img = im
				continue
			}
			rest = append(rest, rn)
		}
		if c.seq == len(c.runs) {
			restSeq = len(rest)
		}
		if img == nil && len(c.floats) == 1 {
			if f, ok := c.floats[0].(*ast.Figure); ok {
				img = f.Image
				c.floats = nil
			}
		}
		if img != nil && len(c.floats) == 0 {
			cap := captionInlines(&content{runs: rest, seq: restSeq})
			fig := &ast.Figure{Image: img.(*ast.Image), Caption: cap}
			if id := r.targetID(outer.attr("draw:name")); id != "" {
				fig.Attr.ID = id
			}
			return []ast.Block{fig}
		}
	}
	return r.blocks(tb)
}

// shapeText returns the paragraphs written inside a drawing shape.
func (r *reader) shapeText(n *node) []ast.Block {
	var out []ast.Block
	for _, k := range n.kids {
		switch k.name {
		case "text:p", "text:h", "text:list":
			out = append(out, r.blocks(&node{kids: []*node{k}})...)
		case "draw:frame":
			if inl, blocks := r.frame(k); inl != nil {
				if img, ok := inl.(*ast.Image); ok {
					out = append(out, &ast.Figure{Image: img})
				}
			} else {
				out = append(out, blocks...)
			}
		default:
			if strings.HasPrefix(k.name, "draw:") {
				out = append(out, r.shapeText(k)...)
			}
		}
	}
	return out
}

// length converts an ODF length ("2.5cm", "12pt", "1in") to points.
func length(v string) string {
	v = strings.TrimSpace(v)
	i := 0
	for i < len(v) && (v[i] >= '0' && v[i] <= '9' || v[i] == '.' || v[i] == '-') {
		i++
	}
	f, err := strconv.ParseFloat(v[:i], 64)
	if err != nil || f <= 0 {
		return ""
	}
	switch strings.TrimSpace(v[i:]) {
	case "cm":
		f *= 72 / 2.54
	case "mm":
		f *= 72 / 25.4
	case "in", "inch":
		f *= 72
	case "pt", "":
	case "pc":
		f *= 12
	case "px":
		f *= 0.75
	case "%":
		return strconv.FormatFloat(f, 'f', -1, 64) + "%"
	default:
		return ""
	}
	return strconv.FormatFloat(float64(int(f*100+0.5))/100, 'f', -1, 64) + "pt"
}
