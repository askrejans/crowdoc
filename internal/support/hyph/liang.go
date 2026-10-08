package hyph

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// lowSymLimit bounds the dense rune→symbol table. It covers Latin, Latin
// Extended-A/B, Greek and Cyrillic, i.e. every alphabet embedded here, so the
// map fallback is only consulted for exotic runes.
const lowSymLimit = 0x500

// maxWordLen caps the number of letters considered a word. Anything longer is
// not a natural-language word (TeX itself stops at 63 letters).
const maxWordLen = 128

// patternSet is a compiled Liang pattern trie plus the exception list of one
// language. It is immutable after construction and safe for concurrent use.
type patternSet struct {
	left, right int

	lowSym  []uint16 // rune → symbol (0 = not in the pattern alphabet)
	highSym map[rune]uint16
	dot     uint16 // symbol of the word boundary marker '.'

	// Trie in compressed form. Node 0 is the root; children of the root are
	// looked up densely in rootNext, all other edges are sorted per node.
	rootNext  []int32
	edgeStart []int32 // edges of node n: [edgeStart[n], edgeStart[n+1])
	edgeSym   []uint16
	edgeNext  []int32
	valStart  []int32 // values of node n: vals[valStart[n]:valStart[n+1]]
	valShift  []uint8 // offset of the first stored value within the pattern
	vals      []uint8

	exceptions map[string][]uint8 // lowercased word → inter-letter values (odd = break)
}

// sym returns the trie symbol of r, or 0 when r never occurs in a pattern.
func (p *patternSet) sym(r rune) uint16 {
	if r >= 0 && r < lowSymLimit {
		return p.lowSym[r]
	}
	return p.highSym[r]
}

// compile parses TeX-style patterns (one or more per line, digits between
// letters, '.' marking word boundaries) and exceptions (words with '-' at
// permitted breaks). It also accepts the Hyphen/libhnj dictionary layout:
// the charset line, keyword lines and '%' comments are skipped, as are
// non-standard "/" rules, which cannot be expressed in Liang's scheme.
func compile(patterns, exceptions string, left, right int) *patternSet {
	p := &patternSet{left: left, right: right, lowSym: make([]uint16, lowSymLimit)}
	nextSym := uint16(1)
	symOf := func(r rune) uint16 {
		if s := p.sym(r); s != 0 {
			return s
		}
		s := nextSym
		nextSym++
		if r >= 0 && r < lowSymLimit {
			p.lowSym[r] = s
		} else {
			if p.highSym == nil {
				p.highSym = make(map[rune]uint16)
			}
			p.highSym[r] = s
		}
		return s
	}
	p.dot = symOf('.')

	// Build phase: one transition map keyed by parent<<16|symbol, plus the
	// value vectors of all patterns in a single flat buffer.
	fields := patternFields(patterns)
	b := trieBuilder{trans: make(map[uint64]int32, 2*len(fields)), nodes: 1}
	var letters []uint16
	var values []uint8
	for _, field := range fields {
		letters, values = letters[:0], values[:0]
		pending := uint8(0)
		for _, r := range field {
			if r >= '0' && r <= '9' {
				pending = uint8(r - '0')
				continue
			}
			values = append(values, pending)
			pending = 0
			letters = append(letters, symOf(foldPatternRune(r)))
		}
		values = append(values, pending)
		if len(letters) > 0 {
			b.add(letters, values)
		}
	}
	p.freeze(&b, int(nextSym))

	for _, field := range patternFields(exceptions) {
		var word []rune
		vals := []uint8{0}
		for _, r := range field {
			if r == '-' {
				vals[len(vals)-1] = 1
				continue
			}
			word = append(word, foldPatternRune(r))
			vals = append(vals, 0)
		}
		if len(word) == 0 {
			continue
		}
		if p.exceptions == nil {
			p.exceptions = make(map[string][]uint8)
		}
		p.exceptions[string(word)] = vals
	}
	return p
}

type trieBuilder struct {
	trans map[uint64]int32 // parent<<16 | symbol → child
	nodes int32
	ends  []patternEnd
	flat  []uint8
}

type patternEnd struct {
	node int32
	off  int32
	n    int32
}

func (b *trieBuilder) add(letters []uint16, values []uint8) {
	n := int32(0)
	for _, s := range letters {
		k := uint64(n)<<16 | uint64(s)
		child, ok := b.trans[k]
		if !ok {
			child = b.nodes
			b.nodes++
			b.trans[k] = child
		}
		n = child
	}
	b.ends = append(b.ends, patternEnd{node: n, off: int32(len(b.flat)), n: int32(len(values))})
	b.flat = append(b.flat, values...)
}

// patternFields splits a pattern file into fields, dropping comments,
// keyword lines and rules the classic algorithm cannot represent.
func patternFields(src string) []string {
	var out []string
	for line := range strings.SplitSeq(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '%' || line[0] == '#' {
			continue
		}
		if c := line[0]; c >= 'A' && c <= 'Z' {
			continue // charset line or keyword (LEFTHYPHENMIN, NEXTLEVEL, …)
		}
		for f := range strings.FieldsSeq(line) {
			if strings.ContainsAny(f, "/=%") {
				continue
			}
			out = append(out, f)
		}
	}
	return out
}

