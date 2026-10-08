package layout

import (
	"math"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/askrejans/crowdoc/v2/ast"
)

// titleBlock finds the document title among the first lines of the first
// page (in reading order): the largest text in the top half of the page,
// larger than anything else in the document, or the lines tagged as the
// title. It returns the indexes of the title lines.
func (d *doc) titleBlock(lines []*line, pageH float64) []int {
	if len(lines) == 0 || d.body <= 0 {
		return nil
	}
	for i, l := range lines {
		if i > 6 {
			break
		}
		if l.role() == RoleTitle {
			idx := []int{i}
			for k := i + 1; k < len(lines) && lines[k].role() == RoleTitle && lines[k].block() == l.block(); k++ {
				idx = append(idx, k)
			}
			return idx
		}
	}
	best := -1
	for i, l := range lines {
		if i > 6 || l.y0 > 0.5*pageH {
			break
		}
		if !hasLetter(l.text()) || l.size < d.body*1.25 || l.role().headingLevel() > 0 {
			continue
		}
		if best < 0 || l.size > lines[best].size*1.02 {
			best = i
		}
	}
	if best < 0 {
		return d.plainTitle(lines)
	}
	t := lines[best]
	if first := t.words[0].text; numberingDepth(first) > 0 && len(t.words) > 1 {
		return nil // a numbered section heading, not a title
	}
	idx := []int{best}
	for k := best + 1; k < len(lines) && len(idx) < 4; k++ {
		l := lines[k]
		prev := lines[idx[len(idx)-1]]
		if !sameStyle(l, t) || l.y0-prev.y1 > 0.9*t.size {
			break
		}
		idx = append(idx, k)
	}
	in := map[*word]bool{}
	for _, k := range idx {
		for _, w := range lines[k].words {
			in[w] = true
		}
	}
	// The title style must be unique in the document.
	for _, q := range d.pages {
		for _, w := range q.words {
			if !in[w] && w.size >= t.size*0.97 && hasLetter(w.text) {
				return nil
			}
		}
	}
	return idx
}

// plainTitle finds the title of a document set in one size (a typewritten
// or plain-text printout): a first line in capitals, set off by a blank
// line.
func (d *doc) plainTitle(lines []*line) []int {
	if len(lines) < 2 {
		return nil
	}
	l := lines[0]
	text := l.text()
	if len(l.words) > 10 || !hasLetter(text) || strings.ToUpper(text) != text || numberingDepth(l.words[0].text) > 0 {
		return nil
	}
	if lines[1].y0-l.y1 < 0.8*l.size {
		return nil
	}
	return []int{0}
}

// pageLines returns the lines of page i in reading order (from the
// flows, so that the columns of a page are not mixed).
func (d *doc) pageLines(i int) []*line {
	var out []*line
	for _, f := range d.flows {
		if f.page != i {
			continue
		}
		for _, it := range f.items {
			if it.l != nil {
				out = append(out, it.l)
			}
		}
	}
	return out
}

// dropLines removes lines from the flows and their words from the pages.
func (d *doc) dropLines(drop map[*line]bool) {
	words := map[*word]bool{}
	for l := range drop {
		for _, w := range l.words {
			words[w] = true
		}
	}
	d.dropWords(words)
}

func sameStyle(a, b *line) bool {
	ab, _, _ := a.styleFrac()
	bb, _, _ := b.styleFrac()
	return a.size >= b.size*0.97 && a.size <= b.size*1.03 && (ab > 0.5) == (bb > 0.5)
}

