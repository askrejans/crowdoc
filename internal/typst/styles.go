package typst

import (
	"sort"
	"strings"
)

// Style describes a built-in document style.
type Style struct {
	Name        string
	Title       string
	Description string
	// Category groups styles: academic, business, legal, technical,
	// correspondence, personal, publishing, data.
	Category string
	Uses     []string
	File     string
	// Pairing is the default typeface pairing.
	Pairing string
	// FontSize is the default body size in points.
	FontSize float64
	// TOC is "auto", "always" or "never".
	TOC            string
	NumberSections bool
	TitlePage      bool
	// Signatures is "auto", "always" or "never".
	Signatures string
	// Abstract makes a leading "Abstract" section the document abstract.
	Abstract bool
	// CitationStyle is the default citation style.
	CitationStyle string
	LinksAsNotes  bool
	Columns       int
	Aliases       []string
}

var registry = map[string]*Style{}

func register(s *Style) {
	registry[s.Name] = s
	for _, a := range s.Aliases {
		registry[a] = s
	}
}

// LookupStyle finds a style by name or alias (case-insensitive).
func LookupStyle(name string) (*Style, bool) {
	s, ok := registry[strings.ToLower(strings.TrimSpace(name))]
	return s, ok
}

// Styles returns all styles sorted by category then name.
func Styles() []*Style {
	seen := map[*Style]bool{}
	var out []*Style
	for _, s := range registry {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// StyleSource returns the Typst source of a style (for exporting or
// customising it).
func StyleSource(s *Style) (string, error) {
	data, err := files.ReadFile("styles/" + s.File)
	return string(data), err
}

// LibrarySource returns the shared Typst library.
func LibrarySource() string {
	data, _ := files.ReadFile("assets/crowdoc.typ")
	return string(data)
}

func init() {
	for _, st := range builtin {
		register(st)
	}
}

// builtin lists the built-in styles. FontSize is the default body size;
// Pairing names a typeface pairing from the font catalogue.
var builtin = []*Style{
	// Academic and publishing
	{Name: "article", Title: "Journal article", Category: "academic", File: "article.typ", Pairing: "libertine", FontSize: 10.5,
		Description: "Classic single-column journal article: centred title block, affiliations, abstract and keywords.",
		Uses:        []string{"journal articles", "research papers", "term papers"},
		TOC:         "never", NumberSections: true, Abstract: true, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"academic", "journal"}},
	{Name: "paper", Title: "Conference paper", Category: "academic", File: "paper.typ", Pairing: "scientific", FontSize: 10,
		Description: "Two-column proceedings paper with author columns, Abstract— lead-in and roman section numbers.",
		Uses:        []string{"conference papers", "proceedings", "engineering papers"},
		TOC:         "never", NumberSections: true, Abstract: true, Signatures: "never", CitationStyle: "ieee", Columns: 2,
		Aliases: []string{"conference", "ieee", "twocolumn", "two-column", "proceedings"}},
	{Name: "apa", Title: "APA manuscript", Category: "academic", File: "apa.typ", Pairing: "scientific", FontSize: 12,
		Description: "APA 7th edition paper: title page, double spacing, APA heading levels and hanging-indent references.",
		Uses:        []string{"psychology and social-science papers", "student papers", "theses chapters"},
		TOC:         "never", NumberSections: false, Abstract: true, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"apa7"}},
	{Name: "essay", Title: "Essay (MLA)", Category: "academic", File: "essay.typ", Pairing: "scientific", FontSize: 12,
		Description: "Humanities essay in the MLA manner: heading block, centred title, double spacing, surname and page header.",
		Uses:        []string{"essays", "literature papers", "coursework"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "mla",
		Aliases: []string{"mla", "coursework"}},
	{Name: "thesis", Title: "Thesis", Category: "academic", File: "thesis.typ", Pairing: "classic", FontSize: 12,
		Description: "Bachelor's, master's or doctoral thesis: institutional title page, roman front matter, chapters, lists of figures and tables.",
		Uses:        []string{"theses", "dissertations", "long academic reports"},
		TOC:         "always", NumberSections: true, Abstract: true, Signatures: "never", CitationStyle: "apa", TitlePage: true,
		Aliases: []string{"dissertation", "bachelor", "master", "phd"}},
	{Name: "preprint", Title: "Preprint", Category: "academic", File: "preprint.typ", Pairing: "computer-modern", FontSize: 10.5,
		Description: "The familiar e-print look: Computer Modern, narrow abstract, preprint marker.",
		Uses:        []string{"preprints", "working papers", "technical reports"},
		TOC:         "never", NumberSections: true, Abstract: true, Signatures: "never", CitationStyle: "chicago",
		Aliases: []string{"arxiv", "working-paper"}},
	{Name: "manuscript", Title: "Review manuscript", Category: "academic", File: "manuscript.typ", Pairing: "scientific", FontSize: 12,
		Description: "Submission manuscript for peer review: double spacing, continuous line numbers, ragged right.",
		Uses:        []string{"journal submissions", "peer review", "drafts for comments"},
		TOC:         "never", NumberSections: true, Abstract: true, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"submission", "review"}},
	{Name: "notes", Title: "Lecture notes", Category: "academic", File: "notes.typ", Pairing: "fira", FontSize: 10.5,
		Description: "Study and lecture notes: numbered theorem and definition boxes, wide annotation margin.",
		Uses:        []string{"lecture notes", "study guides", "problem sets", "handouts"},
		TOC:         "auto", NumberSections: true, Signatures: "never", CitationStyle: "ieee",
		Aliases: []string{"lecture", "handout", "study"}},
	{Name: "book", Title: "Book", Category: "publishing", File: "book.typ", Pairing: "classic", FontSize: 10.5,
		Description: "Two-sided book on B5: title page and verso, chapter openers, running heads, old-style figures.",
		Uses:        []string{"books", "novels", "memoirs", "long-form non-fiction"},
		TOC:         "always", NumberSections: false, Signatures: "never", CitationStyle: "chicago",
		Aliases: []string{"novel"}},
	{Name: "elegant", Title: "Elegant", Category: "publishing", File: "elegant.typ", Pairing: "classic", FontSize: 11,
		Description: "Classical typography with small-caps headings, ornaments and old-style figures.",
		Uses:        []string{"essays", "stories", "speeches", "poetry", "invitations"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "chicago",
		Aliases: []string{"classic", "literary"}},
	{Name: "newsletter", Title: "Newsletter", Category: "publishing", File: "newsletter.typ", Pairing: "newsreader", FontSize: 9.5,
		Description: "Masthead with issue line and a three-column body.",
		Uses:        []string{"newsletters", "bulletins", "club and school news"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "chicago", Columns: 3,
		Aliases: []string{"bulletin", "magazine"}},
	{Name: "slides", Title: "Slides", Category: "publishing", File: "slides.typ", Pairing: "modern", FontSize: 20,
		Description: "16:9 presentation: title slide, then a slide per heading.",
		Uses:        []string{"presentations", "lectures", "pitch decks"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"presentation", "deck", "slideshow"}},
	// Business
	{Name: "report", Title: "Report", Category: "business", File: "report.typ", Pairing: "source", FontSize: 10.5,
		Description: "Professional report with a cover page, numbered sections and running headers.",
		Uses:        []string{"business reports", "analyses", "annual reports"},
		TOC:         "auto", NumberSections: true, TitlePage: true, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"business"}},
	{Name: "proposal", Title: "Proposal", Category: "business", File: "proposal.typ", Pairing: "geometric", FontSize: 10.5,
		Description: "Client proposal with split colour cover, prepared-for/by panel and executive summary.",
		Uses:        []string{"business proposals", "quotes", "statements of work", "pitches"},
		TOC:         "auto", NumberSections: true, TitlePage: true, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"quotation", "sow", "pitch"}},
	{Name: "whitepaper", Title: "White paper", Category: "business", File: "whitepaper.typ", Pairing: "plex", FontSize: 10.5,
		Description: "Authoritative long-form paper with a bold cover band and accent section markers.",
		Uses:        []string{"white papers", "industry reports", "e-books"},
		TOC:         "auto", NumberSections: true, TitlePage: true, Signatures: "never", CitationStyle: "chicago",
		Aliases: []string{"white-paper"}},
	{Name: "brief", Title: "Brief", Category: "business", File: "brief.typ", Pairing: "modern", FontSize: 9,
		Description: "Dense one-page fact sheet or executive brief in two columns under a title band.",
		Uses:        []string{"fact sheets", "executive briefs", "one-pagers", "product sheets"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"one-pager", "factsheet", "fact-sheet", "datasheet-brief"}},
	{Name: "invoice", Title: "Invoice", Category: "business", File: "invoice.typ", Pairing: "modern", FontSize: 10,
		Description: "Invoices and quotes: issuer header, number/date/status, emphasised item tables and payment box.",
		Uses:        []string{"invoices", "quotes", "receipts", "credit notes"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"bill", "receipt", "rēķins", "rekins"}},
	{Name: "minutes", Title: "Meeting minutes", Category: "business", File: "minutes.typ", Pairing: "plex", FontSize: 10.5,
		Description: "Minutes with a meeting-details block (date, place, chair, attendees) and numbered agenda items.",
		Uses:        []string{"meeting minutes", "board minutes", "protocols"},
		TOC:         "never", NumberSections: true, Signatures: "auto", CitationStyle: "apa",
		Aliases: []string{"protocol", "protokols"}},
	{Name: "policy", Title: "Policy", Category: "business", File: "policy.typ", Pairing: "source", FontSize: 10.5,
		Description: "Controlled policies, procedures and standards with a document-control table and 1.1.1 clause numbering.",
		Uses:        []string{"policies", "procedures", "standards", "guidelines"},
		TOC:         "auto", NumberSections: true, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"procedure", "sop", "standard"}},
	{Name: "data", Title: "Data report", Category: "business", File: "data.typ", Pairing: "plex", FontSize: 9,
		Description: "Spreadsheet and CSV exports: striped tables with repeating headers, tabular figures.",
		Uses:        []string{"spreadsheets", "CSV exports", "data listings", "price lists"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"spreadsheet", "table", "datasheet"}},
	// Technical
	{Name: "technical", Title: "Technical", Category: "technical", File: "technical.typ", Pairing: "modern", FontSize: 10,
		Description: "Specifications and API guides: sans text, framed code, version box and admonitions.",
		Uses:        []string{"specifications", "API documentation", "engineering guides", "READMEs"},
		TOC:         "auto", NumberSections: true, TitlePage: true, Signatures: "never", CitationStyle: "ieee",
		Aliases: []string{"spec", "documentation", "api"}},
	{Name: "manual", Title: "Manual", Category: "technical", File: "manual.typ", Pairing: "modern", FontSize: 10,
		Description: "User manuals and handbooks: chapter openers, coloured thumb tabs and numbered steps.",
		Uses:        []string{"user manuals", "handbooks", "playbooks", "training material"},
		TOC:         "always", NumberSections: true, TitlePage: true, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"handbook", "guide", "playbook"}},
	// Legal
	{Name: "legal", Title: "Legal agreement", Category: "legal", File: "legal.typ", Pairing: "classic", FontSize: 11,
		Description: "Contracts, NDAs and agreements: numbered clauses, optional cover and signature blocks.",
		Uses:        []string{"contracts", "NDAs", "service agreements", "terms"},
		TOC:         "auto", NumberSections: true, Signatures: "auto", CitationStyle: "chicago",
		Aliases: []string{"contract", "agreement", "nda"}},
	{Name: "ligums", Title: "Līgums (Latvian agreement)", Category: "legal", File: "ligums.typ", Pairing: "source", FontSize: 11,
		Description: "Latvian agreements: sober monochrome layout, place and date line, party signature table.",
		Uses:        []string{"līgumi", "vienošanās", "pilnvaras", "akti"},
		TOC:         "never", NumberSections: false, Signatures: "auto", CitationStyle: "apa",
		Aliases: []string{"līgums", "lv-agreement"}},
	// Correspondence
	{Name: "letter", Title: "Business letter", Category: "correspondence", File: "letter.typ", Pairing: "source", FontSize: 10.5,
		Description: "Letterhead, window-envelope address block (DIN 5008 on A4), subject line and signature.",
		Uses:        []string{"business letters", "cover letters", "formal correspondence"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"correspondence", "cover-letter"}},
	{Name: "memo", Title: "Memorandum", Category: "correspondence", File: "memo.typ", Pairing: "modern", FontSize: 10.5,
		Description: "Internal memo with To/From/Date/Subject block and summary box.",
		Uses:        []string{"internal memos", "announcements", "notices"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"memorandum", "notice"}},
	// General and personal
	{Name: "minimal", Title: "Minimal", Category: "general", File: "minimal.typ", Pairing: "literata", FontSize: 11,
		Description: "Quiet, generous typography with no title page. For notes, drafts and general writing.",
		Uses:        []string{"notes", "drafts", "blog posts", "general documents"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "chicago",
		Aliases: []string{"plain", "simple", "default"}},
	{Name: "modern", Title: "Modern", Category: "general", File: "modern.typ", Pairing: "modern", FontSize: 10.5,
		Description: "Contemporary sans-serif layout with bold headlines and a strong accent colour.",
		Uses:        []string{"guides", "articles", "portfolios", "briefings"},
		TOC:         "auto", NumberSections: false, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"contemporary"}},
	{Name: "cv", Title: "CV / résumé", Category: "personal", File: "cv.typ", Pairing: "modern", FontSize: 9.5,
		Description: "Résumé with name banner, contact line and ruled section labels.",
		Uses:        []string{"CVs", "résumés", "bios"},
		TOC:         "never", NumberSections: false, Signatures: "never", CitationStyle: "apa",
		Aliases: []string{"resume", "résumé", "cv-modern"}},
}
