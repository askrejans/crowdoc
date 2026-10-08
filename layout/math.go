package layout

import (
	"sort"
	"strings"
	"unicode"

	"github.com/askrejans/crowdoc/v2/ast"
	"golang.org/x/text/unicode/norm"
)

// isMathRune reports whether r is typical of typeset mathematics.
func isMathRune(r rune) bool {
	switch {
	case r >= 0x1D400 && r <= 0x1D7FF, r == 0x210E: // mathematical alphanumerics
		return true
	case r >= 0x2200 && r <= 0x22FF, r >= 0x27C0 && r <= 0x27EF, r >= 0x2980 && r <= 0x2AFF: // operators
		return true
	case r >= 0x0391 && r <= 0x03C9: // Greek
		return true
	}
	return strings.ContainsRune("=+<>±×÷√∞′″⁄", r)
}

// isMathLine reports whether a line is display mathematics: tagged as a
// formula, or set apart from the left margin and made mostly of
// mathematical symbols.
func (d *doc) isMathLine(l *line, f *flow) bool {
	switch l.role() {
	case RoleFormula:
		return true
	case RoleAuto:
	default:
		return false
	}
	n, m := 0, 0
	for _, w := range l.words {
		for _, r := range w.text {
			n++
			if isMathRune(r) {
				m++
			}
		}
	}
	if n == 0 || n > 160 || m*10 < n*4 {
		return false
	}
	return l.x0 > f.left+1.5*l.size
}

// collectMath gathers a display formula: lines of the same tagged block,
// or untagged math lines together with the small limit lines above and
// below them.
func (d *doc) collectMath(items []flowItem, i int, f *flow) (*proto, int) {
	first := items[i].l
	p := &proto{kind: pMath, flow: f, lines: []*line{first}}
	blk := first.block()
	j := i + 1
	for ; j < len(items); j++ {
		n := items[j].l
		if n == nil {
			break
		}
		prev := p.lines[len(p.lines)-1]
		if blk != 0 {
			if n.block() != blk {
				break
			}
		} else if n.y0-prev.y1 > 0.6*max(n.size, prev.size) || !d.isMathLine(n, f) && !(n.size < 0.9*prev.size && n.chars() <= 12) {
			break
		}
		p.lines = append(p.lines, n)
	}
	return p, j
}

// mathBlock renders a display formula as a centred paragraph. The lines of
// the formula (limits above and below an integral, the rows of a fraction)
// are merged left to right; smaller raised and lowered pieces become
// superscripts and subscripts.
func (d *doc) mathBlock(p *proto) ast.Block {
	var ws []*word
	for _, l := range p.lines {
		ws = append(ws, l.words...)
	}
	sort.SliceStable(ws, func(i, j int) bool { return ws[i].x0 < ws[j].x0 })
	merged := p.lines[0].subLine(ws)
	ins := d.inlines([]*line{merged}, inlineOpts{noNotes: true})
	if len(ins) == 0 {
		return nil
	}
	return &ast.Div{Attr: ast.Attr{Classes: []string{"center"}}, Blocks: []ast.Block{&ast.Para{Inlines: ins}}}
}

// mathText replaces mathematical alphanumeric symbols (𝐻, 𝜋, 𝐱) with plain
// letters, which every text font has, and reports whether they were
// italic or bold.
func mathText(s string) (string, bool, bool) {
	if strings.IndexFunc(s, func(r rune) bool { return r >= 0x1D400 && r <= 0x1D7FF || r == 0x210E }) < 0 {
		return s, false, false
	}
	var sb strings.Builder
	italic, bold, n := 0, 0, 0
	for _, r := range s {
		it, bd, ok := mathStyle(r)
		if !ok {
			sb.WriteRune(r)
			continue
		}
		n++
		if it {
			italic++
		}
		if bd {
			bold++
		}
		for _, c := range norm.NFKC.String(string(r)) {
			if unicode.IsPrint(c) {
				sb.WriteRune(c)
			}
		}
	}
	return sb.String(), italic*2 > n, bold*2 > n
}

// mathStyle classifies a mathematical alphanumeric symbol.
func mathStyle(r rune) (italic, bold, ok bool) {
	switch {
	case r == 0x210E:
		return true, false, true
	case r >= 0x1D400 && r < 0x1D6A4:
		// Thirteen Latin alphabets of 52 letters each: bold, italic, bold
		// italic, script, bold script, fraktur, double-struck, bold
		// fraktur, sans, sans bold, sans italic, sans bold italic, mono.
		switch (r - 0x1D400) / 52 {
		case 0, 4, 7, 9:
			return false, true, true
		case 1, 10:
			return true, false, true
		case 2, 11:
			return true, true, true
		}
		return false, false, true
	case r >= 0x1D6A8 && r < 0x1D7CA:
		// Five Greek alphabets of 58 symbols: bold, italic, bold italic,
		// sans bold, sans bold italic.
		switch (r - 0x1D6A8) / 58 {
		case 0, 3:
			return false, true, true
		case 1:
			return true, false, true
		}
		return true, true, true
	case r >= 0x1D7CE && r <= 0x1D7FF:
		k := (r - 0x1D7CE) / 10
		return false, k == 0 || k == 3, true
	case r >= 0x1D400 && r <= 0x1D7FF:
		return true, false, true // dotless i and j
	}
	return false, false, false
}
