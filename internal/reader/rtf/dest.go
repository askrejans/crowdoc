package rtf

import (
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// dest is the destination receiving text in the current group.
type dest uint8

const (
	dNormal       dest = iota // paragraph text
	dSkipText                 // control words are processed, text is dropped
	dFontTable                // \fonttbl
	dColorTable               // \colortbl
	dStyleSheet               // \stylesheet
	dCollect                  // text is appended to state.collect
	dFldInst                  // field instruction
	dPict                     // picture data
	dUpr                      // \upr wrapper (ANSI/Unicode alternatives)
	dInfo                     // \info
	dInfoTime                 // \creatim
	dListTable                // \listtable, \listoverridetable
	dList                     // \list
	dListLevel                // \listlevel
	dListOverride             // \listoverride
	dPn                       // old-style \pn paragraph numbering
)

func (d dest) isText() bool { return d == dNormal }

// owner marks what a group created, so its closing brace can finish it.
type owner uint8

const (
	ownNone owner = iota
	ownFootnote
	ownTextBox
	ownField
	ownPict
	ownShape
	ownSp
	ownSn
	ownSv
	ownInfoField
	ownInfoTime
	ownListText
	ownPn
	ownBookmark
	ownList
	ownListOverride
	ownObject
	ownNeXT
)

type charState struct {
	bold, italic, underline, strike bool
	smallcaps, hidden, highlight    bool
	codeStyle                       bool
	vert                            int8
	font                            int
	size                            int // half-points
	lang                            int
	link                            int // index+1 into parser.links
}

func defaultChar(font int) charState { return charState{font: font, size: 24} }

type paraState struct {
	style   int
	intbl   bool
	itap    int
	ls      int
	ilvl    int
	outline int
	align   ast.Align
	li      int
	pagebb  bool
}

func defaultPara() paraState { return paraState{outline: -1} }

// depth is the table nesting level of the paragraph (0 = body).
func (pp paraState) depth() int {
	if !pp.intbl && pp.itap <= 0 {
		return 0
	}
	return max(pp.itap, 1)
}

type pnInfo struct {
	bullet, cont bool
	level        int
	style        int // 0 decimal, 1 upper roman, 2 lower roman, 3 upper letter, 4 lower letter
	start        int
}

type spRec struct {
	name, value []byte
}

type shapeRec struct {
	done bool
	alt  string
	imgs []*ast.Image
}

type overrideRec struct{ listid, ls int }

// state is the per-group parser state.
type state struct {
	dest       dest
	parentDest dest
	chr        charState
	para       paraState
	uc         int
	starred    bool
	fresh      bool
	uprChild   bool
	sheetRoot  bool
	nestProps  bool
	owner      owner
	infoKey    string
	collect    *[]byte
	field      *fieldRec
	pict       *pictRec
	shape      *shapeRec
	sp         *spRec
	list       *listDef
	level      int
	override   *overrideRec
	pn         *pnInfo
	objImages  int
}

func (p *parser) collectInto(st *state, o owner) {
	buf := []byte{}
	st.collect = &buf
	st.dest = dCollect
	st.owner = o
}

// skipDests are destinations whose content never contributes to the body.
var skipDests = map[string]bool{
	"header": true, "headerl": true, "headerr": true, "headerf": true,
	"footer": true, "footerl": true, "footerr": true, "footerf": true,
	"ftnsep": true, "ftnsepc": true, "ftncn": true, "aftnsep": true, "aftnsepc": true, "aftncn": true,
	"revtbl": true, "rsidtbl": true, "xmlnstbl": true, "mmathPr": true, "generator": true,
	"themedata": true, "colorschememapping": true, "datastore": true, "latentstyles": true,
	"pgdsctbl": true, "listpicture": true, "pntxta": true, "pntxtb": true, "annotation": true,
	"atnid": true, "atnauthor": true, "atndate": true, "atnref": true, "atrfstart": true,
	"atrfend": true, "atnparent": true, "atnicn": true, "tc": true, "tcn": true, "xe": true,
	"txe": true, "rxe": true, "template": true, "docvar": true, "userprops": true,
	"falt": true, "panose": true, "fname": true, "fontemb": true, "fontfile": true,
	"passwordhash": true, "protusertbl": true, "bkmkend": true, "pnseclvl": true,
	"filetbl": true, "wgrffmtfilter": true, "keycode": true, "leveltext": true,
	"levelnumbers": true, "listname": true, "objdata": true, "objclass": true, "objname": true,
	"objalias": true, "objsect": true, "objitem": true, "objtopic": true, "nonshppict": true,
	"nonesttables": true, "moMath": true, "moMathPara": true, "mmath": true, "do": true,
	"blipuid": true, "fldtype": true, "datafield": true, "htmltag": true,
	"expandedcolortbl": true, "ffdeftext": true, "ffformat": true, "ffhelptext": true,
	"ffstattext": true, "ffentrymcr": true, "ffexitmcr": true, "ffname": true,
	"background": true, "pgptbl": true, "revtim": true, "printim": true,
	"buptim": true, "hlinkbase": true, "oldcprops": true, "oldpprops": true,
	"oldsprops": true, "oldtprops": true,
}

// destination handles control words that start or configure a destination.
// It reports whether the word was consumed.
func (p *parser) destination(t token, st *state) bool {
	switch st.dest {
	case dFontTable:
		if p.fontWord(t, st) {
			return true
		}
	case dStyleSheet:
		if p.styleWord(t, st) {
			return true
		}
	case dListTable, dList, dListLevel, dListOverride:
		if p.listWord(t, st) {
			return true
		}
	case dPn:
		if p.pnWord(t, st) {
			return true
		}
	case dPict:
		if p.pictWord(t, st) {
			return true
		}
	case dInfo:
		switch t.name {
		case "title", "subject", "author", "keywords", "doccomm", "company", "manager", "category":
			p.collectInto(st, ownInfoField)
			st.infoKey = t.name
			return true
		case "creatim":
			st.dest = dInfoTime
			st.owner = ownInfoTime
			return true
		case "operator", "comment", "version", "vern", "edmins", "nofpages", "nofwords",
			"nofchars", "nofcharsws", "id":
			p.skip()
			return true
		}
	case dInfoTime:
		switch t.name {
		case "yr":
			p.infoDate[0] = t.param
		case "mo":
			p.infoDate[1] = t.param
		case "dy":
			p.infoDate[2] = t.param
		}
		return true
	}

	if skipDests[t.name] {
		if t.name == "mmath" {
			p.warn.Addf("equations are not supported in RTF input; their fallback pictures are used where present")
		}
		p.skip()
		return true
	}
	switch t.name {
	case "fonttbl":
		st.dest = dFontTable
	case "colortbl":
		p.colors = p.colors[:0]
		st.dest = dColorTable
	case "stylesheet":
		st.dest = dStyleSheet
		st.sheetRoot = true
	case "info":
		st.dest = dInfo
	case "listtable", "listoverridetable":
		st.dest = dListTable
	case "listtext", "pntext":
		p.collectInto(st, ownListText)
	case "pn":
		st.dest = dPn
		st.pn = &pnInfo{}
		st.owner = ownPn
	case "field":
		f := &fieldRec{images: p.images}
		if st.dest == dFldInst {
			f.inInst = st.field
		}
		st.field = f
		st.owner = ownField
	case "fldinst":
		st.dest = dFldInst
	case "fldrslt":
		p.fieldResult(st)
	case "footnote":
		p.pushCtx()
		st.owner = ownFootnote
		st.dest = dNormal
		st.para = defaultPara()
	case "bkmkstart":
		p.collectInto(st, ownBookmark)
	case "pict":
		st.dest = dPict
		st.pict = &pictRec{scaleX: 100, scaleY: 100}
		st.owner = ownPict
	case "shppict", "mmathPict", "nesttableprops", "ud", "picprop":
		if t.name == "nesttableprops" {
			st.nestProps = true
		}
	case "shp":
		st.shape = &shapeRec{}
		st.owner = ownShape
	case "shpinst", "shpgrp":
		st.dest = dSkipText
	case "sp":
		st.sp = &spRec{}
		st.owner = ownSp
		st.dest = dSkipText
	case "sn":
		p.collectInto(st, ownSn)
	case "sv":
		if st.sp == nil {
			p.skip()
			return true
		}
		switch string(st.sp.name) {
		case "pib":
			st.dest = dSkipText
		case "wzDescription", "wzName":
			p.collectInto(st, ownSv)
		default:
			p.skip()
		}
	case "shptxt":
		p.pushCtx()
		st.owner = ownTextBox
		st.dest = dNormal
		st.para = defaultPara()
		if st.shape != nil {
			st.shape.done = true
		}
	case "shprslt":
		if st.shape != nil && st.shape.done {
			p.skip()
			return true
		}
		st.dest = dNormal
	case "object":
		st.owner = ownObject
		st.objImages = p.images
		st.dest = dSkipText
	case "result":
		st.dest = dNormal
	case "upr":
		st.parentDest = st.dest
		st.dest = dUpr
	case "NeXTGraphic":
		p.collectInto(st, ownNeXT)
	default:
		return false
	}
	return true
}

// endGroup finishes whatever the closed group owned.
func (p *parser) endGroup(closed *state) {
	switch closed.owner {
	case ownFootnote:
		blocks := p.popCtxWith(closed.para)
		if len(blocks) > 0 {
			p.paraInline(noteInline(blocks))
		}
	case ownTextBox:
		blocks := p.popCtxWith(closed.para)
		c := p.cur()
		c.after = append(c.after, blocks...)
	case ownField:
		p.endField(closed.field)
	case ownPict:
		p.endPict(closed.pict, closed.shape)
	case ownShape:
		if sh := closed.shape; sh != nil && sh.alt != "" {
			for _, img := range sh.imgs {
				if img.Alt == "" {
					img.Alt = sh.alt
				}
			}
		}
	case ownSn:
		if sp := p.st().sp; sp != nil && closed.collect != nil {
			sp.name = append(sp.name[:0], strings.TrimSpace(string(*closed.collect))...)
		}
	case ownSv:
		if sp := p.st().sp; sp != nil && closed.collect != nil {
			sp.value = append(sp.value[:0], *closed.collect...)
		}
	case ownSp:
		if sp := closed.sp; sp != nil && string(sp.name) == "wzDescription" {
			alt := strings.TrimSpace(string(sp.value))
			if sh := p.st().shape; sh != nil && sh.alt == "" {
				sh.alt = alt
			}
			if pc := p.st().pict; pc != nil && pc.alt == "" {
				pc.alt = alt
			}
		}
	case ownInfoField:
		if closed.collect != nil {
			p.info[closed.infoKey] = strings.TrimSpace(string(*closed.collect))
		}
	case ownListText:
		if closed.collect != nil {
			c := p.cur()
			c.para.listText = strings.TrimSpace(strings.ReplaceAll(string(*closed.collect), "\t", " "))
			c.para.hasListText = true
		}
	case ownPn:
		if closed.pn != nil {
			pn := *closed.pn
			p.cur().para.pn = &pn
		}
	case ownBookmark:
		if closed.collect != nil {
			if id := sanitizeID(string(*closed.collect)); id != "" {
				p.paraInline(anchor(id))
			}
		}
	case ownList:
		if closed.list != nil {
			p.lists[closed.list.id] = closed.list
		}
	case ownListOverride:
		if o := closed.override; o != nil && o.ls > 0 {
			p.overrides[o.ls] = o.listid
		}
	case ownObject:
		if p.images == closed.objImages {
			p.warn.Addf("embedded OLE object without a preview image was dropped")
		}
	case ownNeXT:
		name := ""
		if closed.collect != nil {
			name = strings.TrimSpace(string(*closed.collect))
		}
		if name == "" {
			name = "attachment"
		}
		p.warn.Addf("image %q is stored outside the RTF file and is not supported", name)
		if len(p.stack) > 2 {
			// An attachment is followed by a placeholder character.
			p.st().dest = dSkipText
		}
	}
}

// ---------------------------------------------------------------------------
// Font table
// ---------------------------------------------------------------------------

func (p *parser) font(id int) *fontInfo {
	f := p.fonts[id]
	if f == nil {
		f = &fontInfo{}
		p.fonts[id] = f
	}
	return f
}

func (p *parser) fontWord(t token, st *state) bool {
	switch t.name {
	case "f":
		st.chr.font = t.param
		p.font(t.param)
		p.fontName = p.fontName[:0]
	case "fcharset":
		f := p.font(st.chr.font)
		f.charset = t.param
		f.cp = charsetCodePage(t.param)
	case "fnil", "froman", "fswiss", "fmodern", "fscript", "fdecor", "ftech", "fbidi":
		p.font(st.chr.font).family = t.name
	case "fprq":
		p.font(st.chr.font).pitch = t.param
	case "cpg":
		if singleByte(t.param) != nil || multiByte(t.param) != nil {
			p.font(st.chr.font).cp = t.param
		}
	default:
		return false
	}
	return true
}

func (p *parser) fontText(s string) {
	for {
		i := strings.IndexByte(s, ';')
		if i < 0 {
			p.fontName = append(p.fontName, s...)
			return
		}
		p.fontName = append(p.fontName, s[:i]...)
		p.finishFont()
		s = s[i+1:]
	}
}

func (p *parser) finishFont() {
	f := p.font(p.st().chr.font)
	f.name = strings.TrimSpace(string(p.fontName))
	p.fontName = p.fontName[:0]
	lower := strings.ToLower(f.name)
	if f.cp == cpSymbol && strings.HasPrefix(lower, "wingdings") {
		f.cp = cpWingdings
	}
	f.mono = isMonoFont(lower, f)
}

func isMonoFont(lower string, f *fontInfo) bool {
	for _, k := range monoNames {
		if strings.Contains(lower, k) {
			return true
		}
	}
	switch f.charset {
	case 128, 129, 130, 134, 136:
		return false // CJK body fonts are often fixed-pitch
	}
	if f.cp == cpSymbol || f.cp == cpWingdings {
		return false
	}
	return f.pitch == 1 || f.family == "fmodern" && f.pitch != 2
}

var monoNames = []string{
	"courier", "consolas", "menlo", "monaco", "mono", "source code", "fira code",
	"cascadia code", "lucida console", "lucida sans typewriter", "inconsolata",
	"andale", "fixedsys", "letter gothic", "ocr a", "ocr-a", "hack", "jetbrains",
}

// ---------------------------------------------------------------------------
// Style sheet
// ---------------------------------------------------------------------------

func (p *parser) styleWord(t token, st *state) bool {
	switch t.name {
	case "s":
		p.curStyle = p.style(p.styles, t.param)
	case "cs":
		p.curStyle = p.style(p.cstyles, t.param)
	case "ts", "ds":
		p.curStyle = nil
	case "outlinelevel":
		if p.curStyle != nil {
			p.curStyle.outline = t.param
		}
	case "sbasedon":
		if p.curStyle != nil {
			p.curStyle.basedOn = t.param
		}
	case "lang":
		if p.curStyle != nil {
			p.curStyle.lang = t.param
		}
	default:
		return false
	}
	return true
}

func (p *parser) style(m map[int]*styleInfo, id int) *styleInfo {
	s := m[id]
	if s == nil {
		s = &styleInfo{outline: -1, basedOn: -1}
		m[id] = s
	}
	p.styleName = p.styleName[:0]
	return s
}

func (p *parser) styleText(s string) {
	for {
		i := strings.IndexByte(s, ';')
		if i < 0 {
			p.styleName = append(p.styleName, s...)
			return
		}
		p.styleName = append(p.styleName, s[:i]...)
		if p.curStyle != nil {
			p.curStyle.name = strings.TrimSpace(string(p.styleName))
		}
		p.styleName = p.styleName[:0]
		s = s[i+1:]
	}
}

// resolve classifies a paragraph style, following \sbasedon chains.
func (p *parser) resolve(id int) *styleInfo {
	s := p.styles[id]
	if s == nil {
		return nil
	}
	if s.resolved {
		return s
	}
	s.resolved = true
	s.kind, s.level = classifyStyle(s.name)
	if s.kind == skNone && s.outline >= 0 && s.outline <= 8 {
		s.kind, s.level = skHeading, s.outline+1
	}
	if s.kind == skNone && s.basedOn >= 0 && s.basedOn != id {
		if base := p.resolve(s.basedOn); base != nil && base.kind != skTitle && base.kind != skSubtitle && base.kind != skTOC {
			s.kind, s.level = base.kind, base.level
		}
	}
	return s
}

var headingWords = []string{
	"heading", "überschrift", "titre", "virsraksts", "заголовок", "otsikko", "rubrik",
	"overskrift", "kop", "nagłówek", "título", "titolo", "antraštė", "pealkiri", "nadpis",
	"címsor", "başlık", "επικεφαλίδα", "заглавие", "kopteksts", "naslov", "antraste",
}

var styleNames = map[string]styleKind{
	"title": skTitle, "titel": skTitle, "titre": skTitle, "nosaukums": skTitle, "название": skTitle,
	"título": skTitle, "titolo": skTitle, "tytuł": skTitle, "otsikko": skTitle, "pavadinimas": skTitle,
	"subtitle": skSubtitle, "untertitel": skSubtitle, "sous-titre": skSubtitle,
	"apakšvirsraksts": skSubtitle, "подзаголовок": skSubtitle, "subtítulo": skSubtitle,
	"sottotitolo": skSubtitle, "podtytuł": skSubtitle, "paantraštė": skSubtitle,
	"quote": skQuote, "intense quote": skQuote, "block text": skQuote, "quotations": skQuote,
	"zitat": skQuote, "citation": skQuote, "citāts": skQuote, "цитата": skQuote, "cita": skQuote,
	"html preformatted": skCode, "preformatted text": skCode, "code": skCode, "source code": skCode,
	"macro text": skCode, "plain text": skCode, "verbatim": skCode, "code block": skCode,
	"caption": skCaption, "beschriftung": skCaption, "légende": skCaption, "paraksts": skCaption,
	"figure": skCaption, "illustration": skCaption, "drawing": skCaption,
	"didascalia": skCaption, "epígrafe": skCaption, "название объекта": skCaption,
	"toc heading": skTOC, "contents heading": skTOC,
}

// classifyStyle maps a style name onto a structural role.
func classifyStyle(name string) (styleKind, int) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return skNone, 0
	}
	if k, ok := styleNames[n]; ok {
		return k, 0
	}
	// "heading 1", "Überschrift 2", "Heading1", "toc 3"
	base := strings.TrimRight(n, "0123456789")
	digits := n[len(base):]
	base = strings.TrimSpace(base)
	if digits != "" && len(digits) == 1 && digits != "0" {
		level := int(digits[0] - '0')
		if base == "toc" || base == "contents" || base == "satura rādītājs" || base == "verzeichnis" {
			return skTOC, level
		}
		for _, w := range headingWords {
			if base == w {
				return skHeading, level
			}
		}
	}
	return skNone, 0
}

