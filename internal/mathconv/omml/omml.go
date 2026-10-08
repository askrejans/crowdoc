// Package omml converts Office Math Markup Language (OMML), the equation
// format embedded in WordprocessingML documents, to LaTeX math-mode source.
//
// The output never contains raw source text: identifiers are escaped,
// Unicode operators are mapped to amsmath/amssymb commands and literal text
// is wrapped in \text{...} with text-mode escaping. Only \oiint and \oiiint
// require unicode-math; everything else is plain amsmath/amssymb.
package omml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Node is a decoded XML element of an OMML tree. Space is the namespace URI
// (or the bare prefix when the prefix is undeclared, or a short code such as
// "m" or "w"); only elements in the WordprocessingML namespace ("w") are
// treated differently from math elements.
type Node struct {
	Space    string
	Local    string
	Attr     []xml.Attr
	Children []*Node
	Text     string
}

// maxDepth bounds element nesting so hostile input cannot exhaust the stack.
const maxDepth = 256

// Parse decodes an XML fragment and returns its first root element.
// Elements nested deeper than an internal limit are dropped.
func Parse(data []byte) (*Node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var root *Node
	var stack []*Node
	skip := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if skip > 0 || len(stack) >= maxDepth || (root != nil && len(stack) == 0) {
				skip++
				continue
			}
			n := &Node{Space: t.Name.Space, Local: t.Name.Local, Attr: t.Attr}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if skip == 0 && len(stack) > 0 {
				stack[len(stack)-1].Text += string(t)
			}
		}
	}
	if root == nil {
		return nil, errors.New("omml: no XML element found")
	}
	return root, nil
}

// Convert converts a standalone <m:oMath> or <m:oMathPara> fragment (the
// first one found in data) to LaTeX. display reports an m:oMathPara.
func Convert(data []byte) (tex string, display bool, err error) {
	root, err := Parse(data)
	if err != nil {
		return "", false, err
	}
	n := findMath(root)
	if n == nil {
		return "", false, errors.New("omml: no m:oMath element found")
	}
	tex, display = ToLaTeX(n)
	return tex, display, nil
}

func findMath(n *Node) *Node {
	if n == nil {
		return nil
	}
	if !isWordNS(n.Space) && (n.Local == "oMathPara" || n.Local == "oMath") {
		return n
	}
	for _, k := range n.Children {
		if m := findMath(k); m != nil {
			return m
		}
	}
	return nil
}

// ToLaTeX converts an m:oMath or m:oMathPara element (or any single OMML
// math object) to LaTeX without math delimiters. display is true for
// m:oMathPara; several equations in one paragraph are stacked in a
// gathered environment.
func ToLaTeX(n *Node) (tex string, display bool) {
	if n == nil {
		return "", false
	}
	c := &conv{}
	switch n.Local {
	case "oMathPara":
		return c.oMathPara(n), true
	case "oMath":
		return c.oMath(n), false
	}
	return c.seq([]*Node{n}), false
}

// conv carries the state of one conversion.
type conv struct {
	depth int
	// alignHere is set while converting the top level of an equation-array
	// row (or a broken display line): '&' and m:aln become alignment points.
	alignHere bool
	amps      int
	// inFName is set while converting the runs of a function name.
	inFName bool
}

func (c *conv) oMathPara(n *Node) string {
	var lines []string
	for _, k := range n.Children {
		if isWordNS(k.Space) || k.Local != "oMath" {
			continue
		}
		if s := c.oMath(k); s != "" {
			lines = append(lines, s)
		}
	}
	if len(lines) == 1 {
		return lines[0]
	}
	if len(lines) == 0 {
		return ""
	}
	return `\begin{gathered}` + strings.Join(lines, ` \\ `) + `\end{gathered}`
}