// extractTitle removes the title block from the first page — title,
// subtitle, authors with affiliations and e-mail addresses, and the date —
// and returns it as metadata.
func (d *doc) extractTitle() ast.Meta {
	var m ast.Meta
	if len(d.pages) == 0 {
		return m
	}
	lines := d.pageLines(0)
	idx := d.titleBlock(lines, d.pages[0].h)
	if idx == nil {
		return m
	}
	var title []*line
	for _, k := range idx {
		title = append(title, lines[k])
	}
	m.Title = plainText(d.inlines(title, inlineOpts{noBold: true, noItalic: true, noNotes: true}))
	consumed := map[*word]bool{}
	for _, l := range title {
		for _, w := range l.words {
			consumed[w] = true
		}
	}
	tb := &titleScan{d: d, m: &m, tsize: title[0].size, centre: (title[0].x0 + title[0].x1) / 2}
	last := title[len(title)-1]
	region := d.titleRegion(title)
	for k, l := range region {
		if k >= 14 || l.y0-last.y1 > 4*max(d.body, l.size) || l.y0 > 0.7*d.pages[0].h {
			break
		}
		tb.next = nil
		if k+1 < len(region) {
			tb.next = region[k+1]
		}
		if !tb.take(l, last) {
			break
		}
		for _, w := range l.words {
			consumed[w] = true
		}
		last = l
	}
	if len(tb.subtitle) > 0 {
		m.Subtitle = plainText(d.inlines(tb.subtitle, inlineOpts{noBold: true, noItalic: true, noNotes: true}))
	}
	d.dropWords(consumed)
	return m
}

// titleRegion returns the page lines below the title, cut to the column
// that holds the title: a title block may sit in the first column of a
// two-column page or span the page above the columns.
func (d *doc) titleRegion(title []*line) []*line {
	last := title[len(title)-1]
	x0, x1 := math.Inf(1), math.Inf(-1)
	if f := last.fl; f != nil {
		x0, x1 = f.left, f.right
	}
	for _, l := range title {
		x0, x1 = min(x0, l.x0), max(x1, l.x1)
	}
	pad := 0.5 * d.body
	var out []*line
	for _, pl := range d.pages[0].lines {
		if pl.y0 < last.y1-0.3*last.size {
			continue
		}
		var ws []*word
		for _, w := range pl.words {
			if w.cx() >= x0-pad && w.cx() <= x1+pad {
				ws = append(ws, w)
			}
		}
		if len(ws) > 0 {
			out = append(out, pl.subLine(ws))
		}
	}
	return out
}

// dropWords removes words from the flows and the pages.
func (d *doc) dropWords(words map[*word]bool) {
	if len(words) == 0 {
		return
	}
	for _, f := range d.flows {
		items := f.items[:0]
		for _, it := range f.items {
			if it.l != nil {
				ws := it.l.words[:0:0]
				for _, w := range it.l.words {
					if !words[w] {
						ws = append(ws, w)
					}
				}
				if len(ws) == 0 {
					continue
				}
				if len(ws) != len(it.l.words) {
					it.l.words = ws
					it.l.markScripts()
				}
			}
			items = append(items, it)
		}
		f.items = items
	}
	for _, p := range d.pages {
		p.dropWords(words)
	}
}

// titleScan classifies the lines below the title.
type titleScan struct {
	d        *doc
	m        *ast.Meta
	tsize    float64
	centre   float64 // horizontal centre of the title
	next     *line   // the line after the one being classified
	subtitle []*line
	marks    [][]string // affiliation marks of each author
	cols     []float64  // centres of author columns, when side by side
}

// take consumes line l of the title block, or reports false at the first
// line that is not part of it.
func (t *titleScan) take(l, prev *line) bool {
	d, m := t.d, t.m
	text := l.text()
	if l.role().headingLevel() > 0 || abstractLabelRe.MatchString(text) || runInAbstractRe.MatchString(text) {
		return false
	}
	if len(t.cols) >= 2 && l.y0-prev.y1 < 1.6*l.size && l.size <= d.body*1.05 && t.columnLine(l) {
		return true
	}
	if parts := authorParts(l, d.body); parts != nil {
		if groups := splitAtGaps(l.words, 1.5); len(groups) == len(parts) && len(parts) >= 2 && len(m.Authors) == 0 {
			// Authors side by side, each heading a column of affiliation
			// lines.
			for _, g := range groups {
				t.cols = append(t.cols, (g[0].x0+g[len(g)-1].x1)/2)
			}
		}
		for _, p := range parts {
			a := ast.Author{Name: p.name}
			for _, mk := range p.marks {
				if mk == "*" || mk == "∗" {
					a.Corresponding = true
				}
			}
			m.Authors = append(m.Authors, a)
			t.marks = append(t.marks, p.marks)
		}
		return true
	}
	switch {
	case len(m.Authors) == 0 && len(t.subtitle) == 0 && l.size >= d.body*1.08 && l.size < t.tsize*0.97 &&
		!d.numberedHeading(l) && d.uniqueStyle(l) && !t.leadsBody(l):
		t.subtitle = append(t.subtitle, l)
		return true
	case len(m.Authors) == 0 && len(t.subtitle) == 0 && t.italicSubtitle(l):
		t.subtitle = append(t.subtitle, l)
		return true
	case len(t.subtitle) > 0 && len(m.Authors) == 0 && sameStyle(l, t.subtitle[0]) && l.y0-prev.y1 < 0.9*l.size:
		t.subtitle = append(t.subtitle, l)
		return true
	case m.Date == "" && isDate(text):
		m.Date = text
		return true
	case t.email(l):
		return true
	case len(m.Authors) > 0 && t.affiliation(l):
		return true
	}
	return t.byline(l)
}

