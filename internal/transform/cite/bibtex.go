package cite

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// bibEntry is a raw BibTeX entry; field values keep their braces so names
// and titles can be analysed before LaTeX conversion.
type bibEntry struct {
	typ    string
	key    string
	fields map[string]string
	line   int
}

type bibParser struct {
	s      []byte
	i      int
	macros map[string]string
	warns  []string

	// lineOff/line cache the last line computation (offsets mostly grow).
	lineOff, line int
}

var monthMacros = map[string]string{
	"jan": "January", "feb": "February", "mar": "March", "apr": "April", "may": "May", "jun": "June",
	"jul": "July", "aug": "August", "sep": "September", "oct": "October", "nov": "November", "dec": "December",
}

// ParseBibTeX parses BibTeX and BibLaTeX data. Malformed entries are
// skipped with a warning; parsing continues with the next entry.
func ParseBibTeX(data []byte) ([]ast.Reference, []string) {
	p := &bibParser{s: bytes.TrimPrefix(data, utf8BOM), macros: map[string]string{}}
	for k, v := range monthMacros {
		p.macros[k] = v
	}
	entries := p.parse()
	byKey := map[string]*bibEntry{}
	var unique []*bibEntry
	for i := range entries {
		e := &entries[i]
		lk := strings.ToLower(e.key)
		if byKey[lk] != nil {
			p.warnf(e.line, "duplicate key %s ignored", e.key)
			continue
		}
		byKey[lk] = e
		unique = append(unique, e)
	}
	inheritCrossrefs(unique, byKey)
	refs := make([]ast.Reference, 0, len(unique))
	for _, e := range unique {
		refs = append(refs, bibToRef(e))
	}
	return refs, p.warns
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func (p *bibParser) lineAt(off int) int {
	off = min(off, len(p.s))
	if off < p.lineOff {
		p.lineOff, p.line = 0, 0
	}
	p.line += bytes.Count(p.s[p.lineOff:off], []byte{'\n'})
	p.lineOff = off
	return p.line + 1
}

func (p *bibParser) warnf(line int, format string, args ...any) {
	p.warns = append(p.warns, fmt.Sprintf("bibtex: line %d: ", line)+fmt.Sprintf(format, args...))
}

func (p *bibParser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

func (p *bibParser) skipWS() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r', '\f':
			p.i++
		case '%':
			// Comment lines between fields.
			for p.i < len(p.s) && p.s[p.i] != '\n' {
				p.i++
			}
		default:
			return
		}
	}
}

func isIdentByte(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\f', '"', '#', '%', '\'', '(', ')', ',', '=', '{', '}', '@':
		return false
	}
	return c > ' ' && c != 0x7f
}

func (p *bibParser) ident() string {
	start := p.i
	for p.i < len(p.s) && isIdentByte(p.s[p.i]) {
		p.i++
	}
	return string(p.s[start:p.i])
}

func (p *bibParser) parse() []bibEntry {
	var out []bibEntry
	for p.i < len(p.s) {
		at := bytes.IndexByte(p.s[p.i:], '@')
		if at < 0 {
			break
		}
		start := p.i + at
		p.i = start + 1
		p.skipWS()
		typ := strings.ToLower(p.ident())
		p.skipWS()
		if typ == "" || (p.peek() != '{' && p.peek() != '(') {
			continue // a stray "@", e.g. an e-mail address in a comment
		}
		closer := byte('}')
		if p.peek() == '(' {
			closer = ')'
		}
		p.i++
		if typ == "comment" || typ == "preamble" {
			p.skipBalanced(closer)
			continue
		}
		line := p.lineAt(start)
		if typ == "string" {
			if !p.parseString(closer) {
				p.warnf(line, "malformed @string definition skipped")
				p.recover(start)
			}
			continue
		}
		e, err := p.parseEntry(typ, closer)
		if err != "" {
			p.warnf(line, "malformed @%s entry skipped: %s", typ, err)
			p.recover(start)
			continue
		}
		e.line = line
		out = append(out, e)
	}
	return out
}

