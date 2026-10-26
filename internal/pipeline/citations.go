package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/transform/cite"
	"github.com/askrejans/crowdoc/v2/internal/typst"
)

type bibResult struct {
	entries []typst.BibEntry
	numeric bool
}

var bibTitles = map[string]bool{
	"references": true, "bibliography": true, "works cited": true, "literature": true, "sources": true, "literature cited": true,
	"literatūra": true, "izmantotā literatūra": true, "izmantotie avoti": true, "avoti": true, "literatūras saraksts": true,
	"literaturverzeichnis": true, "literatur": true, "quellen": true, "bibliographie": true, "références": true,
	"bibliografía": true, "referencias": true, "bibliografia": true, "riferimenti": true, "literatuur": true,
	"referenties": true, "bibliografi": true, "kirjallisuus": true, "lähteet": true, "litteratur": true, "källor": true,
	"literatura": true, "šaltiniai": true, "kirjandus": true, "piśmiennictwo": true, "источники": true,
	"литература": true, "список литературы": true, "література": true, "βιβλιογραφία": true, "kaynakça": true,
	"irodalom": true, "bibliografie": true, "参考文献": true, "참고문헌": true, "المراجع": true, "ביבליוגרפיה": true,
}

func isBibTitle(s string) bool {
	return bibTitles[strings.ToLower(strings.Trim(strings.TrimSpace(s), ".:"))]
}

// processCitations loads bibliographies, resolves citations and decides
// where the reference list goes.
func processCitations(ctx context.Context, p *prepared, opts Options) []string {
	doc := p.doc
	if doc == nil {
		return nil
	}
	var warns []string
	hasCites := false
	ast.WalkInlines(doc.Blocks, func(in ast.Inline) {
		if _, ok := in.(*ast.Cite); ok {
			hasCites = true
		}
	})
	if !hasCites {
		ast.WalkInlines(doc.Meta.Abstract, func(in ast.Inline) {
			if _, ok := in.(*ast.Cite); ok {
				hasCites = true
			}
		})
	}
	if !hasCites && len(doc.Meta.NoCite) == 0 {
		cite.LinkPlainReferences(doc)
		return nil
	}

	refs, w := loadReferences(p, opts)
	warns = append(warns, w...)
	refs = append(refs, doc.References...)

	style := opts.CitationStyle
	if style == "" {
		style = doc.Meta.CitationStyle
	}
	if style == "" && p.style != nil {
		style = p.style.CitationStyle
	}
	res, err := cite.Process(doc, refs, cite.Options{Style: style, Lang: doc.Meta.Lang})
	if err != nil {
		return append(warns, "citations: "+err.Error())
	}
	warns = append(warns, res.Warnings...)
	if len(res.Entries) == 0 {
		return warns
	}
	name, _ := cite.NormalizeStyle(firstNonEmptyStr(style, "apa"))
	b := &bibResult{numeric: res.Numeric}
	for _, e := range res.Entries {
		label := e.Label
		if label != "" {
			if name == "vancouver" {
				label += "."
			} else {
				label = "[" + label + "]"
			}
		}
		b.entries = append(b.entries, typst.BibEntry{ID: e.ID, Label: label, Inlines: e.Inlines})
	}
	p.bibliography = b
	placeBibliography(doc)
	return warns
}

// placeBibliography turns an empty trailing "References" section into the
// bibliography position, keeping its title.
func placeBibliography(doc *ast.Document) {
	has := false
	ast.WalkBlocks(doc.Blocks, func(b ast.Block) bool {
		if _, ok := b.(*ast.Bibliography); ok {
			has = true
		}
		return true
	})
	if has {
		return
	}
	for i := len(doc.Blocks) - 1; i >= 0; i-- {
		h, ok := doc.Blocks[i].(*ast.Heading)
		if !ok {
			continue
		}
		if !isBibTitle(ast.PlainText(h.Inlines)) {
			return
		}
		// Only an empty section (or one holding just a placeholder) is
		// replaced; an existing hand-written list is kept as it is.
		rest := doc.Blocks[i+1:]
		for _, b := range rest {
			if _, ok := b.(*ast.Heading); ok {
				return
			}
			if p, ok := b.(*ast.Para); ok && len(ast.TrimInlines(p.Inlines)) > 0 {
				return
			}
		}
		if doc.Meta.ReferenceSectionTitle == "" {
			doc.Meta.ReferenceSectionTitle = ast.PlainText(h.Inlines)
		}
		doc.Blocks = append(doc.Blocks[:i], &ast.Bibliography{})
		return
	}
}

