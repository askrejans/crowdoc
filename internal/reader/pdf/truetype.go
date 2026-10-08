package pdf

import (
	"bytes"
	"encoding/binary"
)

// trueTypeInfo holds what text extraction needs from an embedded TrueType
// program: the glyph → Unicode map (inverted cmap and post names) and, for
// symbolic fonts, the code → glyph map.
type trueTypeInfo struct {
	gidText    map[int]string
	codeGID    map[int]int // (3,0) or (1,0) subtable, keyed by the single-byte code
	unicodeGID map[rune]int
}

func be16(b []byte, i int) int {
	if i < 0 || i+2 > len(b) {
		return 0
	}
	return int(binary.BigEndian.Uint16(b[i:]))
}

func be32(b []byte, i int) int {
	if i < 0 || i+4 > len(b) {
		return 0
	}
	return int(binary.BigEndian.Uint32(b[i:]))
}

// parseTrueType extracts cmap and post information. Malformed tables are
// ignored.
func parseTrueType(b []byte) *trueTypeInfo {
	if len(b) < 12 {
		return nil
	}
	tables := map[string][]byte{}
	n := be16(b, 4)
	for i := 0; i < n && 12+16*i+16 <= len(b); i++ {
		rec := 12 + 16*i
		tag := string(b[rec : rec+4])
		off, length := be32(b, rec+8), be32(b, rec+12)
		if off < 0 || length < 0 || off+length > len(b) || off+length < off {
			continue
		}
		tables[tag] = b[off : off+length]
	}
	info := &trueTypeInfo{gidText: map[int]string{}, codeGID: map[int]int{}, unicodeGID: map[rune]int{}}
	if cm := tables["cmap"]; cm != nil {
		info.readCmap(cm)
	}
	if post := tables["post"]; post != nil {
		info.readPost(post)
	}
	return info
}

func (t *trueTypeInfo) readCmap(cm []byte) {
	n := be16(cm, 2)
	type sub struct {
		pid, eid, off int
	}
	var subs []sub
	for i := 0; i < n && 4+8*i+8 <= len(cm); i++ {
		subs = append(subs, sub{be16(cm, 4+8*i), be16(cm, 6+8*i), be32(cm, 8+8*i)})
	}
	for _, s := range subs {
		unicode := s.pid == 0 || (s.pid == 3 && (s.eid == 1 || s.eid == 10))
		symbol := s.pid == 3 && s.eid == 0
		mac := s.pid == 1 && s.eid == 0
		if !unicode && !symbol && !mac {
			continue
		}
		readCmapSubtable(cm, s.off, func(code, gid int) {
			if gid == 0 {
				return
			}
			switch {
			case unicode:
				if _, ok := t.gidText[gid]; !ok {
					t.gidText[gid] = string(rune(code))
				}
				if _, ok := t.unicodeGID[rune(code)]; !ok {
					t.unicodeGID[rune(code)] = gid
				}
			case symbol:
				if code >= 0xF000 && code <= 0xF0FF {
					code -= 0xF000
				}
				if code < 256 {
					if _, ok := t.codeGID[code]; !ok {
						t.codeGID[code] = gid
					}
				}
			case mac:
				if code < 256 {
					if _, ok := t.codeGID[code]; !ok {
						t.codeGID[code] = gid
					}
				}
			}
		})
	}
}

const maxCmapEntries = 1 << 18

func readCmapSubtable(cm []byte, off int, fn func(code, gid int)) {
	if off < 0 || off+6 > len(cm) {
		return
	}
	switch be16(cm, off) {
	case 0:
		for c := 0; c < 256 && off+6+c < len(cm); c++ {
			fn(c, int(cm[off+6+c]))
		}
	case 4:
		segX2 := be16(cm, off+6)
		ends := off + 14
		starts := ends + segX2 + 2
		deltas := starts + segX2
		ranges := deltas + segX2
		count := 0
		for i := 0; i < segX2/2; i++ {
			end, start := be16(cm, ends+2*i), be16(cm, starts+2*i)
			delta, ro := be16(cm, deltas+2*i), be16(cm, ranges+2*i)
			if start > end || start == 0xFFFF {
				continue
			}
			for c := start; c <= end && count < maxCmapEntries; c++ {
				count++
				var gid int
				if ro == 0 {
					gid = (c + delta) & 0xFFFF
				} else {
					p := ranges + 2*i + ro + 2*(c-start)
					gid = be16(cm, p)
					if gid != 0 {
						gid = (gid + delta) & 0xFFFF
					}
				}
				fn(c, gid)
			}
		}
	case 6:
		first, count := be16(cm, off+6), be16(cm, off+8)
		for i := 0; i < count; i++ {
			fn(first+i, be16(cm, off+10+2*i))
		}
	case 12:
		groups := be32(cm, off+12)
		count := 0
		for i := 0; i < groups && off+16+12*i+12 <= len(cm); i++ {
			g := off + 16 + 12*i
			start, end, gid := be32(cm, g), be32(cm, g+4), be32(cm, g+8)
			for c := start; c <= end && count < maxCmapEntries; c++ {
				fn(c, gid+c-start)
				count++
			}
		}
	}
}

// readPost adds glyph names from a format 2 'post' table for glyphs that
// the cmap did not cover (ligatures, alternates).
func (t *trueTypeInfo) readPost(p []byte) {
	if be32(p, 0) != 0x00020000 || len(p) < 34 {
		return
	}
	n := be16(p, 32)
	if 34+2*n > len(p) {
		return
	}
	idx := make([]int, n)
	maxIdx := 0
	for i := range idx {
		idx[i] = be16(p, 34+2*i)
		maxIdx = max(maxIdx, idx[i])
	}
	var names []string
	pos := 34 + 2*n
	for pos < len(p) && len(names) <= maxIdx-258 {
		l := int(p[pos])
		pos++
		if pos+l > len(p) {
			break
		}
		names = append(names, string(p[pos:pos+l]))
		pos += l
	}
	for gid, ix := range idx {
		if _, ok := t.gidText[gid]; ok || ix < 258 {
			continue
		}
		if k := ix - 258; k < len(names) {
			if s := glyphText(names[k]); s != "" {
				t.gidText[gid] = s
			}
		}
	}
}

// type1Encoding reads the built-in encoding of an embedded Type 1 program
// ("dup 65 /A put" entries). It reports false for StandardEncoding or when
// no encoding is found.
func type1Encoding(prog []byte) (map[int]string, bool) {
	i := bytes.Index(prog, []byte("/Encoding"))
	if i < 0 {
		return nil, false
	}
	end := bytes.Index(prog[i:], []byte("readonly def"))
	if end < 0 {
		end = bytes.Index(prog[i:], []byte("eexec"))
	}
	if end < 0 {
		end = min(len(prog)-i, 64<<10)
	}
	seg := prog[i : i+end]
	if bytes.Contains(seg[:min(len(seg), 40)], []byte("StandardEncoding")) {
		return nil, false
	}
	lx := newLexer(seg)
	out := map[int]string{}
	var prev [3]any
	for {
		tok, ok := lx.next()
		if !ok {
			break
		}
		if tok == keyword("put") {
			if code, ok := prev[1].(int); ok && prev[0] == keyword("dup") {
				if n, ok := prev[2].(name); ok && code >= 0 && code < 256 {
					out[code] = string(n)
				}
			}
		}
		prev[0], prev[1], prev[2] = prev[1], prev[2], tok
	}
	return out, len(out) > 0
}