// skipBalanced moves past the group opened just before p.i.
func (p *bibParser) skipBalanced(closer byte) {
	depth := 1
	opener := byte('{')
	if closer == ')' {
		opener = '('
	}
	for p.i < len(p.s) {
		c := p.s[p.i]
		p.i++
		switch c {
		case opener:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return
			}
		}
	}
}

// recover resumes parsing at the next "@" that starts a line.
func (p *bibParser) recover(start int) {
	for i := start + 1; i < len(p.s); i++ {
		if p.s[i] != '@' {
			continue
		}
		j := i - 1
		for j >= 0 && (p.s[j] == ' ' || p.s[j] == '\t') {
			j--
		}
		if j < 0 || p.s[j] == '\n' || p.s[j] == '\r' {
			p.i = i
			return
		}
	}
	p.i = len(p.s)
}

func (p *bibParser) parseString(closer byte) bool {
	p.skipWS()
	name := strings.ToLower(p.ident())
	p.skipWS()
	if name == "" || p.peek() != '=' {
		return false
	}
	p.i++
	val, ok := p.value()
	if !ok {
		return false
	}
	p.skipWS()
	if p.peek() != closer {
		return false
	}
	p.i++
	p.macros[name] = val
	return true
}

func (p *bibParser) parseEntry(typ string, closer byte) (bibEntry, string) {
	e := bibEntry{typ: typ, fields: map[string]string{}}
	p.skipWS()
	start := p.i
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == ',' || c == closer || c == '\n' || c == '\r' || c == '=' || c == '{' || c == '}' {
			break
		}
		p.i++
	}
	e.key = strings.TrimSpace(string(p.s[start:p.i]))
	p.skipWS()
	switch {
	case p.peek() == '=' || e.key == "":
		return e, "missing citation key"
	case p.peek() == closer:
		p.i++
		return e, ""
	case p.peek() != ',':
		return e, "expected ',' after key " + e.key
	}
	p.i++
	for {
		p.skipWS()
		if p.i >= len(p.s) {
			return e, "unexpected end of input"
		}
		if p.peek() == closer {
			p.i++
			return e, ""
		}
		name := strings.ToLower(p.ident())
		if name == "" {
			return e, fmt.Sprintf("unexpected %q", p.peek())
		}
		p.skipWS()
		if p.peek() != '=' {
			return e, "expected '=' after field " + name
		}
		p.i++
		val, ok := p.value()
		if !ok {
			return e, "bad value for field " + name
		}
		if _, dup := e.fields[name]; !dup {
			e.fields[name] = val
		}
		p.skipWS()
		switch p.peek() {
		case ',':
			p.i++
		case closer:
			p.i++
			return e, ""
		default:
			return e, "expected ',' or end of entry after field " + name
		}
	}
}

// value reads a field value: braced or quoted strings, numbers and macro
// names joined with "#".
func (p *bibParser) value() (string, bool) {
	var sb strings.Builder
	for {
		p.skipWS()
		c := p.peek()
		switch {
		case c == '{':
			s, ok := p.delimited('{')
			if !ok {
				return "", false
			}
			sb.WriteString(s)
		case c == '"':
			s, ok := p.delimited('"')
			if !ok {
				return "", false
			}
			sb.WriteString(s)
		case c >= '0' && c <= '9':
			start := p.i
			for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
				p.i++
			}
			sb.Write(p.s[start:p.i])
		case isIdentByte(c):
			start := p.i
			name := p.ident()
			if v, ok := p.macros[strings.ToLower(name)]; ok {
				sb.WriteString(v)
			} else {
				p.warnf(p.lineAt(start), "undefined @string macro %q", name)
			}
		default:
			return "", false
		}
		p.skipWS()
		if p.peek() != '#' {
			return sb.String(), true
		}
		p.i++
	}
}

