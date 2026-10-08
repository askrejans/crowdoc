package epub

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/askrejans/crowdoc/v2/internal/reader/html"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

const maxDepth = 512

func (r *reader) convert() error {
	for _, d := range r.docs {
		unwrapSVGImages(d.root)
		d.ids = map[string]string{}
		walkElems(d.root, 0, func(n *xhtml.Node) {
			raw := attrOf(n, "id")
			if raw == "" && n.Data == "a" {
				raw = attrOf(n, "name")
			}
			if sid := html.SanitizeID(raw); sid != "" {
				if _, ok := d.ids[sid]; !ok {
					d.ids[sid] = r.unique(sid, d.stem)
				}
			}
		})
	}

	idx := html.NewIndex()
	opts := make([]html.ConvertOptions, len(r.docs))
	for i, d := range r.docs {
		opts[i] = r.options(d, idx)
		idx.AddNotes(bodyOf(d.root), opts[i])
	}
	for i, d := range r.docs {
		idx.AddLinks(bodyOf(d.root), opts[i])
	}

	anyLinear := false
	for _, d := range r.docs {
		anyLinear = anyLinear || d.linear
	}
	for i, d := range r.docs {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		if anyLinear && !d.linear || isCover(d.root) {
			continue
		}
		blocks, warns := html.ReadNode(r.ctx, bodyOf(d.root), opts[i])
		for _, w := range warns {
			r.warn.Addf("%s", w)
		}
		r.doc.Blocks = append(r.doc.Blocks, blocks...)
	}
	return r.ctx.Err()
}

func (r *reader) options(d *spineDoc, idx *html.Index) html.ConvertOptions {
	return html.ConvertOptions{
		Resources: r.doc.Resources,
		Index:     idx,
		Limits:    r.limits,
		RewriteID: func(raw string) string {
			sid := html.SanitizeID(raw)
			if v, ok := d.ids[sid]; ok {
				return v
			}
			return sid
		},
		RewriteLink:  func(href string) string { return r.rewriteLink(d, href) },
		ResolveImage: func(src string) string { return r.resolveImage(d, src) },
	}
}

// unique returns sid, or a file-prefixed variant when another document
// already uses it.
func (r *reader) unique(sid, stem string) string {
	cand := sid
	for n := 1; r.used[cand]; n++ {
		if n == 1 {
			cand = stem + "-" + sid
		} else {
			cand = fmt.Sprintf("%s-%s-%d", stem, sid, n)
		}
	}
	r.used[cand] = true
	return cand
}

func externalScheme(href string) (scheme string, ok bool) {
	u, err := url.Parse(href)
	if err != nil || u.Scheme == "" || len(u.Scheme) == 1 { // "c:" is a path
		return "", false
	}
	return strings.ToLower(u.Scheme), true
}

// rewriteLink maps an href inside document d to an internal anchor
// ("ch2.xhtml#s3" -> "#s3"), keeps web links and drops everything else.
func (r *reader) rewriteLink(d *spineDoc, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if scheme, ok := externalScheme(href); ok {
		switch scheme {
		case "http", "https", "mailto", "ftp", "tel":
			return href
		}
		return ""
	}
	target, frag := d, ""
	if p, f, found := strings.Cut(href, "#"); found {
		frag = f
		if p != "" {
			target = r.byPath[hrefPath(d.path, p)]
		}
	} else {
		target = r.byPath[hrefPath(d.path, href)]
	}
	if target == nil {
		return ""
	}
	if frag != "" {
		if u, err := url.PathUnescape(frag); err == nil {
			frag = u
		}
		if id, ok := target.ids[html.SanitizeID(frag)]; ok {
			return "#" + id
		}
	}
	if id := r.firstID(target); id != "" {
		return "#" + id
	}
	return ""
}

