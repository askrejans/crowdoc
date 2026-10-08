package cite

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// citeItem is one cited key with its resolved entry (nil when unknown).
type citeItem struct {
	it ast.CiteItem
	e  *entry
}

func (p *processor) resolve(c *ast.Cite) []citeItem {
	out := make([]citeItem, 0, len(c.Items))
	for _, it := range c.Items {
		out = append(out, citeItem{it: it, e: p.lookup(it.Key)})
	}
	return out
}

func (p *processor) renderCite(c *ast.Cite) {
	if len(c.Items) == 0 {
		return
	}
	items := p.resolve(c)
	known := false
	for _, ci := range items {
		if ci.e != nil {
			known = true
		}
	}
	b := p.f.builder()
	switch {
	case !known:
		for i, ci := range items {
			if i > 0 {
				b.text("; ")
			}
			unknownItem(b, ci.it.Key)
		}
	case p.f.st.numeric:
		p.renderNumeric(b, items, c.Mode == ast.CiteNarrative)
	default:
		p.renderAuthorDate(b, items, c.Mode == ast.CiteNarrative)
	}
	c.Rendered = b.result()
}

func unknownItem(b *builder, key string) {
	b.node(&ast.Strong{Inlines: []ast.Inline{&ast.Text{Value: "?" + key}}}, "?"+key)
}

// ---------------------------------------------------------------------------
// Names and titles in citations
// ---------------------------------------------------------------------------

// citeNames renders the author part of a citation as plain text:
// "Smith", "Smith & Lee", "Smith et al.", "J. Smith".
func (p *processor) citeNames(e *entry, narrative bool) string {
	short := make([]string, len(e.names))
	for i, n := range e.names {
		short[i] = familyDisplay(n)
		if i == 0 && e.initials && n.Literal == "" {
			if ini := initials(n.Given, true, true); ini != "" {
				short[i] = ini + " " + short[i]
			}
		}
	}
	return p.f.joinCiteNames(short, e.others, narrative, e.showNames)
}

func (f *fmtr) joinCiteNames(names []string, others, narrative bool, show int) string {
	n := len(names)
	if n == 0 {
		return ""
	}
	and := f.term("and")
	if !narrative && f.st.parenAnd != "" {
		and = f.st.parenAnd
	}
	if others || n >= f.st.citeEtAlMin {
		k := max(show, 1)
		if others || k < n {
			k = min(k, n)
			if k == 1 {
				return names[0] + " " + f.term("et-al")
			}
			return strings.Join(names[:k], ", ") + ", " + f.term("et-al")
		}
	}
	switch n {
	case 1:
		return names[0]
	case 2:
		return names[0] + " " + and + " " + names[1]
	}
	sep := " "
	if f.st.serialComma {
		sep = ", "
	}
	return strings.Join(names[:n-1], ", ") + sep + and + " " + names[n-1]
}

// authorPart writes the author of e, or its short title for anonymous works.
func (p *processor) authorPart(b *builder, e *entry, narrative bool) {
	if len(e.names) > 0 {
		b.text(p.citeNames(e, narrative))
		return
	}
	p.titlePart(b, e)
}

// titlePart writes the short title of e in the style used for its full
// title: italics for standalone works, quotation marks for parts.
func (p *processor) titlePart(b *builder, e *entry) {
	t := shortTitle(e.ref)
	switch {
	case t == "":
		b.text(e.key)
	case e.kind == kLegal:
		b.text(t)
	case e.standalone():
		b.emph(t)
	default:
		p.f.quote(b, t)
	}
}

// ---------------------------------------------------------------------------
// Locators, prefixes, suffixes
// ---------------------------------------------------------------------------

var locatorLabels = map[string]string{
	"p": "page", "pp": "page", "pg": "page", "pgs": "page", "page": "page", "pages": "page",
	"lpp": "page", "s": "page", "S": "page",
	"chap": "chapter", "chaps": "chapter", "ch": "chapter", "chapter": "chapter", "chapters": "chapter",
	"sec": "section", "secs": "section", "sect": "section", "section": "section", "sections": "section", "§": "section", "§§": "section",
	"fig": "figure", "figs": "figure", "figure": "figure", "figures": "figure",
	"tab": "table", "tbl": "table", "table": "table", "tables": "table",
	"para": "paragraph", "paras": "paragraph", "par": "paragraph", "paragraph": "paragraph", "paragraphs": "paragraph", "¶": "paragraph", "¶¶": "paragraph",
	"l": "line", "ll": "line", "line": "line", "lines": "line",
	"vol": "volume", "vols": "volume", "volume": "volume", "volumes": "volume",
	"n": "note", "nn": "note", "note": "note", "notes": "note",
	"col": "column", "cols": "column", "column": "column", "columns": "column",
	"pt": "part", "pts": "part", "part": "part", "parts": "part",
	"v": "verse", "vv": "verse", "verse": "verse", "verses": "verse",
	"bk": "book", "bks": "book", "book": "book", "books": "book",
}

