// Package odt reads OpenDocument Text (.odt packages and flat .fodt XML)
// into the crowdoc AST.
//
// Styles are resolved through their parent chains (automatic styles over
// common styles over defaults) so formatting, headings, quotations, code,
// captions and list numbering follow what the word processor shows.
package odt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/html"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

type reader struct {
	ctx     context.Context
	zip     *rd.Zip // nil for flat XML documents
	doc     *ast.Document
	warn    rd.Warnings
	styles  *styleSet
	limits  rd.Limits
	targets map[string]bool // ids that internal links point to
	images  map[string]string
	refIDs  map[string]bool
	imageN  int
	cells   int

	listCounts map[string]int // list style or xml:id -> last number used
	captions   map[*ast.Para]string
	title      []string
	subtitle   []string
}

// Read parses an OpenDocument text document (zip package or flat XML).
func Read(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	r := &reader{
		ctx:        ctx,
		doc:        &ast.Document{Resources: ast.NewResources()},
		styles:     newStyleSet(),
		limits:     o.Limits.Normalized(),
		targets:    map[string]bool{},
		images:     map[string]string{},
		refIDs:     map[string]bool{},
		listCounts: map[string]int{},
		captions:   map[*ast.Para]string{},
	}
	var content, meta *node
	if bytes.HasPrefix(data, []byte("PK")) {
		z, err := rd.OpenZip(data, o.Limits)
		if err != nil {
			return nil, nil, err
		}
		r.zip = z
		raw, err := z.Read("content.xml")
		if err != nil {
			return nil, nil, fmt.Errorf("odt: %w", err)
		}
		if content, err = parseXML(raw); err != nil {
			return nil, nil, fmt.Errorf("odt: invalid content.xml: %w", err)
		}
		if raw, err := z.Read("styles.xml"); err == nil {
			if st, err := parseXML(raw); err == nil {
				r.styles.loadFonts(st)
				r.styles.loadStyles(st)
			}
		} else if errors.Is(err, rd.ErrLimit) {
			return nil, nil, err
		}
		if raw, err := z.Read("meta.xml"); err == nil {
			meta, _ = parseXML(raw)
		}
	} else {
		var err error
		if content, err = parseXML(data); err != nil {
			return nil, nil, fmt.Errorf("odt: invalid flat XML document: %w", err)
		}
		meta = content
	}
	r.styles.loadFonts(content)
	r.styles.loadStyles(content)

	body := content.find("office:body")
	text := body.child("office:text")
	if text == nil {
		return nil, nil, errors.New("odt: document has no text body")
	}
	r.readMeta(meta)
	r.scanTargets(text, 0)
	r.doc.Blocks = r.blocks(text)
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	html.PruneLinks(r.doc.Blocks)
	if len(r.title) > 0 {
		r.doc.Meta.Title = strings.Join(r.title, " ")
	}
	if len(r.subtitle) > 0 {
		r.doc.Meta.Subtitle = strings.Join(r.subtitle, " ")
	}
	if r.doc.Meta.Lang == "" {
		r.doc.Meta.Lang = r.styles.lang
	}
	return r.doc, r.warn.List(), nil
}

func (r *reader) readMeta(root *node) {
	m := root.find("office:meta")
	if m == nil {
		return
	}
	meta := &r.doc.Meta
	get := func(name string) string { return strings.TrimSpace(collapseODF(m.child(name).textContent())) }
	meta.Title = get("dc:title")
	meta.Subject = get("dc:subject")
	meta.Summary = get("dc:description")
	meta.Lang = get("dc:language")
	author := get("meta:initial-creator")
	if author == "" {
		author = get("dc:creator")
	}
	if author != "" {
		meta.Authors = []ast.Author{{Name: author}}
	}
	date := get("dc:date")
	if date == "" {
		date = get("meta:creation-date")
	}
	if len(date) > 10 && date[10] == 'T' {
		date = date[:10]
	}
	meta.Date = date
	seen := map[string]bool{}
	for _, k := range m.kids {
		if k.name != "meta:keyword" {
			continue
		}
		for _, kw := range strings.FieldsFunc(k.textContent(), func(r rune) bool { return r == ',' || r == ';' }) {
			if kw = strings.TrimSpace(kw); kw != "" && !seen[strings.ToLower(kw)] {
				seen[strings.ToLower(kw)] = true
				meta.Keywords = append(meta.Keywords, kw)
			}
		}
	}
}

// scanTargets records the targets of internal links so that only
// referenced bookmarks become anchors.
func (r *reader) scanTargets(n *node, depth int) {
	if depth > maxDepth {
		return
	}
	for _, k := range n.kids {
		switch k.name {
		case "text:table-of-content", "text:illustration-index", "text:table-index",
			"text:object-index", "text:user-index", "text:alphabetical-index", "text:bibliography":
			continue
		case "text:a", "draw:a":
			if href := k.attr("xlink:href"); strings.HasPrefix(href, "#") {
				name, _ := splitTarget(href[1:])
				if id := html.SanitizeID(name); id != "" {
					r.targets[id] = true
				}
			}
		}
		r.scanTargets(k, depth+1)
	}
}

// targetID returns the anchor id for a named element (table, frame,
// section, heading text) when a link points to it.
func (r *reader) targetID(name string) string {
	if id := html.SanitizeID(name); id != "" && r.targets[id] {
		return id
	}
	return ""
}
