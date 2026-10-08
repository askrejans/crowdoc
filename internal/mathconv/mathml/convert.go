package mathml

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// state is inherited down the tree (mstyle mathvariant).
type state struct {
	variant string
}

type converter struct{}

// builder joins TeX fragments, inserting a space where a control word would
// otherwise run into a following letter (`\alpha` + `x`).
type builder struct{ sb strings.Builder }

func (b *builder) add(s string) {
	if s == "" {
		return
	}
	if cur := b.sb.String(); cur != "" && isASCIILetter(s[0]) && endsWithControlWord(cur) {
		b.sb.WriteByte(' ')
	}
	b.sb.WriteString(s)
}

func (b *builder) String() string { return b.sb.String() }

func join(parts ...string) string {
	var b builder
	for _, p := range parts {
		b.add(p)
	}
	return b.String()
}

func endsWithControlWord(s string) bool {
	i := len(s)
	for i > 0 && isASCIILetter(s[i-1]) {
		i--
	}
	if i == len(s) || i == 0 || s[i-1] != '\\' {
		return false
	}
	slashes := 0
	for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
		slashes++
	}
	return slashes%2 == 1
}

func (c *converter) conv(n *node, st state) string {
	if n == nil {
		return ""
	}
	if n.name == "" {
		if t := strings.TrimSpace(n.text); t != "" {
			return escMath(collapse(t))
		}
		return ""
	}
	switch n.name {
	case "mi":
		return c.mi(n, st)
	case "mn":
		return c.mn(n, st)
	case "mo":
		return c.mo(n)
	case "mtext":
		return mtext(n)
	case "ms":
		return ms(n)
	case "mspace":
		return mspace(n)
	case "mglyph":
		return escMath(n.attr("alt"))
	case "annotation", "annotation-xml", "none", "mprescripts", "maligngroup",
		"malignmark", "msline", "mscarries", "mscarry":
		return ""
	case "semantics":
		for _, k := range n.elems() {
			if k.name != "annotation" && k.name != "annotation-xml" {
				return c.conv(k, st)
			}
		}
		return ""
	case "maction":
		els := n.elems()
		sel, err := strconv.Atoi(strings.TrimSpace(n.attr("selection")))
		if err != nil || sel < 1 || sel > len(els) {
			sel = 1
		}
		if len(els) == 0 {
			return ""
		}
		return c.conv(els[sel-1], st)
	case "mstyle":
		return c.style(n, st)
	case "mphantom":
		return `\phantom{` + c.row(n.kids, st) + `}`
	case "msqrt":
		return `\sqrt{` + c.row(n.kids, st) + `}`
	case "mroot":
		els := n.elems()
		return `\sqrt[` + c.at(els, 1, st) + `]{` + c.at(els, 0, st) + `}`
	case "mfrac":
		return c.frac(n, st)
	case "msub", "msup", "msubsup":
		return c.scripts(n, st)
	case "munder", "mover", "munderover":
		return c.underOver(n, st)
	case "mmultiscripts":
		return c.multiscripts(n, st)
	case "mfenced":
		return c.mfenced(n, st)
	case "menclose":
		return c.menclose(n, st)
	case "mtable":
		return c.table(n, st, "")
	}
	// math, mrow, mpadded, merror, mtd, mstack, mlongdiv and unknown
	// elements: their content.
	return c.row(n.kids, st)
}

// at converts the i-th element of els, or returns "" when it is missing.
func (c *converter) at(els []*node, i int, st state) string {
	if i < len(els) {
		return c.conv(els[i], st)
	}
	return ""
}

// significant drops whitespace-only text nodes.
func significant(kids []*node) []*node {
	out := make([]*node, 0, len(kids))
	for _, k := range kids {
		if k.name == "" && strings.TrimSpace(k.text) == "" {
			continue
		}
		out = append(out, k)
	}
	return out
}

func (c *converter) row(kids []*node, st state) string {
	items := significant(kids)
	if s, ok := c.fenced(items, st); ok {
		return s
	}
	var b builder
	for _, k := range items {
		b.add(c.conv(k, st))
	}
	return b.String()
}

