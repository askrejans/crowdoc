package locale

import (
	"strings"
	"testing"
	"time"
)

var oct8 = time.Date(2026, time.October, 8, 15, 4, 5, 0, time.UTC)

func TestFormatDateLong(t *testing.T) {
	tests := map[string]string{
		"en":      "8 October 2026",
		"en-GB":   "8 October 2026",
		"en-US":   "October 8, 2026",
		"lv":      "2026. gada 8. oktobris",
		"lt":      "2026 m. spalio 8 d.",
		"et":      "8. oktoober 2026",
		"fi":      "8. lokakuuta 2026",
		"sv":      "8 oktober 2026",
		"nb":      "8. oktober 2026",
		"da":      "8. oktober 2026",
		"is":      "8. október 2026",
		"de":      "8. Oktober 2026",
		"nl":      "8 oktober 2026",
		"lb":      "8. Oktober 2026",
		"fr":      "8 octobre 2026",
		"it":      "8 ottobre 2026",
		"es":      "8 de octubre de 2026",
		"pt":      "8 de outubro de 2026",
		"pl":      "8 października 2026",
		"cs":      "8. října 2026",
		"sk":      "8. októbra 2026",
		"sl":      "8. oktobra 2026",
		"hr":      "8. listopada 2026.",
		"cnr":     "8. oktobra 2026.",
		"sq":      "8 tetor 2026",
		"mk":      "8 октомври 2026",
		"bg":      "8 октомври 2026 г.",
		"el":      "8 Οκτωβρίου 2026",
		"ro":      "8 octombrie 2026",
		"hu":      "2026. október 8.",
		"tr":      "8 Ekim 2026",
		"uk":      "8 жовтня 2026 р.",
		"ru":      "8 октября 2026 г.",
		"sr":      "8. октобра 2026.",
		"sr-Latn": "8. oktobra 2026.",
		"ca":      "8 d’octubre de 2026",
		"eu":      "2026ko urriaren 8a",
		"ga":      "8 Deireadh Fómhair 2026",
		"ja":      "2026年10月8日",
		"zh":      "2026年10月8日",
		"zh-TW":   "2026年10月8日",
		"ko":      "2026년 10월 8일",
		"ar":      "٨ أكتوبر ٢٠٢٦",
		"ar-MA":   "8 أكتوبر 2026",
		"he":      "8 באוקטובר 2026",
	}
	for tag, want := range tests {
		if got := FormatDate(oct8, tag, true); got != want {
			t.Errorf("FormatDate(%s, long) = %q, want %q", tag, got, want)
		}
	}
}

func TestFormatDateShort(t *testing.T) {
	const nbsp = "\u00a0"
	tests := map[string]string{
		"en":    "08/10/2026",
		"en-US": "10/08/2026",
		"en-CA": "2026-10-08",
		"lv":    "08.10.2026.",
		"de":    "08.10.2026",
		"de-CH": "08.10.2026",
		"fr":    "08/10/2026",
		"fr-CA": "2026-10-08",
		"nl":    "08-10-2026",
		"sv":    "2026-10-08",
		"lt":    "2026-10-08",
		"fi":    "8.10.2026",
		"cs":    "8." + nbsp + "10." + nbsp + "2026",
		"hr":    "8." + nbsp + "10." + nbsp + "2026.",
		"hu":    "2026." + nbsp + "10." + nbsp + "08.",
		"bg":    "08.10.2026" + nbsp + "г.",
		"eu":    "2026/10/08",
		"ja":    "2026/10/08",
		"zh":    "2026-10-08",
		"zh-TW": "2026/10/08",
		"ko":    "2026." + nbsp + "10." + nbsp + "8.",
		"ar":    "٨\u200f/١٠\u200f/٢٠٢٦",
		"he":    "8.10.2026",
	}
	for tag, want := range tests {
		if got := FormatDate(oct8, tag, false); got != want {
			t.Errorf("FormatDate(%s, short) = %q, want %q", tag, got, want)
		}
	}
}

func TestFormatDateFirstOfMonth(t *testing.T) {
	jan1 := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	tests := map[string]string{
		"fr": "1er janvier 2025",
		"it": "1º gennaio 2025",
		"eu": "2025eko urtarrilaren 1a",
		"en": "1 January 2025",
		"he": "1 בינואר 2025",
	}
	for tag, want := range tests {
		if got := FormatDate(jan1, tag, true); got != want {
			t.Errorf("FormatDate(%s) = %q, want %q", tag, got, want)
		}
	}
	// Basque: days 11 and 31 already end in -a (hamaika).
	if got := FormatDate(time.Date(2031, 10, 31, 0, 0, 0, 0, time.UTC), "eu", true); got != "2031ko urriaren 31" {
		t.Errorf("Basque 31 October 2031 = %q", got)
	}
}

func TestFormatDateUsesLocation(t *testing.T) {
	// 23:30 UTC on 7 October is already 8 October in Riga.
	riga := time.FixedZone("EEST", 3*3600)
	at := time.Date(2026, 10, 7, 23, 30, 0, 0, time.UTC).In(riga)
	if got := FormatDate(at, "lv", true); got != "2026. gada 8. oktobris" {
		t.Errorf("got %q", got)
	}
}

