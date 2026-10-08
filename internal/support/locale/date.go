package locale

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date patterns use these tokens; everything else is literal text:
//
//	{d} {dd}      day, without / with leading zero
//	{M} {MM}      month number, without / with leading zero
//	{yyyy}        year
//	{month}       nominative month name
//	{gen}         month name as inflected inside dates (MonthsGenitive)
//	{mon}         abbreviated month name
//	{d.fr}        French day: "1er" on the first of the month
//	{d.it}        Italian day: "1º" on the first of the month
//	{d.eu}        Basque day with the absolutive article: "8a", but "11", "31"
//	{yyyy.eu}     Basque year with its -ko/-eko suffix: "2026ko", "2025eko"

// regionDates overrides the long and short patterns for regions whose
// conventions differ from the language default.
var regionDates = map[string][2]string{
	"en-US": {"{month} {d}, {yyyy}", "{MM}/{dd}/{yyyy}"},
	"en-PH": {"{month} {d}, {yyyy}", "{MM}/{dd}/{yyyy}"},
	"en-CA": {"{month} {d}, {yyyy}", "{yyyy}-{MM}-{dd}"},
	"fr-CA": {"", "{yyyy}-{MM}-{dd}"},
	"fr-CH": {"", "{dd}.{MM}.{yyyy}"},
	"it-CH": {"", "{dd}.{MM}.{yyyy}"},
	"de-CH": {"", "{dd}.{MM}.{yyyy}"},
	"pt-BR": {"", "{dd}/{MM}/{yyyy}"},
}

// latinDigitArabic lists the regions where Arabic is written with Latin
// digits (the Maghreb); elsewhere formal documents use Arabic-Indic digits.
var latinDigitArabic = map[string]bool{"MA": true, "DZ": true, "TN": true, "LY": true, "EH": true, "MR": true}

// FormatDate formats the calendar date of t (in t's location) the way the
// language writes it in running text and document headers (long) or as a
// compact numeric date (short):
//
//	en    8 October 2026        08/10/2026
//	en-US October 8, 2026       10/08/2026
//	lv    2026. gada 8. oktobris 08.10.2026.
//	de    8. Oktober 2026       08.10.2026
//	lt    2026 m. spalio 8 d.   2026-10-08
//	hu    2026. október 8.      2026. 10. 08.
//	ja    2026年10月8日          2026/10/08
//	ar    ٨ أكتوبر ٢٠٢٦          ٨/١٠/٢٠٢٦ (with U+200F after day and month)
//
// Arabic uses Arabic-Indic digits, as formal documents in the Mashriq and
// the Gulf do, and Latin digits for the Maghreb regions (ar-MA, ar-DZ,
// ar-TN, ar-LY, ar-EH, ar-MR). Numeric dates that contain spaces use
// no-break spaces so that they never split across lines.
func FormatDate(t time.Time, tag string, long bool) string {
	terms := Get(tag)
	pattern := terms.shortDate
	if long {
		pattern = terms.longDate
	}
	if base, _, region, ok := parseTag(tag); ok {
		if o, ok := regionDates[base+"-"+region]; ok {
			if long && o[0] != "" {
				pattern = o[0]
			} else if !long && o[1] != "" {
				pattern = o[1]
			}
		}
	}
	return terms.localDigits(formatPattern(pattern, t, terms))
}

// FormatPageOf formats "page n of total" for running footers, e.g.
// "Page 3 of 12", "3. lappuse no 12", "3 (12)" in Finnish, with the same
// digits as FormatDate.
func FormatPageOf(page, total int, tag string) string {
	t := Get(tag)
	return t.localDigits(fmt.Sprintf(t.PageOfFmt, page, total))
}

func formatPattern(p string, t time.Time, terms *Terms) string {
	y, m, d := t.Date()
	var b strings.Builder
	for p != "" {
		i := strings.IndexByte(p, '{')
		if i < 0 {
			b.WriteString(p)
			break
		}
		b.WriteString(p[:i])
		j := strings.IndexByte(p[i:], '}')
		if j < 0 {
			b.WriteString(p[i:])
			break
		}
		b.WriteString(dateToken(p[i+1:i+j], y, m, d, terms))
		p = p[i+j+1:]
	}
	return b.String()
}

func dateToken(tok string, y int, m time.Month, d int, terms *Terms) string {
	switch tok {
	case "d":
		return strconv.Itoa(d)
	case "dd":
		return fmt.Sprintf("%02d", d)
	case "M":
		return strconv.Itoa(int(m))
	case "MM":
		return fmt.Sprintf("%02d", int(m))
	case "yyyy":
		return strconv.Itoa(y)
	case "month":
		return terms.Months[m-1]
	case "gen":
		return terms.MonthsGenitive[m-1]
	case "mon":
		return terms.MonthsShort[m-1]
	case "d.fr":
		if d == 1 {
			return "1er"
		}
		return strconv.Itoa(d)
	case "d.it":
		if d == 1 {
			return "1º"
		}
		return strconv.Itoa(d)
	case "d.eu":
		if d == 11 || d == 31 { // hamaika, hogeita hamaika already end in -a
			return strconv.Itoa(d)
		}
		return strconv.Itoa(d) + "a"
	case "yyyy.eu":
		return strconv.Itoa(y) + basqueYearSuffix(y)
	}
	return "{" + tok + "}"
}

// basqueYearSuffix returns the -ko/-eko ending a Basque year takes. The
// epenthetic e appears after numerals that end in a consonant when spoken:
// bat (1), bost (5), hamar (10), hamabost (15) and ehun (hundreds), counted
// in the vigesimal system (30 = hogeita hamar, 31 = hogeita hamaika).
func basqueYearSuffix(y int) string {
	if y < 0 {
		y = -y
	}
	r := y % 100
	if r == 0 {
		if y%1000 == 0 {
			return "ko" // mila
		}
		return "eko" // …ehun
	}
	switch r % 20 {
	case 1, 5, 10, 15:
		return "eko"
	}
	return "ko"
}

