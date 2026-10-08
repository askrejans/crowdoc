package rtf

// tokKind classifies an RTF token.
type tokKind uint8

const (
	tEOF        tokKind = iota
	tGroupStart         // {
	tGroupEnd           // }
	tWord               // control word, e.g. \b0
	tSymbol             // control symbol, e.g. \~ (ch holds the symbol)
	tHex                // \'hh (ch holds the byte)
	tText               // literal text without CR/LF
	tBin                // \binN payload
)

type token struct {
	kind     tokKind
	name     string // control word name
	param    int
	hasParam bool
	ch       byte
	text     string // text or binary payload
}

// lexer splits RTF source into tokens. It never fails: malformed escapes are
// dropped and truncated input simply ends the token stream.
type lexer struct {
	src string
	pos int
}

const maxParamDigits = 10

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func (l *lexer) next() token {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch c {
		case '{':
			l.pos++
			return token{kind: tGroupStart}
		case '}':
			l.pos++
			return token{kind: tGroupEnd}
		case '\\':
			if tok, ok := l.control(); ok {
				return tok
			}
		case '\r', '\n', 0:
			l.pos++
		default:
			start := l.pos
			for l.pos < len(l.src) {
				c := l.src[l.pos]
				if c == '\\' || c == '{' || c == '}' || c == '\r' || c == '\n' || c == 0 {
					break
				}
				l.pos++
			}
			return token{kind: tText, text: l.src[start:l.pos]}
		}
	}
	return token{kind: tEOF}
}

// control reads the token starting at a backslash. ok is false when the
// escape was malformed and produced nothing.
func (l *lexer) control() (token, bool) {
	l.pos++ // backslash
	if l.pos >= len(l.src) {
		return token{}, false
	}
	c := l.src[l.pos]
	if !isLetter(c) {
		l.pos++
		switch c {
		case '\'':
			if l.pos+1 < len(l.src) {
				h, ok1 := hexVal(l.src[l.pos])
				lo, ok2 := hexVal(l.src[l.pos+1])
				if ok1 && ok2 {
					l.pos += 2
					return token{kind: tHex, ch: h<<4 | lo}, true
				}
			}
			// Truncated or invalid escape: skip whatever hex digits exist.
			for i := 0; i < 2 && l.pos < len(l.src); i++ {
				if _, ok := hexVal(l.src[l.pos]); !ok {
					break
				}
				l.pos++
			}
			return token{}, false
		case '\r', '\n':
			// "\<newline>" is an alias for \par.
			return token{kind: tWord, name: "par"}, true
		}
		return token{kind: tSymbol, ch: c}, true
	}
	start := l.pos
	for l.pos < len(l.src) && isLetter(l.src[l.pos]) && l.pos-start < 32 {
		l.pos++
	}
	tok := token{kind: tWord, name: l.src[start:l.pos]}
	// Skip any excess letters of an over-long control word.
	for l.pos < len(l.src) && isLetter(l.src[l.pos]) {
		l.pos++
	}
	neg := false
	if l.pos+1 < len(l.src) && l.src[l.pos] == '-' && isDigit(l.src[l.pos+1]) {
		neg = true
		l.pos++
	}
	if l.pos < len(l.src) && isDigit(l.src[l.pos]) {
		tok.hasParam = true
		n, digits := 0, 0
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			if digits < maxParamDigits {
				n = n*10 + int(l.src[l.pos]-'0')
			}
			digits++
			l.pos++
		}
		if n > 1<<31-1 {
			n = 1<<31 - 1
		}
		if neg {
			n = -n
		}
		tok.param = n
	}
	if l.pos < len(l.src) && l.src[l.pos] == ' ' {
		l.pos++
	}
	if tok.name == "bin" {
		n := max(tok.param, 0)
		n = min(n, len(l.src)-l.pos)
		tok.kind = tBin
		tok.text = l.src[l.pos : l.pos+n]
		l.pos += n
	}
	return tok, true
}

// skipGroup consumes input up to and including the brace that closes the
// group the lexer is currently inside. It is a fast path for destinations
// whose content is ignored (theme data, headers, embedded objects, ...).
func (l *lexer) skipGroup() {
	depth := 1
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch c {
		case '{':
			depth++
			l.pos++
		case '}':
			depth--
			l.pos++
			if depth == 0 {
				return
			}
		case '\\':
			if l.pos+4 < len(l.src) && l.src[l.pos+1:l.pos+4] == "bin" && (isDigit(l.src[l.pos+4]) || l.src[l.pos+4] == '-') {
				l.control() // consumes the payload
				continue
			}
			l.pos += 2
		default:
			l.pos++
		}
	}
}
