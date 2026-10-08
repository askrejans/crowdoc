// Package docx reads WordprocessingML documents (.docx, .docm, .dotx) into
// the crowdoc AST.
//
// The main document part is streamed one body block at a time; styles,
// numbering, notes and relationships are parsed up front. Citation fields
// written by common reference managers and by the word processor's own
// citation feature become ast.Cite nodes with structured references.
package docx

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// Read parses a WordprocessingML document and returns it with non-fatal
// warnings about content that could not be represented.
func Read(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	z, err := rd.OpenZip(data, o.Limits)
	if err != nil {
		return nil, nil, fmt.Errorf("docx: %w", err)
	}
	r := newReader(ctx, z)
	if err := r.read(); err != nil {
		return nil, r.warn.List(), err
	}
	return r.doc, r.warn.List(), nil
}

// rel is one package relationship.
type rel struct {
	typ      string
	target   string // resolved archive path, or the raw URL when external
	external bool
}

// part is an XML part with its relationships (needed to resolve hyperlink
// and image references inside it).
type part struct {
	path string
	rels map[string]rel
}

type reader struct {
	ctx  context.Context
	z    *rd.Zip
	warn rd.Warnings
	doc  *ast.Document
	err  error

	main   *part
	styles *styleSheet
	num    *numbering

	notes     map[string]*node // "footnote:3", "endnote:2"
	noteParts map[string]*part // "footnote", "endnote"
	notesBusy map[string]bool

	// Citations.
	sources   map[string]ast.Reference // word processor bibliography sources by tag
	refIndex  map[string]int           // reference key -> index in doc.References
	refIdent  map[string]string        // reference identity -> key
	usedKeys  map[string]string        // key -> identity
	refSerial int

	// Bookmarks and internal links.
	referenced map[string]bool   // bookmark names targeted by links or fields
	bmIDs      map[string]string // bookmark name -> sanitised id
	idOwner    map[string]string // sanitised id -> bookmark name
	alias      map[string]string // id -> id of the element that carries it
	links      []*ast.Link       // internal links, rewritten through alias at the end

	media     map[string]string // archive path -> "res:" source
	charts    map[string]bool
	langCount map[string]int
	titles    []string
	subtitles []string
	// titleClosed is set once body content follows the title paragraphs;
	// later Title-styled paragraphs are section titles, not the document's.
	titleClosed bool
}

func newReader(ctx context.Context, z *rd.Zip) *reader {
	return &reader{
		ctx:        ctx,
		z:          z,
		doc:        &ast.Document{Resources: ast.NewResources()},
		notes:      map[string]*node{},
		noteParts:  map[string]*part{},
		notesBusy:  map[string]bool{},
		sources:    map[string]ast.Reference{},
		refIndex:   map[string]int{},
		refIdent:   map[string]string{},
		usedKeys:   map[string]string{},
		referenced: map[string]bool{},
		bmIDs:      map[string]string{},
		idOwner:    map[string]string{},
		alias:      map[string]string{},
		media:      map[string]string{},
		charts:     map[string]bool{},
		langCount:  map[string]int{},
	}
}

func (r *reader) read() error {
	pkg := r.readRels("")
	mainPath := ""
	for _, rl := range sortedRels(pkg) {
		if strings.HasSuffix(rl.typ, "/officeDocument") && !rl.external && r.z.Has(rl.target) {
			mainPath = rl.target
			break
		}
	}
	if mainPath == "" {
		if !r.z.Has("word/document.xml") {
			return errors.New("docx: missing word/document.xml (not a Word document?)")
		}
		mainPath = "word/document.xml"
	}
	r.main = &part{path: mainPath, rels: r.readRels(mainPath)}

	r.styles = parseStyles(r.optionalPart(r.relTarget(r.main, "/styles", "word/styles.xml")))
	r.num = parseNumbering(r.optionalPart(r.relTarget(r.main, "/numbering", "word/numbering.xml")), r.styles)
	r.readCore(pkg)
	r.readSources()

	var noteData [][]byte
	for _, kind := range []string{"footnote", "endnote"} {
		p := r.relTarget(r.main, "/"+kind+"s", "word/"+kind+"s.xml")
		data := r.optionalBytes(p)
		if data == nil {
			continue
		}
		noteData = append(noteData, data)
		root, err := parseXML(data)
		if err != nil && root == nil {
			r.warn.Addf("%ss part is malformed and was skipped", kind)
			continue
		}
		r.noteParts[kind] = &part{path: p, rels: r.readRels(p)}
		for _, n := range root.childrenNamed(kind) {
			switch n.attr("type") {
			case "separator", "continuationSeparator", "continuationNotice":
				continue
			}
			r.notes[kind+":"+n.attr("id")] = n
		}
	}

	data, err := r.z.Read(r.main.path)
	if err != nil {
		return fmt.Errorf("docx: reading %s: %w", r.main.path, err)
	}
	r.scanReferences(data)
	for _, d := range noteData {
		r.scanReferences(d)
	}

	items, err := r.body(data)
	if err != nil {
		return err
	}
	if r.err != nil {
		return r.err
	}
	r.promoteFakeHeadings(items)
	blocks := r.assemble(items, blockCtx{top: true})
	if r.err != nil {
		return r.err
	}
	r.doc.Blocks = r.finishBlocks(blocks)
	r.finishMeta()
	r.rewriteLinks()
	return nil
}