// fenced handles rows that open and close with fence operators: matrices,
// binomials, cases and stretchy \left…\right pairs.
func (c *converter) fenced(items []*node, st state) (string, bool) {
	if len(items) < 2 {
		return "", false
	}
	open, okOpen := fenceOf(items[0], openFences)
	if okOpen && open == "{" {
		rest := items[1:]
		if n := len(rest); n > 1 && rest[n-1].name == "mo" && tokenText(rest[n-1]) == "" {
			rest = rest[:n-1]
		}
		if len(rest) == 1 && rest[0].name == "mtable" {
			return c.table(rest[0], st, "cases"), true
		}
	}
	closer, okClose := fenceOf(items[len(items)-1], closeFences)
	if !okOpen || !okClose {
		return "", false
	}
	inner := items[1 : len(items)-1]
	if len(inner) == 1 {
		switch in := inner[0]; {
		case in.name == "mtable":
			if env, ok := matrixEnvs[open+closer]; ok {
				return c.table(in, st, env), true
			}
		case in.name == "mfrac" && open == "(" && closer == ")" && isZeroLength(in.attr("linethickness")):
			els := in.elems()
			return `\binom{` + c.at(els, 0, st) + `}{` + c.at(els, 1, st) + `}`, true
		}
	}
	if !balanced(inner, open) {
		return "", false
	}
	stretchy := items[0].attr("stretchy")
	if stretchy == "false" || stretchy != "true" && !tallAny(inner) {
		return "", false
	}
	var b builder
	for _, k := range inner {
		b.add(c.conv(k, st))
	}
	return join(`\left`+delimiters[open], b.String(), `\right`+delimiters[closer]), true
}

func fenceOf(n *node, set map[string]bool) (string, bool) {
	if n.name != "mo" {
		return "", false
	}
	t := tokenText(n)
	if set[t] {
		if _, ok := delimiters[t]; ok {
			return t, true
		}
	}
	return "", false
}

// balanced reports whether the top-level fences inside a candidate
// \left…\right pair nest properly, so `(a)(b)` is not wrapped as one pair.
func balanced(inner []*node, open string) bool {
	depth := 0
	for _, k := range inner {
		if k.name != "mo" {
			continue
		}
		t := tokenText(k)
		if t == open && closeFences[t] {
			return false // |a|b|: ambiguous
		}
		switch {
		case openFences[t] && !closeFences[t]:
			depth++
		case closeFences[t] && !openFences[t]:
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// tallAny reports whether content is tall enough to need stretchy fences.
func tallAny(items []*node) bool {
	for _, k := range items {
		if tall(k, 0) {
			return true
		}
	}
	return false
}

func tall(n *node, depth int) bool {
	switch n.name {
	case "":
		return false
	case "mfrac":
		return n.attr("bevelled") != "true"
	case "mtable", "munderover":
		return true
	case "mo":
		r, _ := utf8.DecodeRuneInString(tokenText(n))
		return limitOps[symbols[r]] && !isLimitWord(symbols[r])
	}
	if depth > 20 {
		return false
	}
	for _, k := range n.kids {
		if tall(k, depth+1) {
			return true
		}
	}
	return false
}

func isLimitWord(cmd string) bool {
	switch cmd {
	case `\lim`, `\limsup`, `\liminf`, `\max`, `\min`, `\sup`, `\inf`, `\det`, `\gcd`, `\Pr`:
		return true
	}
	return false
}

// tokenText returns the collapsed, trimmed text of a token element.
func tokenText(n *node) string {
	return strings.TrimSpace(collapse(n.textContent()))
}

// collapse folds ASCII whitespace runs into one space (NBSP is kept).
func collapse(s string) string {
	if !strings.ContainsAny(s, "\t\n\r\f") && !strings.Contains(s, "  ") {
		return s
	}
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
	}), " ")
}

func (c *converter) mi(n *node, st state) string {
	t := tokenText(n)
	if t == "" {
		return ""
	}
	v := n.attr("mathvariant")
	if v == "" {
		v = st.variant
	}
	if utf8.RuneCountInString(t) == 1 {
		if v == "" || v == "italic" {
			return escMath(t)
		}
		return withVariant(v, escMath(t))
	}
	if v == "" || v == "normal" {
		if f, ok := functions[t]; ok {
			return f
		}
		return `\mathrm{` + escMath(t) + `}`
	}
	return withVariant(v, escMath(t))
}

func (c *converter) mn(n *node, st state) string {
	t := tokenText(n)
	v := n.attr("mathvariant")
	if v == "" {
		v = st.variant
	}
	if t == "" || v == "" || v == "normal" {
		return escMath(t)
	}
	return withVariant(v, escMath(t))
}

func (c *converter) mo(n *node) string {
	t := tokenText(n)
	if t == "" {
		return ""
	}
	if f, ok := functions[t]; ok {
		return f
	}
	if len(t) > 1 && isASCIIWord(t) {
		return `\operatorname{` + t + `}`
	}
	return escMath(t)
}

func isASCIIWord(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isASCIILetter(s[i]) {
			return false
		}
	}
	return s != ""
}