// oMath converts one equation. Manual line breaks (m:brk on a run) split it
// into lines that are aligned on m:aln points when there are any.
func (c *conv) oMath(n *Node) string {
	lines := splitLines(n.Children)
	if len(lines) <= 1 {
		return c.seq(n.Children)
	}
	savedAmps := c.amps
	c.amps = 0
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		c.alignHere = true
		out = append(out, c.seq(l))
	}
	c.alignHere = false
	env := "gathered"
	if c.amps > 0 {
		env = "aligned"
	}
	c.amps = savedAmps
	return `\begin{` + env + `}` + strings.Join(out, ` \\ `) + `\end{` + env + `}`
}

func splitLines(kids []*Node) [][]*Node {
	var lines [][]*Node
	var cur []*Node
	for _, k := range kids {
		if k.Local == "r" && !isWordNS(k.Space) && k.child("rPr").child("brk") != nil && len(cur) > 0 {
			lines = append(lines, cur)
			cur = nil
		}
		cur = append(cur, k)
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

func (c *conv) seq(nodes []*Node) string {
	var b texBuf
	for _, n := range nodes {
		c.node(n, &b)
	}
	return b.String()
}

// arg converts the content of the named argument element (m:e, m:num, ...).
func (c *conv) arg(n *Node, name string) string {
	e := n.child(name)
	if e == nil {
		return ""
	}
	return c.seq(e.Children)
}

func (c *conv) node(n *Node, b *texBuf) {
	if c.depth >= maxDepth {
		return
	}
	c.depth++
	defer func() { c.depth-- }()
	if isWordNS(n.Space) {
		c.wordNode(n, b)
		return
	}
	if n.Local == "r" {
		c.run(n, b)
		return
	}
	savedAlign, savedFName := c.alignHere, c.inFName
	c.alignHere, c.inFName = false, false
	defer func() { c.alignHere, c.inFName = savedAlign, savedFName }()

	switch n.Local {
	case "f":
		b.add(c.frac(n))
	case "sSup":
		b.add(c.base(n) + "^{" + c.arg(n, "sup") + "}")
	case "sSub":
		b.add(c.base(n) + "_{" + c.arg(n, "sub") + "}")
	case "sSubSup":
		b.add(c.base(n) + "_{" + c.arg(n, "sub") + "}^{" + c.arg(n, "sup") + "}")
	case "sPre":
		b.add("{}_{" + c.arg(n, "sub") + "}^{" + c.arg(n, "sup") + "}")
		b.add(c.arg(n, "e"))
	case "rad":
		b.add(c.rad(n))
	case "nary":
		c.nary(n, b)
	case "d":
		c.delim(n, b)
	case "func":
		c.function(n, b)
	case "limLow":
		b.add(c.limLow(n))
	case "limUpp":
		b.add(c.limUpp(n))
	case "acc":
		b.add(c.acc(n))
	case "bar":
		b.add(c.bar(n))
	case "groupChr":
		b.add(c.groupChr(n))
	case "box":
		b.add(c.arg(n, "e"))
	case "borderBox":
		b.add(`\boxed{` + c.arg(n, "e") + "}")
	case "eqArr":
		b.add(c.eqArr(n, "aligned"))
	case "m":
		b.add(c.matrix(n, "matrix"))
	case "phant":
		b.add(c.phant(n))
	case "oMathPara":
		b.add(c.oMathPara(n))
	case "AlternateContent":
		alt := n.anyChild("Fallback")
		if alt == nil {
			alt = n.anyChild("Choice")
		}
		if alt != nil {
			for _, k := range alt.Children {
				c.node(k, b)
			}
		}
	default:
		if strings.HasSuffix(n.Local, "Pr") {
			return
		}
		for _, k := range n.Children {
			c.node(k, b)
		}
	}
}

// wordNode handles WordprocessingML markup that may wrap math content
// (revision marks, content controls) or appear inside an equation.
func (c *conv) wordNode(n *Node, b *texBuf) {
	switch n.Local {
	case "r":
		var sb strings.Builder
		for _, k := range n.Children {
			switch k.Local {
			case "t":
				sb.WriteString(k.Text)
			case "tab":
				sb.WriteByte(' ')
			}
		}
		if s := sb.String(); strings.TrimSpace(s) != "" {
			b.add(textMode(s))
		}
	case "ins", "moveTo", "sdt", "sdtContent", "smartTag", "customXml", "hyperlink", "fldSimple", "dir", "bdo":
		for _, k := range n.Children {
			c.node(k, b)
		}
	}
}

// run converts an m:r element.
func (c *conv) run(n *Node, b *texBuf) {
	pr := n.child("rPr")
	if c.alignHere && onOff(pr, "aln") {
		b.add("&")
		c.amps++
	}
	var sb strings.Builder
	for _, k := range n.Children {
		if k.Local == "t" {
			sb.WriteString(k.Text)
		}
	}
	s := sb.String()
	if s == "" {
		return
	}
	if onOff(pr, "nor") && hasLetter(s) {
		b.add(textMode(s))
		return
	}
	sty, _ := prop(pr, "sty")
	scr, _ := prop(pr, "scr")
	c.mathText(s, styleCmd(sty, scr), b)
}

// styleCmd returns the LaTeX alphabet command for a run's m:sty / m:scr.
func styleCmd(sty, scr string) string {
	switch scr {
	case "double-struck":
		return `\mathbb`
	case "script":
		return `\mathcal`
	case "fraktur":
		return `\mathfrak`
	case "sans-serif":
		return `\mathsf`
	case "monospace":
		return `\mathtt`
	}
	switch sty {
	case "p":
		return `\mathrm`
	case "b":
		return `\mathbf`
	case "bi":
		return `\boldsymbol`
	}
	return ""
}

// mathText converts the text of a math run.
func (c *conv) mathText(s, style string, b *texBuf) {
	rs := []rune(s)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case isWordLetter(r):
			j := i
			for j < len(rs) && isWordLetter(rs[j]) && !(j+1 < len(rs) && isCombining(rs[j+1])) {
				j++
			}
			if j == i { // a letter carrying a combining accent
				b.add(accented(c.word(string(r), style), rs[i+1]))
				i += 2
				continue
			}
			b.add(c.word(string(rs[i:j]), style))
			i = j
		case r >= '0' && r <= '9':
			j := i + 1
			for j < len(rs) && (isDigit(rs[j]) || (rs[j] == '.' && j+1 < len(rs) && isDigit(rs[j+1]))) {
				j++
			}
			b.add(digitsTeX(string(rs[i:j]), style))
			i = j
		case r == '&' && c.alignHere:
			b.add("&")
			c.amps++
			i++
		default:
			t := symbolTeX(r, style)
			if i+1 < len(rs) && isCombining(rs[i+1]) && t != "" {
				t = accented(t, rs[i+1])
				i++
			}
			b.add(t)
			i++
		}
	}
}

