package tex2typ

import "strings"

// edge classifies the first or last character of a rendered chunk so the
// renderer can decide whether two neighbours need a separating space.
type edge uint8

const (
	eNone  edge = iota // closers, escapes, operators that never merge
	eWord              // identifier or number characters
	ePunct             // raw ASCII punctuation that may form a Typst shorthand
	eDot               // a raw '.', which would be field access after a name
	eText              // a string literal; adjacent spaces would be visible
	eCode              // an embedded #expression; adjacent spaces would be visible
)

type nkind uint8

const (
	nAtom   nkind = iota // s is Typst code
	nSeq                 // kids in order
	nCall                // s(kids..., named...)
	nAttach              // base with scripts and primes
	nAlign               // alignment point &
	nBreak               // line break
	nMat                 // s is "mat" or "cases"; rows of cells
	nRows                // multi-line environment; kids are the rows
	nEmbed               // s + "$" + kids[0] + "$" + post: code wrapping math
)

// node is the Typst syntax tree built by the parser. It is kept small
// because one is allocated per atom; rarely used fields live in nodeExt.
type node struct {
	kind   nkind
	l, r   edge // nAtom edges
	cls    symClass
	letter bool  // nAtom: a single letter from the source
	top    bool  // nRows: the whole formula, rendered with Typst line breaks
	primes int32 // nAttach
	s      string
	kids   []*node

	base, sub, sup *node // nAttach
	x              *nodeExt
}

type nodeExt struct {
	post  string     // nEmbed suffix
	named []namedArg // nCall, nMat
	rows  [][]*node  // nMat cells
}

func (n *node) ext() *nodeExt {
	if n.x == nil {
		n.x = &nodeExt{}
	}
	return n.x
}

func (n *node) named() []namedArg {
	if n.x == nil {
		return nil
	}
	return n.x.named
}

func (n *node) rows() [][]*node {
	if n.x == nil {
		return nil
	}
	return n.x.rows
}

// with appends named arguments and returns n.
func (n *node) with(args ...namedArg) *node {
	n.ext().named = append(n.ext().named, args...)
	return n
}

func mat(name string, rows [][]*node, args ...namedArg) *node {
	n := &node{kind: nMat, s: name}
	n.ext().rows = rows
	return n.with(args...)
}

func embed(pre string, kid *node, post string) *node {
	n := &node{kind: nEmbed, s: pre, kids: []*node{kid}}
	n.ext().post = post
	return n
}

func code(name, value string) namedArg { return namedArg{name: name, code: value} }

// namedArg is a named call argument holding either code or math content.
type namedArg struct {
	name    string
	code    string
	content *node
}

func atom(s string, l, r edge) *node { return &node{kind: nAtom, s: s, l: l, r: r} }

func word(s string) *node { return &node{kind: nAtom, s: s, l: eWord, r: eWord} }

func seq(kids ...*node) *node { return &node{kind: nSeq, kids: kids} }

func call(fn string, args ...*node) *node { return &node{kind: nCall, s: fn, kids: args} }

func strAtom(text string) *node {
	return &node{kind: nAtom, s: quote(text), l: eText, r: eText}
}

func codeAtom(code string) *node { return &node{kind: nAtom, s: code, l: eCode, r: eCode} }

