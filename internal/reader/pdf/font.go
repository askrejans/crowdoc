package pdf

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// pdfFont is a font resource prepared for text extraction.
type pdfFont struct {
	name      string // base font name without the subset tag
	composite bool
	type3     bool
	fm        matrix // glyph space → text space (Type 3)
	vertical  bool

	// Composite fonts.
	enc      *cmap // code → CID; nil means Identity (two-byte codes)
	encUTF16 bool  // predefined Unicode CMap: codes are UTF-16
	cidW     []cidWidth
	dw       float64
	cidGID   []uint16 // CIDToGIDMap stream, nil = identity
	tt       *trueTypeInfo

	// Simple fonts.
	text     [256]string
	widths   [256]float64
	hasWidth [256]bool
	std      *stdMetrics

	toUni    *cmap
	missingW float64
	avgW     float64

	bold, italic, mono bool
	unmapped           int

	// scriptCodes are codes of superscript/subscript glyph variants of
	// digits (OpenType "sups"/"subs" glyphs): their text is a plain digit
	// but they are printed small and raised or lowered.
	scriptCodes map[uint32]bool
}

type cidWidth struct {
	lo, hi int
	w      float64
}

// glyph is one decoded character code.
type glyph struct {
	text   string
	w      float64 // advance in text space per unit font size
	space  bool    // single-byte code 32 (word spacing applies)
	script bool    // a superscript or subscript variant glyph
}

func stripSubset(n string) string {
	if len(n) > 7 && n[6] == '+' {
		upper := true
		for i := 0; i < 6; i++ {
			if n[i] < 'A' || n[i] > 'Z' {
				upper = false
				break
			}
		}
		if upper {
			return n[7:]
		}
	}
	return n
}

// loadFont builds a pdfFont from a font dictionary.
func (f *file) loadFont(fd dict) *pdfFont {
	ft := &pdfFont{fm: matrix{0.001, 0, 0, 0.001, 0, 0}, dw: 1000, missingW: 0}
	base, _ := f.resolve(fd["BaseFont"]).(name)
	ft.name = stripSubset(string(base))
	subtype, _ := f.resolve(fd["Subtype"]).(name)
	if tu, ok := f.resolve(fd["ToUnicode"]).(*stream); ok {
		if data, _, _, err := f.decodeStream(tu, false); err == nil {
			ft.toUni = parseCMap(data)
		}
	}
	desc, _ := f.resolve(fd["FontDescriptor"]).(dict)
	switch subtype {
	case "Type0":
		ft.composite = true
		f.loadComposite(ft, fd)
		if desc == nil {
			if desc0, ok := f.descendant(fd); ok {
				desc, _ = f.resolve(desc0["FontDescriptor"]).(dict)
			}
		}
	default:
		if subtype == "Type3" {
			ft.type3 = true
			if m, ok := matrixFrom(f.resolveArray(fd["FontMatrix"])); ok && m[0] != 0 {
				ft.fm = m
			}
		}
		f.loadSimple(ft, fd, desc)
	}
	ft.style(desc, f)
	ft.findScriptDigits()
	return ft
}

// findScriptDigits marks codes that render a digit with a clearly smaller
// advance than another code of the same font rendering the same digit.
// Such glyphs are the superscript or subscript variants that typesetting
// engines select through OpenType features: they share the Unicode value
// and the font size of the regular digit, so their size gives them away.
func (ft *pdfFont) findScriptDigits() {
	type variant struct {
		code uint32
		w    float64
	}
	byDigit := map[string][]variant{}
	add := func(code uint32, text string, w float64) {
		if len(text) == 1 && text[0] >= '0' && text[0] <= '9' && w > 0 && len(byDigit[text]) < 8 {
			byDigit[text] = append(byDigit[text], variant{code, w})
		}
	}
	switch {
	case ft.composite && ft.toUni != nil:
		if len(ft.toUni.bf) > 1<<14 {
			return
		}
		for code, text := range ft.toUni.bf {
			cid := int(code)
			if ft.enc != nil {
				if c, ok := ft.enc.cidOf(code, int(ft.toUni.bfLen[code])); ok {
					cid = c
				}
			}
			add(code, text, ft.cidWidth(cid))
		}
	case !ft.composite:
		for c := 0; c < 256; c++ {
			text := ft.text[c]
			if ft.toUni != nil {
				if t, ok := ft.toUni.lookup(uint32(c), 1); ok && usefulText(t) {
					text = t
				}
			}
			if ft.hasWidth[c] {
				add(uint32(c), text, ft.widths[c])
			}
		}
	}
	for _, vs := range byDigit {
		if len(vs) < 2 {
			continue
		}
		widest := 0.0
		for _, v := range vs {
			widest = max(widest, v.w)
		}
		for _, v := range vs {
			if v.w < 0.85*widest {
				if ft.scriptCodes == nil {
					ft.scriptCodes = map[uint32]bool{}
				}
				ft.scriptCodes[v.code] = true
			}
		}
	}
}

