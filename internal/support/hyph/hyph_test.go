package hyph

import (
	"bufio"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// mark renders word with '-' at the given rune offsets.
func mark(word string, cuts []int) string {
	var b strings.Builder
	i := 0
	for k, r := range []rune(word) {
		if i < len(cuts) && cuts[i] == k {
			b.WriteByte('-')
			i++
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TestHyphenateNaturalWords checks real words against TeX's output for the
// same patterns, filtered by each language's hyphenmins.
func TestHyphenateNaturalWords(t *testing.T) {
	tests := []struct{ tag, want string }{
		{"lv", "star-ptau-tis-kā"},
		{"lv", "sa-dar-bī-ba"},
		{"lv", "no-dro-ši-nā-ša-na"},
		{"lv", "grā-mat-ve-dī-ba"},
		{"lv", "uz-ņē-mēj-dar-bī-ba"},
		{"lv", "ap-lie-ci-nā-jums"},
		{"lv", "ģi-me-ne"},
		{"lv", "ķir-bis"},
		{"lv", "ļau-dis"},
		{"lv", "šķēr-slis"},
		{"lv", "žur-nā-lists"},
		{"lv", "uni-ver-si-tā-te"},
		{"lv", "priekš-li-kums"},
		{"lv", "Lat-vi-ja"},
		{"lv", "LAT-VI-JAS"},
		{"lv", "teh-no-lo-ģi-jas"},
		{"lv", "vien-o-ša-nās"},
		{"lv", "ap-stip-ri-nā-ša-na"},
		{"lv", "Wel-lin-gton"}, // foreign words follow the Latvian patterns, as in TeX
		{"lv", "džungļi"},
		{"lv", "ozols"},
		{"lv", "un"},
		{"lv-LV", "dzī-vok-lis"},
		{"ro", "in-ter-națio-na-lă"},
		{"ro", "co-la-bo-ra-rea"},
		{"ro", "ad-mi-nis-tra-ți-i-lor"},
		{"ro", "Ro-mâ-nia"},
		{"ro", "în-vă-țămân-tul"},
		{"ro", "în-tre-prin-de-rea"},
		{"ro", "res-pon-sa-bi-li-ta-te"},
		{"ro", "le-gi-sla-ție"},
		{"ro", "con-ta-bi-li-ta-te"},
		{"mk", "ме-ѓу-на-род-на"},
		{"mk", "са-мо-уп-ра-ви-те"},
		{"mk", "Ма-ке-до-ни-ја"},
		{"mk", "џа-ми-ја"},
		{"ga", "idir-náis-iúnta"},
		{"ga", "comh-oib-riú"},
		{"ga", "Éir-eann"},
		{"ga", "teic-neol-aí-ocht"},
		{"ga", "oid-eachas"},
		{"ga", "bhrachtaí"},   // exception: never divided
		{"ga", "dtiom-áintí"}, // exception
		{"ga", "tiom-áintí"},  // patterns, right hyphenmin 3
		{"eu", "na-zioar-te-ko"},
		{"eu", "ad-mi-nis-tra-zioen-tzat"},
		{"eu", "uni-ber-tsi-ta-tea"},
		{"eu", "hiz-kun-tza"},
		{"eu", "He-rria"},
		{"cnr", "me-đu-na-rod-na"},
		{"cnr", "sa-mo-upra-va-ma"},
		{"cnr", "lju-bav"},
		{"cnr", "džun-gla"},
		{"cnr", "Pod-go-ri-ca"},
		{"sr-Latn", "od-go-vor-nost"},
		{"sr-ME", "sa-rad-nja"},
		{"bs", "do-ku-men-ta-ci-ja"},
		{"hr", "injek-ci-ja"},
	}
	for _, tt := range tests {
		word := strings.ReplaceAll(tt.want, "-", "")
		if got := mark(word, Hyphenate(word, tt.tag)); got != tt.want {
			t.Errorf("Hyphenate(%q, %q) = %s, want %s", word, tt.tag, got, tt.want)
		}
	}
}

// TestPatternsMatchReference compares the raw algorithm (hyphenmins 1/1)
// with reference output for natural words and random letter strings.
func TestPatternsMatchReference(t *testing.T) {
	f, err := os.Open("testdata/reference.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	raw := map[string]*patternSet{}
	h := &hyphenator{}
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		tag, want, _ := strings.Cut(line, "\t")
		l := langs[tag]
		if raw[tag] == nil {
			raw[tag] = compile(embedded(l.patterns), embedded(l.exceptions), 1, 1)
		}
		h.l, h.p = l, raw[tag]
		word := strings.ReplaceAll(want, "-", "")
		if !h.analyse(word) {
			t.Errorf("%s: %q not analysable", tag, word)
			continue
		}
		cuts := make([]int, 0, len(h.s.breaks))
		for _, m := range h.s.breaks {
			cuts = append(cuts, h.s.runeOff[m])
		}
		if got := mark(word, cuts); got != want {
			t.Errorf("%s: got %s, want %s", tag, got, want)
		}
		n++
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if n < 1800 {
		t.Fatalf("only %d reference words checked", n)
	}
}

func TestCompileSemantics(t *testing.T) {
	// Odd values allow a break, even values forbid it, the highest value at a
	// gap wins, '.' anchors a pattern to a word boundary, and exceptions
	// replace the patterns entirely.
	p := compile("a1b\na2bc\n.ab3\n% comment\nLEFTHYPHENMIN 1\nx/y=z,1,1", "ta-ble\nnoth-ing", 1, 1)
	h := &hyphenator{l: langs["lv"], p: p}
	cases := []struct{ word, want string }{
		{"abab", "a-b-a-b"},
		{"abc", "ab-c"},  // a2bc outranks a1b; .ab3 applies at the start
		{"cabc", "cabc"}, // .ab3 does not apply inside the word
		{"cab", "ca-b"},  // a1b alone
		{"table", "ta-ble"},
		{"Table", "Ta-ble"}, // exceptions match case-insensitively
		{"nothing", "noth-ing"},
	}
	for _, c := range cases {
		if !h.analyse(c.word) {
			t.Fatalf("%q not analysable", c.word)
		}
		cuts := []int{}
		for _, m := range h.s.breaks {
			cuts = append(cuts, h.s.runeOff[m])
		}
		if got := mark(c.word, cuts); got != c.want {
			t.Errorf("%q: got %s, want %s", c.word, got, c.want)
		}
	}
	for _, r := range "/=,LEFTHYPN" {
		if p.sym(r) != 0 {
			t.Errorf("rune %q from a skipped line entered the alphabet", r)
		}
	}
}

func TestHyphenmins(t *testing.T) {
	// Irish uses right hyphenmin 3: no break may leave fewer than three
	// letters at the end of a word.
	for _, w := range []string{"comhoibriú", "teicneolaíocht", "riaracháin"} {
		for _, c := range Hyphenate(w, "ga") {
			if n := utf8.RuneCountInString(w); n-c < 3 || c < 2 {
				t.Errorf("ga %q: break %d violates hyphenmins", w, c)
			}
		}
	}
	for _, w := range []string{"ab", "abc", "ozo", "ar"} {
		if got := Hyphenate(w, "lv"); got != nil {
			t.Errorf("short word %q hyphenated: %v", w, got)
		}
	}
}

func TestHyphenateEdgeCases(t *testing.T) {
	tests := []struct {
		name, word, tag string
		want            []int
	}{
		{"unsupported language", "internationalisation", "en", nil},
		{"Typst-covered Serbian Cyrillic", "међународна", "sr", nil},
		{"digits", "sadarbība2026", "lv", nil},
		{"hyphen", "Rīga-Jūrmala", "lv", nil},
		{"soft hyphen", "sadar\u00adbība", "lv", nil},
		{"underscore", "sadarbība_x", "lv", nil},
		{"space", "sadarbība un", "lv", nil},
		{"Cyrillic in Latvian", "сотрудничество", "lv", nil},
		{"mixed scripts", "sadarbībaсотрудничество", "lv", nil},
		{"Latin in Macedonian", "sadarbiba", "mk", nil},
		{"empty", "", "lv", nil},
		{"apostrophe splits", "valoda'valoda", "lv", []int{2, 4, 9, 11}},
		{"typographic apostrophe", "valoda’valoda", "lv", []int{2, 4, 9, 11}},
		{"short part after apostrophe", "l'administrațiilor", "ro", []int{4, 6, 9, 12, 14, 15}},
		{"Romanian cedilla letters", "administraţiilor", "ro", []int{2, 4, 7, 10, 12, 13}},
		{"Romanian comma letters", "administrațiilor", "ro", []int{2, 4, 7, 10, 12, 13}},
		{"POSIX locale tag", "sadarbība", "lv_LV.UTF-8", []int{2, 5, 7}},
		{"upper-case tag", "sadarbība", "LV", []int{2, 5, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Hyphenate(tt.word, tt.tag)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Hyphenate(%q, %q) = %v, want %v", tt.word, tt.tag, got, tt.want)
			}
		})
	}
}

func TestHyphenateDecomposedInput(t *testing.T) {
	nfc := "uzņēmējdarbība"
	nfd := norm.NFD.String(nfc)
	if nfd == nfc {
		t.Fatal("test word has no decomposition")
	}
	want := Hyphenate(nfc, "lv")
	got := Hyphenate(nfd, "lv")
	if len(got) != len(want) || len(want) == 0 {
		t.Fatalf("NFD breaks %v, NFC breaks %v", got, want)
	}
	// Offsets refer to the decomposed runes and must never split a letter
	// from its combining mark.
	runes, composed := []rune(nfd), []rune(nfc)
	for i, c := range got {
		if isMark(runes[c]) {
			t.Errorf("break %d lands on a combining mark", c)
		}
		if norm.NFC.String(string(runes[:c])) != string(composed[:want[i]]) {
			t.Errorf("break %d does not correspond to NFC break %d", c, want[i])
		}
	}
}

func TestSupported(t *testing.T) {
	yes := []string{"lv", "lv-LV", "LV", "lv_LV", "ro", "ro-MD", "mo", "mk", "ga", "ga-IE", "eu", "eu-ES",
		"cnr", "cnr-ME", "sr-Latn", "sr-Latn-RS", "sr-ME", "sh", "hr", "bs"}
	no := []string{"", "en", "de", "sr", "sr-Cyrl", "sr-RS", "lb", "ja", "zh", "ko", "ar", "he", "xx", "!!", "und"}
	for _, tag := range yes {
		if !Supported(tag) {
			t.Errorf("Supported(%q) = false", tag)
		}
	}
	for _, tag := range no {
		if Supported(tag) {
			t.Errorf("Supported(%q) = true", tag)
		}
	}
}

func TestTypstHyphenates(t *testing.T) {
	yes := []string{"en", "en-US", "de", "de-AT", "fr", "lt", "et", "fi", "sv", "nb", "no", "nn", "da", "is",
		"nl", "it", "es", "pt", "pt-BR", "pl", "cs", "sk", "sl", "hr", "sq", "bg", "el", "hu", "tr", "uk",
		"ru", "sr", "sr-Cyrl", "ca"}
	no := []string{"lv", "lb", "cnr", "bs", "mk", "ro", "eu", "ga", "gl", "sr-Latn", "sr-ME",
		"ja", "zh", "ko", "ar", "he", "", "xx"}
	for _, tag := range yes {
		if !TypstHyphenates(tag) {
			t.Errorf("TypstHyphenates(%q) = false", tag)
		}
	}
	for _, tag := range no {
		if TypstHyphenates(tag) {
			t.Errorf("TypstHyphenates(%q) = true", tag)
		}
	}
	// Every embedded language must fill a gap Typst leaves, except Croatian,
	// whose patterns are only embedded for its sister varieties.
	for code := range langs {
		if code != "hr" && code != "sr" && TypstHyphenates(code) {
			t.Errorf("patterns for %q duplicate Typst's own hyphenation", code)
		}
	}
}

func TestAllPatternSetsLoad(t *testing.T) {
	for code, l := range langs {
		p := l.load()
		if len(p.edgeSym) < 100 {
			t.Errorf("%s: pattern trie has only %d edges", code, len(p.edgeSym))
		}
	}
	if p := langs["ga"].load(); len(p.exceptions) < 40 {
		t.Errorf("ga: %d exceptions loaded", len(p.exceptions))
	}
}
