package fonts

import "strings"

// Pairing is a named set of typefaces that work together. Each role holds a
// fallback chain: the intended family first, close substitutes next, and a
// family Typst embeds last, so a pairing renders even with nothing
// installed. Heading is empty when headings use the main face.
type Pairing struct {
	Name        string
	Title       string
	Description string
	Category    string // book, academic, business, modern, accessible, multilingual
	Uses        []string
	Main        []string
	Sans        []string
	Mono        []string
	Math        []string
	Heading     []string
}

// Families Typst ships inside its binary; always available to it.
const (
	embeddedSerif = "Libertinus Serif"
	embeddedCM    = "New Computer Modern"
	embeddedMath  = "New Computer Modern Math"
	embeddedMono  = "DejaVu Sans Mono"
)

var embeddedFamilies = []string{embeddedSerif, embeddedCM, embeddedMath, embeddedMono}

// systemSans are sans families commonly preinstalled on macOS, Windows and
// Linux. Typst embeds no sans, so these bridge the gap before the final
// serif fallback when the caller scans system fonts too.
var systemSans = []string{"Helvetica Neue", "Helvetica", "Arial", "DejaVu Sans", "Liberation Sans", "Noto Sans"}

func serif(primary ...string) []string { return chain(primary, []string{embeddedSerif}) }
func sans(primary ...string) []string  { return chain(primary, systemSans, []string{embeddedCM}) }
func mono(primary ...string) []string  { return chain(primary, []string{embeddedMono}) }
func math(primary ...string) []string  { return chain(primary, []string{embeddedMath}) }

// sansMain is a sans body face; like every main chain it ends in the
// embedded serif.
func sansMain(primary ...string) []string {
	return chain(primary, systemSans, []string{embeddedSerif})
}

