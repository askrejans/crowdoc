package docx

import (
	"strconv"
	"strings"
	"unicode"
)

// tri is an unset/on/off property value, so style inheritance can tell
// "not specified" from "explicitly off".
type tri int8

const (
	unset tri = iota
	on
	off
)

func triOf(n *node) tri {
	if n == nil {
		return unset
	}
	if onOff(n) {
		return on
	}
	return off
}

func (t tri) or(base tri) tri {
	if t != unset {
		return t
	}
	return base
}

// runProps are the run properties the reader cares about.
type runProps struct {
	bold, italic, underline, strike, smallCaps, highlight, vanish tri
	sup, sub                                                      tri
	font                                                          string
	size                                                          int // half-points
	position                                                      int // baseline shift, half-points
	lang                                                          string
	rStyle                                                        string
}

func parseRunProps(rpr *node) runProps {
	var p runProps
	if rpr == nil {
		return p
	}
	for _, k := range rpr.kids {
		switch k.name {
		case "b":
			p.bold = triOf(k)
		case "i":
			p.italic = triOf(k)
		case "u":
			switch strings.ToLower(k.attr("val")) {
			case "none", "0", "false", "off":
				p.underline = off
			default:
				p.underline = on
			}
		case "strike", "dstrike":
			if t := triOf(k); t == on || p.strike == unset {
				p.strike = t
			}
		case "smallCaps":
			p.smallCaps = triOf(k)
		case "vanish", "specVanish":
			p.vanish = triOf(k)
		case "highlight":
			if strings.EqualFold(k.attr("val"), "none") {
				p.highlight = off
			} else {
				p.highlight = on
			}
		case "shd":
			if p.highlight == unset && shadeColored(k) {
				p.highlight = on
			}
		case "vertAlign":
			switch k.attr("val") {
			case "superscript":
				p.sup, p.sub = on, off
			case "subscript":
				p.sub, p.sup = on, off
			case "baseline":
				p.sup, p.sub = off, off
			}
		case "rFonts":
			if f := k.attr("ascii"); f != "" {
				p.font = f
			} else if f := k.attr("hAnsi"); f != "" {
				p.font = f
			}
		case "sz":
			if v, err := strconv.Atoi(k.attr("val")); err == nil && v > 0 {
				p.size = v
			}
		case "position":
			if v, err := strconv.Atoi(k.attr("val")); err == nil {
				p.position = v
			}
		case "lang":
			p.lang = k.attr("val")
		case "rStyle":
			p.rStyle = k.attr("val")
		}
	}
	return p
}

// shadeColored reports whether a w:shd paints a visible background.
func shadeColored(shd *node) bool {
	fill := strings.ToLower(shd.attr("fill"))
	if fill != "" && fill != "auto" && fill != "ffffff" {
		return true
	}
	if strings.EqualFold(shd.attr("val"), "solid") {
		c := strings.ToLower(shd.attr("color"))
		return c != "" && c != "auto" && c != "ffffff"
	}
	return false
}

// over returns p with every property set in q applied on top.
func (p runProps) over(q runProps) runProps {
	p.bold = q.bold.or(p.bold)
	p.italic = q.italic.or(p.italic)
	p.underline = q.underline.or(p.underline)
	p.strike = q.strike.or(p.strike)
	p.smallCaps = q.smallCaps.or(p.smallCaps)
	p.highlight = q.highlight.or(p.highlight)
	p.vanish = q.vanish.or(p.vanish)
	p.sup = q.sup.or(p.sup)
	p.sub = q.sub.or(p.sub)
	if q.font != "" {
		p.font = q.font
	}
	if q.size != 0 {
		p.size = q.size
	}
	if q.position != 0 {
		p.position = q.position
	}
	if q.lang != "" {
		p.lang = q.lang
	}
	return p
}

// paraStyleProps are paragraph properties inheritable through styles.
type paraStyleProps struct {
	outline         int // -1 unset; 0-8 heading levels, 9 body text
	numID           string
	ilvl            int
	hasNum, hasIlvl bool
	pageBreakBefore tri
	jc              string
	indLeft         int
	hasInd          bool
}

