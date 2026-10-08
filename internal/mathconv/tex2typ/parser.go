package tex2typ

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxDepth bounds structural nesting. Typst refuses deeply nested markup, and
// each LaTeX level can open a few Typst levels (call, script, wrapper).
const maxDepth = 40

type stopSet uint16

const (
	stopClose   stopSet = 1 << iota // '}'
	stopAlign                       // '&'
	stopRow                         // '\\' or \cr
	stopEnd                         // \end
	stopRight                       // \right
	stopMiddle                      // \middle
	stopBracket                     // ']' closing an optional argument
	stopDollar                      // '$' closing math inside \text
	stopParen                       // \) closing math inside \text
)

type parser struct {
	src   string
	toks  []token
	pos   int
	depth int // recursion depth, bounded by maxDepth
	items int // nesting of items; 0 is the top level of the formula

	warns []string
	seen  map[string]bool

	label     string
	noNumbers int  // \nonumber / \notag count
	rows      int  // rows of a numbered top-level environment
	starred   bool // a starred (unnumbered) environment
}

func (p *parser) warn(msg string) {
	if p.seen == nil {
		p.seen = map[string]bool{}
	}
	if !p.seen[msg] {
		p.seen[msg] = true
		p.warns = append(p.warns, msg)
	}
}

func (p *parser) peek() token {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return token{kind: tEOF, pos: len(p.src)}
}

func (p *parser) next() token {
	t := p.peek()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return t
}

func (p *parser) skipSpace() {
	for p.pos < len(p.toks) && p.toks[p.pos].kind == tSpace {
		p.pos++
	}
}

func (p *parser) peekCmd(name string) bool {
	t := p.peek()
	return t.kind == tCmd && t.val == name
}

func isRowCmd(t token) bool {
	return t.kind == tCmd && (t.val == "\\" || t.val == "cr" || t.val == "newline")
}

// parseList parses items until EOF or a token in stop, which is left unread.
func (p *parser) parseList(stop stopSet) *node {
	var items []*node
	for {
		p.skipSpace()
		t := p.peek()
		switch t.kind {
		case tEOF:
			return seq(items...)
		case tClose:
			if stop&stopClose != 0 {
				return seq(items...)
			}
			p.next()
			p.warn("unbalanced '}' ignored")
			continue
		case tAlign:
			if stop&stopAlign != 0 {
				return seq(items...)
			}
			p.next()
			if p.items == 0 {
				items = append(items, &node{kind: nAlign})
			} else {
				p.warn("misplaced '&' dropped")
			}
			continue
		case tChar:
			if (t.r == ']' && stop&stopBracket != 0) || (t.r == '$' && stop&stopDollar != 0) {
				return seq(items...)
			}
		case tCmd:
			if done, ok := p.listCommand(t.val, stop, &items); ok {
				if done {
					return seq(items...)
				}
				continue
			}
		}
		item := p.parseItem()
		if item == nil {
			continue
		}
		items = append(items, p.parseScripts(item))
	}
}

// listCommand handles commands that act on the surrounding list: terminators,
// generalised fractions and declarations that apply to the rest of the group.
// ok reports whether the command was handled; done whether the list ends.
func (p *parser) listCommand(name string, stop stopSet, items *[]*node) (done, ok bool) {
	switch name {
	case "\\", "cr", "newline":
		if stop&stopRow != 0 {
			return true, true
		}
		p.next()
		p.skipRowOpts()
		*items = append(*items, &node{kind: nBreak})
		return false, true
	case "end":
		if stop&stopEnd != 0 {
			return true, true
		}
		p.next()
		p.warn(`unmatched \end{` + p.rawArg() + `} ignored`)
		return false, true
	case "right":
		if stop&stopRight != 0 {
			return true, true
		}
		p.next()
		p.warn(`\right without \left`)
		*items = append(*items, delimAtom(p.parseDelim()))
		return false, true
	case "middle":
		if stop&stopMiddle != 0 {
			return true, true
		}
		p.next()
		p.warn(`\middle outside \left … \right`)
		*items = append(*items, delimAtom(p.parseDelim()))
		return false, true
	case ")":
		if stop&stopParen != 0 {
			return true, true
		}
	case "over", "choose", "atop", "above", "brace", "brack":
		p.next()
		if name == "above" {
			p.parseDimen()
		}
		if p.tooDeep() {
			return false, true
		}
		den := p.parseList(stop)
		p.depth--
		*items = []*node{genFrac(name, seq(*items...), den)}
		return false, true
	case "color":
		p.next()
		col := p.parseColor()
		if p.tooDeep() {
			return false, true
		}
		rest := p.parseList(stop)
		p.depth--
		*items = append(*items, colored(col, rest))
		return false, true
	}
	fn, isStyle := styleCmds[name]
	font, isFont := fontSwitches[name]
	if !isStyle && !isFont {
		return false, false
	}
	// A declaration applies to the rest of the list.
	p.next()
	if p.tooDeep() {
		return false, true
	}
	rest := p.parseList(stop)
	p.depth--
	if isStyle {
		*items = append(*items, call(fn, rest))
	} else {
		*items = append(*items, applyFont(fontCmds[font], rest))
	}
	return false, true
}

