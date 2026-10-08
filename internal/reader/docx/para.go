package docx

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/mathconv/omml"
)

// paraProps are the resolved properties of one paragraph.
type paraProps struct {
	styleID         string
	kind            styleKind
	heading         int
	list            *listRef
	indent          int
	jc              string
	pageBreakBefore bool
	sectionBreak    bool
	bottomBorder    bool
	dropCap         bool
}

func (r *reader) paraProps(p *node) *paraProps {
	ppr := p.child("pPr")
	styleID := ppr.val("pStyle")
	if styleID == "" {
		styleID = r.styles.defaultPara
	}
	sp := r.styles.paraProps(styleID)
	dp := parseParaProps(ppr)
	props := &paraProps{styleID: styleID, kind: r.styles.kind(styleID)}
	if props.kind == kindNone && r.styles.byID[styleID] == nil {
		// A style that is referenced but never defined still tells us something.
		props.kind = kindFromName(strings.ToLower(styleID))
	}

	switch {
	case dp.outline >= 0:
		if dp.outline <= 8 {
			props.heading = dp.outline + 1
		}
	case r.styles.byID[styleID] == nil:
		props.heading = headingFromName(strings.ToLower(styleID))
	default:
		props.heading = r.styles.headingLevel(styleID)
	}
	if props.kind == kindTitle || props.kind == kindSubtitle || props.kind == kindTOC {
		props.heading = 0
	}

	props.jc = sp.jc
	if dp.jc != "" {
		props.jc = dp.jc
	}
	props.indent = sp.indLeft
	if dp.hasInd {
		props.indent = dp.indLeft
	}
	props.pageBreakBefore = dp.pageBreakBefore.or(sp.pageBreakBefore) == on
	if sect := ppr.child("sectPr"); sect != nil {
		switch sect.val("type") {
		case "", "nextPage", "oddPage", "evenPage":
			props.sectionBreak = true
		}
	}
	switch ppr.child("framePr").attr("dropCap") {
	case "drop", "margin":
		props.dropCap = true
	}
	if b := ppr.child("pBdr").child("bottom"); b != nil {
		v := b.attr("val")
		props.bottomBorder = v != "" && v != "none" && v != "nil"
	}

	numID, ilvl, hasIlvl := sp.numID, sp.ilvl, sp.hasIlvl
	if dp.hasNum {
		numID = dp.numID
	}
	if dp.hasIlvl {
		ilvl, hasIlvl = dp.ilvl, true
	}
	if _, known := r.num.nums[numID]; known && numID != "0" {
		if !hasIlvl {
			if l, ok := r.num.levelForStyle(numID, styleID); ok {
				ilvl = l
			}
		}
		lvl := r.num.level(numID, ilvl)
		if ordered, style, ok := listFormat(lvl); ok {
			// Numbered headings (outline numbering) stay headings, but they
			// still advance the counters so deeper levels restart under them.
			number := r.num.next(numID, ilvl)
			if props.heading == 0 && props.kind != kindTitle && props.kind != kindSubtitle &&
				props.kind != kindTOC && props.kind != kindCaption {
				lr := &listRef{numID: numID, ilvl: ilvl, ordered: ordered, style: style, number: number}
				if lvl != nil && !ordered {
					lr.task = taskFromMarker(lvl.text)
				}
				props.list = lr
			}
		}
	}
	return props
}

func taskFromMarker(s string) ast.TaskState {
	switch strings.TrimSpace(s) {
	case "☐":
		return ast.TaskOpen
	case "☑", "☒":
		return ast.TaskDone
	}
	return ast.TaskNone
}

// paragraph converts a w:p into items (usually one).
func (r *reader) paragraph(s *story, p *node, bc blockCtx) []item {
	props := r.paraProps(p)
	pb := &paraBuilder{r: r, s: s, bc: bc, props: props}
	pb.bookmarks, s.bookmarks = s.bookmarks, nil
	pb.children(p.kids)
	items := pb.finish()
	if len(pb.bookmarks) > 0 {
		s.bookmarks = append(s.bookmarks, pb.bookmarks...)
	}
	return items
}

// paraBuilder collects the content of one paragraph.
type paraBuilder struct {
	r     *reader
	s     *story
	bc    blockCtx
	props *paraProps

	root      []seg
	after     []item
	bookmarks []string
	seqIDs    []string
	task      ast.TaskState

	textRunes, boldRunes, monoRunes int
	maxSize                         int
}