func (f *file) descendant(fd dict) (dict, bool) {
	a, _ := f.resolve(fd["DescendantFonts"]).(array)
	if len(a) == 0 {
		return nil, false
	}
	d, ok := f.resolve(a[0]).(dict)
	return d, ok
}

func (f *file) loadComposite(ft *pdfFont, fd dict) {
	switch e := f.resolve(fd["Encoding"]).(type) {
	case name:
		if e == "Identity-V" {
			ft.vertical = true
		}
		if utf16Codes, ok := unicodeCMap(string(e)); ok && utf16Codes {
			ft.encUTF16 = true
		}
		if strings.HasSuffix(string(e), "-V") {
			ft.vertical = true
		}
	case *stream:
		if data, _, _, err := f.decodeStream(e, false); err == nil {
			ft.enc = parseCMap(data)
			ft.vertical = ft.enc.vertical
			if _, ok := unicodeCMap(ft.enc.usecmap); ok && len(ft.enc.cid) == 0 && len(ft.enc.cidRanges) == 0 {
				ft.encUTF16 = true
			}
		}
	}
	d, ok := f.descendant(fd)
	if !ok {
		return
	}
	if dw, ok := num(f.resolve(d["DW"])); ok && dw > 0 {
		ft.dw = dw
	}
	if w, ok := f.resolve(d["W"]).(array); ok {
		ft.cidW = f.parseCIDWidths(w)
		sort.SliceStable(ft.cidW, func(i, j int) bool { return ft.cidW[i].lo < ft.cidW[j].lo })
	}
	if g, ok := f.resolve(d["CIDToGIDMap"]).(*stream); ok {
		if data, _, _, err := f.decodeStream(g, false); err == nil && len(data) < 1<<21 {
			ft.cidGID = make([]uint16, len(data)/2)
			for i := range ft.cidGID {
				ft.cidGID[i] = uint16(data[2*i])<<8 | uint16(data[2*i+1])
			}
		}
	}
	if ft.toUni == nil && !ft.encUTF16 {
		if desc, ok := f.resolve(d["FontDescriptor"]).(dict); ok {
			if ff, ok := f.resolve(desc["FontFile2"]).(*stream); ok {
				if data, _, _, err := f.decodeStream(ff, false); err == nil {
					ft.tt = parseTrueType(data)
				}
			}
		}
	}
}

func (f *file) parseCIDWidths(w array) []cidWidth {
	var out []cidWidth
	for i := 0; i < len(w) && len(out) < 1<<17; {
		first, ok := integer(f.resolve(w[i]))
		if !ok || i+1 >= len(w) {
			break
		}
		switch next := f.resolve(w[i+1]).(type) {
		case array:
			for k, v := range next {
				if x, ok := num(f.resolve(v)); ok {
					out = append(out, cidWidth{first + k, first + k, x})
				}
			}
			i += 2
		default:
			last, ok1 := integer(next)
			if i+2 >= len(w) {
				return out
			}
			x, ok2 := num(f.resolve(w[i+2]))
			if ok1 && ok2 && last >= first {
				out = append(out, cidWidth{first, last, x})
			}
			i += 3
		}
	}
	return out
}