// tooDeep enters one recursion level, or reports (with a warning) that the
// formula is nested too deeply, in which case the caller skips the level.
func (p *parser) tooDeep() bool {
	if p.depth >= maxDepth {
		p.warn("formula nested too deeply; inner content dropped")
		return true
	}
	p.depth++
	return false
}

func genFrac(name string, num, den *node) *node {
	switch name {
	case "choose":
		return call("binom", num, den)
	case "atop":
		return mat("mat", [][]*node{{num}, {den}}, code("delim", "#none"))
	case "brace":
		return call("lr", seq(word("brace.l"), mat("mat", [][]*node{{num}, {den}}, code("delim", "#none")), word("brace.r")))
	case "brack":
		return mat("mat", [][]*node{{num}, {den}}, code("delim", `"["`))
	}
	return call("frac", num, den)
}

// parseItem parses one item: a character, a group or a command with its
// arguments. It returns nil for commands that produce nothing.
func (p *parser) parseItem() *node {
	t := p.peek()
	if p.depth >= maxDepth {
		p.skipItem()
		p.warn("formula nested too deeply; inner content dropped")
		return word("dots.h")
	}
	p.depth++
	p.items++
	defer func() { p.depth--; p.items-- }()
	switch t.kind {
	case tChar:
		return p.parseChar()
	case tOpen:
		return p.parseGroup()
	case tCmd:
		p.next()
		return p.parseCommand(t.val)
	case tTilde:
		p.next()
		return word("space.nobreak")
	case tHash:
		p.next()
		return atom(`\#`, eNone, eNone)
	case tPrime, tSup, tSub:
		// A script without a base, as in {}^{14}C or a leading prime.
		return p.parseScripts(seq())
	case tAlign:
		p.next()
		p.warn("misplaced '&' dropped")
		return nil
	case tClose:
		p.next()
		p.warn("unbalanced '}' ignored")
		return nil
	}
	p.next()
	return nil
}

// skipItem consumes one item without interpreting it, keeping groups and
// environments balanced. Used beyond maxDepth.
func (p *parser) skipItem() {
	t := p.next()
	switch {
	case t.kind == tOpen:
		p.skipBalanced(func(t token) int {
			switch t.kind {
			case tOpen:
				return 1
			case tClose:
				return -1
			}
			return 0
		})
	case t.kind == tCmd && t.val == "left":
		p.skipBalanced(func(t token) int {
			if t.kind == tCmd && t.val == "left" {
				return 1
			} else if t.kind == tCmd && t.val == "right" {
				return -1
			}
			return 0
		})
		p.parseDelim()
	case t.kind == tCmd && t.val == "begin":
		p.skipBalanced(func(t token) int {
			if t.kind == tCmd && t.val == "begin" {
				return 1
			} else if t.kind == tCmd && t.val == "end" {
				return -1
			}
			return 0
		})
		p.rawArg()
	}
}

func (p *parser) skipBalanced(delta func(token) int) {
	for depth := 1; ; {
		t := p.next()
		if t.kind == tEOF {
			return
		}
		if depth += delta(t); depth == 0 {
			return
		}
	}
}