// columnLine assigns the pieces of a line below side-by-side authors to
// the author above each piece: e-mail addresses and affiliation lines,
// which may wrap.
func (t *titleScan) columnLine(l *line) bool {
	spacing := t.cols[1] - t.cols[0]
	groups := make([][]*word, len(t.cols))
	for _, w := range l.words {
		c := 0
		for c+1 < len(t.cols) && w.cx() > (t.cols[c]+t.cols[c+1])/2 {
			c++
		}
		// A word straddling the boundary between two columns belongs to
		// text running across them.
		if c > 0 && w.x0 < (t.cols[c-1]+t.cols[c])/2-0.3*w.w() || c+1 < len(t.cols) && w.x1 > (t.cols[c]+t.cols[c+1])/2+0.3*w.w() {
			return false
		}
		if math.Abs(w.cx()-t.cols[c]) > spacing || c >= len(t.m.Authors) {
			return false
		}
		groups[c] = append(groups[c], w)
	}
	for c, g := range groups {
		if len(g) == 0 {
			continue
		}
		a := &t.m.Authors[c]
		text := strings.TrimSpace(wordsText(g))
		if s := strings.Trim(text, "<>()"); emailRe.MatchString(s) && a.Email == "" {
			a.Email = s
			continue
		}
		if n := len(a.Affiliations); n > 0 {
			last := a.Affiliations[n-1]
			if strings.HasSuffix(last, "\u00AD") {
				a.Affiliations[n-1] = strings.TrimSuffix(last, "\u00AD") + text
			} else {
				a.Affiliations[n-1] = last + " " + text
			}
			continue
		}
		a.Affiliations = append(a.Affiliations, text)
	}
	return true
}

// leadsBody reports whether body text follows l at the usual distance,
// as below a section heading (a subtitle is followed by authors, a date or
// space).
func (t *titleScan) leadsBody(l *line) bool {
	n := t.next
	return n != nil && math.Abs(n.size-t.d.body) < 0.1*t.d.body && n.y0-l.y1 < 1.2*n.size && len(n.words) >= 4 && !isDate(n.text())
}

// italicSubtitle reports whether l is a short italic line centred under
// the title ("Between A and B").
func (t *titleScan) italicSubtitle(l *line) bool {
	_, it, _ := l.styleFrac()
	if it < 0.9 || len(l.words) > 15 || l.size < t.d.body*0.9 {
		return false
	}
	c := (l.x0 + l.x1) / 2
	return math.Abs(c-t.centre) < 2*l.size
}