func (f *file) loadSimple(ft *pdfFont, fd, desc dict) {
	first, _ := integer(f.resolve(fd["FirstChar"]))
	if ws, ok := f.resolve(fd["Widths"]).(array); ok {
		sum, n := 0.0, 0
		for i, v := range ws {
			c := first + i
			if c < 0 || c > 255 {
				continue
			}
			if x, ok := num(f.resolve(v)); ok {
				ft.widths[c] = x
				ft.hasWidth[c] = true
				if x > 0 {
					sum += x
					n++
				}
			}
		}
		if n > 0 {
			ft.avgW = sum / float64(n)
		}
	}
	if desc != nil {
		if mw, ok := num(f.resolve(desc["MissingWidth"])); ok && mw > 0 {
			ft.missingW = mw
		}
	}
	ft.std = standardMetrics(ft.name)

	// Encoding: base table, then the font program's own encoding, then
	// /Differences.
	var enc [256]string
	baseSet := false
	var diffs array
	symbolic := false
	if desc != nil {
		if fl, ok := integer(f.resolve(desc["Flags"])); ok && fl&4 != 0 && fl&32 == 0 {
			symbolic = true
		}
	}
	switch e := f.resolve(fd["Encoding"]).(type) {
	case name:
		enc, baseSet = baseEncoding(e)
	case dict:
		if bn, ok := f.resolve(e["BaseEncoding"]).(name); ok {
			enc, baseSet = baseEncoding(bn)
		}
		diffs, _ = f.resolve(e["Differences"]).(array)
	}
	lower := strings.ToLower(ft.name)
	if !baseSet {
		switch {
		case strings.Contains(lower, "symbol"):
			for c := 0; c < 256; c++ {
				if r, ok := symbolMap[byte(c)]; ok {
					enc[c] = string(r)
				}
			}
			baseSet = true
		case strings.Contains(lower, "dingbat"):
			for c := 0; c < 256; c++ {
				if r, ok := dingbats[byte(c)]; ok {
					enc[c] = string(r)
				} else if c > 32 {
					enc[c] = "•"
				}
			}
			baseSet = true
		}
	}
	if !baseSet && desc != nil {
		if prog, ok := f.resolve(desc["FontFile"]).(*stream); ok {
			if data, _, _, err := f.decodeStream(prog, false); err == nil {
				if m, ok := type1Encoding(data[:min(len(data), 1<<20)]); ok {
					for c, g := range m {
						enc[c] = glyphText(g)
					}
					baseSet = true
				}
			}
		}
		if !baseSet {
			if prog, ok := f.resolve(desc["FontFile2"]).(*stream); ok && symbolic && ft.toUni == nil {
				if data, _, _, err := f.decodeStream(prog, false); err == nil {
					ft.tt = parseTrueType(data)
				}
			}
		}
	}
	if !baseSet {
		if ft.type3 || symbolic && ft.tt == nil {
			enc, _ = baseEncoding("StandardEncoding")
			// Symbolic fonts without any mapping usually still use
			// Latin codes for Latin text.
			win, _ := baseEncoding("WinAnsiEncoding")
			for c := range enc {
				if enc[c] == "" {
					enc[c] = win[c]
				}
			}
		} else {
			enc, _ = baseEncoding("StandardEncoding")
			if !strings.Contains(lower, "times") && !strings.Contains(lower, "helvetica") && !strings.Contains(lower, "courier") {
				// Non-standard fonts without an encoding are almost always
				// Windows Latin in practice.
				win, _ := baseEncoding("WinAnsiEncoding")
				for c := 0x80; c < 256; c++ {
					if enc[c] == "" {
						enc[c] = win[c]
					}
				}
			}
		}
	}
	code := 0
	for _, v := range diffs {
		switch x := f.resolve(v).(type) {
		case int:
			code = x
		case float64:
			code = int(x)
		case name:
			if code >= 0 && code < 256 {
				enc[code] = glyphText(string(x))
				if enc[code] == "" && ft.tt == nil {
					// Unknown glyph names such as "g37" or "c65" carry no
					// meaning; keep the code unmapped.
					enc[code] = ""
				}
			}
			code++
		}
	}
	if ft.tt != nil {
		for c, gid := range ft.tt.codeGID {
			if enc[c] == "" || symbolic {
				if s, ok := ft.tt.gidText[gid]; ok {
					enc[c] = s
				}
			}
		}
	}
	ft.text = enc
}