func parseParaProps(ppr *node) paraStyleProps {
	p := paraStyleProps{outline: -1}
	if ppr == nil {
		return p
	}
	for _, k := range ppr.kids {
		switch k.name {
		case "outlineLvl":
			if v, err := strconv.Atoi(k.attr("val")); err == nil && v >= 0 {
				p.outline = v
			}
		case "numPr":
			if id := k.child("numId"); id != nil {
				p.numID, p.hasNum = id.attr("val"), true
			}
			if l := k.child("ilvl"); l != nil {
				if v, err := strconv.Atoi(l.attr("val")); err == nil && v >= 0 && v < 9 {
					p.ilvl, p.hasIlvl = v, true
				}
			}
		case "pageBreakBefore":
			p.pageBreakBefore = triOf(k)
		case "jc":
			p.jc = k.attr("val")
		case "ind":
			v := k.attr("left")
			if v == "" {
				v = k.attr("start")
			}
			if n, err := strconv.Atoi(v); err == nil {
				p.indLeft, p.hasInd = n, true
			}
		}
	}
	return p
}

type style struct {
	id, name, typ, basedOn string
	isDefault              bool
	ppr                    paraStyleProps
	rpr                    runProps
	firstRowBold           bool // table style: conditional first-row formatting is bold
}

// styleKind classifies paragraph styles that change block structure.
type styleKind int

const (
	kindNone styleKind = iota
	kindTitle
	kindSubtitle
	kindQuote
	kindCaption
	kindCode
	kindTOC
)

type styleSheet struct {
	byID        map[string]*style
	defaultPara string
	docRun      runProps
	// monoBody is set when the document's body font is monospace; font-based
	// code detection is then meaningless and disabled.
	monoBody bool

	runCache  map[string]runProps
	kindCache map[string]styleKind
	headCache map[string]int
	paraCache map[string]paraStyleProps
}

func parseStyles(root *node) *styleSheet {
	st := &styleSheet{
		byID:      map[string]*style{},
		runCache:  map[string]runProps{},
		kindCache: map[string]styleKind{},
		headCache: map[string]int{},
		paraCache: map[string]paraStyleProps{},
	}
	if root == nil {
		return st
	}
	if dd := root.child("docDefaults"); dd != nil {
		st.docRun = parseRunProps(dd.child("rPrDefault").child("rPr"))
	}
	for _, s := range root.childrenNamed("style") {
		sty := &style{
			id:        s.attr("styleId"),
			name:      strings.ToLower(strings.TrimSpace(s.val("name"))),
			typ:       s.attr("type"),
			basedOn:   s.val("basedOn"),
			isDefault: truthy(s.attr("default")),
			ppr:       parseParaProps(s.child("pPr")),
			rpr:       parseRunProps(s.child("rPr")),
		}
		for _, cond := range s.childrenNamed("tblStylePr") {
			if cond.attr("type") == "firstRow" && parseRunProps(cond.child("rPr")).bold == on {
				sty.firstRowBold = true
			}
		}
		if sty.id == "" {
			continue
		}
		st.byID[sty.id] = sty
		if sty.typ == "paragraph" && sty.isDefault && st.defaultPara == "" {
			st.defaultPara = sty.id
		}
	}
	if st.defaultPara == "" {
		if _, ok := st.byID["Normal"]; ok {
			st.defaultPara = "Normal"
		}
	}
	st.monoBody = isMonoFont(st.paraRun(st.defaultPara).font)
	return st
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "on", "true":
		return true
	}
	return false
}

// chain returns the style and its basedOn ancestors (self first), guarding
// against cycles.
func (st *styleSheet) chain(id string) []*style {
	var out []*style
	seen := map[string]bool{}
	for id != "" && !seen[id] && len(out) < 16 {
		seen[id] = true
		s := st.byID[id]
		if s == nil {
			break
		}
		out = append(out, s)
		id = s.basedOn
	}
	return out
}

// styleRun returns the run properties a style defines, inheritance applied.
func (st *styleSheet) styleRun(id string) runProps {
	if p, ok := st.runCache[id]; ok {
		return p
	}
	ch := st.chain(id)
	var p runProps
	for i := len(ch) - 1; i >= 0; i-- {
		p = p.over(ch[i].rpr)
	}
	st.runCache[id] = p
	return p
}

// paraRun returns the document defaults with paragraph style id applied:
// the baseline every run of such a paragraph starts from.
func (st *styleSheet) paraRun(id string) runProps {
	if id == "" {
		id = st.defaultPara
	}
	return st.docRun.over(st.styleRun(id))
}

