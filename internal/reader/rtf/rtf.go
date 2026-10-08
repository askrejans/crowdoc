// Package rtf reads Rich Text Format documents, as written by word
// processors and text editors, into the crowdoc AST.
//
// The reader is a tokenizer plus a group state machine: every "{" pushes a
// copy of the character, paragraph and destination state and every "}"
// restores it. Text is routed by the current destination (body text, font
// table, field instruction, picture data, ...). Paragraphs are collected as
// formatted runs and handed to a flow builder that assembles lists, code
// blocks, quotes, tables and figures.
package rtf

import (
	"bytes"
	"context"
	"errors"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// maxDepth bounds the group stack; deeper groups in hostile input are
// tracked by a counter only.
const maxDepth = 2048

// Read parses an RTF document.
func Read(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	head := data[:min(len(data), 4096)]
	start := bytes.Index(head, []byte(`{\rtf`))
	if start < 0 {
		return nil, nil, errors.New("not an RTF document: missing {\\rtf header")
	}
	p := newParser(ctx, data[start:], o)
	if err := p.run(); err != nil {
		return nil, nil, err
	}
	return p.doc, p.warn.List(), nil
}

type fontInfo struct {
	name    string
	charset int
	cp      int
	family  string
	pitch   int
	mono    bool
}

type styleInfo struct {
	name     string
	lang     int
	outline  int // -1 = none
	basedOn  int
	resolved bool
	kind     styleKind
	level    int
}

type styleKind uint8

const (
	skNone styleKind = iota
	skHeading
	skTitle
	skSubtitle
	skQuote
	skCode
	skCaption
	skTOC
)

type rgb struct {
	r, g, b int
	auto    bool
}

type listLevelDef struct {
	nfc     int
	start   int
	hasNfc  bool
	hasNfcN bool
}

type listDef struct {
	id     int
	levels []listLevelDef
}

type linkInfo struct {
	url, title string
}

type parser struct {
	ctx  context.Context
	lx   lexer
	doc  *ast.Document
	warn rd.Warnings
	opts rd.Options

	stack    []state
	overflow int
	tokens   int

	docCP       int
	literalUTF8 bool
	dec         decoder
	pending     []byte
	ucSkip      int
	highSurr    rune

	fonts     map[int]*fontInfo
	fontName  []byte
	colors    []rgb
	colorCur  rgb
	colorSet  bool
	styles    map[int]*styleInfo
	cstyles   map[int]*styleInfo
	curStyle  *styleInfo
	styleName []byte
	lists     map[int]*listDef
	overrides map[int]int
	defFont   int
	defLang   int

	ctxs []*rtfCtx

	links        []linkInfo
	listCounters map[string]*[9]int
	images       int
	langChars    map[int]int
	sizeChars    map[int]int
	styleHeads   bool
	targets      map[string]bool
	info         map[string]string
	infoDate     [3]int

	rowDefs     map[int]*rowDef
	cellPending map[int]cellDef

	titleFromBody bool
	imageSrcs     map[[32]byte]string
}

func newParser(ctx context.Context, data []byte, o rd.Options) *parser {
	p := &parser{
		ctx:          ctx,
		lx:           lexer{src: string(data)},
		doc:          &ast.Document{Resources: ast.NewResources()},
		opts:         o,
		docCP:        1252,
		fonts:        map[int]*fontInfo{},
		styles:       map[int]*styleInfo{},
		cstyles:      map[int]*styleInfo{},
		lists:        map[int]*listDef{},
		overrides:    map[int]int{},
		listCounters: map[string]*[9]int{},
		langChars:    map[int]int{},
		sizeChars:    map[int]int{},
		targets:      map[string]bool{},
		info:         map[string]string{},
		rowDefs:      map[int]*rowDef{},
		cellPending:  map[int]cellDef{},
		defFont:      -1,
	}
	p.literalUTF8 = hasHighBytes(data) && utf8.Valid(data)
	p.stack = append(p.stack, state{dest: dNormal, uc: 1, chr: defaultChar(-1), para: defaultPara()})
	p.ctxs = append(p.ctxs, &rtfCtx{flow: newFlow(p, true)})
	return p
}

func hasHighBytes(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return true
		}
	}
	return false
}