// standardMetrics returns built-in widths for the standard sans and serif
// faces when referenced by name.
func standardMetrics(n string) *stdMetrics {
	l := strings.ToLower(strings.ReplaceAll(n, " ", ""))
	bold := strings.Contains(l, "bold")
	italic := strings.Contains(l, "italic") || strings.Contains(l, "oblique")
	switch {
	case strings.HasPrefix(l, "helvetica"), strings.HasPrefix(l, "arial"):
		if bold {
			return stdFontMetrics["helveticaBold"]
		}
		return stdFontMetrics["helvetica"]
	case strings.HasPrefix(l, "times"):
		switch {
		case bold && italic:
			return stdFontMetrics["timesBoldItalic"]
		case bold:
			return stdFontMetrics["timesBold"]
		case italic:
			return stdFontMetrics["timesItalic"]
		}
		return stdFontMetrics["times"]
	}
	return nil
}

func (m *stdMetrics) width(text string) (float64, bool) {
	r, _ := utf8.DecodeRuneInString(text)
	if r >= 0x20 && r < 0x100 {
		if w := m.latin[r-0x20]; w > 0 {
			return float64(w), true
		}
	}
	i := 0
	for _, x := range extraRunes {
		if x == r {
			return float64(m.extra[i]), true
		}
		i++
	}
	return 0, false
}

// style derives bold/italic/monospace from the descriptor and the name.
func (ft *pdfFont) style(desc dict, f *file) {
	l := strings.ToLower(ft.name)
	ft.bold = boldName(l)
	ft.italic = italicName(l)
	ft.mono = monoName(l)
	if desc != nil {
		flags, _ := integer(f.resolve(desc["Flags"]))
		if flags&1 != 0 {
			ft.mono = true
		}
		if flags&64 != 0 {
			ft.italic = true
		}
		if flags&(1<<18) != 0 {
			ft.bold = true
		}
		if w, ok := num(f.resolve(desc["FontWeight"])); ok && w >= 600 {
			ft.bold = true
		}
		if a, ok := num(f.resolve(desc["ItalicAngle"])); ok && math.Abs(a) > 3 && !ft.mono {
			ft.italic = true
		}
	}
	if !ft.mono && !ft.composite && ft.avgW > 0 {
		// Equal non-zero widths across a real alphabet mean a fixed-pitch
		// face even when the descriptor does not say so.
		var w0 float64
		same, n := true, 0
		for c := 'a'; c <= 'z'; c++ {
			if !ft.hasWidth[c] || ft.widths[c] == 0 {
				continue
			}
			if w0 == 0 {
				w0 = ft.widths[c]
			} else if math.Abs(ft.widths[c]-w0) > 0.5 {
				same = false
				break
			}
			n++
		}
		if same && n >= 12 {
			ft.mono = true
		}
	}
}

