// Package epub reads EPUB 2 and EPUB 3 publications into the crowdoc AST.
//
// The OPF package document supplies metadata and the reading order; every
// spine document is converted with the HTML reader, with images resolved
// inside the archive, links between chapters rewritten to internal anchors
// and EPUB footnotes turned into notes.
package epub

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/html"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// spineDoc is one content document in reading order.
type spineDoc struct {
	path   string // archive path
	stem   string // sanitised file stem for id prefixes
	root   *xhtml.Node
	linear bool
	ids    map[string]string // sanitised source id -> document-wide id
	first  string            // id of the first heading (assigned on demand)
}

type reader struct {
	ctx    context.Context
	zip    *rd.Zip
	doc    *ast.Document
	warn   rd.Warnings
	types  map[string]string // archive path -> media type from the manifest
	images map[string]string // archive path -> res: reference
	docs   []*spineDoc
	byPath map[string]*spineDoc
	css    map[string]string // archive path -> stylesheet text
	used   map[string]bool   // ids taken across all documents
	limits rd.Limits
}

// Read parses an EPUB publication.
func Read(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	z, err := rd.OpenZip(data, o.Limits)
	if err != nil {
		return nil, nil, err
	}
	r := &reader{
		ctx:    ctx,
		zip:    z,
		doc:    &ast.Document{Resources: ast.NewResources()},
		types:  map[string]string{},
		images: map[string]string{},
		byPath: map[string]*spineDoc{},
		css:    map[string]string{},
		used:   map[string]bool{},
		limits: o.Limits.Normalized(),
	}
	opfPath, err := r.findOPF()
	if err != nil {
		return nil, nil, err
	}
	opfData, err := z.Read(opfPath)
	if err != nil {
		return nil, nil, err
	}
	var pkg opfPackage
	if err := unmarshalXML(opfData, &pkg); err != nil {
		return nil, nil, fmt.Errorf("epub: invalid package document: %w", err)
	}
	r.readMeta(&pkg.Metadata)
	if err := r.loadSpine(&pkg, opfPath); err != nil {
		return nil, nil, err
	}
	if err := r.convert(); err != nil {
		return nil, nil, err
	}
	html.PruneLinks(r.doc.Blocks)
	return r.doc, r.warn.List(), nil
}

func (r *reader) findOPF() (string, error) {
	if data, err := r.zip.Read("META-INF/container.xml"); err == nil {
		var c container
		if err := unmarshalXML(data, &c); err == nil {
			for _, rf := range c.Rootfiles {
				if rf.FullPath != "" && (rf.MediaType == "" || strings.Contains(rf.MediaType, "oebps-package")) && r.zip.Has(rf.FullPath) {
					return rd.CleanZipPath(rf.FullPath), nil
				}
			}
		}
	} else if errors.Is(err, rd.ErrLimit) {
		return "", err
	}
	for _, name := range r.zip.Names() {
		if strings.HasSuffix(strings.ToLower(name), ".opf") {
			return name, nil
		}
	}
	return "", errors.New("epub: no package document (OPF) found")
}

func (r *reader) readMeta(md *opfMetadata) {
	m := &r.doc.Meta
	for _, t := range md.Titles {
		v := clean(t.Value)
		if v == "" {
			continue
		}
		switch md.refinement(t.ID, "title-type") {
		case "main":
			m.Title = v
		case "subtitle":
			if m.Subtitle == "" {
				m.Subtitle = v
			}
		default:
			if m.Title == "" {
				m.Title = v
			}
		}
	}
	for _, c := range md.Creators {
		name := clean(c.Value)
		role := strings.ToLower(c.Role)
		if role == "" {
			role = strings.ToLower(md.refinement(c.ID, "role"))
		}
		if name != "" && (role == "" || role == "aut") {
			m.Authors = append(m.Authors, ast.Author{Name: name})
		}
	}
	if len(md.Languages) > 0 {
		m.Lang = clean(md.Languages[0])
	}
	for _, d := range md.Dates {
		v := clean(d.Value)
		if v == "" {
			continue
		}
		if m.Date == "" || strings.EqualFold(d.Event, "publication") {
			m.Date = v
		}
	}
	if len(m.Date) > 10 && m.Date[4] == '-' && m.Date[10] == 'T' {
		m.Date = m.Date[:10]
	}
	if len(md.Descriptions) > 0 {
		m.Summary = stripTags(md.Descriptions[0])
	}
	seen := map[string]bool{}
	for _, s := range md.Subjects {
		for _, kw := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
			if kw = clean(kw); kw != "" && !seen[strings.ToLower(kw)] {
				seen[strings.ToLower(kw)] = true
				m.Keywords = append(m.Keywords, kw)
			}
		}
	}
	if len(md.Publishers) > 0 {
		m.Organization = clean(md.Publishers[0])
	}
}

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