// loadReferences reads bibliography files named by the caller and the
// document. Untrusted documents may only use uploaded data or files inside
// their own directory.
func loadReferences(p *prepared, opts Options) ([]ast.Reference, []string) {
	if p.referencesLoaded {
		return p.references, nil
	}
	var refs []ast.Reference
	var warns []string
	p.referenceFiles = make(map[string]bool)
	add := func(name string, data []byte) {
		r, w, err := cite.Parse(name, data)
		if err != nil {
			warns = append(warns, fmt.Sprintf("bibliography %s: %v", filepath.Base(name), err))
			return
		}
		for _, x := range w {
			warns = append(warns, fmt.Sprintf("bibliography %s: %s", filepath.Base(name), x))
		}
		refs = append(refs, r...)
		p.referenceFiles[name] = true
	}
	names := make([]string, 0, len(opts.BibliographyData))
	for n := range opts.BibliographyData {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		add(n, opts.BibliographyData[n])
	}
	if !p.src.Untrusted {
		for _, path := range opts.Bibliography {
			data, err := os.ReadFile(path)
			if err != nil {
				warns = append(warns, fmt.Sprintf("bibliography %s: %v", path, err))
				continue
			}
			add(path, data)
		}
	}
	for _, name := range p.doc.Meta.Bibliography {
		if _, ok := opts.BibliographyData[filepath.Base(name)]; ok {
			p.referenceFiles[name] = p.referenceFiles[filepath.Base(name)]
			continue // supplied by the caller
		}
		path, ok := bibPath(p, name)
		if !ok {
			warns = append(warns, fmt.Sprintf("bibliography %s is not accessible for this document", name))
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			warns = append(warns, fmt.Sprintf("bibliography %s not found", name))
			continue
		}
		add(path, data)
		p.referenceFiles[name] = p.referenceFiles[path]
	}
	if v, ok := p.doc.Meta.Extra["references"]; ok {
		r, w := cite.FromCSLValue(v)
		refs = append(refs, r...)
		warns = append(warns, w...)
	}
	p.referencesLoaded = true
	p.references = refs
	return refs, warns
}

// EmbedBibliography retains the resolved citation data instead of depending on
// bibliography files beside the original input. Unresolved paths remain in the
// metadata so their existing warnings and opportunity for repair are preserved.
func (p *prepared) EmbedBibliography(opts Options) []string {
	refs, warns := loadReferences(p, opts)
	seen := make(map[string]bool)
	var merged []ast.Reference
	for _, group := range [][]ast.Reference{refs, p.doc.References} {
		for _, ref := range group {
			id := strings.TrimSpace(ref.ID)
			if id != "" && !seen[id] {
				merged = append(merged, ref)
				seen[id] = true
			}
		}
	}
	if len(merged) > 0 {
		p.doc.References = merged
		// The writer emits the merged CSL data, including entries originally
		// supplied by DOCX fields, caller data or inline frontmatter.
		delete(p.doc.Meta.Extra, "references")
	}
	var remaining []string
	for _, name := range p.doc.Meta.Bibliography {
		if !p.referenceFiles[name] {
			remaining = append(remaining, name)
		}
	}
	p.doc.Meta.Bibliography = remaining
	return warns
}

func bibPath(p *prepared, name string) (string, bool) {
	if p.baseDir == "" {
		return "", false
	}
	if !p.src.Untrusted {
		if filepath.IsAbs(name) {
			return name, true
		}
		return filepath.Join(p.baseDir, filepath.FromSlash(name)), true
	}
	if filepath.IsAbs(name) || strings.Contains(filepath.ToSlash(name), "..") {
		return "", false
	}
	full := filepath.Join(p.baseDir, filepath.FromSlash(name))
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", false
	}
	base, err := filepath.EvalSymlinks(p.baseDir)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(base, real)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return real, true
}