func withVariant(v, s string) string {
	if s == "" {
		return ""
	}
	cmd := ""
	switch v {
	case "bold":
		cmd = `\mathbf`
		if strings.HasPrefix(s, `\`) {
			cmd = `\boldsymbol`
		}
	case "italic":
		cmd = `\mathit`
	case "bold-italic", "sans-serif-bold-italic":
		cmd = `\boldsymbol`
	case "double-struck":
		cmd = `\mathbb`
	case "script", "bold-script":
		cmd = `\mathcal`
	case "fraktur", "bold-fraktur":
		cmd = `\mathfrak`
	case "sans-serif", "bold-sans-serif", "sans-serif-italic":
		cmd = `\mathsf`
	case "monospace":
		cmd = `\mathtt`
	case "normal":
		cmd = `\mathrm`
	default:
		return s
	}
	return cmd + "{" + s + "}"
}

// mathAlnum decodes a Mathematical Alphanumeric Symbol into its variant and
// plain ASCII letter or digit.
func mathAlnum(r rune) (string, rune, bool) {
	if r < 0x1D400 || r > 0x1D7FF {
		return "", 0, false
	}
	for _, g := range mathAlnumRanges {
		if r >= g.start && r < g.start+52 {
			i := r - g.start
			if i < 26 {
				return g.variant, 'A' + i, true
			}
			return g.variant, 'a' + i - 26, true
		}
	}
	for _, g := range mathDigitRanges {
		if r >= g.start && r < g.start+10 {
			return g.variant, '0' + r - g.start, true
		}
	}
	return "", 0, false
}

// escMath renders text for math mode: known symbols become commands, TeX
// specials are escaped and control characters dropped.
func escMath(s string) string {
	var b builder
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		if cmd, ok := symbols[r]; ok {
			b.add(cmd)
			continue
		}
		if v, base, ok := mathAlnum(r); ok {
			if v == "italic" {
				b.add(string(base))
			} else {
				b.add(withVariant(v, string(base)))
			}
			continue
		}
		b.add(string(r))
	}
	return b.String()
}

var textEscaper = strings.NewReplacer(
	`\`, `\textbackslash{}`, `{`, `\{`, `}`, `\}`, `$`, `\$`, `&`, `\&`,
	`#`, `\#`, `^`, `\textasciicircum{}`, `_`, `\_`, `%`, `\%`,
	`~`, `\textasciitilde{}`, "\u00A0", `~`,
)

// escText renders text for \text{}.
func escText(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	return textEscaper.Replace(s)
}

func mtext(n *node) string {
	raw := n.textContent()
	t := strings.TrimSpace(collapse(raw))
	if t == "" {
		if raw != "" {
			return `\ `
		}
		return ""
	}
	return `\text{` + escText(t) + `}`
}

func ms(n *node) string {
	lq, rq := `"`, `"`
	if v, ok := n.attrs["lquote"]; ok {
		lq = v
	}
	if v, ok := n.attrs["rquote"]; ok {
		rq = v
	}
	return `\text{` + escText(lq+tokenText(n)+rq) + `}`
}

func mspace(n *node) string {
	if lb := n.attr("linebreak"); lb == "newline" || lb == "indentingnewline" {
		return `\quad`
	}
	w := strings.TrimSpace(n.attr("width"))
	switch w {
	case "":
		return ""
	case "veryverythinmathspace", "verythinmathspace", "thinmathspace":
		return `\,`
	case "mediummathspace":
		return `\:`
	case "thickmathspace":
		return `\;`
	case "verythickmathspace", "veryverythickmathspace":
		return `\quad`
	case "negativeveryverythinmathspace", "negativeverythinmathspace",
		"negativethinmathspace", "negativemediummathspace", "negativethickmathspace":
		return `\!`
	}
	em, _ := lengthEm(w)
	switch {
	case em <= -0.1:
		return `\!`
	case em < 0.05:
		return ""
	case em < 0.2:
		return `\,`
	case em < 0.25:
		return `\:`
	case em < 0.4:
		return `\;`
	case em < 0.75:
		return `\enspace`
	case em < 1.5:
		return `\quad`
	}
	return `\qquad`
}

// lengthEm converts a MathML length to em (approximately).
func lengthEm(s string) (float64, bool) {
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.' || s[i] == '-' || s[i] == '+') {
		i++
	}
	v, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, false
	}
	switch strings.TrimSpace(s[i:]) {
	case "ex":
		v /= 2
	case "px":
		v /= 16
	case "pt":
		v /= 10
	case "mu":
		v /= 18
	case "mm":
		v /= 3.5
	case "cm":
		v *= 2.85
	case "in":
		v *= 7.2
	}
	return v, true
}

func isZeroLength(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	v, ok := lengthEm(s)
	return ok && v == 0
}
