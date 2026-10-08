package pdf

import (
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Glyph names used by font encodings, mapped to Unicode. Accented Latin
// letters ("Amacron", "ncommaaccent", ...) are composed algorithmically in
// glyphText and need no entry here.
const glyphTable = `
space 20 exclam 21 quotedbl 22 numbersign 23 dollar 24 percent 25 ampersand 26
quoteright 2019 quotesingle 27 parenleft 28 parenright 29 asterisk 2A plus 2B
comma 2C hyphen 2D period 2E slash 2F zero 30 one 31 two 32 three 33 four 34
five 35 six 36 seven 37 eight 38 nine 39 colon 3A semicolon 3B less 3C equal 3D
greater 3E question 3F at 40 bracketleft 5B backslash 5C bracketright 5D
asciicircum 5E underscore 5F quoteleft 2018 grave 60 braceleft 7B bar 7C
braceright 7D asciitilde 7E exclamdown A1 cent A2 sterling A3 fraction 2044
yen A5 florin 192 section A7 currency A4 quotedblleft 201C guillemotleft AB
guillemetleft AB guilsinglleft 2039 guilsinglright 203A fi FB01 fl FB02 ff FB00
ffi FB03 ffl FB04 endash 2013 dagger 2020 daggerdbl 2021 periodcentered B7
paragraph B6 bullet 2022 quotesinglbase 201A quotedblbase 201E
quotedblright 201D guillemotright BB guillemetright BB ellipsis 2026
perthousand 2030 questiondown BF acute B4 circumflex 2C6 tilde 2DC macron AF
breve 2D8 dotaccent 2D9 dieresis A8 ring 2DA cedilla B8 hungarumlaut 2DD
ogonek 2DB caron 2C7 emdash 2014 AE C6 ordfeminine AA Lslash 141 Oslash D8
OE 152 ordmasculine BA ae E6 dotlessi 131 dotlessj 237 lslash 142 oslash F8
oe 153 germandbls DF Euro 20AC euro 20AC trademark 2122 copyright A9
registered AE degree B0 plusminus B1 multiply D7 divide F7 minus 2212
logicalnot AC brokenbar A6 mu B5 onesuperior B9 twosuperior B2
threesuperior B3 onequarter BC onehalf BD threequarters BE nbspace A0
nonbreakingspace A0 sfthyphen AD softhyphen AD Eth D0 eth F0 Thorn DE thorn FE
Dcroat 110 dcroat 111 Dslash 110 dslash 111 Hbar 126 hbar 127 Tbar 166 tbar 167
Ldot 13F ldot 140 napostrophe 149 Eng 14A eng 14B kgreenlandic 138 IJ 132
ij 133 longs 17F Schwa 18F schwa 259 Ohorn 1A0 ohorn 1A1 Uhorn 1AF uhorn 1B0
Alpha 391 Beta 392 Gamma 393 Delta 394 Epsilon 395 Zeta 396 Eta 397 Theta 398
Iota 399 Kappa 39A Lambda 39B Mu 39C Nu 39D Xi 39E Omicron 39F Pi 3A0 Rho 3A1
Sigma 3A3 Tau 3A4 Upsilon 3A5 Phi 3A6 Chi 3A7 Psi 3A8 Omega 3A9 alpha 3B1
beta 3B2 gamma 3B3 delta 3B4 epsilon 3B5 zeta 3B6 eta 3B7 theta 3B8 iota 3B9
kappa 3BA lambda 3BB nu 3BD xi 3BE omicron 3BF pi 3C0 rho 3C1 sigma1 3C2
sigma 3C3 tau 3C4 upsilon 3C5 phi 3C6 chi 3C7 psi 3C8 omega 3C9 theta1 3D1
phi1 3D5 omega1 3D6 Upsilon1 3D2 epsilon1 3F5 rho1 3F1 kappa1 3F0
infinity 221E lessequal 2264 greaterequal 2265 notequal 2260 approxequal 2248
equivalence 2261 proportional 221D partialdiff 2202 summation 2211
product 220F radical 221A integral 222B gradient 2207 nabla 2207 element 2208
notelement 2209 suchthat 220B intersection 2229 union 222A emptyset 2205
logicaland 2227 logicalor 2228 universal 2200 existential 2203 therefore 2234
perpendicular 22A5 angle 2220 propersubset 2282 propersuperset 2283
reflexsubset 2286 reflexsuperset 2287 circleplus 2295 circlemultiply 2297
similar 223C congruent 2245 dotmath 22C5 arrowleft 2190 arrowup 2191
arrowright 2192 arrowdown 2193 arrowboth 2194 arrowupdn 2195 arrowdblleft 21D0
arrowdblup 21D1 arrowdblright 21D2 arrowdbldown 21D3 arrowdblboth 21D4
minute 2032 second 2033 prime 2032 aleph 2135 weierstrass 2118 Ifraktur 2111
Rfraktur 211C angleleft 2329 angleright 232A lozenge 25CA spade 2660 club 2663
heart 2665 diamond 2666 dotlessjmath 237 asteriskmath 2217 minusplus 2213
plusminusmath B1 bulletmath 2219 circlemath 2218 diamondmath 22C4 lessmuch 226A
greatermuch 226B precedes 227A follows 227B subset 2282 superset 2283
mapsto 21A6 openbullet 25E6 filledbox 25A0 blacksquare 25A0 whitesquare 25A1
H18533 25CF H18543 25AA H18551 25AB H22073 25A1 circle 25CB invbullet 25D8
triagup 25B2 triagdn 25BC triagrt 25BA triaglf 25C4 checkmark 2713 visiblespace 2423
figuredash 2012 hyphentwo 2010 quotereversed 201B quotedblreversed 201F
afii00208 2015 horizontalbar 2015 suppress 337 exclamsmall 21
cwm 200C compwordmark 200C zerowidthspace 200B enspace 2002 emspace 2003
thinspace 2009 hairspace 200A figurespace 2007 numero 2116 estimated 212E
afii61352 2116 Omegainv 2127 ohm 2126 Ohm 2126 cedi 20B5 lira 20A4 rupee 20A8
won 20A9 sheqel 20AA dong 20AB franc 20A3 peseta 20A7 colonmonetary 20A1
centinferior 2322 interrobang 203D asciicircumupper 5E onedotenleader 2024
twodotenleader 2025 dieresistonos 385 tonos 384 commaaccent 326
`

var (
	glyphOnce sync.Once
	glyphMap  map[string]string
)

func glyphs() map[string]string {
	glyphOnce.Do(func() {
		f := strings.Fields(glyphTable)
		glyphMap = make(map[string]string, len(f)/2)
		for i := 0; i+1 < len(f); i += 2 {
			if cp, err := strconv.ParseUint(f[i+1], 16, 32); err == nil {
				glyphMap[f[i]] = string(rune(cp))
			}
		}
	})
	return glyphMap
}

// combining marks for the accent suffixes of composed glyph names.
var accentSuffix = []struct {
	name string
	mark rune
}{
	{"hungarumlaut", 0x30B}, {"circumflex", 0x302}, {"commaaccent", 0x327},
	{"dotaccent", 0x307}, {"dieresis", 0x308}, {"cedilla", 0x327},
	{"ogonek", 0x328}, {"macron", 0x304}, {"acute", 0x301}, {"grave", 0x300},
	{"tilde", 0x303}, {"caron", 0x30C}, {"breve", 0x306}, {"ring", 0x30A},
}

// glyphText maps a glyph name to its Unicode text ("" when unknown).
func glyphText(n string) string {
	if n == "" || n == ".notdef" {
		return ""
	}
	if s, ok := glyphs()[n]; ok {
		return s
	}
	if len(n) == 1 {
		c := n[0]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
			return n
		}
	}
	// Variant suffixes (".sc", ".alt", ".oldstyle") and ligatures ("f_f_i").
	if i := strings.IndexByte(n, '.'); i > 0 {
		return glyphText(n[:i])
	}
	if strings.Contains(n, "_") {
		var sb strings.Builder
		for _, part := range strings.Split(n, "_") {
			t := glyphText(part)
			if t == "" {
				return ""
			}
			sb.WriteString(t)
		}
		return sb.String()
	}
	if strings.HasPrefix(n, "uni") && len(n) >= 7 && (len(n)-3)%4 == 0 {
		var sb strings.Builder
		for i := 3; i+4 <= len(n); i += 4 {
			cp, err := strconv.ParseUint(n[i:i+4], 16, 32)
			if err != nil || (cp >= 0xD800 && cp <= 0xDFFF) {
				return ""
			}
			sb.WriteRune(rune(cp))
		}
		return sb.String()
	}
	if n[0] == 'u' && len(n) >= 5 && len(n) <= 7 {
		if cp, err := strconv.ParseUint(n[1:], 16, 32); err == nil && cp <= utf8.MaxRune && (cp < 0xD800 || cp > 0xDFFF) {
			return string(rune(cp))
		}
	}
	for _, a := range accentSuffix {
		if !strings.HasSuffix(n, a.name) || len(n) == len(a.name) {
			continue
		}
		base := n[:len(n)-len(a.name)]
		if base == "dotlessi" {
			base = "i"
		}
		bt := glyphText(base)
		if bt == "" || utf8.RuneCountInString(bt) != 1 {
			return ""
		}
		mark := a.mark
		if a.name == "commaaccent" && (bt == "s" || bt == "S") {
			mark = 0x326
		}
		return norm.NFC.String(bt + string(mark))
	}
	return ""
}

