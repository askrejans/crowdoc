package cite

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// fmtr formats references and citations for one style and language.
type fmtr struct {
	st  *style
	loc *locale
	en  bool
}

func newFmtr(st *style, lang string) *fmtr {
	loc := getLocale(lang)
	return &fmtr{st: st, loc: loc, en: loc.lang == "en"}
}

// term looks a term up in the style's English overrides, the locale and
// finally English.
func (f *fmtr) term(key string) string {
	if f.en {
		if v, ok := f.st.enTerms[key]; ok {
			return v
		}
	}
	if v, ok := f.loc.terms[key]; ok {
		return v
	}
	return enTerms[key]
}

// termN picks the singular or plural form of a term.
func (f *fmtr) termN(key string, n int) string {
	if n > 1 {
		if v := f.term(key + "s"); v != "" {
			return v
		}
	}
	return f.term(key)
}

func (f *fmtr) builder() *builder {
	return &builder{moveInside: f.en && f.st.punctInQuotes}
}

// quote writes s in the style's quotation marks.
func (f *fmtr) quote(b *builder, s string) {
	open, close := f.loc.open, f.loc.close
	if f.en && f.st.singleQuotes {
		open, close = f.loc.open2, f.loc.close2
	}
	b.quoted(open, close, func() { b.text(s) })
}

// labelled renders a number with its term: "p. 33", "vol. 2", "33. lpp.".
func (f *fmtr) labelled(key, value string) string {
	n := 1
	if isMulti(value) {
		n = 2
	}
	t := f.termN(key, n)
	if t == "" {
		return value
	}
	if f.loc.postfix[key] {
		return postfixNumber(value) + " " + t
	}
	return t + " " + value
}

// isMulti reports whether a locator or page value names several pages.
func isMulti(v string) bool {
	return strings.ContainsAny(v, "–-—,&") || strings.Contains(v, " and ")
}

func capFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

func lowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || r == utf8.RuneError {
		return s
	}
	return string(unicode.ToLower(r)) + s[size:]
}

// ---------------------------------------------------------------------------
// Dates
// ---------------------------------------------------------------------------

func (f *fmtr) monthName(m int, short bool) string {
	if m < 1 || m > 12 {
		return ""
	}
	if short {
		if f.en && f.st.enMonthsShort != nil {
			return f.st.enMonthsShort[m-1]
		}
		return f.loc.monthsShort[m-1]
	}
	return f.loc.months[m-1]
}

// dayMonth renders "May 1" / "1 May" / "1. maijs" (or just the month).
func (f *fmtr) dayMonth(d ast.Date, short bool) string {
	mn := f.monthName(d.Month, short)
	if mn == "" {
		return ""
	}
	return f.loc.dayMonth(d.Day, d.Month, mn, short, f.st.dmy)
}

// date renders a date as precisely as known: "May 1, 2020", "May 2020",
// "2020", or the literal text.
func (f *fmtr) date(d ast.Date, short bool) string {
	if d.Year == 0 {
		return d.Literal
	}
	if d.Month < 1 || d.Month > 12 {
		return strconv.Itoa(d.Year)
	}
	return f.loc.fullDate(d.Year, d.Day, f.dayMonth(d, short), f.st.dmy)
}

// accessedDate renders a date used after "Retrieved"/"Accessed"; Latvian
// needs the locative month ("2021. gada 1. maijā").
func (f *fmtr) accessedDate(d ast.Date, short bool) string {
	if f.loc.monthsLoc != nil && d.Year != 0 && d.Month >= 1 && d.Month <= 12 && !short {
		dm := f.loc.monthsLoc[d.Month-1]
		if d.Day > 0 {
			dm = strconv.Itoa(d.Day) + ". " + dm
		}
		return f.loc.fullDate(d.Year, d.Day, dm, f.st.dmy)
	}
	return f.date(d, short)
}

// yearText is the year shown in citations and author-date entries,
// including any disambiguation suffix: "2020a", "n.d.", "n.d.-a".
func (f *fmtr) yearText(e *entry) string {
	d := e.ref.Issued
	switch {
	case d.Year != 0:
		s := strconv.Itoa(d.Year)
		if d.Year < 0 {
			s = strconv.Itoa(-d.Year) + " BCE"
		}
		if d.Circa {
			s = f.term("circa") + " " + s
		}
		return s + e.suffix
	case d.Literal != "":
		if e.suffix != "" {
			return d.Literal + "-" + e.suffix
		}
		return d.Literal
	}
	nd := f.term("no-date")
	if e.suffix != "" {
		nd += f.st.ndSuffixSep + e.suffix
	}
	return nd
}

