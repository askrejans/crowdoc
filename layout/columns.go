package layout

import (
	"math"
	"sort"
)

// channel is a vertical strip of whitespace that separates text columns.
type channel struct {
	x0, x1, y0, y1 float64
}

func (c channel) sep() float64 { return (c.x0 + c.x1) / 2 }

// obstacle is a horizontal band of the page with the x-intervals it
// covers (the words of a line, or an image).
type obstacle struct {
	y0, y1 float64
	xs     [][2]float64
	words  []*word // nil for images
}

// flow is a run of content in reading order inside one column of one zone
// of a page.
type flow struct {
	page        int
	items       []flowItem
	left, right float64
	multi       bool // the page region has several columns
	cols        int  // number of columns in the region
	ragged      bool // lines are set ragged right
}

type flowItem struct {
	l   *line
	img *pageImage
}

func (it flowItem) y0() float64 {
	if it.l != nil {
		return it.l.y0
	}
	return it.img.y0
}

// findGutters detects column gutters: whitespace channels at least minGap
// wide that run past several lines with substantial text on both sides.
func findGutters(p *pageData, body float64) []channel {
	obs := make([]obstacle, 0, len(p.lines)+len(p.images))
	for _, l := range p.lines {
		o := obstacle{y0: l.y0, y1: l.y1, words: l.words}
		for _, w := range l.words {
			o.xs = append(o.xs, [2]float64{w.x0, w.x1})
		}
		obs = append(obs, o)
	}
	for _, im := range p.images {
		obs = append(obs, obstacle{y0: im.y0, y1: im.y1, xs: [][2]float64{{im.x0, im.x1}}})
	}
	sort.SliceStable(obs, func(i, j int) bool { return obs[i].y0 < obs[j].y0 })
	minGap := 0.8 * body
	var out []channel
	for i, o := range obs {
		for k := 1; k < len(o.words); k++ {
			a, b := o.words[k-1], o.words[k]
			if b.x0-a.x1 < minGap {
				continue
			}
			cy := (o.y0 + o.y1) / 2
			covered := false
			for _, c := range out {
				if overlap(a.x1, b.x0, c.x0, c.x1) > 0 && cy >= c.y0 && cy <= c.y1 {
					covered = true
					break
				}
			}
			if covered {
				continue
			}
			if c, ok := traceChannel(obs, i, a.x1, b.x0, minGap, body); ok {
				out = append(out, c)
			}
		}
	}
	return out
}

// traceChannel follows the gap [x0, x1] of obstacle i up and down the page
// while it stays at least minGap wide, and judges whether it separates text
// columns.
func traceChannel(obs []obstacle, i int, x0, x1, minGap, body float64) (channel, bool) {
	c := channel{x0: x0, x1: x1, y0: obs[i].y0, y1: obs[i].y1}
	var leftW, rightW []float64
	var leftN, rightN []int
	support := 0
	measure := func(o obstacle) bool {
		touched := false
		if lw, n := sideSegment(o, c.x0, -1, minGap); n > 0 && c.x0-o.xs[0][0] >= 0 {
			if gapToSide(o, c.x0, -1) < 1.2*body {
				leftW = append(leftW, lw)
				leftN = append(leftN, n)
				touched = true
			}
		}
		if rw, n := sideSegment(o, c.x1, 1, minGap); n > 0 {
			if gapToSide(o, c.x1, 1) < 1.2*body {
				rightW = append(rightW, rw)
				rightN = append(rightN, n)
				touched = true
			}
		}
		if touched {
			support++
		}
		return touched
	}
	measure(obs[i])
	for _, dir := range []int{-1, 1} {
		for k := i + dir; k >= 0 && k < len(obs); k += dir {
			nx0, nx1, ok := shrink(c.x0, c.x1, obs[k].xs, minGap)
			if !ok {
				break
			}
			c.x0, c.x1 = nx0, nx1
			if dir < 0 {
				c.y0 = min(c.y0, obs[k].y0)
			} else {
				c.y1 = max(c.y1, obs[k].y1)
			}
			if obs[k].words != nil {
				measure(obs[k])
			}
		}
	}
	if support < 4 || len(leftW) < 3 || len(rightW) < 3 {
		return c, false
	}
	if median(leftW) < 8*body || median(rightW) < 8*body || medianInt(leftN) < 3 || medianInt(rightN) < 3 {
		return c, false
	}
	return c, true
}

