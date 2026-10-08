package table

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// builtinFormats are the format codes of the built-in number format ids
// that are not locale dependent (or represent their locale variants).
var builtinFormats = map[int]string{
	0: "General", 1: "0", 2: "0.00", 3: "#,##0", 4: "#,##0.00",
	5: "#,##0;-#,##0", 6: "#,##0;-#,##0", 7: "#,##0.00;-#,##0.00", 8: "#,##0.00;-#,##0.00",
	9: "0%", 10: "0.00%", 11: "0.00E+00", 12: "# ?/?", 13: "# ??/??",
	14: "yyyy-mm-dd", 15: "d-mmm-yy", 16: "d-mmm", 17: "mmm-yy", 18: "h:mm AM/PM",
	19: "h:mm:ss AM/PM", 20: "h:mm", 21: "h:mm:ss", 22: "yyyy-mm-dd h:mm",
	27: "yyyy-mm-dd", 28: "yyyy-mm-dd", 29: "yyyy-mm-dd", 30: "yyyy-mm-dd", 31: "yyyy-mm-dd",
	32: "h:mm", 33: "h:mm:ss", 34: "yyyy-mm-dd", 35: "yyyy-mm-dd", 36: "yyyy-mm-dd",
	37: "#,##0 ;(#,##0)", 38: "#,##0 ;(#,##0)", 39: "#,##0.00;(#,##0.00)", 40: "#,##0.00;(#,##0.00)",
	41: "#,##0", 42: "#,##0", 43: "#,##0.00", 44: "#,##0.00",
	45: "mm:ss", 46: "[h]:mm:ss", 47: "mm:ss.0", 48: "##0.0E+0", 49: "@",
	50: "yyyy-mm-dd", 51: "yyyy-mm-dd", 52: "yyyy-mm-dd", 53: "yyyy-mm-dd", 54: "yyyy-mm-dd",
	55: "yyyy-mm-dd", 56: "yyyy-mm-dd", 57: "yyyy-mm-dd", 58: "yyyy-mm-dd",
}

// dateParts reports which date/time tokens a format code uses, ignoring
// quoted literals, escapes and bracketed modifiers.
type dateParts struct {
	date, time, hours, seconds, duration bool
}

func classifyFormat(code string) dateParts {
	var p dateParts
	prevTime := false
	for i := 0; i < len(code); i++ {
		c := code[i]
		switch c {
		case '"':
			if j := strings.IndexByte(code[i+1:], '"'); j >= 0 {
				i += j + 1
			} else {
				i = len(code)
			}
			continue
		case '\\', '_', '*':
			i++
			continue
		case '[':
			j := strings.IndexByte(code[i:], ']')
			if j < 0 {
				return p
			}
			switch strings.ToLower(strings.Trim(code[i+1:i+j], "]")) {
			case "h", "hh":
				p.duration, p.time, p.hours, prevTime = true, true, true, true
			case "m", "mm", "s", "ss":
				p.duration, p.time, prevTime = true, true, true
			}
			i += j
			continue
		case ';':
			return p // the first section decides
		}
		switch c | 0x20 {
		case 'y', 'd':
			p.date = true
			prevTime = false
		case 'h':
			p.time, p.hours = true, true
			prevTime = true
		case 's':
			p.time, p.seconds = true, true
			prevTime = true
		case 'm':
			// "m" after an hour or before a second is minutes, else month.
			rest := strings.TrimLeft(code[i:], "mM")
			if prevTime || strings.HasPrefix(strings.TrimLeft(rest, ":"), "s") || strings.HasPrefix(strings.TrimLeft(rest, ":"), "S") {
				p.time = true
			} else {
				p.date = true
			}
			i += len(code[i:]) - len(rest) - 1
		case 'a', 'p':
			if strings.HasPrefix(strings.ToUpper(code[i:]), "AM/PM") || strings.HasPrefix(strings.ToUpper(code[i:]), "A/P") {
				p.time = true
			}
		}
	}
	return p
}

func (p dateParts) isDate() bool { return p.date || p.time }

// formatNumber renders v with a spreadsheet number format code.
func formatNumber(v float64, code string, date1904 bool) (string, cellKind) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return generalNumber(v), kNumber
	}
	if p := classifyFormat(code); p.isDate() {
		if s, ok := formatDate(v, p, date1904); ok {
			return s, kDate
		}
	}
	sections := splitSections(code)
	sec := sections[0]
	neg := v < 0
	switch {
	case v < 0 && len(sections) >= 2:
		sec, v, neg = sections[1], -v, false
	case v == 0 && len(sections) >= 3:
		sec = sections[2]
	}
	return formatSection(v, neg, sec), kNumber
}