// parseLocator returns the canonical label and value of a locator. A
// label still embedded in the value ("p. 33", "chap. 2") is recognised.
func parseLocator(it ast.CiteItem) (string, string) {
	value := strings.TrimSpace(it.Locator)
	if value == "" {
		return "", ""
	}
	label := strings.TrimSpace(it.LocatorLabel)
	if label == "" {
		word, rest, ok := strings.Cut(value, " ")
		if !ok && (strings.HasPrefix(value, "§") || strings.HasPrefix(value, "¶")) {
			r, size := utf8.DecodeRuneInString(value)
			word, rest, ok = string(r), value[size:], true
		}
		if ok {
			if canon, known := locatorLabels[strings.TrimSuffix(word, ".")]; known && strings.TrimSpace(rest) != "" {
				return canon, strings.TrimSpace(rest)
			}
		}
		return "page", value
	}
	if canon, ok := locatorLabels[strings.ToLower(strings.TrimSuffix(label, "."))]; ok {
		return canon, value
	}
	return label, value
}

// locatorText renders a locator: "p. 33", "pp. 33–35", "33" (Chicago),
// "Chapter 3" (APA), "33. lpp." (Latvian).
func (f *fmtr) locatorText(it ast.CiteItem) (string, bool) {
	label, value := parseLocator(it)
	if value == "" {
		return "", false
	}
	if label == "page" {
		v := f.pageRange(value)
		if f.st.barePages {
			return v, true
		}
		return f.labelled("page", v), true
	}
	if f.term(label) == "" {
		return label + " " + value, false
	}
	return f.labelled(label, value), false
}

func (p *processor) prefix(b *builder, s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	b.text(s)
	b.text(" ")
}

// suffix appends free text after a citation item, separated by a comma
// unless it starts with its own punctuation.
func suffix(b *builder, s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	if strings.ContainsRune(",;:.)]", rune(s[0])) {
		b.text(s)
		return
	}
	b.text(", ")
	b.text(s)
}

func anchorLink(b *builder, e *entry, ins []ast.Inline) {
	if len(ins) == 0 {
		return
	}
	b.node(&ast.Link{URL: "#" + e.anchor, Inlines: ins}, ast.PlainText(ins))
}

// ---------------------------------------------------------------------------
// Author-date and author-page citations
// ---------------------------------------------------------------------------

func (p *processor) renderAuthorDate(b *builder, items []citeItem, narrative bool) {
	st := p.f.st
	lead := ""
	if !narrative && st.sortCites && len(items) > 1 && !prefixAfterFirst(items) {
		lead = items[0].it.Prefix
		items[0].it.Prefix = ""
		slices.SortStableFunc(items, func(a, b citeItem) int { return rank(a) - rank(b) })
	}
	groups := p.groupItems(items)
	if narrative {
		for i, g := range groups {
			if i > 0 {
				b.text("; ")
			}
			p.narrativeGroup(b, g)
		}
		return
	}
	b.text("(")
	p.prefix(b, lead)
	for i, g := range groups {
		if i > 0 {
			b.text("; ")
		}
		p.parenGroup(b, g)
	}
	b.text(")")
}

func prefixAfterFirst(items []citeItem) bool {
	for _, ci := range items[1:] {
		if strings.TrimSpace(ci.it.Prefix) != "" {
			return true
		}
	}
	return false
}

// rank orders citation items by bibliography position, unknown keys last.
func rank(ci citeItem) int {
	if ci.e == nil {
		return 1 << 30
	}
	return ci.e.pos
}

