// Package cite is a native citation processor. It reads bibliography files
// (BibTeX/BibLaTeX, CSL-JSON, CSL-YAML, RIS, MEDLINE), resolves the
// citations of a document and formats the reference list in common
// academic styles (APA, Chicago author-date, Harvard, IEEE, Vancouver,
// MLA). It produces AST nodes only — emphasis, links and labels — and
// leaves typesetting to the writers.
package cite

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/askrejans/crowdoc/v2/ast"
)

// Options configures Process.
type Options struct {
	// Style is a style name or alias (see Styles and NormalizeStyle).
	// Empty selects the document's CitationStyle, then "apa".
	Style string
	// Lang is a BCP 47 tag used for terms, dates, quotation marks and
	// sorting. Empty selects the document language, then English.
	Lang string
	// NoCite lists keys to include in the bibliography without citing
	// them; "*" includes every known reference.
	NoCite []string
}

// Result is the outcome of Process.
type Result struct {
	// Entries is the formatted bibliography in its final order.
	Entries []ast.ReferenceEntry
	// Numeric reports that entries carry number labels (writers use a
	// labelled list).
	Numeric bool
	// Warnings lists unknown and duplicate keys and references missing
	// essential fields.
	Warnings []string
}

// EntryID returns the anchor id of the bibliography entry for a citation
// key: "ref-" followed by the key with every character other than
// lowercase ASCII letters, digits, '-', '_', ':' and '.' replaced by '-'.
func EntryID(key string) string {
	var sb strings.Builder
	sb.Grow(len(key) + 4)
	sb.WriteString("ref-")
	for _, r := range strings.ToLower(key) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == ':', r == '.':
			sb.WriteRune(r)
		default:
			sb.WriteByte('-')
		}
	}
	return sb.String()
}

// entry is a reference prepared for formatting.
type entry struct {
	ref    *ast.Reference
	key    string
	anchor string
	kind   string

	names  []ast.Name // primary contributors without the "others" marker
	role   string     // "author", "editor", "translator" or ""
	others bool       // the name list was truncated in the source

	order int  // position in the reference database
	pos   int  // position in the bibliography
	first int  // index of the first citation, -1 when never cited
	inBib bool // listed in the bibliography
	num   int  // numeric label

	suffix    string // year disambiguation letter
	showNames int    // names shown before "et al." in citations (0 = style default)
	initials  bool   // first author cited with initials (different people, same family name)
	needTitle bool   // author-page styles: cite with a short title
	sameNames bool   // same contributors as the previous entry (MLA "---.")

	sortNames [][2]string
	sortYear  int
	sortTitle string
}

type processor struct {
	f       *fmtr
	coll    *collate.Collator
	all     []*entry
	byKey   map[string]*entry
	byLower map[string]*entry
	list    []*entry // bibliography entries
	unknown map[string]bool
	next    int
	warn    func(format string, args ...any)
}

// Process resolves every *ast.Cite in doc — including those in footnotes,
// captions, table cells and the abstract — by setting Cite.Rendered, and
// returns the formatted bibliography of the cited references (plus NoCite
// keys). refs are references from external bibliography files; they take
// precedence over doc.References with the same id, which only fill gaps.
// Unknown keys render as "?key" in bold and produce a warning; they never
// fail the document.
func Process(doc *ast.Document, refs []ast.Reference, o Options) (*Result, error) {
	if doc == nil {
		return nil, errors.New("cite: nil document")
	}
	res := &Result{}
	warn := func(format string, args ...any) {
		res.Warnings = append(res.Warnings, fmt.Sprintf(format, args...))
	}
	styleName := o.Style
	if strings.TrimSpace(styleName) == "" {
		styleName = doc.Meta.CitationStyle
	}
	name, ok := NormalizeStyle(styleName)
	if !ok {
		if strings.TrimSpace(styleName) != "" {
			warn("unknown citation style %q; using apa", styleName)
		}
		name = "apa"
	}
	lang := o.Lang
	if lang == "" {
		lang = doc.Meta.Lang
	}
	p := newProcessor(styles[name], lang, warn)
	p.load(refs, doc.References)

	var cites []*ast.Cite
	collect := func(in ast.Inline) {
		if c, ok := in.(*ast.Cite); ok {
			cites = append(cites, c)
		}
	}
	ast.WalkInlines(doc.Meta.Abstract, collect)
	ast.WalkInlines(doc.Blocks, collect)

	p.register(cites)
	p.addNoCite(o.NoCite)
	p.addNoCite(doc.Meta.NoCite)
	p.prepare()
	for _, c := range cites {
		p.renderCite(c)
	}
	res.Entries = p.bibliography()
	res.Numeric = p.f.st.numeric
	return res, nil
}