// ---------------------------------------------------------------------------
// Pages, editions, identifiers
// ---------------------------------------------------------------------------

// normalizeRange turns any dash between page numbers into an en dash and
// drops the spaces around it: "45 - 67", "45--67" → "45–67".
func normalizeRange(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	var sb strings.Builder
	rs := []rune(p)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if isDash(r) {
			for i+1 < len(rs) && (isDash(rs[i+1]) || rs[i+1] == ' ') {
				i++
			}
			s := strings.TrimRight(sb.String(), " ")
			sb.Reset()
			sb.WriteString(s)
			sb.WriteRune('–')
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func isDash(r rune) bool {
	switch r {
	case '-', '–', '—', '‐', '‑', '−':
		return true
	}
	return false
}

// pageRange formats pages with the style's dash and range abbreviation.
func (f *fmtr) pageRange(p string) string {
	p = normalizeRange(p)
	if p == "" {
		return ""
	}
	segs := strings.Split(p, ",")
	for i, seg := range segs {
		seg = strings.TrimSpace(seg)
		if a, b, ok := strings.Cut(seg, "–"); ok {
			seg = a + f.st.pageDash + abbreviateRange(a, b, f.st.pageRange)
		}
		segs[i] = seg
	}
	return strings.Join(segs, ", ")
}

// pagesLabelled renders "p. 5" / "pp. 45–67" / "45.–67. lpp.".
func (f *fmtr) pagesLabelled(p string) string {
	pr := f.pageRange(p)
	if pr == "" {
		return ""
	}
	return f.labelled("page", pr)
}

// abbreviateRange shortens the end of a numeric range following the
// CSL page-range formats ("expanded", "minimal", "minimal-two",
// "chicago").
func abbreviateRange(a, b, format string) string {
	ai, err1 := strconv.Atoi(a)
	bi, err2 := strconv.Atoi(b)
	if err1 != nil || err2 != nil || bi <= ai || len(a) != len(b) {
		return b
	}
	switch format {
	case "minimal":
		return minimalEnd(a, b, 1)
	case "minimal-two":
		return minimalEnd(a, b, 2)
	case "chicago":
		switch {
		case ai < 100 || ai%100 == 0:
			return b
		case ai%100 < 10:
			return minimalEnd(a, b, 1)
		default:
			return minimalEnd(a, b, 2)
		}
	}
	return b
}

func minimalEnd(a, b string, keep int) string {
	i := 0
	for i < len(b) && a[i] == b[i] {
		i++
	}
	if len(b)-i < keep {
		i = max(len(b)-keep, 0)
	}
	return b[i:]
}

var editionWords = map[string]int{
	"first": 1, "second": 2, "third": 3, "fourth": 4, "fifth": 5, "sixth": 6, "seventh": 7,
	"eighth": 8, "ninth": 9, "tenth": 10, "revised": 0,
}

// editionNumber extracts the number of an edition ("2", "2nd", "Second
// edition"); 0 when the edition is not numeric.
func editionNumber(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	n := 0
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int(s[i]-'0')
		i++
	}
	if i > 0 {
		rest := strings.TrimLeft(s[i:], ". ")
		for _, suf := range []string{"st", "nd", "rd", "th", "e", "ª", ".ª", ":a", ":e", "-asis", "ed", "edition", "izd", "aufl", "uppl", "dr", "éd"} {
			if rest == "" || strings.HasPrefix(rest, suf) {
				return n
			}
		}
		return 0
	}
	first, _, _ := strings.Cut(s, " ")
	return editionWords[first]
}

var editionTerms = map[string]bool{
	"ed": true, "edn": true, "edition": true, "izd": true, "izdevums": true, "aufl": true,
	"auflage": true, "éd": true, "édition": true, "uppl": true, "upplaga": true, "wyd": true,
	"wydanie": true, "leid": true, "druk": true, "painos": true, "trükk": true,
}

// edition renders an edition statement ("2nd ed.", "2. izd."); first
// editions are omitted.
func (f *fmtr) edition(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	n := editionNumber(s)
	switch {
	case n == 1:
		return ""
	case n > 1:
		return f.loc.editionLabel(n, f.term("edition"))
	}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		if editionTerms[strings.TrimSuffix(w, ".")] {
			return s // already says "edition"
		}
	}
	return s + " " + f.term("edition")
}

// thesisLabel names the kind of a thesis from its genre.
func (f *fmtr) thesisLabel(genre string) string {
	switch thesisKind(genre) {
	case "master":
		return f.term("thesis-master")
	case "bachelor":
		return f.term("thesis-bachelor")
	case "phd":
		return f.term("thesis-phd")
	}
	if genre != "" {
		return genre
	}
	return f.term("thesis")
}