// chain concatenates the parts; a repeated family keeps its first position.
func chain(parts ...[]string) []string {
	var out []string
	for _, part := range parts {
		for _, n := range part {
			if !containsFold(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
}

// pairings are ordered roughly by how widely they suit documents. Serif
// faces with no Greek list Noto Serif before the embedded fallback: its
// proportions match contemporary text faces better than Libertinus does.
var pairings = []Pairing{
	{
		Name: "classic", Title: "Classic book",
		Description: "EB Garamond, a Renaissance old-style roman with true small caps and old-style figures, set with a quiet humanist sans and Garamond-Math.",
		Category:    "book", Uses: []string{"books", "novels", "essays", "humanities theses"},
		Main: serif("EB Garamond"), Sans: sans("Source Sans 3", "Source Sans Pro"),
		Mono: mono("Source Code Pro"), Math: math("Garamond-Math", "Libertinus Math"),
	},
	{
		Name: "elegant", Title: "Elegant",
		Description: "Light, crisp Spectral for text under refined Cormorant Garamond display headings.",
		Category:    "book", Uses: []string{"literary books", "poetry", "invitations", "programmes"},
		Heading: serif("Cormorant Garamond", "EB Garamond", "Spectral"),
		Main:    serif("Spectral"), Sans: sans("Lato"),
		Mono: mono("IBM Plex Mono"), Math: math("Libertinus Math", "STIX Two Math"),
	},
	{
		Name: "libertine", Title: "Libertinus",
		Description: "The Libertinus superfamily: a warm old-style serif with matching sans, typewriter mono and math. The serif is embedded, so this works offline.",
		Category:    "academic", Uses: []string{"papers", "theses", "lecture notes", "reports"},
		Main: serif(embeddedSerif), Sans: sans("Libertinus Sans"),
		Mono: mono("Libertinus Mono"), Math: math("Libertinus Math"),
	},
	{
		Name: "computer-modern", Title: "Computer Modern",
		Description: "The classic look of mathematical typesetting: New Computer Modern for text and math (embedded), with its sans and mono when TeX Live is installed.",
		Category:    "academic", Uses: []string{"mathematics", "physics", "computer science", "preprints"},
		Main: serif(embeddedCM), Sans: sans("New Computer Modern Sans", "Latin Modern Sans"),
		Mono: mono("New Computer Modern Mono", "Latin Modern Mono"), Math: math(embeddedMath),
	},
	{
		Name: "scientific", Title: "Scientific",
		Description: "STIX Two Text and STIX Two Math, the Times-like faces of scientific publishing with the widest symbol coverage, with Inter and JetBrains Mono.",
		Category:    "academic", Uses: []string{"journal articles", "engineering", "chemistry", "formula-heavy documents"},
		Main: serif("STIX Two Text"), Sans: sans("Inter"),
		Mono: mono("JetBrains Mono"), Math: math("STIX Two Math"),
	},
	{
		Name: "source", Title: "Source",
		Description: "The Source superfamily: a Fournier-inspired text serif, a humanist sans and a clear code face designed together.",
		Category:    "business", Uses: []string{"reports", "documentation", "proposals", "manuals"},
		Main: serif("Source Serif 4", "Source Serif Pro"), Sans: sans("Source Sans 3", "Source Sans Pro"),
		Mono: mono("Source Code Pro"), Math: math("STIX Two Math", "Libertinus Math"),
	},
	{
		Name: "plex", Title: "Plex",
		Description: "IBM Plex serif, sans, mono and math: engineered, neutral and corporate in the good sense.",
		Category:    "business", Uses: []string{"technical reports", "specifications", "white papers", "slides"},
		Main: serif("IBM Plex Serif"), Sans: sans("IBM Plex Sans"),
		Mono: mono("IBM Plex Mono"), Math: math("IBM Plex Math", "STIX Two Math"),
	},
	{
		Name: "modern", Title: "Modern sans",
		Description: "Inter throughout, a neo-grotesque tuned for legibility, with JetBrains Mono for code and a sans-serif math face.",
		Category:    "modern", Uses: []string{"product documentation", "memos", "handbooks", "screen PDFs"},
		Main: sansMain("Inter"), Sans: sans("Inter"),
		Mono: mono("JetBrains Mono"), Math: math("Fira Math", "Noto Sans Math"),
	},
	{
		Name: "fira", Title: "Fira",
		Description: "Fira Sans for text with Fira Mono and Fira Math: a humanist sans family that reads well in print and on screen.",
		Category:    "modern", Uses: []string{"technical notes", "slides", "teaching material", "reports"},
		Main: sansMain("Fira Sans"), Sans: sans("Fira Sans"),
		Mono: mono("Fira Mono", "Fira Code"), Math: math("Fira Math"),
	},
	{
		Name: "charter", Title: "Charter",
		Description: "Charis SIL, a Charter descendant: sturdy, economical and very legible even at small sizes or on modest printers.",
		Category:    "business", Uses: []string{"letters", "contracts", "reports", "linguistics"},
		Main: serif("Charis SIL", "XCharter", "Noto Serif"), Sans: sans("Fira Sans"),
		Mono: mono("Fira Mono"), Math: math("XCharter Math", "STIX Two Math"),
	},
	{
		Name: "crimson", Title: "Crimson",
		Description: "Crimson Pro, a contemporary old-style text face in the Garamond tradition, with Lato for supporting text.",
		Category:    "book", Uses: []string{"books", "essays", "dissertations", "long reports"},
		Main: serif("Crimson Pro"), Sans: sans("Lato"),
		Mono: mono("Source Code Pro"), Math: math("Libertinus Math", "Garamond-Math"),
	},
	{
		Name: "literata", Title: "Literata",
		Description: "Literata, a text face designed for long reading with optical sizes, paired with Open Sans.",
		Category:    "book", Uses: []string{"e-books", "long reads", "guides", "course readers"},
		Main: serif("Literata"), Sans: sans("Open Sans"),
		Mono: mono("Roboto Mono"), Math: math("STIX Two Math"),
	},
	{
		Name: "editorial", Title: "Editorial",
		Description: "High-contrast Playfair Display headings over Source Serif body text, the look of a quality magazine.",
		Category:    "business", Uses: []string{"newsletters", "annual reports", "articles", "brochures"},
		Heading: serif("Playfair Display", "Source Serif 4", "Source Serif Pro"),
		Main:    serif("Source Serif 4", "Source Serif Pro"), Sans: sans("Source Sans 3", "Source Sans Pro"),
		Mono: mono("Source Code Pro"), Math: math("STIX Two Math"),
	},
	{
		Name: "baskerville", Title: "Baskerville",
		Description: "Libre Baskerville, a transitional roman, with Libre Franklin as its classic gothic companion.",
		Category:    "book", Uses: []string{"books", "certificates", "formal letters", "programmes"},
		Main: serif("Libre Baskerville", "Noto Serif"), Sans: sans("Libre Franklin"),
		Mono: mono("IBM Plex Mono"), Math: math("STIX Two Math"),
	},
	{
		Name: "humanist", Title: "Humanist",
		Description: "Alegreya and Alegreya Sans, calligraphic and lively, designed together for literature.",
		Category:    "book", Uses: []string{"literature", "poetry", "history", "essays"},
		Main: serif("Alegreya"), Sans: sans("Alegreya Sans"),
		Mono: mono("Fira Mono"), Math: math("Libertinus Math"),
	},
	{
		Name: "noto", Title: "Noto",
		Description: "Noto Serif, Sans and Mono for the broadest language coverage; Arabic, Hebrew and CJK Noto faces join automatically when installed.",
		Category:    "multilingual", Uses: []string{"multilingual documents", "translations", "international reports"},
		Main: serif("Noto Serif"), Sans: sans("Noto Sans"),
		Mono: mono("Noto Sans Mono"), Math: math("STIX Two Math", "Noto Sans Math"),
	},
	{
		Name: "accessible", Title: "Accessible",
		Description: "Atkinson Hyperlegible Next and Mono, designed for low-vision readers: every letter and figure is unmistakable.",
		Category:    "accessible", Uses: []string{"large-print documents", "public information", "forms", "education"},
		Main: sansMain("Atkinson Hyperlegible Next"), Sans: sans("Atkinson Hyperlegible Next"),
		Mono: mono("Atkinson Hyperlegible Mono"), Math: math("Fira Math", "Noto Sans Math"),
	},
	{
		Name: "geometric", Title: "Geometric",
		Description: "Bold Montserrat headings with friendly Figtree text: clean, contemporary and confident.",
		Category:    "modern", Uses: []string{"proposals", "pitch documents", "brochures", "invoices"},
		Heading: sans("Montserrat", "Outfit", "Figtree"),
		Main:    sansMain("Figtree"), Sans: sans("Figtree"),
		Mono: mono("Roboto Mono"), Math: math("Noto Sans Math", "Fira Math"),
	},
	{
		Name: "newsreader", Title: "Newsreader",
		Description: "Newsreader, an optical-size text face made for news, with Work Sans for captions and tables.",
		Category:    "book", Uses: []string{"articles", "newsletters", "journalism", "reports"},
		Main: serif("Newsreader 16pt", "Noto Serif"), Sans: sans("Work Sans"),
		Mono: mono("Roboto Mono"), Math: math("STIX Two Math"),
	},
	{
		Name: "pt", Title: "PT",
		Description: "PT Serif, Sans and Mono, a superfamily drawn for Latin and Cyrillic alike.",
		Category:    "multilingual", Uses: []string{"Cyrillic documents", "official papers", "bilingual texts"},
		Main: serif("PT Serif", "Noto Serif"), Sans: sans("PT Sans"),
		Mono: mono("PT Mono"), Math: math("STIX Two Math"),
	},
	{
		Name: "tech", Title: "Tech",
		Description: "Space Grotesk headings over DM Sans text with DM Mono: precise, modern and technical.",
		Category:    "modern", Uses: []string{"engineering docs", "changelogs", "API guides", "data sheets"},
		Heading: sans("Space Grotesk", "DM Sans 9pt"),
		Main:    sansMain("DM Sans 9pt"), Sans: sans("DM Sans 9pt"),
		Mono: mono("DM Mono", "JetBrains Mono"), Math: math("Fira Math", "Noto Sans Math"),
	},
	{
		Name: "warm", Title: "Warm",
		Description: "Soft, characterful Fraunces headings over Lora, a well-balanced contemporary serif.",
		Category:    "book", Uses: []string{"cookbooks", "memoirs", "newsletters", "invitations"},
		Heading: serif("Fraunces", "Lora"),
		Main:    serif("Lora", "Noto Serif"), Sans: sans("Figtree"),
		Mono: mono("Fira Code", "Fira Mono"), Math: math("Libertinus Math"),
	},
	{
		Name: "magazine", Title: "Magazine",
		Description: "Striking DM Serif Display headings over sturdy Merriweather text, with DM Sans and DM Mono.",
		Category:    "business", Uses: []string{"magazines", "brochures", "case studies", "posters"},
		Heading: serif("DM Serif Display", "Merriweather"),
		Main:    serif("Merriweather", "Noto Serif"), Sans: sans("DM Sans 9pt"),
		Mono: mono("DM Mono"), Math: math("STIX Two Math"),
	},
}

// pairingAliases lists alternative names per pairing, already normalised.
var pairingAliases = map[string][]string{
	"classic":         {"garamond", "ebgaramond", "book"},
	"elegant":         {"cormorant", "spectral"},
	"libertine":       {"libertinus", "linuxlibertine"},
	"computer-modern": {"cm", "latex", "tex", "newcomputermodern"},
	"scientific":      {"stix", "stixtwo", "science"},
	"source":          {"sourceserif", "sourcesans"},
	"plex":            {"ibmplex"},
	"modern":          {"inter", "sans"},
	"fira":            {"firasans"},
	"charter":         {"charis", "charissil", "xcharter"},
	"crimson":         {"crimsonpro"},
	"editorial":       {"playfair"},
	"baskerville":     {"librebaskerville"},
	"humanist":        {"alegreya"},
	"noto":            {"multilingual"},
	"accessible":      {"atkinson", "hyperlegible", "legible"},
	"geometric":       {"montserrat", "figtree"},
	"newsreader":      {"news"},
	"pt":              {"ptserif"},
	"tech":            {"spacegrotesk", "dmsans"},
	"warm":            {"fraunces", "lora"},
	"magazine":        {"dmserif", "merriweather"},
}

// Pairings returns all pairings. The result is a copy.
func Pairings() []Pairing {
	out := make([]Pairing, len(pairings))
	for i, p := range pairings {
		out[i] = p.clone()
	}
	return out
}

// LookupPairing finds a pairing by name or alias, ignoring case, spaces,
// hyphens and underscores ("Computer Modern", "computer_modern", "cm").
func LookupPairing(name string) (Pairing, bool) {
	key := normalizeName(name)
	for _, p := range pairings {
		if normalizeName(p.Name) == key || contains(pairingAliases[p.Name], key) {
			return p.clone(), true
		}
	}
	return Pairing{}, false
}

// Chain returns the fallback chain for a role. Heading falls back to Main
// when the pairing has no separate heading face.
func (p Pairing) Chain(r Role) []string {
	var c []string
	switch r {
	case RoleMain:
		c = p.Main
	case RoleSans:
		c = p.Sans
	case RoleMono:
		c = p.Mono
	case RoleMath:
		c = p.Math
	case RoleHeading:
		c = p.Heading
		if len(c) == 0 {
			c = p.Main
		}
	}
	return append([]string(nil), c...)
}

// Fonts is the font list to give the engine for a role: the resolved chain
// plus installed script-coverage families. The document language's own
// script (Arabic, Hebrew, Chinese, Japanese, Korean) is placed before the
// embedded fallback so it wins over it; other scripts follow at the end so
// quoted foreign text still renders. lang is a BCP 47 tag; empty is fine.
func (p Pairing) Fonts(r Role, lang string, available map[string]bool) []string {
	c := p.Chain(r)
	resolved := Resolve(c, available)
	if r == RoleMath || len(c) == 0 {
		return resolved
	}
	useSerif := (r == RoleMain || r == RoleHeading) && isSerifFamily(c[0])
	own, rest := scriptFallbacks(lang, useSerif)
	avail := foldSet(available)
	installed := func(names []string) []string {
		var out []string
		for _, n := range names {
			if avail[strings.ToLower(n)] && !containsFold(resolved, n) && !containsFold(out, n) {
				out = append(out, n)
			}
		}
		return out
	}
	own = installed(own)
	rest = installed(rest)

	at := len(resolved)
	for i, n := range resolved {
		if isEmbedded(n) {
			at = i
			break
		}
	}
	out := append([]string{}, resolved[:at]...)
	out = append(out, own...)
	out = append(out, resolved[at:]...)
	for _, n := range rest {
		if !containsFold(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// Resolve filters a fallback chain to the families present in available
// (matched case-insensitively, as Typst does). Families Typst embeds and the
// chain's final fallback are always kept; duplicates are dropped.
func Resolve(chain []string, available map[string]bool) []string {
	avail := foldSet(available)
	var out []string
	for i, n := range chain {
		if containsFold(out, n) {
			continue
		}
		if avail[strings.ToLower(n)] || isEmbedded(n) || i == len(chain)-1 {
			out = append(out, n)
		}
	}
	return out
}

// scriptFonts are the coverage families per script, in fallback order.
var scriptFonts = []struct{ script, serif, sans string }{
	{"arabic", "Noto Naskh Arabic", "Noto Sans Arabic"},
	{"hebrew", "Noto Serif Hebrew", "Noto Sans Hebrew"},
	{"zh-hans", "Noto Serif SC", "Noto Sans SC"},
	{"zh-hant", "Noto Serif TC", "Noto Sans TC"},
	{"ja", "Noto Serif JP", "Noto Sans JP"},
	{"ko", "Noto Serif KR", "Noto Sans KR"},
}

const emojiFamily = "Noto Color Emoji"

// scriptFallbacks splits the coverage families into the ones for lang's
// own script and all others (emoji last).
func scriptFallbacks(lang string, useSerif bool) (own, rest []string) {
	script := langScript(lang)
	for _, s := range scriptFonts {
		name := s.sans
		if useSerif {
			name = s.serif
		}
		if s.script == script {
			own = append(own, name)
		} else {
			rest = append(rest, name)
		}
	}
	return own, append(rest, emojiFamily)
}

// langScript maps a BCP 47 tag to the script needing a dedicated font. Han
// glyph shapes differ between Chinese, Japanese and Korean conventions, so
// the CJK choice follows the language and, for Chinese, its script/region.
func langScript(lang string) string {
	parts := strings.FieldsFunc(strings.ToLower(lang), func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case "ar", "fa", "ur", "ps", "sd", "ug", "ckb":
		return "arabic"
	case "he", "iw", "yi":
		return "hebrew"
	case "ja":
		return "ja"
	case "ko":
		return "ko"
	case "zh":
		for _, p := range parts[1:] {
			switch p {
			case "hant", "tw", "hk", "mo":
				return "zh-hant"
			}
		}
		return "zh-hans"
	}
	return ""
}

func isSerifFamily(name string) bool {
	if f, ok := lookupFamily(name); ok {
		return f.Category == "serif" || f.Category == "display"
	}
	return strings.EqualFold(name, embeddedSerif) || strings.EqualFold(name, embeddedCM)
}

func isEmbedded(name string) bool { return containsFold(embeddedFamilies, name) }

func (p Pairing) clone() Pairing {
	cp := func(s []string) []string { return append([]string(nil), s...) }
	p.Uses, p.Main, p.Sans, p.Mono, p.Math, p.Heading = cp(p.Uses), cp(p.Main), cp(p.Sans), cp(p.Mono), cp(p.Math), cp(p.Heading)
	return p
}

func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func foldSet(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		if v {
			out[strings.ToLower(k)] = true
		}
	}
	return out
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
