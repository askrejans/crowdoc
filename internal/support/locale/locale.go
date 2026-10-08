// Package locale provides the localised fixed strings of a document —
// captions, callout titles, business and academic labels, month names and
// typographic conventions — for 40 languages, together with native date
// formatting.
//
// Every string follows the conventions a professional publisher in the
// language's main country would use: correct diacritics, native quotation
// marks, the customary term where several exist. Alternatives are noted next
// to the data only where usage is genuinely divided.
//
// Numbered captions and references must be built with Terms.Label or
// Terms.CaptionPrefix rather than by concatenation: several languages put
// the number first ("3. attēls", "3. ábra", "3 pav.") or attach it without a
// space ("図3", "第3章").
package locale

import (
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/language"
)

// Terms holds the fixed strings of one language. Values returned by this
// package are copies; callers may modify them freely.
type Terms struct {
	Tag         string // BCP 47 tag of the strings: "lv", "sr-Latn", "zh-Hant", …
	EnglishName string
	NativeName  string
	RTL         bool   // written right to left
	Script      string // ISO 15924: Latn, Cyrl, Grek, Arab, Hebr, Jpan, Hans, Hant, Kore
	TypstLang   string // value for Typst's text(lang:)
	TypstRegion string // value for Typst's text(region:); "" means none
	LaTeXBabel  string // babel language name accepted by \usepackage[…]{babel}; "" if unsupported

	// Captions and document structure.
	Contents, ListOfFigures, ListOfTables, ListOfListings, Figure, Table, Listing, Equation,
	Chapter, Section, Part, Appendix, Abstract, Keywords, References, Bibliography, Index,
	Page, PageOfFmt, Continued, ContinuedOnNext, Notes, Footnotes string

	// CaptionSep separates a numbered caption label from the caption text,
	// e.g. ": " in English ("Figure 1: …"), ". " in Latvian ("1. attēls. …"),
	// " — " in Russian (GOST: "Рисунок 1 — …").
	CaptionSep string

	// Callout and theorem-like titles.
	Note, Tip, Info, Important, Warning, Caution, Danger, Success, Example, Summary,
	Theorem, Lemma, Corollary, Proposition, Definition, Remark, Proof, Exercise, Solution string

	// Business documents. And joins the parties of an agreement ("between A
	// and B") and is therefore lower case; ForParty heads a signature block
	// signed on behalf of a party (English "For"). Languages that express
	// "on behalf of" with a postposition use a standalone phrase instead.
	Version, Status, Date, Author, Authors, PreparedBy, PreparedFor, Classification, Confidential,
	Draft, Final, Subject, To, From, CC, Re, Enclosure, Attachment, Signature, Name, Position, Place,
	Invoice, InvoiceNumber, IssueDate, DueDate, Total, Subtotal, Tax, Amount, Quantity, UnitPrice,
	Description, Memorandum, Minutes, Attendees, Agenda, ActionItems, Decisions,
	Agreement, Parties, Between, And, SignedOn, ForParty string

	// Academic title pages.
	Supervisor, Advisor, Faculty, Department, Institution, Degree, SubmittedBy, SubmittedTo,
	Thesis, Dissertation, MastersThesis, BachelorsThesis, Declaration, Acknowledgements, Dedication string

	// Months are nominative (stand-alone) names. MonthsGenitive holds the
	// form used inside a date where the language inflects it (Polish
	// "października", Finnish partitive "lokakuuta", Catalan "d’octubre",
	// Hebrew "באוקטובר"); otherwise it equals Months.
	Months, MonthsShort, MonthsGenitive [12]string

	// Typographic conventions, mainly for the LaTeX backend (Typst handles
	// quotes itself). Quote marks include any spacing the language requires.
	QuoteOpen, QuoteClose, InnerQuoteOpen, InnerQuoteClose string
	DecimalSep, ThousandsSep                               string

	longDate, shortDate string     // date patterns, see formatPattern
	label               labelStyle // order of number and term in labels
	digits              string     // the ten native digits, "" for ASCII
}

type labelStyle uint8

const (
	labelTermFirst labelStyle = iota // "Figure 3"
	labelNumberDot                   // "3. attēls": ordinal number first
	labelNumber                      // "3 pav.": cardinal number first
	labelTight                       // "図3", "第3章"
	labelKorean                      // "그림 3", "제3장"
)

