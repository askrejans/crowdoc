package rtf

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Pseudo code pages for fonts whose glyphs are not encoded as text.
const (
	cpSymbol    = -2
	cpWingdings = -3
	cpUTF8      = 65001
)

// singleByte returns the charmap for a Windows/DOS/Mac code page.
func singleByte(cp int) *charmap.Charmap {
	switch cp {
	case 1250:
		return charmap.Windows1250
	case 1251:
		return charmap.Windows1251
	case 1252:
		return charmap.Windows1252
	case 1253:
		return charmap.Windows1253
	case 1254:
		return charmap.Windows1254
	case 1255:
		return charmap.Windows1255
	case 1256:
		return charmap.Windows1256
	case 1257:
		return charmap.Windows1257
	case 1258:
		return charmap.Windows1258
	case 874:
		return charmap.Windows874
	case 437:
		return charmap.CodePage437
	case 850:
		return charmap.CodePage850
	case 852:
		return charmap.CodePage852
	case 855:
		return charmap.CodePage855
	case 866:
		return charmap.CodePage866
	case 10000:
		return charmap.Macintosh
	case 10007:
		return charmap.MacintoshCyrillic
	case 20866:
		return charmap.KOI8R
	case 21866:
		return charmap.KOI8U
	case 28591:
		return charmap.ISO8859_1
	case 28592:
		return charmap.ISO8859_2
	case 28594:
		return charmap.ISO8859_4
	case 28595:
		return charmap.ISO8859_5
	case 28597:
		return charmap.ISO8859_7
	case 28603:
		return charmap.ISO8859_13
	case 28605:
		return charmap.ISO8859_15
	}
	return nil
}

// multiByte returns a decoder for the double-byte East Asian code pages.
func multiByte(cp int) encoding.Encoding {
	switch cp {
	case 932:
		return japanese.ShiftJIS
	case 936:
		return simplifiedchinese.GBK
	case 949:
		return korean.EUCKR
	case 950:
		return traditionalchinese.Big5
	}
	return nil
}

// charsetCodePage maps a \fcharset value to a code page; 0 means "use the
// document code page".
func charsetCodePage(cs int) int {
	switch cs {
	case 2:
		return cpSymbol
	case 77:
		return 10000
	case 128:
		return 932
	case 129:
		return 949
	case 134:
		return 936
	case 136:
		return 950
	case 161:
		return 1253
	case 162:
		return 1254
	case 163:
		return 1258
	case 177:
		return 1255
	case 178:
		return 1256
	case 186:
		return 1257
	case 204:
		return 1251
	case 222:
		return 874
	case 238:
		return 1250
	case 254:
		return 437
	case 255:
		return 850
	}
	return 0
}

// decoder turns code-page bytes into text, caching charmap tables.
type decoder struct {
	tables map[int]*[256]rune
}

func (d *decoder) table(cp int) *[256]rune {
	if t, ok := d.tables[cp]; ok {
		return t
	}
	cm := singleByte(cp)
	if cm == nil {
		cm = charmap.Windows1252
	}
	t := new([256]rune)
	for i := range 256 {
		t[i] = cm.DecodeByte(byte(i))
	}
	if d.tables == nil {
		d.tables = map[int]*[256]rune{}
	}
	d.tables[cp] = t
	return t
}

// decode converts raw bytes in code page cp to UTF-8 text.
func (d *decoder) decode(b []byte, cp int) string {
	switch cp {
	case cpSymbol:
		return mapBytes(b, symbolRune)
	case cpWingdings:
		return mapBytes(b, wingdingsRune)
	case cpUTF8:
		return strings.ToValidUTF8(string(b), "\ufffd")
	}
	if enc := multiByte(cp); enc != nil {
		out, err := enc.NewDecoder().Bytes(b)
		if err == nil && utf8.Valid(out) {
			return string(out)
		}
	}
	t := d.table(cp)
	buf := make([]rune, 0, len(b))
	for _, c := range b {
		if r := t[c]; r != utf8.RuneError {
			buf = append(buf, r)
		}
	}
	return string(buf)
}

func mapBytes(b []byte, f func(byte) rune) string {
	buf := make([]rune, 0, len(b))
	for _, c := range b {
		if r := f(c); r != 0 {
			buf = append(buf, r)
		}
	}
	return string(buf)
}

// symbolRune maps the Adobe Symbol font encoding to Unicode.
func symbolRune(c byte) rune {
	if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
		return symbolLetters[c]
	}
	if r, ok := symbolExtra[c]; ok {
		return r
	}
	if c < 0x80 {
		return rune(c)
	}
	return 0
}

