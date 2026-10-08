package odt

import (
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// flags are inline formatting attributes of a text run.
type flags uint16

const (
	fBold flags = 1 << iota
	fItalic
	fUnderline
	fStrike
	fSuper
	fSub
	fSmallCaps
	fHighlight
	fCode
)

// wrapOrder lists the container flags in nesting preference order.
var wrapOrder = []flags{fBold, fItalic, fUnderline, fStrike, fSuper, fSub, fSmallCaps, fHighlight}

// textProps is a partial set of formatting: set marks the flags the style
// defines, on their values.
type textProps struct{ set, on flags }

func (p *textProps) put(f flags, v bool) {
	p.set |= f
	if v {
		p.on |= f
	} else {
		p.on &^= f
	}
}

// over applies p on top of base.
func (p textProps) over(base flags) flags {
	return base&^p.set | p.on
}

// merge fills the flags p does not set from parent.
func (p textProps) merge(parent textProps) textProps {
	return textProps{set: p.set | parent.set, on: p.on | parent.on&^p.set}
}

type style struct {
	name, display, family, parent string
	text                          textProps
	align                         ast.Align
	breakBefore, breakAfter       bool
	masterPage                    string
	listStyle                     string
	outline                       int
}

type paraRole int

const (
	roleNormal paraRole = iota
	roleTitle
	roleSubtitle
	roleQuote
	roleCode
	roleCaption
	roleHeading
)

// resolved is a style with its parent chain applied.
type resolved struct {
	text        textProps
	align       ast.Align
	breakBefore bool
	breakAfter  bool
	masterPage  string
	listStyle   string
	role        paraRole
	caption     string // "table" or "figure" for caption styles
	level       int    // heading level of "Heading N" paragraph styles
}

type listLevel struct {
	ordered bool
	style   ast.NumberStyle
	start   int
}

type styleSet struct {
	styles   map[string]*style // family + "/" + name
	defaults map[string]*style // family -> default style
	lists    map[string][]listLevel
	monoFace map[string]bool // font-face names with fixed pitch or a monospace family
	faces    map[string]string
	cache    map[string]*resolved
	lang     string
}

func newStyleSet() *styleSet {
	return &styleSet{
		styles:   map[string]*style{},
		defaults: map[string]*style{},
		lists:    map[string][]listLevel{},
		monoFace: map[string]bool{},
		faces:    map[string]string{},
		cache:    map[string]*resolved{},
	}
}

// decodeName turns encoded style names ("Heading_20_1") into display form.
func decodeName(s string) string {
	if !strings.Contains(s, "_") {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			if j := strings.IndexByte(s[i+1:], '_'); j > 0 && j <= 4 {
				if v, err := strconv.ParseUint(s[i+1:i+1+j], 16, 32); err == nil {
					sb.WriteRune(rune(v))
					i += j + 1
					continue
				}
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

var monoPatterns = []string{
	"mono", "courier", "consolas", "menlo", "monaco", "source code", "fira code",
	"inconsolata", "lucida console", "cascadia", "andale", "letter gothic", "ocr a",
	"terminal", "fixedsys", "hack", "iosevka", "code",
}

func isMonoFamily(fam string) bool {
	fam = strings.ToLower(strings.Trim(strings.TrimSpace(strings.Split(fam, ",")[0]), `"'`))
	if fam == "" {
		return false
	}
	for _, p := range monoPatterns {
		if strings.Contains(fam, p) {
			return true
		}
	}
	return false
}

// loadFonts reads office:font-face-decls.
func (s *styleSet) loadFonts(root *node) {
	decls := root.find("office:font-face-decls")
	if decls == nil {
		return
	}
	for _, f := range decls.kids {
		if f.name != "style:font-face" {
			continue
		}
		name := f.attr("style:name")
		fam := f.attr("svg:font-family")
		s.faces[name] = fam
		if f.attr("style:font-pitch") == "fixed" || isMonoFamily(fam) || isMonoFamily(name) {
			s.monoFace[name] = true
		}
	}
}

// loadStyles reads office:styles and office:automatic-styles below root.
func (s *styleSet) loadStyles(root *node) {
	for _, sect := range []string{"office:styles", "office:automatic-styles"} {
		var walk func(*node)
		walk = func(n *node) {
			for _, k := range n.kids {
				if k.name == sect {
					s.addStyles(k)
				} else if k.name == "office:document" || k.name == "office:document-styles" || k.name == "office:document-content" || k.name == "#document" {
					walk(k)
				}
			}
		}
		walk(root)
	}
}

func (s *styleSet) addStyles(sect *node) {
	for _, k := range sect.kids {
		switch k.name {
		case "style:style":
			st := s.parseStyle(k)
			s.styles[st.family+"/"+st.name] = st
		case "style:default-style":
			st := s.parseStyle(k)
			s.defaults[st.family] = st
			if st.family == "paragraph" && s.lang == "" {
				if tp := k.child("style:text-properties"); tp != nil {
					if l := tp.attr("fo:language"); l != "" && l != "zxx" && l != "none" {
						s.lang = l
						if c := tp.attr("fo:country"); c != "" && c != "none" {
							s.lang += "-" + c
						}
					}
				}
			}
		case "text:list-style":
			s.lists[k.attr("style:name")] = parseListStyle(k)
		}
	}
}

func (s *styleSet) parseStyle(k *node) *style {
	st := &style{
		name:       k.attr("style:name"),
		display:    k.attr("style:display-name"),
		family:     k.attr("style:family"),
		parent:     k.attr("style:parent-style-name"),
		masterPage: k.attr("style:master-page-name"),
		listStyle:  k.attr("style:list-style-name"),
	}
	if v, err := strconv.Atoi(k.attr("style:default-outline-level")); err == nil {
		st.outline = v
	}
	if tp := k.child("style:text-properties"); tp != nil {
		st.text = s.parseTextProps(tp)
	}
	if pp := k.child("style:paragraph-properties"); pp != nil {
		st.breakBefore = pp.attr("fo:break-before") == "page"
		st.breakAfter = pp.attr("fo:break-after") == "page"
		switch pp.attr("fo:text-align") {
		case "start", "left":
			st.align = ast.AlignLeft
		case "center":
			st.align = ast.AlignCenter
		case "end", "right":
			st.align = ast.AlignRight
		}
	}
	return st
}

func (s *styleSet) parseTextProps(tp *node) textProps {
	var p textProps
	switch w := tp.attr("fo:font-weight"); w {
	case "":
	case "bold", "bolder", "600", "700", "800", "900":
		p.put(fBold, true)
	default:
		p.put(fBold, false)
	}
	switch tp.attr("fo:font-style") {
	case "":
	case "italic", "oblique":
		p.put(fItalic, true)
	default:
		p.put(fItalic, false)
	}
	if v := tp.attr("style:text-underline-style"); v != "" {
		p.put(fUnderline, v != "none")
	}
	if v := tp.attr("style:text-line-through-style"); v != "" {
		p.put(fStrike, v != "none")
	}
	if v := tp.attr("style:text-position"); v != "" {
		sup, sub := textPosition(v)
		p.put(fSuper, sup)
		p.put(fSub, sub)
	}
	if v := tp.attr("fo:font-variant"); v != "" {
		p.put(fSmallCaps, v == "small-caps")
	}
	if v := tp.attr("fo:background-color"); v != "" {
		v = strings.ToLower(v)
		p.put(fHighlight, v != "transparent" && v != "#ffffff" && v != "white")
	}
	font := tp.attr("style:font-name")
	fam := tp.attr("fo:font-family")
	switch {
	case font != "":
		p.put(fCode, s.monoFace[font] || isMonoFamily(font) || isMonoFamily(s.faces[font]))
	case fam != "":
		p.put(fCode, isMonoFamily(fam) || tp.attr("style:font-pitch") == "fixed")
	}
	return p
}

// textPosition parses style:text-position ("super 58%", "sub", "33%",
// "-33% 58%", "0%").
func textPosition(v string) (sup, sub bool) {
	f := strings.Fields(v)
	if len(f) == 0 {
		return false, false
	}
	switch f[0] {
	case "super":
		return true, false
	case "sub":
		return false, true
	}
	n, err := strconv.ParseFloat(strings.TrimSuffix(f[0], "%"), 64)
	if err != nil {
		return false, false
	}
	return n > 0, n < 0
}

func parseListStyle(k *node) []listLevel {
	levels := make([]listLevel, 11)
	for i := range levels {
		levels[i] = listLevel{start: 1}
	}
	for _, l := range k.kids {
		lv, err := strconv.Atoi(l.attr("text:level"))
		if err != nil || lv < 1 || lv > 10 {
			continue
		}
		ll := listLevel{start: 1}
		if l.name == "text:list-level-style-number" {
			format := l.attr("style:num-format")
			ll.ordered = format != ""
			ll.style = numFormat(format)
			if v, err := strconv.Atoi(l.attr("text:start-value")); err == nil && v > 0 {
				ll.start = v
			}
		}
		levels[lv] = ll
	}
	return levels
}

func numFormat(f string) ast.NumberStyle {
	switch f {
	case "a":
		return ast.NumberLowerAlpha
	case "A":
		return ast.NumberUpperAlpha
	case "i":
		return ast.NumberLowerRoman
	case "I":
		return ast.NumberUpperRoman
	}
	return ast.NumberDecimal
}

var (
	titleNames    = map[string]bool{"title": true}
	subtitleNames = map[string]bool{"subtitle": true}
	quoteNames    = map[string]bool{"quotations": true, "quote": true, "quotation": true, "block quotation": true, "block text": true, "intense quote": true}
	codeNames     = map[string]bool{"preformatted text": true, "source code": true, "code": true, "source text": true, "html preformatted": true, "code block": true, "listing contents": true}
	figureCaption = map[string]bool{"caption": true, "illustration": true, "figure": true, "drawing": true, "listing": true, "figure caption": true, "image caption": true}
	codeCharNames = map[string]bool{"source text": true, "source code": true, "code": true, "teletype": true, "monospace": true, "verbatim char": true, "html code": true}
)

func (s *styleSet) lookup(family, name string) *style {
	if st, ok := s.styles[family+"/"+name]; ok {
		return st
	}
	return nil
}

// chain returns the style and its ancestors, nearest first.
func (s *styleSet) chain(family, name string) []*style {
	var out []*style
	seen := map[string]bool{}
	for name != "" && !seen[name] && len(out) < 32 {
		seen[name] = true
		st := s.lookup(family, name)
		if st == nil {
			break
		}
		out = append(out, st)
		name = st.parent
	}
	return out
}

func styleKey(st *style) []string {
	return []string{strings.ToLower(decodeName(st.name)), strings.ToLower(st.display)}
}

// para resolves a paragraph style.
func (s *styleSet) para(name string) *resolved {
	key := "paragraph/" + name
	if r, ok := s.cache[key]; ok {
		return r
	}
	r := &resolved{}
	chain := s.chain("paragraph", name)
	roleSet := false
	for _, st := range chain {
		r.text = r.text.merge(st.text)
		if r.align == ast.AlignDefault {
			r.align = st.align
		}
		if r.listStyle == "" {
			r.listStyle = st.listStyle
		}
		if st == chain[0] {
			r.breakBefore, r.breakAfter, r.masterPage = st.breakBefore, st.breakAfter, st.masterPage
		}
		if roleSet {
			continue
		}
		for _, k := range styleKey(st) {
			if role, caption, level, ok := roleOf(k); ok {
				r.role, r.caption, r.level = role, caption, level
				roleSet = true
				break
			}
		}
	}
	if !roleSet {
		// A paragraph style with an outline level makes headings even
		// when the document uses text:p for them.
		for _, st := range chain {
			if st.outline > 0 && st.outline <= 10 {
				r.role, r.level = roleHeading, st.outline
				break
			}
		}
	}
	if d := s.defaults["paragraph"]; d != nil {
		r.text = r.text.merge(d.text)
	}
	s.cache[key] = r
	return r
}

// roleOf classifies a paragraph style by its (display) name.
func roleOf(k string) (role paraRole, caption string, level int, ok bool) {
	switch {
	case k == "":
		return 0, "", 0, false
	case titleNames[k]:
		return roleTitle, "", 0, true
	case subtitleNames[k]:
		return roleSubtitle, "", 0, true
	case quoteNames[k]:
		return roleQuote, "", 0, true
	case codeNames[k]:
		return roleCode, "", 0, true
	case k == "table" || k == "table caption":
		return roleCaption, "table", 0, true
	case figureCaption[k]:
		return roleCaption, "figure", 0, true
	case strings.HasPrefix(k, "heading "):
		if lv, err := strconv.Atoi(strings.TrimPrefix(k, "heading ")); err == nil && lv >= 1 && lv <= 10 {
			return roleHeading, "", lv, true
		}
	}
	return 0, "", 0, false
}

// text resolves a character (text) style.
func (s *styleSet) text(name string) textProps {
	key := "text/" + name
	if r, ok := s.cache[key]; ok {
		return r.text
	}
	var p textProps
	for _, st := range s.chain("text", name) {
		p = p.merge(st.text)
		for _, k := range styleKey(st) {
			if codeCharNames[k] && p.set&fCode == 0 {
				p.put(fCode, true)
			}
			if (k == "emphasis") && p.set&fItalic == 0 {
				p.put(fItalic, true)
			}
			if (k == "strong emphasis" || k == "strong") && p.set&fBold == 0 {
				p.put(fBold, true)
			}
		}
	}
	s.cache[key] = &resolved{text: p}
	return p
}

// cellAlign returns the text alignment of a paragraph style.
func (s *styleSet) cellAlign(name string) ast.Align {
	return s.para(name).align
}

// listLevels returns the levels of a list style.
func (s *styleSet) listLevels(name string) []listLevel {
	return s.lists[name]
}
