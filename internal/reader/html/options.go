package html

import (
	"context"
	"strings"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// ConvertOptions configures ReadNode. The hooks let container formats such
// as EPUB map resources, links and ids across several HTML documents.
type ConvertOptions struct {
	// Resources receives decoded data: URI images (and whatever the
	// ResolveImage hook stores). A new set is used when nil.
	Resources *ast.Resources

	// ResolveImage maps an <img> source to the final Image.Src, typically a
	// "res:" reference. "" drops the image (its alt text is kept). When nil,
	// sources are kept as written; data: URIs are always decoded.
	ResolveImage func(src string) string

	// RewriteLink maps an href to the final link URL; "" keeps only the
	// link text. When nil, "#frag" becomes "#"+SanitizeID(frag) and other
	// URLs are kept.
	RewriteLink func(href string) string

	// RewriteID maps an element id to the AST id; "" drops it. Defaults to
	// SanitizeID. It must agree with RewriteLink for internal links.
	RewriteID func(id string) string

	// Index carries footnotes and link targets gathered over several
	// documents. When nil it is built from the converted root alone.
	Index *Index

	// Limits bounds table sizes.
	Limits rd.Limits
}

func (o ConvertOptions) withDefaults() ConvertOptions {
	if o.Resources == nil {
		o.Resources = ast.NewResources()
	}
	if o.RewriteID == nil {
		o.RewriteID = SanitizeID
	}
	if o.RewriteLink == nil {
		o.RewriteLink = defaultRewriteLink
	}
	o.Limits = o.Limits.Normalized()
	return o
}

func defaultRewriteLink(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "#") {
		if id := SanitizeID(href[1:]); id != "" {
			return "#" + id
		}
		return ""
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	return href
}

// Index holds information that must be known before conversion starts:
// footnote bodies, which of them are referenced, and which ids are link
// targets (only those ids become anchors).
type Index struct {
	bodies     map[string]*html.Node
	isBody     map[*html.Node]string
	containers map[*html.Node]bool
	refs       map[string]bool
	targets    map[string]bool
	ids        map[string]*html.Node
	linkTarget map[*html.Node]string
	backrefs   map[*html.Node]bool
	links      []linkInfo
	dirty      bool
}

// NewIndex returns an empty index.
func NewIndex() *Index {
	return &Index{
		bodies:     map[string]*html.Node{},
		isBody:     map[*html.Node]string{},
		containers: map[*html.Node]bool{},
		refs:       map[string]bool{},
		targets:    map[string]bool{},
		ids:        map[string]*html.Node{},
		linkTarget: map[*html.Node]string{},
		backrefs:   map[*html.Node]bool{},
	}
}

// AddNotes records the footnote bodies found below root. With several
// documents, call AddNotes for all of them before AddLinks.
func (x *Index) AddNotes(root *html.Node, opts ConvertOptions) {
	opts = opts.withDefaults()
	x.collectNotes(root, false, opts, 0)
}

func (x *Index) collectNotes(n *html.Node, inContainer bool, opts ConvertOptions, depth int) {
	if depth > maxDepth {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if isNoteBody(c, inContainer) {
			if id := opts.RewriteID(attr(c, "id")); id != "" {
				if _, dup := x.bodies[id]; !dup {
					x.bodies[id] = c
					x.isBody[c] = id
				}
				continue
			}
		}
		container := isNoteContainer(c)
		if container {
			x.containers[c] = true
		}
		x.collectNotes(c, inContainer || container, opts, depth+1)
	}
}

// AddLinks records internal link targets and footnote references below root.
func (x *Index) AddLinks(root *html.Node, opts ConvertOptions) {
	opts = opts.withDefaults()
	x.dirty = true
	var walk func(*html.Node, string, int)
	walk = func(n *html.Node, body string, depth int) {
		if depth > maxDepth {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if id := attr(c, "id"); id != "" {
				if rid := opts.RewriteID(id); rid != "" {
					if _, dup := x.ids[rid]; !dup {
						x.ids[rid] = c
					}
				}
			}
			inner := body
			if id, ok := x.isBody[c]; ok {
				inner = id
			}
			if c.Data == "a" || c.Data == "area" {
				if href := attr(c, "href"); href != "" {
					if u := opts.RewriteLink(href); strings.HasPrefix(u, "#") && len(u) > 1 {
						x.linkTarget[c] = u[1:]
						x.links = append(x.links, linkInfo{node: c, target: u[1:], inBody: inner, backref: isBackref(c)})
					}
				}
			}
			walk(c, inner, depth+1)
		}
	}
	walk(root, "", 0)
}

type linkInfo struct {
	node    *html.Node
	target  string
	inBody  string // id of the footnote body containing the link
	backref bool
}

// finalize resolves the recorded links once all documents are indexed.
func (x *Index) finalize() {
	if !x.dirty {
		return
	}
	x.dirty = false
	for _, l := range x.links {
		if l.backref || l.inBody != "" && x.pointsTo(x.ids[l.target], l.inBody, 0) {
			x.backrefs[l.node] = true
			continue
		}
		x.targets[l.target] = true
		if _, ok := x.bodies[l.target]; ok && l.inBody != l.target {
			x.refs[l.target] = true
		}
	}
}

// pointsTo reports whether n is, or contains, a link to id: a link from a
// footnote to such an element is the note's back-reference.
func (x *Index) pointsTo(n *html.Node, id string, depth int) bool {
	if n == nil || depth > 3 {
		return false
	}
	if x.linkTarget[n] == id {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if x.pointsTo(c, id, depth+1) {
			return true
		}
	}
	return false
}

func isNoteContainer(n *html.Node) bool {
	for _, t := range epubType(n) {
		if t == "footnotes" || t == "endnotes" || t == "rearnotes" {
			return true
		}
	}
	if hasToken(n, "role", "doc-endnotes") || hasToken(n, "role", "doc-footnotes") {
		return true
	}
	for _, cl := range classes(n) {
		switch cl {
		case "footnotes", "footnotes-list", "footnote-list", "footnotes-end-of-document", "references":
			return true
		}
	}
	return attr(n, "id") == "footnotes"
}

func isNoteBody(n *html.Node, inContainer bool) bool {
	if attr(n, "id") == "" {
		return false
	}
	for _, t := range epubType(n) {
		switch t {
		case "footnote", "endnote", "rearnote", "note":
			return true
		}
	}
	if hasToken(n, "role", "doc-footnote") || hasToken(n, "role", "doc-endnote") {
		return true
	}
	if n.Data == "aside" && hasClass(n, "footnote") || n.Data == "table" && hasClass(n, "footnote") {
		return true
	}
	return inContainer && n.Data == "li"
}

// isBackref reports whether a link points from a footnote back to its
// reference.
func isBackref(a *html.Node) bool {
	if hasToken(a, "role", "doc-backlink") || hasEpubType(a, "backlink") {
		return true
	}
	for _, cl := range classes(a) {
		switch cl {
		case "footnote-back", "footnote-backref", "reversefootnote", "fn-backref", "backlink", "footnote-return":
			return true
		}
	}
	href := strings.ToLower(attr(a, "href"))
	if strings.HasPrefix(href, "#fnref") || strings.HasPrefix(href, "#cite_ref") || strings.HasPrefix(href, "#ref-fn") {
		switch normSpace(textContent(a)) {
		case "↩", "↩\uFE0E", "↑", "^", "⤴", "⏎", "", "[return]", "back":
			return true
		}
	}
	return false
}

// ReadNode converts the content of root (an element, typically <body>, or a
// document node) into blocks. Warnings are returned for dropped content.
func ReadNode(ctx context.Context, root *html.Node, opts ConvertOptions) ([]ast.Block, []string) {
	c := newConv(ctx, opts, false)
	return c.run(root), c.warn.List()
}

const maxDepth = 512

type conv struct {
	ctx       context.Context
	opts      ConvertOptions
	idx       *Index
	res       *ast.Resources
	warn      rd.Warnings
	page      bool // strip site chrome (full web pages only)
	cells     int
	display   map[*ast.Math]bool
	skipped   map[*html.Node]bool
	active    map[string]bool // footnotes being converted (cycle guard)
	memo      map[*html.Node]bool
	inHeading int
	inLink    int
	inNote    int
	depth     int
	images    int
}

func newConv(ctx context.Context, opts ConvertOptions, page bool) *conv {
	opts = opts.withDefaults()
	return &conv{
		ctx:     ctx,
		opts:    opts,
		idx:     opts.Index,
		res:     opts.Resources,
		page:    page,
		display: map[*ast.Math]bool{},
		skipped: map[*html.Node]bool{},
		active:  map[string]bool{},
		memo:    map[*html.Node]bool{},
	}
}

func (c *conv) run(root *html.Node) []ast.Block {
	if root == nil {
		return nil
	}
	c.ensureIndex(root)
	if root.Type == html.ElementNode && c.skip(root) {
		return nil
	}
	return c.blocks(root, false)
}

func (c *conv) ensureIndex(root *html.Node) {
	if c.idx == nil {
		c.idx = NewIndex()
		c.idx.AddNotes(root, c.opts)
		c.idx.AddLinks(root, c.opts)
	}
	c.idx.finalize()
}

// keptID returns the AST id for n when something links to it.
func (c *conv) keptID(n *html.Node) string {
	raw := attr(n, "id")
	if raw == "" && n.Data == "a" {
		raw = attr(n, "name")
	}
	if raw == "" {
		return ""
	}
	id := c.opts.RewriteID(raw)
	if id != "" && c.idx.targets[id] {
		if _, isNote := c.idx.bodies[id]; !isNote {
			return id
		}
	}
	return ""
}
