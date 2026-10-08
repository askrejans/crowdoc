// Package pdf reads text-based PDF files and reconstructs a semantic
// document from the positioned text, so the content can be typeset again.
//
// The package contains its own small PDF parser (cross-reference tables and
// streams, object streams, the standard filters, RC4/AES encryption with an
// empty user password, and recovery of damaged files) and a content-stream
// interpreter that turns glyphs into positioned text runs with font size and
// style. Layout analysis (columns, paragraphs, headings, lists, tables,
// footnotes) is done by the layout package, which also serves OCR output.
package pdf

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
	"github.com/askrejans/crowdoc/v2/layout"
)

const (
	// MaxPages is the number of pages read; later pages are dropped with a
	// warning.
	MaxPages = 2000
	// MaxSize is the largest PDF accepted.
	MaxSize = 512 << 20
)

// ErrNoText is returned for PDFs without a text layer.
var ErrNoText = errors.New("this PDF has no text layer (it is a scan); run OCR first")

// Read parses a PDF document and returns it with non-fatal warnings.
func Read(ctx context.Context, data []byte, o rd.Options) (doc *ast.Document, warnings []string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(data) > MaxSize {
		return nil, nil, fmt.Errorf("pdf: %w: file is larger than %d MiB", rd.ErrLimit, MaxSize>>20)
	}
	defer func() {
		// Last line of defence for malformed input: a bug in the parser
		// must not take the caller down. The fuzz tests call read directly
		// so that such bugs surface.
		if r := recover(); r != nil {
			doc, warnings, err = nil, nil, fmt.Errorf("pdf: malformed file (%v)", r)
		}
	}()
	return read(ctx, data, o)
}

func read(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	ex, err := extract(ctx, data, o)
	if err != nil {
		return nil, nil, err
	}
	res := ast.NewResources()
	blocks, hint, lw := layout.Reconstruct(ctx, ex.pages, res, layout.Options{Lang: ex.info.lang})
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, w := range lw {
		ex.warn.Addf("%s", w)
	}
	doc := &ast.Document{Meta: mergeMeta(ex.info, hint), Blocks: blocks, Resources: res}
	return doc, ex.warn.List(), nil
}

// extraction is the positioned content and metadata of a PDF.
type extraction struct {
	pages []layout.Page
	info  docInfo
	warn  rd.Warnings
}

// extractPages returns the positioned content of every page.
func extractPages(ctx context.Context, data []byte, o rd.Options) ([]layout.Page, error) {
	ex, err := extract(ctx, data, o)
	if err != nil {
		return nil, err
	}
	return ex.pages, nil
}

func extract(ctx context.Context, data []byte, o rd.Options) (*extraction, error) {
	ex := &extraction{}
	f, err := openFile(data, o.Limits)
	if err != nil {
		if errors.Is(err, ErrPassword) {
			return nil, err
		}
		return nil, fmt.Errorf("pdf: %w", err)
	}
	pages, truncated := f.pages(MaxPages)
	if truncated {
		ex.warn.Addf("only the first %d pages were read", MaxPages)
	}
	if len(pages) == 0 {
		return nil, errors.New("pdf: the document has no pages")
	}
	ex.pages = make([]layout.Page, 0, len(pages))
	var structHints map[ref]map[int]mcHint
	if useStructure {
		structHints = f.structureHints()
	}
	images := map[*stream]imageData{}
	chars, scanPages, rotated := 0, 0, 0
	for i, p := range pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pc := f.extractPage(ctx, p, structHints[p.ref])
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if pc.truncated {
			ex.warn.Addf("page %d is too complex; its content was truncated", i+1)
		}
		rotated += pc.rotated
		lp := layout.Page{Width: p.box.w(), Height: p.box.h(), Runs: append(pc.runs, checkboxRuns(pc.marks, pc.runs)...)}
		if p.rotate == 90 || p.rotate == 270 {
			lp.Width, lp.Height = lp.Height, lp.Width
		}
		chars += pc.visibleChars + pc.invisibleChars
		pageArea := lp.Width * lp.Height
		for _, im := range pc.images {
			cover := im.w * im.h / max(pageArea, 1)
			if cover > 0.5 && pc.visibleChars < 40 {
				scanPages++
			}
			if cover > 0.7 && pc.invisibleChars > 0 {
				// The scanned page behind an OCR text layer.
				continue
			}
			d, ok := images[im.obj]
			if !ok {
				d.data, d.mediaType, d.err = f.decodeImage(im.obj)
				images[im.obj] = d
			}
			if d.err != nil {
				ex.warn.Addf("an image on page %d was skipped: %v", i+1, d.err)
				continue
			}
			lp.Images = append(lp.Images, layout.Image{X: im.x, Y: im.y, W: im.w, H: im.h, Data: d.data, MediaType: d.mediaType})
		}
		in := &interp{page: p}
		lp.Links = f.links(p, in.toDisplay)
		ex.pages = append(ex.pages, lp)
	}
	if chars == 0 || (chars < 20*len(pages) && scanPages*2 >= len(pages)) {
		return nil, ErrNoText
	}
	checkHints(ex.pages)
	if rotated > 0 {
		ex.warn.Addf("%d characters of rotated or vertical text were skipped", rotated)
	}
	unmapped := 0
	for _, ft := range f.fonts {
		if ft != nil {
			unmapped += ft.unmapped
		}
	}
	if unmapped > 0 && unmapped*10 > chars {
		ex.warn.Addf("some text could not be decoded (%s of %s characters use fonts without a Unicode mapping)", strconv.Itoa(unmapped), strconv.Itoa(chars))
	}
	ex.info = f.info()
	return ex, nil
}

// useStructure enables structure-tree hints; tests turn it off to check
// the purely geometric analysis on tagged files.
var useStructure = true

// checkHints drops structure hints that cannot be trusted: artifacts that
// cover most of a page (a producer marking real content as furniture) and
// structure trees that leave most of the text untagged.
func checkHints(pages []layout.Page) {
	tagged, total := 0, 0
	for i := range pages {
		runs := pages[i].Runs
		art, all := 0, 0
		for _, r := range runs {
			n := utf8.RuneCountInString(r.Text)
			all += n
			if r.Role == layout.RoleArtifact {
				art += n
			} else if r.Block != 0 {
				tagged += n
			}
		}
		total += all - art
		if art*10 > all*6 {
			for k := range runs {
				if runs[k].Role == layout.RoleArtifact {
					runs[k].Role = layout.RoleAuto
				}
			}
			total += art
		}
	}
	if tagged*2 >= total {
		return
	}
	for i := range pages {
		for k := range pages[i].Runs {
			if r := &pages[i].Runs[k]; r.Role != layout.RoleArtifact {
				r.Block, r.Role = 0, layout.RoleAuto
			}
		}
	}
}

type imageData struct {
	data      []byte
	mediaType string
	err       error
}
