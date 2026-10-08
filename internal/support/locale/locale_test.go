package locale

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode"
)

// everyTerms returns the 40 supported languages plus the script variants.
func everyTerms() []*Terms {
	return append(Supported(), Get("sr-Latn"), Get("zh-Hant"))
}

// stringFields visits every exported string field and month name.
func stringFields(t *Terms, visit func(name, value string)) {
	v := reflect.ValueOf(t).Elem()
	typ := v.Type()
	for i := range v.NumField() {
		f, sf := v.Field(i), typ.Field(i)
		if !sf.IsExported() {
			continue
		}
		switch f.Kind() {
		case reflect.String:
			visit(sf.Name, f.String())
		case reflect.Array:
			for j := range f.Len() {
				visit(fmt.Sprintf("%s[%d]", sf.Name, j), f.Index(j).String())
			}
		}
	}
}

// optional fields may legitimately be empty.
var optional = map[string]bool{"TypstRegion": true}

// spaced fields carry deliberate leading or trailing spacing.
var spaced = map[string]bool{"CaptionSep": true, "QuoteOpen": true, "QuoteClose": true,
	"InnerQuoteOpen": true, "InnerQuoteClose": true, "ThousandsSep": true}

func TestEveryFieldFilled(t *testing.T) {
	for _, terms := range everyTerms() {
		stringFields(terms, func(name, value string) {
			if value == "" && !optional[name] {
				t.Errorf("%s: %s is empty", terms.Tag, name)
			}
			if !spaced[name] && value != strings.TrimSpace(value) {
				t.Errorf("%s: %s has surrounding whitespace: %q", terms.Tag, name, value)
			}
			if strings.Contains(value, "  ") {
				t.Errorf("%s: %s has a double space: %q", terms.Tag, name, value)
			}
		})
		if terms.longDate == "" || terms.shortDate == "" {
			t.Errorf("%s: date patterns missing", terms.Tag)
		}
		for _, months := range [][12]string{terms.Months, terms.MonthsShort, terms.MonthsGenitive} {
			seen := map[string]bool{}
			for _, m := range months {
				if seen[m] {
					t.Errorf("%s: duplicate month name %q in %v", terms.Tag, m, months)
				}
				seen[m] = true
			}
		}
		if got := fmt.Sprintf(terms.PageOfFmt, 7, 42); strings.Contains(got, "%!") ||
			!strings.Contains(got, "7") || !strings.Contains(got, "42") {
			t.Errorf("%s: PageOfFmt %q formats as %q", terms.Tag, terms.PageOfFmt, got)
		}
	}
}

// scriptTables maps ISO 15924 codes to the Unicode scripts their letters
// may come from.
var scriptTables = map[string][]*unicode.RangeTable{
	"Latn": {unicode.Latin},
	"Cyrl": {unicode.Cyrillic},
	"Grek": {unicode.Greek},
	"Arab": {unicode.Arabic},
	"Hebr": {unicode.Hebrew},
	"Jpan": {unicode.Han, unicode.Hiragana, unicode.Katakana},
	"Hans": {unicode.Han},
	"Hant": {unicode.Han},
	"Kore": {unicode.Hangul, unicode.Han},
}

// TestLettersMatchScript guards against look-alike letters from another
// script (a Latin "o" inside a Cyrillic word, say) and wrong-script data.
func TestLettersMatchScript(t *testing.T) {
	identity := map[string]bool{"Tag": true, "EnglishName": true, "Script": true,
		"TypstLang": true, "TypstRegion": true, "LaTeXBabel": true}
	for _, terms := range everyTerms() {
		tables, ok := scriptTables[terms.Script]
		if !ok {
			t.Errorf("%s: unknown script %q", terms.Tag, terms.Script)
			continue
		}
		tables = append(tables, unicode.Common, unicode.Inherited)
		stringFields(terms, func(name, value string) {
			if identity[name] {
				return
			}
			for _, r := range strings.ReplaceAll(value, "%d", "") {
				if unicode.IsLetter(r) && !unicode.In(r, tables...) {
					t.Errorf("%s: %s = %q contains %q (U+%04X) outside %s", terms.Tag, name, value, r, r, terms.Script)
					return
				}
			}
		})
	}
}

