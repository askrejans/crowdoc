package cite

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/askrejans/crowdoc/v2/ast"
)

// Parse reads a bibliography file. The format is chosen by the extension
// of name: .bib .bibtex .biblatex (BibTeX/BibLaTeX), .json (CSL-JSON), .yaml
// .yml (CSL-YAML), .ris (RIS) and .nbib (PubMed MEDLINE). Unknown
// extensions are recognised by content. Warnings report skipped or
// incomplete entries; the error is reserved for unreadable files.
func Parse(name string, data []byte) ([]ast.Reference, []string, error) {
	data = bytes.TrimPrefix(data, utf8BOM)
	switch strings.ToLower(filepath.Ext(name)) {
	case ".bib", ".bibtex", ".biblatex":
		refs, w := ParseBibTeX(data)
		return refs, w, nil
	case ".json", ".csljson":
		return ParseCSLJSON(data)
	case ".yaml", ".yml":
		return ParseCSLYAML(data)
	case ".ris":
		refs, w := ParseRIS(data)
		return refs, w, nil
	case ".nbib", ".medline":
		refs, w := ParseNBIB(data)
		return refs, w, nil
	}
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	switch {
	case len(trimmed) == 0:
		return nil, nil, nil
	case trimmed[0] == '@' || bytes.Contains(data, []byte("\n@")):
		refs, w := ParseBibTeX(data)
		return refs, w, nil
	case trimmed[0] == '[' || trimmed[0] == '{':
		return ParseCSLJSON(data)
	case bytes.HasPrefix(trimmed, []byte("TY  -")) || bytes.HasPrefix(trimmed, []byte("TY -")):
		refs, w := ParseRIS(data)
		return refs, w, nil
	case bytes.HasPrefix(trimmed, []byte("PMID-")):
		refs, w := ParseNBIB(data)
		return refs, w, nil
	case bytes.HasPrefix(trimmed, []byte("---")) || bytes.HasPrefix(trimmed, []byte("references:")) ||
		bytes.HasPrefix(trimmed, []byte("- ")):
		return ParseCSLYAML(data)
	}
	return nil, nil, fmt.Errorf("cite: unrecognised bibliography format: %s", name)
}

// ---------------------------------------------------------------------------
// CSL types
// ---------------------------------------------------------------------------

var cslTypes = map[string]bool{
	"article": true, "article-journal": true, "article-magazine": true, "article-newspaper": true,
	"bill": true, "book": true, "broadcast": true, "chapter": true, "classic": true, "collection": true,
	"dataset": true, "document": true, "entry": true, "entry-dictionary": true, "entry-encyclopedia": true,
	"event": true, "figure": true, "graphic": true, "hearing": true, "interview": true, "legal_case": true,
	"legislation": true, "manuscript": true, "map": true, "motion_picture": true, "musical_score": true,
	"pamphlet": true, "paper-conference": true, "patent": true, "performance": true, "periodical": true,
	"personal_communication": true, "post": true, "post-weblog": true, "regulation": true, "report": true,
	"review": true, "review-book": true, "software": true, "song": true, "speech": true, "standard": true,
	"thesis": true, "treaty": true, "webpage": true,
}

var typeAliases = map[string]string{
	"journal-article": "article-journal", "journal": "article-journal", "journalarticle": "article-journal",
	"magazine-article": "article-magazine", "newspaper-article": "article-newspaper",
	"book-chapter": "chapter", "book-section": "chapter", "booksection": "chapter", "section": "chapter",
	"incollection": "chapter", "inbook": "chapter", "book-part": "chapter",
	"proceedings-article": "paper-conference", "conference-paper": "paper-conference",
	"inproceedings": "paper-conference", "conference": "paper-conference", "conferencepaper": "paper-conference",
	"dissertation": "thesis", "phdthesis": "thesis", "mastersthesis": "thesis",
	"monograph": "book", "edited-book": "book", "reference-book": "book", "book-set": "book",
	"book-series": "book", "proceedings": "book",
	"reference-entry": "entry-encyclopedia", "encyclopedia-article": "entry-encyclopedia",
	"dictionary-entry": "entry-dictionary", "web": "webpage", "website": "webpage", "web-page": "webpage", "online": "webpage",
	"electronic": "webpage", "www": "webpage", "blog-post": "post-weblog", "blogpost": "post-weblog",
	"blog": "post-weblog", "forum-post": "post",
	"posted-content": "article", "preprint": "article",
	"data": "dataset", "data-set": "dataset", "computer-program": "software", "program": "software",
	"techreport": "report", "tech-report": "report", "misc": "document", "generic": "document",
	"other": "document", "unpublished": "manuscript", "statute": "legislation", "law": "legislation",
	"case": "legal_case",
}

