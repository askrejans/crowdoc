package html

import (
	"encoding/base64"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
)

type srcCandidate struct {
	url       string
	w         float64 // width descriptor
	x         float64 // density descriptor
	preferred bool    // formats every engine can include
}

// parseSrcset parses a srcset attribute per the HTML candidate grammar,
// which allows commas inside URLs (data: URIs).
func parseSrcset(s string, preferred bool) []srcCandidate {
	var out []srcCandidate
	i := 0
	for i < len(s) {
		for i < len(s) && (isHTMLSpace(s[i]) || s[i] == ',') {
			i++
		}
		start := i
		for i < len(s) && !isHTMLSpace(s[i]) {
			i++
		}
		u := s[start:i]
		desc := ""
		if strings.HasSuffix(u, ",") {
			u = strings.TrimRight(u, ",")
		} else {
			start = i
			for i < len(s) && s[i] != ',' {
				i++
			}
			desc = strings.TrimSpace(s[start:i])
		}
		if u == "" {
			continue
		}
		cand := srcCandidate{url: u, x: 1, preferred: preferred}
		for _, d := range strings.Fields(desc) {
			if len(d) < 2 {
				continue
			}
			v, err := strconv.ParseFloat(d[:len(d)-1], 64)
			if err != nil || v <= 0 {
				continue
			}
			switch d[len(d)-1] {
			case 'w':
				cand.w = v
			case 'x':
				cand.x = v
			}
		}
		out = append(out, cand)
	}
	return out
}

func isHTMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

// best picks the largest width descriptor, else the highest density,
// preferring broadly supported formats.
func best(cands []srcCandidate) string {
	pick := func(onlyPreferred bool) string {
		var bestC *srcCandidate
		for i := range cands {
			cd := &cands[i]
			if onlyPreferred && !cd.preferred {
				continue
			}
			if bestC == nil || cd.w > bestC.w || cd.w == bestC.w && cd.x > bestC.x {
				bestC = cd
			}
		}
		if bestC == nil {
			return ""
		}
		return bestC.url
	}
	if u := pick(true); u != "" {
		return u
	}
	return pick(false)
}

// lazyAttrs hold the real image of lazy-loading scripts.
var lazyAttrs = []string{"data-src", "data-original", "data-lazy-src", "data-url", "data-hi-res-src", "data-full-src"}

func isPlaceholder(src string) bool {
	return src == "" || strings.HasPrefix(src, "data:") && len(src) < 512 || strings.Contains(src, "placeholder") || strings.Contains(src, "blank.gif")
}

func (c *conv) imageSource(img, picture *html.Node) string {
	src := strings.TrimSpace(attr(img, "src"))
	srcset := attr(img, "srcset")
	for _, a := range lazyAttrs {
		if v := strings.TrimSpace(attr(img, a)); v != "" && isPlaceholder(src) {
			src = v
			break
		}
	}
	if v := attr(img, "data-srcset"); v != "" && srcset == "" {
		srcset = v
	}
	var cands []srcCandidate
	if src != "" {
		cands = append(cands, srcCandidate{url: src, x: 1, preferred: true})
	}
	cands = append(cands, parseSrcset(srcset, true)...)
	if picture != nil {
		for ch := picture.FirstChild; ch != nil; ch = ch.NextSibling {
			if !isElem(ch, "source") {
				continue
			}
			if strings.Contains(strings.ToLower(attr(ch, "media")), "dark") {
				continue
			}
			t := strings.ToLower(attr(ch, "type"))
			pref := t == "" || t == "image/png" || t == "image/jpeg" || t == "image/jpg" || t == "image/gif"
			set := attr(ch, "srcset")
			if set == "" {
				set = attr(ch, "data-srcset")
			}
			cands = append(cands, parseSrcset(set, pref)...)
		}
	}
	return best(cands)
}

func (c *conv) picture(n *html.Node) []ast.Inline {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, "img") {
			return c.image(ch, n)
		}
	}
	return nil
}

