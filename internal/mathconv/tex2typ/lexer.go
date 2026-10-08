package tex2typ

import (
	"unicode"
	"unicode/utf8"
)

type tokKind uint8

const (
	tEOF   tokKind = iota
	tChar          // a single character (letter, digit, punctuation, any Unicode)
	tCmd           // control word or control symbol; val is the name without '\'
	tOpen          // {
	tClose         // }
	tSup           // ^
	tSub           // _
	tAlign         // &
	tPrime         // '
	tSpace         // a run of white space
	tTilde         // ~ (non-breaking space)
	tHash          // # (macro parameter character, literal in our model)
)

type token struct {
	kind tokKind
	r    rune   // tChar
	val  string // tCmd name; tChar text (the rune plus any combining marks)
	pos  int    // byte offset in the source
}

func add(toks []token, pos int, t token) []token {
	t.pos = pos
	return append(toks, t)
}

// lex splits LaTeX source into tokens in a single pass. Comments are dropped,
// white-space runs collapse into one tSpace token and, as in TeX, white space
// after a control word is skipped.
func lex(s string) []token {
	toks := make([]token, 0, len(s)+1)
	for i := 0; i < len(s); {
		start := i
		c := s[i]
		switch {
		case c == '\\':
			i++
			if i >= len(s) {
				// A lone trailing backslash: keep it as a literal.
				toks = add(toks, start, token{kind: tChar, r: '\\', val: "\\"})
				continue
			}
			if isASCIILetter(s[i]) {
				j := i
				for j < len(s) && isASCIILetter(s[j]) {
					j++
				}
				toks = add(toks, start, token{kind: tCmd, val: s[i:j]})
				i = j
				for i < len(s) && isSpaceByte(s[i]) {
					i++
				}
				continue
			}
			r, n := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && n <= 1 {
				r = unicode.ReplacementChar
			}
			name := s[i : i+n]
			if isSpaceByte(s[i]) {
				name = " "
			} else if r == unicode.ReplacementChar {
				name = string(unicode.ReplacementChar)
			}
			toks = add(toks, start, token{kind: tCmd, val: name})
			i += n
		case c == '%':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			// TeX also swallows the newline and the next line's indentation.
			for i < len(s) && isSpaceByte(s[i]) {
				i++
			}
		case isSpaceByte(c):
			for i < len(s) && isSpaceByte(s[i]) {
				i++
			}
			if n := len(toks); n == 0 || toks[n-1].kind != tSpace {
				toks = add(toks, start, token{kind: tSpace})
			}
		case c == '{':
			toks = add(toks, start, token{kind: tOpen})
			i++
		case c == '}':
			toks = add(toks, start, token{kind: tClose})
			i++
		case c == '^':
			toks = add(toks, start, token{kind: tSup})
			i++
		case c == '_':
			toks = add(toks, start, token{kind: tSub})
			i++
		case c == '&':
			toks = add(toks, start, token{kind: tAlign})
			i++
		case c == '\'':
			toks = add(toks, start, token{kind: tPrime})
			i++
		case c == '~':
			toks = add(toks, start, token{kind: tTilde})
			i++
		case c == '#':
			toks = add(toks, start, token{kind: tHash})
			i++
		case c < 0x20 || c == 0x7f:
			i++ // control characters carry no meaning in math
		default:
			r, n := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && n <= 1 {
				r = unicode.ReplacementChar
			}
			j := i + n
			// Keep combining marks with their base character so that the
			// pair is emitted (and spaced) as one grapheme.
			for j < len(s) {
				m, k := utf8.DecodeRuneInString(s[j:])
				if !isMark(m) {
					break
				}
				j += k
			}
			val := s[i:j]
			if r == unicode.ReplacementChar && n <= 1 {
				val = string(unicode.ReplacementChar)
			}
			toks = add(toks, start, token{kind: tChar, r: r, val: val})
			i = j
		}
	}
	return toks
}

func isASCIILetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

func isMark(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Mc, r) ||
		(r >= 0xFE00 && r <= 0xFE0F) || r == 0x200D
}

// isWordRune reports whether r may continue a Typst identifier. Two such
// characters written next to each other would merge into one (unknown)
// multi-letter identifier, so they must be separated by a space.
func isWordRune(r rune) bool {
	if r < utf8.RuneSelf {
		return isASCIILetter(byte(r)) || (r >= '0' && r <= '9') || r == '_'
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Nl, r) ||
		unicode.Is(unicode.Pc, r) || isMark(r) ||
		unicode.Is(unicode.Other_ID_Start, r) || unicode.Is(unicode.Other_ID_Continue, r)
}