func TestSupportedOrder(t *testing.T) {
	want := []string{"en", "lv", "lt", "et", "fi", "sv", "nb", "da", "is", "de", "nl", "lb", "fr", "it",
		"es", "pt", "pl", "cs", "sk", "sl", "hr", "cnr", "sq", "mk", "bg", "el", "ro", "hu", "tr", "uk",
		"ru", "sr", "ca", "eu", "ga", "ja", "zh", "ko", "ar", "he"}
	var got []string
	for _, terms := range Supported() {
		got = append(got, terms.Tag)
		if terms.TypstLang != terms.Tag {
			t.Errorf("%s: TypstLang %q differs from the tag", terms.Tag, terms.TypstLang)
		}
		if terms.RTL != (terms.Tag == "ar" || terms.Tag == "he") {
			t.Errorf("%s: RTL = %v", terms.Tag, terms.RTL)
		}
		if terms.LaTeXBabel == "" {
			t.Errorf("%s: no babel name", terms.Tag)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("Supported() order\n got %v\nwant %v", got, want)
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		tag, want string
		exact     bool
	}{
		{"lv", "lv", true},
		{"lv-LV", "lv", true},
		{"LV", "lv", true},
		{"lv_LV.UTF-8", "lv", true},
		{" lv ", "lv", true},
		{"pt-BR", "pt", true},
		{"de-AT", "de", true},
		{"en-US", "en", true},
		{"deu", "de", true},
		{"iw", "he", true},
		{"mo", "ro", true},
		{"zh", "zh", true},
		{"zh-CN", "zh", true},
		{"zh-Hans", "zh", true},
		{"zh-SG", "zh", true},
		{"zh-TW", "zh-Hant", true},
		{"zh-HK", "zh-Hant", true},
		{"zh-Hant", "zh-Hant", true},
		{"zh-Hant-CN", "zh-Hant", true},
		{"sr", "sr", true},
		{"sr-RS", "sr", true},
		{"sr-Cyrl", "sr", true},
		{"sr-Latn", "sr-Latn", true},
		{"sr-latn-rs", "sr-Latn", true},
		{"sr-ME", "sr-Latn", true},
		{"sh", "sr-Latn", true},
		{"cnr", "cnr", true},
		{"cnr-ME", "cnr", true},
		{"no", "nb", true},
		{"nb-NO", "nb", true},
		{"nn", "nb", false},
		{"ja-Jpan", "ja", true},
		{"ko-Hang", "ko", true},
		{"ru-Latn", "ru", false},
		{"xx", "en", false},
		{"fy", "en", false},
		{"", "en", false},
		{"und", "en", false},
		{"!!not a tag", "en", false},
	}
	for _, tt := range tests {
		got, exact := Match(tt.tag)
		if got == nil || got.Tag != tt.want || exact != tt.exact {
			t.Errorf("Match(%q) = %v, %v; want %s, %v", tt.tag, got.Tag, exact, tt.want, tt.exact)
		}
		if g := Get(tt.tag); g == nil || g.Tag != tt.want {
			t.Errorf("Get(%q) = %v, want %s", tt.tag, g, tt.want)
		}
	}
}

func TestGetReturnsCopies(t *testing.T) {
	a := Get("lv")
	a.Contents = "changed"
	a.Months[0] = "changed"
	if b := Get("lv"); b.Contents != "Saturs" || b.Months[0] != "janvāris" {
		t.Fatal("modifying a returned Terms changed the package data")
	}
	s := Supported()
	s[0].Contents = "changed"
	if Supported()[0].Contents != "Contents" {
		t.Fatal("modifying Supported() changed the package data")
	}
}

func TestSpotChecks(t *testing.T) {
	tests := []struct{ tag, field, want string }{
		{"lv", "Contents", "Saturs"},
		{"lv", "Figure", "Attēls"},
		{"lv", "Table", "Tabula"},
		{"lv", "References", "Literatūra"},
		{"lv", "Bibliography", "Izmantotie avoti"},
		{"lv", "Appendix", "Pielikums"},
		{"lv", "QuoteOpen", "„"},
		{"lv", "QuoteClose", "“"},
		{"de", "Contents", "Inhaltsverzeichnis"},
		{"de", "ListOfFigures", "Abbildungsverzeichnis"},
		{"de", "QuoteOpen", "„"},
		{"de", "QuoteClose", "“"},
		{"fr", "QuoteOpen", "«\u202f"},
		{"fr", "QuoteClose", "\u202f»"},
		{"fr", "Contents", "Table des matières"},
		{"fr", "ThousandsSep", "\u202f"},
		{"ru", "QuoteOpen", "«"},
		{"ru", "InnerQuoteOpen", "„"},
		{"ru", "Figure", "Рисунок"},
		{"ru", "CaptionSep", " — "},
		{"pl", "QuoteClose", "”"},
		{"pl", "Contents", "Spis treści"},
		{"cs", "Subject", "Věc"},
		{"hu", "Contents", "Tartalomjegyzék"},
		{"ja", "QuoteOpen", "「"},
		{"ja", "Contents", "目次"},
		{"zh", "Contents", "目录"},
		{"zh-TW", "Contents", "目錄"},
		{"zh-TW", "QuoteOpen", "「"},
		{"ko", "Contents", "목차"},
		{"el", "Contents", "Περιεχόμενα"},
		{"ar", "Contents", "المحتويات"},
		{"he", "Contents", "תוכן העניינים"},
		{"ro", "Contents", "Cuprins"},
		{"sr-Latn", "Contents", "Sadržaj"},
		{"sr-Latn", "Chapter", "Poglavlje"},
		{"sr-Latn", "Section", "Odeljak"},
		{"sr-Latn", "Proposition", "Tvrđenje"},
		{"fi", "PageOfFmt", "%d (%d)"},
		{"en", "MastersThesis", "Master’s Thesis"},
	}
	for _, tt := range tests {
		v := reflect.ValueOf(Get(tt.tag)).Elem().FieldByName(tt.field)
		if !v.IsValid() {
			t.Fatalf("no field %s", tt.field)
		}
		if got := v.String(); got != tt.want {
			t.Errorf("%s.%s = %q, want %q", tt.tag, tt.field, got, tt.want)
		}
	}
	months := []struct{ tag, nom, gen string }{
		{"pl", "październik", "października"},
		{"cs", "říjen", "října"},
		{"ru", "октябрь", "октября"},
		{"uk", "жовтень", "жовтня"},
		{"lt", "spalis", "spalio"},
		{"el", "Οκτώβριος", "Οκτωβρίου"},
		{"fi", "lokakuu", "lokakuuta"},
		{"hr", "listopad", "listopada"},
		{"ca", "octubre", "d’octubre"},
		{"he", "אוקטובר", "באוקטובר"},
		{"lv", "oktobris", "oktobris"},
		{"de", "Oktober", "Oktober"},
	}
	for _, m := range months {
		terms := Get(m.tag)
		if terms.Months[9] != m.nom || terms.MonthsGenitive[9] != m.gen {
			t.Errorf("%s October = %q/%q, want %q/%q", m.tag, terms.Months[9], terms.MonthsGenitive[9], m.nom, m.gen)
		}
	}
}

func TestLabel(t *testing.T) {
	const nbsp = "\u00a0"
	tests := []struct {
		tag, term, number, want string
	}{
		{"en", "Figure", "3", "Figure" + nbsp + "3"},
		{"lv", "Figure", "3", "3." + nbsp + "attēls"},
		{"lv", "Figure", "2.4", "2.4." + nbsp + "attēls"},
		{"lv", "Appendix", "A", "A" + nbsp + "pielikums"},
		{"lv", "Chapter", "1", "1." + nbsp + "nodaļa"},
		{"lt", "Figure", "3", "3" + nbsp + "pav."},
		{"lt", "Table", "3", "3" + nbsp + "lentelė"},
		{"hu", "Figure", "3", "3." + nbsp + "ábra"},
		{"eu", "Table", "3", "3." + nbsp + "taula"},
		{"de", "Figure", "3", "Abbildung" + nbsp + "3"},
		{"ja", "Figure", "3", "図3"},
		{"ja", "Chapter", "3", "第3章"},
		{"zh", "Section", "2", "第2节"},
		{"zh-TW", "Figure", "1", "圖1"},
		{"ko", "Figure", "3", "그림" + nbsp + "3"},
		{"ko", "Chapter", "3", "제3장"},
		{"ar", "Figure", "12", "الشكل" + nbsp + "١٢"},
		{"ar-MA", "Figure", "12", "الشكل" + nbsp + "12"},
	}
	for _, tt := range tests {
		terms := Get(tt.tag)
		term := reflect.ValueOf(terms).Elem().FieldByName(tt.term).String()
		if got := terms.Label(term, tt.number); got != tt.want {
			t.Errorf("%s Label(%s, %s) = %q, want %q", tt.tag, tt.term, tt.number, got, tt.want)
		}
	}
	prefixes := []struct{ tag, term, want string }{
		{"en", "Figure", "Figure" + nbsp + "1: "},
		{"lv", "Table", "1." + nbsp + "tabula. "},
		{"lt", "Figure", "1" + nbsp + "pav. "}, // no doubled full stop after the abbreviation
		{"ru", "Figure", "Рисунок" + nbsp + "1 — "},
		{"fr", "Table", "Tableau" + nbsp + "1 – "},
		{"ja", "Table", "表1\u3000"},
	}
	for _, p := range prefixes {
		terms := Get(p.tag)
		term := reflect.ValueOf(terms).Elem().FieldByName(p.term).String()
		if got := terms.CaptionPrefix(term, "1"); got != p.want {
			t.Errorf("%s CaptionPrefix = %q, want %q", p.tag, got, p.want)
		}
	}
	for tag, want := range map[string]bool{"lv": true, "lt": true, "hu": true, "eu": true, "en": false, "ja": false, "et": false} {
		if got := Get(tag).NumberFirst(); got != want {
			t.Errorf("%s NumberFirst = %v", tag, got)
		}
	}
}

func TestSerbianTransliteration(t *testing.T) {
	tests := map[string]string{
		"Љубав":            "Ljubav",
		"ЉУБАВ":            "LJUBAV",
		"Њ":                "NJ",
		"Џеп":              "Džep",
		"ђак, ћерка, жена": "đak, ćerka, žena",
		"Страна %d од %d":  "Strana %d od %d",
		"„цитат“":          "„citat“",
	}
	for in, want := range tests {
		if got := serbianToLatin(in); got != want {
			t.Errorf("serbianToLatin(%q) = %q, want %q", in, got, want)
		}
	}
	lat := Get("sr-Latn")
	if lat.Script != "Latn" || lat.LaTeXBabel != "serbian-latin" || lat.TypstLang != "sr" {
		t.Errorf("sr-Latn identity: %+v", []string{lat.Script, lat.LaTeXBabel, lat.TypstLang})
	}
	if lat.MonthsGenitive[9] != "oktobra" || lat.Months[11] != "decembar" {
		t.Errorf("sr-Latn months: %v", lat.MonthsGenitive)
	}
}
