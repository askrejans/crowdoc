package mathml

import (
	"strings"
	"unicode/utf8"
)

func (c *converter) frac(n *node, st state) string {
	els := n.elems()
	num, den := c.at(els, 0, st), c.at(els, 1, st)
	if isZeroLength(n.attr("linethickness")) {
		return `\genfrac{}{}{0pt}{}{` + num + `}{` + den + `}`
	}
	if n.attr("bevelled") == "true" {
		return scriptBase(num) + "/" + scriptBase(den)
	}
	return `\frac{` + num + `}{` + den + `}`
}

// scriptBase braces a base unless it is a single atom.
func scriptBase(s string) string {
	if s == "" {
		return "{}"
	}
	if simpleAtom(s) {
		return s
	}
	return "{" + s + "}"
}

// simpleAtom reports whether s is one TeX atom: a character, a control word,
// a number, or a command applied to one braced argument (`\mathbf{x}`).
func simpleAtom(s string) bool {
	if utf8.RuneCountInString(s) == 1 {
		return true
	}
	if isNumber(s) {
		return true
	}
	if s[0] == '{' {
		return groupEnd(s, 0) == len(s)-1
	}
	if s[0] != '\\' {
		return false
	}
	i := 1
	for i < len(s) && isASCIILetter(s[i]) {
		i++
	}
	if i == 1 {
		return len(s) == 2 // control symbol
	}
	if i == len(s) {
		return true
	}
	return s[i] == '{' && groupEnd(s, i) == len(s)-1
}

func isNumber(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] < '0' || s[i] > '9') && s[i] != '.' {
			return false
		}
	}
	return true
}

// groupEnd returns the index of the brace closing the group opened at i.
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
				return j
			}
		}
	}
	return -1
}

// singleGroupCommand reports whether s is exactly `cmd{...}`.
func singleGroupCommand(s, cmd string) bool {
	return strings.HasPrefix(s, cmd+"{") && groupEnd(s, len(cmd)) == len(s)-1
}

func primes(n *node) (string, bool) {
	if n == nil || n.name != "mo" {
		return "", false
	}
	var sb strings.Builder
	for _, r := range tokenText(n) {
		switch r {
		case '′', '\'':
			sb.WriteByte('\'')
		case '″':
			sb.WriteString("''")
		case '‴':
			sb.WriteString("'''")
		default:
			return "", false
		}
	}
	return sb.String(), sb.Len() > 0
}

func (c *converter) scripts(n *node, st state) string {
	els := n.elems()
	if len(els) == 0 {
		return ""
	}
	base := c.conv(els[0], st)
	var sub string
	var supNode *node
	switch n.name {
	case "msub":
		sub = c.at(els, 1, st)
	case "msup":
		if len(els) > 1 {
			supNode = els[1]
		}
	case "msubsup":
		sub = c.at(els, 1, st)
		if len(els) > 2 {
			supNode = els[2]
		}
	}
	out := scriptBase(base)
	if sub != "" {
		out += "_{" + sub + "}"
	}
	if p, ok := primes(supNode); ok {
		return out + p
	}
	if sup := c.conv(supNode, st); sup != "" {
		out += "^{" + sup + "}"
	}
	if out == "{}" {
		return ""
	}
	return out
}

func (c *converter) underOver(n *node, st state) string {
	els := n.elems()
	if len(els) == 0 {
		return ""
	}
	baseN := els[0]
	var underN, overN *node
	switch n.name {
	case "munder":
		underN = nth(els, 1)
	case "mover":
		overN = nth(els, 1)
	case "munderover":
		underN, overN = nth(els, 1), nth(els, 2)
	}
	base := c.conv(baseN, st)

	if strings.HasPrefix(base, `\operatorname{`) {
		base = `\operatorname*` + base[len(`\operatorname`):]
	}
	if limitOps[base] || strings.HasPrefix(base, `\operatorname*{`) || baseN.attr("largeop") == "true" {
		out := scriptBase(base)
		if u := c.conv(underN, st); u != "" {
			out += "_{" + u + "}"
		}
		if o := c.conv(overN, st); o != "" {
			out += "^{" + o + "}"
		}
		return out
	}

	if baseN.name == "mo" && overN != nil {
		if x, ok := extensibleArrows[tokenText(baseN)]; ok {
			o := c.conv(overN, st)
			if u := c.conv(underN, st); u != "" {
				return x + "[" + u + "]{" + o + "}"
			}
			return x + "{" + o + "}"
		}
	}

	out := base
	if underN != nil {
		out = c.under(out, underN, n.attr("accentunder"), st)
	}
	if overN != nil {
		out = c.over(out, overN, n.attr("accent"), st)
	}
	return out
}