func newProcessor(st *style, lang string, warn func(string, ...any)) *processor {
	f := newFmtr(st, lang)
	tag, err := language.Parse(lang)
	if err != nil || lang == "" {
		tag = language.English
	}
	return &processor{
		f:       f,
		coll:    collate.New(tag, collate.IgnoreCase),
		byKey:   map[string]*entry{},
		byLower: map[string]*entry{},
		unknown: map[string]bool{},
		warn:    warn,
	}
}

func (p *processor) load(external, embedded []ast.Reference) {
	add := func(r *ast.Reference, ext bool) {
		id := strings.TrimSpace(r.ID)
		if id == "" {
			if ext {
				p.warn("reference without an id ignored (title %q)", r.Title)
			}
			return
		}
		if _, dup := p.byKey[id]; dup {
			if ext {
				p.warn("duplicate citation key: %s (first definition kept)", id)
			}
			return
		}
		e := &entry{ref: r, key: id, first: -1, order: len(p.all)}
		p.byKey[id] = e
		if lk := strings.ToLower(id); p.byLower[lk] == nil {
			p.byLower[lk] = e
		}
		p.all = append(p.all, e)
	}
	for i := range external {
		add(&external[i], true)
	}
	for i := range embedded {
		add(&embedded[i], false)
	}
}

// lookup finds a key exactly, then case-insensitively (BibTeX keys are
// case-insensitive).
func (p *processor) lookup(key string) *entry {
	key = strings.TrimSpace(key)
	if e := p.byKey[key]; e != nil {
		return e
	}
	return p.byLower[strings.ToLower(key)]
}

func (p *processor) register(cites []*ast.Cite) {
	for _, c := range cites {
		for _, it := range c.Items {
			e := p.lookup(it.Key)
			if e == nil {
				if !p.unknown[it.Key] {
					p.unknown[it.Key] = true
					p.warn("citation key not found: %s", it.Key)
				}
				continue
			}
			if e.first < 0 {
				e.first = p.next
				p.next++
			}
			p.include(e)
		}
	}
}

func (p *processor) include(e *entry) {
	if !e.inBib {
		e.inBib = true
		p.list = append(p.list, e)
	}
}

func (p *processor) addNoCite(keys []string) {
	for _, k := range keys {
		k = strings.TrimPrefix(strings.TrimSpace(k), "@")
		switch k {
		case "":
			continue
		case "*":
			for _, e := range p.all {
				p.include(e)
			}
			continue
		}
		if e := p.lookup(k); e != nil {
			p.include(e)
		} else if !p.unknown[k] {
			p.unknown[k] = true
			p.warn("nocite key not found: %s", k)
		}
	}
}

// primaryNames returns the contributors shown first: authors, else
// editors, else translators.
func primaryNames(r *ast.Reference) ([]ast.Name, string, bool) {
	for _, c := range []struct {
		ns   []ast.Name
		role string
	}{{r.Author, "author"}, {r.Editor, "editor"}, {r.Translator, "translator"}} {
		if ns, others := splitOthers(c.ns); len(ns) > 0 {
			return ns, c.role, others
		}
	}
	return nil, "", false
}

