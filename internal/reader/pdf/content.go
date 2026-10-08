package pdf

import (
	"context"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/layout"
)

const (
	maxOpsPerPage = 20_000_000
	maxFormDepth  = 12
	maxOperands   = 4096
	// minImageSide is the smallest placed image (points) worth extracting.
	minImageSide = 24.0
)

type gstate struct {
	ctm       matrix
	font      *pdfFont
	fontSize  float64
	charSpace float64
	wordSpace float64
	hscale    float64
	leading   float64
	rise      float64
	render    int
}

// imagePlacement is an image XObject drawn on the page.
type imagePlacement struct {
	obj        *stream
	x, y, w, h float64 // display box
}

// markedContent is one level of BMC/BDC nesting.
type markedContent struct {
	actual   *actualSpan // the /ActualText replacement, if any
	mcid     int         // the /MCID linking to the structure tree, or -1
	artifact bool        // /Artifact: page furniture outside the content
}

// actualSpan collects the glyphs covered by a marked-content /ActualText.
type actualSpan struct {
	text    string
	started bool
	first   placedGlyph
	endX    float64
}

// placedGlyph is a glyph in display coordinates.
type placedGlyph struct {
	text               string
	x, y               float64 // baseline origin
	adv                float64 // advance along the baseline
	size               float64
	font               *pdfFont
	bold, italic, mono bool
	invisible          bool
	mcid               int // innermost marked-content id, or -1
	artifact           bool
}

// pageContent is what the interpreter extracts from one page.
type pageContent struct {
	runs           []layout.Run
	images         []imagePlacement
	visibleChars   int
	invisibleChars int
	rotated        int
	truncated      bool
	marks          []vecMark // check boxes and check marks
}

type interp struct {
	f    *file
	ctx  context.Context
	page pageInfo
	out  *pageContent

	dispW, dispH float64
	gs           gstate
	stack        []gstate
	tm, tlm      matrix
	inText       bool

	cur      runBuilder
	accent   *placedGlyph
	marks    []markedContent
	trackEm  float64 // letter spacing of the current TJ, in em
	path     pathBox
	hints    map[int]mcHint // structure hints of this page by MCID
	ops      int
	forms    []ref
	canceled bool
}

type runBuilder struct {
	active             bool
	sb                 strings.Builder
	font               *pdfFont
	size               float64
	baseY              float64
	x0, endX           float64
	bold, italic, mono bool
	lastX0, lastX1     float64 // last glyph box (for accent composition)
	lastText           string
	mcid               int
	artifact           bool
}

// extractPage interprets the page content streams.
func (f *file) extractPage(ctx context.Context, p pageInfo, hints map[int]mcHint) *pageContent {
	out := &pageContent{}
	in := &interp{f: f, ctx: ctx, page: p, out: out, hints: hints}
	in.dispW, in.dispH = p.box.w(), p.box.h()
	if p.rotate == 90 || p.rotate == 270 {
		in.dispW, in.dispH = in.dispH, in.dispW
	}
	in.gs = gstate{ctm: identity, hscale: 1}
	var content []byte
	switch c := f.resolve(p.d["Contents"]).(type) {
	case *stream:
		content, _, _, _ = f.decodeStream(c, false)
	case array:
		var parts [][]byte
		for _, e := range c {
			if s, ok := f.resolve(e).(*stream); ok {
				if data, _, _, err := f.decodeStream(s, false); err == nil {
					parts = append(parts, data)
				}
			}
		}
		content = joinStreams(parts)
	}
	in.run(content, p.resources, 0)
	in.flush()
	return out
}

func joinStreams(parts [][]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p) + 1
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
		out = append(out, '\n')
	}
	return out
}

// toDisplay maps user space to top-left display coordinates, applying the
// crop box origin and the page rotation.
func (in *interp) toDisplay(x, y float64) (float64, float64) {
	b := in.page.box
	switch in.page.rotate {
	case 90:
		return y - b.y0, x - b.x0
	case 180:
		return b.x1 - x, y - b.y0
	case 270:
		return b.y1 - y, b.x1 - x
	}
	return x - b.x0, b.y1 - y
}