// ---------------------------------------------------------------------------
// Fields and links share one frame stack per story: a complex field can
// start in one paragraph and end in another.

type frameKind int

const (
	frField frameKind = iota
	frLink
)

type fieldMode int

const (
	modeKeep fieldMode = iota
	modeDrop
	modeLink
	modeCite
)

type frame struct {
	kind    frameKind
	inInstr bool
	instr   strings.Builder
	mode    fieldMode
	url     string // external target
	anchor  string // internal target (bookmark name)
	cite    []ast.CiteItem
	ffData  *node
	segs    []seg
}

func (f *frame) collecting() bool {
	return f.kind == frLink || f.mode == modeLink || f.mode == modeCite
}

func (f *frame) suppressing() bool {
	return f.kind == frField && (f.inInstr || f.mode == modeDrop)
}

// emit routes a segment to the innermost collecting frame, or the
// paragraph, unless a field instruction or a dropped field result is open.
func (pb *paraBuilder) emit(sg seg) bool {
	fr := pb.s.frames
	for i := len(fr) - 1; i >= 0; i-- {
		f := fr[i]
		if f.suppressing() {
			return false
		}
		if f.collecting() {
			f.segs = append(f.segs, sg)
			return true
		}
	}
	pb.root = append(pb.root, sg)
	return true
}

// emitMark places a block-level marker that splits the paragraph.
func (pb *paraBuilder) emitMark(b ast.Block) {
	for _, f := range pb.s.frames {
		if f.suppressing() {
			return
		}
	}
	pb.root = append(pb.root, seg{mark: b})
}

func (pb *paraBuilder) suppressed() bool {
	for _, f := range pb.s.frames {
		if f.suppressing() {
			return true
		}
	}
	return false
}

func (pb *paraBuilder) atStart() bool {
	for _, s := range pb.root {
		if !isSpaceText(s) {
			return false
		}
	}
	for _, f := range pb.s.frames {
		if len(f.segs) > 0 {
			return false
		}
	}
	return true
}

func (pb *paraBuilder) topField() *frame {
	for i := len(pb.s.frames) - 1; i >= 0; i-- {
		if pb.s.frames[i].kind == frField {
			return pb.s.frames[i]
		}
	}
	return nil
}

func (pb *paraBuilder) removeFrame(f *frame) {
	fr := pb.s.frames
	for i := len(fr) - 1; i >= 0; i-- {
		if fr[i] == f {
			pb.s.frames = append(fr[:i:i], fr[i+1:]...)
			return
		}
	}
}

// closeFrame removes f and emits what it collected.
func (pb *paraBuilder) closeFrame(f *frame) {
	pb.removeFrame(f)
	for _, sg := range pb.frameOutput(f, f.segs) {
		pb.emit(sg)
	}
	f.segs = nil
}

// frameOutput wraps collected segments in a link or citation.
func (pb *paraBuilder) frameOutput(f *frame, segs []seg) []seg {
	if len(segs) == 0 {
		return nil
	}
	switch {
	case f.kind == frLink || f.mode == modeLink:
		if f.url == "" && f.anchor == "" {
			return segs
		}
		lead, inner, trail := peelSpace(segs)
		clearFlags(inner, fUnder)
		common := commonFlags(inner)
		clearFlags(inner, common)
		ins := buildInlines(inner, false)
		if len(ins) == 0 {
			return segs
		}
		var link *ast.Link
		if f.url != "" {
			link = &ast.Link{URL: f.url, Inlines: ins}
		} else {
			link = pb.r.internalLink(f.anchor, ins)
		}
		return wrapSeg(lead, seg{f: common, node: link}, trail)
	case f.mode == modeCite:
		if len(f.cite) == 0 {
			return segs
		}
		lead, inner, trail := peelSpace(segs)
		ins := buildInlines(inner, false)
		return wrapSeg(lead, seg{node: &ast.Cite{Items: f.cite, Fallback: ins}}, trail)
	}
	return segs
}

func wrapSeg(lead string, s seg, trail string) []seg {
	out := make([]seg, 0, 3)
	if lead != "" {
		out = append(out, seg{text: lead})
	}
	out = append(out, s)
	if trail != "" {
		out = append(out, seg{text: trail})
	}
	return out
}

