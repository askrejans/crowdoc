// Package ast defines the format-neutral document model shared by every
// crowdoc reader and writer.
//
// Readers (Markdown, HTML, DOCX, ODT, RTF, notebooks, spreadsheets, plain
// text) produce a *Document; the LaTeX writer consumes it. The model is close
// to Pandoc's: a tree of Block and Inline nodes plus document metadata and
// embedded binary resources such as images.
//
// All text in the tree is plain Unicode. Writers are responsible for escaping.
package ast

// Document is a complete parsed document.
type Document struct {
	Meta   Meta
	Blocks []Block

	// Resources holds binary assets embedded in the source (images inside a
	// DOCX, notebook outputs, data: URIs, ...). Image.Src refers to them with
	// the "res:" scheme, e.g. "res:media/image1.png".
	Resources *Resources

	// References are bibliography entries carried by the source itself
	// (Zotero/Mendeley/Word citations, a `references:` frontmatter list).
	// They are merged with any external bibliography files.
	References []Reference
}

// Block is a block-level node.
type Block interface{ isBlock() }

// Inline is an inline (phrasing) node.
type Inline interface{ isInline() }

// Attr carries an identifier, classes and key/value attributes, as written
// with Pandoc-style `{#id .class key=value}` syntax or derived from source
// formats (HTML id/class, DOCX bookmarks/styles).
type Attr struct {
	ID      string
	Classes []string
	KV      map[string]string
}

// HasClass reports whether the attribute set contains class c.
func (a Attr) HasClass(c string) bool {
	for _, x := range a.Classes {
		if x == c {
			return true
		}
	}
	return false
}

// Get returns the value of attribute key, or "".
func (a Attr) Get(key string) string {
	if a.KV == nil {
		return ""
	}
	return a.KV[key]
}

// Align is a horizontal alignment.
type Align int

const (
	AlignDefault Align = iota
	AlignLeft
	AlignCenter
	AlignRight
)

// ---------------------------------------------------------------------------
// Blocks
// ---------------------------------------------------------------------------

// Para is a paragraph.
type Para struct{ Inlines []Inline }

// Plain is inline content without paragraph semantics (tight list items,
// table cells).
type Plain struct{ Inlines []Inline }

// Heading is a section heading. Level 1 is the highest level present in the
// source; writers map levels onto the style's sectioning commands.
type Heading struct {
	Level   int
	Inlines []Inline
	Attr    Attr
	// Unnumbered suppresses automatic numbering for this heading
	// (`{-}` / `{.unnumbered}` in Markdown).
	Unnumbered bool
}

// CodeBlock is preformatted code or text.
type CodeBlock struct {
	Lang    string
	Text    string
	Attr    Attr
	Caption []Inline
}

// MathBlock is display mathematics written in LaTeX notation (without the
// surrounding `$$` or `\[ \]`). Label, when set, numbers the equation and
// makes it a cross-reference target (e.g. "eq:energy").
type MathBlock struct {
	TeX   string
	Label string
}

// RawBlock is source passed through verbatim to a writer of the same Format
// (e.g. "latex"). Writers drop raw blocks of other formats.
type RawBlock struct {
	Format string
	Text   string
}

// BlockQuote is a quotation.
type BlockQuote struct {
	Blocks []Block
}

// NumberStyle is the marker style of an ordered list.
type NumberStyle int

const (
	NumberDecimal NumberStyle = iota
	NumberLowerAlpha
	NumberUpperAlpha
	NumberLowerRoman
	NumberUpperRoman
)

// TaskState marks a list item as a checkbox item.
type TaskState int

const (
	TaskNone TaskState = iota
	TaskOpen
	TaskDone
)

// List is a bullet or ordered list.
type List struct {
	Ordered bool
	Start   int // first number of an ordered list (0 means 1)
	Style   NumberStyle
	Tight   bool
	Items   []ListItem
}

// ListItem is one list entry.
type ListItem struct {
	Blocks []Block
	Task   TaskState
}

// DefinitionList is a list of terms and their definitions.
type DefinitionList struct {
	Items []DefinitionItem
}

// DefinitionItem is one term with one or more definitions.
type DefinitionItem struct {
	Term        []Inline
	Definitions [][]Block
}

// ColSpec describes one table column.
type ColSpec struct {
	Align Align
	// Width is the relative width (0..1) of the column, or 0 for automatic.
	Width float64
}

// Cell is a table cell.
type Cell struct {
	Blocks  []Block
	ColSpan int // 0 or 1 means no spanning
	RowSpan int
	Align   Align // overrides the column alignment when not AlignDefault
}

// Row is a table row.
type Row struct {
	Cells []Cell
}

// Table is a table with optional caption, header and footer rows.
type Table struct {
	Attr    Attr
	Caption []Inline
	Cols    []ColSpec
	Head    []Row
	Body    []Row
	Foot    []Row
}

// Figure is a captioned float holding one image.
type Figure struct {
	Attr    Attr
	Image   *Image
	Caption []Inline
}

// HorizontalRule is a thematic break.
type HorizontalRule struct{}

// PageBreak forces a new page.
type PageBreak struct{}

// Div is a generic container. Writers give special meaning to classes:
//
//	note, tip, info, important, warning, caution, danger, success,
//	example, abstract, theorem, lemma, corollary, proposition,
//	definition, remark, proof, appendix, center, landscape
//
// Title is an optional heading shown by callout boxes.
type Div struct {
	Attr   Attr
	Title  []Inline
	Blocks []Block
}