func (c *conv) image(n, picture *html.Node) []ast.Inline {
	alt := normSpace(attr(n, "alt"))
	altText := func() []ast.Inline {
		if alt == "" {
			return nil
		}
		return []ast.Inline{&ast.Text{Value: alt}}
	}
	w := parseLength(attr(n, "width"))
	if w == "" {
		w = parseLength(styleProp(n, "width"))
	}
	h := parseLength(attr(n, "height"))
	if h == "" {
		h = parseLength(styleProp(n, "height"))
	}
	if (w == "1px" || w == "0px") && (h == "1px" || h == "0px") {
		return nil // tracking pixel
	}
	raw := c.imageSource(n, picture)
	if isFileName(alt, raw) {
		alt = "" // tools that fill alt with the file name
	}
	src := c.resolveImage(raw)
	if src == "" {
		return altText()
	}
	return []ast.Inline{&ast.Image{Src: src, Alt: alt, Title: attr(n, "title"), Width: w, Height: h}}
}

func isFileName(alt, src string) bool {
	if alt == "" || strings.ContainsAny(alt, " \t") {
		return false
	}
	if alt == src || alt == path.Base(src) {
		return true
	}
	ext := strings.ToLower(path.Ext(alt))
	return strings.Contains(alt, "/") && imageExtSet[ext]
}

var imageExtSet = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true, ".bmp": true, ".tif": true, ".tiff": true}

func (c *conv) resolveImage(src string) string {
	if src == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(src), "data:") {
		mt, data, ok := decodeDataURI(src)
		if !ok || len(data) == 0 {
			c.warn.Addf("html: undecodable data: image dropped")
			return ""
		}
		if !strings.HasPrefix(mt, "image/") {
			c.warn.Addf("html: data: resource of type %q dropped", mt)
			return ""
		}
		c.images++
		return c.res.Add("image-"+strconv.Itoa(c.images)+extFor(mt), mt, data)
	}
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}
	if c.opts.ResolveImage != nil {
		return c.opts.ResolveImage(src)
	}
	if !safeURL(src) {
		return ""
	}
	return src
}

var imageExts = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/jpg": ".jpg", "image/gif": ".gif",
	"image/svg+xml": ".svg", "image/webp": ".webp", "image/bmp": ".bmp",
	"image/tiff": ".tif", "image/avif": ".avif", "application/pdf": ".pdf",
}

func extFor(mt string) string {
	if e, ok := imageExts[mt]; ok {
		return e
	}
	return ""
}

// decodeDataURI decodes an RFC 2397 data URI.
func decodeDataURI(s string) (mediaType string, data []byte, ok bool) {
	rest := strings.TrimSpace(s)[len("data:"):]
	meta, payload, found := strings.Cut(rest, ",")
	if !found {
		return "", nil, false
	}
	parts := strings.Split(meta, ";")
	mediaType = strings.ToLower(strings.TrimSpace(parts[0]))
	if mediaType == "" {
		mediaType = "text/plain"
	}
	b64 := false
	for _, p := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(p), "base64") {
			b64 = true
		}
	}
	if !b64 {
		dec, err := url.PathUnescape(payload)
		if err != nil {
			return "", nil, false
		}
		return mediaType, []byte(dec), true
	}
	if strings.Contains(payload, "%") {
		if dec, err := url.PathUnescape(payload); err == nil {
			payload = dec
		}
	}
	payload = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, payload)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if d, err := enc.DecodeString(payload); err == nil {
			return mediaType, d, true
		}
	}
	return "", nil, false
}

// media turns <video>/<audio> into a link to the file.
func (c *conv) media(n *html.Node) []ast.Inline {
	src := strings.TrimSpace(attr(n, "src"))
	if src == "" {
		for ch := n.FirstChild; ch != nil && src == ""; ch = ch.NextSibling {
			if isElem(ch, "source") {
				src = strings.TrimSpace(attr(ch, "src"))
			}
		}
	}
	if src == "" || !safeURL(src) {
		c.warn.Addf("html: <%s> without a usable source dropped", n.Data)
		return nil
	}
	label := attr(n, "title")
	if label == "" {
		u := src
		if p, err := url.Parse(src); err == nil && p.Path != "" {
			u = p.Path
		}
		label = path.Base(u)
	}
	return []ast.Inline{&ast.Link{URL: c.opts.RewriteLink(src), Inlines: []ast.Inline{&ast.Text{Value: label}}}}
}

var captionLabel = regexp.MustCompile(`^(?i)(figure|fig\.|image|img\.|table|tab\.|listing|abbildung|abb\.|tabelle|attēls|tabula|рис\.|рисунок|таблица)\s*[0-9]+(\.[0-9]+)*\s*[:.–—-]\s*`)