// normType maps a type name from any supported format to a CSL type.
func normType(t string) string {
	s := strings.ToLower(strings.TrimSpace(t))
	if cslTypes[s] {
		return s
	}
	s = strings.NewReplacer(" ", "-", "_", "-").Replace(s)
	if cslTypes[s] {
		return s
	}
	if a, ok := typeAliases[s]; ok {
		return a
	}
	if u := strings.ReplaceAll(s, "-", "_"); cslTypes[u] {
		return u
	}
	return "document"
}

// ---------------------------------------------------------------------------
// Dates
// ---------------------------------------------------------------------------

var monthNames = buildMonthNames()

func buildMonthNames() map[string]int {
	m := map[string]int{"sept": 9}
	for _, l := range locales {
		for i := range 12 {
			m[strings.ToLower(strings.TrimSuffix(l.months[i], "."))] = i + 1
			m[strings.ToLower(strings.TrimSuffix(l.monthsShort[i], "."))] = i + 1
			if l.monthsDate != nil {
				m[strings.ToLower(l.monthsDate[i])] = i + 1
			}
		}
	}
	for i, n := range enMonths {
		m[strings.ToLower(n[:3])] = i + 1
	}
	return m
}

// monthNumber parses a month given as a number, name or abbreviation.
func monthNumber(s string) int {
	s = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), ".")))
	if s == "" {
		return 0
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n >= 1 && n <= 12 {
			return n
		}
		return 0
	}
	if n, ok := monthNames[s]; ok {
		return n
	}
	if len(s) > 3 {
		if n, ok := monthNames[s[:3]]; ok && strings.HasPrefix(enMonthsLower[n-1], s) {
			return n
		}
	}
	return 0
}