// all lists the 40 supported languages in a stable order.
var all = []*Terms{
	en, lv, lt, et, fi, sv, nb, da, is, de, nl, lb, fr, it, es, pt, pl, cs, sk, sl,
	hr, cnr, sq, mk, bg, el, ro, hu, tr, uk, ru, sr, ca, eu, ga, ja, zh, ko, ar, he,
}

// srLatn and zhHant are script variants reachable through Get and Match;
// they are not separate entries of Supported.
var (
	srLatn = latinSerbian(sr)
	byTag  = map[string]*Terms{}
)

func init() {
	for _, t := range all {
		byTag[t.Tag] = t
	}
	byTag[srLatn.Tag] = srLatn
	byTag[zhHant.Tag] = zhHant
}

// Supported returns the 40 supported languages in a stable order.
func Supported() []*Terms {
	out := make([]*Terms, len(all))
	for i, t := range all {
		out[i] = t.clone()
	}
	return out
}

// Get returns the terms for a BCP 47 tag. Matching is case-insensitive and
// accepts '_' separators and POSIX suffixes ("lv_LV.UTF-8"); region subtags
// fall back to the language ("pt-BR" → Portuguese), Chinese regions and
// scripts select Simplified or Traditional strings ("zh-TW" → Traditional),
// and "sr-Latn" selects Latin-script Serbian. Unknown tags yield English.
// The result is never nil.
func Get(tag string) *Terms {
	t, _ := Match(tag)
	return t
}

// Match is like Get and also reports whether the language was recognised.
// exact is false when the English fallback was used, when a related written
// standard stands in (Norwegian Nynorsk → Bokmål), or when the tag asks for
// a script the strings are not written in. Region subtags never affect it.
func Match(tag string) (t *Terms, exact bool) {
	base, script, region, ok := parseTag(tag)
	if !ok {
		return en.clone(), false
	}
	exact = true
	var found *Terms
	switch base {
	case "sr":
		found = sr
		if script == "Latn" {
			found = srLatn
		}
	case "zh":
		found = zh
		if script == "Hant" {
			found = zhHant
		}
	case "no":
		found = nb
	case "nn":
		found, exact = nb, false
	default:
		found = byTag[base]
	}
	if found == nil {
		return en.clone(), false
	}
	if script != "" && !scriptCompatible(script, found.Script) {
		exact = false
	}
	t = found.clone()
	if t.Tag == "ar" && latinDigitArabic[region] {
		// The Maghreb writes Arabic with Latin digits and European separators.
		t.digits, t.DecimalSep, t.ThousandsSep = "", ",", "."
	}
	return t, exact
}

// scriptCompatible reports whether text in script want is served by strings
// written in script have; Japanese and Korean use composite script codes.
func scriptCompatible(want, have string) bool {
	switch {
	case want == have:
		return true
	case have == "Jpan":
		return want == "Hira" || want == "Kana" || want == "Hrkt" || want == "Hani"
	case have == "Kore":
		return want == "Hang" || want == "Hani"
	}
	return false
}

// parseTag extracts the base language, the explicit or implied script and
// the explicit region. The script is "" unless the tag states it, except for
// Serbian and Chinese, whose script follows from the region.
func parseTag(tag string) (base, script, region string, ok bool) {
	tag = strings.TrimSpace(tag)
	if i := strings.IndexAny(tag, ".@"); i >= 0 {
		tag = tag[:i]
	}
	if tag == "" {
		return "", "", "", false
	}
	lt, err := language.Parse(tag)
	if err != nil {
		return "", "", "", false
	}
	b, conf := lt.Base()
	if conf != language.Exact || lt.IsRoot() {
		return "", "", "", false
	}
	base = b.String()
	if s, sconf := lt.Script(); sconf == language.Exact || base == "sr" || base == "zh" {
		script = s.String()
	}
	if r, rconf := lt.Region(); rconf == language.Exact {
		region = r.String()
	}
	return base, script, region, true
}

func (t *Terms) clone() *Terms {
	c := *t
	return &c
}

