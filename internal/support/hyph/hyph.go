// Package hyph hyphenates words with Frank Liang's algorithm (the one TeX
// uses) for languages whose hyphenation the Typst typesetter does not
// provide. Patterns are embedded from free-licensed sources (see PATTERNS.md)
// and compiled lazily, once per language, on first use.
//
// Languages that are written without hyphenation are deliberately not covered:
// Chinese, Japanese and Korean break lines between characters or syllable
// blocks, and Arabic and Hebrew are justified without word division. No
// free-licensed patterns exist for Luxembourgish.
package hyph

import (
	"embed"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/language"
)

//go:embed patterns/*
var patternFS embed.FS

// lang describes one embedded pattern set.
type lang struct {
	patterns    string // file under patterns/
	exceptions  string // optional file under patterns/
	left, right int    // typesetting hyphenmins from the pattern source
	script      *unicode.RangeTable
	fold        func(rune) rune // extra per-language letter folding, or nil

	once sync.Once
	set  *patternSet
}

func (l *lang) load() *patternSet {
	l.once.Do(func() {
		l.set = compile(embedded(l.patterns), embedded(l.exceptions), l.left, l.right)
	})
	return l.set
}

// embedded returns an embedded pattern file. The files are compiled into
// the binary, so a read error cannot occur in a correct build; it would
// merely disable hyphenation for the language.
func embedded(name string) string {
	if name == "" {
		return ""
	}
	b, err := patternFS.ReadFile("patterns/" + name)
	if err != nil {
		return ""
	}
	return string(b)
}

// Serbo-Croatian Latin varieties share the Croatian patterns: the Serbian
// Latin TeX patterns are LPPL-only, while the Croatian ones are also offered
// under a permissive licence, and the syllabification rules are the same.
var serboCroatianLatin = &lang{patterns: "hyph-hr.pat.txt", left: 2, right: 2, script: unicode.Latin}

var langs = map[string]*lang{
	"lv":  {patterns: "hyph-lv.pat.txt", left: 2, right: 2, script: unicode.Latin},
	"ro":  {patterns: "hyph_ro_RO.dic", left: 2, right: 2, script: unicode.Latin, fold: foldRomanian},
	"mk":  {patterns: "hyph-mk.pat.txt", left: 2, right: 2, script: unicode.Cyrillic},
	"ga":  {patterns: "hyph-ga.pat.txt", exceptions: "hyph-ga.hyp.txt", left: 2, right: 3, script: unicode.Latin},
	"eu":  {patterns: "hyph-eu.pat.txt", left: 2, right: 2, script: unicode.Latin},
	"hr":  serboCroatianLatin,
	"bs":  serboCroatianLatin,
	"cnr": serboCroatianLatin,
	"sr":  serboCroatianLatin, // Latin script only; see resolve
}

// foldRomanian maps the legacy cedilla letters, still common in Romanian
// text, to the comma-below letters the patterns use.
func foldRomanian(r rune) rune {
	switch r {
	case 'ş':
		return 'ș'
	case 'ţ':
		return 'ț'
	}
	return r
}

// typstLangs lists the ISO 639-1 codes Typst 0.15 hyphenates, verified by
// typesetting sample words in a narrow justified column with each code.
// Serbian is hyphenated in Cyrillic script only. Notably absent although the
// underlying pattern library knows them: Galician (gl). Three-letter codes
// (deu, nob, …) are not hyphenated by Typst at all.
var typstLangs = map[string]bool{
	"af": true, "be": true, "bg": true, "ca": true, "cs": true, "da": true,
	"de": true, "el": true, "en": true, "es": true, "et": true, "fi": true,
	"fr": true, "hr": true, "hu": true, "is": true, "it": true, "ka": true,
	"ku": true, "la": true, "lt": true, "mn": true, "nb": true, "nl": true,
	"nn": true, "no": true, "pl": true, "pt": true, "ru": true, "sk": true,
	"sl": true, "sq": true, "sr": true, "sv": true, "tk": true, "tr": true,
	"uk": true,
}

// parseTag returns the lowercase base language and script of a BCP 47 tag
// (accepting '_' separators and POSIX locale suffixes like ".UTF-8").
func parseTag(tag string) (base, script string) {
	tag = strings.TrimSpace(tag)
	if i := strings.IndexAny(tag, ".@"); i >= 0 {
		tag = tag[:i]
	}
	t, err := language.Parse(tag)
	if err != nil {
		return "", ""
	}
	b, conf := t.Base()
	if conf == language.No || t.IsRoot() {
		return "", ""
	}
	s, _ := t.Script()
	return b.String(), s.String()
}

// resolve returns the pattern set for tag, or nil when none is embedded.
func resolve(tag string) *lang {
	base, script := parseTag(tag)
	if base == "sr" && script != "Latn" {
		return nil
	}
	if base == "hr" || base == "bs" || base == "cnr" {
		if script == "Cyrl" {
			return nil
		}
	}
	return langs[base]
}

// Supported reports whether patterns for tag are embedded in this package:
// Latvian, Romanian, Macedonian, Irish, Basque and the Latin-script
// Serbo-Croatian varieties (Croatian, Bosnian, Montenegrin, Serbian Latin).
func Supported(tag string) bool {
	return resolve(tag) != nil
}

// TypstHyphenates reports whether Typst 0.15 hyphenates text tagged with tag
// by itself. When it does not but Supported(tag) is true, callers should
// insert soft hyphens with InsertSoftHyphens before handing text to Typst.
func TypstHyphenates(tag string) bool {
	base, script := parseTag(tag)
	if base == "sr" {
		return script != "Latn"
	}
	if base == "" || len(base) != 2 {
		return false
	}
	return typstLangs[base]
}

// Hyphenate returns the positions, as rune offsets into word, before which a
// hyphen may be inserted, in ascending order. It returns nil when tag has no
// patterns here or when word is not a plain word: it contains digits,
// underscores, hyphens, soft hyphens, whitespace or punctuation other than
// apostrophes, or letters of another script. Parts separated by apostrophes
// are hyphenated independently.
func Hyphenate(word, tag string) []int {
	l := resolve(tag)
	if l == nil || word == "" {
		return nil
	}
	for _, r := range word {
		if !isWordRune(r) || isBlockingRune(r) {
			return nil
		}
	}
	h := newHyphenator(l)
	var out []int
	start, startRune, runes := 0, 0, 0
	piece := func(end int) {
		if h.analyse(word[start:end]) {
			for _, m := range h.s.breaks {
				out = append(out, startRune+h.s.runeOff[m])
			}
		}
	}
	for i, r := range word {
		runes++
		if isApostrophe(r) {
			piece(i)
			start, startRune = i+utf8.RuneLen(r), runes
		}
	}
	piece(len(word))
	return out
}