func (p *processor) prepare() {
	taken := map[string]bool{}
	for _, e := range p.list {
		r := e.ref
		e.kind = classify(normType(r.Type), r.ContainerTitle)
		e.names, e.role, e.others = primaryNames(r)
		if p.f.st.authorPage && len(e.names) == 1 && e.sameAsAuthor(r.Publisher) && e.names[0].Literal != "" {
			// MLA: an organisation that is both author and publisher is
			// listed only as publisher; the entry starts with the title.
			e.names, e.role = nil, ""
		}
		base := EntryID(e.key)
		a := base
		for i := 2; taken[a]; i++ {
			a = base + "-" + strconv.Itoa(i)
		}
		taken[a] = true
		e.anchor = a
		p.validate(e)
	}
	if p.f.st.numeric {
		for i, e := range p.list {
			e.num = i + 1
			e.pos = i
		}
		return
	}
	for _, e := range p.list {
		p.sortKeys(e)
	}
	slices.SortStableFunc(p.list, p.compare)
	for i, e := range p.list {
		e.pos = i
		if i > 0 && len(e.names) > 0 {
			prev := p.list[i-1]
			e.sameNames = prev.role == e.role && prev.others == e.others && slices.Equal(prev.names, e.names)
		}
	}
	p.disambiguate()
}

func (p *processor) validate(e *entry) {
	r := e.ref
	if strings.TrimSpace(r.Title) == "" {
		p.warn("reference %s has no title", e.key)
	}
	if e.kind == kJournal && r.ContainerTitle == "" {
		p.warn("reference %s has no journal title", e.key)
	}
	if len(e.names) == 0 && strings.TrimSpace(r.Title) == "" {
		p.warn("reference %s has neither author nor title", e.key)
	}
}

// ---------------------------------------------------------------------------
// Bibliography order
// ---------------------------------------------------------------------------

var leadingArticles = map[string][]string{
	"en": {"the ", "a ", "an "},
	"de": {"der ", "die ", "das ", "ein ", "eine "},
	"fr": {"le ", "la ", "les ", "l'", "un ", "une "},
	"es": {"el ", "la ", "los ", "las ", "un ", "una "},
	"it": {"il ", "lo ", "la ", "i ", "gli ", "le ", "l'", "un ", "una "},
	"pt": {"o ", "a ", "os ", "as ", "um ", "uma "},
	"nl": {"de ", "het ", "een "},
}

func (p *processor) sortTitleOf(t string) string {
	t = strings.TrimLeft(t, " \"'“”‘’„«»([{¿¡")
	low := strings.ToLower(t)
	for _, a := range leadingArticles[p.f.loc.lang] {
		if strings.HasPrefix(low, a) && len(t) > len(a) {
			return t[len(a):]
		}
	}
	return t
}

func (p *processor) sortKeys(e *entry) {
	r := e.ref
	e.sortTitle = p.sortTitleOf(r.Title)
	if len(e.names) == 0 {
		e.sortNames = [][2]string{{e.sortTitle, ""}}
	} else {
		for _, n := range e.names {
			fam, rest := nameSortKey(n)
			e.sortNames = append(e.sortNames, [2]string{p.sortTitleOf(fam), rest})
		}
	}
	switch {
	case r.Issued.Year != 0:
		e.sortYear = r.Issued.Year
	case r.Issued.Literal != "":
		e.sortYear = 1 << 20 // "in press", "forthcoming" come last
	}
}

func (p *processor) compare(a, b *entry) int {
	for i := 0; i < len(a.sortNames) && i < len(b.sortNames); i++ {
		if c := p.coll.CompareString(a.sortNames[i][0], b.sortNames[i][0]); c != 0 {
			return c
		}
		if c := p.coll.CompareString(a.sortNames[i][1], b.sortNames[i][1]); c != 0 {
			return c
		}
	}
	if len(a.sortNames) != len(b.sortNames) {
		return len(a.sortNames) - len(b.sortNames)
	}
	if a.sortYear != b.sortYear {
		return a.sortYear - b.sortYear
	}
	if c := p.coll.CompareString(a.sortTitle, b.sortTitle); c != 0 {
		return c
	}
	return strings.Compare(a.key, b.key)
}

// ---------------------------------------------------------------------------
// Disambiguation
// ---------------------------------------------------------------------------