// flushFrames moves content collected by fields still open at the end of
// the paragraph into the paragraph, so nothing leaks into the next one.
func (pb *paraBuilder) flushFrames() {
	fr := pb.s.frames
	for i := len(fr) - 1; i >= 0; i-- {
		f := fr[i]
		if !f.collecting() || len(f.segs) == 0 {
			continue
		}
		out := pb.frameOutput(f, f.segs)
		f.segs = nil
		for j := i - 1; j >= 0 && out != nil; j-- {
			if fr[j].suppressing() {
				out = nil
			} else if fr[j].collecting() {
				fr[j].segs = append(fr[j].segs, out...)
				out = nil
			}
		}
		pb.root = append(pb.root, out...)
	}
	// Hyperlink elements cannot outlive their paragraph.
	kept := fr[:0]
	for _, f := range fr {
		if f.kind == frField {
			kept = append(kept, f)
		}
	}
	pb.s.frames = kept
}

// ---------------------------------------------------------------------------
// Paragraph content

func (pb *paraBuilder) children(kids []*node) {
	for _, n := range kids {
		pb.inline(n)
	}
}

func (pb *paraBuilder) inline(n *node) {
	switch n.ns {
	case "w":
		switch n.name {
		case "r":
			pb.run(n)
		case "hyperlink":
			pb.hyperlink(n)
		case "fldSimple":
			pb.fldSimple(n)
		case "sdt":
			pb.sdt(n)
		case "bookmarkStart":
			if name := n.attr("name"); name != "" {
				pb.bookmarks = append(pb.bookmarks, name)
			}
		case "del", "moveFrom", "commentReference", "subDoc":
			if n.name == "subDoc" {
				pb.r.warn.Addf("linked subdocument was dropped")
			}
		default:
			if !strings.HasSuffix(n.name, "Pr") {
				pb.children(n.kids)
			}
		}
	case "m":
		switch n.name {
		case "oMathPara":
			if tex, _ := omml.ToLaTeX(toOMML(n, 0)); strings.TrimSpace(tex) != "" {
				pb.emitMark(&ast.MathBlock{TeX: tex})
			}
		case "oMath":
			if tex, _ := omml.ToLaTeX(toOMML(n, 0)); strings.TrimSpace(tex) != "" {
				pb.emit(seg{node: &ast.Math{TeX: tex}})
			}
		}
	case "mc":
		if n.name == "AlternateContent" {
			if alt := chooseAlternate(n); alt != nil {
				pb.children(alt.kids)
			}
		}
	default:
		if !strings.HasSuffix(n.name, "Pr") {
			pb.children(n.kids)
		}
	}
}

// toOMML adapts a DOM subtree for the math converter.
func toOMML(n *node, depth int) *omml.Node {
	o := &omml.Node{Space: n.ns, Local: n.name, Attr: n.attrs, Text: n.text}
	if depth < maxDepth {
		o.Children = make([]*omml.Node, 0, len(n.kids))
		for _, k := range n.kids {
			o.Children = append(o.Children, toOMML(k, depth+1))
		}
	}
	return o
}

// runFmt is the resolved formatting of a run.
type runFmt struct {
	flags  fmtFlags
	sym    string // symbol font table
	lang   string
	bold   bool // effective bold, paragraph style included
	mono   bool // effective font is monospace
	size   int
	hidden bool
}

func (r *reader) runFormat(paraStyle string, rpr *node) runFmt {
	direct := parseRunProps(rpr)
	var char runProps
	code := false
	if direct.rStyle != "" {
		var ignore bool
		code, ignore = r.styles.charStyleClass(direct.rStyle)
		if !ignore {
			char = r.styles.styleRun(direct.rStyle)
		}
	}
	own := char.over(direct)
	eff := r.styles.paraRun(paraStyle).over(own)
	var f fmtFlags
	set := func(t tri, flag fmtFlags) {
		if t == on {
			f |= flag
		}
	}
	set(own.bold, fBold)
	set(own.italic, fItalic)
	set(own.underline, fUnder)
	set(own.strike, fStrike)
	set(own.smallCaps, fSmall)
	set(own.highlight, fHigh)
	set(own.sup, fSup)
	if own.sup != on {
		set(own.sub, fSub)
	}
	// Some producers raise or lower smaller text instead of using
	// vertAlign; treat that as super/subscript.
	if own.sup == unset && own.sub == unset && own.position != 0 {
		base := r.styles.paraRun(paraStyle).size
		if base == 0 {
			base = 20
		}
		if eff.size != 0 && eff.size < base {
			if own.position > 0 {
				f |= fSup
			} else {
				f |= fSub
			}
		}
	}
	if code || (!r.styles.monoBody && isMonoFont(own.font)) {
		f |= fCode
	}
	size := eff.size
	if size == 0 {
		size = 20
	}
	return runFmt{
		flags:  f,
		sym:    symbolFont(eff.font),
		lang:   eff.lang,
		bold:   eff.bold == on,
		mono:   !r.styles.monoBody && isMonoFont(eff.font),
		size:   size,
		hidden: eff.vanish == on,
	}
}

