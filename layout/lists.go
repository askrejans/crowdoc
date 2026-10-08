package layout

import (
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// listNode is a list being assembled; items hold paragraphs (line groups)
// and nested lists in order.
type listNode struct {
	ordered bool
	style   int
	start   int
	x       float64 // marker position
	items   []*listItem
}

type listItem struct {
	textX  float64
	parts  []listPart
	marker marker
}

type listPart struct {
	lines []*line
	sub   *listNode
}

// ambiguousBullets also occur at the start of ordinary text lines (dashes
// in prose, multiplication, a lone letter).
var ambiguousBullets = map[string]bool{"-": true, "–": true, "—": true, "*": true, "o": true, "·": true}

// lineMarker recognises a list marker at the start of a line. Markers that
// also occur in running text (dashes, "1990.", "8.") need context: prev is
// the preceding line, and the marker only counts when that line ended a
// paragraph. A nil prev skips the check (inside a list).
func (d *doc) lineMarker(l *line, f *flow, prev *line) (marker, bool) {
	m, ok := d.rawMarker(l, f)
	if !ok || prev == nil || (!m.ordered && !ambiguousBullets[m.text]) || l.words[0].role == RoleListLabel {
		return m, ok
	}
	pt := prev.text()
	switch {
	case short(prev, f), strings.HasSuffix(pt, ":"), strings.HasSuffix(pt, ";"):
		return m, true
	case l.base-prev.base > d.pitchFor(prev.size)*1.3+0.5:
		return m, true
	case endsSentence(pt) && (l.x0 > prev.x0+0.4*l.size || m.ordered && m.num == 1):
		// Indented below a finished sentence, or the first item of a
		// numbered list.
		return m, true
	}
	if pm, ok := d.rawMarker(prev, f); ok && pm.ordered == m.ordered {
		return m, true
	}
	return marker{}, false
}

func (d *doc) rawMarker(l *line, f *flow) (marker, bool) {
	if len(l.words) < 2 {
		return marker{}, false
	}
	switch l.role() {
	case RoleAuto, RoleListBody, RoleListLabel:
	default:
		// The producer tagged the line as something else.
		return marker{}, false
	}
	w := l.words[0]
	m, ok := parseMarker(w.text)
	n := 1
	// Some list renderers draw the marker a second time at the tab stop.
	if len(l.words) >= 3 && l.words[1].text == w.text {
		if !ok && len(w.text) <= 3 && strings.Trim(w.text, "0123456789") == "" {
			m, ok = parseMarker(w.text + ".")
		}
		n = 2
	}
	next := l.words[n]
	gap := next.x0 - l.words[n-1].x1
	if !ok {
		if w.text == "o" && gap >= 0.8*l.size {
			m, ok = marker{text: "o"}, true
		}
	}
	if !ok || w.sup {
		return marker{}, false
	}
	if gap < 0.15*l.size {
		return marker{}, false
	}
	m.x, m.textX, m.gap, m.words = w.x0, next.x0, gap, n
	tagged := w.role == RoleListLabel
	if m.ordered && !tagged && (gap < 0.25*l.size || w.bold && l.size > d.body*1.1) {
		return marker{}, false
	}
	return m, true
}

func newList(m marker) *listNode {
	return &listNode{ordered: m.ordered, style: m.style, start: m.num, x: m.x}
}

func (n *listNode) add(m marker, l *line) *listItem {
	it := &listItem{textX: m.textX, marker: m}
	if l != nil {
		it.parts = append(it.parts, listPart{lines: []*line{l}})
	}
	n.items = append(n.items, it)
	return it
}

// withoutMarker returns the line without its marker words.
func withoutMarker(l *line, m marker) *line {
	n := max(m.words, 1)
	if len(l.words) <= n {
		return nil
	}
	return l.subLine(l.words[n:])
}

// collectList gathers a (possibly nested) list starting at item i.
func (d *doc) collectList(items []flowItem, i int, f *flow, tables map[int]tableRegion) (*proto, int, bool) {
	first := items[i].l
	m0, _ := d.lineMarker(first, f, nil)
	root := newList(m0)
	stack := []*listNode{root}
	cur := root.add(m0, withoutMarker(first, m0))
	lastLine := first
	size := first.size
	j := i + 1
	for j < len(items) {
		if _, ok := tables[j]; ok {
			break
		}
		n := items[j].l
		if n == nil || d.isCode(n) || d.isHeadingLine(n) {
			break
		}
		pitch := n.base - lastLine.base
		exp := d.pitchFor(lastLine.size)
		if m, ok := d.lineMarker(n, f, nil); ok {
			if pitch > exp*2.6 || m.x < root.x-1.5*size {
				break
			}
			top := stack[len(stack)-1]
			switch {
			case m.x > top.x+0.5*size && m.x >= cur.textX-0.8*size:
				sub := newList(m)
				cur.parts = append(cur.parts, listPart{sub: sub})
				stack = append(stack, sub)
				top = sub
			case m.x < top.x-0.5*size:
				for len(stack) > 1 && stack[len(stack)-1].x > m.x+0.5*size {
					stack = stack[:len(stack)-1]
				}
				top = stack[len(stack)-1]
			}
			if top == root && m.ordered != root.ordered {
				break
			}
			cur = top.add(m, withoutMarker(n, m))
			lastLine = n
			j++
			continue
		}
		// Continuation of the current item: indented to its text.
		if n.x0 < cur.textX-0.6*n.size || pitch > exp*2.6 {
			break
		}
		if kind, _ := captionLabel(n.text()); kind != "" {
			break
		}
		if styleFlip(lastLine, n) && pitch > exp*1.1 {
			break
		}
		if r := n.role(); r != RoleAuto && r != RoleListBody && r != RoleListLabel {
			break
		}
		switch {
		case len(cur.parts) > 0 && cur.parts[len(cur.parts)-1].sub == nil && pitch <= exp*1.3+0.5:
			pp := &cur.parts[len(cur.parts)-1]
			pp.lines = append(pp.lines, n)
		case pitch <= exp*1.8 && n.x0 <= cur.textX+0.5*n.size:
			// A further paragraph of the same item.
			cur.parts = append(cur.parts, listPart{lines: []*line{n}})
		default:
			goto done
		}
		lastLine = n
		j++
	}
done:
	if root.ordered && len(root.items) == 1 && root.items[0].marker.gap < 0.8*size {
		return nil, i, false
	}
	p := &proto{kind: pList, flow: f, list: root}
	for k := i; k < j; k++ {
		p.lines = append(p.lines, items[k].l)
	}
	return p, j, true
}

// listBlock converts an assembled list to an ast.List.
func (d *doc) listBlock(n *listNode) *ast.List {
	l := &ast.List{Ordered: n.ordered, Tight: true}
	if n.ordered {
		l.Style = ast.NumberStyle(n.style)
		if n.start > 1 {
			l.Start = n.start
		}
		// "i." after "h." is a letter, not a roman numeral.
		fixRomanLetters(n, l)
	}
	for _, it := range n.items {
		item := ast.ListItem{Task: it.marker.task}
		paras := 0
		for _, p := range it.parts {
			if p.sub != nil {
				item.Blocks = append(item.Blocks, d.listBlock(p.sub))
				continue
			}
			if ins := d.inlines(p.lines, inlineOpts{}); len(ins) > 0 {
				item.Blocks = append(item.Blocks, &ast.Plain{Inlines: ins})
				paras++
			}
		}
		if paras > 1 {
			l.Tight = false
		}
		l.Items = append(l.Items, item)
	}
	if !l.Tight {
		for _, it := range l.Items {
			for k, b := range it.Blocks {
				if p, ok := b.(*ast.Plain); ok {
					it.Blocks[k] = &ast.Para{Inlines: p.Inlines}
				}
			}
		}
	}
	return l
}

func fixRomanLetters(n *listNode, l *ast.List) {
	if len(n.items) == 0 {
		return
	}
	first := n.items[0].marker
	if (first.style == 3 || first.style == 4) && first.num == 1 && len(n.items) > 1 {
		second := n.items[1].marker
		if second.letter == "ii" || second.letter == "II" {
			return
		}
		// "i", "j"… is a letter sequence starting at i.
		if first.style == 3 {
			l.Style = ast.NumberLowerAlpha
		} else {
			l.Style = ast.NumberUpperAlpha
		}
		l.Start = 9
	}
}
