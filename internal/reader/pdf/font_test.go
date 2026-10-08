package pdf

import (
	"encoding/binary"
	"testing"

	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

const testCMap = `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Test def
2 begincodespacerange
<00> <7F>
<8000> <FFFF>
endcodespacerange
3 beginbfchar
<41> <0042>
<8001> <00660069>
<8002> /eacute
endbfchar
3 beginbfrange
<61> <63> <0041>
<9000> <9002> [<0078> <D835DC00> <00410042>]
<A0FE> <A101> <0061>
endbfrange
1 begincidrange
<8100> <81FF> 500
endcidrange
1 begincidchar
<8200> 1000
endcidchar
endcmap
CMapName currentdict /CMap defineresource pop
end
end`

func TestCMap(t *testing.T) {
	c := parseCMap([]byte(testCMap))
	lookups := []struct {
		code uint32
		n    int
		want string
	}{
		{0x41, 1, "B"},
		{0x61, 1, "A"},
		{0x63, 1, "C"},
		{0x8001, 2, "fi"},
		{0x8002, 2, "é"},
		{0x9000, 2, "x"},
		{0x9001, 2, "𝐀"}, // a surrogate pair in the array form
		{0x9002, 2, "AB"},
		{0xA0FF, 2, "b"}, // a range crossing a byte boundary
		{0xA101, 2, "d"},
	}
	for _, l := range lookups {
		if got, ok := c.lookup(l.code, l.n); !ok || got != l.want {
			t.Errorf("lookup(%#x, %d) = %q, %v; want %q", l.code, l.n, got, ok, l.want)
		}
	}
	if _, ok := c.lookup(0x64, 1); ok {
		t.Error("code outside every range was mapped")
	}
	if n := c.codeLen([]byte{0x41, 0x80, 0x01}, 2); n != 1 {
		t.Errorf("codeLen(41 ..) = %d", n)
	}
	if n := c.codeLen([]byte{0x80, 0x01}, 2); n != 2 {
		t.Errorf("codeLen(80 01) = %d", n)
	}
	if cid, ok := c.cidOf(0x8105, 2); !ok || cid != 505 {
		t.Errorf("cidOf range = %d, %v", cid, ok)
	}
	if cid, ok := c.cidOf(0x8200, 2); !ok || cid != 1000 {
		t.Errorf("cidOf char = %d, %v", cid, ok)
	}
	// A nil map (no ToUnicode) maps nothing.
	var none *cmap
	if _, ok := none.lookup(0x41, 1); ok {
		t.Error("nil cmap mapped a code")
	}
}

func TestCMapHostile(t *testing.T) {
	// Huge ranges and garbage do not blow up.
	c := parseCMap([]byte("1 beginbfrange <0000> <FFFFFFFF> <0041> endbfrange 1 beginbfchar <00> endbfchar <41 begincodespacerange"))
	if _, ok := c.lookup(0x10, 2); ok {
		t.Error("oversized range accepted")
	}
}

func fontFile(t *testing.T, objs ...string) *file {
	t.Helper()
	all := append([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [] /Count 0 >>",
	}, objs...)
	f, err := openFile(classicPDF("/Root 1 0 R", all...), rd.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func decodeAll(ft *pdfFont, s string) ([]string, []float64) {
	var texts []string
	var widths []float64
	ft.decode([]byte(s), func(g glyph) {
		texts = append(texts, g.text)
		widths = append(widths, g.w)
	})
	return texts, widths
}

func TestCompositeFont(t *testing.T) {
	toUni := `begincmap 1 begincodespacerange <0000> <FFFF> endcodespacerange
5 beginbfchar <0001> <0048> <0002> <0069> <0003> <0031> <0004> <0031> <0005> <00AD> endbfchar endcmap`
	f := fontFile(t,
		"<< /Type /Font /Subtype /Type0 /BaseFont /ABCDEF+TestSans-Bold /Encoding /Identity-H /DescendantFonts [4 0 R] /ToUnicode 5 0 R >>",
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /ABCDEF+TestSans-Bold /DW 1000 /W [1 [700 300] 3 3 520 4 4 310] /FontDescriptor 6 0 R >>",
		streamObj("", []byte(toUni)),
		"<< /Type /FontDescriptor /FontName /ABCDEF+TestSans-Bold /Flags 32 /ItalicAngle 0 >>",
	)
	ft := f.fontByRef(ref{3, 0})
	if ft == nil || !ft.composite || ft.name != "TestSans-Bold" || !ft.bold || ft.italic || ft.mono {
		t.Fatalf("font %+v", ft)
	}
	texts, widths := decodeAll(ft, "\x00\x01\x00\x02\x00\x03\x00\x04\x00\x09")
	if want := []string{"H", "i", "1", "1", "�"}; !equalStrings(texts, want) {
		t.Errorf("texts %q, want %q", texts, want)
	}
	if want := []float64{0.7, 0.3, 0.52, 0.31, 1}; !equalFloats(widths, want) {
		t.Errorf("widths %v, want %v", widths, want)
	}
	// Code 4 renders "1" much narrower than code 3: a superscript form.
	if !ft.scriptCodes[4] || ft.scriptCodes[3] {
		t.Errorf("script codes %v", ft.scriptCodes)
	}
	if ft.unmapped != 1 {
		t.Errorf("unmapped = %d", ft.unmapped)
	}
}

func TestSimpleFontEncoding(t *testing.T) {
	f := fontFile(t,
		"<< /Type /Font /Subtype /Type1 /BaseFont /Times-Italic /FirstChar 65 /LastChar 66 /Widths [722 667] /Encoding << /BaseEncoding /WinAnsiEncoding /Differences [65 /Aring 97 /fi /uni0101 /g37 /gcommaaccent /f_f_i /a.sc] >> >>",
	)
	ft := f.fontByRef(ref{3, 0})
	if ft == nil || !ft.italic || ft.bold || ft.composite {
		t.Fatalf("font %+v", ft)
	}
	texts, widths := decodeAll(ft, "ABabcdef\xe9")
	if want := []string{"Å", "B", "ﬁ", "ā", "�", "ģ", "ffi", "a", "é"}; !equalStrings(texts, want) {
		t.Errorf("texts %q, want %q", texts, want)
	}
	if widths[0] != 0.722 || widths[1] != 0.667 {
		t.Errorf("widths %v", widths)
	}
	// Characters outside /Widths fall back to the standard metrics.
	if widths[8] != 0.444 {
		t.Errorf("é width %v", widths[8])
	}
}

func TestGlyphNames(t *testing.T) {
	for name, want := range map[string]string{
		"A": "A", "Aacute": "Á", "ncommaaccent": "ņ", "scommaaccent": "ș", "Lcommaaccent": "Ļ",
		"emacron": "ē", "uni00410042": "AB", "u1F600": "😀", "f_f_i": "ffi", "one.sups": "1",
		"g37": "", ".notdef": "", "uniD800": "",
	} {
		if got := glyphText(name); got != want {
			t.Errorf("glyphText(%q) = %q, want %q", name, got, want)
		}
	}
	for _, c := range []struct{ base, accent, want string }{
		{"a", "¨", "ä"}, {"s", "ˇ", "š"}, {"ı", "´", "í"}, {"g", "¸", "ģ"}, {"x", "ˇ", ""},
	} {
		if got := combineAccent(c.base, c.accent); got != c.want {
			t.Errorf("combineAccent(%q, %q) = %q, want %q", c.base, c.accent, got, c.want)
		}
	}
}

func TestTrueTypeCmap(t *testing.T) {
	// A font program with only a (3,1) format 4 cmap: 'A'..'C' → 10..12.
	sub := make([]byte, 0, 64)
	put := func(v ...uint16) {
		for _, x := range v {
			sub = binary.BigEndian.AppendUint16(sub, x)
		}
	}
	put(4, 32, 0, 4, 4, 1, 0)       // format, length, language, segCountX2, search fields
	put(0x43, 0xFFFF)               // end codes
	put(0)                          // reserved pad
	put(0x41, 0xFFFF)               // start codes
	put(uint16(0x10000+10-0x41), 1) // deltas (mod 65536)
	put(0, 0)                       // range offsets
	cmapTable := binary.BigEndian.AppendUint16(nil, 0)
	cmapTable = binary.BigEndian.AppendUint16(cmapTable, 1)
	cmapTable = binary.BigEndian.AppendUint16(cmapTable, 3)
	cmapTable = binary.BigEndian.AppendUint16(cmapTable, 1)
	cmapTable = binary.BigEndian.AppendUint32(cmapTable, 12)
	cmapTable = append(cmapTable, sub...)
	font := []byte{0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	font = append(font, "cmap"...)
	font = binary.BigEndian.AppendUint32(font, 0)
	font = binary.BigEndian.AppendUint32(font, 28)
	font = binary.BigEndian.AppendUint32(font, uint32(len(cmapTable)))
	font = append(font, cmapTable...)
	info := parseTrueType(font)
	if info == nil || info.gidText[10] != "A" || info.gidText[12] != "C" || info.unicodeGID['B'] != 11 {
		t.Fatalf("info %+v", info)
	}
	// Truncated programs are ignored safely.
	for n := 0; n < len(font); n += 3 {
		parseTrueType(font[:n])
	}
}

func equalStrings(a, b []string) bool {
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

func equalFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if d := a[i] - b[i]; d > 1e-9 || d < -1e-9 {
			return false
		}
	}
	return true
}
