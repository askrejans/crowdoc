package layout

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

// line is a visual line: words sharing a baseline, sorted left to right.
// Lines of different columns that happen to share a baseline are split
// later by the column analysis.
type line struct {
	words          []*word
	x0, y0, x1, y1 float64
	size           float64 // dominant font size (by characters)
	base           float64 // dominant bottom edge, a baseline proxy
	page           int
	ocr            bool  // word sizes came from box heights
	fl             *flow // the flow (column) holding the line

	coreY0, coreY1, coreSize float64 // extent of the largest word, for grouping
}

func (l *line) h() float64 { return l.y1 - l.y0 }

// buildLines groups the words of one page into visual lines.
func buildLines(ws []*word, page int, ocr bool) []*line {
	sort.SliceStable(ws, func(i, j int) bool {
		ci, cj := ws[i].cy(), ws[j].cy()
		if ci != cj {
			return ci < cj
		}
		return ws[i].x0 < ws[j].x0
	})
	var lines []*line
	for _, w := range ws {
		var best *line
		bestOv := 0.0
		for k := len(lines) - 1; k >= 0 && k >= len(lines)-80; k-- {
			l := lines[k]
			ref0, ref1 := l.coreY0, l.coreY1
			// Compare with the nearest word: on a page-wide line, text in
			// another column may have another size.
			if nw, dist := l.nearest(w); nw != nil && dist < 4*max(w.size, nw.size) {
				ref0, ref1 = nw.y0, nw.y1
			}
			ch := ref1 - ref0
			if ch <= 0 {
				continue
			}
			ov := overlap(w.y0, w.y1, ref0, ref1) / min(w.h(), ch)
			if ov < 0.5 || ov <= bestOv || l.collides(w) {
				continue
			}
			best, bestOv = l, ov
		}
		if best == nil {
			lines = append(lines, &line{words: []*word{w}, coreY0: w.y0, coreY1: w.y1, coreSize: w.size, page: page, ocr: ocr})
			continue
		}
		best.words = append(best.words, w)
		if w.size > best.coreSize*1.1 {
			best.coreY0, best.coreY1, best.coreSize = w.y0, w.y1, w.size
		}
	}
	for _, l := range lines {
		sort.SliceStable(l.words, func(i, j int) bool { return l.words[i].x0 < l.words[j].x0 })
		l.markScripts()
	}
	sortLines(lines)
	return lines
}

func sortLines(lines []*line) {
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].y0 != lines[j].y0 {
			return lines[i].y0 < lines[j].y0
		}
		return lines[i].x0 < lines[j].x0
	})
}

func overlap(a0, a1, b0, b1 float64) float64 {
	return max(0, min(a1, b1)-max(a0, b0))
}

// nearest returns the word of the line horizontally closest to w and its
// distance.
func (l *line) nearest(w *word) (*word, float64) {
	var best *word
	bestD := math.Inf(1)
	for _, o := range l.words {
		d := max(o.x0-w.x1, w.x0-o.x1, 0)
		if d < bestD {
			best, bestD = o, d
		}
	}
	return best, bestD
}

// collides reports whether w would overprint a word already on the line.
// Smaller scripts tucked under or over the end of a word (β₁, x²) do not
// count.
func (l *line) collides(w *word) bool {
	for _, o := range l.words {
		ov := overlap(w.x0, w.x1, o.x0, o.x1)
		if ov <= 0.5*min(w.w(), o.w()) || ov <= 0.5 {
			continue
		}
		ratio := w.size / max(o.size, 0.01)
		if ratio < 0.8 || ratio > 1.25 {
			continue
		}
		if overlap(w.y0, w.y1, o.y0, o.y1) > 0.7*min(w.h(), o.h()) {
			return true
		}
	}
	return false
}

// markScripts computes the line geometry, the dominant size and baseline,
// and marks raised and lowered words.
func (l *line) markScripts() {
	for _, w := range l.words {
		w.sup, w.sub = false, false
	}
	l.recompute()
	if l.ocr {
		l.markOCRScripts()
		return
	}
	for _, w := range l.words {
		if w.size >= 0.88*l.size {
			continue
		}
		switch {
		case w.y1 < l.base-0.18*l.size:
			w.sup = true
		case w.y1 > l.base+0.12*l.size && w.y0 > l.y0+0.25*l.size:
			w.sub = true
		}
	}
}