func (pb *paraBuilder) run(n *node) {
	rf := pb.r.runFormat(pb.props.styleID, n.child("rPr"))
	pb.runContent(n.kids, rf)
}

func (pb *paraBuilder) runContent(kids []*node, rf runFmt) {
	for _, k := range kids {
		if k.name == "AlternateContent" {
			if alt := chooseAlternate(k); alt != nil {
				pb.runContent(alt.kids, rf)
			}
			continue
		}
		if k.ns != "w" {
			continue
		}
		switch k.name {
		case "fldChar":
			pb.fldChar(k)
			continue
		case "instrText":
			if f := pb.topField(); f != nil && f.inInstr {
				f.instr.WriteString(k.text)
			}
			continue
		}
		if rf.hidden {
			continue
		}
		switch k.name {
		case "t":
			pb.text(k.text, rf)
		case "tab", "ptab":
			pb.text("\t", rf)
		case "br":
			pb.br(k)
		case "cr":
			pb.emit(seg{node: &ast.LineBreak{}})
		case "noBreakHyphen":
			pb.text("\u2011", rf)
		case "sym":
			pb.sym(k, rf)
		case "drawing":
			pb.drawing(k)
		case "pict":
			pb.pict(k)
		case "object":
			pb.object(k)
		case "footnoteReference":
			pb.noteRef("footnote", k.attr("id"))
		case "endnoteReference":
			pb.noteRef("endnote", k.attr("id"))
		case "ruby":
			if base := k.child("rubyBase"); base != nil {
				pb.children(base.kids)
			}
		case "contentPart":
			pb.r.warn.Addf("ink annotation was dropped")
		}
	}
}

// text emits run text, mapping legacy symbol-font characters.
func (pb *paraBuilder) text(s string, rf runFmt) {
	if s == "" {
		return
	}
	if rf.sym != "" {
		s = pb.mapSymbolText(s, rf.sym)
		rf.sym = ""
	}
	if !pb.emit(seg{f: rf.flags, text: s}) {
		return
	}
	visible := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			visible++
		}
	}
	if visible == 0 {
		return
	}
	pb.textRunes += visible
	if rf.bold {
		pb.boldRunes += visible
	}
	if rf.mono {
		pb.monoRunes += visible
	}
	if rf.size > pb.maxSize {
		pb.maxSize = rf.size
	}
	if pb.s.body {
		pb.r.countLang(rf.lang, s)
	}
}

func (pb *paraBuilder) br(k *node) {
	switch k.attr("type") {
	case "page":
		if pb.bc.top {
			pb.emitMark(&ast.PageBreak{})
		}
	case "column":
		pb.emit(seg{text: " "})
	default:
		pb.emit(seg{node: &ast.LineBreak{}})
	}
}

func (pb *paraBuilder) sym(k *node, rf runFmt) {
	code, err := strconv.ParseUint(strings.TrimSpace(k.attr("char")), 16, 32)
	if err != nil {
		return
	}
	c := rune(code)
	if c >= 0xF000 && c <= 0xF0FF {
		c -= 0xF000
	}
	font := k.attr("font")
	table := symbolFont(font)
	rf.sym = ""
	if table == "" {
		if c >= 0x20 && !(c >= 0xE000 && c <= 0xF8FF) && utf8.ValidRune(c) {
			pb.text(string(c), rf)
		}
		return
	}
	if t, ok := symbolChar(table, c); ok {
		pb.text(t, rf)
		return
	}
	pb.r.warn.Addf("symbol character %04X of font %q has no Unicode equivalent and was dropped", code, font)
}

func (pb *paraBuilder) mapSymbolText(s, table string) string {
	var b strings.Builder
	for _, c := range s {
		code := c
		if code >= 0xF000 && code <= 0xF0FF {
			code -= 0xF000
		}
		if code > 0xFF {
			b.WriteRune(c)
			continue
		}
		if t, ok := symbolChar(table, code); ok {
			b.WriteString(t)
			continue
		}
		if unicode.IsSpace(c) {
			b.WriteRune(c)
			continue
		}
		pb.r.warn.Addf("symbol character %04X of a %s font has no Unicode equivalent and was dropped", int(code), table)
	}
	return b.String()
}