// splitSections splits a format code on ';' outside quotes and brackets.
func splitSections(code string) []string {
	var out []string
	start, quoted, bracket := 0, false, false
	for i := 0; i < len(code); i++ {
		switch c := code[i]; {
		case c == '\\' && !quoted:
			i++
		case c == '"':
			quoted = !quoted
		case c == '[' && !quoted:
			bracket = true
		case c == ']' && !quoted:
			bracket = false
		case c == ';' && !quoted && !bracket:
			out = append(out, code[start:i])
			start = i + 1
		}
	}
	return append(out, code[start:])
}

// numPattern is a parsed number section.
type numPattern struct {
	prefix, suffix strings.Builder
	intZeros       int // minimum integer digits ('0' placeholders)
	decimals       int // decimal places shown at most
	minDecimals    int // decimal places always shown ('0' placeholders)
	group          bool
	percent        int
	scale          int // trailing commas divide by 1000 each
	sci            bool
	expDigits      int
	placeholders   bool
	fraction       bool
}

func parsePattern(sec string) *numPattern {
	p := &numPattern{}
	inDecimals, seenDigit, afterDigits := false, false, false
	lit := func(s string) {
		if seenDigit {
			p.suffix.WriteString(s)
		} else {
			p.prefix.WriteString(s)
		}
	}
	for i := 0; i < len(sec); i++ {
		c := sec[i]
		switch c {
		case '"':
			j := strings.IndexByte(sec[i+1:], '"')
			if j < 0 {
				j = len(sec) - i - 1
			}
			lit(sec[i+1 : i+1+j])
			i += j + 1
		case '\\':
			if i+1 < len(sec) {
				lit(sec[i+1 : i+2])
				i++
			}
		case '_', '*':
			i++ // padding and fill characters
		case '[':
			j := strings.IndexByte(sec[i:], ']')
			if j < 0 {
				i = len(sec)
				continue
			}
			inner := sec[i+1 : i+j]
			if strings.HasPrefix(inner, "$") {
				sym, _, _ := strings.Cut(inner[1:], "-")
				lit(sym)
			}
			i += j
		case '0', '#', '?':
			p.placeholders = true
			if afterDigits && !inDecimals {
				// digits after a literal (e.g. a fraction) are not supported
				continue
			}
			seenDigit = true
			if inDecimals {
				p.decimals++
				if c == '0' {
					p.minDecimals++
				}
			} else if c == '0' {
				p.intZeros++
			}
		case '.':
			if seenDigit || i+1 < len(sec) && strings.ContainsRune("0#?", rune(sec[i+1])) {
				inDecimals, seenDigit = true, true
			} else {
				lit(".")
			}
		case ',':
			if !seenDigit {
				lit(",")
			} else if i+1 < len(sec) && strings.ContainsRune("0#?", rune(sec[i+1])) {
				p.group = true
			} else if !inDecimals {
				p.scale++
			}
		case '%':
			p.percent++
			lit("%")
		case 'E', 'e':
			if seenDigit && i+1 < len(sec) && (sec[i+1] == '+' || sec[i+1] == '-') {
				p.sci = true
				i++
				for i+1 < len(sec) && strings.ContainsRune("0#?", rune(sec[i+1])) {
					p.expDigits++
					i++
				}
				afterDigits = true
				continue
			}
			lit(string(c))
		case '/':
			p.fraction = true
		case '@':
		default:
			lit(string(c))
			if seenDigit {
				afterDigits = true
			}
		}
	}
	return p
}

func formatSection(v float64, neg bool, sec string) string {
	trimmed := strings.TrimSpace(sec)
	if trimmed == "" || strings.EqualFold(trimmed, "General") || trimmed == "@" {
		return generalNumber(sign(v, neg))
	}
	p := parsePattern(sec)
	if !p.placeholders || p.fraction {
		if !p.placeholders && !p.fraction {
			// A literal-only section (e.g. "-" for zero) shows the literal.
			if s := p.prefix.String() + p.suffix.String(); s != "" {
				return s
			}
		}
		return generalNumber(sign(v, neg))
	}
	a := math.Abs(v)
	for range p.percent {
		a *= 100
	}
	for range p.scale {
		a /= 1000
	}
	var num string
	if p.sci {
		num = strconv.FormatFloat(a, 'E', p.decimals, 64)
		mant, exp, _ := strings.Cut(num, "E")
		sgn := exp[0]
		exp = strings.TrimLeft(exp[1:], "0")
		for len(exp) < max(p.expDigits, 1) {
			exp = "0" + exp
		}
		num = mant + "E" + string(sgn) + exp
	} else {
		num = fixed(a, p)
	}
	out := strings.TrimSpace(p.prefix.String() + num + p.suffix.String())
	if neg && strings.Trim(num, "0.,") != "" {
		out = "-" + out
	}
	return out
}