func thesisKind(genre string) string {
	g := strings.ToLower(genre)
	has := func(ws ...string) bool {
		for _, w := range ws {
			if strings.Contains(g, w) {
				return true
			}
		}
		return false
	}
	switch {
	case g == "":
		return ""
	case has("master", "mphil", "msc", "m.sc", "m.a.", "m.s.", "maģistr", "magist", "mémoire de master", "pro gradu"):
		return "master"
	case has("bachelor", "bsc", "b.sc", "b.a.", "bakalaur", "licencj", "kandidat", "diplom"):
		return "bachelor"
	case has("phd", "ph.d", "doctor", "doktor", "dissertation", "promocij", "thèse", "tesis doctoral", "proefschrift", "väitöskirja", "disertac"):
		return "phd"
	}
	return ""
}

// cleanDOI strips resolver prefixes from a DOI.
func cleanDOI(s string) string {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	for _, p := range []string{"https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "http://dx.doi.org/", "doi.org/", "doi:"} {
		if strings.HasPrefix(low, p) {
			return strings.TrimSpace(s[len(p):])
		}
	}
	return s
}

func doiURL(doi string) string { return "https://doi.org/" + cleanDOI(doi) }

// ---------------------------------------------------------------------------
// Reference kinds
// ---------------------------------------------------------------------------

// Formatting kinds shared by all styles.
const (
	kJournal    = "journal"
	kMagazine   = "magazine"
	kNewspaper  = "newspaper"
	kBlogPost   = "blogpost"
	kChapter    = "chapter"
	kConference = "conference"
	kBook       = "book"
	kThesis     = "thesis"
	kReport     = "report"
	kWebpage    = "webpage"
	kDataset    = "dataset"
	kSoftware   = "software"
	kPatent     = "patent"
	kStandard   = "standard"
	kLegal      = "legislation"
	kManuscript = "manuscript"
	kPreprint   = "preprint"
	kGeneric    = "generic"
)

// classify maps a CSL type to a formatting kind.
func classify(typ, container string) string {
	switch typ {
	case "article-journal", "review", "review-book", "periodical":
		return kJournal
	case "article":
		if container != "" {
			return kJournal
		}
		return kPreprint
	case "article-magazine":
		return kMagazine
	case "article-newspaper":
		return kNewspaper
	case "post-weblog":
		return kBlogPost
	case "post":
		if container != "" {
			return kBlogPost
		}
		return kWebpage
	case "chapter", "entry", "entry-dictionary", "entry-encyclopedia":
		return kChapter
	case "paper-conference", "speech", "presentation":
		return kConference
	case "book", "collection", "pamphlet", "classic", "musical_score", "map":
		return kBook
	case "thesis":
		return kThesis
	case "report":
		return kReport
	case "webpage":
		return kWebpage
	case "dataset":
		return kDataset
	case "software":
		return kSoftware
	case "patent":
		return kPatent
	case "standard":
		return kStandard
	case "legislation", "bill", "regulation", "treaty", "legal_case", "hearing":
		return kLegal
	case "manuscript":
		return kManuscript
	}
	return kGeneric
}

// inContainer reports whether the work is part of a larger titled work
// (its own title is quoted, the container's is italic).
func (e *entry) inContainer() bool {
	if e.ref.ContainerTitle == "" {
		return false
	}
	switch e.kind {
	case kJournal, kMagazine, kNewspaper, kBlogPost, kChapter, kConference:
		return true
	}
	return false
}

// standalone reports whether the title is set in italics.
func (e *entry) standalone() bool {
	if e.kind == kLegal {
		return false
	}
	return !e.inContainer()
}

// sameAsAuthor reports whether s repeats the (institutional) author.
func (e *entry) sameAsAuthor(s string) bool {
	if s == "" || len(e.names) != 1 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(familyDisplay(e.names[0])), strings.TrimSpace(s))
}

// placePublisher renders "City: Publisher" with whichever part is known.
func placePublisher(place, pub string) string {
	switch {
	case place != "" && pub != "":
		return place + ": " + pub
	case pub != "":
		return pub
	}
	return place
}

// joinNonEmpty joins the non-empty parts with sep.
func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// shortTitle is the title used in place of an author in citations:
// the explicit short title, else the main title up to four words.
func shortTitle(r *ast.Reference) string {
	if r.ShortTitle != "" {
		return r.ShortTitle
	}
	t := r.Title
	if i := strings.Index(t, ":"); i > 0 {
		t = t[:i]
	}
	words := strings.Fields(t)
	if len(words) > 4 {
		words = words[:4]
	}
	return strings.Join(words, " ")
}
