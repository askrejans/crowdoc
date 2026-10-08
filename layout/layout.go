package layout

import (
	"context"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// doc is the reconstruction state for one document.
type doc struct {
	ctx   context.Context
	opts  Options
	res   *ast.Resources
	pages []*pageData
	flows []*flow
	notes []*footnote
	// noteOf maps reference marks to their footnotes.
	noteOf map[*word]*footnote
	meta   ast.Meta

	body      float64             // body text size
	bodyMono  bool                // body text is monospaced (plain-text printouts)
	pitches   map[float64]float64 // line pitch by font size
	freq      map[string]int      // word frequencies, for de-hyphenation
	indentDoc bool                // paragraphs start with a first-line indent
	// taggedHeadings is set when the producer marked headings: untagged
	// lines are then not taken for headings.
	taggedHeadings bool
	imgCount       int
}

// minFigureSide is the smallest image (points) kept as a figure; smaller
// ones are decorations.
const minFigureSide = 24.0

// Reconstruct converts positioned page content into document blocks. It
// also returns metadata recovered from the layout (title, subtitle,
// authors, a run-in abstract, manual section numbering) and non-fatal
// warnings. Images are stored in res.
func Reconstruct(ctx context.Context, pages []Page, res *ast.Resources, o Options) ([]ast.Block, ast.Meta, []string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if res == nil {
		res = ast.NewResources()
	}
	d := &doc{ctx: ctx, opts: o, res: res}
	ocr := true
	for _, p := range pages {
		for _, r := range p.Runs {
			if r.FontSize > 0 {
				ocr = false
			}
			if r.Role.headingLevel() > 0 {
				d.taggedHeadings = true
			}
		}
	}
	for i, p := range pages {
		if ctx.Err() != nil {
			return nil, d.meta, nil
		}
		pd := &pageData{idx: i, w: p.Width, h: p.Height, links: p.Links}
		pd.words = makeWords(p)
		if pd.h <= 0 || pd.w <= 0 {
			pd.w, pd.h = extent(pd.words)
		}
		for _, im := range p.Images {
			if im.W < minFigureSide || im.H < minFigureSide || len(im.Data) == 0 || !finite(im.X, im.Y, im.W, im.H) {
				continue
			}
			pd.images = append(pd.images, &pageImage{x0: im.X, y0: im.Y, x1: im.X + im.W, y1: im.Y + im.H,
				data: im.Data, mediaType: im.MediaType, page: i})
		}
		assignLinks(pd.words, p.Links)
		pd.lines = buildLines(pd.words, i, ocr)
		d.pages = append(d.pages, pd)
	}
	d.bodyStats()
	var titleWords []*word
	if len(d.pages) > 0 {
		for _, k := range d.titleBlock(d.pages[0].lines, d.pages[0].h) {
			l := d.pages[0].lines[k]
			for _, w := range l.words {
				if w.size >= 0.97*l.size {
					titleWords = append(titleWords, w)
				}
			}
		}
	}
	removeFurniture(d.pages, d.body, titleWords)
	d.wordStats()
	for _, p := range d.pages {
		if ctx.Err() != nil {
			return nil, d.meta, nil
		}
		gutters := findGutters(p, d.body)
		d.flows = append(d.flows, buildFlows(p, gutters)...)
	}
	d.meta = d.extractTitle()
	d.coverMeta()
	d.removeContents()
	d.columnStats()
	d.flowStats()
	for _, f := range d.flows {
		d.extractFootnotes(f, d.pages[f.page])
	}
	blocks := d.assemble()
	if ctx.Err() != nil {
		return nil, d.meta, nil
	}
	blocks = extractFrontMatter(blocks, &d.meta)
	blocks = append(blocks, d.endNotes()...)
	return blocks, d.meta, nil
}

// Document reconstructs pages into a complete document: the blocks, the
// metadata found in the layout, and the page images as resources. The
// language hint becomes the document language unless the layout says
// otherwise.
func Document(ctx context.Context, pages []Page, opts Options) (*ast.Document, []string) {
	res := ast.NewResources()
	blocks, meta, warns := Reconstruct(ctx, pages, res, opts)
	if meta.Lang == "" {
		meta.Lang = opts.Lang
	}
	return &ast.Document{Meta: meta, Blocks: blocks, Resources: res}, warns
}

func extent(ws []*word) (float64, float64) {
	w, h := 0.0, 0.0
	for _, x := range ws {
		w, h = max(w, x.x1), max(h, x.y1)
	}
	return w, h
}

// assignLinks attaches link targets to the words inside link areas.
func assignLinks(ws []*word, links []Link) {
	for _, lb := range links {
		if lb.URL == "" || !finite(lb.X, lb.Y, lb.W, lb.H) {
			continue
		}
		for _, w := range ws {
			cx, cy := w.cx(), w.cy()
			if cx >= lb.X-1 && cx <= lb.X+lb.W+1 && cy >= lb.Y-1 && cy <= lb.Y+lb.H+1 {
				w.link = lb.URL
			}
		}
	}
}

// bodyStats finds the body text size (the size covering most characters)
// and whether body text is monospaced.
func (d *doc) bodyStats() {
	sizes := map[float64]int{}
	mono := map[float64]int{}
	for _, p := range d.pages {
		for _, w := range p.words {
			n := utf8.RuneCountInString(w.text)
			s := roundTo(w.size, 0.5)
			sizes[s] += n
			if w.mono {
				mono[s] += n
			}
		}
	}
	best, bestN := 0.0, 0
	for s, n := range sizes {
		if n > bestN || (n == bestN && s < best) {
			best, bestN = s, n
		}
	}
	d.body = best
	if d.body <= 0 {
		d.body = 10
	}
	d.bodyMono = bestN > 0 && mono[best]*10 >= bestN*6
}

func (d *doc) wordStats() {
	d.freq = map[string]int{}
	for _, p := range d.pages {
		for _, w := range p.words {
			if k := wordKey(w.text); k != "" {
				d.freq[k]++
			}
		}
	}
}

// flowStats measures line pitch per font size and whether paragraphs are
// marked by first-line indents.
func (d *doc) flowStats() {
	samples := map[float64][]float64{}
	indented, flush := 0, 0
	for _, f := range d.flows {
		var prev *line
		for k, it := range f.items {
			l := it.l
			if l == nil {
				prev = nil
				continue
			}
			if prev != nil && roundTo(prev.size, 0.5) == roundTo(l.size, 0.5) {
				if pitch := l.base - prev.base; pitch > 0.8*l.size && pitch < 2.5*l.size {
					s := roundTo(l.size, 0.5)
					samples[s] = append(samples[s], pitch)
				}
			}
			if prev != nil && k+1 < len(f.items) && f.items[k+1].l != nil {
				next := f.items[k+1].l
				ind := l.x0 - f.left
				prevEnds := endsSentence(prev.text()) || short(prev, f)
				if prevEnds && prev.x0-f.left <= 0.3*l.size && next.x0-f.left <= 0.3*l.size {
					if ind > 0.6*l.size && ind < 4*l.size {
						indented++
					} else if ind <= 0.3*l.size && short(prev, f) && !startsLower(l.text()) && l.base-prev.base < 1.3*l.size*1.25 {
						flush++
					}
				}
			}
			prev = l
		}
		f.ragged = raggedFlow(f)
	}
	d.pitches = map[float64]float64{}
	for s, v := range samples {
		if len(v) >= 2 {
			d.pitches[s] = linePitch(v)
		}
	}
	d.indentDoc = indented >= 2 && indented > flush
}

// columnStats records a multi-column layout in the metadata when most of
// the body text is set in columns.
func (d *doc) columnStats() {
	byCols := map[int]int{}
	total := 0
	for _, f := range d.flows {
		for _, it := range f.items {
			if it.l != nil && math.Abs(it.l.size-d.body) < 0.1*d.body {
				n := it.l.chars()
				byCols[f.cols] += n
				total += n
			}
		}
	}
	best, bestN := 1, 0
	for c, n := range byCols {
		if n > bestN {
			best, bestN = c, n
		}
	}
	if best >= 2 && best <= 3 && bestN*2 > total {
		d.meta.Columns = best
	}
}

// linePitch picks the line pitch from baseline distances: the smallest
// distance shared by a good part of the samples. Paragraph and list item
// spacing produce larger distances that may well be more frequent (a list
// of one-line items), so the plain median would overestimate it.
func linePitch(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	need := max(2, len(s)/5)
	for i, j := 0, 0; i < len(s); i++ {
		for j < len(s) && s[j]-s[i] <= max(0.6, 0.04*s[i]) {
			j++
		}
		if j-i >= need {
			return median(s[i:j])
		}
	}
	return median(s)
}

// pitchFor returns the usual baseline distance for text of the given size.
func (d *doc) pitchFor(size float64) float64 {
	if p, ok := d.pitches[roundTo(size, 0.5)]; ok {
		return p
	}
	if p, ok := d.pitches[roundTo(d.body, 0.5)]; ok && d.body > 0 {
		return p * size / d.body
	}
	return 1.2 * size
}

// raggedFlow reports whether a flow is set ragged right (line ends vary).
func raggedFlow(f *flow) bool {
	n, off := 0, 0
	for _, it := range f.items {
		if it.l == nil {
			continue
		}
		n++
		if it.l.x1 < f.right-1.5*it.l.size {
			off++
		}
	}
	return n >= 4 && off*10 >= n*4
}