func foldPatternRune(r rune) rune {
	if r == '.' {
		return r
	}
	return unicode.ToLower(r)
}

// freeze converts the build state into flat arrays: edges sorted by parent
// and symbol, and per-node value vectors with zero padding trimmed.
func (p *patternSet) freeze(b *trieBuilder, nsym int) {
	type edge struct {
		key   uint64
		child int32
	}
	edges := make([]edge, 0, len(b.trans))
	for k, c := range b.trans {
		edges = append(edges, edge{k, c})
	}
	slices.SortFunc(edges, func(x, y edge) int { return cmp.Compare(x.key, y.key) })

	n := int(b.nodes)
	p.rootNext = make([]int32, nsym)
	p.edgeStart = make([]int32, n+1)
	p.edgeSym = make([]uint16, len(edges))
	p.edgeNext = make([]int32, len(edges))
	e := 0
	for node := range n {
		p.edgeStart[node] = int32(e)
		for ; e < len(edges) && int(edges[e].key>>16) == node; e++ {
			sym := uint16(edges[e].key)
			p.edgeSym[e], p.edgeNext[e] = sym, edges[e].child
			if node == 0 {
				p.rootNext[sym] = edges[e].child
			}
		}
	}
	p.edgeStart[n] = int32(e)

	// A repeated pattern overrides the earlier one, as the last definition
	// would in a TeX format.
	end := make([]int32, n) // index+1 into b.ends
	for i, pe := range b.ends {
		end[pe.node] = int32(i + 1)
	}
	p.valStart = make([]int32, n+1)
	p.valShift = make([]uint8, n)
	for node := range n {
		p.valStart[node] = int32(len(p.vals))
		if end[node] == 0 {
			continue
		}
		pe := b.ends[end[node]-1]
		v := b.flat[pe.off : pe.off+pe.n]
		for len(v) > 0 && v[len(v)-1] == 0 {
			v = v[:len(v)-1]
		}
		shift := 0
		for shift < len(v) && v[shift] == 0 {
			shift++
		}
		p.valShift[node] = uint8(shift)
		p.vals = append(p.vals, v[shift:]...)
	}
	p.valStart[n] = int32(len(p.vals))
}

func (p *patternSet) child(n int32, s uint16) int32 {
	if n == 0 {
		return p.rootNext[s]
	}
	end := p.edgeStart[n+1]
	for e := p.edgeStart[n]; e < end; e++ {
		switch es := p.edgeSym[e]; {
		case es == s:
			return p.edgeNext[e]
		case es > s:
			return 0
		}
	}
	return 0
}

// scratch holds reusable buffers so that hyphenating a stream of words does
// not allocate per word.
type scratch struct {
	letters []rune   // NFC, lowercased letters of the current word
	breakOK []bool   // letter k starts a grapheme cluster (a break before it is allowed)
	offset  []int    // byte offset of letter k's cluster in the source word
	runeOff []int    // rune offset of letter k's cluster in the source word
	syms    []uint16 // '.' + letters + '.'
	vals    []uint8
	key     []byte
	breaks  []int // letter indices before which a hyphen may go
}

// hyphenate computes break positions for s.letters (already folded) and
// stores the letter indices in s.breaks.
func (p *patternSet) hyphenate(s *scratch) {
	s.breaks = s.breaks[:0]
	n := len(s.letters)
	if n < p.left+p.right || n > maxWordLen {
		return
	}
	if p.exceptions != nil {
		s.key = s.key[:0]
		for _, r := range s.letters {
			s.key = utf8.AppendRune(s.key, r)
		}
		if v, ok := p.exceptions[string(s.key)]; ok && len(v) == n+1 {
			p.collect(s, v)
			return
		}
	}

	s.syms = append(s.syms[:0], p.dot)
	for _, r := range s.letters {
		s.syms = append(s.syms, p.sym(r))
	}
	s.syms = append(s.syms, p.dot)
	s.vals = s.vals[:0]
	for range len(s.syms) + 1 {
		s.vals = append(s.vals, 0)
	}

	syms, vals := s.syms, s.vals
	for i, first := range syms {
		if first == 0 {
			continue
		}
		node := p.rootNext[first]
		for j := i + 1; node != 0; j++ {
			if vs, ve := p.valStart[node], p.valStart[node+1]; vs != ve {
				at := i + int(p.valShift[node])
				for _, v := range p.vals[vs:ve] {
					if v > vals[at] {
						vals[at] = v
					}
					at++
				}
			}
			if j >= len(syms) || syms[j] == 0 {
				break
			}
			node = p.child(node, syms[j])
		}
	}
	// vals[k] is the priority between syms[k-1] and syms[k]; with the leading
	// '.', the gap before letter m is vals[m+1].
	p.collect(s, vals[1:n+2])
}

// collect turns inter-letter values (index m = gap before letter m) into
// break positions, honouring the hyphenmins and cluster boundaries.
func (p *patternSet) collect(s *scratch, v []uint8) {
	n := len(s.letters)
	for m := p.left; m <= n-p.right; m++ {
		if v[m]%2 == 1 && s.breakOK[m] {
			s.breaks = append(s.breaks, m)
		}
	}
}
