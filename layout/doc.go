// Package layout reconstructs a structured document — title, headings,
// paragraphs, lists, tables, figures with captions, footnotes, block
// quotations and code — from text positioned on pages.
//
// It is the analysis stage behind the PDF reader, and it works equally on
// the output of an OCR engine: the input is nothing more than pieces of
// text with their boxes.
//
// # Input
//
// Each [Page] lists text [Run]s with their bounding boxes, plus optional
// raster [Image]s and [Link] areas. All coordinates are in points
// (1/72 inch) with the origin at the top-left corner of the page, x growing
// to the right and y growing downwards. A box is its top-left corner (X, Y)
// and its size (W, H). The order of runs within a page does not matter, and
// a run may hold a single glyph, a word or a whole line: runs that contain
// spaces are split into words with proportional boxes.
//
// Text extracted from a PDF content stream carries a font size and the
// bold, italic and monospace flags of its font; for such runs the box
// should span the font's em height (top = baseline - 0.8 × size, height =
// size), which is how the bundled PDF reader fills it.
//
// # OCR output
//
// OCR engines report words with pixel boxes and usually no font
// information. Map them as follows:
//
//   - Convert pixels to points with the scan resolution:
//     pt = px × 72 / dpi. Tesseract TSV (left, top, width, height), hOCR
//     ("bbox x0 y0 x1 y1", i.e. corners) and ALTO (HPOS, VPOS, WIDTH,
//     HEIGHT, when MeasurementUnit is pixel) all use a top-left origin, so
//     only the scale changes. Engines with a bottom-left origin need
//     Y = pageHeight - top.
//   - Emit one Run per recognised word with Text, X, Y, W and H. Leave
//     FontSize at 0: the size is then estimated from the box heights of
//     each line (heights vary with ascenders and descenders, so a median
//     per line is used). Leave Bold, Italic and Mono false unless the
//     engine reports font attributes.
//   - Set Page.Width and Page.Height to the image size in points.
//   - Filter low-confidence words before calling; layout does not see
//     confidences.
//   - Engines that detect paragraphs may pass the paragraph number in
//     Run.Block, which then keeps those words in one block.
//
// Reconstruction is fully automatic when every FontSize is 0 (OCR mode):
// headings are then recognised from relative box heights, spacing and
// position rather than from font metrics.
//
// # Output
//
// [Reconstruct] returns ast blocks, metadata found in the layout (title,
// subtitle, authors, date, abstract, keywords, manual section numbering)
// and non-fatal warnings. Images are stored in the given [ast.Resources]
// and referenced with "res:" sources. [Document] wraps the result in an
// [ast.Document].
package layout