func (p *parser) st() *state   { return &p.stack[len(p.stack)-1] }
func (p *parser) cur() *rtfCtx { return p.ctxs[len(p.ctxs)-1] }

func (p *parser) run() error {
	for {
		tok := p.lx.next()
		p.tokens++
		if p.tokens&4095 == 0 {
			if err := p.ctx.Err(); err != nil {
				return err
			}
		}
		switch tok.kind {
		case tEOF:
			p.flushBytes()
			p.finish()
			return nil
		case tGroupStart:
			p.flushBytes()
			p.ucSkip = 0
			p.push()
		case tGroupEnd:
			p.flushBytes()
			p.ucSkip = 0
			p.pop()
		case tHex:
			if p.ucSkip > 0 {
				p.ucSkip--
				continue
			}
			p.addByte(tok.ch)
		case tText:
			p.text(tok.text)
		case tBin:
			p.flushBytes()
			if p.ucSkip > 0 {
				p.ucSkip--
				continue
			}
			if s := p.st(); s.dest == dPict && s.pict != nil {
				s.pict.data = append(s.pict.data, tok.text...)
			}
		case tSymbol:
			p.flushBytes()
			if p.ucSkip > 0 {
				p.ucSkip--
				continue
			}
			p.symbol(tok.ch)
		case tWord:
			p.flushBytes()
			if p.ucSkip > 0 && tok.name != "u" {
				p.ucSkip--
				continue
			}
			p.ucSkip = 0
			p.word(tok)
		}
	}
}

// push opens a group: the new state inherits everything except the
// per-group destination markers.
func (p *parser) push() {
	if len(p.stack) >= maxDepth {
		p.overflow++
		return
	}
	parent := p.st()
	if parent.sheetRoot {
		// Each child of \stylesheet defines one style; unnumbered ones are
		// style 0 ("Normal").
		p.curStyle = p.style(p.styles, 0)
	}
	s := *parent
	s.sheetRoot = false
	s.starred = false
	s.fresh = true
	s.owner = ownNone
	s.uprChild = s.dest == dUpr
	if s.dest == dUpr {
		s.dest = dSkipText
	}
	p.stack = append(p.stack, s)
}

func (p *parser) pop() {
	if p.overflow > 0 {
		p.overflow--
		return
	}
	if len(p.stack) <= 1 {
		return // unbalanced closing brace
	}
	closed := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	p.endGroup(&closed)
}

// skip discards the rest of the current group.
func (p *parser) skip() {
	p.lx.skipGroup()
	if p.overflow > 0 {
		p.overflow--
		return
	}
	if len(p.stack) > 1 {
		p.stack = p.stack[:len(p.stack)-1]
	}
}

func (p *parser) finish() {
	// Close groups left open by truncated input so pending footnotes,
	// fields and pictures are finalised.
	for len(p.stack) > 1 {
		p.pop()
	}
	for len(p.ctxs) > 1 {
		p.popCtx()
	}
	c := p.cur()
	if !c.para.empty() {
		p.endParagraph()
	}
	p.syncTables(c, 0)
	p.doc.Blocks = c.flow.finish()
	p.applyInfo()
	p.doc.Blocks = pruneAnchors(p.doc.Blocks, p.targets)
}

// ---------------------------------------------------------------------------
// Text handling
// ---------------------------------------------------------------------------

func (p *parser) addByte(b byte) {
	if p.st().dest == dPict {
		return
	}
	p.pending = append(p.pending, b)
}

func (p *parser) flushBytes() {
	if len(p.pending) == 0 {
		return
	}
	s := p.dec.decode(p.pending, p.codePage())
	p.pending = p.pending[:0]
	p.emit(s)
}

// codePage returns the code page for \'hh bytes in the current font.
func (p *parser) codePage() int {
	if f := p.fonts[p.st().chr.font]; f != nil && f.cp != 0 {
		return f.cp
	}
	return p.docCP
}