// markOCRScripts handles words whose size came from their box height:
// heights vary with ascenders and descenders, so only clearly raised small
// boxes count as superscripts, and every other word gets the line size.
func (l *line) markOCRScripts() {
	ems := make([]float64, 0, len(l.words))
	for _, w := range l.words {
		if w.em > 0 {
			ems = append(ems, w.em)
		}
	}
	if len(ems) == 0 {
		for _, w := range l.words {
			ems = append(ems, w.h())
		}
	}
	m := median(ems)
	for _, w := range l.words {
		if w.em > 0 && w.em < 0.7*m && w.y1 < l.base-0.25*m {
			w.sup = true
			w.size = w.em
			continue
		}
		w.size = m
	}
	l.recompute()
}

func (l *line) recompute() {
	l.x0, l.y0 = l.words[0].x0, l.words[0].y0
	l.x1, l.y1 = l.words[0].x1, l.words[0].y1
	sizes := map[float64]int{}
	for _, w := range l.words {
		l.x0, l.x1 = min(l.x0, w.x0), max(l.x1, w.x1)
		l.y0, l.y1 = min(l.y0, w.y0), max(l.y1, w.y1)
		sizes[roundTo(w.size, 0.25)] += utf8.RuneCountInString(w.text)
	}
	best, bestN := 0.0, -1
	for s, n := range sizes {
		if n > bestN || (n == bestN && s > best) {
			best, bestN = s, n
		}
	}
	l.size = best
	var bottoms []float64
	for _, w := range l.words {
		if roundTo(w.size, 0.25) == best {
			bottoms = append(bottoms, w.y1)
		}
	}
	l.base = median(bottoms)
}

func roundTo(x, step float64) float64 {
	return float64(int(x/step+0.5)) * step
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)/2]
}

// spaceBefore reports whether a space separates word b from the preceding
// word a on the same line.
func spaceBefore(a, b *word) bool {
	gap := b.x0 - a.x1
	sz := min(a.size, b.size)
	if a.sup || a.sub || b.sup || b.sub {
		return gap > 0.2*max(a.size, b.size)
	}
	// OCR engines report whole words with tight boxes: two of them are two
	// words even when the gap is narrow. Single characters may still be
	// glyph-level boxes, so they keep the geometric rule.
	if a.ocr && b.ocr && utf8.RuneCountInString(a.text) > 1 && utf8.RuneCountInString(b.text) > 1 {
		return gap > -0.3*sz
	}
	return gap > 0.12*sz
}

// text returns the plain text of the line.
func (l *line) text() string {
	return wordsText(l.words)
}

func wordsText(ws []*word) string {
	var sb strings.Builder
	for i, w := range ws {
		if i > 0 && spaceBefore(ws[i-1], w) {
			sb.WriteByte(' ')
		}
		sb.WriteString(w.text)
	}
	return sb.String()
}

// styleFrac returns the fraction of characters that are bold, italic and
// monospaced.
func (l *line) styleFrac() (bold, italic, mono float64) {
	n := 0
	var b, i, m int
	for _, w := range l.words {
		c := utf8.RuneCountInString(w.text)
		n += c
		if w.bold {
			b += c
		}
		if w.italic {
			i += c
		}
		if w.mono {
			m += c
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	return float64(b) / float64(n), float64(i) / float64(n), float64(m) / float64(n)
}

// block returns the block hint shared by most of the line's characters,
// or 0.
func (l *line) block() int {
	if len(l.words) == 0 || l.words[0].block == 0 && l.words[len(l.words)-1].block == 0 {
		return 0
	}
	counts := map[int]int{}
	best, bestN := 0, 0
	for _, w := range l.words {
		n := counts[w.block] + utf8.RuneCountInString(w.text)
		counts[w.block] = n
		if n > bestN {
			best, bestN = w.block, n
		}
	}
	return best
}

// role returns the role hint shared by most of the line's characters.
func (l *line) role() Role {
	var counts [32]int
	best, bestN := RoleAuto, 0
	for _, w := range l.words {
		if int(w.role) >= len(counts) {
			continue
		}
		n := counts[w.role] + utf8.RuneCountInString(w.text)
		counts[w.role] = n
		if n > bestN {
			best, bestN = w.role, n
		}
	}
	return best
}

func (l *line) chars() int {
	n := 0
	for _, w := range l.words {
		n += utf8.RuneCountInString(w.text)
	}
	return n
}

// subLine returns a new line holding ws (a subset of l's words), with
// superscripts re-evaluated against the new line.
func (l *line) subLine(ws []*word) *line {
	n := &line{words: ws, page: l.page, ocr: l.ocr, fl: l.fl}
	n.markScripts()
	return n
}