// word converts a run of letters. Upright runs and function names become
// operator names when LaTeX knows them. Words with letters math fonts
// may lack (accented Latin, Cyrillic, ...) are set in the text font.
func (c *conv) word(w, style string) string {
	if !isASCIIWord(w) {
		switch {
		case c.inFName || style == `\mathrm`:
			return textMode(w)
		case style == "":
			return `\textit{` + w + "}"
		case style == `\mathbf`:
			return `\textbf{` + w + "}"
		case style == `\boldsymbol`:
			return `\textbf{\textit{` + w + "}}"
		}
		return textMode(w)
	}
	if c.inFName || style == `\mathrm` {
		if fn, ok := functions[w]; ok {
			return fn
		}
		if len(w) == 1 {
			if style == `\mathrm` {
				return `\mathrm{` + w + "}"
			}
			return w
		}
		if c.inFName {
			return `\operatorname{` + w + "}"
		}
		return `\mathrm{` + w + "}"
	}
	if style == "" {
		return w
	}
	return style + "{" + w + "}"
}

func digitsTeX(d, style string) string {
	switch style {
	case `\mathbf`, `\boldsymbol`, `\mathsf`, `\mathtt`, `\mathfrak`:
		return style + "{" + d + "}"
	}
	return d
}

// symbolTeX converts a single non-letter, non-digit character.
func symbolTeX(r rune, style string) string {
	switch {
	case r == ' ' || r == '\t' || r == '\n' || r == '\r':
		return ""
	case unicode.IsControl(r) || r == utf8.RuneError:
		return ""
	}
	if st, base, ok := mathAlnum(r); ok {
		inner := string(base)
		if t, ok := symbols[base]; ok {
			inner = t
		}
		if st == "" {
			return inner
		}
		if isGreekLetter(base) && st != `\boldsymbol` {
			st = `\boldsymbol`
		}
		return st + "{" + inner + "}"
	}
	if t, ok := symbols[r]; ok {
		if t != "" && isGreekLetter(r) && (style == `\mathbf` || style == `\boldsymbol`) {
			return `\boldsymbol{` + t + "}"
		}
		return t
	}
	if r < 0x80 {
		return string(r)
	}
	if unicode.Is(unicode.Mn, r) {
		return ""
	}
	return textMode(string(r))
}