// email assigns the addresses on an e-mail line to the authors.
func (t *titleScan) email(l *line) bool {
	var addrs []string
	for _, w := range l.words {
		s := strings.Trim(w.text, "<>()[],;")
		if s == "" {
			continue
		}
		if !emailRe.MatchString(s) {
			return false
		}
		addrs = append(addrs, s)
	}
	if len(addrs) == 0 || len(addrs) > 12 {
		return false
	}
	as := t.m.Authors
	for _, addr := range addrs {
		target := -1
		local := strings.ToLower(addr[:strings.IndexByte(addr, '@')])
		for i, a := range as {
			if a.Email != "" {
				continue
			}
			for _, part := range strings.Fields(strings.ToLower(a.Name)) {
				if len([]rune(part)) >= 3 && strings.Contains(local, asciiFold(part)) {
					target = i
				}
			}
		}
		if target < 0 {
			for i, a := range as {
				if a.Email == "" && (a.Corresponding || target < 0) {
					target = i
					if a.Corresponding {
						break
					}
				}
			}
		}
		if target < 0 {
			if len(as) > 0 {
				break
			}
			// An address without author names: keep it as a nameless
			// author so the address is not lost.
			t.m.Authors = append(t.m.Authors, ast.Author{Email: addr})
			continue
		}
		as[target].Email = addr
	}
	return true
}

// affiliation assigns an affiliation line ("1Faculty of …", "Department
// of …") to the authors carrying its mark, or to all authors.
func (t *titleScan) affiliation(l *line) bool {
	if l.size > t.d.body*1.05 {
		return false
	}
	words := l.words
	mark := ""
	if w := words[0]; w.sup && len(words) > 1 {
		mark, words = markerKey(w.text), words[1:]
	} else if g := gluedMarkerRe.FindStringSubmatch(w.text); g != nil && t.hasMark(g[1]) {
		_, tail := splitWord(w, len(g[1]))
		mark, words = g[1], append([]*word{tail}, words[1:]...)
	} else if len(words) > 1 && t.hasMark(w.text) && words[1].x0-w.x1 < 0.3*l.size {
		// A mark set tight against the text, in a font without a
		// distinct superscript form.
		mark, words = w.text, words[1:]
	}
	text := strings.TrimSpace(wordsText(words))
	if mark == "" && !affiliationRe.MatchString(text) {
		return false
	}
	for i := range t.m.Authors {
		if mark == "" || i < len(t.marks) && contains(t.marks[i], mark) {
			t.m.Authors[i].Affiliations = append(t.m.Authors[i].Affiliations, text)
		}
	}
	return true
}

func (t *titleScan) hasMark(mk string) bool {
	for _, ms := range t.marks {
		if contains(ms, mk) {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// byline consumes a line like "Jane Roe · 15 January 2026" or "Prepared by
// the Data Team | March 2026": short pieces separated by a middle dot, a
// bar or a dash, one of which is a date. The other pieces are authors.
func (t *titleScan) byline(l *line) bool {
	pieces := bylineSepRe.Split(l.text(), -1)
	if len(pieces) < 2 || len(pieces) > 4 {
		return false
	}
	date := ""
	var names []string
	for _, p := range pieces {
		p = strings.TrimSpace(p)
		switch {
		case p == "":
		case date == "" && isDate(p):
			date = p
		case len(strings.Fields(p)) <= 5 && strings.IndexFunc(p, unicode.IsDigit) < 0:
			names = append(names, strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(p, "by "), "By ")))
		default:
			return false
		}
	}
	if date == "" {
		return false
	}
	if t.m.Date == "" {
		t.m.Date = date
	}
	if len(t.m.Authors) == 0 {
		for _, n := range names {
			t.m.Authors = append(t.m.Authors, ast.Author{Name: n})
		}
	}
	return true
}

var (
	emailRe     = regexp.MustCompile(`^[\p{L}0-9._%+\-]+@[\p{L}0-9.\-]+\.\p{L}{2,}$`)
	bylineSepRe = regexp.MustCompile(`\s+[·•|–—]\s+`)
	monthNames  = `(?:jan(?:uary|uar|vār(?:is|a)|\.)?|feb(?:ruary|ruar|ruār(?:is|a)|\.)?|mar(?:ch|ts|ta|\.)?|märz|apr(?:il|īl(?:is|a)|\.)?|may|mai(?:js|ja)?|jun(?:e|i|ijs|ija|\.)?|jūn(?:ijs|ija)|jul(?:y|i|ijs|ija|\.)?|jūl(?:ijs|ija)|aug(?:ust|usts|usta|\.)?|sep(?:tember|tembris|tembra|t\.|\.)?|oct(?:ober|\.)?|okt(?:ober|obris|obra|\.)?|nov(?:ember|embris|embra|\.)?|dec(?:ember|embris|embra|\.)?|dez(?:ember)?)`
	dateRe      = regexp.MustCompile(`(?i)^(?:` +
		`\d{4}-\d{1,2}-\d{1,2}` +
		`|\d{1,2}[./]\d{1,2}[./]\d{2,4}\.?` +
		`|\d{1,2}\.?\s+` + monthNames + `,?\s+\d{4}\.?` +
		`|` + monthNames + `\s+\d{1,2}(?:st|nd|rd|th)?,?\s+\d{4}` +
		`|` + monthNames + `\s+\d{4}` +
		`|\d{4}\.\s*gada\s+\d{1,2}\.\s*` + monthNames +
		`)$`)
)

// isDate reports whether s is a date and nothing else.
func isDate(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) <= 40 && dateRe.MatchString(s)
}