// delimited reads a braced or quoted string; braces nest and protect
// quotes, escaped braces (\{ \}) do not count.
func (p *bibParser) delimited(open byte) (string, bool) {
	p.i++
	start := p.i
	depth := 0
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '\\' && p.i+1 < len(p.s) && (p.s[p.i+1] == '{' || p.s[p.i+1] == '}') {
			p.i += 2
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				if open != '{' {
					return "", false
				}
				s := string(p.s[start:p.i])
				p.i++
				return s, true
			}
			depth--
		case '"':
			if open == '"' && depth == 0 {
				s := string(p.s[start:p.i])
				p.i++
				return s, true
			}
		}
		p.i++
	}
	return "", false
}

// inheritCrossrefs copies missing fields from crossref'd parent entries
// (a proceedings volume's title becomes the paper's booktitle).
func inheritCrossrefs(entries []*bibEntry, byKey map[string]*bibEntry) {
	for _, e := range entries {
		ref := strings.TrimSpace(stripBraces(e.fields["crossref"]))
		if ref == "" {
			continue
		}
		parent := byKey[strings.ToLower(ref)]
		if parent == nil || parent == e {
			continue
		}
		for k, v := range parent.fields {
			switch k {
			case "crossref", "ids", "key", "entryset":
				continue
			case "title", "subtitle":
				bk := "book" + k
				if e.fields[bk] == "" {
					e.fields[bk] = v
				}
				continue
			}
			if _, ok := e.fields[k]; !ok {
				e.fields[k] = v
			}
		}
	}
}

func stripBraces(s string) string {
	return strings.NewReplacer("{", "", "}", "").Replace(s)
}

// bibTypes maps BibTeX/BibLaTeX entry types to CSL types.
var bibTypes = map[string]string{
	"article": "article-journal", "suppperiodical": "article-journal",
	"book": "book", "mvbook": "book", "proceedings": "book", "mvproceedings": "book",
	"collection": "book", "mvcollection": "book", "reference": "book", "mvreference": "book",
	"manual": "book", "booklet": "pamphlet",
	"inbook": "chapter", "incollection": "chapter", "bookinbook": "chapter", "suppbook": "chapter",
	"suppcollection": "chapter", "inreference": "entry-encyclopedia",
	"inproceedings": "paper-conference", "conference": "paper-conference",
	"mastersthesis": "thesis", "phdthesis": "thesis", "thesis": "thesis",
	"techreport": "report", "report": "report",
	"online": "webpage", "electronic": "webpage", "www": "webpage",
	"unpublished": "manuscript", "misc": "document",
	"patent": "patent", "dataset": "dataset", "software": "software", "standard": "standard",
	"periodical": "periodical", "legislation": "legislation", "legal": "legislation",
	"jurisdiction": "legal_case", "review": "review",
}

var bibThesisTypes = map[string]string{
	"phdthesis": "PhD thesis", "phd": "PhD thesis", "mathesis": "Master's thesis",
	"mastersthesis": "Master's thesis", "masterthesis": "Master's thesis", "bathesis": "Bachelor's thesis",
	"bachelorthesis": "Bachelor's thesis", "candthesis": "Master's thesis",
}