func (pb *paraBuilder) noteRef(kind, id string) {
	if pb.suppressed() {
		return
	}
	if note := pb.r.note(kind, id); note != nil {
		pb.emit(seg{node: note})
	}
}

func (pb *paraBuilder) hyperlink(n *node) {
	f := &frame{kind: frLink}
	if rid := n.attrNS("r", "id"); rid != "" {
		if rl, ok := pb.s.part.rels[rid]; ok && rl.external {
			f.url = safeURL(rl.target)
		}
	}
	if anchor := strings.TrimSpace(n.attr("anchor")); anchor != "" {
		if f.url != "" {
			if !strings.Contains(f.url, "#") {
				f.url += "#" + anchor
			}
		} else if n.attrNS("r", "id") == "" {
			f.anchor = anchor
		}
	}
	pb.s.frames = append(pb.s.frames, f)
	pb.children(n.kids)
	pb.closeFrame(f)
}

func (pb *paraBuilder) fldSimple(n *node) {
	f := &frame{kind: frField, inInstr: true}
	f.instr.WriteString(n.attr("instr"))
	pb.s.frames = append(pb.s.frames, f)
	pb.resolveField(f)
	pb.children(n.kids)
	pb.closeFrame(f)
}

func (pb *paraBuilder) fldChar(k *node) {
	switch k.attr("fldCharType") {
	case "begin":
		pb.s.frames = append(pb.s.frames, &frame{kind: frField, inInstr: true, ffData: k.child("ffData")})
	case "separate":
		if f := pb.topField(); f != nil && f.inInstr {
			pb.resolveField(f)
		}
	case "end":
		f := pb.topField()
		if f == nil {
			return
		}
		if f.inInstr {
			pb.resolveField(f)
		}
		pb.closeFrame(f)
	}
}

// sdt unwraps an inline content control.
func (pb *paraBuilder) sdt(n *node) {
	pr := n.child("sdtPr")
	if cb := pr.child("checkbox"); cb != nil {
		done := onOff(cb.child("checked"))
		if pb.atStart() && pb.props.list != nil && pb.task == ast.TaskNone {
			pb.task = ast.TaskOpen
			if done {
				pb.task = ast.TaskDone
			}
			return
		}
		if done {
			pb.emit(seg{text: "☒"})
		} else {
			pb.emit(seg{text: "☐"})
		}
		return
	}
	if content := pb.r.sdtContent(n); content != nil {
		pb.children(content.kids)
	}
}

// sdtContent returns the content of a content control, or nil when it only
// shows placeholder text or wraps a generated table of contents.
func (r *reader) sdtContent(n *node) *node {
	pr := n.child("sdtPr")
	if pr != nil {
		if onOff(pr.child("showingPlcHdr")) {
			return nil
		}
		if g := pr.find("docPartGallery"); g != nil {
			if strings.Contains(strings.ToLower(g.attr("val")), "table of contents") {
				return nil
			}
		}
	}
	return n.child("sdtContent")
}

// ---------------------------------------------------------------------------
// Finishing

func (pb *paraBuilder) finish() []item {
	pb.flushFrames()
	props := pb.props
	var items []item
	if props.pageBreakBefore && pb.bc.top {
		items = append(items, item{kind: kBlock, block: &ast.PageBreak{}})
	}
	if props.kind == kindTOC {
		pb.root = nil
	}

	start := 0
	produced := false
	first := -1
	for i := 0; i <= len(pb.root); i++ {
		if i < len(pb.root) && pb.root[i].mark == nil {
			continue
		}
		if it, ok := pb.makeItem(pb.root[start:i]); ok {
			if first < 0 {
				first = len(items)
				it.bookmarks = pb.bookmarks
				pb.bookmarks = nil
			}
			produced = true
			items = append(items, it)
		}
		if i < len(pb.root) {
			switch m := pb.root[i].mark.(type) {
			case *ast.PageBreak:
				if pb.bc.top {
					items = append(items, item{kind: kBlock, block: m})
				}
			default:
				items = append(items, item{kind: kBlock, block: m})
				produced = true
			}
		}
		start = i + 1
	}

	if props.list != nil && first >= 0 {
		for k := first + 1; k < len(items); k++ {
			items[k].cont = true
		}
	}
	if !produced && len(pb.after) == 0 {
		switch {
		case props.kind == kindCode:
			items = append(items, item{kind: kPara, p: props, code: true})
		case props.bottomBorder && pb.bc.top:
			items = append(items, item{kind: kBlock, block: &ast.HorizontalRule{}})
		}
	}
	items = append(items, pb.after...)
	if props.sectionBreak && pb.bc.top {
		items = append(items, item{kind: kBlock, block: &ast.PageBreak{}})
	}
	return items
}