func boldName(l string) bool {
	for _, k := range []string{"bold", "black", "heavy", "semibold", "demi", "-bd", ",bd", "-sb"} {
		if strings.Contains(l, k) {
			return true
		}
	}
	if i := strings.Index(l, "medi"); i >= 0 && !strings.HasPrefix(l[i:], "medium") {
		return true
	}
	for _, p := range []string{"cmbx", "cmb10", "cmb12", "cmssbx", "cmbsy", "sfbx", "sfbi", "sfsx", "ecbx", "ecrb"} {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}

func italicName(l string) bool {
	for _, k := range []string{"italic", "oblique", "ital", "-it", ",it", "slanted", "kursiv", "-obl"} {
		if strings.Contains(l, k) {
			return true
		}
	}
	for _, p := range []string{"cmti", "cmsl", "cmbxti", "cmbxsl", "cmmi", "cmssi", "cmitt", "sfti", "sfsl", "sfsi", "sfbi", "sfit", "ecti", "ecsl"} {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}

func monoName(l string) bool {
	for _, k := range []string{"mono", "courier", "consola", "menlo", "monaco", "typewriter", "inconsolata", "fixed", "sourcecode", "firacode", "cascadiacode", "lucidaconsole", "ocr-a", "ocrb"} {
		if strings.Contains(l, k) {
			return true
		}
	}
	for _, p := range []string{"cmtt", "cmsltt", "cmitt", "cmtex", "sftt", "sfit", "sfst", "ectt", "lmmono", "lmtt"} {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}

// decode splits a string operand into glyphs.
func (ft *pdfFont) decode(s []byte, fn func(g glyph)) {
	if ft.composite {
		ft.decodeComposite(s, fn)
		return
	}
	for _, c := range s {
		text, ok := "", false
		if ft.toUni != nil {
			text, ok = ft.toUni.lookup(uint32(c), 1)
			if ok && !usefulText(text) {
				ok = false
			}
		}
		if !ok {
			text = ft.text[c]
		}
		if text == "" {
			ft.unmapped++
			text = "�"
		}
		fn(glyph{text: text, w: ft.simpleWidth(c, text), space: c == 32, script: ft.scriptCodes[uint32(c)]})
	}
}

// usefulText rejects ToUnicode results that carry no readable text
// (control characters), so the encoding can be used instead.
func usefulText(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r >= 0x20 && r != 0xFFFD && !(r >= 0x7F && r < 0xA0) {
			return true
		}
	}
	return false
}

func (ft *pdfFont) simpleWidth(c byte, text string) float64 {
	w := 0.0
	if ft.hasWidth[c] {
		w = ft.widths[c]
	}
	if w == 0 && ft.std != nil {
		if sw, ok := ft.std.width(text); ok {
			w = sw
		}
	}
	if w == 0 && !ft.hasWidth[c] {
		switch {
		case ft.missingW > 0:
			w = ft.missingW
		case ft.avgW > 0:
			w = ft.avgW
		case strings.Contains(strings.ToLower(ft.name), "courier"):
			w = 600
		default:
			w = 500
		}
	}
	if ft.type3 {
		return w * ft.fm[0]
	}
	return w / 1000
}

func (ft *pdfFont) decodeComposite(s []byte, fn func(g glyph)) {
	for i := 0; i < len(s); {
		var code uint32
		n := 2
		if ft.encUTF16 && ft.enc == nil {
			text, k := decodeUTF16Code(s[i:])
			n = k
			code = codeOf(s[i : i+n])
			if t, ok := ft.toUni.lookup(code, n); ok && usefulText(t) {
				text = t
			}
			fn(glyph{text: text, w: ft.cidWidth(int(code)) / 1000, script: ft.scriptCodes[code]})
			i += n
			continue
		}
		if ft.enc != nil {
			n = ft.enc.codeLen(s[i:], 2)
		} else {
			n = min(2, len(s)-i)
		}
		code = codeOf(s[i : i+n])
		cid := int(code)
		if ft.enc != nil {
			if c, ok := ft.enc.cidOf(code, n); ok {
				cid = c
			}
		}
		text, ok := ft.toUni.lookup(code, n)
		if ok && !usefulText(text) {
			ok = false
		}
		if !ok && ft.encUTF16 {
			text, _ = decodeUTF16Code(s[i : i+n])
			ok = true
		}
		if !ok && ft.tt != nil {
			gid := cid
			if ft.cidGID != nil {
				if cid < len(ft.cidGID) {
					gid = int(ft.cidGID[cid])
				}
			}
			text, ok = ft.tt.gidText[gid]
		}
		if !ok || text == "" {
			ft.unmapped++
			text = "�"
		}
		fn(glyph{text: text, w: ft.cidWidth(cid) / 1000, space: n == 1 && s[i] == 32, script: ft.scriptCodes[code]})
		i += n
	}
}

func (ft *pdfFont) cidWidth(cid int) float64 {
	// cidW is sorted by lo; ranges rarely overlap, so the entry with the
	// largest lo <= cid (or one just before it) holds the width.
	i := sort.Search(len(ft.cidW), func(i int) bool { return ft.cidW[i].lo > cid })
	for k := i - 1; k >= 0 && k >= i-8; k-- {
		if w := ft.cidW[k]; cid >= w.lo && cid <= w.hi {
			return w.w
		}
	}
	return ft.dw
}

// isSpaceText reports whether a glyph's text is (only) blank.
func isSpaceText(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsSpace(r) && r != 0x200B {
			return false
		}
	}
	return true
}