// paraProps resolves inheritable paragraph properties of style id.
func (st *styleSheet) paraProps(id string) paraStyleProps {
	if p, ok := st.paraCache[id]; ok {
		return p
	}
	p := paraStyleProps{outline: -1}
	ch := st.chain(id)
	for i := len(ch) - 1; i >= 0; i-- {
		q := ch[i].ppr
		if q.outline >= 0 {
			p.outline = q.outline
		}
		if q.hasNum {
			p.numID, p.hasNum = q.numID, true
		}
		if q.hasIlvl {
			p.ilvl, p.hasIlvl = q.ilvl, true
		}
		p.pageBreakBefore = q.pageBreakBefore.or(p.pageBreakBefore)
		if q.jc != "" {
			p.jc = q.jc
		}
		if q.hasInd {
			p.indLeft, p.hasInd = q.indLeft, true
		}
	}
	st.paraCache[id] = p
	return p
}

// headingLevel returns the heading level (1-9) of paragraph style id, or 0.
// An explicit outline level wins; otherwise the (possibly localised) style
// name decides.
func (st *styleSheet) headingLevel(id string) int {
	if id == "" {
		return 0
	}
	if l, ok := st.headCache[id]; ok {
		return l
	}
	level := 0
	ch := st.chain(id)
	found := false
	for i, s := range ch {
		if s.ppr.outline < 0 {
			continue
		}
		if s.ppr.outline <= 8 {
			level, found = s.ppr.outline+1, true
		} else if i == 0 {
			// The style itself declares body text.
			found = true
		}
		break
	}
	if !found {
		for _, s := range ch {
			if l := headingFromName(s.name); l > 0 {
				level = l
				break
			}
			if l := headingFromName(strings.ToLower(s.id)); l > 0 {
				level = l
				break
			}
		}
	}
	if level > 0 && st.kind(id) == kindTOC {
		level = 0
	}
	st.headCache[id] = level
	return level
}

// headingPrefixes are "heading" in the languages word processors localise
// style names into (lower case, as in w:name or w:styleId).
var headingPrefixes = []string{
	"heading", "virsraksts", "überschrift", "berschrift", "titre", "título", "ttulo",
	"titolo", "kop", "nagłówek", "nagwek", "nadpis", "rubrik", "overskrift",
	"otsikko", "pealkiri", "antraštė", "antrat", "заголовок", "címsor", "başlık",
	"balk", "titlu", "naslov", "заглавие", "επικεφαλίδα", "見出し", "标题", "제목",
}

func headingFromName(name string) int {
	if name == "" {
		return 0
	}
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	if i == len(name) {
		if name == "heading" {
			return 1
		}
		return 0
	}
	level, err := strconv.Atoi(name[i:])
	if err != nil || level < 1 || level > 9 {
		return 0
	}
	prefix := strings.TrimSpace(name[:i])
	for _, p := range headingPrefixes {
		if prefix == p {
			return level
		}
	}
	return 0
}

// kind classifies paragraph style id (inheritance considered).
func (st *styleSheet) kind(id string) styleKind {
	if id == "" {
		return kindNone
	}
	if k, ok := st.kindCache[id]; ok {
		return k
	}
	k := kindNone
	for _, s := range st.chain(id) {
		if k = kindFromName(s.name); k != kindNone {
			break
		}
		if k = kindFromName(strings.ToLower(s.id)); k != kindNone {
			break
		}
	}
	if k == kindNone && !st.monoBody && isMonoFont(st.styleRun(id).font) {
		k = kindCode
	}
	st.kindCache[id] = k
	return k
}

