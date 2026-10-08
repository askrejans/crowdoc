package layout

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// footnote is a note found at the bottom of a page.
type footnote struct {
	page   int
	marker string
	lines  []*line
	used   bool
	note   *ast.Note
}

var (
	noteMarkerRe = regexp.MustCompile(`^(?:\d{1,3}|[*∗†‡§¶#]{1,3}|[a-z])[.)]?$`)
	// gluedMarkerRe matches a note marker printed without a space before
	// the note text ("1Data are …").
	gluedMarkerRe = regexp.MustCompile(`^(\d{1,3}|[*∗†‡§¶]{1,3})\pL`)
)

func markerKey(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), ".),;:")
}

// noteRefs returns the superscript note markers used in the body lines of a
// flow.
func noteRefs(f *flow) map[string]bool {
	refs := map[string]bool{}
	for _, it := range f.items {
		if it.l == nil {
			continue
		}
		for _, w := range it.l.words {
			if (w.sup || w.role == RoleNoteRef) && w.role != RoleNote && noteMarkerRe.MatchString(w.text) {
				refs[markerKey(w.text)] = true
			}
		}
	}
	return refs
}

// extractFootnotes removes the footnote area (small text below the body,
// each note starting with a marker) from the bottom of a flow.
func (d *doc) extractFootnotes(f *flow, p *pageData) {
	if d.extractTaggedNotes(f) {
		return
	}
	n := len(f.items)
	if n < 2 || d.body <= 0 {
		return
	}
	// The run of small lines at the bottom of the flow.
	start := n
	for k := n - 1; k >= 0; k-- {
		it := f.items[k]
		if it.l == nil || it.l.size > d.body*0.93 || it.l.y1 < 0.45*p.h {
			break
		}
		start = k
	}
	if start == n || start == 0 {
		return
	}
	refs := noteRefs(f)
	// The area begins at the first line (from the top of that run) that
	// starts with a marker, below a visible gap.
	begin := -1
	for k := start; k < n; k++ {
		if _, _, ok := noteStart(f.items[k].l, f, refs); ok {
			begin = k
			break
		}
	}
	if begin < 0 {
		return
	}
	// Small lines above the first marker that are set apart from the body
	// continue the last note of the previous page.
	top := begin
	if begin > start && len(d.notes) > 0 && d.notes[len(d.notes)-1].page == f.page-1 {
		top = start
	}
	prev := f.items[top-1]
	if prev.l == nil {
		return
	}
	first := f.items[top].l
	gap := first.y0 - prev.l.y1
	if gap < 0.4*first.size || (prev.l.size < first.size*1.06 && gap < first.size) {
		return
	}
	var cur *footnote
	if top < begin {
		cur = d.notes[len(d.notes)-1]
	}
	for k := top; k < n; k++ {
		l := f.items[k].l
		if m, rest, ok := noteStart(l, f, refs); ok {
			cur = &footnote{page: f.page, marker: markerKey(m)}
			d.notes = append(d.notes, cur)
			if len(rest) > 0 {
				cur.lines = append(cur.lines, l.subLine(rest))
			}
			continue
		}
		if cur != nil {
			cur.lines = append(cur.lines, l)
		}
	}
	f.items = f.items[:top]
}