// disambiguate makes every author-date citation unique: first authors
// sharing a family name get initials, differing author lists that
// truncate to the same "et al." form show more names, and works that
// remain identical get year suffixes (2020a, 2020b) in bibliography order.
// Author-page styles cite with a short title instead.
func (p *processor) disambiguate() {
	p.markInitials()
	if p.f.st.authorPage {
		groups := p.groupBy(func(e *entry) string { return p.citeKey(e) })
		for _, g := range groups {
			if len(g) > 1 {
				for _, e := range g {
					e.needTitle = true
				}
			}
		}
		return
	}
	yearOf := func(e *entry) string {
		d := e.ref.Issued
		if d.Year != 0 {
			return strconv.Itoa(d.Year)
		}
		return "\x01" + d.Literal
	}
	for _, g := range p.groupBy(func(e *entry) string { return p.citeKey(e) + "\x00" + yearOf(e) }) {
		if len(g) < 2 {
			continue
		}
		p.addNames(g)
		sub := map[string][]*entry{}
		var order []string
		for _, e := range g {
			k := p.citeKey(e)
			if sub[k] == nil {
				order = append(order, k)
			}
			sub[k] = append(sub[k], e)
		}
		for _, k := range order {
			if s := sub[k]; len(s) > 1 {
				for i, e := range s {
					e.suffix = suffixLetter(i)
				}
			}
		}
	}
}

// groupBy partitions the bibliography by key, keeping bibliography order.
func (p *processor) groupBy(key func(*entry) string) [][]*entry {
	idx := map[string]int{}
	var out [][]*entry
	for _, e := range p.list {
		k := key(e)
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			out = append(out, nil)
		}
		out[i] = append(out[i], e)
	}
	return out
}

// markInitials flags first authors whose family name is shared by a
// different person among the cited works ("J. Smith" vs "A. Smith").
// Compatible initials ("J." and "J. A.") are taken to be the same person.
func (p *processor) markInitials() {
	byFamily := map[string][]*entry{}
	for _, e := range p.list {
		if len(e.names) == 0 || e.names[0].Literal != "" {
			continue
		}
		fam := strings.ToLower(familyDisplay(e.names[0]))
		byFamily[fam] = append(byFamily[fam], e)
	}
	for _, group := range byFamily {
		for _, a := range group {
			ia := initials(a.names[0].Given, false, false)
			for _, b := range group {
				ib := initials(b.names[0].Given, false, false)
				if !strings.HasPrefix(ia, ib) && !strings.HasPrefix(ib, ia) {
					a.initials = true
					break
				}
			}
		}
	}
}

// addNames shows more names for works whose author lists differ but are
// abbreviated to the same "First et al." form.
func (p *processor) addNames(g []*entry) {
	distinct := func() int {
		seen := map[string]bool{}
		for _, e := range g {
			seen[p.citeKey(e)] = true
		}
		return len(seen)
	}
	base := distinct()
	maxN := 0
	for _, e := range g {
		maxN = max(maxN, len(e.names))
	}
	bestK, best := 0, base
	for k := 2; k <= maxN; k++ {
		for _, e := range g {
			e.showNames = k
		}
		if d := distinct(); d > best {
			bestK, best = k, d
			if d == len(g) {
				break
			}
		}
	}
	for _, e := range g {
		e.showNames = bestK
	}
}

func suffixLetter(i int) string {
	s := ""
	for i++; i > 0; i /= 26 {
		i--
		s = string(rune('a'+i%26)) + s
	}
	return s
}

// citeKey is the plain text of an entry's author part in a citation.
func (p *processor) citeKey(e *entry) string {
	if len(e.names) == 0 {
		return "\x02" + shortTitle(e.ref)
	}
	return p.citeNames(e, false)
}

// ---------------------------------------------------------------------------
// Bibliography
// ---------------------------------------------------------------------------

func (p *processor) bibliography() []ast.ReferenceEntry {
	out := make([]ast.ReferenceEntry, 0, len(p.list))
	for _, e := range p.list {
		re := ast.ReferenceEntry{ID: e.anchor, Inlines: p.f.st.entry(p.f, e)}
		if p.f.st.numeric {
			re.Label = strconv.Itoa(e.num)
		}
		out = append(out, re)
	}
	return out
}