// bibToRef converts a raw entry to a Reference.
func bibToRef(e *bibEntry) ast.Reference {
	raw := e.fields
	get := func(k string) string { return latexToUnicode(raw[k]) }
	withSub := func(main, sub string) string {
		m, s := get(main), get(sub)
		if s == "" {
			return m
		}
		if m == "" {
			return s
		}
		if strings.ContainsAny(m[len(m)-1:], "?!:.") {
			return m + " " + s
		}
		return m + ": " + s
	}
	r := ast.Reference{ID: e.key}
	typ := bibTypes[e.typ]
	if typ == "" {
		typ = "document"
	}
	switch strings.ToLower(get("entrysubtype")) {
	case "magazine":
		if typ == "article-journal" {
			typ = "article-magazine"
		}
	case "newspaper":
		if typ == "article-journal" {
			typ = "article-newspaper"
		}
	}
	kindField := get("type")
	switch e.typ {
	case "phdthesis":
		r.Genre = "PhD thesis"
	case "mastersthesis":
		r.Genre = "Master's thesis"
	}
	if kindField != "" {
		if g, ok := bibThesisTypes[strings.ToLower(kindField)]; ok {
			r.Genre = g
		} else {
			r.Genre = kindField
		}
	}

	r.Author = parseBibNames(raw["author"])
	r.Editor = parseBibNames(raw["editor"])
	r.Translator = parseBibNames(raw["translator"])
	r.Title = withSub("title", "subtitle")
	r.ShortTitle = get("shorttitle")
	journal := withSub("journaltitle", "journalsubtitle")
	if journal == "" {
		journal = get("journal")
	}
	book := withSub("booktitle", "booksubtitle")
	if book == "" {
		book = withSub("maintitle", "mainsubtitle")
	}
	switch typ {
	case "article-journal", "article-magazine", "article-newspaper", "periodical", "review":
		r.ContainerTitle = firstNonEmpty(journal, book)
	default:
		r.ContainerTitle = firstNonEmpty(book, journal)
	}
	r.CollectionTitle = get("series")
	r.Publisher = get("publisher")
	if r.Publisher == "" {
		r.Publisher = firstNonEmpty(get("institution"), get("school"), get("organization"))
	}
	r.PublisherPlace = firstNonEmpty(get("location"), get("address"))
	r.Edition = get("edition")
	r.Volume = get("volume")
	number, issue := get("number"), get("issue")
	switch typ {
	case "article-journal", "article-magazine", "article-newspaper", "periodical", "review":
		r.Issue = firstNonEmpty(number, issue)
	case "report", "patent", "standard", "dataset", "legislation", "document":
		r.Number = number
		r.Issue = issue
	default:
		r.Issue = issue
		if number != "" && r.CollectionTitle != "" {
			r.CollectionTitle += " " + number
		}
	}
	r.Page = normalizeRange(get("pages"))
	r.Event = get("eventtitle")
	r.EventPlace = get("venue")
	r.Version = get("version")
	r.Language = get("language")
	r.Note = firstNonEmpty(get("note"), get("addendum"))
	r.Abstract = get("abstract")
	r.ISBN = get("isbn")
	r.ISSN = get("issn")
	r.DOI = cleanDOI(rawURL(raw["doi"]))
	r.URL = rawURL(raw["url"])
	if hp := raw["howpublished"]; hp != "" {
		if u := extractURL(hp); u != "" {
			if r.URL == "" {
				r.URL = u
			}
		} else if r.Publisher == "" {
			r.Publisher = get("howpublished")
		} else {
			r.Medium = get("howpublished")
		}
	}
	if eprint := rawURL(raw["eprint"]); eprint != "" {
		kind := strings.ToLower(firstNonEmpty(get("eprinttype"), get("archiveprefix")))
		lowE := strings.ToLower(eprint)
		switch {
		case kind == "arxiv" || strings.HasPrefix(lowE, "arxiv:"):
			id := eprint
			if strings.HasPrefix(lowE, "arxiv:") {
				id = eprint[len("arxiv:"):]
			}
			if r.URL == "" {
				r.URL = "https://arxiv.org/abs/" + id
			}
			if r.ContainerTitle == "" {
				if typ == "document" || typ == "manuscript" || typ == "webpage" {
					typ = "article"
				}
				if r.Number == "" {
					r.Number = "arXiv:" + id
				}
				if r.Publisher == "" {
					r.Publisher = "arXiv"
				}
			}
		case kind == "pubmed" && r.URL == "":
			r.URL = "https://pubmed.ncbi.nlm.nih.gov/" + eprint + "/"
		case (kind == "hdl" || kind == "handle") && r.URL == "":
			r.URL = "https://hdl.handle.net/" + eprint
		case kind == "doi" && r.DOI == "":
			r.DOI = cleanDOI(eprint)
		}
	}
	r.Type = typ

	// Dates are read without LaTeX conversion: EDTF uses "~" for circa.
	rawDate := func(k string) string { return strings.TrimSpace(stripBraces(raw[k])) }
	if d := rawDate("date"); d != "" {
		r.Issued = parseDateString(d)
	} else if y := rawDate("year"); y != "" {
		r.Issued = parseDateString(y)
		if r.Issued.Year != 0 {
			if m := monthNumber(get("month")); m > 0 {
				r.Issued.Month = m
				if d := atoiSafe(get("day")); d >= 1 && d <= 31 {
					r.Issued.Day = d
				}
			}
		}
	}
	r.Accessed = parseDateString(rawDate("urldate"))
	return r
}

