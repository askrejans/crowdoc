package ast

// Meta is document metadata. Readers fill what their source provides
// (frontmatter, DOCX core properties, HTML <meta>, notebook metadata);
// callers may override any field.
type Meta struct {
	Title      string
	Subtitle   string
	ShortTitle string // running-head title
	Authors    []Author
	// Organization is the issuing company or institution (cover pages,
	// letterheads). Legacy "author" values that name a company stay in
	// Authors; templates fall back to the first author when this is empty.
	Organization string
	Date         string
	Version      string
	Status       string // e.g. DRAFT, FINAL
	// Classification is a handling label such as CONFIDENTIAL or PUBLIC.
	Classification string
	DocType        string // agreement, report, paper, letter, invoice, memo, ...
	Style          string // requested style name
	Lang           string // BCP 47 language tag, e.g. "en", "lv", "de-AT"
	Subject        string
	Keywords       []string
	// Summary is a short plain-text description (cover pages, PDF subject).
	Summary string
	// Abstract is a rich abstract (academic styles). Readers may fill it
	// from an "Abstract" section or frontmatter.
	Abstract []Block

	// Layout
	TOC            *bool // nil = style default
	LOF            bool  // list of figures
	LOT            bool  // list of tables
	NumberSections *bool // nil = style default
	NoTitlePage    bool
	// ShowTitle forces the title block on or off; nil prints real titles
	// and hides a title derived from the file name unless a title page is
	// printed.
	ShowTitle *bool
	// TitleFromName is set when Title was derived from the file name
	// because neither metadata nor content named the document. Such a
	// title still names the PDF but is not printed by default.
	TitleFromName bool
	Signatures    *bool // nil = style default
	FontSize      int   // 9..14 pt; 0 = style default
	Paper         string
	Landscape     bool
	Columns       int    // 1 or 2; 0 = style default
	LineSpacing   string // "single", "onehalf", "double" or a factor
	Margins       Margins
	HeaderLeft    string
	HeaderRight   string
	FooterLeft    string
	FooterRight   string
	Logo          string // path or res: name
	LinksAsNotes  *bool  // print link targets as footnotes
	PDFA          bool   // produce an archival PDF/A document
	// Fonts overrides the style's font choices (family names).
	Fonts Fonts
	// Accent overrides the style's accent colour ("#6A00FF").
	Accent string
	// ColorScheme names a built-in colour palette ("oxford", "emerald", …).
	ColorScheme string
	// Colors overrides individual colours by role (accent, secondary, ink,
	// heading, muted, link, rule, code-bg, table-head, stripe, quote, page).
	Colors map[string]string

	// Citations
	Bibliography          []string // .bib, .json (CSL), .yaml (CSL), .ris files
	CitationStyle         string   // apa, ieee, chicago, harvard, vancouver, mla
	ReferenceSectionTitle string   // overrides the localised "References"
	NoCite                []string // keys to list without citing ("*" = all)

	// Letters and memos
	Recipient []string // address lines
	Sender    []string // address lines
	To        string
	From      string
	CC        string
	Opening   string // "Dear Ms. Smith,"
	Closing   string // "Sincerely,"
	Place     string // place of signing / sending

	// Theses and title pages
	Institution string
	Faculty     string
	Department  string
	Degree      string
	Supervisor  string
	Location    string

	// Jurisdiction metadata shown by invoice/report styles.
	Jurisdiction      string
	SellerCountryCode string
	BuyerCountryCode  string
	VATBreakdown      string
	LegalNotice       string

	// Parties to an agreement (legal styles).
	Parties []string

	// Extra holds every frontmatter key not mapped above, for custom
	// templates (`.Meta.Extra.client`).
	Extra map[string]any
}

// Fonts names font families; empty fields keep the style default.
type Fonts struct {
	// Pairing selects a named typeface pairing ("classic", "modern", …).
	Pairing                string
	Main, Sans, Mono, Math string
}

// Margins are page margins as TeX lengths ("2.5cm"); empty = style default.
type Margins struct {
	Top, Bottom, Left, Right string
}

// Author is a document author.
type Author struct {
	Name          string
	Affiliations  []string
	Email         string
	ORCID         string
	Corresponding bool
}

// AuthorNames returns the display names of all authors.
func (m *Meta) AuthorNames() []string {
	out := make([]string, 0, len(m.Authors))
	for _, a := range m.Authors {
		if a.Name != "" {
			out = append(out, a.Name)
		}
	}
	return out
}

// Reference is a bibliography entry, modelled on CSL-JSON.
type Reference struct {
	ID   string
	Type string // CSL type: article-journal, book, chapter, paper-conference, thesis, report, webpage, ...

	Title           string
	ShortTitle      string
	ContainerTitle  string // journal, book or proceedings title
	CollectionTitle string // series
	Publisher       string
	PublisherPlace  string
	Edition         string
	Volume          string
	Issue           string
	Page            string
	Number          string // report/patent/standard number
	Genre           string // e.g. "PhD thesis"
	Event           string
	EventPlace      string
	Medium          string
	Version         string
	Language        string
	URL             string
	DOI             string
	ISBN            string
	ISSN            string
	Note            string
	Abstract        string

	Author     []Name
	Editor     []Name
	Translator []Name

	Issued   Date
	Accessed Date
}

// Name is a personal or institutional name.
type Name struct {
	Family   string
	Given    string
	Particle string // "van", "de"
	Suffix   string // "Jr."
	// Literal is used for institutions and names that must not be split.
	Literal string
}

// Date is a (possibly partial) calendar date.
type Date struct {
	Year, Month, Day int
	// Literal holds dates that could not be parsed ("forthcoming", "n.d.").
	Literal string
	// Circa marks approximate dates.
	Circa bool
}

// IsZero reports whether no date information is present.
func (d Date) IsZero() bool { return d.Year == 0 && d.Literal == "" }