// symText converts a short character string (an operator or accent glyph).
func symText(s string) string {
	var b texBuf
	for _, r := range s {
		if isASCIILetter(r) || isDigit(r) {
			b.add(string(r))
			continue
		}
		b.add(symbolTeX(r, ""))
	}
	return b.String()
}

func accented(base string, mark rune) string {
	cmds, ok := accents[mark]
	if !ok {
		return base
	}
	return cmds[0] + "{" + base + "}"
}

// textMode wraps literal text in \text{...} with text-mode escaping. Math
// symbols that text fonts may lack are switched back into math mode.
func textMode(s string) string {
	var b strings.Builder
	b.WriteString(`\text{`)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\textbackslash{}`)
		case '{', '}', '#', '$', '%', '&', '_':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '~':
			b.WriteString(`\textasciitilde{}`)
		case '^':
			b.WriteString(`\textasciicircum{}`)
		case '\t', '\n', '\r':
			b.WriteByte(' ')
		case '\u00a0':
			b.WriteByte('~')
		default:
			if unicode.IsControl(r) || r == utf8.RuneError {
				continue
			}
			if r >= 0x80 {
				if unicode.IsSpace(r) {
					b.WriteByte(' ')
					continue
				}
				if t, ok := symbols[r]; ok && !unicode.IsLetter(r) {
					if t != "" {
						b.WriteString("$" + t + "$")
					}
					continue
				}
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('}')
	return b.String()
}

// base converts the m:e of a script object, bracing it when the script
// would otherwise attach to only part of it.
func (c *conv) base(n *Node) string {
	return brace(c.arg(n, "e"))
}

func brace(s string) string {
	if isAtom(s) {
		return s
	}
	return "{" + s + "}"
}

func (c *conv) frac(n *Node) string {
	typ, _ := prop(n.child("fPr"), "type")
	num, den := c.arg(n, "num"), c.arg(n, "den")
	switch typ {
	case "lin":
		return brace(num) + "/" + brace(den)
	case "skw":
		return "{}^{" + num + `}\!/\!{}_{` + den + "}"
	case "noBar":
		return `\genfrac{}{}{0pt}{}{` + num + "}{" + den + "}"
	}
	return `\frac{` + num + "}{" + den + "}"
}

func (c *conv) rad(n *Node) string {
	e := c.arg(n, "e")
	if onOff(n.child("radPr"), "degHide") {
		return `\sqrt{` + e + "}"
	}
	deg := c.arg(n, "deg")
	switch {
	case deg == "":
		return `\sqrt{` + e + "}"
	case strings.Contains(deg, "]"):
		return `\sqrt[{` + deg + `}]{` + e + "}"
	}
	return `\sqrt[` + deg + `]{` + e + "}"
}

func (c *conv) nary(n *Node, b *texBuf) {
	pr := n.child("naryPr")
	chr := "∫"
	if v, ok := prop(pr, "chr"); ok && v != "" {
		chr = v
	}
	op, ok := naryOps[chr]
	if !ok {
		op = `\mathop{` + symText(chr) + "}"
	}
	limLoc, _ := prop(pr, "limLoc")
	switch limLoc {
	case "undOvr":
		op += `\limits`
	case "subSup":
		if !integralOps[chr] {
			op += `\nolimits`
		}
	}
	if !onOff(pr, "subHide") {
		if s := c.arg(n, "sub"); s != "" {
			op += "_{" + s + "}"
		}
	}
	if !onOff(pr, "supHide") {
		if s := c.arg(n, "sup"); s != "" {
			op += "^{" + s + "}"
		}
	}
	b.add(op)
	if e := c.arg(n, "e"); e != "" {
		b.space()
		b.add(e)
	}
}

func (c *conv) delim(n *Node, b *texBuf) {
	pr := n.child("dPr")
	beg, end, sep := "(", ")", "|"
	if v, ok := prop(pr, "begChr"); ok {
		beg = v
	}
	if v, ok := prop(pr, "endChr"); ok {
		end = v
	}
	if v, ok := prop(pr, "sepChr"); ok {
		sep = v
	}
	es := n.children("e")
	if len(es) == 1 {
		if m := soleChild(es[0], "m"); m != nil {
			if env := matrixEnv(beg, end); env != "" {
				b.add(c.matrix(m, env))
				return
			}
		}
		if eq := soleChild(es[0], "eqArr"); eq != nil && beg == "{" && end == "" {
			b.add(c.eqArr(eq, "cases"))
			return
		}
	}
	grow := pr.child("grow") == nil || onOff(pr, "grow")
	if grow {
		b.add(`\left` + delimTeX(beg))
	} else {
		b.add(plainDelim(beg))
	}
	sepTeX := separatorTeX(sep)
	for i, e := range es {
		if i > 0 {
			b.add(sepTeX)
		}
		b.add(c.seq(e.Children))
	}
	if grow {
		b.add(`\right` + delimTeX(end))
	} else {
		b.add(plainDelim(end))
	}
}

func plainDelim(s string) string {
	d := delimTeX(s)
	if d == "." {
		return ""
	}
	return d
}

func separatorTeX(s string) string {
	switch s {
	case "|", "∣":
		return `\mid`
	case "‖", "∥":
		return `\|`
	case "":
		return ""
	}
	return symText(s)
}

// soleChild returns the only meaningful child of e if it is the named
// element.
func soleChild(e *Node, local string) *Node {
	var found *Node
	for _, k := range e.Children {
		if strings.HasSuffix(k.Local, "Pr") {
			continue
		}
		if found != nil || isWordNS(k.Space) || k.Local != local {
			return nil
		}
		found = k
	}
	return found
}

func (c *conv) matrix(m *Node, env string) string {
	var rows []string
	for _, mr := range m.children("mr") {
		var cells []string
		for _, e := range mr.children("e") {
			cells = append(cells, c.seq(e.Children))
		}
		rows = append(rows, strings.Join(cells, " & "))
	}
	return `\begin{` + env + `}` + strings.Join(rows, ` \\ `) + `\end{` + env + `}`
}

// eqArr converts an equation array. Alignment points become '&'; without
// any the rows are simply centred (gathered).
func (c *conv) eqArr(n *Node, env string) string {
	savedAlign, savedAmps := c.alignHere, c.amps
	c.amps = 0
	var rows []string
	for _, e := range n.children("e") {
		c.alignHere = true
		rows = append(rows, c.seq(e.Children))
	}
	amps := c.amps
	c.alignHere, c.amps = savedAlign, savedAmps
	if env == "aligned" && amps == 0 {
		env = "gathered"
	}
	return `\begin{` + env + `}` + strings.Join(rows, ` \\ `) + `\end{` + env + `}`
}

func (c *conv) function(n *Node, b *texBuf) {
	c.inFName = true
	name := c.arg(n, "fName")
	c.inFName = false
	b.add(name)
	if e := c.arg(n, "e"); e != "" {
		b.space()
		b.add(e)
	}
}

func (c *conv) limLow(n *Node) string {
	base, lim := c.arg(n, "e"), c.arg(n, "lim")
	if lim == "" {
		return base
	}
	if w := opWord(n.child("e")); w != "" {
		if fn, ok := functions[w]; ok && limitOps[fn] {
			return fn + "_{" + lim + "}"
		}
		if _, known := functions[w]; !known {
			return `\operatorname*{` + w + "}_{" + lim + "}"
		}
	}
	if limitOps[base] || (strings.HasPrefix(base, `\underbrace{`) && isAtom(base)) {
		return base + "_{" + lim + "}"
	}
	return `\underset{` + lim + "}{" + base + "}"
}

func (c *conv) limUpp(n *Node) string {
	base, lim := c.arg(n, "e"), c.arg(n, "lim")
	if lim == "" {
		return base
	}
	if strings.HasPrefix(base, `\overbrace{`) && isAtom(base) {
		return base + "^{" + lim + "}"
	}
	return `\overset{` + lim + "}{" + base + "}"
}

// opWord returns the text of e when it is a single upright multi-letter word
// written as plain runs (an operator name such as "lim" or "argmax").
func opWord(e *Node) string {
	if e == nil {
		return ""
	}
	var sb strings.Builder
	for _, k := range e.Children {
		if strings.HasSuffix(k.Local, "Pr") {
			continue
		}
		if isWordNS(k.Space) || k.Local != "r" {
			return ""
		}
		pr := k.child("rPr")
		if sty, _ := prop(pr, "sty"); sty != "p" {
			return ""
		}
		for _, t := range k.Children {
			if t.Local == "t" {
				sb.WriteString(t.Text)
			}
		}
	}
	w := strings.TrimSpace(sb.String())
	if len(w) < 2 {
		return ""
	}
	for i := 0; i < len(w); i++ {
		if !isASCIILetter(rune(w[i])) {
			return ""
		}
	}
	return w
}

func (c *conv) acc(n *Node) string {
	base := c.arg(n, "e")
	chr := "\u0302"
	if v, ok := prop(n.child("accPr"), "chr"); ok {
		chr = v
	}
	if chr == "" {
		return base
	}
	r, _ := utf8.DecodeRuneInString(chr)
	cmds, ok := accents[r]
	if !ok {
		sym := symText(chr)
		if sym == "" {
			// A combining mark LaTeX has no accent for; keep the base.
			return base
		}
		return `\overset{` + sym + "}{" + base + "}"
	}
	if isSingle(base) {
		return cmds[0] + "{" + base + "}"
	}
	return cmds[1] + "{" + base + "}"
}

func (c *conv) bar(n *Node) string {
	e := c.arg(n, "e")
	if pos, _ := prop(n.child("barPr"), "pos"); pos == "top" {
		return `\overline{` + e + "}"
	}
	return `\underline{` + e + "}"
}

func (c *conv) groupChr(n *Node) string {
	pr := n.child("groupChrPr")
	chr, pos := "⏟", "bot"
	if v, ok := prop(pr, "chr"); ok {
		chr = v
	}
	if v, ok := prop(pr, "pos"); ok && v != "" {
		pos = v
	}
	e := c.arg(n, "e")
	top := pos == "top"
	pick := func(over, under string) string {
		if top {
			return over + "{" + e + "}"
		}
		return under + "{" + e + "}"
	}
	switch chr {
	case "⏟", "︸", "⏞", "︷":
		return pick(`\overbrace`, `\underbrace`)
	case "→", "⟶":
		return pick(`\overrightarrow`, `\underrightarrow`)
	case "←", "⟵":
		return pick(`\overleftarrow`, `\underleftarrow`)
	case "↔", "⟷":
		return pick(`\overleftrightarrow`, `\underleftrightarrow`)
	case "⏜", "⏝", "⌢", "⌣":
		if top {
			return `\overset{\frown}{` + e + "}"
		}
		return `\underset{\smile}{` + e + "}"
	case "":
		return e
	}
	if top {
		return `\overset{` + symText(chr) + "}{" + e + "}"
	}
	return `\underset{` + symText(chr) + "}{" + e + "}"
}

// phant converts a phantom. m:show defaults to on, so a phantom without
// properties is simply its content.
func (c *conv) phant(n *Node) string {
	pr := n.child("phantPr")
	e := c.arg(n, "e")
	visible := pr.child("show") == nil || onOff(pr, "show")
	zw, za, zd := onOff(pr, "zeroWid"), onOff(pr, "zeroAsc"), onOff(pr, "zeroDesc")
	if visible {
		if za || zd {
			return `\smash{` + e + "}"
		}
		return e
	}
	switch {
	case zw && !za && !zd:
		return `\vphantom{` + e + "}"
	case (za || zd) && !zw:
		return `\hphantom{` + e + "}"
	}
	return `\phantom{` + e + "}"
}

// ---------------------------------------------------------------------------
// XML helpers

func isWordNS(space string) bool {
	return space == "w" || strings.Contains(space, "wordprocessingml")
}

// child returns the first math child named local (nil-safe).
func (n *Node) child(local string) *Node {
	if n == nil {
		return nil
	}
	for _, k := range n.Children {
		if k.Local == local && !isWordNS(k.Space) {
			return k
		}
	}
	return nil
}

// anyChild returns the first child named local in any namespace.
func (n *Node) anyChild(local string) *Node {
	if n == nil {
		return nil
	}
	for _, k := range n.Children {
		if k.Local == local {
			return k
		}
	}
	return nil
}

func (n *Node) children(local string) []*Node {
	if n == nil {
		return nil
	}
	var out []*Node
	for _, k := range n.Children {
		if k.Local == local && !isWordNS(k.Space) {
			out = append(out, k)
		}
	}
	return out
}

func (n *Node) attr(local string) (string, bool) {
	if n == nil {
		return "", false
	}
	for _, a := range n.Attr {
		if a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

// prop returns the m:val of property element name inside pr and whether
// the element is present.
func prop(pr *Node, name string) (string, bool) {
	e := pr.child(name)
	if e == nil {
		return "", false
	}
	v, _ := e.attr("val")
	return v, true
}

// onOff evaluates an ST_OnOff property: present without a value means on.
func onOff(pr *Node, name string) bool {
	e := pr.child(name)
	if e == nil {
		return false
	}
	v, ok := e.attr("val")
	if !ok {
		return true
	}
	switch strings.ToLower(v) {
	case "1", "on", "true":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// TeX output helpers

// texBuf accumulates TeX, inserting a space where a control word would
// otherwise run into a following letter (\alpha x, not \alphax).
type texBuf []byte

func (t *texBuf) add(s string) {
	if s == "" {
		return
	}
	if isASCIILetter(rune(s[0])) && endsWithControlWord(*t) {
		*t = append(*t, ' ')
	}
	*t = append(*t, s...)
}

func (t *texBuf) space() {
	if n := len(*t); n > 0 && (*t)[n-1] != ' ' {
		*t = append(*t, ' ')
	}
}

func (t texBuf) String() string { return trimTeX(string(t)) }

func endsWithControlWord(b []byte) bool {
	i := len(b)
	for i > 0 && isASCIILetter(rune(b[i-1])) {
		i--
	}
	if i == len(b) || i == 0 {
		return false
	}
	n := 0
	for j := i - 1; j >= 0 && b[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}

// trimTeX trims surrounding spaces without breaking a trailing control
// space ("\ ").
func trimTeX(s string) string {
	s = strings.TrimLeft(s, " ")
	for strings.HasSuffix(s, " ") {
		n := 0
		for j := len(s) - 2; j >= 0 && s[j] == '\\'; j-- {
			n++
		}
		if n%2 == 1 {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

// isAtom reports whether s is a single TeX token or group that can carry a
// sub/superscript without extra braces.
func isAtom(s string) bool {
	if s == "" {
		return false
	}
	if _, size := utf8.DecodeRuneInString(s); size == len(s) {
		return s != " " && s != "&"
	}
	switch s[0] {
	case '{':
		return groupEnd(s, 0) == len(s)
	case '\\':
		name, rest := ctrlSeq(s)
		if rest == "" {
			return true
		}
		if name == `\left` {
			return leftRightEnd(s) == len(s)
		}
		if atomCmds[name] && rest[0] == '{' {
			return groupEnd(rest, 0) == len(rest)
		}
	}
	return false
}

// isSingle reports whether s is one character or one control word, the
// bases that take narrow accents.
func isSingle(s string) bool {
	if _, size := utf8.DecodeRuneInString(s); size == len(s) && s != "" {
		return true
	}
	if len(s) > 1 && s[0] == '\\' {
		_, rest := ctrlSeq(s)
		return rest == ""
	}
	return false
}

// atomCmds take one braced argument and produce a single atom.
var atomCmds = map[string]bool{
	`\mathrm`: true, `\mathbf`: true, `\mathit`: true, `\mathbb`: true,
	`\mathcal`: true, `\mathfrak`: true, `\mathsf`: true, `\mathtt`: true,
	`\boldsymbol`: true, `\text`: true, `\operatorname`: true, `\hat`: true,
	`\widehat`: true, `\tilde`: true, `\widetilde`: true, `\bar`: true,
	`\overline`: true, `\underline`: true, `\vec`: true, `\overrightarrow`: true,
	`\overleftarrow`: true, `\overleftrightarrow`: true, `\dot`: true,
	`\ddot`: true, `\dddot`: true, `\check`: true, `\acute`: true, `\grave`: true,
	`\breve`: true, `\mathring`: true, `\sqrt`: true, `\boxed`: true,
	`\underbrace`: true, `\overbrace`: true,
}

// ctrlSeq splits a leading control sequence off s (which starts with '\').
func ctrlSeq(s string) (name, rest string) {
	if len(s) < 2 {
		return s, ""
	}
	if !isASCIILetter(rune(s[1])) {
		_, size := utf8.DecodeRuneInString(s[1:])
		return s[:1+size], s[1+size:]
	}
	i := 1
	for i < len(s) && isASCIILetter(rune(s[i])) {
		i++
	}
	return s[:i], s[i:]
}

// groupEnd returns the index just past the brace group starting at s[i],
// or -1.
func groupEnd(s string, i int) int {
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return -1
}

// leftRightEnd returns the index just past the \right delimiter that
// closes the \left at the start of s, or -1.
func leftRightEnd(s string) int {
	depth := 0
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			i++
			continue
		}
		name, _ := ctrlSeq(s[i:])
		i += len(name)
		switch name {
		case `\left`:
			depth++
			i = skipDelim(s, i)
		case `\right`:
			depth--
			i = skipDelim(s, i)
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func skipDelim(s string, i int) int {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i >= len(s) {
		return i
	}
	if s[i] == '\\' {
		name, _ := ctrlSeq(s[i:])
		return i + len(name)
	}
	_, size := utf8.DecodeRuneInString(s[i:])
	return i + size
}

func isASCIILetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }

// isWordLetter reports letters that form identifiers: ASCII letters and
// other letters without a dedicated math command (Greek and letterlike
// symbols are converted one by one).
func isWordLetter(r rune) bool {
	if isASCIILetter(r) {
		return true
	}
	if r < 0x80 || !unicode.IsLetter(r) {
		return false
	}
	if _, ok := symbols[r]; ok {
		return false
	}
	_, _, alnum := mathAlnum(r)
	return !alnum
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func isASCIIWord(w string) bool {
	for i := 0; i < len(w); i++ {
		if w[i] >= 0x80 {
			return false
		}
	}
	return true
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func isCombining(r rune) bool {
	_, ok := accents[r]
	return ok && unicode.Is(unicode.Mn, r)
}