func sign(v float64, neg bool) float64 {
	if neg {
		return -math.Abs(v)
	}
	return v
}

// fixed formats a non-negative value with the pattern's digit rules.
func fixed(a float64, p *numPattern) string {
	intPart, frac := roundDecimal(a, p.decimals)
	for len(frac) > p.minDecimals && strings.HasSuffix(frac, "0") {
		frac = frac[:len(frac)-1]
	}
	if intPart == "0" && p.intZeros == 0 {
		intPart = ""
	}
	for len(intPart) < p.intZeros {
		intPart = "0" + intPart
	}
	if p.group && len(intPart) > 3 {
		var b strings.Builder
		for i, d := range intPart {
			if i > 0 && (len(intPart)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteRune(d)
		}
		intPart = b.String()
	}
	if frac != "" {
		return intPart + "." + frac
	}
	if intPart == "" {
		return "0"
	}
	return intPart
}

// roundDecimal rounds a non-negative value half-up to d decimals on its
// decimal (15 significant digit) representation, as spreadsheets display
// it: 2.675 -> 2.68 although the binary value is slightly below.
func roundDecimal(a float64, d int) (string, string) {
	r, err := strconv.ParseFloat(strconv.FormatFloat(a, 'g', 15, 64), 64)
	if err != nil {
		r = a
	}
	s := strconv.FormatFloat(r, 'f', -1, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	if len(frac) <= d {
		return intPart, frac + strings.Repeat("0", d-len(frac))
	}
	up := frac[d] >= '5'
	digits := []byte(intPart + frac[:d])
	for i := len(digits) - 1; up && i >= 0; i-- {
		if digits[i] == '9' {
			digits[i] = '0'
			continue
		}
		digits[i]++
		up = false
	}
	if up {
		digits = append([]byte{'1'}, digits...)
	}
	n := len(digits) - d
	return string(digits[:n]), string(digits[n:])
}

// generalNumber renders a number the way the General format does, rounded
// to 15 significant digits so binary noise (0.1+0.2) does not show.
func generalNumber(v float64) string {
	if v == 0 {
		return "0"
	}
	r, err := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 15, 64), 64)
	if err != nil {
		r = v
	}
	if a := math.Abs(r); a >= 1e15 || a < 1e-9 {
		return strconv.FormatFloat(r, 'G', -1, 64)
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}

var (
	epoch1900 = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	epoch1904 = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
)

// excelTime converts a serial date to a time.
func excelTime(v float64, date1904 bool) (time.Time, bool) {
	if v < 0 || v > 2958466 {
		return time.Time{}, false
	}
	base := epoch1900
	switch {
	case date1904:
		base = epoch1904
	case v < 60:
		base = base.AddDate(0, 0, 1) // the fictional 1900-02-29
	}
	days := math.Floor(v)
	secs := math.Round((v - days) * 86400)
	return base.AddDate(0, 0, int(days)).Add(time.Duration(secs) * time.Second), true
}

func formatDate(v float64, p dateParts, date1904 bool) (string, bool) {
	if p.duration {
		total := int64(math.Round(math.Abs(v) * 86400))
		h, m, s := total/3600, total/60%60, total%60
		out := strconv.FormatInt(h, 10) + ":" + pad2(m)
		if p.seconds {
			out += ":" + pad2(s)
		}
		if v < 0 {
			out = "-" + out
		}
		return out, true
	}
	t, ok := excelTime(v, date1904)
	if !ok {
		return "", false
	}
	if !p.date {
		switch {
		case p.seconds && !p.hours:
			return t.Format("04:05"), true
		case p.seconds:
			return t.Format("15:04:05"), true
		}
		return t.Format("15:04"), true
	}
	if t.Hour() != 0 || t.Minute() != 0 || t.Second() != 0 {
		return t.Format("2006-01-02 15:04"), true
	}
	return t.Format("2006-01-02"), true
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

// isoDateTime formats an ISO 8601 date or date-time value (XLSX t="d",
// ODS office:date-value).
func isoDateTime(s string) string {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02T15:04:05.999999999Z07:00", "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
				return t.Format("2006-01-02")
			}
			return t.Format("2006-01-02 15:04")
		}
	}
	return s
}