func (p *parser) text(s string) {
	if p.ucSkip > 0 {
		n := min(p.ucSkip, len(s))
		p.ucSkip -= n
		s = s[n:]
	}
	if s == "" {
		return
	}
	st := p.st()
	switch st.dest {
	case dPict:
		if st.pict != nil {
			st.pict.addHex(s)
		}
		return
	case dSkipText:
		return
	}
	if st.dest == dNormal {
		if cp := p.codePage(); cp == cpSymbol || cp == cpWingdings {
			// Symbol fonts encode glyphs, not letters: "a" is alpha.
			p.pending = append(p.pending, s...)
			return
		}
	}
	if p.literalUTF8 {
		p.flushBytes()
		p.emit(s)
		return
	}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			if start < i {
				p.flushBytes()
				p.emit(s[start:i])
			}
			p.pending = append(p.pending, s[i])
			start = i + 1
		}
	}
	if start < len(s) {
		p.flushBytes()
		p.emit(s[start:])
	}
}

func (p *parser) unicode(n int) {
	if n < 0 {
		n += 65536
	}
	p.ucSkip = p.st().uc
	r := rune(n)
	if utf16.IsSurrogate(r) {
		if r < 0xDC00 {
			p.highSurr = r
			return
		}
		if p.highSurr == 0 {
			return
		}
		r = utf16.DecodeRune(p.highSurr, r)
	}
	p.highSurr = 0
	if r >= 0xF020 && r <= 0xF0FF {
		switch p.codePage() {
		case cpSymbol:
			r = symbolRune(byte(r - 0xF000))
		case cpWingdings:
			r = wingdingsRune(byte(r - 0xF000))
		}
		if r == 0 {
			return
		}
	}
	p.emit(string(r))
}

// emit routes decoded text to the current destination.
func (p *parser) emit(s string) {
	st := p.st()
	switch st.dest {
	case dFontTable:
		p.fontText(s)
	case dColorTable:
		for i := 0; i < len(s); i++ {
			if s[i] == ';' {
				p.colors = append(p.colors, rgb{p.colorCur.r, p.colorCur.g, p.colorCur.b, !p.colorSet})
				p.colorCur, p.colorSet = rgb{}, false
			}
		}
	case dStyleSheet:
		p.styleText(s)
	case dCollect:
		if st.collect != nil {
			*st.collect = append(*st.collect, s...)
		}
	case dFldInst:
		if st.field != nil {
			st.field.inst = append(st.field.inst, s...)
		}
	default:
		if st.dest.isText() {
			p.paraText(s)
		}
	}
}

// symbol handles control symbols.
func (p *parser) symbol(c byte) {
	st := p.st()
	switch c {
	case '*':
		if st.fresh {
			st.starred = true
		}
		return // the destination word that follows still opens the group
	case '\\', '{', '}':
		p.emit(string(c))
	case '~':
		p.emit("\u00a0")
	case '_':
		p.emit("\u2011")
	case '-', ':', '|':
		// optional hyphen, index subentry, formula: nothing visible
	case '\t':
		p.emit("\t")
	}
	st.fresh = false
}

// ---------------------------------------------------------------------------
// Control words
// ---------------------------------------------------------------------------

func (p *parser) word(t token) {
	st := p.st()
	fresh, starred := st.fresh, st.starred
	st.fresh = false
	if st.uprChild {
		st.uprChild = false
		if t.name != "ud" {
			p.skip()
			return
		}
		st.dest = p.stack[len(p.stack)-2].parentDest
		return
	}
	if p.destination(t, st) {
		return
	}
	if starred && fresh {
		// Unknown ignorable destination.
		p.skip()
		return
	}
	if p.charWord(t, st) || p.paraWord(t, st) || p.tableWord(t, st) {
		return
	}
	p.otherWord(t, st)
}

func boolParam(t token) bool { return !t.hasParam || t.param != 0 }