func medianInt(v []int) float64 {
	f := make([]float64, len(v))
	for i, x := range v {
		f[i] = float64(x)
	}
	return median(f)
}

// shrink narrows [x0, x1] to the part not covered by xs. It keeps the
// piece containing the centre (or the widest piece) and fails when that is
// narrower than minGap.
func shrink(x0, x1 float64, xs [][2]float64, minGap float64) (float64, float64, bool) {
	parts := [][2]float64{{x0, x1}}
	for _, iv := range xs {
		if iv[1] <= x0 || iv[0] >= x1 {
			continue
		}
		var next [][2]float64
		for _, p := range parts {
			if iv[1] <= p[0] || iv[0] >= p[1] {
				next = append(next, p)
				continue
			}
			if iv[0] > p[0] {
				next = append(next, [2]float64{p[0], iv[0]})
			}
			if iv[1] < p[1] {
				next = append(next, [2]float64{iv[1], p[1]})
			}
		}
		parts = next
	}
	cx := (x0 + x1) / 2
	best := -1
	for i, p := range parts {
		if p[0] <= cx && p[1] >= cx {
			best = i
			break
		}
	}
	if best < 0 {
		for i, p := range parts {
			if best < 0 || p[1]-p[0] > parts[best][1]-parts[best][0] {
				best = i
			}
		}
	}
	if best < 0 || parts[best][1]-parts[best][0] < minGap {
		return 0, 0, false
	}
	return parts[best][0], parts[best][1], true
}

// sideSegment measures the text next to edge x on one side (dir -1 = left)
// up to the next wide gap: its width and word count.
func sideSegment(o obstacle, x float64, dir int, minGap float64) (float64, int) {
	ws := o.words
	n := 0
	var edge, far float64
	if dir < 0 {
		for k := len(ws) - 1; k >= 0; k-- {
			w := ws[k]
			if w.x1 > x+0.5 {
				continue
			}
			if n > 0 && far-w.x1 > minGap {
				break
			}
			if n == 0 {
				edge = w.x1
			}
			far = w.x0
			n++
		}
		return edge - far, n
	}
	for _, w := range ws {
		if w.x0 < x-0.5 {
			continue
		}
		if n > 0 && w.x0-far > minGap {
			break
		}
		if n == 0 {
			edge = w.x0
		}
		far = w.x1
		n++
	}
	return far - edge, n
}

// gapToSide is the distance from x to the nearest word on one side.
func gapToSide(o obstacle, x float64, dir int) float64 {
	best := 1e9
	for _, w := range o.words {
		if dir < 0 && w.x1 <= x+0.5 {
			best = min(best, x-w.x1)
		}
		if dir > 0 && w.x0 >= x-0.5 {
			best = min(best, w.x0-x)
		}
	}
	return best
}

