package tex2typ

import "strings"

// matrixDelims maps matrix environments to the Typst delim argument
// ("" keeps the default parentheses).
var matrixDelims = map[string]string{
	"matrix": "#none", "pmatrix": "", "bmatrix": `"["`, "Bmatrix": `"{"`,
	"vmatrix": `"|"`, "Vmatrix": `"‖"`, "smallmatrix": "#none",
	"psmallmatrix": "", "bsmallmatrix": `"["`, "Bsmallmatrix": `"{"`,
	"vsmallmatrix": `"|"`, "Vsmallmatrix": `"‖"`,
}

// pairGaps is the space amsmath leaves between "rl" column pairs.
var pairGaps = map[string]string{
	"align": "wide", "flalign": "wide", "xalignat": "wide", "xxalignat": "wide",
	"aligned": "quad", "split": "quad", "multlined": "quad",
}

// numberedEnvs are top-level environments that LaTeX numbers; the value
// tells whether each row gets its own number.
var numberedEnvs = map[string]bool{
	"equation": false, "multline": false, "align": true, "flalign": true,
	"alignat": true, "gather": true, "eqnarray": true, "xalignat": true,
}

// parseEnv parses \begin{name} … \end{name}; \begin was just consumed.
func (p *parser) parseEnv() *node {
	name := p.rawArg()
	base := strings.TrimSuffix(name, "*")
	star := base != name
	if _, ok := numberedEnvs[base]; ok && p.items <= 1 && star {
		p.starred = true
	}
	switch base {
	case "matrix", "pmatrix", "bmatrix", "Bmatrix", "vmatrix", "Vmatrix", "smallmatrix",
		"psmallmatrix", "bsmallmatrix", "Bsmallmatrix", "vsmallmatrix", "Vsmallmatrix":
		return p.parseMatrix(name, base, star)
	case "array", "darray", "tabular":
		return p.parseArray(name)
	case "subarray":
		p.rawArg()
		rows, _ := p.parseRows(name)
		var items []*node
		for i, r := range rows {
			if i > 0 {
				items = append(items, &node{kind: nBreak})
			}
			items = append(items, joinCells(r, ""))
		}
		return trimBreaks(seq(items...))
	case "cases", "dcases", "rcases", "drcases":
		return p.parseCases(name, base)
	case "aligned", "alignedat", "split", "gathered", "multlined", "align", "alignat",
		"flalign", "gather", "multline", "eqnarray", "xalignat", "xxalignat":
		return p.parseAlign(name, base)
	case "equation", "displaymath", "math":
		if base != "equation" && p.items <= 1 {
			p.starred = true
		}
		if p.items <= 1 {
			p.rows++
		}
		body := p.parseList(stopEnd)
		p.endEnv(name)
		return body
	}
	p.warn("unknown environment " + name + " rendered as aligned rows")
	return p.parseAlign(name, "aligned")
}

// endEnv consumes \end{name}.
func (p *parser) endEnv(name string) {
	if !p.peekCmd("end") {
		p.warn(`missing \end{` + name + `} inserted`)
		return
	}
	p.next()
	if got := p.rawArg(); got != name {
		p.warn(`\begin{` + name + `} ended by \end{` + got + `}`)
	}
}

// parseRows parses the cells of an environment body up to its \end. hlines
// lists the row indices before which a horizontal rule was requested.
func (p *parser) parseRows(name string) (rows [][]*node, hlines []int) {
	var cells []*node
	for {
		for {
			p.skipSpace()
			t := p.peek()
			if t.kind != tCmd {
				break
			}
			if t.val == "hline" || t.val == "hdashline" || t.val == "toprule" ||
				t.val == "midrule" || t.val == "bottomrule" {
				p.next()
				if len(hlines) == 0 || hlines[len(hlines)-1] != len(rows) {
					hlines = append(hlines, len(rows))
				}
				continue
			}
			if t.val == "cline" || t.val == "cmidrule" {
				p.next()
				p.rawOptArg()
				p.rawArg()
				continue
			}
			break
		}
		cells = append(cells, p.parseList(stopAlign|stopRow|stopEnd))
		t := p.peek()
		switch {
		case t.kind == tAlign:
			p.next()
		case isRowCmd(t):
			p.next()
			p.skipRowOpts()
			rows = append(rows, cells)
			cells = nil
		default:
			if !(len(cells) == 1 && cells[0].isEmpty()) {
				rows = append(rows, cells)
			}
			p.endEnv(name)
			return rows, hlines
		}
	}
}

func (p *parser) parseMatrix(name, base string, star bool) *node {
	align := ""
	if star {
		if opt, ok := p.rawOptArg(); ok {
			align = columnAlign(opt)
		}
	}
	rows, _ := p.parseRows(name)
	n := mat("mat", rows)
	if d := matrixDelims[base]; d != "" {
		n.with(code("delim", d))
	}
	if align != "" {
		n.with(code("align", align))
	}
	if strings.Contains(base, "small") {
		// Script-size entries, as in LaTeX; script() would shrink twice.
		return call("text", n).with(code("size", "#0.7em"))
	}
	return n
}

func columnAlign(c string) string {
	switch c {
	case "l":
		return "#left"
	case "r":
		return "#right"
	}
	return ""
}