func nth(els []*node, i int) *node {
	if i < len(els) {
		return els[i]
	}
	return nil
}

// narrowBase reports whether an accent on base should use the narrow form
// (\hat rather than \widehat).
func narrowBase(base string) bool {
	if utf8.RuneCountInString(base) == 1 {
		return true
	}
	if base == "" || base[0] != '\\' {
		return false
	}
	for i := 1; i < len(base); i++ {
		if !isASCIILetter(base[i]) {
			return false
		}
	}
	return true
}

func (c *converter) over(base string, overN *node, accentAttr string, st state) string {
	if overN.name == "mo" && accentAttr != "false" {
		if a, ok := overAccents[tokenText(overN)]; ok {
			cmd := a.wide
			if narrowBase(base) {
				cmd = a.narrow
			}
			return cmd + "{" + base + "}"
		}
	}
	o := c.conv(overN, st)
	if o == "" {
		return base
	}
	if singleGroupCommand(base, `\overbrace`) {
		return base + "^{" + o + "}"
	}
	return `\overset{` + o + `}{` + base + `}`
}

func (c *converter) under(base string, underN *node, accentAttr string, st state) string {
	if underN.name == "mo" && accentAttr != "false" {
		if a, ok := underAccents[tokenText(underN)]; ok {
			return a.narrow + "{" + base + "}"
		}
	}
	u := c.conv(underN, st)
	if u == "" {
		return base
	}
	if singleGroupCommand(base, `\underbrace`) {
		return base + "_{" + u + "}"
	}
	return `\underset{` + u + `}{` + base + `}`
}

func (c *converter) multiscripts(n *node, st state) string {
	els := n.elems()
	if len(els) == 0 {
		return ""
	}
	type pair struct{ sub, sup string }
	var post, pre []pair
	cur := &post
	rest := els[1:]
	for i := 0; i < len(rest); {
		if rest[i].name == "mprescripts" {
			cur = &pre
			i++
			continue
		}
		p := pair{sub: c.conv(rest[i], st)}
		if i+1 < len(rest) && rest[i+1].name != "mprescripts" {
			p.sup = c.conv(rest[i+1], st)
			i += 2
		} else {
			i++
		}
		*cur = append(*cur, p)
	}
	script := func(p pair) string {
		s := ""
		if p.sub != "" {
			s += "_{" + p.sub + "}"
		}
		if p.sup != "" {
			s += "^{" + p.sup + "}"
		}
		return s
	}
	var b strings.Builder
	for _, p := range pre {
		if s := script(p); s != "" {
			b.WriteString("{}" + s)
		}
	}
	b.WriteString(scriptBase(c.conv(els[0], st)))
	for i, p := range post {
		if s := script(p); s != "" {
			if i > 0 {
				b.WriteString("{}")
			}
			b.WriteString(s)
		}
	}
	return b.String()
}

func (c *converter) mfenced(n *node, st state) string {
	open, closer, seps := "(", ")", ","
	if v, ok := n.attrs["open"]; ok {
		open = strings.TrimSpace(v)
	}
	if v, ok := n.attrs["close"]; ok {
		closer = strings.TrimSpace(v)
	}
	if v, ok := n.attrs["separators"]; ok {
		seps = strings.Join(strings.Fields(v), "")
	}
	els := n.elems()
	if len(els) == 1 && els[0].name == "mtable" {
		if env, ok := matrixEnvs[open+closer]; ok {
			return c.table(els[0], st, env)
		}
		if open == "{" && closer == "" {
			return c.table(els[0], st, "cases")
		}
	}
	sepRunes := []rune(seps)
	var b builder
	for i, e := range els {
		if i > 0 && len(sepRunes) > 0 {
			k := min(i-1, len(sepRunes)-1)
			b.add(escMath(string(sepRunes[k])))
		}
		b.add(c.conv(e, st))
	}
	inner := b.String()
	lo, okL := delimiters[open]
	ro, okR := delimiters[closer]
	if okL && okR && n.attr("stretchy") != "false" && tallAny(els) {
		return join(`\left`+lo, inner, `\right`+ro)
	}
	return join(escMath(open), inner, escMath(closer))
}