// Label joins a term such as t.Figure, t.Table or t.Chapter with a number
// the way the language prints numbered captions and cross-references:
// "Figure 3" (en), "3. attēls" (lv), "3 pav." (lt), "3. ábra" (hu),
// "3. irudia" (eu), "図3" and "第3章" (ja), "제3장" (ko). Term and number are
// bound with a no-break space where a space is used, and digits are
// converted to the language's native digits.
func (t *Terms) Label(term, number string) string {
	number = t.localDigits(number)
	ordinal := term == t.Chapter || term == t.Section || term == t.Part
	switch t.label {
	case labelNumberDot:
		if r, _ := utf8.DecodeLastRuneInString(number); unicode.IsDigit(r) {
			number += "."
		}
		return number + "\u00a0" + lowerFirst(term)
	case labelNumber:
		return number + "\u00a0" + lowerFirst(term)
	case labelTight:
		if ordinal {
			return "第" + number + term
		}
		return term + number
	case labelKorean:
		if ordinal {
			return "제" + number + term
		}
		return term + "\u00a0" + number
	}
	return term + "\u00a0" + number
}

// CaptionPrefix returns the numbered label followed by CaptionSep, ready to
// precede caption text: "Figure 3: ", "3. attēls. ", "3 pav. ".
func (t *Terms) CaptionPrefix(term, number string) string {
	l := t.Label(term, number)
	sep := t.CaptionSep
	if strings.HasSuffix(l, ".") && strings.HasPrefix(sep, ".") {
		sep = sep[1:]
	}
	return l + sep
}

// NumberFirst reports whether the number precedes the term in labels (see
// Label), which typesetters need to know when they build captions natively.
func (t *Terms) NumberFirst() bool {
	return t.label == labelNumberDot || t.label == labelNumber
}

// lowerFirst lower-cases the first letter of a capitalised word, leaving
// abbreviations and all-caps words alone.
func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if !unicode.IsUpper(r) {
		return s
	}
	if next, _ := utf8.DecodeRuneInString(s[n:]); unicode.IsUpper(next) {
		return s
	}
	return string(unicode.ToLower(r)) + s[n:]
}

func (t *Terms) localDigits(s string) string {
	if t.digits == "" {
		return s
	}
	digits := []rune(t.digits)
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return digits[r-'0']
		}
		return r
	}, s)
}

// latinSerbian derives Latin-script Serbian from the Cyrillic strings. The
// standard transliteration is one-to-one, so deriving it keeps both scripts
// in step.
func latinSerbian(cyr *Terms) *Terms {
	t := cyr.clone()
	v := reflect.ValueOf(t).Elem()
	for i := range v.NumField() {
		f := v.Field(i)
		if !f.CanSet() {
			continue
		}
		switch f.Kind() {
		case reflect.String:
			f.SetString(serbianToLatin(f.String()))
		case reflect.Array:
			for j := range f.Len() {
				f.Index(j).SetString(serbianToLatin(f.Index(j).String()))
			}
		}
	}
	t.Tag, t.EnglishName, t.NativeName = "sr-Latn", "Serbian (Latin script)", "srpski (latinica)"
	t.Script, t.TypstLang, t.TypstRegion, t.LaTeXBabel = "Latn", "sr", "RS", "serbian-latin"
	return t
}

var serbianLatin = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'ђ': "đ", 'е': "e", 'ж': "ž", 'з': "z",
	'и': "i", 'ј': "j", 'к': "k", 'л': "l", 'љ': "lj", 'м': "m", 'н': "n", 'њ': "nj", 'о': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'ћ': "ć", 'у': "u", 'ф': "f", 'х': "h", 'ц': "c",
	'ч': "č", 'џ': "dž", 'ш': "š",
}

func serbianToLatin(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		lower := unicode.ToLower(r)
		lat, ok := serbianLatin[lower]
		switch {
		case !ok:
			b.WriteRune(r)
		case r == lower:
			b.WriteString(lat)
		case i+1 < len(rs) && unicode.IsUpper(rs[i+1]) || len(rs) == 1:
			b.WriteString(strings.ToUpper(lat)) // all-caps context: "ЉУБАВ" → "LJUBAV"
		default:
			first, n := utf8.DecodeRuneInString(lat)
			b.WriteString(string(unicode.ToUpper(first)) + lat[n:]) // "Љубав" → "Ljubav"
		}
	}
	return b.String()
}