// firstID returns the id of a document's first heading (or first block),
// giving it one when it has none, so links to a whole file have a target.
func (r *reader) firstID(d *spineDoc) string {
	if d.first != "" {
		return d.first
	}
	var target *xhtml.Node
	walkElems(bodyOf(d.root), 0, func(n *xhtml.Node) {
		if target == nil && len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' {
			target = n
		}
	})
	if target == nil {
		walkElems(bodyOf(d.root), 0, func(n *xhtml.Node) {
			if target == nil && (n.Data == "p" || n.Data == "div") && strings.TrimSpace(textOf(n)) != "" {
				target = n
			}
		})
	}
	if target == nil {
		return ""
	}
	if raw := attrOf(target, "id"); raw != "" {
		if id, ok := d.ids[html.SanitizeID(raw)]; ok {
			d.first = id
			return id
		}
	}
	id := r.unique(d.stem, "doc")
	target.Attr = append(target.Attr, xhtml.Attribute{Key: "id", Val: id})
	d.ids[id] = id
	d.first = id
	return id
}

// resolveImage stores an image of the archive as a resource.
func (r *reader) resolveImage(d *spineDoc, src string) string {
	if scheme, ok := externalScheme(src); ok {
		if scheme == "http" || scheme == "https" {
			return src
		}
		return ""
	}
	p := hrefPath(d.path, src)
	if ref, ok := r.images[p]; ok {
		return ref
	}
	data, err := r.zip.Read(p)
	if err != nil {
		if errors.Is(err, rd.ErrLimit) {
			r.warn.Addf("epub: image %q exceeds the size limits and was dropped", p)
		} else {
			r.warn.Addf("epub: image %q not found in the archive", p)
		}
		r.images[p] = ""
		return ""
	}
	mt := r.types[p]
	if mt == "" {
		mt = rd.MediaTypeFromName(p)
	}
	ref := r.doc.Resources.Add(p, mt, data)
	r.images[p] = ref
	return ref
}

// unwrapSVGImages replaces <svg> wrappers around a single raster image
// (the usual EPUB cover markup) with an <img>.
func unwrapSVGImages(root *xhtml.Node) {
	var svgs []*xhtml.Node
	walkElems(root, 0, func(n *xhtml.Node) {
		if n.Data == "svg" {
			svgs = append(svgs, n)
		}
	})
	for _, s := range svgs {
		var images []*xhtml.Node
		hasText := false
		walkElems(s, 0, func(n *xhtml.Node) {
			switch n.Data {
			case "image":
				images = append(images, n)
			case "text":
				hasText = true
			}
		})
		if len(images) != 1 || hasText || s.Parent == nil {
			continue
		}
		href := ""
		for _, a := range images[0].Attr {
			if a.Key == "href" || a.Key == "xlink:href" {
				href = a.Val
			}
		}
		if href == "" {
			continue
		}
		img := &xhtml.Node{Type: xhtml.ElementNode, Data: "img", DataAtom: atom.Img,
			Attr: []xhtml.Attribute{{Key: "src", Val: href}, {Key: "alt", Val: attrOf(s, "aria-label")}}}
		s.Parent.InsertBefore(img, s)
		s.Parent.RemoveChild(s)
	}
}

// isCover reports whether a document shows nothing but one image.
func isCover(root *xhtml.Node) bool {
	body := bodyOf(root)
	if strings.TrimSpace(textOf(body)) != "" {
		return false
	}
	imgs := 0
	walkElems(body, 0, func(n *xhtml.Node) {
		if n.Data == "img" {
			imgs++
		}
	})
	return imgs == 1
}

func bodyOf(root *xhtml.Node) *xhtml.Node {
	var body *xhtml.Node
	walkElems(root, 0, func(n *xhtml.Node) {
		if body == nil && n.Data == "body" {
			body = n
		}
	})
	if body == nil {
		return root
	}
	return body
}

func walkElems(n *xhtml.Node, depth int, fn func(*xhtml.Node)) {
	if depth > maxDepth {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == xhtml.ElementNode {
			fn(c)
			walkElems(c, depth+1, fn)
		}
	}
}

func attrOf(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// textOf returns the text below n, ignoring scripts and styles.
func textOf(n *xhtml.Node) string {
	var sb strings.Builder
	var walk func(*xhtml.Node, int)
	walk = func(x *xhtml.Node, depth int) {
		if depth > maxDepth {
			return
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			switch c.Type {
			case xhtml.TextNode:
				sb.WriteString(c.Data)
			case xhtml.ElementNode:
				if c.Data != "script" && c.Data != "style" && c.Data != "title" {
					walk(c, depth+1)
				}
			}
		}
	}
	walk(n, 0)
	return sb.String()
}