var isoLayouts = []string{
	"2006-01-02",
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04",
	"2006-01-02T15:04Z07:00",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04",
	"2006-01-02 15:04Z07:00",
	"2006-01-02 15:04:05 -0700",
}

// ParseDate recognises the date formats commonly found in document
// metadata: ISO 8601 dates and date-times ("2026-10-08",
// "2026-10-08T14:30:00+03:00"), numeric day-month-year dates with dots
// ("8.10.2026", "08.10.2026.", "8. 10. 2026"), year-first numeric dates
// ("2026/10/08", "2026. 10. 08."), slash or dash dates whose order is
// unambiguous ("25/10/2026"), and English dates ("October 8, 2026",
// "8 October 2026", "Thu, 8th Oct 2026"). Date-only values are returned at
// midnight UTC. It returns false for anything ambiguous, such as
// "10/08/2026", two-digit years, or invalid dates.
func ParseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return time.Time{}, false
	}
	for _, layout := range isoLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			if t.Year() < 1 {
				return time.Time{}, false
			}
			return t, true
		}
	}
	if t, ok := parseNumericDate(s); ok {
		return t, true
	}
	return parseEnglishDate(s)
}

func makeDate(y, m, d int) (time.Time, bool) {
	if y < 1 || y > 9999 || m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, false
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.Day() != d { // 31 February and the like
		return time.Time{}, false
	}
	return t, true
}

// parseNumericDate handles three digit groups separated by one '.', '-' or
// '/' each (optionally followed by spaces), with an optional trailing dot
// after dotted dates. Day-first is certain only with dots; with slashes or
// dashes the order must follow from the values.
func parseNumericDate(s string) (time.Time, bool) {
	var groups []string
	var seps []byte // the separator after each group, 0 at the end
	for i := 0; i < len(s); {
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i || j-i > 4 {
			return time.Time{}, false
		}
		groups = append(groups, s[i:j])
		if j == len(s) {
			seps = append(seps, 0)
			break
		}
		if c := s[j]; c == '.' || c == '-' || c == '/' {
			seps = append(seps, c)
		} else {
			return time.Time{}, false
		}
		i = j + 1
		for i < len(s) {
			if s[i] == ' ' {
				i++
			} else if strings.HasPrefix(s[i:], "\u00a0") {
				i += len("\u00a0")
			} else {
				break
			}
		}
		if i == len(s) {
			seps = append(seps, 0)
		}
	}
	if len(groups) != 3 || seps[0] != seps[1] || (seps[2] != 0 && !(seps[2] == '.' && seps[0] == '.')) {
		return time.Time{}, false
	}
	var n [3]int
	for k, g := range groups {
		v, err := strconv.Atoi(g)
		if err != nil {
			return time.Time{}, false
		}
		n[k] = v
	}
	switch {
	case len(groups[0]) == 4:
		return makeDate(n[0], n[1], n[2])
	case len(groups[2]) != 4:
		return time.Time{}, false // two-digit or odd year: unsure
	case seps[0] == '.':
		return makeDate(n[2], n[1], n[0]) // dotted dates are always day first
	case n[0] == n[1] || n[0] > 12:
		return makeDate(n[2], n[1], n[0])
	case n[1] > 12:
		return makeDate(n[2], n[0], n[1])
	}
	return time.Time{}, false // "10/08/2026": day-month or month-day
}

var englishMonths = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6, "july": 7,
	"august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "jun": 6, "jul": 7, "aug": 8, "sep": 9,
	"sept": 9, "oct": 10, "nov": 11, "dec": 12,
}

var englishWeekdays = map[string]bool{
	"monday": true, "tuesday": true, "wednesday": true, "thursday": true, "friday": true,
	"saturday": true, "sunday": true, "mon": true, "tue": true, "tues": true, "wed": true,
	"thu": true, "thur": true, "thurs": true, "fri": true, "sat": true, "sun": true,
}

// parseEnglishDate accepts "October 8, 2026", "Oct. 8 2026", "8 October
// 2026", "8th of October, 2026", optionally preceded by a weekday.
func parseEnglishDate(s string) (time.Time, bool) {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == ',' || r == '\u00a0'
	})
	var toks []string
	for i, f := range fields {
		f = strings.TrimSuffix(f, ".")
		if i == 0 && englishWeekdays[f] || f == "of" || f == "" {
			continue
		}
		toks = append(toks, f)
	}
	if len(toks) != 3 {
		return time.Time{}, false
	}
	day := func(f string) (int, bool) {
		for _, suf := range []string{"st", "nd", "rd", "th"} {
			f = strings.TrimSuffix(f, suf)
		}
		if len(f) == 0 || len(f) > 2 {
			return 0, false
		}
		n, err := strconv.Atoi(f)
		return n, err == nil
	}
	year := func(f string) (int, bool) {
		if len(f) != 4 {
			return 0, false
		}
		n, err := strconv.Atoi(f)
		return n, err == nil
	}
	y, ok := year(toks[2])
	if !ok {
		return time.Time{}, false
	}
	if m, isMonth := englishMonths[toks[0]]; isMonth {
		if d, ok := day(toks[1]); ok {
			return makeDate(y, m, d)
		}
	}
	if m, isMonth := englishMonths[toks[1]]; isMonth {
		if d, ok := day(toks[0]); ok {
			return makeDate(y, m, d)
		}
	}
	return time.Time{}, false
}