// body streams the w:body children of the main document part.
func (r *reader) body(data []byte) ([]item, error) {
	dec := newDecoder(data)
	s := r.newStory(r.main, true)
	var items []item
	inBody := false
	for {
		if err := r.ctx.Err(); err != nil {
			return nil, err
		}
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if !inBody || len(items) == 0 {
				return nil, fmt.Errorf("docx: malformed %s: %w", r.main.path, err)
			}
			r.warn.Addf("document body is malformed; content after the error was dropped")
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if !inBody {
				inBody = t.Name.Local == "body"
				continue
			}
			n, err := readElement(dec, t)
			items = append(items, r.blockItems(s, []*node{n}, blockCtx{top: true})...)
			if r.err != nil {
				return nil, r.err
			}
			if err != nil {
				r.warn.Addf("document body is truncated; content after the error was dropped")
				return items, nil
			}
		case xml.EndElement:
			if inBody && t.Name.Local == "body" {
				return items, nil
			}
		}
	}
	if !inBody {
		return nil, fmt.Errorf("docx: %s has no document body", r.main.path)
	}
	return items, nil
}

// ---------------------------------------------------------------------------
// Package parts

// readRels parses the relationships of partPath ("" = the package itself).
func (r *reader) readRels(partPath string) map[string]rel {
	relsPath := "_rels/.rels"
	if partPath != "" {
		dir, base := path.Split(partPath)
		relsPath = dir + "_rels/" + base + ".rels"
	}
	out := map[string]rel{}
	data := r.optionalBytes(relsPath)
	if data == nil {
		return out
	}
	root, _ := parseXML(data)
	for _, n := range root.childrenNamed("Relationship") {
		id := n.attr("Id")
		target := strings.TrimSpace(n.attr("Target"))
		if id == "" || target == "" {
			continue
		}
		rl := rel{typ: n.attr("Type"), target: target}
		if strings.EqualFold(n.attr("TargetMode"), "External") {
			rl.external = true
		} else if partPath == "" {
			rl.target = rd.CleanZipPath(target)
		} else {
			rl.target = rd.ResolveZipPath(partPath, target)
		}
		out[id] = rl
	}
	return out
}

func sortedRels(m map[string]rel) []rel {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]rel, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}

// relTarget returns the target of the first relationship of p whose type
// ends with suffix, or def.
func (r *reader) relTarget(p *part, suffix, def string) string {
	for _, rl := range sortedRels(p.rels) {
		if strings.HasSuffix(rl.typ, suffix) && !rl.external {
			return rl.target
		}
	}
	return def
}

// optionalBytes reads an optional part; missing parts yield nil and limit
// violations a warning.
func (r *reader) optionalBytes(name string) []byte {
	if name == "" || !r.z.Has(name) {
		return nil
	}
	data, err := r.z.Read(name)
	if err != nil {
		r.warn.Addf("could not read %s: %v", name, err)
		return nil
	}
	return data
}

func (r *reader) optionalPart(name string) *node {
	data := r.optionalBytes(name)
	if data == nil {
		return nil
	}
	root, err := parseXML(data)
	if err != nil && root == nil {
		r.warn.Addf("%s is malformed and was skipped", name)
	}
	return root
}

// readCore fills metadata from the core properties part.
func (r *reader) readCore(pkg map[string]rel) {
	name := ""
	for _, rl := range sortedRels(pkg) {
		if strings.HasSuffix(rl.typ, "/core-properties") && !rl.external {
			name = rl.target
			break
		}
	}
	if name == "" {
		name = "docProps/core.xml"
	}
	root := r.optionalPart(name)
	if root == nil {
		return
	}
	m := &r.doc.Meta
	for _, k := range root.kids {
		v := strings.TrimSpace(rd.CleanText(k.textContent()))
		if v == "" {
			continue
		}
		switch k.name {
		case "title":
			m.Title = v
		case "creator":
			for _, a := range strings.Split(v, ";") {
				if a = strings.TrimSpace(a); a != "" {
					m.Authors = append(m.Authors, ast.Author{Name: a})
				}
			}
		case "subject":
			m.Subject = v
		case "description":
			m.Summary = v
		case "keywords":
			for _, kw := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' }) {
				if kw = strings.TrimSpace(kw); kw != "" {
					m.Keywords = append(m.Keywords, kw)
				}
			}
		case "language":
			m.Lang = v
		}
	}
}