// LineBlock preserves line breaks (poetry, addresses).
type LineBlock struct {
	Lines [][]Inline
}

// Bibliography marks where the reference list is placed. When a document
// has citations but no Bibliography block, writers append one at the end.
type Bibliography struct{}

// ReferenceList is a pre-formatted reference list found in the source
// itself (a "References" section of numbered or hanging entries) when no
// structured bibliography is available. Labels are optional ("[1]", "1.").
type ReferenceList struct {
	Entries []ReferenceEntry
}

// ReferenceEntry is one entry of a ReferenceList.
type ReferenceEntry struct {
	ID      string // anchor id, e.g. "ref-1"
	Label   string // visible label without brackets, e.g. "1"; empty for author-date lists
	Inlines []Inline
}

func (*Para) isBlock()           {}
func (*Plain) isBlock()          {}
func (*Heading) isBlock()        {}
func (*CodeBlock) isBlock()      {}
func (*MathBlock) isBlock()      {}
func (*RawBlock) isBlock()       {}
func (*BlockQuote) isBlock()     {}
func (*List) isBlock()           {}
func (*DefinitionList) isBlock() {}
func (*Table) isBlock()          {}
func (*Figure) isBlock()         {}
func (*HorizontalRule) isBlock() {}
func (*PageBreak) isBlock()      {}
func (*Div) isBlock()            {}
func (*LineBlock) isBlock()      {}
func (*Bibliography) isBlock()   {}
func (*ReferenceList) isBlock()  {}

// ---------------------------------------------------------------------------
// Inlines
// ---------------------------------------------------------------------------

// Text is a run of plain text.
type Text struct{ Value string }

// Emph is emphasised (italic) text.
type Emph struct{ Inlines []Inline }

// Strong is strongly emphasised (bold) text.
type Strong struct{ Inlines []Inline }

// Strike is struck-out text.
type Strike struct{ Inlines []Inline }

// Underline is underlined text.
type Underline struct{ Inlines []Inline }

// Superscript is raised text.
type Superscript struct{ Inlines []Inline }

// Subscript is lowered text.
type Subscript struct{ Inlines []Inline }

// SmallCaps is small-capital text.
type SmallCaps struct{ Inlines []Inline }

// Highlight is marked (highlighted) text.
type Highlight struct{ Inlines []Inline }

// Code is inline code.
type Code struct{ Text string }

// Math is inline mathematics in LaTeX notation, without delimiters.
type Math struct{ TeX string }

// Link is a hyperlink. URL may be external ("https://…", "mailto:…") or an
// internal anchor ("#section-id").
type Link struct {
	URL     string
	Title   string
	Inlines []Inline
}

// Image is an image reference. Src is a path relative to the document base
// directory, an absolute path, a "res:" resource name, a data: URI, or an
// http(s) URL. Width and Height accept CSS-like lengths: "50%", "8cm",
// "3in", "240px", "120pt"; empty means natural size.
type Image struct {
	Src    string
	Alt    string
	Title  string
	Width  string
	Height string
	Attr   Attr
}

// Note is a footnote; its content is block-level.
type Note struct{ Blocks []Block }

// CiteMode selects how a citation is rendered.
type CiteMode int

const (
	// CiteParenthetical renders "(Smith, 2020)" / "[1]".
	CiteParenthetical CiteMode = iota
	// CiteNarrative renders "Smith (2020)" / "Smith [1]".
	CiteNarrative
)

// CiteItem cites one reference.
type CiteItem struct {
	Key            string
	Prefix         string // e.g. "see"
	Locator        string // e.g. "33-35"
	LocatorLabel   string // "page", "chapter", "section", "figure", ... ("" = page)
	Suffix         string // free text after the locator
	SuppressAuthor bool   // [-@key]
}

// Cite is a citation of one or more references. Rendered is filled in by
// the citation processor; Fallback holds the source text used when no
// processor ran or the keys are unknown.
type Cite struct {
	Items    []CiteItem
	Mode     CiteMode
	Fallback []Inline
	Rendered []Inline
}

// Ref is a cross-reference to a labelled figure, table, section, equation
// or listing (`@fig:arch`). Writers render a localised "Figure 3".
type Ref struct {
	Target string
	// Bare omits the "Figure"/"Table" prefix (`[-@fig:x]`).
	Bare bool
}

// LineBreak is a hard line break.
type LineBreak struct{}

// SoftBreak is a source line break inside a paragraph (rendered as a space).
type SoftBreak struct{}

// Span is a generic inline container (classes: "kbd", "mark", "nowrap",
// "smallcaps", "underline").
type Span struct {
	Attr    Attr
	Inlines []Inline
}

// RawInline is inline source passed through to a writer of the same Format.
type RawInline struct {
	Format string
	Text   string
}

func (*Text) isInline()        {}
func (*Emph) isInline()        {}
func (*Strong) isInline()      {}
func (*Strike) isInline()      {}
func (*Underline) isInline()   {}
func (*Superscript) isInline() {}
func (*Subscript) isInline()   {}
func (*SmallCaps) isInline()   {}
func (*Highlight) isInline()   {}
func (*Code) isInline()        {}
func (*Math) isInline()        {}
func (*Link) isInline()        {}
func (*Image) isInline()       {}
func (*Note) isInline()        {}
func (*Cite) isInline()        {}
func (*Ref) isInline()         {}
func (*LineBreak) isInline()   {}
func (*SoftBreak) isInline()   {}
func (*Span) isInline()        {}
func (*RawInline) isInline()   {}