func (in *interp) run(content []byte, res dict, depth int) {
	lx := newLexer(content)
	operands := make([]any, 0, 16)
	for {
		if in.canceled {
			return
		}
		tok, ok := lx.next()
		if !ok {
			return
		}
		op, isOp := tok.(keyword)
		if !isOp {
			v, _ := lx.objectFrom(tok, 0)
			if len(operands) < maxOperands {
				operands = append(operands, v)
			}
			continue
		}
		in.ops++
		if in.ops&0xFFFF == 0 && in.ctx.Err() != nil {
			in.canceled = true
			return
		}
		if in.ops > maxOpsPerPage {
			in.out.truncated = true
			in.canceled = true
			return
		}
		if op == "BI" {
			in.skipInlineImage(lx)
			operands = operands[:0]
			continue
		}
		in.do(op, operands, res, depth)
		operands = operands[:0]
	}
}

// letterSpacing recognises a TJ array that spreads its glyphs evenly (one
// glyph per string, the same negative adjustment between all of them) and
// returns that spacing in em; 0 otherwise.
func letterSpacing(a array) float64 {
	counts := map[float64]int{}
	strs, between := 0, 0
	prevString := false
	for _, e := range a {
		switch x := e.(type) {
		case pdfString:
			strs++
			if len(x) > 2 {
				return 0
			}
			prevString = true
		default:
			if adj, ok := num(x); ok && prevString {
				counts[math.Round(adj)]++
				between++
			}
			prevString = false
		}
	}
	if strs < 4 || between < 3 {
		return 0
	}
	mode, n := 0.0, 0
	for v, c := range counts {
		if c > n {
			mode, n = v, c
		}
	}
	if mode >= 0 || mode < -400 || n*10 < between*6 {
		return 0
	}
	return -mode / 1000
}

