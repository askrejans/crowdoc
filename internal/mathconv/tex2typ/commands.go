package tex2typ

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// boxStart opens a framed box around display-style math; the text edges are
// set to the glyph bounds so tall content such as fractions stays inside.
const boxStart = `#box(stroke: 0.5pt, inset: 3pt, text(top-edge: "bounds", bottom-edge: "bounds", `

// parseCommand converts a control sequence whose token was just consumed.
func (p *parser) parseCommand(name string) *node {
	switch name {
	case "frac", "dfrac", "tfrac", "cfrac":
		if name == "cfrac" {
			p.rawOptArg()
		}
		num, den := p.parseArg(), p.parseArg()
		return styled(name, call("frac", num, den))
	case "binom", "dbinom", "tbinom":
		n, k := p.parseArg(), p.parseArg()
		return styled(name, call("binom", n, k))
	case "genfrac":
		return p.parseGenfrac()
	case "sqrt":
		idx := p.parseOptArg()
		rad := p.parseArg()
		if idx != nil && !idx.isEmpty() {
			return call("root", idx, rad)
		}
		return call("sqrt", rad)
	case "left":
		return p.parseLeftRight()
	case "begin":
		return p.parseEnv()
	case "text", "textrm", "textnormal", "textup", "textmd", "mbox", "hbox", "textsc", "mathtext":
		return p.parseTextArg("")
	case "textit", "textsl", "emph":
		return p.parseTextArg("italic")
	case "textbf":
		return p.parseTextArg("bold")
	case "textsf":
		return p.parseTextArg("sans")
	case "texttt":
		return p.parseTextArg("mono")
	case "intertext", "shortintertext":
		// Text between the rows of an alignment gets a line of its own.
		return seq(p.parseTextArg(""), &node{kind: nBreak})
	case "operatorname":
		limits := p.consumeStar()
		content := joinLetters(p.parseArg())
		return opNode(content, limits)
	case "mathop":
		return opNode(p.parseArg(), false)
	case "overline", "underline":
		return call(name, p.parseArg())
	case "overbrace", "underbrace", "overbracket", "underbracket", "overparen", "underparen":
		return call(name, p.parseArg())
	case "overset", "stackrel":
		top := p.parseArg()
		return &node{kind: nAttach, base: call("limits", p.parseArg()), sup: top}
	case "underset":
		bot := p.parseArg()
		return &node{kind: nAttach, base: call("limits", p.parseArg()), sub: bot}
	case "boxed":
		return embed(boxStart, call("display", p.parseArg()), "))")
	case "fbox", "framebox":
		p.rawOptArg()
		return embed(boxStart, p.parseTextArg(""), "))")
	case "colorbox":
		col := p.parseColor()
		body := p.parseTextArg("")
		if col == "" {
			return body
		}
		return embed("#box(fill: "+col+", inset: 2pt, ", body, ")")
	case "phantom":
		return embed("#hide(", p.parseArg(), ")")
	case "hphantom":
		return embed("#box(height: 0pt, hide(", p.parseArg(), "))")
	case "vphantom":
		return embed("#box(width: 0pt, hide(", p.parseArg(), "))")
	case "mathstrut", "strut":
		return embed("#box(width: 0pt, hide(", atom(`\(`, eNone, eNone), "))")
	case "smash", "mathclap", "mathllap", "mathrlap", "clap", "llap", "rlap", "ensuremath",
		"vcenter":
		p.rawOptArg()
		return p.parseArg()
	case "cancel":
		return call("cancel", p.parseArg())
	case "bcancel":
		return call("cancel", p.parseArg()).with(code("inverted", "#true"))
	case "xcancel":
		return call("cancel", p.parseArg()).with(code("cross", "#true"))
	case "cancelto":
		to := p.parseArg()
		p.warn(`\cancelto drawn as a struck-out term with a superscript`)
		return &node{kind: nAttach, base: call("cancel", p.parseArg()), sup: to}
	case "sout":
		return call("cancel", p.parseArg())
	case "not":
		return p.parseNot()
	case "pmod":
		return seq(word("quad"), atom(`\(`, eNone, eNone), word("mod"), p.parseArg(), atom(`\)`, eNone, eNone))
	case "pod":
		return seq(word("quad"), atom(`\(`, eNone, eNone), p.parseArg(), atom(`\)`, eNone, eNone))
	case "mod":
		return seq(word("quad"), word("mod"), word("thin"), p.parseArg())
	case "bmod":
		return word("mod")
	case "hspace", "hskip", "kern", "mkern", "mskip", "mspace":
		if name == "hspace" {
			p.consumeStar()
		}
		if d := p.parseDimen(); d != "" {
			return codeAtom("#h(" + d + ")")
		}
		return nil
	case "label":
		if l := p.rawArg(); p.label == "" {
			p.label = l
		} else {
			p.warn(`additional \label{` + l + `} dropped`)
		}
		return nil
	case "tag":
		p.consumeStar()
		p.warn(`\tag{` + p.rawArg() + `} replaced by automatic numbering`)
		return nil
	case "nonumber", "notag":
		p.noNumbers++
		return nil
	case "eqref", "ref":
		ref := p.rawArg()
		p.warn(`\` + name + `{` + ref + `} kept as text`)
		if name == "eqref" {
			return strAtom("(" + ref + ")")
		}
		return strAtom(ref)
	case "color":
		// Only reached where a whole list cannot follow, e.g. x^\color{red}.
		p.parseColor()
		return nil
	case "textcolor":
		col := p.parseColor()
		return colored(col, p.parseArg())
	case "substack":
		return p.parseSubstack()
	case "sideset":
		return p.parseSideset()
	case "prescript":
		sup, sub := p.parseArg(), p.parseArg()
		base := p.parseArg()
		return call("attach", base).with(namedArg{name: "tl", content: sup}, namedArg{name: "bl", content: sub})
	case "dots":
		return p.dotsKind()
	case "iff":
		return seq(word("thick"), word("arrow.l.r.double.long"), word("thick"))
	case "implies":
		return seq(word("thick"), word("arrow.r.double.long"), word("thick"))
	case "impliedby":
		return seq(word("thick"), word("arrow.l.double.long"), word("thick"))
	case "colon":
		return call("class", strAtom("punctuation"), word("colon"))
	case "#", "$", "&", "_":
		return atom(`\`+name, eNone, eNone)
	case "%":
		return atom("%", eNone, eNone)
	case "-", "*":
		return nil
	case "\\", "cr", "newline":
		return &node{kind: nBreak}
	case "TeX", "LaTeX":
		return strAtom(name)
	case "And":
		return atom(`\&`, eNone, eNone)
	case "ldotp":
		return atom(".", eDot, eDot)
	case "idotsint":
		return opNode(seq(word("integral"), word("dots.h.c"), word("integral")), false)
	case "mathchoice":
		d := p.parseArg()
		p.parseArg()
		p.parseArg()
		p.parseArg()
		return d
	case "unicode":
		return p.parseUnicode()
	case "newcommand", "renewcommand", "providecommand", "DeclareMathOperator", "def",
		"let", "newenvironment", "setlength", "arraystretch", "renewenvironment":
		p.skipDefinition(name)
		return nil
	}
	if s, ok := symbols[name]; ok {
		return symNode(s)
	}
	if s, ok := bigOps[name]; ok {
		n := word(s.typ)
		n.cls = clsOp
		return n
	}
	if functions[name] {
		n := word(name)
		n.cls = clsOp
		return n
	}
	if limits, ok := extraFunctions[name]; ok {
		text := name
		switch name {
		case "injlim", "varinjlim":
			text = "inj lim"
		case "projlim", "varprojlim":
			text = "proj lim"
		}
		return opNode(strAtom(text), limits)
	}
	if a, ok := accents[name]; ok {
		arg := p.parseArg()
		if strings.Contains(a, ".") {
			return call("accent", arg, word(a))
		}
		return call(a, arg)
	}
	if f, ok := fontCmds[name]; ok {
		return applyFont(f, p.parseArg())
	}
	if f, ok := fontSwitches[name]; ok {
		// A declaration used where only one item fits, e.g. x_\rm d.
		return applyFont(fontCmds[f], p.parseArg())
	}
	if sp, ok := spaces[name]; ok {
		if strings.HasPrefix(sp, "#") {
			return codeAtom(sp)
		}
		return word(sp)
	}
	if a, ok := xArrows[name]; ok {
		return p.parseXArrow(a)
	}
	if b, ok := bigDelims[name]; ok {
		d := p.parseDelim()
		if d == "" {
			return nil
		}
		n := call("lr", word(d))
		n.with(code("size", "#"+b.size))
		if b.cls != "" {
			return call("class", strAtom(b.cls), n)
		}
		return n
	}
	if cls, ok := mathClasses[name]; ok {
		return call("class", strAtom(cls), p.parseArg())
	}
	if _, ok := styleCmds[name]; ok {
		return nil
	}
	if ignoredCmds[name] {
		return nil
	}
	if strings.HasPrefix(name, "var") {
		if s, ok := symbols[name[3:]]; ok && name[3] >= 'A' && name[3] <= 'Z' {
			return call("italic", word(s.typ))
		}
	}
	if acc, ok := textAccents[name]; ok {
		return p.textAccentNode(acc)
	}
	if s, ok := textSymbols[name]; ok && isASCIILetter(name[0]) {
		return strAtom(s)
	}
	return p.unknown(name)
}

func styled(name string, n *node) *node {
	switch name[0] {
	case 'd', 'c':
		return call("display", n)
	case 't':
		return call("inline", n)
	}
	return n
}

func symNode(s sym) *node {
	var n *node
	if s.typ == "" {
		n = rawChar(string(s.ch))
	} else {
		n = word(s.typ)
	}
	n.cls = s.cls
	if s.typ == "bar.v" || s.typ == "bar.v.double" {
		// Typst spaces lone bars like relations; keep the TeX class.
		cls := "normal"
		switch s.cls {
		case clsOpen:
			cls = "opening"
		case clsClose:
			cls = "closing"
		}
		return call("class", strAtom(cls), n)
	}
	return n
}

// opNode builds op(…) for an operator name, joining a lone string.
func opNode(content *node, limits bool) *node {
	u := content.unwrap()
	if u.isEmpty() {
		return seq()
	}
	n := call("op", u)
	if limits {
		n.with(code("limits", "#true"))
	}
	return n
}

// unknown renders an unsupported command as an upright operator name.
func (p *parser) unknown(name string) *node {
	if name == "" {
		return nil
	}
	if !isASCIILetter(name[0]) {
		p.warn(`unknown command \` + name + ` kept as a character`)
		return rawChar(name)
	}
	p.warn(`unknown command \` + name)
	return call("op", strAtom(name))
}

func (p *parser) parseGenfrac() *node {
	left := strings.TrimSpace(p.rawArg())
	right := strings.TrimSpace(p.rawArg())
	thick := strings.TrimSpace(p.rawArg())
	style := strings.TrimSpace(p.rawArg())
	num, den := p.parseArg(), p.parseArg()
	var n *node
	if thick != "" {
		if d, ok := parseDimenText(thick); ok && (d == "0pt" || d == "0em" || d == "0mm") {
			n = mat("mat", [][]*node{{num}, {den}}, code("delim", "#none"))
		}
	}
	if n == nil {
		n = call("frac", num, den)
	}
	if left != "" || right != "" {
		ld, rd := genfracDelim(left), genfracDelim(right)
		n = call("lr", seq(delimAtom(ld), n, delimAtom(rd)))
	}
	switch style {
	case "0":
		return call("display", n)
	case "1":
		return call("inline", n)
	case "2":
		return call("script", n)
	case "3":
		return call("sscript", n)
	}
	return n
}

func genfracDelim(s string) string {
	if s == "" {
		return ""
	}
	if s[0] == '\\' {
		return delimCmds[s[1:]]
	}
	r := []rune(s)[0]
	return delimChars[r]
}

// parseSubstack parses \substack{a \\ b} into lines for a limit.
func (p *parser) parseSubstack() *node {
	p.skipSpace()
	if p.peek().kind != tOpen {
		return p.parseArg()
	}
	p.next()
	var items []*node
	for {
		row := p.parseList(stopClose | stopRow)
		if len(items) > 0 {
			items = append(items, &node{kind: nBreak})
		}
		items = append(items, row)
		if isRowCmd(p.peek()) {
			p.next()
			p.skipRowOpts()
			continue
		}
		break
	}
	if p.peek().kind == tClose {
		p.next()
	} else {
		p.warn("missing '}' inserted")
	}
	return trimBreaks(seq(items...))
}

// trimBreaks removes leading and trailing line breaks and empty rows.
func trimBreaks(n *node) *node {
	k := n.kids
	for len(k) > 0 && (k[0].kind == nBreak || k[0].isEmpty()) {
		k = k[1:]
	}
	for len(k) > 0 && (k[len(k)-1].kind == nBreak || k[len(k)-1].isEmpty()) {
		k = k[:len(k)-1]
	}
	n.kids = k
	return n
}

// parseSideset handles \sideset{_a^b}{_c^d}\sum.
func (p *parser) parseSideset() *node {
	pre, post := p.parseArg(), p.parseArg()
	op := p.parseItem()
	if op == nil {
		op = seq()
	}
	op = p.parseScripts(op)
	c := call("attach", op)
	if op.kind == nAttach {
		c.kids[0] = op.base
		if op.sub != nil {
			c.with(namedArg{name: "b", content: op.sub})
		}
		if op.sup != nil {
			c.with(namedArg{name: "t", content: op.sup})
		}
	}
	if a := scriptsOf(pre); a != nil {
		if a.sub != nil {
			c.with(namedArg{name: "bl", content: a.sub})
		}
		if a.sup != nil {
			c.with(namedArg{name: "tl", content: a.sup})
		}
	}
	if a := scriptsOf(post); a != nil {
		if a.sub != nil {
			c.with(namedArg{name: "br", content: a.sub})
		}
		if a.sup != nil {
			c.with(namedArg{name: "tr", content: a.sup})
		}
	}
	return c
}

func scriptsOf(n *node) *node {
	u := n.unwrap()
	if u != nil && u.kind == nAttach {
		return u
	}
	return nil
}

// parseXArrow handles \xrightarrow[below]{above} and friends.
func (p *parser) parseXArrow(arrow string) *node {
	below := p.parseOptArg()
	above := p.parseArg()
	base := call("stretch", word(arrow))
	c := call("attach", base)
	if !above.isEmpty() {
		c.with(namedArg{name: "t", content: above})
	}
	if below != nil && !below.isEmpty() {
		c.with(namedArg{name: "b", content: below})
	}
	if len(c.named()) == 0 {
		base.with(code("size", "#1.5em"))
		return base
	}
	return c
}

// parseUnicode handles MathJax's \unicode{x2208} / \unicode{8712}.
func (p *parser) parseUnicode() *node {
	p.rawOptArg()
	s := strings.TrimSpace(p.rawArg())
	var v int64
	base := int64(10)
	if strings.HasPrefix(s, "x") || strings.HasPrefix(s, "X") {
		base, s = 16, s[1:]
	}
	if s == "" || len(s) > 8 {
		p.warn(`invalid \unicode argument`)
		return nil
	}
	for _, c := range s {
		d := int64(-1)
		switch {
		case c >= '0' && c <= '9':
			d = int64(c - '0')
		case base == 16 && c >= 'a' && c <= 'f':
			d = int64(c-'a') + 10
		case base == 16 && c >= 'A' && c <= 'F':
			d = int64(c-'A') + 10
		}
		if d < 0 {
			p.warn(`invalid \unicode argument`)
			return nil
		}
		v = v*base + d
	}
	if v <= 0x20 || v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF) || (v >= 0x7F && v < 0xA0) {
		p.warn(`invalid \unicode argument`)
		return nil
	}
	return rawChar(string(rune(v)))
}

// skipDefinition drops a macro definition; macros are not expanded.
func (p *parser) skipDefinition(name string) {
	p.warn(`\` + name + ` ignored (macros are not expanded)`)
	switch name {
	case "def":
		p.skipSpace()
		if p.peek().kind == tCmd {
			p.next()
		}
		for t := p.peek(); t.kind != tEOF && t.kind != tOpen; t = p.peek() {
			p.next()
		}
		p.rawArg()
	case "let":
		p.skipSpace()
		p.next()
		p.skipSpace()
		if t := p.peek(); t.kind == tChar && t.r == '=' {
			p.next()
		}
		p.skipSpace()
		p.next()
	case "newcommand", "renewcommand", "providecommand", "DeclareMathOperator":
		p.consumeStar()
		p.rawArg()
		for {
			if _, ok := p.rawOptArg(); !ok {
				break
			}
		}
		p.rawArg()
	case "newenvironment", "renewenvironment":
		p.rawArg()
		for {
			if _, ok := p.rawOptArg(); !ok {
				break
			}
		}
		p.rawArg()
		p.rawArg()
	case "setlength":
		p.rawArg()
		p.rawArg()
	}
}

// textAccentNode handles text accents such as \"a used in math.
func (p *parser) textAccentNode(mark rune) *node {
	s := strings.TrimSpace(p.rawArg())
	if s == "" {
		return nil
	}
	rs := []rune(s)
	first := norm.NFC.String(string(rs[0]) + string(mark))
	n := accented(first)
	if n == nil {
		n = rawChar(first)
	}
	out := []*node{n}
	for _, r := range rs[1:] {
		out = append(out, rawChar(string(r)))
	}
	return seq(out...)
}