// quote renders text as a Typst string literal.
func quote(text string) string {
	var b strings.Builder
	b.Grow(len(text) + 2)
	b.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n', '\r', '\t':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// isEmpty reports whether n renders to nothing.
func (n *node) isEmpty() bool {
	if n == nil {
		return true
	}
	switch n.kind {
	case nSeq:
		for _, k := range n.kids {
			if !k.isEmpty() {
				return false
			}
		}
		return true
	case nAtom:
		return n.s == ""
	}
	return false
}

// unwrap returns the only non-empty item of nested sequences, or n itself.
func (n *node) unwrap() *node {
	for n != nil && n.kind == nSeq {
		var only *node
		for _, k := range n.kids {
			if k.isEmpty() {
				continue
			}
			if only != nil {
				return n
			}
			only = k
		}
		if only == nil {
			return n
		}
		n = only
	}
	return n
}

// renderer serialises the tree, inserting only the spaces Typst needs to
// keep neighbouring tokens apart: in math, white space is invisible except
// next to text, so spaces are never written beside strings or #code.
type renderer struct {
	b    strings.Builder
	last edge
	warn func(string)
}

// ctx carries how the current position is parsed by Typst.
type ctx struct {
	args bool // inside function-call arguments: ',' ';' ':' are syntax
	cell bool // in the run of a matrix cell or case, where Typst ignores line breaks
}

// ownRun lists functions whose arguments are laid out as separate runs, in
// which line breaks work even inside a matrix cell.
var ownRun = map[string]bool{"frac": true, "binom": true, "sqrt": true, "root": true, "attach": true}

func needSpace(prev, next edge) bool {
	switch {
	case prev == eText || next == eText || prev == eCode || next == eCode:
		return false
	case prev == eWord:
		return next == eWord || next == eDot
	case prev == ePunct || prev == eDot:
		return next == ePunct || next == eDot
	}
	return false
}

func (w *renderer) raw(s string, last edge) {
	w.b.WriteString(s)
	w.last = last
}

func (w *renderer) atom(s string, l, r edge, c ctx) {
	if s == "" {
		return
	}
	if c.args && (s[0] == ',' || s[0] == ';' || s[0] == ':') {
		s = "\\" + s
		l, r = eNone, eNone
	}
	if needSpace(w.last, l) {
		w.b.WriteByte(' ')
	}
	// After an embedded expression, '.' would continue it as a field access
	// and ';' would be swallowed as its terminator.
	if w.last == eCode && (s[0] == '.' || s[0] == ';') {
		w.b.WriteByte('\\')
	}
	w.b.WriteString(s)
	w.last = r
}

func (w *renderer) render(n *node, c ctx) {
	if n == nil {
		return
	}
	switch n.kind {
	case nAtom:
		w.atom(n.s, n.l, n.r, c)
	case nSeq:
		for _, k := range n.kids {
			w.render(k, c)
		}
	case nCall:
		w.renderCall(n, c)
	case nAttach:
		w.renderAttach(n, c)
	case nAlign:
		w.raw("&", eNone)
	case nBreak:
		if c.cell {
			if w.warn != nil {
				w.warn("line break inside a matrix cell or case dropped")
			}
			return
		}
		w.b.WriteString(" \\\n")
		w.last = eNone
	case nMat:
		w.renderMat(n)
	case nRows:
		w.renderRows(n, c)
	case nEmbed:
		w.atom(n.s, eCode, eNone, c)
		w.b.WriteByte('$')
		w.last = eNone
		w.render(n.kids[0], ctx{cell: c.cell})
		w.b.WriteByte('$')
		w.b.WriteString(n.x.post)
		w.last = eCode
	}
}

func (w *renderer) renderCall(n *node, c ctx) {
	inner := ctx{args: true, cell: c.cell && !ownRun[n.s]}
	w.atom(n.s, eWord, eNone, ctx{})
	w.raw("(", eNone)
	first := true
	sep := func() {
		if !first {
			w.b.WriteString(", ")
		}
		first = false
		w.last = eNone
	}
	for _, k := range n.kids {
		sep()
		w.arg(k, inner)
	}
	for _, a := range n.named() {
		sep()
		w.b.WriteString(a.name)
		w.b.WriteString(": ")
		if a.content != nil {
			w.last = eNone
			w.arg(a.content, inner)
		} else {
			w.b.WriteString(a.code)
		}
	}
	w.raw(")", eNone)
}

// arg writes a content argument; an empty one would be dropped by Typst.
func (w *renderer) arg(n *node, c ctx) {
	start := w.b.Len()
	w.render(n, c)
	if w.b.Len() == start {
		w.raw(`""`, eText)
	}
}

func (w *renderer) renderMat(n *node) {
	w.atom(n.s, eWord, eNone, ctx{})
	w.raw("(", eNone)
	first := true
	for _, a := range n.named() {
		if !first {
			w.b.WriteString(", ")
		}
		first = false
		w.b.WriteString(a.name)
		w.b.WriteString(": ")
		w.b.WriteString(a.code)
	}
	for i, row := range n.rows() {
		if i > 0 {
			if n.s == "mat" {
				w.b.WriteString("; ")
			} else {
				w.b.WriteString(", ")
			}
		} else if !first {
			w.b.WriteString(", ")
		}
		for j, cell := range row {
			if j > 0 {
				w.b.WriteString(", ")
			}
			w.last = eNone
			// An empty cell must still be written: a bare trailing comma
			// would not count as one.
			w.arg(cell, ctx{args: true, cell: true})
		}
	}
	w.raw(")", eNone)
}

func (w *renderer) renderRows(n *node, c ctx) {
	if n.top {
		for i, row := range n.kids {
			if i > 0 {
				w.b.WriteString(" \\\n")
				w.last = eNone
			}
			w.render(row, c)
		}
		return
	}
	// Nested: a borderless one-column matrix whose cells keep their
	// alignment points; big items are set in display style as LaTeX does.
	w.atom("mat", eWord, eNone, ctx{})
	w.b.WriteString("(delim: #none")
	for i, row := range n.kids {
		if i == 0 {
			w.b.WriteString(", ")
		} else {
			w.b.WriteString("; ")
		}
		w.last = eNone
		w.arg(displayItems(row), ctx{args: true, cell: true})
	}
	w.raw(")", eNone)
}

// displayItems wraps the structured items of a row in display() so they keep
// display style inside a matrix cell without hiding alignment points.
func displayItems(row *node) *node {
	var out []*node
	var walk func(n *node)
	walk = func(n *node) {
		switch n.kind {
		case nSeq:
			// Cells are nested lists; the alignment points must stay at the
			// top level of the row.
			for _, k := range n.kids {
				walk(k)
			}
		case nCall, nAttach, nMat:
			out = append(out, call("display", n))
		default:
			out = append(out, n)
		}
	}
	walk(row)
	return seq(out...)
}

// scriptSimple reports whether a script can follow ^ or _ without
// parentheses: Typst attaches exactly one identifier, number or string.
func scriptSimple(n *node) bool {
	return n.kind == nAtom && (n.l == eWord && n.r == eWord || n.l == eText) && !strings.ContainsAny(n.s, " (")
}

func (w *renderer) renderAttach(n *node, c ctx) {
	base := n.base.unwrap()
	switch {
	case base == nil || base.isEmpty() || base.kind == nAlign || base.kind == nBreak:
		w.atom(`""`, eText, eText, c)
	case base.kind == nSeq || base.kind == nAttach || base.kind == nRows:
		// A braced group is a single ordinary atom in TeX.
		w.render(call("class", strAtom("normal"), base), c)
	default:
		w.render(base, c)
	}
	for i := int32(0); i < n.primes; i++ {
		w.raw("'", ePunct)
	}
	w.script("_", n.sub)
	w.script("^", n.sup)
}

func (w *renderer) script(mark string, s *node) {
	if s == nil {
		return
	}
	w.b.WriteString(mark)
	w.last = eNone
	s = s.unwrap()
	if !s.isEmpty() && scriptSimple(s) {
		w.render(s, ctx{})
		return
	}
	w.b.WriteByte('(')
	w.render(s, ctx{})
	w.raw(")", eNone)
}