func (p *parser) charWord(t token, st *state) bool {
	c := &st.chr
	switch t.name {
	case "b":
		c.bold = boolParam(t)
	case "i":
		c.italic = boolParam(t)
	case "ul", "uld", "uldb", "uldash", "uldashd", "uldashdd", "ulhwave", "ulldash",
		"ulth", "ulthd", "ulthdash", "ulthdashd", "ulthdashdd", "ulthldash", "ululdbwave",
		"ulw", "ulwave":
		c.underline = boolParam(t)
	case "ulnone":
		c.underline = false
	case "strike":
		c.strike = boolParam(t)
	case "striked":
		c.strike = boolParam(t)
	case "super":
		c.vert = 1
	case "sub":
		c.vert = -1
	case "nosupersub":
		c.vert = 0
	case "up":
		c.vert = sign(t.param, t.hasParam, 1)
	case "dn":
		c.vert = -sign(t.param, t.hasParam, 1)
	case "scaps":
		c.smallcaps = boolParam(t)
	case "caps", "outl", "shad", "embo", "impr":
		// rendering effects that keep the text as typed
	case "v", "webhidden":
		c.hidden = boolParam(t)
	case "plain":
		link := c.link // a field result stays a link
		*c = defaultChar(p.defFont)
		c.lang, c.link = p.defLang, link
	case "f":
		c.font = t.param
	case "fs":
		if t.param > 0 {
			c.size = t.param
		}
	case "lang":
		c.lang = t.param
	case "highlight":
		c.highlight = t.param > 0
	case "cb", "chcbpat":
		c.highlight = p.shadeColor(t.param)
	case "cs":
		c.codeStyle = false
		if cs := p.cstyles[t.param]; cs != nil {
			c.codeStyle = isCodeCharStyle(cs.name)
		}
	default:
		return false
	}
	return true
}

func sign(v int, has bool, def int8) int8 {
	if !has {
		return def
	}
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// shadeColor reports whether a background colour index is a visible
// highlight (not "auto", not white).
func (p *parser) shadeColor(i int) bool {
	if i <= 0 || i >= len(p.colors) {
		return false
	}
	c := p.colors[i]
	return !c.auto && !(c.r >= 240 && c.g >= 240 && c.b >= 240)
}

func (p *parser) paraWord(t token, st *state) bool {
	pp := &st.para
	switch t.name {
	case "pard":
		*pp = defaultPara()
	case "s":
		pp.style = t.param
	case "intbl":
		pp.intbl = boolParam(t)
	case "itap":
		pp.itap = t.param
	case "ls":
		pp.ls = t.param
	case "ilvl":
		pp.ilvl = min(max(t.param, 0), 8)
	case "outlinelevel":
		pp.outline = t.param
	case "ql", "qj", "qd", "qt", "qk":
		pp.align = ast.AlignDefault
	case "qc":
		pp.align = ast.AlignCenter
	case "qr":
		pp.align = ast.AlignRight
	case "li", "lin":
		pp.li = t.param
	case "pagebb":
		pp.pagebb = boolParam(t)
	default:
		return false
	}
	return true
}

func (p *parser) otherWord(t token, st *state) {
	switch t.name {
	case "par":
		p.endParagraph()
	case "sect":
		if !p.cur().para.empty() {
			p.endParagraph()
		}
	case "line", "lbr":
		p.paraInline(&ast.LineBreak{})
	case "page":
		p.pageBreak()
	case "tab":
		p.emit("\t")
	case "cell":
		p.cellEnd(1)
	case "nestcell":
		p.cellEnd(max(2, len(p.cur().tables)))
	case "row":
		p.rowEnd(1)
	case "nestrow":
		p.rowEnd(max(2, len(p.cur().tables)))
	case "u":
		p.unicode(t.param)
	case "uc":
		st.uc = max(t.param, 0)
	case "ansi":
		p.docCP = 1252
	case "mac":
		p.docCP = 10000
	case "pc":
		p.docCP = 437
	case "pca":
		p.docCP = 850
	case "ansicpg":
		if singleByte(t.param) != nil || multiByte(t.param) != nil || t.param == cpUTF8 {
			p.docCP = t.param
		}
	case "deff":
		p.defFont = t.param
		if len(p.stack) <= 2 {
			p.stack[0].chr.font = t.param
			st.chr.font = t.param
		}
	case "deflang":
		p.defLang = t.param
		st.chr.lang = t.param
	case "emdash":
		p.emit("—")
	case "endash":
		p.emit("–")
	case "emspace":
		p.emit("\u2003")
	case "enspace":
		p.emit("\u2002")
	case "qmspace":
		p.emit("\u2005")
	case "bullet":
		p.emit("•")
	case "lquote":
		p.emit("‘")
	case "rquote":
		p.emit("’")
	case "ldblquote":
		p.emit("“")
	case "rdblquote":
		p.emit("”")
	case "zwj":
		p.emit("\u200d")
	case "zwnj":
		p.emit("\u200c")
	case "red":
		p.colorCur.r, p.colorSet = t.param, true
	case "green":
		p.colorCur.g, p.colorSet = t.param, true
	case "blue":
		p.colorCur.b, p.colorSet = t.param, true
	}
}