// stripTags reduces an HTML description (as written by some tools) to
// plain text.
func stripTags(s string) string {
	if !strings.Contains(s, "<") {
		return clean(s)
	}
	root, err := html.ParseDocument(s)
	if err != nil {
		return clean(s)
	}
	var sb strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
			if c.Type == xhtml.ElementNode && (c.Data == "p" || c.Data == "br" || c.Data == "div") {
				sb.WriteByte(' ')
			}
		}
	}
	walk(root)
	return clean(sb.String())
}

func isContentType(mt, href string) bool {
	switch strings.ToLower(mt) {
	case "application/xhtml+xml", "text/html", "application/xml", "text/xml":
		return true
	case "":
		ext := strings.ToLower(path.Ext(href))
		return ext == ".xhtml" || ext == ".html" || ext == ".htm"
	}
	return false
}

func hrefPath(base, href string) string {
	href, _, _ = strings.Cut(href, "#")
	href, _, _ = strings.Cut(href, "?")
	if u, err := url.PathUnescape(href); err == nil {
		href = u
	}
	return rd.ResolveZipPath(base, href)
}

func (r *reader) loadSpine(pkg *opfPackage, opfPath string) error {
	items := map[string]opfItem{}
	for _, it := range pkg.Manifest {
		items[it.ID] = it
		if it.Href != "" {
			r.types[hrefPath(opfPath, it.Href)] = strings.ToLower(it.MediaType)
		}
	}
	for _, ref := range pkg.Spine.Itemrefs {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		it, ok := items[ref.IDRef]
		if !ok || it.Href == "" {
			r.warn.Addf("epub: spine item %q missing from the manifest", ref.IDRef)
			continue
		}
		if strings.Contains(" "+it.Properties+" ", " nav ") || it.ID == pkg.Spine.Toc || strings.Contains(it.MediaType, "dtbncx") {
			continue
		}
		if !isContentType(it.MediaType, it.Href) {
			r.warn.Addf("epub: spine item %q of type %q skipped", it.Href, it.MediaType)
			continue
		}
		p := hrefPath(opfPath, it.Href)
		if _, dup := r.byPath[p]; dup {
			continue
		}
		data, err := r.zip.Read(p)
		if err != nil {
			if errors.Is(err, rd.ErrLimit) {
				return err
			}
			r.warn.Addf("epub: content document %q unreadable", p)
			continue
		}
		root, err := html.ParseXHTML(decodeText(data))
		if err != nil {
			r.warn.Addf("epub: content document %q unparsable", p)
			continue
		}
		r.stylesheet(p, root).Apply(root)
		stem := strings.TrimSuffix(path.Base(p), path.Ext(p))
		d := &spineDoc{path: p, stem: html.SanitizeID(stem), root: root, linear: ref.Linear != "no"}
		r.docs = append(r.docs, d)
		r.byPath[p] = d
	}
	if len(r.docs) == 0 {
		return errors.New("epub: no readable content documents in the spine")
	}
	return nil
}

// decodeText converts a content document to UTF-8 using its XML
// declaration or <meta charset> when it is not UTF-8 already.
func decodeText(data []byte) string {
	if utf8.Valid(data) {
		return strings.TrimPrefix(string(data), "\uFEFF")
	}
	head := string(data[:min(len(data), 1024)])
	label := ""
	if i := strings.Index(head, "encoding="); i >= 0 {
		rest := head[i+len("encoding="):]
		if len(rest) > 1 && (rest[0] == '"' || rest[0] == '\'') {
			if j := strings.IndexByte(rest[1:], rest[0]); j > 0 {
				label = rest[1 : j+1]
			}
		}
	} else if i := strings.Index(strings.ToLower(head), "charset="); i >= 0 {
		rest := strings.TrimLeft(head[i+len("charset="):], `"'`)
		if j := strings.IndexAny(rest, `"'; />`); j > 0 {
			label = rest[:j]
		}
	}
	if enc, err := htmlindex.Get(label); err == nil && label != "" {
		if out, err := enc.NewDecoder().Bytes(data); err == nil {
			return string(out)
		}
	}
	out, err := charmap.Windows1252.NewDecoder().Bytes(data)
	if err != nil {
		return strings.ToValidUTF8(string(data), "�")
	}
	return string(out)
}

// stylesheet gathers the CSS a content document links to or embeds; class
// rules carry the formatting in many converted books.
func (r *reader) stylesheet(docPath string, root *xhtml.Node) *html.Stylesheet {
	sheet := html.ParseStylesheet("")
	walkElems(root, 0, func(n *xhtml.Node) {
		switch n.Data {
		case "link":
			if !strings.Contains(strings.ToLower(attrOf(n, "rel")), "stylesheet") {
				return
			}
			href := attrOf(n, "href")
			if href == "" {
				return
			}
			if _, external := externalScheme(href); external {
				return
			}
			p := hrefPath(docPath, href)
			css, ok := r.css[p]
			if !ok {
				if data, err := r.zip.Read(p); err == nil {
					css = string(data)
				}
				r.css[p] = css
			}
			sheet.Add(css)
		case "style":
			sheet.Add(textOf(n))
		}
	})
	return sheet
}