// asciiFold removes diacritics from Latin letters for loose matching.
func asciiFold(s string) string {
	var sb strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

var affiliationRe = regexp.MustCompile(`(?i)\b(university|universitāte|universit[äa]t|université|universidad|universit|institute|institūts|institut|department|katedra|faculty|fakultāte|laborator|college|school|centre|center|academy|akadēmija|hospital|corporation|inc\.|ltd|gmbh)\b`)

var nameParticles = map[string]bool{"von": true, "van": true, "de": true, "der": true, "den": true, "la": true,
	"le": true, "da": true, "di": true, "du": true, "del": true, "dos": true, "bin": true, "ibn": true, "ter": true}

// authorPart is one name on an author line with its affiliation marks.
type authorPart struct {
	name  string
	marks []string
}

var andWords = map[string]bool{"and": true, "&": true, "un": true, "und": true, "et": true, "y": true, "e": true, "ir": true, "ja": true, "i": true}

// authorParts parses a line of author names ("Jane Roe1,*, John Doe2 and
// Ann Lee"), keeping the superscript affiliation marks of each name. Wide
// gaps separate names too. It returns nil when the line is not made of
// person names.
func authorParts(l *line, body float64) []authorPart {
	if l.size < body*0.8 || l.size > body*1.35 || l.chars() > 200 {
		return nil
	}
	if b, _, _ := l.styleFrac(); b > 0.6 {
		return nil
	}
	var parts []authorPart
	var cur authorPart
	var prev *word
	flush := func() {
		cur.name = strings.TrimSpace(strings.Trim(strings.TrimSpace(cur.name), "*†‡,;"))
		if cur.name != "" || len(cur.marks) > 0 && len(parts) > 0 {
			if cur.name == "" {
				parts[len(parts)-1].marks = append(parts[len(parts)-1].marks, cur.marks...)
			} else {
				parts = append(parts, cur)
			}
		}
		cur, prev = authorPart{}, nil
	}
	for _, ws := range splitAtGaps(l.words, 1.5) {
		for _, w := range ws {
			if w.sup {
				if cur.name == "" && len(parts) == 0 {
					return nil // "1Department of …": an affiliation line
				}
				cur.marks = append(cur.marks, splitMarks(w.text)...)
				continue
			}
			text := w.text
			sep := strings.HasSuffix(text, ",") || strings.HasSuffix(text, ";")
			core := strings.TrimRight(text, ",;")
			if andWords[strings.ToLower(core)] {
				flush()
				continue
			}
			if core != "" {
				if prev != nil && spaceBefore(prev, w) {
					cur.name += " "
				}
				cur.name += core
				prev = w
			}
			if sep {
				flush()
			}
		}
		flush()
	}
	if len(parts) == 0 || len(parts) > 12 {
		return nil
	}
	for _, p := range parts {
		if !personName(p.name) || affiliationRe.MatchString(p.name) {
			return nil
		}
	}
	return parts
}

// splitMarks splits a superscript like "1,2" or "1*" into marks.
func splitMarks(s string) []string {
	var out []string
	digits := ""
	for _, r := range s {
		switch {
		case unicode.IsDigit(r):
			digits += string(r)
			continue
		case digits != "":
			out = append(out, digits)
			digits = ""
		}
		if r != ',' && r != ' ' {
			out = append(out, string(r))
		}
	}
	if digits != "" {
		out = append(out, digits)
	}
	return out
}

// uniqueStyle reports whether no text outside the neighbourhood of l
// shares its size and weight (headings repeat their style, a subtitle does
// not). Words are compared, not lines: a page line may join text of two
// columns.
func (d *doc) uniqueStyle(l *line) bool {
	lb, _, _ := l.styleFrac()
	for _, q := range d.pages {
		for _, w := range q.words {
			if w.size < l.size*0.97 || w.size > l.size*1.03 || w.bold != (lb > 0.5) || !hasLetter(w.text) {
				continue
			}
			if q.idx == l.page && w.y0 >= l.y0-2*l.size && w.y0 <= l.y1+3*l.size {
				continue
			}
			return false
		}
	}
	return true
}

func (d *doc) numberedHeading(l *line) bool {
	return len(l.words) > 1 && numberingDepth(l.words[0].text) > 0
}

func plainText(ins []ast.Inline) string {
	return strings.Join(strings.Fields(ast.PlainText(ins)), " ")
}

var abstractLabelRe = regexp.MustCompile(`(?i)^(abstract|anotācija|anotacija|kopsavilkums|zusammenfassung|kurzfassung|résumé|resumé|resumen|riassunto|resumo|samenvatting|streszczenie|abstrakt|tiivistelmä|sammanfattning|sammendrag|santrauka|kokkuvõte|аннотация|анотація|реферат)\s*[.:—–-]?\s*$`)

var keywordLabelRe = regexp.MustCompile(`(?i)^\s*(keywords|key words|index terms|atslēgvārdi|atslēgas vārdi|schlüsselwörter|stichwörter|mots[- ]clés|palabras clave|parole chiave|palavras[- ]chave|trefwoorden|słowa kluczowe|klíčová slova|avainsanat|nyckelord|nøgleord|nøkkelord|raktažodžiai|märksõnad|ключевые слова)\s*[:—–.-]\s*`)

// extractFrontMatter moves the abstract — an "Abstract" heading with the
// paragraphs below it, or a paragraph starting with a bold "Abstract."
// label — and a "Keywords: …" paragraph from the start of the document into
// the metadata.
func extractFrontMatter(blocks []ast.Block, m *ast.Meta) []ast.Block {
	out := make([]ast.Block, 0, len(blocks))
	i := 0
	for ; i < len(blocks) && i < 12; i++ {
		switch b := blocks[i].(type) {
		case *ast.Heading:
			if len(m.Abstract) > 0 || !abstractLabelRe.MatchString(plainText(b.Inlines)) {
				return append(out, blocks[i:]...)
			}
			j := i + 1
			for ; j < len(blocks) && len(m.Abstract) < 4; j++ {
				if _, ok := blocks[j].(*ast.Heading); ok {
					break
				}
				if takeKeywords(blocks[j], m) {
					j++
					break
				}
				m.Abstract = append(m.Abstract, unwrapQuote(blocks[j])...)
			}
			i = j - 1
		case *ast.Para:
			if takeKeywords(b, m) {
				continue
			}
			if len(m.Abstract) == 0 && abstractLabelRe.MatchString(plainText(b.Inlines)) {
				// The label on a line of its own, tagged as a paragraph.
				j := i + 1
				for ; j < len(blocks) && len(m.Abstract) < 4; j++ {
					if _, ok := blocks[j].(*ast.Heading); ok {
						break
					}
					if takeKeywords(blocks[j], m) {
						j++
						break
					}
					m.Abstract = append(m.Abstract, unwrapQuote(blocks[j])...)
				}
				i = j - 1
				continue
			}
			if len(m.Abstract) == 0 {
				if abs := runInAbstract(b); abs != nil {
					m.Abstract = abs
					continue
				}
			}
			out = append(out, b)
		default:
			out = append(out, b)
		}
	}
	return append(out, blocks[i:]...)
}

// unwrapQuote returns the content of a block quotation (abstracts are
// often indented on both sides), or the block itself.
func unwrapQuote(b ast.Block) []ast.Block {
	if q, ok := b.(*ast.BlockQuote); ok {
		return q.Blocks
	}
	return []ast.Block{b}
}

// takeKeywords parses a "Keywords: a, b" paragraph into m.Keywords (unless
// the file metadata already lists keywords) and reports whether b was one.
func takeKeywords(b ast.Block, m *ast.Meta) bool {
	p, ok := b.(*ast.Para)
	if !ok {
		return false
	}
	text := plainText(p.Inlines)
	loc := keywordLabelRe.FindStringIndex(text)
	if loc == nil || len(text) > 600 {
		return false
	}
	if len(m.Keywords) == 0 {
		for _, k := range strings.FieldsFunc(text[loc[1]:], func(r rune) bool { return r == ',' || r == ';' || r == '·' || r == '•' }) {
			if k = strings.TrimRight(strings.TrimSpace(k), "."); k != "" {
				m.Keywords = append(m.Keywords, k)
			}
		}
	}
	return true
}

// runInAbstract returns the abstract of a paragraph that starts with an
// "Abstract." or "Abstract—" label (usually bold or italic), or nil.
func runInAbstract(p *ast.Para) []ast.Block {
	if len(p.Inlines) < 2 || !runInAbstractRe.MatchString(plainText(p.Inlines)) {
		return nil
	}
	var rest []ast.Inline
	switch first := p.Inlines[0].(type) {
	case *ast.Strong, *ast.Emph:
		label := strings.TrimSpace(ast.PlainText(ast.InlineChildren(first)))
		if !abstractLabelRe.MatchString(label) {
			return nil
		}
		rest = ast.TrimInlines(p.Inlines[1:])
	case *ast.Text:
		loc := runInAbstractRe.FindStringIndex(first.Value)
		if loc == nil {
			return nil
		}
		rest = append([]ast.Inline{&ast.Text{Value: first.Value[loc[1]:]}}, p.Inlines[1:]...)
	default:
		return nil
	}
	if len(rest) == 0 {
		return nil
	}
	if t, ok := rest[0].(*ast.Text); ok {
		v := strings.TrimLeft(t.Value, " .:—–-")
		rest = append([]ast.Inline{&ast.Text{Value: v}}, rest[1:]...)
	}
	return []ast.Block{&ast.Para{Inlines: ast.TrimInlines(rest)}}
}

var runInAbstractRe = regexp.MustCompile(`(?i)^\s*(abstract|anotācija|kopsavilkums|zusammenfassung|résumé|resumen|riassunto|resumo|streszczenie|abstrakt|tiivistelmä|sammanfattning|santrauka|kokkuvõte|аннотация)\s*[.:—–-]\s*`)

// personName reports whether s looks like a personal name: two to five
// capitalised words (particles such as "van" aside), without digits.
func personName(s string) bool {
	f := strings.Fields(s)
	if len(f) < 2 || len(f) > 5 {
		return false
	}
	capped := 0
	for _, w := range f {
		if nameParticles[strings.ToLower(w)] {
			continue
		}
		r := firstRune(w)
		if !unicode.IsUpper(r) {
			return false
		}
		if strings.IndexFunc(w, func(r rune) bool { return unicode.IsDigit(r) || r == '@' || r == '/' }) >= 0 {
			return false
		}
		capped++
	}
	return capped >= 2
}

// coverLabels maps the labels of cover-page fields to metadata fields.
var coverLabels = map[string]string{
	"version": "version", "versija": "version", "revision": "version", "rev.": "version", "ver.": "version",
	"status": "status", "statuss": "status",
	"date": "date", "datums": "date", "datum": "date", "issued": "date", "published": "date",
	"author": "author", "authors": "author", "autors": "author", "autori": "author", "autor": "author",
	"prepared by": "author", "written by": "author", "sagatavoja": "author", "sagatavotājs": "author", "erstellt von": "author",
	"classification": "classification", "klasifikācija": "classification", "confidentiality": "classification",
	"organization": "organization", "organisation": "organization", "company": "organization",
	"uzņēmums": "organization", "organizācija": "organization", "institution": "organization",
	"department": "department", "nodaļa": "department",
}

// coverLabel returns the metadata field of a cover label ("VERSION",
// "Prepared by:"), or "".
func coverLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), ":")))
	return coverLabels[strings.Join(strings.Fields(s), " ")]
}