// finishMeta applies title paragraphs and the document language.
func (r *reader) finishMeta() {
	m := &r.doc.Meta
	if len(r.titles) > 0 {
		m.Title = strings.Join(r.titles, " ")
	}
	if len(r.subtitles) > 0 {
		m.Subtitle = strings.Join(r.subtitles, " ")
	}
	if m.Lang != "" {
		return
	}
	best, bestN := "", 0
	for lang, n := range r.langCount {
		if n > bestN || (n == bestN && lang < best) {
			best, bestN = lang, n
		}
	}
	if best == "" && validLang(r.styles.paraRun("").lang) {
		best = r.styles.paraRun("").lang
	}
	m.Lang = best
}

// ---------------------------------------------------------------------------
// Stories

// story is one flow of text (the body, a footnote, a text box). Complex
// fields may span paragraphs, so their state lives here.
type story struct {
	part      *part
	frames    []*frame
	body      bool
	bookmarks []string // bookmark starts not yet attached to a paragraph
	// dropCap holds the enlarged initial of the next paragraph, which word
	// processors keep in a separate framed paragraph.
	dropCap []ast.Inline
}

func (r *reader) newStory(p *part, body bool) *story { return &story{part: p, body: body} }

// note converts footnote/endnote id.
func (r *reader) note(kind, id string) *ast.Note {
	key := kind + ":" + id
	n := r.notes[key]
	if n == nil || r.notesBusy[key] {
		if n == nil {
			r.warn.Addf("%s %s is missing", kind, id)
		}
		return nil
	}
	r.notesBusy[key] = true
	defer delete(r.notesBusy, key)
	s := r.newStory(r.noteParts[kind], false)
	items := r.blockItems(s, n.kids, blockCtx{inNote: true})
	blocks := r.assemble(items, blockCtx{inNote: true})
	if len(blocks) == 0 {
		return nil
	}
	return &ast.Note{Blocks: blocks}
}

// ---------------------------------------------------------------------------
// Bookmarks and links

// idFor returns the sanitised element id for bookmark name.
func (r *reader) idFor(name string) string {
	if id, ok := r.bmIDs[name]; ok {
		return id
	}
	base := sanitizeID(name)
	if base == "" {
		base = "bookmark"
	}
	id := base
	for n := 2; ; n++ {
		owner, taken := r.idOwner[id]
		if !taken || owner == name {
			break
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
	r.idOwner[id] = name
	r.bmIDs[name] = id
	return id
}

// hiddenBookmark reports bookmarks the word processor creates on its own.
func hiddenBookmark(name string) bool { return strings.HasPrefix(name, "_") }

// anchorID chooses the id carried by an element holding bookmarks, records
// the other bookmarks as aliases of it and returns "" when no bookmark is
// worth keeping. Visible bookmarks always count; hidden ones (_Ref, _Toc,
// _GoBack) only when something links to them.
func (r *reader) anchorID(bookmarks []string, keepVisible bool) string {
	chosen := ""
	for _, b := range bookmarks {
		if keepVisible && !hiddenBookmark(b) {
			chosen = r.idFor(b)
			break
		}
	}
	if chosen == "" {
		for _, b := range bookmarks {
			if r.referenced[b] {
				chosen = r.idFor(b)
				break
			}
		}
	}
	if chosen == "" {
		return ""
	}
	for _, b := range bookmarks {
		if !r.referenced[b] && hiddenBookmark(b) {
			continue
		}
		if id := r.idFor(b); id != chosen {
			r.alias[id] = chosen
		}
	}
	return chosen
}

func (r *reader) internalLink(name string, ins []ast.Inline) *ast.Link {
	l := &ast.Link{URL: "#" + r.idFor(name), Inlines: ins}
	r.links = append(r.links, l)
	return l
}

func (r *reader) rewriteLinks() {
	for _, l := range r.links {
		id := strings.TrimPrefix(l.URL, "#")
		for i := 0; i < 8; i++ {
			next, ok := r.alias[id]
			if !ok || next == id {
				break
			}
			id = next
		}
		l.URL = "#" + id
	}
}

// countLang accumulates text length per language for Meta.Lang.
func (r *reader) countLang(lang, text string) {
	if !validLang(lang) {
		return
	}
	r.langCount[lang] += utf8.RuneCountInString(text)
}

// validLang accepts plausible BCP 47 tags ("lv", "en-US", "sr-Latn-RS")
// and rejects placeholders such as "x-none".
func validLang(tag string) bool {
	if len(tag) < 2 || len(tag) > 35 || strings.HasPrefix(strings.ToLower(tag), "x-") {
		return false
	}
	for i, part := range strings.Split(tag, "-") {
		if part == "" || len(part) > 8 || (i == 0 && len(part) > 3) {
			return false
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
				return false
			}
		}
	}
	return true
}