var enMonthsLower = func() [12]string {
	var out [12]string
	for i, m := range enMonths {
		out[i] = strings.ToLower(m)
	}
	return out
}()

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// parseDateString parses the date notations found in bibliography data:
// ISO 8601 / EDTF ("2020-05-01", "2020-05", "2020-05/2020-06", "2020~"),
// RIS ("2020/05/01/"), PubMed ("2020 May 1", "2020 Mar-Apr"), and prose
// ("May 1, 2020", "1 May 2020", "c. 1850"). Ranges keep their start.
// Text without a year becomes a literal date; "n.d." is no date.
func parseDateString(s string) ast.Date {
	s = strings.TrimSpace(s)
	if s == "" {
		return ast.Date{}
	}
	switch strings.ToLower(strings.Trim(s, "[]() ")) {
	case "n.d.", "n.d", "nd", "no date", "s.d.", "s.a.", "b.g.", "b. g.", "o.j.", "o. j.":
		return ast.Date{}
	}
	var d ast.Date
	low := strings.ToLower(s)
	for _, p := range []string{"circa", "ca.", "c.", "approx."} {
		if strings.HasPrefix(low, p) {
			d.Circa = true
			low = strings.TrimSpace(low[len(p):])
			break
		}
	}
	if strings.ContainsAny(low, "~?%") {
		d.Circa = strings.ContainsAny(low, "~%") || d.Circa
	}
	dotted := strings.Contains(low, ".") && !strings.ContainsAny(low, "-/")
	var pre, post []int // numbers before and after the year
	monthWord := 0
scan:
	for _, t := range dateTokens(low) {
		if n, err := strconv.Atoi(t); err == nil {
			switch {
			case len(t) >= 3 && d.Year == 0:
				d.Year = n
			case len(t) >= 3:
				break scan // a second year: the end of a range
			case d.Year == 0:
				pre = append(pre, n)
			default:
				post = append(post, n)
			}
			continue
		}
		if m := monthNumber(t); m > 0 {
			if monthWord == 0 {
				monthWord = m
			} else if d.Year != 0 {
				break scan // "2020 Mar-Apr": keep the first month
			}
		}
	}
	if d.Year == 0 {
		return ast.Date{Literal: s}
	}
	switch {
	case monthWord != 0:
		d.Month = monthWord
		if len(pre) > 0 {
			d.Day = pre[len(pre)-1] // "1 May 2020", "May 1, 2020"
		} else if len(post) > 0 {
			d.Day = post[0] // "2020 May 1"
		}
	case len(post) > 0:
		d.Month = post[0] // "2020-05-01"
		if len(post) > 1 {
			d.Day = post[1]
		}
	case len(pre) >= 2:
		a, b := pre[len(pre)-2], pre[len(pre)-1]
		if dotted || a > 12 {
			d.Day, d.Month = a, b // "01.05.2020"
		} else {
			d.Month, d.Day = a, b // "05/01/2020"
		}
	}
	if d.Month < 1 || d.Month > 12 {
		d.Month, d.Day = 0, 0
	}
	if d.Day < 0 || d.Day > 31 {
		d.Day = 0
	}
	return d
}

// dateTokens splits a date into number and word tokens.
func dateTokens(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = cur[:0]
		}
	}
	prevDigit := false
	for _, r := range s {
		isDigit := r >= '0' && r <= '9'
		switch {
		case unicode.IsLetter(r) || isDigit:
			if len(cur) > 0 && isDigit != prevDigit {
				flush()
			}
			cur = append(cur, r)
			prevDigit = isDigit
		default:
			flush()
		}
	}
	flush()
	return out
}

// ---------------------------------------------------------------------------
// Generated keys
// ---------------------------------------------------------------------------

// keyGen produces unique "<family><year>" keys for formats without ids.
type keyGen struct{ used map[string]int }

func (g *keyGen) next(r *ast.Reference) string {
	if g.used == nil {
		g.used = map[string]int{}
	}
	base := ""
	if ns, _, _ := primaryNames(r); len(ns) > 0 {
		n := ns[0]
		base = asciiKey(firstNonEmpty(n.Family, n.Literal, n.Given))
	}
	if base == "" {
		base = asciiKey(firstWord(r.Title))
	}
	if base == "" {
		base = "ref"
	}
	if r.Issued.Year != 0 {
		base += strconv.Itoa(r.Issued.Year)
	}
	n := g.used[base]
	g.used[base] = n + 1
	if n == 0 {
		return base
	}
	return base + suffixLetter(n)
}

func firstWord(s string) string {
	for _, w := range strings.Fields(s) {
		if k := asciiKey(w); len(k) > 3 {
			return w
		}
	}
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

var foldLetters = map[rune]string{
	'ł': "l", 'Ł': "l", 'ø': "o", 'Ø': "o", 'ß': "ss", 'æ': "ae", 'Æ': "ae", 'œ': "oe", 'Œ': "oe",
	'đ': "d", 'Đ': "d", 'ð': "d", 'þ': "th", 'ı': "i",
}

// asciiKey folds diacritics and keeps lowercase ASCII letters and digits:
// "Bērziņš" → "berzins".
func asciiKey(s string) string {
	var sb strings.Builder
	for _, r := range norm.NFD.String(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			sb.WriteRune(r + 'a' - 'A')
		default:
			if f, ok := foldLetters[r]; ok {
				sb.WriteString(f)
			}
		}
	}
	return sb.String()
}