func isUpperLabel(ws []*word) bool {
	for _, w := range ws {
		for _, r := range w.text {
			if unicode.IsLower(r) {
				return false
			}
		}
	}
	return true
}

// coverMeta reads the label/value fields of a title page — "VERSION 2.1"
// on one line, "Date: 3 May 2026", or a row of labels above a row of
// values — into the metadata and removes them. When the first page is a
// cover (no section text), a lone paragraph on it becomes the summary.
func (d *doc) coverMeta() {
	if len(d.pages) < 2 || d.meta.Title == "" {
		return
	}
	lines := d.pageLines(0)
	m := &d.meta
	drop := map[*line]bool{}
	set := func(field, value string) bool {
		value = strings.TrimSpace(value)
		if value == "" || len(strings.Fields(value)) > 10 {
			return false
		}
		switch field {
		case "version":
			m.Version = value
		case "status":
			m.Status = value
		case "date":
			m.Date = value
		case "author":
			if len(m.Authors) == 0 {
				for _, n := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' }) {
					m.Authors = append(m.Authors, ast.Author{Name: strings.TrimSpace(n)})
				}
			}
		case "classification":
			m.Classification = value
		case "organization":
			m.Organization = value
		case "department":
			m.Department = value
		}
		return true
	}
	for k, l := range lines {
		if drop[l] || l.role().headingLevel() > 0 {
			continue
		}
		groups := splitAtGaps(l.words, 1.5)
		// A row of labels with the values on the next line.
		if len(groups) >= 2 && isUpperLabel(l.words) && k+1 < len(lines) {
			labels := make([]string, len(groups))
			ok := true
			for i, g := range groups {
				if labels[i] = coverLabel(wordsText(g)); labels[i] == "" {
					ok = false
					break
				}
			}
			next := lines[k+1]
			if ok && next.y0-l.y1 < 2*l.size {
				values := make([][]*word, len(groups))
				for _, w := range next.words {
					c := 0
					for i, g := range groups {
						if w.x0 >= g[0].x0-0.5*l.size {
							c = i
						}
					}
					values[c] = append(values[c], w)
				}
				for i := range groups {
					set(labels[i], wordsText(values[i]))
				}
				drop[l], drop[next] = true, true
				continue
			}
		}
		// "LABEL value" or "Label: value" on one line.
		for n := 1; n <= 3 && n < len(l.words); n++ {
			head := l.words[:n]
			label := wordsText(head)
			field := coverLabel(label)
			if field == "" {
				continue
			}
			gap := l.words[n].x0 - head[n-1].x1
			if (isUpperLabel(head) && gap >= 0.8*l.size || strings.HasSuffix(label, ":")) && set(field, wordsText(l.words[n:])) {
				drop[l] = true
			}
			break
		}
	}
	if len(drop) == 0 {
		return
	}
	d.dropLines(drop)
	d.coverSummary(d.pageLines(0))
}

// coverSummary turns the only paragraph left on a cover page (no headings,
// little text) into the summary.
func (d *doc) coverSummary(lines []*line) {
	if d.meta.Summary != "" || len(lines) == 0 || len(lines) > 12 {
		return
	}
	var para []*line
	for _, l := range lines {
		if d.isHeadingLine(l) && l.size > d.body*1.05 {
			return
		}
		if len(para) > 0 {
			prev := para[len(para)-1]
			if math.Abs(l.size-prev.size) > 0.1*l.size || l.base-prev.base > 1.6*l.size {
				if len(para) >= 2 || len(para[0].words) >= 6 {
					break
				}
				para = para[:0]
			}
		}
		para = append(para, l)
	}
	if len(para) == 0 || len(para) == 1 && len(para[0].words) < 6 {
		return
	}
	d.meta.Summary = plainText(d.inlines(para, inlineOpts{noNotes: true}))
	in := map[*line]bool{}
	for _, l := range para {
		in[l] = true
	}
	d.dropLines(in)
}