// groupItems collects consecutive works by the same authors so they are
// cited as "Smith, 2019, 2020a".
func (p *processor) groupItems(items []citeItem) [][]citeItem {
	var out [][]citeItem
	for _, ci := range items {
		if n := len(out); n > 0 && !p.f.st.authorPage && ci.e != nil && strings.TrimSpace(ci.it.Prefix) == "" {
			prev := out[n-1][len(out[n-1])-1]
			if prev.e != nil && prev.e != ci.e && !prev.it.SuppressAuthor && !ci.it.SuppressAuthor &&
				strings.TrimSpace(prev.it.Suffix) == "" && len(ci.e.names) > 0 &&
				p.citeKey(prev.e) == p.citeKey(ci.e) {
				out[n-1] = append(out[n-1], ci)
				continue
			}
		}
		out = append(out, []citeItem{ci})
	}
	for _, g := range out {
		if len(g) > 1 {
			slices.SortStableFunc(g, func(a, b citeItem) int { return rank(a) - rank(b) })
		}
	}
	return out
}

// yearSep writes the separator between author and year.
func (p *processor) yearSep(b *builder) {
	if strings.HasPrefix(p.f.st.yearSep, ",") {
		b.punct(",")
	}
	b.text(" ")
}

func (p *processor) parenGroup(b *builder, g []citeItem) {
	f := p.f
	for j, ci := range g {
		if ci.e == nil {
			p.prefix(b, ci.it.Prefix)
			unknownItem(b, ci.it.Key)
			suffix(b, ci.it.Suffix)
			continue
		}
		if j == 0 {
			p.prefix(b, ci.it.Prefix)
		} else {
			b.text(", ")
		}
		lb := f.builder()
		loc, page := f.locatorText(ci.it)
		if f.st.authorPage {
			p.authorPageItem(lb, ci, loc, page)
		} else {
			if j == 0 && !ci.it.SuppressAuthor {
				p.authorPart(lb, ci.e, false)
				p.yearSep(lb)
			}
			lb.text(f.yearText(ci.e))
			if loc != "" {
				lb.text(", " + loc)
			}
		}
		anchorLink(b, ci.e, lb.result())
		suffix(b, ci.it.Suffix)
	}
}

// authorPageItem renders an MLA item: "Smith 33", "Smith, Short Title 33",
// "Smith, ch. 2".
func (p *processor) authorPageItem(b *builder, ci citeItem, loc string, page bool) {
	e := ci.e
	if !ci.it.SuppressAuthor {
		p.authorPart(b, e, false)
		if e.needTitle && len(e.names) > 0 {
			b.punct(",")
			b.text(" ")
			p.titlePart(b, e)
		}
	} else if loc == "" {
		p.titlePart(b, e)
	}
	if loc == "" {
		return
	}
	if !b.empty() {
		if !page {
			b.punct(",")
		}
		b.space()
	}
	b.text(loc)
}

func (p *processor) narrativeGroup(b *builder, g []citeItem) {
	f := p.f
	first := g[0]
	if first.e == nil {
		p.prefix(b, first.it.Prefix)
		unknownItem(b, first.it.Key)
		suffix(b, first.it.Suffix)
		return
	}
	p.prefix(b, first.it.Prefix)
	if len(g) == 1 {
		e := first.e
		lb := f.builder()
		loc, _ := f.locatorText(first.it)
		switch {
		case f.st.authorPage:
			if !first.it.SuppressAuthor {
				p.authorPart(lb, e, true)
				if e.needTitle && len(e.names) > 0 {
					lb.punct(",")
					lb.text(" ")
					p.titlePart(lb, e)
				}
			}
			if loc != "" || first.it.SuppressAuthor {
				lb.space()
				lb.text("(")
				if loc == "" {
					p.titlePart(lb, e)
				} else {
					lb.text(loc)
				}
				suffix(lb, first.it.Suffix)
				lb.text(")")
			} else {
				suffix(lb, first.it.Suffix)
			}
		default:
			if !first.it.SuppressAuthor {
				p.authorPart(lb, e, true)
				lb.text(" ")
			}
			lb.text("(" + f.yearText(e))
			if loc != "" {
				lb.text(", " + loc)
			}
			suffix(lb, first.it.Suffix)
			lb.text(")")
		}
		anchorLink(b, e, lb.result())
		return
	}
	// Several works by the same authors: "Smith (2019, 2020a)".
	ab := f.builder()
	p.authorPart(ab, first.e, true)
	anchorLink(b, first.e, ab.result())
	b.text(" (")
	for j, ci := range g {
		if j > 0 {
			b.text(", ")
		}
		lb := f.builder()
		lb.text(f.yearText(ci.e))
		if loc, _ := f.locatorText(ci.it); loc != "" {
			lb.text(", " + loc)
		}
		anchorLink(b, ci.e, lb.result())
		suffix(b, ci.it.Suffix)
	}
	b.text(")")
}