// extractTaggedNotes moves lines whose role is RoleNote into footnotes,
// one per block. It reports false when the flow has no such lines.
func (d *doc) extractTaggedNotes(f *flow) bool {
	found := false
	for _, it := range f.items {
		if it.l != nil && it.l.role() == RoleNote {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	refs := noteRefs(f)
	kept := f.items[:0]
	var cur *footnote
	curBlock := 0
	for _, it := range f.items {
		l := it.l
		if l == nil || l.role() != RoleNote {
			kept = append(kept, it)
			continue
		}
		blk := l.block()
		m, rest, marked := noteStart(l, f, refs)
		if !marked && len(l.words) > 1 && noteMarkerRe.MatchString(l.words[0].text) {
			m, rest, marked = l.words[0].text, l.words[1:], true
		}
		if cur != nil && (blk == curBlock && blk != 0 || blk == 0 && !marked) {
			cur.lines = append(cur.lines, l)
			continue
		}
		cur = &footnote{page: f.page}
		curBlock = blk
		d.notes = append(d.notes, cur)
		if !marked {
			cur.lines = append(cur.lines, l)
			continue
		}
		cur.marker = markerKey(m)
		if len(rest) > 0 {
			cur.lines = append(cur.lines, l.subLine(rest))
		}
	}
	f.items = kept
	return true
}

// noteStart reports whether a line starts a footnote and returns its
// marker and the remaining words. refs holds the markers referenced in the
// body, which also identify markers glued to the note text.
func noteStart(l *line, f *flow, refs map[string]bool) (string, []*word, bool) {
	if len(l.words) == 0 {
		return "", nil, false
	}
	w := l.words[0]
	if noteMarkerRe.MatchString(w.text) && len(l.words) >= 2 {
		if w.sup {
			return w.text, l.words[1:], true
		}
		// Plain markers must be digits or symbols at the left edge.
		lowerLetter := len(w.text) == 1 && w.text[0] >= 'a' && w.text[0] <= 'z'
		if w.x0 <= f.left+2*l.size && !lowerLetter {
			return w.text, l.words[1:], true
		}
		return "", nil, false
	}
	if m := gluedMarkerRe.FindStringSubmatch(w.text); m != nil && refs[m[1]] {
		head, tail := splitWord(w, len(m[1]))
		return head.text, append([]*word{tail}, l.words[1:]...), true
	}
	return "", nil, false
}

// splitWord cuts w after n bytes of text into two words with proportional
// boxes.
func splitWord(w *word, n int) (*word, *word) {
	total := utf8.RuneCountInString(w.text)
	k := utf8.RuneCountInString(w.text[:n])
	x := w.x0 + w.w()*float64(k)/float64(max(total, 1))
	a, b := *w, *w
	a.text, a.x1 = w.text[:n], x
	b.text, b.x0 = w.text[n:], x
	return &a, &b
}

// matchNotes pairs footnotes with their reference marks. Notes on a page
// are numbered in the order of their references, so each note takes the
// first matching superscript after the previous note's reference: a
// superscript "2" earlier on the page (a power, say) is not taken for the
// reference of note 2.
func (d *doc) matchNotes(ps []*proto) {
	if len(d.notes) == 0 {
		return
	}
	type cand struct {
		w    *word
		page int
	}
	byPage := map[int][]*word{}
	visit := func(ls []*line) {
		for _, l := range ls {
			for _, w := range l.words {
				if (w.sup || w.role == RoleNoteRef) && w.role != RoleNote && noteMarkerRe.MatchString(w.text) {
					byPage[l.page] = append(byPage[l.page], w)
				}
			}
		}
	}
	for _, p := range ps {
		visit(p.lines)
		visit(p.caption)
	}
	d.noteOf = map[*word]*footnote{}
	next := map[int]int{} // per page: index of the first candidate not yet passed
	for _, fn := range d.notes {
		cands := byPage[fn.page]
		for k := next[fn.page]; k < len(cands); k++ {
			if markerKey(cands[k].text) == fn.marker {
				d.noteOf[cands[k]] = fn
				fn.used = true
				next[fn.page] = k + 1
				break
			}
		}
	}
}

func (d *doc) noteNode(fn *footnote) *ast.Note {
	if fn.note == nil {
		ins := d.inlines(fn.lines, inlineOpts{noNotes: true})
		fn.note = &ast.Note{}
		if len(ins) > 0 {
			fn.note.Blocks = []ast.Block{&ast.Para{Inlines: ins}}
		}
	}
	return fn.note
}

// endNotes renders footnotes whose marker was not found in the text.
func (d *doc) endNotes() []ast.Block {
	var out []ast.Block
	for _, fn := range d.notes {
		if fn.used || len(fn.lines) == 0 {
			continue
		}
		ins := d.inlines(fn.lines, inlineOpts{noNotes: true})
		if len(ins) == 0 {
			continue
		}
		var label []ast.Inline
		if fn.marker != "" {
			label = []ast.Inline{&ast.Superscript{Inlines: []ast.Inline{&ast.Text{Value: fn.marker}}}, &ast.Text{Value: " "}}
		}
		out = append(out, &ast.Para{Inlines: append(label, ins...)})
	}
	if len(out) > 0 {
		out = append([]ast.Block{&ast.HorizontalRule{}}, out...)
	}
	return out
}