// rawURL strips braces, a \url wrapper and TeX escapes from a URL field
// without any other LaTeX conversion (URLs keep ~, _ and %).
func rawURL(s string) string {
	s = strings.TrimSpace(s)
	if u := extractURL(s); u != "" {
		return u
	}
	s = stripBraces(s)
	s = strings.NewReplacer(`\_`, "_", `\%`, "%", `\&`, "&", `\#`, "#", `\~`, "~", `\$`, "$").Replace(s)
	return strings.TrimSpace(s)
}

// extractURL returns the argument of \url{…} / \href{…} or a bare
// http(s) URL found in s.
func extractURL(s string) string {
	for _, cmd := range []string{`\url`, `\href`} {
		if i := strings.Index(s, cmd); i >= 0 {
			t := &texReader{s: s, i: i + len(cmd)}
			if raw, ok := t.rawGroup(); ok {
				return strings.TrimSpace(strings.NewReplacer(`\_`, "_", `\%`, "%", `\&`, "&", `\#`, "#").Replace(raw))
			}
		}
	}
	for _, scheme := range []string{"https://", "http://"} {
		if i := strings.Index(s, scheme); i >= 0 {
			end := i
			for end < len(s) && !unicode.IsSpace(rune(s[end])) && s[end] != '}' {
				end++
			}
			return strings.NewReplacer(`\_`, "_", `\%`, "%", `\&`, "&", `\#`, "#").Replace(s[i:end])
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// BibTeX names
// ---------------------------------------------------------------------------

// parseBibNames splits a BibTeX name list on " and " (outside braces).
func parseBibNames(raw string) []ast.Name {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []ast.Name
	for _, part := range splitAnd(raw) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.EqualFold(part, "others") {
			out = append(out, ast.Name{Literal: othersLiteral})
			continue
		}
		if n := parseBibName(part); n != (ast.Name{}) {
			out = append(out, n)
		}
	}
	return out
}

func splitAnd(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
		case ' ', '\t', '\n', '\r':
			if depth != 0 || i+4 > len(s) {
				continue
			}
			if strings.EqualFold(s[i+1:i+4], "and") && i+4 < len(s) && isBlank(s[i+4]) {
				out = append(out, s[start:i])
				start = i + 4
				i += 3
			}
		}
	}
	return append(out, s[start:])
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// parseBibName parses one name in any of the BibTeX forms "First von
// Last", "von Last, First" and "von Last, Jr, First". A name that is a
// single brace group ("{World Health Organization}") is literal.
func parseBibName(raw string) ast.Name {
	raw = strings.TrimSpace(raw)
	if isWholeGroup(raw) {
		return ast.Name{Literal: latexToUnicode(raw)}
	}
	parts := splitTopLevel(raw, ',')
	words := make([][]string, len(parts))
	for i, p := range parts {
		words[i] = bibWords(p)
	}
	var first, von, last, jr []string
	switch len(parts) {
	case 1:
		w := words[0]
		if len(w) == 0 {
			return ast.Name{}
		}
		vs, ve := -1, -1
		for i := 0; i < len(w)-1; i++ {
			if isLowerBibWord(w[i]) {
				if vs < 0 {
					vs = i
				}
				ve = i
			}
		}
		if vs < 0 {
			first, last = w[:len(w)-1], w[len(w)-1:]
		} else {
			first, von, last = w[:vs], w[vs:ve+1], w[ve+1:]
		}
	default:
		von, last = splitVonLast(words[0])
		if len(parts) == 2 {
			first = words[1]
		} else {
			jr, first = words[1], words[2]
		}
	}
	n := ast.Name{
		Family:   latexToUnicode(strings.Join(last, " ")),
		Given:    latexToUnicode(strings.Join(first, " ")),
		Particle: latexToUnicode(strings.Join(von, " ")),
		Suffix:   latexToUnicode(strings.Join(jr, " ")),
	}
	if n.Given == "" && n.Particle == "" && n.Suffix == "" && len(last) == 1 && isWholeGroup(last[0]) {
		return ast.Name{Literal: n.Family}
	}
	if n.Family == "" && n.Given != "" {
		n.Family, n.Given = n.Given, ""
	}
	return n
}

func splitVonLast(w []string) ([]string, []string) {
	if len(w) < 2 || !isLowerBibWord(w[0]) {
		return nil, w // the last word always belongs to the family name
	}
	ve := 0
	for i := 0; i < len(w)-1; i++ {
		if isLowerBibWord(w[i]) {
			ve = i
		}
	}
	return w[:ve+1], w[ve+1:]
}

// isWholeGroup reports whether s is one brace group: "{…}".
func isWholeGroup(s string) bool {
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 && i != len(s)-1 {
				return false
			}
		}
	}
	return true
}