func isCodeCharStyle(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "code") || strings.Contains(n, "verbatim") ||
		n == "source text" || strings.Contains(n, "html typewriter") || strings.Contains(n, "html keyboard")
}

// ---------------------------------------------------------------------------
// List tables and old-style numbering
// ---------------------------------------------------------------------------

func (p *parser) listWord(t token, st *state) bool {
	switch st.dest {
	case dListTable:
		switch t.name {
		case "list":
			st.list = &listDef{}
			st.dest = dList
			st.owner = ownList
			return true
		case "listoverride":
			st.override = &overrideRec{}
			st.dest = dListOverride
			st.owner = ownListOverride
			return true
		}
	case dList:
		switch t.name {
		case "listid":
			st.list.id = t.param
			return true
		case "listlevel":
			st.list.levels = append(st.list.levels, listLevelDef{start: 1})
			st.level = len(st.list.levels) - 1
			st.dest = dListLevel
			return true
		}
	case dListLevel:
		if st.list == nil || st.level >= len(st.list.levels) {
			return false
		}
		lv := &st.list.levels[st.level]
		switch t.name {
		case "levelnfc":
			lv.nfc, lv.hasNfc = t.param, true
			return true
		case "levelnfcn":
			if !lv.hasNfc {
				lv.nfc = t.param
			}
			lv.hasNfcN = true
			return true
		case "levelstartat":
			lv.start = t.param
			return true
		}
	case dListOverride:
		switch t.name {
		case "listid":
			st.override.listid = t.param
			return true
		case "ls":
			st.override.ls = t.param
			return true
		}
	}
	return false
}

func (p *parser) pnWord(t token, st *state) bool {
	pn := st.pn
	switch t.name {
	case "pnlvlblt":
		pn.bullet = true
	case "pnlvlbody":
		pn.bullet = false
	case "pnlvlcont":
		pn.cont = true
	case "pnlvl":
		pn.level = min(max(t.param-1, 0), 8)
	case "pndec", "pncard", "pnord", "pnordt":
		pn.style = 0
	case "pnucrm":
		pn.style = 1
	case "pnlcrm":
		pn.style = 2
	case "pnucltr":
		pn.style = 3
	case "pnlcltr":
		pn.style = 4
	case "pnstart":
		pn.start = t.param
	default:
		return false
	}
	return true
}