// ---------------------------------------------------------------------------
// Numeric citations
// ---------------------------------------------------------------------------

// numUnit is a single cited number or a compressed range of numbers.
type numUnit struct {
	from, to citeItem
	isRange  bool
}

func plainItem(ci citeItem) bool {
	return ci.e != nil && strings.TrimSpace(ci.it.Locator) == "" &&
		strings.TrimSpace(ci.it.Prefix) == "" && strings.TrimSpace(ci.it.Suffix) == ""
}

// compress sorts items by number and collapses runs of three or more
// consecutive plain numbers into ranges.
func compress(items []citeItem) []numUnit {
	slices.SortStableFunc(items, func(a, b citeItem) int { return a.e.num - b.e.num })
	// Drop repeated plain citations of the same work.
	var uniq []citeItem
	for _, ci := range items {
		if n := len(uniq); n > 0 && plainItem(ci) && plainItem(uniq[n-1]) && uniq[n-1].e == ci.e {
			continue
		}
		uniq = append(uniq, ci)
	}
	var out []numUnit
	for i := 0; i < len(uniq); {
		j := i
		for j+1 < len(uniq) && plainItem(uniq[j]) && plainItem(uniq[j+1]) && uniq[j+1].e.num == uniq[j].e.num+1 {
			j++
		}
		if j-i >= 2 {
			out = append(out, numUnit{from: uniq[i], to: uniq[j], isRange: true})
			i = j + 1
			continue
		}
		out = append(out, numUnit{from: uniq[i]})
		i++
	}
	return out
}

func numLink(b *builder, e *entry) {
	n := strconv.Itoa(e.num)
	b.node(&ast.Link{URL: "#" + e.anchor, Inlines: []ast.Inline{&ast.Text{Value: n}}}, n)
}

func (p *processor) renderNumeric(b *builder, items []citeItem, narrative bool) {
	f := p.f
	if narrative {
		for i, ci := range items {
			if i > 0 {
				b.text("; ")
			}
			if ci.e == nil {
				p.prefix(b, ci.it.Prefix)
				unknownItem(b, ci.it.Key)
				suffix(b, ci.it.Suffix)
				continue
			}
			p.prefix(b, ci.it.Prefix)
			if !ci.it.SuppressAuthor {
				p.authorPart(b, ci.e, true)
				b.text(" ")
			}
			b.text("[")
			numLink(b, ci.e)
			if loc, _ := f.locatorText(ci.it); loc != "" {
				b.text(", " + loc)
			}
			b.text("]")
			suffix(b, ci.it.Suffix)
		}
		return
	}
	var known []citeItem
	var unknown []citeItem
	for _, ci := range items {
		if ci.e == nil {
			unknown = append(unknown, ci)
		} else {
			known = append(known, ci)
		}
	}
	units := compress(known)
	if f.st.vancouverCN {
		b.text("[")
		for i, u := range units {
			if i > 0 {
				b.text(", ")
			}
			if u.isRange {
				numLink(b, u.from.e)
				b.text("–")
				numLink(b, u.to.e)
				continue
			}
			p.prefix(b, u.from.it.Prefix)
			numLink(b, u.from.e)
			if loc, _ := f.locatorText(u.from.it); loc != "" {
				if len(units) == 1 {
					b.text(", " + loc)
				} else {
					b.text(" (" + loc + ")")
				}
			}
			suffix(b, u.from.it.Suffix)
		}
		for _, ci := range unknown {
			b.text(", ")
			unknownItem(b, ci.it.Key)
		}
		b.text("]")
		return
	}
	for i, u := range units {
		if i > 0 {
			b.text(", ")
		}
		if u.isRange {
			b.text("[")
			numLink(b, u.from.e)
			b.text("]–[")
			numLink(b, u.to.e)
			b.text("]")
			continue
		}
		p.prefix(b, u.from.it.Prefix)
		b.text("[")
		numLink(b, u.from.e)
		if loc, _ := f.locatorText(u.from.it); loc != "" {
			b.text(", " + loc)
		}
		b.text("]")
		suffix(b, u.from.it.Suffix)
	}
	for _, ci := range unknown {
		b.text(", ")
		unknownItem(b, ci.it.Key)
	}
}