func (p *parser) parseArray(name string) *node {
	p.rawOptArg()
	aligns, vlines := parseColSpec(p.rawArg())
	rows, hlines := p.parseRows(name)
	n := mat("mat", nil, code("delim", "#none"))
	uniform := true
	for _, a := range aligns {
		if a != aligns[0] {
			uniform = false
		}
	}
	if uniform && len(aligns) > 0 {
		if a := columnAlign(string(aligns[0])); a != "" {
			n.with(code("align", a))
		}
	} else {
		// Mixed alignment: an alignment point at the start of every cell of
		// a column left-aligns it, one at the end right-aligns it.
		for _, row := range rows {
			for j, cell := range row {
				if j >= len(aligns) || cell.isEmpty() {
					continue
				}
				switch aligns[j] {
				case 'l':
					row[j] = seq(&node{kind: nAlign}, cell)
				case 'r':
					row[j] = seq(cell, &node{kind: nAlign})
				}
			}
		}
	}
	ncols := 0
	for _, r := range rows {
		if len(r) > ncols {
			ncols = len(r)
		}
	}
	var aug []string
	if ncols == 0 {
		vlines, hlines = nil, nil
	}
	if v := clampLines(vlines, ncols); v != "" {
		aug = append(aug, "vline: "+v)
	}
	if h := clampLines(hlines, len(rows)); h != "" {
		aug = append(aug, "hline: "+h)
	}
	if len(aug) > 0 {
		n.with(code("augment", "#("+strings.Join(aug, ", ")+")"))
	}
	n.ext().rows = rows
	return n
}

// clampLines formats rule positions inside 0…max as a Typst array.
func clampLines(pos []int, max int) string {
	var b strings.Builder
	n := 0
	for _, v := range pos {
		if v < 0 || v > max {
			continue
		}
		if n > 0 {
			b.WriteString(", ")
		}
		b.WriteString(itoa(v))
		n++
	}
	if n == 0 {
		return ""
	}
	if n == 1 {
		b.WriteByte(',')
	}
	return "(" + b.String() + ")"
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// parseColSpec reads an array column specification such as "c|l@{}r".
func parseColSpec(spec string) (aligns []byte, vlines []int) {
	for i := 0; i < len(spec) && len(aligns) < 256; i++ {
		switch c := spec[i]; c {
		case 'l', 'c', 'r':
			aligns = append(aligns, c)
		case 'S', 'X', 'Y':
			aligns = append(aligns, 'c')
		case 'p', 'm', 'b':
			aligns = append(aligns, 'l')
			i = skipBraced(spec, i+1)
		case '|', ':':
			vlines = append(vlines, len(aligns))
		case '@', '!', '>', '<':
			i = skipBraced(spec, i+1)
		case '*':
			j := skipBraced(spec, i+1)
			count := atoiSafe(strings.TrimSpace(braced(spec, i+1)))
			k := skipBraced(spec, j+1)
			inner := braced(spec, j+1)
			if count > 32 {
				count = 32
			}
			for ; count > 0; count-- {
				a, v := parseColSpec(inner)
				for _, x := range v {
					vlines = append(vlines, len(aligns)+x)
				}
				aligns = append(aligns, a...)
			}
			i = k
		}
	}
	return aligns, vlines
}

// skipBraced returns the index of the '}' closing the group that starts at
// or after i, or len(s)-1 when there is none.
func skipBraced(s string, i int) int {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i >= len(s) || s[i] != '{' {
		return i - 1
	}
	depth := 0
	for ; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return len(s) - 1
}

func braced(s string, i int) string {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i >= len(s) || s[i] != '{' {
		return ""
	}
	j := skipBraced(s, i)
	if j <= i {
		return ""
	}
	return s[i+1 : j]
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' || n > 1000 {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func (p *parser) parseCases(name, base string) *node {
	rows, _ := p.parseRows(name)
	n := mat("cases", nil)
	for _, r := range rows {
		// LaTeX separates the value from the condition by a quad.
		var items []*node
		for i, c := range r {
			if i > 0 {
				items = append(items, &node{kind: nAlign}, word("quad"))
			}
			items = append(items, c)
		}
		row := seq(items...)
		if base[0] == 'd' {
			row = displayItems(row)
		}
		n.x.rows = append(n.x.rows, []*node{row})
	}
	if strings.HasPrefix(base, "r") || base == "drcases" {
		n.with(code("reverse", "#true"))
	}
	return n
}

// joinCells joins the cells of a row with alignment points, adding gap
// after every second cell that is followed by another pair.
func joinCells(cells []*node, gap string) *node {
	var items []*node
	for i, c := range cells {
		if i > 0 {
			items = append(items, &node{kind: nAlign})
		}
		items = append(items, c)
		if gap != "" && i%2 == 1 && i < len(cells)-1 {
			items = append(items, word(gap))
		}
	}
	return seq(items...)
}

func (p *parser) parseAlign(name, base string) *node {
	switch base {
	case "alignat", "alignedat", "xalignat", "xxalignat":
		p.rawArg()
	}
	switch base {
	case "aligned", "alignedat", "gathered", "split", "multlined":
		p.rawOptArg()
		p.rawOptArg()
	}
	rows, _ := p.parseRows(name)
	if each, ok := numberedEnvs[base]; ok && p.items <= 1 {
		if each {
			p.rows += len(rows)
		} else {
			p.rows++
		}
	}
	out := &node{kind: nRows}
	for _, r := range rows {
		if base == "eqnarray" && len(r) >= 3 {
			// rcl columns: keep the relation with the right-hand side.
			merged := append([]*node{r[0], seq(r[1], r[2])}, r[3:]...)
			r = merged
		}
		out.kids = append(out.kids, joinCells(r, pairGaps[base]))
	}
	return out
}
