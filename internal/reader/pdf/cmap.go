package pdf

import (
	"sort"
	"strings"
	"unicode/utf16"
)

// cmap is a parsed CMap: either a ToUnicode map (code → text) or a
// composite-font encoding (code → CID). Only the parts needed for text
// extraction are kept.
type cmap struct {
	space     []codeSpace
	bf        map[uint32]string // key: code | nbytes<<29 is avoided; codes are per length below
	bfLen     map[uint32]uint8  // byte length of each bf code
	bfRanges  []bfRange
	cid       map[uint32]int
	cidRanges []cidRange
	vertical  bool
	usecmap   string
}

type codeSpace struct {
	lo, hi []byte
}

type bfRange struct {
	lo, hi uint32
	n      uint8
	base   []rune   // destination text of lo (incremented for later codes)
	list   []string // explicit destinations (array form)
}

type cidRange struct {
	lo, hi uint32
	n      uint8
	cid    int
}

// maxCMapEntries bounds hostile CMaps.
const maxCMapEntries = 1 << 20

// parseCMap parses CMap program text. It never fails; unknown content is
// ignored.
func parseCMap(data []byte) *cmap {
	c := &cmap{bf: map[uint32]string{}, bfLen: map[uint32]uint8{}, cid: map[uint32]int{}}
	lx := newLexer(data)
	var prevName name
	entries := 0
	for entries < maxCMapEntries {
		tok, ok := lx.next()
		if !ok {
			break
		}
		switch t := tok.(type) {
		case name:
			if t == "WMode" {
				if v, ok := lx.next(); ok {
					if n, _ := v.(int); n == 1 {
						c.vertical = true
					}
				}
			}
			prevName = t
			continue
		case keyword:
			switch t {
			case "usecmap":
				c.usecmap = string(prevName)
			case "begincodespacerange":
				for {
					lo, ok1 := lx.next()
					if lo == keyword("endcodespacerange") || !ok1 {
						break
					}
					hi, _ := lx.next()
					l, a := lo.(pdfString)
					h, b := hi.(pdfString)
					if a && b && len(l) == len(h) && len(l) > 0 && len(l) <= 4 {
						c.space = append(c.space, codeSpace{[]byte(l), []byte(h)})
					}
					entries++
				}
			case "beginbfchar":
				for entries < maxCMapEntries {
					src, ok1 := lx.next()
					if src == keyword("endbfchar") || !ok1 {
						break
					}
					dst, _ := lx.object(0)
					s, ok := src.(pdfString)
					if !ok || len(s) == 0 || len(s) > 4 {
						continue
					}
					code := codeOf([]byte(s))
					c.bf[code] = destText(dst)
					c.bfLen[code] = uint8(len(s))
					entries++
				}
			case "beginbfrange":
				for entries < maxCMapEntries {
					lo, ok1 := lx.next()
					if lo == keyword("endbfrange") || !ok1 {
						break
					}
					hi, _ := lx.next()
					dst, _ := lx.object(0)
					l, a := lo.(pdfString)
					h, b := hi.(pdfString)
					if !a || !b || len(l) == 0 || len(l) > 4 {
						continue
					}
					r := bfRange{lo: codeOf([]byte(l)), hi: codeOf([]byte(h)), n: uint8(len(l))}
					if r.hi < r.lo || r.hi-r.lo > maxCMapEntries {
						continue
					}
					switch d := dst.(type) {
					case pdfString:
						r.base = []rune(utf16BE([]byte(d)))
						if len(d) == 1 {
							r.base = []rune{rune(d[0])}
						}
					case array:
						for _, e := range d {
							r.list = append(r.list, destText(e))
						}
					default:
						continue
					}
					c.bfRanges = append(c.bfRanges, r)
					entries++
				}
			case "begincidchar":
				for entries < maxCMapEntries {
					src, ok1 := lx.next()
					if src == keyword("endcidchar") || !ok1 {
						break
					}
					dst, _ := lx.next()
					s, ok := src.(pdfString)
					cid, ok2 := dst.(int)
					if ok && ok2 && len(s) > 0 && len(s) <= 4 {
						c.cid[codeOf([]byte(s))] = cid
					}
					entries++
				}
			case "begincidrange":
				for entries < maxCMapEntries {
					lo, ok1 := lx.next()
					if lo == keyword("endcidrange") || !ok1 {
						break
					}
					hi, _ := lx.next()
					dst, _ := lx.next()
					l, a := lo.(pdfString)
					h, b := hi.(pdfString)
					cid, ok2 := dst.(int)
					if a && b && ok2 && len(l) > 0 && len(l) <= 4 {
						c.cidRanges = append(c.cidRanges, cidRange{codeOf([]byte(l)), codeOf([]byte(h)), uint8(len(l)), cid})
					}
					entries++
				}
			}
		}
		prevName = ""
	}
	sort.Slice(c.bfRanges, func(i, j int) bool { return c.bfRanges[i].lo < c.bfRanges[j].lo })
	return c
}