// stripLabel removes a "Figure 1:" style label from a caption; numbering is
// generated by the writer.
func stripLabel(ins []ast.Inline) []ast.Inline {
	if len(ins) == 0 {
		return ins
	}
	t, ok := ins[0].(*ast.Text)
	if !ok {
		// The label is often bold: <b>Figure 1:</b> caption.
		if s, ok := ins[0].(*ast.Strong); ok {
			if captionLabel.MatchString(ast.PlainText(s.Inlines) + " ") {
				rest := captionLabel.ReplaceAllString(ast.PlainText(s.Inlines)+" ", "")
				out := ins[1:]
				if strings.TrimSpace(rest) != "" {
					out = append([]ast.Inline{&ast.Strong{Inlines: []ast.Inline{&ast.Text{Value: rest}}}}, out...)
				}
				return normalize(out)
			}
		}
		return ins
	}
	loc := captionLabel.FindStringIndex(t.Value)
	if loc == nil {
		return ins
	}
	rest := t.Value[loc[1]:]
	out := append([]ast.Inline{}, ins[1:]...)
	if rest != "" {
		out = append([]ast.Inline{&ast.Text{Value: rest}}, out...)
	}
	return normalize(out)
}

var figureDivClasses = map[string]bool{"thumb": true, "wp-caption": true, "figure": true, "image-figure": true}

var captionClasses = map[string]bool{
	"thumbcaption": true, "wp-caption-text": true, "caption": true, "figure-caption": true,
	"image-caption": true, "figcaption": true,
}

// isFigureDiv recognises figures built from divs (older wiki thumbnails,
// blog captions) holding exactly one image and a caption.
func isFigureDiv(n *html.Node) bool {
	ok := false
	for _, cl := range classes(n) {
		ok = ok || figureDivClasses[cl]
	}
	if !ok || findCaption(n, 0) == nil {
		return false
	}
	var imgs []*html.Node
	collectElems(n, "img", &imgs, 0)
	return len(imgs) == 1
}

func findCaption(n *html.Node, depth int) *html.Node {
	if depth > 3 {
		return nil
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type != html.ElementNode {
			continue
		}
		for _, cl := range classes(ch) {
			if captionClasses[cl] {
				return ch
			}
		}
		if f := findCaption(ch, depth+1); f != nil {
			return f
		}
	}
	return nil
}

// figure converts <figure>: one image becomes an ast.Figure; tables and
// code listings take the caption; anything else keeps the caption as text.
func (c *conv) figure(n *html.Node) []ast.Block {
	var capNode *html.Node
	hasSVG := false
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, "figcaption") && capNode == nil {
			capNode = ch
		}
		if isElem(ch, "svg") {
			hasSVG = true
		}
	}
	if capNode == nil && n.Data != "figure" {
		capNode = findCaption(n, 0)
	}
	var caption []ast.Inline
	if capNode != nil {
		caption = stripLabel(normalize(c.inlineChildren(capNode)))
		c.skipped[capNode] = true
		defer delete(c.skipped, capNode)
	}
	content := c.blocks(n, false)
	id := c.keptID(n)

	var figs []*ast.Figure
	for _, b := range content {
		if f, ok := b.(*ast.Figure); ok {
			figs = append(figs, f)
		}
	}
	if len(figs) > 0 {
		target := figs[len(figs)-1]
		if len(caption) > 0 {
			target.Caption = caption
		}
		if id != "" && figs[0].Attr.ID == "" {
			figs[0].Attr.ID = id
		}
		return content
	}
	if len(content) == 1 {
		switch b := content[0].(type) {
		case *ast.Table:
			if len(b.Caption) == 0 {
				b.Caption = caption
			}
			if b.Attr.ID == "" {
				b.Attr.ID = id
			}
			return content
		case *ast.CodeBlock:
			b.Caption = caption
			if b.Attr.ID == "" {
				b.Attr.ID = id
			}
			return content
		}
	}
	if hasSVG {
		c.warn.Addf("html: figure with an inline SVG graphic: graphic dropped, caption kept")
	}
	if len(caption) > 0 {
		content = append(content, &ast.Para{Inlines: caption})
	}
	if id != "" {
		content = attachID(content, id)
	}
	return content
}