// buildFlows splits a page into zones (bands with a constant set of
// gutters) and columns, and returns the content in reading order: zones
// top to bottom, columns left to right, lines top to bottom.
func buildFlows(p *pageData, gutters []channel) []*flow {
	type entry struct {
		y0, y1 float64
		l      *line
		img    *pageImage
	}
	var entries []entry
	for _, l := range p.lines {
		entries = append(entries, entry{y0: l.y0, y1: l.y1, l: l})
	}
	for _, im := range p.images {
		entries = append(entries, entry{y0: im.y0, y1: im.y1, img: im})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].y0 < entries[j].y0 })
	if len(gutters) == 0 {
		f := &flow{page: p.idx, cols: 1}
		for _, e := range entries {
			f.items = append(f.items, flowItem{l: e.l, img: e.img})
			if e.l != nil {
				e.l.fl = f
			}
		}
		f.bounds()
		return []*flow{f}
	}
	activeKey := func(cy float64) []int {
		var k []int
		for i, g := range gutters {
			if cy >= g.y0 && cy <= g.y1 {
				k = append(k, i)
			}
		}
		return k
	}
	type zone struct {
		key   []int
		items []entry
	}
	var zones []*zone
	for _, e := range entries {
		k := activeKey((e.y0 + e.y1) / 2)
		if len(zones) == 0 || !sameInts(zones[len(zones)-1].key, k) {
			zones = append(zones, &zone{key: k})
		}
		z := zones[len(zones)-1]
		z.items = append(z.items, e)
	}
	var out []*flow
	for _, z := range zones {
		seps := make([]float64, 0, len(z.key))
		for _, gi := range z.key {
			seps = append(seps, gutters[gi].sep())
		}
		sort.Float64s(seps)
		cols := make([]*flow, len(seps)+1)
		for i := range cols {
			cols[i] = &flow{page: p.idx, multi: len(seps) > 0, cols: len(seps) + 1}
		}
		colOf := func(x float64) int {
			c := 0
			for c < len(seps) && x > seps[c] {
				c++
			}
			return c
		}
		for _, e := range z.items {
			if e.img != nil {
				c := colOf((e.img.x0 + e.img.x1) / 2)
				cols[c].items = append(cols[c].items, flowItem{img: e.img})
				continue
			}
			parts := map[int][]*word{}
			for _, w := range e.l.words {
				c := colOf(w.cx())
				parts[c] = append(parts[c], w)
			}
			if len(parts) == 1 {
				for c := range parts {
					cols[c].items = append(cols[c].items, flowItem{l: e.l})
				}
				continue
			}
			for c := 0; c < len(cols); c++ {
				if ws := parts[c]; len(ws) > 0 {
					cols[c].items = append(cols[c].items, flowItem{l: e.l.subLine(ws)})
				}
			}
		}
		for _, f := range cols {
			for _, it := range f.items {
				if it.l != nil {
					it.l.fl = f
				}
			}
			if len(f.items) > 0 {
				sort.SliceStable(f.items, func(i, j int) bool { return f.items[i].y0() < f.items[j].y0() })
				f.bounds()
				out = append(out, f)
			}
		}
	}
	return out
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// bounds computes the left and right text edges of a flow from its body
// lines: code, much larger or smaller text and tiny fragments are ignored
// so that headings, listings and footnotes do not move the edges.
func (f *flow) bounds() {
	var all, body []*line
	sizes := map[float64]int{}
	for _, it := range f.items {
		if it.l != nil {
			all = append(all, it.l)
			sizes[roundTo(it.l.size, 0.5)] += it.l.chars()
		}
	}
	if len(all) == 0 {
		return
	}
	dom, domN := 0.0, -1
	for s, n := range sizes {
		if n > domN || n == domN && s < dom {
			dom, domN = s, n
		}
	}
	for _, l := range all {
		if _, _, m := l.styleFrac(); m >= 0.95 || l.chars() < 3 || math.Abs(l.size-dom) > 0.15*dom {
			continue
		}
		body = append(body, l)
	}
	if len(body) == 0 {
		body = all
	}
	xs0 := make([]float64, len(body))
	xs1 := make([]float64, len(body))
	for i, l := range body {
		xs0[i], xs1[i] = l.x0, l.x1
	}
	sort.Float64s(xs0)
	sort.Float64s(xs1)
	need := max(2, (len(body)+9)/10)
	if len(body) < 2 {
		need = 1
	}
	// The left edge is the leftmost position shared by several lines: a
	// single outdented line (hanging punctuation, a marker in the margin)
	// does not count, but hanging-indent paragraphs keep their first line
	// on the edge.
	f.left = xs0[0]
	for i, j := 0, 0; i < len(xs0); i++ {
		for j < len(xs0) && xs0[j]-xs0[i] <= 1.5 {
			j++
		}
		if j-i >= need {
			f.left = xs0[i]
			break
		}
	}
	// The right edge is where justified lines end; ragged text falls back
	// to a high percentile.
	f.right = xs1[len(xs1)*9/10]
	for i, j := len(xs1)-1, len(xs1)-1; i >= 0; i-- {
		for j >= 0 && xs1[i]-xs1[j] <= 1.5 {
			j--
		}
		if i-j >= need {
			if xs1[i] >= xs1[len(xs1)*6/10] {
				f.right = xs1[i]
			}
			break
		}
	}
	f.right = max(f.right, f.left+1)
}