func (p *parser) parseChar() *node {
	t := p.next()
	r := t.r
	if isDigit(r) {
		start, end := t.pos, t.pos+1
		dot := false
		// Only directly adjacent characters: a comment between two digits
		// separates them in the token stream but not in the source.
		for p.pos < len(p.toks) {
			u := p.toks[p.pos]
			if u.pos != end {
				break
			}
			if u.kind == tChar && isDigit(u.r) {
				end = u.pos + 1
				p.pos++
				continue
			}
			if !dot && u.kind == tChar && u.r == '.' && p.pos+1 < len(p.toks) {
				if v := p.toks[p.pos+1]; v.kind == tChar && isDigit(v.r) && v.pos == end+1 {
					dot = true
					end = v.pos + 1
					p.pos += 2
					continue
				}
			}
			break
		}
		return word(p.src[start:end])
	}
	return charAtom(t)
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// charAtom converts a literal character in math mode.
func charAtom(t token) *node {
	r := t.r
	switch r {
	case '+', '-', '=', '<', '>', '!', '?', '*', ',', ';', ':':
		return atom(t.val, ePunct, ePunct)
	case '.':
		return atom(".", eDot, eDot)
	case '|':
		// Typst spaces a lone bar like a relation; in TeX it is ordinary.
		return call("class", strAtom("normal"), word("bar.v"))
	case '(', ')', '[', ']', '{', '}', '/', '"', '@', '`', '$', '\\', '#', '&', '_', '^', '\'', '~', '%':
		return atom(`\`+string(r), eNone, eNone)
	}
	if isWordRune(r) {
		if n := accented(t.val); n != nil {
			return n
		}
		n := word(t.val)
		n.letter = unicode.IsLetter(r)
		return n
	}
	if r < 0x80 {
		// White space and control characters carry no meaning in math.
		return seq()
	}
	return rawChar(t.val)
}

// combiningAccents maps combining marks to Typst accents; math fonts often
// position a raw combining mark poorly over italic letters.
var combiningAccents = map[rune]string{
	0x0300: "grave", 0x0301: "acute", 0x0302: "hat", 0x0303: "tilde", 0x0304: "macron",
	0x0306: "breve", 0x0307: "dot", 0x0308: "dot.double", 0x030A: "circle", 0x030C: "caron",
	0x20D7: "arrow", 0x20D6: "arrow.l", 0x20E1: "arrow.l.r", 0x20DB: "dot.triple",
	0x20DC: "dot.quad",
}

// accented converts a letter followed by known combining accents, or
// returns nil. Precomposed letters such as ä pass through unchanged.
func accented(s string) *node {
	rs := []rune(s)
	if len(rs) < 2 || !unicode.IsLetter(rs[0]) {
		return nil
	}
	n := word(string(rs[0]))
	for _, m := range rs[1:] {
		a, ok := combiningAccents[m]
		if !ok {
			return nil
		}
		if strings.Contains(a, ".") {
			n = call("accent", n, word(a))
		} else {
			n = call(a, n)
		}
	}
	return n
}

// rawChar writes a non-ASCII symbol. Opening and closing brackets are
// escaped because Typst pairs them, which breaks call arguments; radical
// signs are escaped because Typst reads them as root syntax.
func rawChar(s string) *node {
	r, _ := utf8.DecodeRuneInString(s)
	if r < 0x80 {
		return charAtom(token{kind: tChar, r: r, val: s})
	}
	if isWordRune(r) {
		return word(s)
	}
	if unicode.Is(unicode.Ps, r) || unicode.Is(unicode.Pe, r) || r == '√' || r == '∛' || r == '∜' ||
		(r >= 0x231C && r <= 0x231F) || r == 0x23B0 || r == 0x23B1 {
		return atom(`\`+s, eNone, eNone)
	}
	return atom(s, eNone, eNone)
}

// parseGroup parses a braced group appearing as an item.
func (p *parser) parseGroup() *node {
	g := p.groupBody()
	u := g.unwrap()
	if u.kind == nAtom && (u.l == ePunct || u.l == eDot || u.cls == clsBin || u.cls == clsRel || u.cls == clsOp) {
		// {=} or {,}: TeX turns the group into an ordinary atom.
		return call("class", strAtom("normal"), u)
	}
	return g
}

// groupBody parses "{ … }" (the opening brace is next) and returns the list.
func (p *parser) groupBody() *node {
	p.next()
	list := p.parseList(stopClose)
	if p.peek().kind == tClose {
		p.next()
	} else {
		p.warn("missing '}' inserted")
	}
	return list
}

// parseArg parses a mandatory argument: a braced group or a single token.
func (p *parser) parseArg() *node {
	p.skipSpace()
	t := p.peek()
	switch t.kind {
	case tOpen:
		if p.depth >= maxDepth {
			p.skipItem()
			p.warn("formula nested too deeply; inner content dropped")
			return word("dots.h")
		}
		p.depth++
		defer func() { p.depth-- }()
		return p.groupBody()
	case tEOF, tClose, tAlign:
		p.warn("missing argument")
		return seq()
	case tCmd:
		if isRowCmd(t) || t.val == "end" || t.val == "right" || t.val == "middle" {
			p.warn("missing argument")
			return seq()
		}
	case tChar:
		// A single character, never a whole number: \frac12 is 1 over 2.
		p.next()
		return charAtom(t)
	}
	n := p.parseItem()
	if n == nil {
		return seq()
	}
	return n
}

// parseOptArg parses "[ … ]" if present; nil otherwise.
func (p *parser) parseOptArg() *node {
	p.skipSpace()
	t := p.peek()
	if t.kind != tChar || t.r != '[' {
		return nil
	}
	p.next()
	list := p.parseList(stopBracket | stopClose)
	if t := p.peek(); t.kind == tChar && t.r == ']' {
		p.next()
	} else {
		p.warn("missing ']' inserted")
	}
	return list
}

// rawArg returns the source text of a braced argument (or a single token)
// without interpreting it.
func (p *parser) rawArg() string {
	p.skipSpace()
	t := p.peek()
	if t.kind != tOpen {
		switch t.kind {
		case tEOF, tClose:
			return ""
		case tCmd:
			p.next()
			return `\` + t.val
		}
		p.next()
		return t.val
	}
	p.next()
	start := t.pos + 1
	for depth := 1; ; {
		u := p.next()
		switch u.kind {
		case tEOF:
			p.warn("missing '}' inserted")
			return strings.ToValidUTF8(strings.TrimSpace(p.src[start:]), "\uFFFD")
		case tOpen:
			depth++
		case tClose:
			if depth--; depth == 0 {
				return strings.ToValidUTF8(strings.TrimSpace(p.src[start:u.pos]), "\uFFFD")
			}
		}
	}
}

// rawOptArg returns the source text of "[ … ]" if present.
func (p *parser) rawOptArg() (string, bool) {
	p.skipSpace()
	t := p.peek()
	if t.kind != tChar || t.r != '[' {
		return "", false
	}
	p.next()
	start := t.pos + 1
	for depth := 0; ; {
		u := p.next()
		switch {
		case u.kind == tEOF:
			return strings.TrimSpace(p.src[start:]), true
		case u.kind == tOpen:
			depth++
		case u.kind == tClose:
			depth--
		case u.kind == tChar && u.r == ']' && depth <= 0:
			return strings.TrimSpace(p.src[start:u.pos]), true
		}
	}
}

func (p *parser) consumeStar() bool {
	if t := p.peek(); t.kind == tChar && t.r == '*' {
		p.next()
		return true
	}
	return false
}

// skipRowOpts drops the "*" and "[dimen]" that may follow \\.
func (p *parser) skipRowOpts() {
	p.consumeStar()
	if t := p.peek(); t.kind == tChar && t.r == '[' {
		save := p.pos
		raw, _ := p.rawOptArg()
		if _, ok := parseDimenText(raw); !ok {
			p.pos = save
		}
	}
}

// parseScripts attaches any following ^, _, primes and \limits to base.
func (p *parser) parseScripts(base *node) *node {
	var att *node
	limits, nested := 0, 0
	get := func() *node {
		if att == nil {
			att = &node{kind: nAttach, base: base}
		}
		return att
	}
loop:
	for {
		save := p.pos
		p.skipSpace()
		t := p.peek()
		switch {
		case t.kind == tCmd && (t.val == "limits" || t.val == "nolimits" || t.val == "displaylimits"):
			p.next()
			switch t.val {
			case "limits":
				limits = 1
			case "nolimits":
				limits = -1
			default:
				limits = 0
			}
		case t.kind == tPrime:
			p.next()
			a := get()
			if a.sup != nil {
				p.warn("prime after superscript")
			}
			a.primes++
		case t.kind == tSup || t.kind == tSub:
			p.next()
			arg := p.parseScriptArg()
			a := get()
			if (t.kind == tSup && a.sup != nil) || (t.kind == tSub && a.sub != nil) {
				p.warn("double script")
				if nested++; nested > 8 {
					continue
				}
				att = &node{kind: nAttach, base: a}
				a = att
			}
			if t.kind == tSup {
				a.sup = arg
			} else {
				a.sub = arg
			}
		default:
			p.pos = save
			break loop
		}
	}
	if att == nil {
		return base
	}
	return p.finishAttach(att, limits)
}

func (p *parser) finishAttach(a *node, limits int) *node {
	// f^{\prime\prime} is f'' and x^\circ is a degree sign.
	if s := a.sup.unwrap(); s != nil {
		if n := countPrimes(s); n > 0 {
			a.primes += int32(n)
			a.sup = nil
		}
	}
	var after *node
	if s := a.sup.unwrap(); s != nil && s.kind == nAtom && s.s == "compose" && a.sub == nil {
		a.sup = nil
		after = word("degree")
	}
	// \overbrace{…}^{label} and \underbrace{…}_{label}.
	if b := a.base.unwrap(); b != nil && b.kind == nCall && len(b.kids) == 1 {
		switch b.s {
		case "overbrace", "overbracket", "overparen", "overshell":
			if a.sup != nil {
				b.kids = append(b.kids, a.sup)
				a.sup = nil
			}
		case "underbrace", "underbracket", "underparen", "undershell":
			if a.sub != nil {
				b.kids = append(b.kids, a.sub)
				a.sub = nil
			}
		}
	}
	var out *node
	if a.sub == nil && a.sup == nil && a.primes == 0 {
		out = a.base
	} else {
		if limits != 0 && (a.sub != nil || a.sup != nil) {
			if isOperator(a.base) {
				fn := "limits"
				if limits < 0 {
					fn = "scripts"
				}
				a.base = call(fn, a.base)
			} else {
				p.warn(`\limits ignored after a non-operator`)
			}
		}
		out = a
	}
	if after != nil {
		return seq(out, after)
	}
	return out
}

// isOperator reports whether n can take \limits: a large operator, an
// operator name or anything built with op().
func isOperator(n *node) bool {
	u := n.unwrap()
	if u == nil {
		return false
	}
	return (u.kind == nAtom && u.cls == clsOp) || (u.kind == nCall && u.s == "op") ||
		(u.kind == nSeq && len(u.kids) > 0 && isOperator(u.kids[len(u.kids)-1]))
}

func countPrimes(n *node) int {
	if n.kind == nAtom {
		if n.s == "prime" {
			return 1
		}
		return 0
	}
	if n.kind != nSeq || len(n.kids) == 0 {
		return 0
	}
	c := 0
	for _, k := range n.kids {
		if k.kind != nAtom || k.s != "prime" {
			return 0
		}
		c++
	}
	return c
}

// parseScriptArg parses the argument of ^ or _.
func (p *parser) parseScriptArg() *node {
	p.skipSpace()
	t := p.peek()
	switch t.kind {
	case tOpen:
		return p.parseArg()
	case tEOF, tClose, tAlign, tSup, tSub:
		p.warn("missing script argument")
		return seq()
	case tPrime:
		p.next()
		return word("prime")
	case tCmd:
		if isRowCmd(t) || t.val == "end" || t.val == "right" || t.val == "middle" {
			p.warn("missing script argument")
			return seq()
		}
		if _, ok := styleCmds[t.val]; ok {
			p.next()
			return seq()
		}
	}
	return p.parseArg()
}

// parseDelim reads the delimiter after \left, \right, \middle or \big and
// returns its Typst symbol, "" for the null delimiter.
func (p *parser) parseDelim() string {
	p.skipSpace()
	t := p.next()
	switch t.kind {
	case tChar:
		if d, ok := delimChars[t.r]; ok {
			return d
		}
		p.warn("unknown delimiter " + t.val)
		return ""
	case tCmd:
		if d, ok := delimCmds[t.val]; ok {
			return d
		}
		if s, ok := symbols[t.val]; ok && s.typ != "" {
			return s.typ
		}
		p.warn(`unknown delimiter \` + t.val)
		return ""
	case tOpen:
		// \left{ is not valid LaTeX; read it as a brace.
		p.pos--
		p.groupBody()
		p.warn("missing delimiter")
		return ""
	}
	p.warn("missing delimiter")
	if t.kind != tEOF {
		p.pos--
	}
	return ""
}

func delimAtom(d string) *node {
	if d == "" {
		return seq()
	}
	return word(d)
}

// parseLeftRight parses the body of \left … \right after \left.
func (p *parser) parseLeftRight() *node {
	left := p.parseDelim()
	items := []*node{delimAtom(left)}
	for {
		body := p.parseList(stopRight | stopMiddle | stopEnd | stopClose)
		items = append(items, body)
		t := p.peek()
		if t.kind == tCmd && t.val == "middle" {
			p.next()
			d := p.parseDelim()
			if d != "" {
				// TeX sets \middle without extra space; Typst would add
				// relation spacing.
				items = append(items, call("class", strAtom("normal"), call("mid", word(d))))
			}
			continue
		}
		if t.kind == tCmd && t.val == "right" {
			p.next()
			items = append(items, delimAtom(p.parseDelim()))
		} else {
			p.warn(`missing \right inserted`)
		}
		break
	}
	return call("lr", seq(items...))
}

// applyFont wraps content in the Typst functions of a math alphabet.
func applyFont(f fontSpec, content *node) *node {
	if f.fn == "" {
		return content
	}
	if f.words {
		content = joinLetters(content)
	}
	u := content.unwrap()
	if u.isEmpty() {
		return seq()
	}
	if f.doubled && u.kind == nAtom && len(u.s) == 1 && u.s[0] >= 'A' && u.s[0] <= 'Z' {
		return word(u.s + u.s)
	}
	if u.kind == nAtom && u.l == eText {
		// A string is already upright.
		if f.fn == "upright" {
			return u
		}
		return call(f.fn, u)
	}
	if f.inner != "" {
		u = call(f.inner, u)
	}
	return call(f.fn, u)
}

// joinLetters merges runs of two or more single letters into one string so
// that \mathrm{max} reads as a word.
func joinLetters(n *node) *node {
	if n.kind == nAtom && n.letter {
		return n
	}
	if n.kind != nSeq {
		return n
	}
	out := make([]*node, 0, len(n.kids))
	var run strings.Builder
	runLen := 0
	var first *node
	flush := func() {
		switch runLen {
		case 0:
		case 1:
			out = append(out, first)
		default:
			out = append(out, strAtom(run.String()))
		}
		run.Reset()
		runLen = 0
	}
	for _, k := range n.kids {
		if k.kind == nAtom && k.letter {
			if runLen == 0 {
				first = k
			}
			run.WriteString(k.s)
			runLen++
			continue
		}
		flush()
		out = append(out, joinLetters(k))
	}
	flush()
	return seq(out...)
}

// parseNot handles \not followed by a relation.
func (p *parser) parseNot() *node {
	p.skipSpace()
	t := p.peek()
	key := ""
	switch t.kind {
	case tChar:
		key = t.val
	case tCmd:
		key = t.val
	}
	if neg, ok := negations[key]; ok && key != "" {
		p.next()
		return word(neg)
	}
	if t.kind == tCmd {
		if s, ok := symbols[t.val]; ok {
			p.next()
			return rawChar(string(s.ch) + "̸")
		}
	}
	if t.kind == tChar && !isWordRune(t.r) && t.r >= 0x80 {
		p.next()
		return rawChar(t.val + "̸")
	}
	item := p.parseArg()
	if item.isEmpty() {
		return word("slash")
	}
	return call("cancel", item)
}

// dotsKind picks \cdots or \ldots for \dots from the following token, as
// amsmath does.
func (p *parser) dotsKind() *node {
	save := p.pos
	p.skipSpace()
	t := p.peek()
	p.pos = save
	centered := false
	switch t.kind {
	case tChar:
		centered = strings.ContainsRune("+-=<>*", t.r)
	case tCmd:
		if s, ok := symbols[t.val]; ok {
			centered = s.cls == clsBin || s.cls == clsRel
		} else if _, ok := bigOps[t.val]; ok {
			centered = true
		}
	}
	if centered {
		return word("dots.h.c")
	}
	return word("dots.h")
}