func splitTopLevel(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
		case sep:
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// bibWords splits a name part into words at top-level whitespace and ties.
func bibWords(s string) []string {
	var out []string
	depth, start := 0, -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '{':
			depth++
		case '}':
			depth--
		}
		sepChar := depth == 0 && (c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '~')
		if sepChar {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

// isLowerBibWord decides whether a name word belongs to the "von" part:
// its first letter at brace level 0 — or the letter of a special
// character such as {\"u} — is lowercase. Other brace groups are caseless.
func isLowerBibWord(w string) bool {
	depth := 0
	for i := 0; i < len(w); i++ {
		c := w[i]
		switch {
		case c == '{':
			if depth == 0 && i+1 < len(w) && w[i+1] == '\\' {
				if lower, ok := specialCharCase(w, i+1); ok {
					return lower
				}
			}
			depth++
		case c == '}':
			depth--
		case depth > 0:
		case c == '\\':
			if lower, ok := specialCharCase(w, i); ok {
				return lower
			}
		default:
			r, _ := utf8.DecodeRuneInString(w[i:])
			if unicode.IsLetter(r) {
				return unicode.IsLower(r)
			}
		}
	}
	return false
}

// specialCharCase returns the case of the letter produced by the TeX
// command starting at w[i] (a backslash): \"u, \v{s}, \aa.
func specialCharCase(w string, i int) (lower, ok bool) {
	j := i + 1
	if j >= len(w) {
		return false, false
	}
	if !isASCIILetter(w[j]) {
		j++ // accent symbol such as \" or \'
	} else {
		k := j
		for k < len(w) && isASCIILetter(w[k]) {
			k++
		}
		cmd := w[j:k]
		if l, found := texLetters[cmd]; found {
			r, _ := utf8.DecodeRuneInString(l)
			return unicode.IsLower(r), true
		}
		if _, accent := accentMarks[cmd]; !accent {
			return false, false
		}
		j = k
	}
	for j < len(w) && (w[j] == '{' || w[j] == ' ') {
		j++
	}
	if j >= len(w) {
		return false, false
	}
	r, _ := utf8.DecodeRuneInString(w[j:])
	if !unicode.IsLetter(r) {
		return false, false
	}
	return unicode.IsLower(r), true
}