func (c *converter) menclose(n *node, st state) string {
	inner := c.row(n.kids, st)
	notation := strings.Fields(n.attr("notation"))
	for _, nt := range notation {
		if nt == "box" || nt == "roundedbox" || nt == "circle" {
			return `\boxed{` + inner + `}`
		}
	}
	for _, nt := range notation {
		switch nt {
		case "top":
			inner = `\overline{` + inner + `}`
		case "bottom":
			inner = `\underline{` + inner + `}`
		case "radical":
			inner = `\sqrt{` + inner + `}`
		}
	}
	return inner
}

func (c *converter) style(n *node, st state) string {
	if v := n.attr("mathvariant"); v != "" {
		st.variant = v
	}
	inner := c.row(n.kids, st)
	if inner == "" {
		return ""
	}
	switch n.attr("displaystyle") {
	case "true":
		return `{\displaystyle ` + inner + `}`
	case "false":
		return `{\textstyle ` + inner + `}`
	}
	return inner
}

// table converts an mtable. env selects the environment; "" picks one from
// the column alignment (equation arrays become aligned).
func (c *converter) table(n *node, st state, env string) string {
	var rows [][]string
	cols := 0
	for _, r := range n.elems() {
		var cells []*node
		switch r.name {
		case "mtr":
			cells = r.elems()
		case "mlabeledtr":
			if cells = r.elems(); len(cells) > 0 {
				cells = cells[1:] // the equation label; numbering is the writer's job
			}
		case "mtd":
			cells = []*node{r}
		default:
			continue
		}
		row := make([]string, 0, len(cells))
		for _, cell := range cells {
			if cell.name == "mtd" {
				row = append(row, c.row(cell.kids, st))
			} else {
				row = append(row, c.conv(cell, st))
			}
		}
		rows = append(rows, row)
		cols = max(cols, len(row))
	}
	if len(rows) == 0 {
		return ""
	}
	spec, leftPad := "", false
	if env == "" {
		env, spec, leftPad = chooseEnv(n, rows, cols)
	}
	var b strings.Builder
	b.WriteString(`\begin{` + env + `}`)
	if spec != "" {
		b.WriteString("{" + spec + "}")
	}
	for i, row := range rows {
		if i > 0 {
			b.WriteString(` \\`)
		}
		b.WriteByte(' ')
		if leftPad {
			b.WriteByte('&')
		}
		b.WriteString(strings.Join(row, " & "))
	}
	b.WriteString(` \end{` + env + `}`)
	return b.String()
}

var relationStarts = []string{"=", "<", ">", `\leq`, `\geq`, `\neq`, `\approx`, `\equiv`, `\to`, `\Rightarrow`, `\Leftrightarrow`, `\sim`, `\simeq`, `\cong`, `\propto`, `\le`, `\ge`, `:=`}

func chooseEnv(n *node, rows [][]string, cols int) (env, spec string, leftPad bool) {
	aligns := strings.Fields(n.attr("columnalign"))
	if len(aligns) >= 2 && aligns[0] == "right" && aligns[1] == "left" {
		return "aligned", "", false
	}
	if cols == 1 {
		switch {
		case len(aligns) > 0 && aligns[0] == "left":
			return "aligned", "", true
		case len(aligns) > 0 && aligns[0] == "right":
			return "aligned", "", false
		}
		return "gathered", "", false
	}
	if len(aligns) == 0 && cols >= 2 && cols <= 3 && relationColumn(rows) {
		return "aligned", "", false
	}
	if len(aligns) > 0 {
		var sb strings.Builder
		for i := 0; i < cols; i++ {
			a := aligns[min(i, len(aligns)-1)]
			switch a {
			case "left":
				sb.WriteByte('l')
			case "right":
				sb.WriteByte('r')
			default:
				sb.WriteByte('c')
			}
		}
		return "array", sb.String(), false
	}
	return "matrix", "", false
}

// relationColumn reports whether every row's second cell starts with a
// relation, the shape of an unannotated equation array.
func relationColumn(rows [][]string) bool {
	for _, r := range rows {
		if len(r) < 2 {
			return false
		}
		ok := false
		for _, rel := range relationStarts {
			if strings.HasPrefix(strings.TrimSpace(r[1]), rel) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
