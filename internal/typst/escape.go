package typst

import (
	"strconv"
	"strings"
	"unicode"
)

// markupEscaper escapes text for Typst markup. lineStart reports whether
// the text begins a line, where list, enum, heading and term markers apply.
func escapeMarkup(s string, lineStart bool) string {
	if s == "" {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(s) + 8)
	rs := []rune(s)
	for i, r := range rs {
		next := rune(0)
		if i+1 < len(rs) {
			next = rs[i+1]
		}
		prev := rune(0)
		if i > 0 {
			prev = rs[i-1]
		}
		atStart := lineStart && i == 0
		switch r {
		case '\\', '#', '*', '_', '`', '$', '<', '>', '@', '[', ']', '~', '=':
			sb.WriteByte('\\')
		case '-':
			// "--", "---" and "-?" are shorthands; "- " starts a list.
			if atStart || next == '-' || next == '?' || prev == '-' || (next >= '0' && next <= '9') {
				sb.WriteByte('\\')
			}
		case '/':
			// "//" and "/*" start comments; "/ " starts a term list.
			if atStart || next == '/' || next == '*' || prev == '/' {
				sb.WriteByte('\\')
			}
		case '+':
			if atStart {
				sb.WriteByte('\\')
			}
		case '.':
			// "1." at the start of a line is an enumeration marker; "..."
			// is fine (it becomes an ellipsis, as intended).
			if lineStart && allDigits(rs[:i]) && i > 0 {
				sb.WriteByte('\\')
			}
		case '\n', '\r':
			sb.WriteByte(' ')
			continue
		case '\u0000':
			continue
		}
		if r < 0x20 && r != '\t' {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func allDigits(rs []rune) bool {
	for _, r := range rs {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// str returns a Typst string literal.
func str(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				sb.WriteString(`\u{` + strconv.FormatInt(int64(r), 16) + `}`)
				continue
			}
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// content wraps escaped markup in a content block.
func content(markup string) string { return "[" + markup + "]" }

// textContent is a content block holding plain text.
func textContent(s string) string { return content(escapeMarkup(s, true)) }

func strArray(items []string) string {
	if len(items) == 0 {
		return "()"
	}
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = str(it)
	}
	if len(parts) == 1 {
		return "(" + parts[0] + ",)"
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func boolean(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// isLabel reports whether s can be written with <label> syntax.
func isLabel(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == ':' || r == '.') {
			return false
		}
	}
	return true
}

// labelSuffix returns " <id>" for an element label, or "".
func labelSuffix(id string) string {
	if id == "" {
		return ""
	}
	if isLabel(id) {
		return " <" + id + ">"
	}
	return "#label(" + str(id) + ")"
}

// labelRef returns a Typst expression for a label value.
func labelRef(id string) string {
	if isLabel(id) {
		return "<" + id + ">"
	}
	return "label(" + str(id) + ")"
}