func TestBasqueYearSuffix(t *testing.T) {
	tests := map[int]string{
		2000: "ko", 2001: "eko", 2002: "ko", 2005: "eko", 2010: "eko", 2011: "ko", 2015: "eko",
		2020: "ko", 2021: "eko", 2025: "eko", 2026: "ko", 2030: "eko", 2031: "ko", 2035: "eko",
		2040: "ko", 2050: "eko", 2051: "ko", 2070: "eko", 2090: "eko", 2100: "eko", 1900: "eko",
		1000: "ko", 3000: "ko", 1999: "ko",
	}
	for y, want := range tests {
		if got := basqueYearSuffix(y); got != want {
			t.Errorf("basqueYearSuffix(%d) = %q, want %q", y, got, want)
		}
	}
}

func TestFormatPageOf(t *testing.T) {
	tests := []struct {
		tag, want string
	}{
		{"en", "Page 3 of 12"},
		{"lv", "3. lappuse no 12"},
		{"de", "Seite 3 von 12"},
		{"fi", "3 (12)"},
		{"hu", "3/12. oldal"},
		{"zh", "第3页，共12页"},
		{"ar", "الصفحة ٣ من ١٢"},
		{"ar-TN", "الصفحة 3 من 12"},
		{"he", "עמוד 3 מתוך 12"},
		{"unknown", "Page 3 of 12"},
	}
	for _, tt := range tests {
		if got := FormatPageOf(3, 12, tt.tag); got != tt.want {
			t.Errorf("FormatPageOf(%s) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	day := func(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }
	accept := []struct {
		in   string
		want time.Time
	}{
		{"2026-10-08", day(2026, 10, 8)},
		{" 2026-10-08 ", day(2026, 10, 8)},
		{"2026-1-8", day(2026, 1, 8)},
		{"2026/10/08", day(2026, 10, 8)},
		{"2026. 10. 08.", day(2026, 10, 8)},
		{"2026.10.8", day(2026, 10, 8)},
		{"8.10.2026", day(2026, 10, 8)},
		{"08.10.2026.", day(2026, 10, 8)},
		{"8. 10. 2026", day(2026, 10, 8)},
		{"8.\u00a010.\u00a02026", day(2026, 10, 8)},
		{"25/10/2026", day(2026, 10, 25)},
		{"10/25/2026", day(2026, 10, 25)},
		{"10/10/2026", day(2026, 10, 10)},
		{"25-10-2026", day(2026, 10, 25)},
		{"October 8, 2026", day(2026, 10, 8)},
		{"october 8 2026", day(2026, 10, 8)},
		{"Oct. 8, 2026", day(2026, 10, 8)},
		{"Sept 30, 2026", day(2026, 9, 30)},
		{"8 October 2026", day(2026, 10, 8)},
		{"8th October 2026", day(2026, 10, 8)},
		{"the 1st of May 2026"[4:], day(2026, 5, 1)},
		{"Thursday, October 8, 2026", day(2026, 10, 8)},
		{"Thu, 8 Oct 2026", day(2026, 10, 8)},
		{"2026-10-08T14:30:00Z", time.Date(2026, 10, 8, 14, 30, 0, 0, time.UTC)},
		{"2026-10-08T14:30:00.5Z", time.Date(2026, 10, 8, 14, 30, 0, 500000000, time.UTC)},
		{"2026-10-08T14:30", time.Date(2026, 10, 8, 14, 30, 0, 0, time.UTC)},
		{"2026-10-08 14:30:00", time.Date(2026, 10, 8, 14, 30, 0, 0, time.UTC)},
	}
	for _, tt := range accept {
		got, ok := ParseDate(tt.in)
		if !ok || !got.Equal(tt.want) {
			t.Errorf("ParseDate(%q) = %v, %v; want %v", tt.in, got, ok, tt.want)
		}
	}
	withZone, ok := ParseDate("2026-10-08T23:30:00+03:00")
	if !ok || FormatDate(withZone, "en", true) != "8 October 2026" {
		t.Errorf("offset date-time: %v %v", withZone, ok)
	}

	reject := []string{
		"", "   ", "2026", "2026-10", "10/08/2026", "08-10-2026", "8.10.26", "26.10.8",
		"31.02.2026", "2026-02-30", "32.10.2026", "8.13.2026", "October 2026", "Octember 8, 2026",
		"8 October", "8 October 26", "8.10.2026 12:00", "yesterday", "8/10/2026/1", "8-10/2026",
		"8..10.2026", "10.2026", "1.2.3.4", strings.Repeat("9", 100), "0.0.2026", "00/00/0000",
		"8 de octubre de 2026", "8. Oktober 2026",
	}
	for _, in := range reject {
		if got, ok := ParseDate(in); ok {
			t.Errorf("ParseDate(%q) = %v, want rejection", in, got)
		}
	}
}

func FuzzParseDate(f *testing.F) {
	for _, s := range []string{"2026-10-08", "8.10.2026", "October 8, 2026", "1/2/3", "2026. 10. 08."} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if got, ok := ParseDate(s); ok && (got.Year() < 1 || got.Year() > 9999) {
			t.Fatalf("ParseDate(%q) = %v", s, got)
		}
	})
}