// makeItem converts one stretch of paragraph content.
func (pb *paraBuilder) makeItem(segs []seg) (item, bool) {
	props := pb.props
	if len(segs) == 0 {
		return item{}, false
	}
	if props.kind == kindCode || pb.allMono() {
		var sb strings.Builder
		for _, s := range segs {
			sb.WriteString(segPlain(s))
		}
		text := strings.TrimRight(strings.ReplaceAll(sb.String(), "\r", ""), " \t")
		if strings.TrimSpace(text) == "" && props.kind != kindCode {
			return item{}, false
		}
		return item{kind: kPara, p: props, code: true, codeText: text}, true
	}
	switch {
	case props.heading > 0 && pb.bc.top, props.kind == kindTitle, props.kind == kindSubtitle, props.kind == kindCaption:
		segs = append([]seg(nil), segs...)
		clearFlags(segs, commonFlags(segs))
	}
	ins := buildInlines(segs, true)
	if len(ins) == 0 {
		return item{}, false
	}
	it := item{kind: kPara, ins: ins, p: props, task: pb.task, allBold: pb.textRunes > 0 && pb.boldRunes == pb.textRunes,
		size: pb.maxSize, plainLen: pb.textRunes}

	if fig := pb.figureWithCaption(ins); fig != nil {
		return item{kind: kBlock, block: fig, figure: fig}, true
	}
	if props.list == nil && len(ins) == 1 {
		switch n := ins[0].(type) {
		case *ast.Image:
			if !pb.bc.inCell {
				fig := &ast.Figure{Image: n}
				return item{kind: kBlock, block: fig, figure: fig}, true
			}
		case *ast.Math:
			return item{kind: kBlock, block: &ast.MathBlock{TeX: n.TeX}}, true
		}
	}
	switch {
	case props.kind == kindTitle && pb.bc.top:
		it.kind = kTitle
	case props.kind == kindSubtitle && pb.bc.top:
		it.kind = kSubtitle
	case props.heading > 0 && pb.bc.top:
		it.kind, it.level = kHeading, props.heading
	}
	it.capKind = pb.captionKind(ins)
	return it, true
}

// figureWithCaption recognises an image followed, in the same paragraph,
// by its caption ("Figure 1: ..."), a layout some authors use instead of a
// separate caption paragraph.
func (pb *paraBuilder) figureWithCaption(ins []ast.Inline) *ast.Figure {
	if pb.bc.inCell || pb.props.list != nil || len(ins) < 2 {
		return nil
	}
	img, ok := ins[0].(*ast.Image)
	if !ok {
		return nil
	}
	rest := ins[1:]
	for len(rest) > 0 {
		if _, br := rest[0].(*ast.LineBreak); br {
			rest = rest[1:]
			continue
		}
		if t, ok := rest[0].(*ast.Text); ok && strings.TrimSpace(t.Value) == "" {
			rest = rest[1:]
			continue
		}
		break
	}
	lead, _ := leadingText(rest)
	kind, n := captionLabel(lead)
	if n == 0 || kind != capFigure && pb.props.kind != kindCaption {
		return nil
	}
	caption := stripCaption(rest)
	if len(caption) == 0 {
		caption = nil
	}
	return &ast.Figure{Image: img, Caption: caption}
}

// allMono reports whether every visible character of the paragraph is set
// in a monospace font (code typed with direct formatting).
func (pb *paraBuilder) allMono() bool {
	p := pb.props
	return !pb.bc.inCell && pb.textRunes > 0 && pb.monoRunes == pb.textRunes && p.heading == 0 &&
		p.list == nil && p.kind == kindNone
}

func (pb *paraBuilder) captionKind(ins []ast.Inline) capKind {
	if pb.props.kind != kindCaption {
		lead, _ := leadingText(ins)
		if k, n := captionLabel(lead); n > 0 && k != capOther {
			return k | capUnstyled
		}
		return capNone
	}
	for _, id := range pb.seqIDs {
		if k := labelKind(id); k != capOther {
			return k
		}
	}
	lead, _ := leadingText(ins)
	if k, n := captionLabel(lead); n > 0 {
		return k
	}
	return capOther
}