func nums(ops []any, n int) ([]float64, bool) {
	if len(ops) < n {
		return nil, false
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		v, ok := num(ops[len(ops)-n+i])
		if !ok {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

func (in *interp) do(op keyword, ops []any, res dict, depth int) {
	gs := &in.gs
	switch op {
	case "q":
		if len(in.stack) < 256 {
			in.stack = append(in.stack, *gs)
		}
	case "Q":
		if n := len(in.stack); n > 0 {
			*gs = in.stack[n-1]
			in.stack = in.stack[:n-1]
		}
	case "cm":
		if v, ok := nums(ops, 6); ok {
			gs.ctm = matrix{v[0], v[1], v[2], v[3], v[4], v[5]}.mul(gs.ctm)
		}
	case "BT":
		in.tm, in.tlm = identity, identity
		in.inText = true
	case "ET":
		in.inText = false
	case "Tc":
		if v, ok := nums(ops, 1); ok {
			gs.charSpace = v[0]
		}
	case "Tw":
		if v, ok := nums(ops, 1); ok {
			gs.wordSpace = v[0]
		}
	case "Tz":
		if v, ok := nums(ops, 1); ok {
			gs.hscale = v[0] / 100
		}
	case "TL":
		if v, ok := nums(ops, 1); ok {
			gs.leading = v[0]
		}
	case "Ts":
		if v, ok := nums(ops, 1); ok {
			gs.rise = v[0]
		}
	case "Tr":
		if v, ok := nums(ops, 1); ok {
			gs.render = int(v[0])
		}
	case "Tf":
		if len(ops) >= 2 {
			if size, ok := num(ops[len(ops)-1]); ok {
				gs.fontSize = size
			}
			if n, ok := ops[len(ops)-2].(name); ok {
				gs.font = in.font(res, n)
			}
		}
	case "Td":
		if v, ok := nums(ops, 2); ok {
			in.tlm = matrix{1, 0, 0, 1, v[0], v[1]}.mul(in.tlm)
			in.tm = in.tlm
		}
	case "TD":
		if v, ok := nums(ops, 2); ok {
			gs.leading = -v[1]
			in.tlm = matrix{1, 0, 0, 1, v[0], v[1]}.mul(in.tlm)
			in.tm = in.tlm
		}
	case "Tm":
		if v, ok := nums(ops, 6); ok {
			in.tlm = matrix{v[0], v[1], v[2], v[3], v[4], v[5]}
			in.tm = in.tlm
		}
	case "T*":
		in.nextLine()
	case "Tj":
		if len(ops) >= 1 {
			if s, ok := ops[len(ops)-1].(pdfString); ok {
				in.show([]byte(s))
			}
		}
	case "'":
		in.nextLine()
		if len(ops) >= 1 {
			if s, ok := ops[len(ops)-1].(pdfString); ok {
				in.show([]byte(s))
			}
		}
	case "\"":
		if len(ops) >= 3 {
			if v, ok := num(ops[len(ops)-3]); ok {
				gs.wordSpace = v
			}
			if v, ok := num(ops[len(ops)-2]); ok {
				gs.charSpace = v
			}
			in.nextLine()
			if s, ok := ops[len(ops)-1].(pdfString); ok {
				in.show([]byte(s))
			}
		}
	case "TJ":
		if len(ops) >= 1 {
			if a, ok := ops[len(ops)-1].(array); ok {
				in.trackEm = letterSpacing(a)
				defer func() { in.trackEm = 0 }()
				for _, e := range a {
					switch x := e.(type) {
					case pdfString:
						in.show([]byte(x))
					default:
						if adj, ok := num(x); ok {
							in.tm = matrix{1, 0, 0, 1, -adj / 1000 * gs.fontSize * gs.hscale, 0}.mul(in.tm)
						}
					}
				}
			}
		}
	case "Do":
		if len(ops) >= 1 {
			if n, ok := ops[len(ops)-1].(name); ok {
				in.xobject(res, n, depth)
			}
		}
	case "gs":
		if len(ops) >= 1 {
			if n, ok := ops[len(ops)-1].(name); ok {
				in.extGState(res, n)
			}
		}
	case "BMC":
		mc := markedContent{mcid: -1}
		if len(ops) >= 1 && ops[len(ops)-1] == name("Artifact") {
			mc.artifact = true
		}
		in.marks = append(in.marks, mc)
	case "BDC":
		in.beginMarked(ops, res)
	case "EMC":
		in.endMarked()
	default:
		in.pathOp(op, ops)
	}
}

func (in *interp) nextLine() {
	in.tlm = matrix{1, 0, 0, 1, 0, -in.gs.leading}.mul(in.tlm)
	in.tm = in.tlm
}

func (in *interp) extGState(res dict, n name) {
	gsd, _ := in.f.resolve(in.f.resolveDict(res, "ExtGState")[n]).(dict)
	if gsd == nil {
		return
	}
	if fa, ok := in.f.resolve(gsd["Font"]).(array); ok && len(fa) == 2 {
		if size, ok := num(in.f.resolve(fa[1])); ok {
			in.gs.fontSize = size
		}
		if r, ok := fa[0].(ref); ok {
			in.gs.font = in.f.fontByRef(r)
		}
	}
}

func (f *file) resolveDict(d dict, key name) dict {
	if d == nil {
		return nil
	}
	out, _ := f.resolve(d[key]).(dict)
	return out
}

func (in *interp) font(res dict, n name) *pdfFont {
	fonts := in.f.resolveDict(res, "Font")
	v := fonts[n]
	if r, ok := v.(ref); ok {
		return in.f.fontByRef(r)
	}
	if fd, ok := v.(dict); ok {
		return in.f.loadFont(fd)
	}
	return nil
}

func (f *file) fontByRef(r ref) *pdfFont {
	if f.fonts == nil {
		f.fonts = map[ref]*pdfFont{}
	}
	if ft, ok := f.fonts[r]; ok {
		return ft
	}
	f.fonts[r] = nil // guards against recursion
	fd, _ := f.resolve(r).(dict)
	if fd == nil {
		return nil
	}
	ft := f.loadFont(fd)
	f.fonts[r] = ft
	return ft
}

func (in *interp) beginMarked(ops []any, res dict) {
	mc := markedContent{mcid: -1}
	var props dict
	if len(ops) >= 2 {
		mc.artifact = ops[len(ops)-2] == name("Artifact")
		switch p := ops[len(ops)-1].(type) {
		case dict:
			props = p
		case name:
			props, _ = in.f.resolve(in.f.resolveDict(res, "Properties")[p]).(dict)
		}
	}
	if s, ok := props["ActualText"].(pdfString); ok {
		mc.actual = &actualSpan{text: textString(s)}
	}
	// MCIDs inside form XObjects belong to the form's own structure
	// parents; only page-level ones are linked.
	if id, ok := integer(props["MCID"]); ok && id >= 0 && len(in.forms) == 0 {
		mc.mcid = id
	}
	in.marks = append(in.marks, mc)
}

func (in *interp) endMarked() {
	n := len(in.marks)
	if n == 0 {
		return
	}
	sp := in.marks[n-1].actual
	in.marks = in.marks[:n-1]
	if sp == nil || !sp.started {
		return
	}
	g := sp.first
	g.text = sp.text
	g.adv = sp.endX - g.x
	if strings.TrimSpace(g.text) == "" {
		if g.text != "" {
			in.endRun()
		}
		return
	}
	in.emit(g)
}

func (in *interp) activeSpan() *actualSpan {
	for i := len(in.marks) - 1; i >= 0; i-- {
		if in.marks[i].actual != nil {
			return in.marks[i].actual
		}
	}
	return nil
}

// markState returns the innermost MCID and whether the current content is
// an artifact.
func (in *interp) markState() (int, bool) {
	mcid, artifact := -1, false
	for i := len(in.marks) - 1; i >= 0; i-- {
		if mcid < 0 && in.marks[i].mcid >= 0 {
			mcid = in.marks[i].mcid
		}
		if in.marks[i].artifact {
			artifact = true
		}
	}
	return mcid, artifact
}

// show renders a string operand glyph by glyph.
func (in *interp) show(s []byte) {
	gs := &in.gs
	ft := gs.font
	if ft == nil {
		return
	}
	ft.decode(s, func(g glyph) {
		in.showGlyph(ft, g)
		tx := (g.w*gs.fontSize + gs.charSpace) * gs.hscale
		if g.space {
			tx += gs.wordSpace * gs.hscale
		}
		in.tm = matrix{1, 0, 0, 1, tx, 0}.mul(in.tm)
	})
}

func (in *interp) showGlyph(ft *pdfFont, g glyph) {
	gs := &in.gs
	if ft.vertical {
		in.out.rotated++
		return
	}
	trm := matrix{gs.fontSize * gs.hscale, 0, 0, gs.fontSize, 0, gs.rise}.mul(in.tm).mul(gs.ctm)
	ox, oy := trm.apply(0, 0)
	ex, ey := trm.apply(g.w, 0)
	ux, uy := trm.apply(0, 1)
	x0, y0 := in.toDisplay(ox, oy)
	x1, y1 := in.toDisplay(ex, ey)
	tx, ty := in.toDisplay(ux, uy)
	dx, dy := x1-x0, y1-y0
	vx, vy := tx-x0, ty-y0
	size := math.Hypot(vx, vy)
	if ft.type3 {
		size *= math.Abs(ft.fm[3]) * 1000
	}
	if size < 0.5 || size > 2000 {
		return
	}
	adv := math.Hypot(dx, dy)
	if adv > 0.01 && (dx <= 0 || math.Abs(dy) > 0.1*adv) || adv <= 0.01 && vy > 0 {
		// Rotated, vertical or mirrored text.
		in.out.rotated++
		return
	}
	if x0 < -size || x0 > in.dispW+size || y0 < -size || y0 > in.dispH+size {
		return
	}
	if g.script {
		// A superscript or subscript digit glyph at the regular size: give
		// it the box it visibly occupies. Digits after a capital letter
		// are chemical subscripts (H2O, CO2); others are superscripts
		// (note marks, powers, affiliations).
		if in.cur.active && unicode.IsUpper(lastRuneOf(in.cur.lastText)) {
			y0 += 0.12 * size
		} else {
			y0 -= 0.36 * size
		}
		size *= 0.62
	}
	invisible := gs.render == 3 || gs.render == 7
	if invisible {
		in.out.invisibleChars++
	} else {
		in.out.visibleChars++
	}
	// A slanted y axis means synthetic italics.
	shear := vx/size < -0.12 || vx/size > 0.12
	pg := placedGlyph{
		text: g.text, x: x0, y: y0, adv: dx, size: size, font: ft,
		bold: ft.bold || gs.render == 2, italic: ft.italic || shear, mono: ft.mono,
		invisible: invisible,
	}
	pg.mcid, pg.artifact = in.markState()
	if sp := in.activeSpan(); sp != nil {
		if !sp.started {
			sp.started = true
			sp.first = pg
		}
		sp.endX = x0 + dx
		return
	}
	in.emit(pg)
}

// emit adds a glyph to the current run, starting a new run at word
// boundaries and style or baseline changes.
func (in *interp) emit(g placedGlyph) { in.emitGlyph(g, true) }

// emitGlyph implements emit. Spacing accents may be held back for one glyph
// (deferAccent) so that an accent printed over the following letter can be
// composed with it.
func (in *interp) emitGlyph(g placedGlyph, deferAccent bool) {
	if isSpaceText(g.text) {
		in.endRun()
		return
	}
	r := &in.cur
	if deferAccent && spacingAccent(g.text) {
		if r.active && overlaps(g, r.lastX0, r.lastX1) && math.Abs(g.size-r.size) < 0.3*r.size {
			if c := combineAccent(r.lastText, g.text); c != "" {
				r.replaceLast(c)
				return
			}
		}
		if in.accent == nil {
			ga := g
			in.accent = &ga
			return
		}
	}
	if a := in.accent; a != nil {
		in.accent = nil
		c := ""
		if overlaps(*a, g.x, g.x+g.adv) && math.Abs(a.size-g.size) < 0.3*g.size {
			c = combineAccent(g.text, a.text)
		}
		if c != "" {
			g.text = c
		} else {
			in.emitGlyph(*a, false)
		}
	}
	if r.active {
		gap := g.x - r.endX
		same := g.font == r.font && math.Abs(g.size-r.size) < 0.05*r.size &&
			math.Abs(g.y-r.baseY) < 0.15*r.size && g.bold == r.bold && g.italic == r.italic &&
			g.mcid == r.mcid && g.artifact == r.artifact
		// Letter-spaced text (tracking through TJ adjustments or Tc) widens
		// every gap; only a gap beyond the tracking separates words.
		track := in.trackEm
		if in.gs.fontSize > 0 && in.gs.charSpace > 0 {
			track += in.gs.charSpace / in.gs.fontSize
		}
		if !same || gap > (0.12+min(track, 1))*r.size || gap < -0.4*r.size {
			in.endRun()
		}
	}
	if !r.active {
		r.active = true
		r.sb.Reset()
		r.font, r.size, r.baseY = g.font, g.size, g.y
		r.x0, r.endX = g.x, g.x
		r.bold, r.italic, r.mono = g.bold, g.italic, g.mono
		r.mcid, r.artifact = g.mcid, g.artifact
	}
	r.sb.WriteString(g.text)
	r.lastText = g.text
	r.lastX0, r.lastX1 = g.x, g.x+g.adv
	r.endX = max(r.endX, g.x+g.adv)
}

func (r *runBuilder) replaceLast(text string) {
	s := r.sb.String()
	s = s[:len(s)-len(r.lastText)] + text
	r.sb.Reset()
	r.sb.WriteString(s)
	r.lastText = text
}

func lastRuneOf(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

func spacingAccent(s string) bool {
	switch s {
	case "¨", "´", "`", "ˆ", "˜", "¯", "˘", "˙", "˚", "¸", "˛", "ˇ", "˝", "ˊ", "ˋ", "ˉ", ",", "^", "~":
		return true
	}
	return false
}

// overlaps reports whether accent glyph a sits over the box [x0, x1].
func overlaps(a placedGlyph, x0, x1 float64) bool {
	if x1-x0 <= 0 {
		return false
	}
	cx := a.x + a.adv/2
	margin := 0.1 * (x1 - x0)
	return cx > x0+margin && cx < x1-margin
}

func (in *interp) endRun() {
	if a := in.accent; a != nil {
		in.accent = nil
		in.emitGlyph(*a, false)
	}
	r := &in.cur
	if !r.active {
		return
	}
	r.active = false
	text := r.sb.String()
	if strings.TrimSpace(text) == "" || !utf8.ValidString(text) {
		return
	}
	fontName := ""
	if r.font != nil {
		fontName = r.font.name
	}
	run := layout.Run{
		Text: text, X: r.x0, Y: r.baseY - 0.8*r.size, W: max(r.endX-r.x0, 0.1), H: r.size,
		FontSize: r.size, Bold: r.bold, Italic: r.italic, Mono: r.mono, FontName: fontName,
	}
	switch {
	case r.artifact:
		run.Role = layout.RoleArtifact
	case r.mcid >= 0:
		if h, ok := in.hints[r.mcid]; ok {
			run.Block, run.Role = h.block, h.role
		}
	}
	in.out.runs = append(in.out.runs, run)
}

func (in *interp) flush() { in.endRun() }

func (in *interp) xobject(res dict, n name, depth int) {
	v := in.f.resolveDict(res, "XObject")[n]
	s, _ := in.f.resolve(v).(*stream)
	if s == nil {
		return
	}
	switch s.d["Subtype"] {
	case name("Image"):
		in.placeImage(s)
	case name("Form"):
		r, _ := v.(ref)
		if depth >= maxFormDepth {
			return
		}
		for _, x := range in.forms {
			if r.num != 0 && x == r {
				return
			}
		}
		data, _, _, err := in.f.decodeStream(s, false)
		if err != nil {
			return
		}
		formRes, _ := in.f.resolve(s.d["Resources"]).(dict)
		if formRes == nil {
			formRes = res
		}
		saved, savedStack := in.gs, len(in.stack)
		savedTm, savedTlm, savedIn := in.tm, in.tlm, in.inText
		if m, ok := matrixFrom(in.f.resolveArray(s.d["Matrix"])); ok {
			in.gs.ctm = m.mul(in.gs.ctm)
		}
		in.forms = append(in.forms, r)
		in.run(data, formRes, depth+1)
		in.forms = in.forms[:len(in.forms)-1]
		in.gs = saved
		in.stack = in.stack[:min(savedStack, len(in.stack))]
		in.tm, in.tlm, in.inText = savedTm, savedTlm, savedIn
	}
}

func (in *interp) placeImage(s *stream) {
	if mask, _ := s.d["ImageMask"].(bool); mask {
		return
	}
	var xs, ys []float64
	for _, c := range [4][2]float64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		ux, uy := in.gs.ctm.apply(c[0], c[1])
		dx, dy := in.toDisplay(ux, uy)
		xs = append(xs, dx)
		ys = append(ys, dy)
	}
	x0, x1 := minMax(xs)
	y0, y1 := minMax(ys)
	x0, y0 = max(x0, 0), max(y0, 0)
	x1, y1 = min(x1, in.dispW), min(y1, in.dispH)
	if x1-x0 < minImageSide || y1-y0 < minImageSide {
		return
	}
	if len(in.out.images) >= 200 {
		return
	}
	in.out.images = append(in.out.images, imagePlacement{obj: s, x: x0, y: y0, w: x1 - x0, h: y1 - y0})
}

func minMax(v []float64) (float64, float64) {
	lo, hi := v[0], v[0]
	for _, x := range v[1:] {
		lo, hi = min(lo, x), max(hi, x)
	}
	return lo, hi
}

// skipInlineImage moves the lexer past "BI … ID <data> EI".
func (in *interp) skipInlineImage(lx *lexer) {
	for {
		tok, ok := lx.next()
		if !ok {
			return
		}
		if tok == keyword("ID") {
			break
		}
	}
	b := lx.b
	i := lx.pos + 1
	for i+2 <= len(b) {
		j := indexFrom(b, i, "EI")
		if j < 0 {
			lx.pos = len(b)
			return
		}
		before := j == 0 || isSpace(b[j-1])
		after := j+2 >= len(b) || isSpace(b[j+2]) || isDelim(b[j+2])
		if before && after {
			lx.pos = j + 2
			return
		}
		i = j + 2
	}
	lx.pos = len(b)
}