// combineAccent composes a spacing accent with the base letter it is
// printed over, as typeset by engines that build accented letters from two
// glyphs. It returns "" when the pair does not compose.
func combineAccent(base, accent string) string {
	var mark rune
	switch accent {
	case "¨", "˝", "\"":
		if accent == "˝" {
			mark = 0x30B
		} else {
			mark = 0x308
		}
	case "´", "ˊ", "'":
		mark = 0x301
	case "`", "ˋ":
		mark = 0x300
	case "ˆ", "^":
		mark = 0x302
	case "˜", "~":
		mark = 0x303
	case "¯", "ˉ":
		mark = 0x304
	case "˘":
		mark = 0x306
	case "˙":
		mark = 0x307
	case "˚", "°":
		mark = 0x30A
	case "¸", ",":
		mark = 0x327
	case "˛":
		mark = 0x328
	case "ˇ":
		mark = 0x30C
	default:
		return ""
	}
	switch base {
	case "ı":
		base = "i"
	case "ȷ":
		base = "j"
	}
	if mark == 0x327 && (base == "s" || base == "S") {
		mark = 0x326
	}
	if utf8.RuneCountInString(base) != 1 {
		return ""
	}
	out := norm.NFC.String(base + string(mark))
	if utf8.RuneCountInString(out) != 1 {
		return ""
	}
	return out
}