func kindFromName(name string) styleKind {
	switch name {
	case "":
		return kindNone
	case "title", "titel", "titre", "título", "titolo", "nosaukums", "tytuł", "název", "заголовок":
		return kindTitle
	case "subtitle", "untertitel", "sous-titre", "subtítulo", "sottotitolo", "apakšvirsraksts",
		"podtytuł", "podtitul", "ondertitel", "undertitel", "подзаголовок":
		return kindSubtitle
	case "quote", "intense quote", "intensequote", "block text", "blocktext", "quotations",
		"block quotation", "blockquotation",
		"zitat", "intensives zitat", "citation", "citation intense", "cita", "cita destacada",
		"citāts", "intensīvs citāts", "citaat", "duidelijk citaat", "cytat", "citát", "blockquote",
		"block quote":
		return kindQuote
	case "caption", "beschriftung", "légende", "epígrafe", "didascalia", "paraksts", "bijschrift",
		"legenda", "popisek", "image caption", "imagecaption", "table caption", "tablecaption",
		"figure caption", "figurecaption", "figure", "table", "illustration", "drawing":
		return kindCaption
	case "html preformatted", "htmlpreformatted", "macro text", "macrotext", "preformatted text",
		"preformattedtext", "source code", "sourcecode", "code", "code block", "codeblock",
		"verbatim", "listing", "program code":
		return kindCode
	case "toc heading", "tocheading", "table of figures", "tableoffigures", "index heading":
		return kindTOC
	case "captioned figure", "capturedfigure", "captionedfigure":
		return kindNone
	}
	if strings.HasPrefix(name, "toc ") || (strings.HasPrefix(name, "toc") && len(name) == 4 && name[3] >= '1' && name[3] <= '9') {
		return kindTOC
	}
	if strings.Contains(name, "caption") {
		return kindCaption
	}
	if strings.Contains(name, "quote") || strings.Contains(name, "quotation") {
		return kindQuote
	}
	if strings.Contains(name, "code") || strings.Contains(name, "preformatted") || strings.Contains(name, "verbatim") {
		return kindCode
	}
	return kindNone
}

// charStyleClass reports how character style id affects formatting: code
// styles mark inline code, link styles are ignored entirely.
func (st *styleSheet) charStyleClass(id string) (code, ignore bool) {
	for _, s := range st.chain(id) {
		name := s.name
		lid := strings.ToLower(s.id)
		switch {
		case name == "hyperlink" || name == "followedhyperlink" || lid == "hyperlink" || lid == "followedhyperlink" ||
			name == "internet link" || name == "visited internet link":
			return false, true
		case strings.Contains(name, "code") || strings.Contains(name, "verbatim") ||
			name == "source text" || strings.HasPrefix(name, "html typewriter") ||
			strings.HasPrefix(name, "html keyboard") || strings.HasPrefix(name, "html sample") ||
			lid == "verbatimchar" || lid == "htmlcode":
			return true, false
		}
	}
	return false, false
}

// monoFonts are well-known monospace families whose names do not contain
// "mono".
var monoFonts = []string{
	"consolas", "courier", "lucida console", "lucida sans typewriter", "menlo", "monaco",
	"source code", "fira code", "inconsolata", "anonymous pro", "hack", "andale",
	"ocr a", "ocr-a", "fixedsys", "terminal", "letter gothic", "prestige elite",
	"cascadia", "iosevka", "sf mono", "pt mono", "cousine", "everson mono", "monospace",
	"monofur", "envy code", "proggy", "droid sans mono", "jetbrains mono",
}

func isMonoFont(name string) bool {
	if name == "" {
		return false
	}
	n := strings.ToLower(name)
	if strings.HasPrefix(n, "monotype") {
		return false
	}
	if strings.Contains(n, "mono") {
		return true
	}
	for _, f := range monoFonts {
		if strings.HasPrefix(n, f) {
			return true
		}
	}
	return false
}

// symbolFont reports which symbol-font table applies to a font name.
func symbolFont(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == "symbol":
		return "symbol"
	case n == "wingdings" || n == "wingdings 1":
		return "wingdings"
	case n == "wingdings 2":
		return "wingdings2"
	case n == "wingdings 3":
		return "wingdings3"
	case n == "webdings":
		return "webdings"
	}
	return ""
}

// sanitizeID turns a bookmark or anchor name into an identifier made of
// lowercase ASCII letters, digits, '-', '_', ':' and '.'.
func sanitizeID(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == ':', r == '.':
			b.WriteRune(r)
			dash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(unicode.ToLower(r))
			dash = false
		case r >= 0xC0 && r <= 0x17F:
			c := foldLatin[r-0xC0]
			if c == '-' {
				if !dash && b.Len() > 0 {
					b.WriteByte('-')
					dash = true
				}
				continue
			}
			b.WriteByte(c)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// foldLatin maps U+00C0..U+017F to a lowercase ASCII letter ('-' = none).
const foldLatin = "aaaaaaaceeeeiiii" + "dnooooo-ouuuuyts" + "aaaaaaaceeeeiiii" + "dnooooo-ouuuuyty" +
	"aaaaaaccccccccdd" + "ddeeeeeeeeeegggg" + "gggghhhhiiiiiiii" + "iiiijjkkklllllll" +
	"lllnnnnnnnnnoooo" + "oooorrrrrrssssss" + "ssttttttuuuuuuuu" + "uuuuwwyyyzzzzzzs"
