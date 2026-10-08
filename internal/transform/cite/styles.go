package cite

import (
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// StyleInfo describes a built-in citation style.
type StyleInfo struct {
	Name        string // "apa"
	Title       string // "APA 7th edition"
	Kind        string // "author-date", "numeric", "author-page"
	Description string
}

// style is the internal definition of a citation style: the in-text rules
// plus the bibliography entry formatter.
type style struct {
	info StyleInfo

	numeric    bool
	authorPage bool // MLA: author + page, no year

	// In-text name list rules.
	citeEtAlMin int    // use "et al." when a list has at least this many names
	parenAnd    string // conjunction inside parentheses ("&" for APA); "" = localised "and"
	serialComma bool   // "A, B, and C" in citations
	sortCites   bool   // sort the items of a parenthetical citation alphabetically

	yearSep     string // between author and year: ", " or " "
	ndSuffixSep string // between "n.d." and a disambiguation letter
	barePages   bool   // page locators without "p." (Chicago, MLA)
	vancouverCN bool   // numeric citation rendered as one bracket "[1, 3–5]"

	punctInQuotes bool // American style: ". ," go inside closing quotes (English only)
	singleQuotes  bool // British single quotes for article titles (English only)
	dmy           bool // English dates as "1 May 2020"
	pageRange     string
	pageDash      string

	enTerms       map[string]string
	enMonthsShort *[12]string

	entry func(f *fmtr, e *entry) []ast.Inline
}

var (
	ieeeMonths      = [12]string{"Jan.", "Feb.", "Mar.", "Apr.", "May", "Jun.", "Jul.", "Aug.", "Sep.", "Oct.", "Nov.", "Dec."}
	vancouverMonths = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
)

var styleOrder = []string{"apa", "chicago", "harvard", "ieee", "vancouver", "mla"}

var styles = map[string]*style{
	"apa": {
		info: StyleInfo{
			Name: "apa", Title: "APA 7th edition", Kind: "author-date",
			Description: "American Psychological Association, 7th edition: (Author, 2020) citations and an alphabetical reference list.",
		},
		citeEtAlMin: 3, parenAnd: "&", sortCites: true, serialComma: true,
		yearSep: ", ", ndSuffixSep: "-",
		punctInQuotes: true, pageRange: "expanded", pageDash: "–",
		enTerms: map[string]string{
			"editor": "Ed.", "editors": "Eds.", "translator": "Trans.", "translators": "Trans.",
			"volume": "Vol.", "volumes": "Vols.", "chapter": "Chapter", "section": "Section",
			"figure": "Figure", "table": "Table", "line": "line", "lines": "lines",
			"paragraph": "para.", "note": "n.",
		},
		entry: apaEntry,
	},
	"chicago": {
		info: StyleInfo{
			Name: "chicago", Title: "Chicago 17th edition (author-date)", Kind: "author-date",
			Description: "Chicago Manual of Style, 17th edition, author-date system: (Smith 2020, 33) citations and an alphabetical reference list.",
		},
		citeEtAlMin: 4, serialComma: true,
		yearSep: " ", ndSuffixSep: "-", barePages: true,
		punctInQuotes: true, pageRange: "chicago", pageDash: "–",
		enTerms: map[string]string{
			"thesis-phd": "PhD diss.", "chapter": "chap.",
		},
		entry: chicagoEntry,
	},
	"harvard": {
		info: StyleInfo{
			Name: "harvard", Title: "Harvard (author-date)", Kind: "author-date",
			Description: "Harvard author-date referencing in its common British form: (Smith, 2020, p. 33) citations, single-quoted article titles and an alphabetical reference list.",
		},
		citeEtAlMin: 4,
		yearSep:     ", ", ndSuffixSep: " ",
		singleQuotes: true, dmy: true, pageRange: "expanded", pageDash: "–",
		enTerms: map[string]string{
			"no-date": "no date", "editor": "ed.", "editors": "eds", "edition": "edn",
			"thesis-phd": "PhD thesis", "dataset": "Dataset", "software": "Computer program",
			"in": "in",
		},
		entry: harvardEntry,
	},
	"ieee": {
		info: StyleInfo{
			Name: "ieee", Title: "IEEE", Kind: "numeric",
			Description: "IEEE reference style: bracketed numbers [1] in order of first citation, ranges as [1]–[3].",
		},
		numeric: true, citeEtAlMin: 3,
		punctInQuotes: true, pageRange: "expanded", pageDash: "–",
		enMonthsShort: &ieeeMonths,
		enTerms: map[string]string{
			"editor": "Ed.", "editors": "Eds.", "chapter": "ch.", "section": "Sec.", "figure": "Fig.",
			"table": "Table", "thesis-phd": "Ph.D. dissertation", "thesis-master": "M.S. thesis",
			"thesis-bachelor": "B.S. thesis", "report": "Rep.", "available-at": "Available",
			"in": "in", "version": "ver.",
		},
		entry: ieeeEntry,
	},
	"vancouver": {
		info: StyleInfo{
			Name: "vancouver", Title: "Vancouver (NLM/ICMJE)", Kind: "numeric",
			Description: "Vancouver style as used in medicine and the sciences (NLM Citing Medicine): numbers [1–3] in order of first citation.",
		},
		numeric: true, citeEtAlMin: 3, vancouverCN: true,
		pageRange: "minimal", pageDash: "-",
		enMonthsShort: &vancouverMonths,
		enTerms: map[string]string{
			"editor": "editor", "editors": "editors", "in": "In:", "pages": "p.",
			"thesis-phd": "dissertation", "thesis-master": "master's thesis",
			"thesis-bachelor": "bachelor's thesis", "dataset": "dataset", "software": "computer program",
			"report-no": "Report No.:", "manuscript": "unpublished manuscript",
		},
		entry: vancouverEntry,
	},
	"mla": {
		info: StyleInfo{
			Name: "mla", Title: "MLA 9th edition", Kind: "author-page",
			Description: "Modern Language Association, 9th edition: (Smith 33) citations and an alphabetical works-cited list.",
		},
		authorPage: true, citeEtAlMin: 3,
		barePages: true, punctInQuotes: true, dmy: true, pageRange: "minimal-two", pageDash: "–",
		enTerms: map[string]string{
			"editor": "editor", "editors": "editors", "translator": "translator", "translators": "translators",
			"chapter": "ch.", "section": "sec.", "line": "line", "lines": "lines",
			"thesis-phd": "PhD dissertation",
		},
		entry: mlaEntry,
	},
}

// Styles lists the built-in citation styles.
func Styles() []StyleInfo {
	out := make([]StyleInfo, 0, len(styleOrder))
	for _, n := range styleOrder {
		out = append(out, styles[n].info)
	}
	return out
}

var styleAliases = map[string]string{
	"apa": "apa", "apa7": "apa", "apa-7": "apa", "apa7th": "apa", "apa-7th": "apa",
	"apa-7th-edition": "apa", "american-psychological-association": "apa",
	"american-psychological-association-7th-edition": "apa", "apa6": "apa", "psychology": "apa",

	"chicago": "chicago", "chicago-author-date": "chicago", "chicago-17": "chicago",
	"chicago17": "chicago", "chicago-17th": "chicago", "chicago-author-date-17th-edition": "chicago",
	"chicago-manual-of-style": "chicago", "chicago-manual-of-style-17th-edition": "chicago",
	"cmos": "chicago", "cms": "chicago", "turabian": "chicago", "turabian-author-date": "chicago",
	"authoryear": "chicago", "author-year": "chicago",

	"harvard": "harvard", "harvard-cite-them-right": "harvard", "cite-them-right": "harvard",
	"harvard1": "harvard", "harvard-ctr": "harvard",

	"ieee": "ieee", "ieee-transactions": "ieee", "ieeetr": "ieee", "numeric": "ieee",
	"ieee-with-url": "ieee",

	"vancouver": "vancouver", "nlm": "vancouver", "icmje": "vancouver", "citing-medicine": "vancouver",
	"vancouver-brackets": "vancouver", "nature": "vancouver", "ama": "vancouver",
	"american-medical-association": "vancouver",

	"mla": "mla", "mla9": "mla", "mla-9": "mla", "mla9th": "mla", "mla-9th": "mla", "mla8": "mla",
	"mla-8": "mla", "modern-language-association": "mla",
	"modern-language-association-9th-edition": "mla",
}

// NormalizeStyle maps a style name or common alias ("APA7", "chicago-author-date",
// "harvard-cite-them-right", "mla9", "nature", "ieee.csl") to the name of a
// built-in style. Journal-specific numeric styles map to "vancouver",
// generic author-year to "chicago". The boolean is false when the name is
// not recognised.
func NormalizeStyle(name string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(name))
	if s == "" {
		return "", false
	}
	s = strings.TrimSuffix(s, ".csl")
	s = strings.NewReplacer(" ", "-", "_", "-").Replace(s)
	if n, ok := styleAliases[s]; ok {
		return n, true
	}
	switch {
	case strings.HasPrefix(s, "harvard"):
		return "harvard", true
	case strings.HasPrefix(s, "apa"):
		return "apa", true
	case strings.HasPrefix(s, "chicago"):
		return "chicago", true
	case strings.HasPrefix(s, "ieee"):
		return "ieee", true
	case strings.HasPrefix(s, "vancouver"):
		return "vancouver", true
	case strings.HasPrefix(s, "mla") || strings.HasPrefix(s, "modern-language"):
		return "mla", true
	}
	return "", false
}