func codeOf(b []byte) uint32 {
	var v uint32
	for _, x := range b {
		v = v<<8 | uint32(x)
	}
	return v
}

func destText(v any) string {
	switch d := v.(type) {
	case pdfString:
		if len(d) == 1 {
			return string(rune(d[0]))
		}
		return utf16BE([]byte(d))
	case name:
		return glyphText(string(d))
	}
	return ""
}

// lookup returns the text mapped to code (of n bytes; n = 0 matches any
// length).
func (c *cmap) lookup(code uint32, n int) (string, bool) {
	if c == nil {
		return "", false
	}
	if s, ok := c.bf[code]; ok && (n == 0 || int(c.bfLen[code]) == n) {
		return s, true
	}
	i := sort.Search(len(c.bfRanges), func(i int) bool { return c.bfRanges[i].hi >= code })
	for ; i < len(c.bfRanges) && c.bfRanges[i].lo <= code; i++ {
		r := c.bfRanges[i]
		if code > r.hi || (n != 0 && int(r.n) != n) {
			continue
		}
		k := code - r.lo
		if r.list != nil {
			if int(k) < len(r.list) {
				return r.list[k], true
			}
			continue
		}
		if len(r.base) == 0 {
			return "", true
		}
		out := append([]rune(nil), r.base...)
		out[len(out)-1] += rune(k)
		return string(out), true
	}
	if n != 0 {
		// Some writers use a different code length in ToUnicode than in
		// the font encoding.
		if s, ok := c.bf[code]; ok {
			return s, true
		}
	}
	return "", false
}

// cidOf maps a code to a CID through cidchar/cidrange entries.
func (c *cmap) cidOf(code uint32, n int) (int, bool) {
	if cid, ok := c.cid[code]; ok {
		return cid, true
	}
	for _, r := range c.cidRanges {
		if code >= r.lo && code <= r.hi && (n == 0 || int(r.n) == n) {
			return r.cid + int(code-r.lo), true
		}
	}
	return 0, false
}

// codeLen returns the byte length of the code starting at b according to
// the codespace ranges (def when none match).
func (c *cmap) codeLen(b []byte, def int) int {
	if c == nil || len(c.space) == 0 {
		return min(def, len(b))
	}
	for n := 1; n <= 4 && n <= len(b); n++ {
		for _, s := range c.space {
			if len(s.lo) != n {
				continue
			}
			match := true
			for i := 0; i < n; i++ {
				if b[i] < s.lo[i] || b[i] > s.hi[i] {
					match = false
					break
				}
			}
			if match {
				return n
			}
		}
	}
	// No range matches: consume the shortest code length in use.
	short := 4
	for _, s := range c.space {
		short = min(short, len(s.lo))
	}
	return min(short, len(b))
}

// unicodeCMap reports whether a predefined CMap name encodes Unicode
// directly and with which code unit.
func unicodeCMap(n string) (utf16Codes bool, ok bool) {
	switch {
	case strings.Contains(n, "UCS2"), strings.Contains(n, "UTF16"):
		return true, true
	}
	return false, false
}

// decodeUTF16Code decodes one code of a Unicode-based CMap.
func decodeUTF16Code(b []byte) (string, int) {
	if len(b) < 2 {
		return string(rune(b[0])), 1
	}
	u := uint16(b[0])<<8 | uint16(b[1])
	if utf16.IsSurrogate(rune(u)) && len(b) >= 4 {
		u2 := uint16(b[2])<<8 | uint16(b[3])
		return string(utf16.DecodeRune(rune(u), rune(u2))), 4
	}
	return string(rune(u)), 2
}