var symbolLetters = func() [128]rune {
	var t [128]rune
	upper := "ΑΒΧΔΕΦΓΗΙϑΚΛΜΝΟΠΘΡΣΤΥςΩΞΨΖ"
	lower := "αβχδεφγηιϕκλμνοπθρστυϖωξψζ"
	i := 0
	for _, r := range upper {
		t['A'+i] = r
		i++
	}
	i = 0
	for _, r := range lower {
		t['a'+i] = r
		i++
	}
	return t
}()

var symbolExtra = map[byte]rune{
	0x22: '∀', 0x24: '∃', 0x27: '∋', 0x2A: '∗', 0x2D: '−', 0x40: '≅', 0x5C: '∴', 0x5E: '⊥',
	0x60: '‾', 0x7E: '∼', 0xA1: 'ϒ', 0xA2: '′', 0xA3: '≤', 0xA4: '⁄', 0xA5: '∞', 0xA6: 'ƒ',
	0xA7: '♣', 0xA8: '♦', 0xA9: '♥', 0xAA: '♠', 0xAB: '↔', 0xAC: '←', 0xAD: '↑', 0xAE: '→',
	0xAF: '↓', 0xB0: '°', 0xB1: '±', 0xB2: '″', 0xB3: '≥', 0xB4: '×', 0xB5: '∝', 0xB6: '∂',
	0xB7: '•', 0xB8: '÷', 0xB9: '≠', 0xBA: '≡', 0xBB: '≈', 0xBC: '…', 0xBF: '↵', 0xC0: 'ℵ',
	0xC1: 'ℑ', 0xC2: 'ℜ', 0xC3: '℘', 0xC4: '⊗', 0xC5: '⊕', 0xC6: '∅', 0xC7: '∩', 0xC8: '∪',
	0xC9: '⊃', 0xCA: '⊇', 0xCB: '⊄', 0xCC: '⊂', 0xCD: '⊆', 0xCE: '∈', 0xCF: '∉', 0xD0: '∠',
	0xD1: '∇', 0xD2: '®', 0xD3: '©', 0xD4: '™', 0xD5: '∏', 0xD6: '√', 0xD7: '⋅', 0xD8: '¬',
	0xD9: '∧', 0xDA: '∨', 0xDB: '⇔', 0xDC: '⇐', 0xDD: '⇑', 0xDE: '⇒', 0xDF: '⇓', 0xE0: '◊',
	0xE1: '〈', 0xE5: '∑', 0xF1: '〉', 0xF2: '∫',
}

// wingdingsRune maps the handful of Wingdings glyphs commonly used as
// bullets and check marks; everything else is dropped.
func wingdingsRune(c byte) rune {
	switch c {
	case 0x6C, 0x9F:
		return '●'
	case 0x6E, 0xA7:
		return '▪'
	case 0x71, 0x6F:
		return '□'
	case 0x76:
		return '❖'
	case 0xA8:
		return '◻'
	case 0xD8:
		return '➢'
	case 0xE0, 0xE8:
		return '➔'
	case 0xFB:
		return '✗'
	case 0xFC:
		return '✓'
	case 0xFE:
		return '☑'
	}
	return 0
}

// lcidTags maps common Windows language identifiers to BCP 47 tags.
var lcidTags = map[int]string{
	1025: "ar", 1026: "bg", 1027: "ca", 1028: "zh-TW", 1029: "cs", 1030: "da", 1031: "de",
	1032: "el", 1033: "en-US", 1034: "es", 1035: "fi", 1036: "fr", 1037: "he", 1038: "hu",
	1039: "is", 1040: "it", 1041: "ja", 1042: "ko", 1043: "nl", 1044: "nb", 1045: "pl",
	1046: "pt-BR", 1048: "ro", 1049: "ru", 1050: "hr", 1051: "sk", 1053: "sv", 1055: "tr",
	1057: "id", 1058: "uk", 1059: "be", 1060: "sl", 1061: "et", 1062: "lv", 1063: "lt",
	1066: "vi", 1071: "mk", 1081: "hi", 1086: "ms", 1087: "kk", 2052: "zh-CN", 2055: "de-CH",
	2057: "en-GB", 2058: "es-MX", 2060: "fr-BE", 2067: "nl-BE", 2068: "nn", 2070: "pt-PT",
	2074: "sr-Latn", 3079: "de-AT", 3081: "en-AU", 3082: "es", 3084: "fr-CA", 3098: "sr-Cyrl",
	4105: "en-CA", 5129: "en-NZ", 6153: "en-IE",
}
