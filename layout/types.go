package layout

// Run is a piece of text with its bounding box on the page. Coordinates are
// in points with a top-left origin (see the package documentation).
type Run struct {
	// Text is the text of the run: a glyph, a word or a line.
	Text string
	// X and Y locate the top-left corner of the box; W and H are its
	// width and height.
	X, Y, W, H float64
	// FontSize is the font size in points; 0 means unknown, in which case
	// it is estimated from the box height.
	FontSize float64
	// Bold, Italic and Mono describe the font, when known.
	Bold, Italic, Mono bool
	// FontName is the font's name without a subset prefix; optional.
	FontName string
	// Block optionally groups runs into logical blocks: runs that share a
	// non-zero Block on a page belong to the same paragraph, heading, list
	// item, table cell or note, and runs with different values are never
	// merged into one paragraph. OCR engines that detect paragraphs and
	// tagged PDFs provide this; 0 means unknown.
	Block int
	// Role optionally states what the run's block is. It is a hint that
	// overrides the layout analysis; RoleAuto lets the analysis decide.
	Role Role
}

// Role is the logical role of a block of runs, as known to the producer
// of the input (for example from the structure tree of a tagged PDF).
type Role uint8

// Roles. Heading levels are given by the role (RoleHeading1 is the
// highest level).
const (
	RoleAuto Role = iota
	RoleParagraph
	RoleTitle
	RoleHeading1
	RoleHeading2
	RoleHeading3
	RoleHeading4
	RoleHeading5
	RoleHeading6
	RoleListLabel // a list marker ("1.", "•")
	RoleListBody  // the text of a list item
	RoleTableHeader
	RoleTableCell
	RoleCaption
	RoleNote      // footnote or endnote text, including its own marker
	RoleNoteRef   // a footnote reference mark in the text
	RoleQuote     // block quotation
	RoleCode      // preformatted code
	RoleFormula   // mathematics
	RoleArtifact  // page furniture (running heads, page numbers): dropped
	RoleTOC       // table of contents entries: dropped
	RoleReference // bibliography entry
)

// headingLevel returns the level of a heading role, or 0.
func (r Role) headingLevel() int {
	if r >= RoleHeading1 && r <= RoleHeading6 {
		return int(r-RoleHeading1) + 1
	}
	return 0
}

// Image is a raster image placed on a page.
type Image struct {
	// X, Y, W, H is the box the image is drawn in, in points.
	X, Y, W, H float64
	// Data is the encoded image.
	Data []byte
	// MediaType is "image/png", "image/jpeg" or another image type.
	MediaType string
}

// Link is a clickable area pointing to a URL. Words inside the area
// become link text.
type Link struct {
	X, Y, W, H float64
	URL        string
}

// Page is one page of positioned content.
type Page struct {
	// Width and Height are the page size in points. When zero they are
	// derived from the content.
	Width, Height float64
	Runs          []Run
	Images        []Image
	Links         []Link
}

// Options tunes reconstruction.
type Options struct {
	// Lang is the document language as a BCP 47 tag ("en", "lv"), when
	// known. It informs language-specific decisions such as removing
	// hyphenation; empty means unknown.
	Lang string
}
