package pdf

import (
	"bytes"
	"math"
	"strconv"
)

// maxNesting bounds array/dictionary nesting in hostile input.
const maxNesting = 64

// lexer tokenises PDF syntax (file objects and content streams). It never
// fails: malformed bytes are skipped and the end of input yields io EOF
// semantics through ok=false.
type lexer struct {
	b   []byte
	pos int
	// names interns names and keywords: content streams repeat a handful of
	// operators and resource names thousands of times.
	names map[string]string
}

func newLexer(b []byte) *lexer {
	return &lexer{b: b, names: make(map[string]string, 32)}
}

func isSpace(c byte) bool {
	switch c {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	}
	return false
}

func isDelim(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

func isRegular(c byte) bool { return !isSpace(c) && !isDelim(c) }

// skipSpace skips whitespace and comments.
func (lx *lexer) skipSpace() {
	for lx.pos < len(lx.b) {
		c := lx.b[lx.pos]
		if isSpace(c) {
			lx.pos++
			continue
		}
		if c == '%' {
			for lx.pos < len(lx.b) && lx.b[lx.pos] != '\n' && lx.b[lx.pos] != '\r' {
				lx.pos++
			}
			continue
		}
		return
	}
}

// delimiter tokens returned by next.
type delim byte

const (
	dArrayOpen  delim = '['
	dArrayClose delim = ']'
	dDictOpen   delim = '<'
	dDictClose  delim = '>'
	dProcOpen   delim = '{'
	dProcClose  delim = '}'
)

// next returns the next token: int, float64, name, pdfString, keyword or
// delim. ok is false at the end of input.
func (lx *lexer) next() (tok any, ok bool) {
	for {
		lx.skipSpace()
		if lx.pos >= len(lx.b) {
			return nil, false
		}
		c := lx.b[lx.pos]
		switch c {
		case '[', ']', '{', '}':
			lx.pos++
			return delim(c), true
		case '<':
			if lx.pos+1 < len(lx.b) && lx.b[lx.pos+1] == '<' {
				lx.pos += 2
				return dDictOpen, true
			}
			lx.pos++
			return lx.hexString(), true
		case '>':
			lx.pos++
			if lx.pos < len(lx.b) && lx.b[lx.pos] == '>' {
				lx.pos++
				return dDictClose, true
			}
			continue // stray '>'
		case '(':
			lx.pos++
			return lx.literalString(), true
		case ')':
			lx.pos++
			continue // stray ')'
		case '/':
			lx.pos++
			return lx.name(), true
		}
		start := lx.pos
		for lx.pos < len(lx.b) && isRegular(lx.b[lx.pos]) {
			lx.pos++
		}
		word := lx.b[start:lx.pos]
		if v, isNum := parseNumber(word); isNum {
			return v, true
		}
		return keyword(lx.intern(word)), true
	}
}

func (lx *lexer) intern(b []byte) string {
	if s, ok := lx.names[string(b)]; ok {
		return s
	}
	s := string(b)
	if len(lx.names) < 4096 {
		lx.names[s] = s
	}
	return s
}

// parseNumber parses PDF integers and reals, tolerating common writer bugs
// ("--5", "1.2.3", "5-"). It reports false for non-numeric words.
func parseNumber(b []byte) (any, bool) {
	if len(b) == 0 {
		return nil, false
	}
	i := 0
	neg := false
	for i < len(b) && (b[i] == '+' || b[i] == '-') {
		neg = b[i] == '-'
		i++
	}
	if i == len(b) {
		return nil, false
	}
	if b[i] != '.' && (b[i] < '0' || b[i] > '9') {
		return nil, false
	}
	var ip int64
	digits := 0
	overflow := false
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		if ip > (math.MaxInt64-9)/10 {
			overflow = true
		} else {
			ip = ip*10 + int64(b[i]-'0')
		}
		digits++
		i++
	}
	if i == len(b) && !overflow {
		if ip > math.MaxInt32 {
			// Keep huge integers as reals; they are never valid offsets
			// or counts we would act on.
			f := float64(ip)
			if neg {
				f = -f
			}
			return f, true
		}
		if neg {
			ip = -ip
		}
		return int(ip), true
	}
	if i < len(b) && b[i] != '.' {
		// Garbage after digits ("12abc"): treat as keyword unless it is a
		// stray sign ("5-").
		for j := i; j < len(b); j++ {
			if b[j] != '-' && b[j] != '+' {
				return nil, false
			}
		}
		f := float64(ip)
		if neg {
			f = -f
		}
		if f == math.Trunc(f) && math.Abs(f) < math.MaxInt32 {
			return int(f), true
		}
		return f, true
	}
	// Fraction.
	f := float64(ip)
	if overflow {
		v, err := strconv.ParseFloat(string(b[:i]), 64)
		if err == nil {
			f = math.Abs(v)
		}
	}
	if i < len(b) && b[i] == '.' {
		i++
		scale := 0.1
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			f += float64(b[i]-'0') * scale
			scale /= 10
			digits++
			i++
		}
	}
	if digits == 0 {
		return nil, false
	}
	if neg {
		f = -f
	}
	return f, true
}

func (lx *lexer) name() name {
	start := lx.pos
	hasEscape := false
	for lx.pos < len(lx.b) && isRegular(lx.b[lx.pos]) {
		if lx.b[lx.pos] == '#' {
			hasEscape = true
		}
		lx.pos++
	}
	raw := lx.b[start:lx.pos]
	if !hasEscape {
		return name(lx.intern(raw))
	}
	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] == '#' && i+2 < len(raw) {
			if h, ok := unhex2(raw[i+1], raw[i+2]); ok {
				out = append(out, h)
				i += 2
				continue
			}
		}
		out = append(out, raw[i])
	}
	return name(lx.intern(out))
}

func unhexDigit(c byte) (byte, bool) {
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

func unhex2(a, b byte) (byte, bool) {
	x, ok1 := unhexDigit(a)
	y, ok2 := unhexDigit(b)
	return x<<4 | y, ok1 && ok2
}

func (lx *lexer) hexString() pdfString {
	var out []byte
	var hi byte
	half := false
	for lx.pos < len(lx.b) {
		c := lx.b[lx.pos]
		lx.pos++
		if c == '>' {
			break
		}
		d, ok := unhexDigit(c)
		if !ok {
			continue
		}
		if half {
			out = append(out, hi<<4|d)
			half = false
		} else {
			hi = d
			half = true
		}
	}
	if half {
		out = append(out, hi<<4)
	}
	return pdfString(out)
}

func (lx *lexer) literalString() pdfString {
	start := lx.pos
	// Fast path: no escapes and no nested parentheses.
	for i := start; i < len(lx.b); i++ {
		c := lx.b[i]
		if c == '\\' || c == '(' || c == '\r' {
			break
		}
		if c == ')' {
			lx.pos = i + 1
			return pdfString(lx.b[start:i])
		}
	}
	out := make([]byte, 0, 32)
	depth := 1
	for lx.pos < len(lx.b) {
		c := lx.b[lx.pos]
		lx.pos++
		switch c {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return pdfString(out)
			}
		case '\r':
			// End-of-line markers inside strings are read as \n.
			if lx.pos < len(lx.b) && lx.b[lx.pos] == '\n' {
				lx.pos++
			}
			c = '\n'
		case '\\':
			if lx.pos >= len(lx.b) {
				return pdfString(out)
			}
			e := lx.b[lx.pos]
			lx.pos++
			switch e {
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			case 'b':
				c = '\b'
			case 'f':
				c = '\f'
			case '\r':
				if lx.pos < len(lx.b) && lx.b[lx.pos] == '\n' {
					lx.pos++
				}
				continue
			case '\n':
				continue
			default:
				if e >= '0' && e <= '7' {
					v := int(e - '0')
					for k := 0; k < 2 && lx.pos < len(lx.b) && lx.b[lx.pos] >= '0' && lx.b[lx.pos] <= '7'; k++ {
						v = v*8 + int(lx.b[lx.pos]-'0')
						lx.pos++
					}
					c = byte(v)
				} else {
					c = e
				}
			}
		}
		out = append(out, c)
	}
	return pdfString(out)
}

// object parses one complete object. References ("1 0 R") are recognised.
// Keywords other than true/false/null are returned as keyword values so the
// caller can react to "obj", "stream", "endobj" or content operators.
func (lx *lexer) object(depth int) (any, bool) {
	tok, ok := lx.next()
	if !ok {
		return nil, false
	}
	return lx.objectFrom(tok, depth)
}

func (lx *lexer) objectFrom(tok any, depth int) (any, bool) {
	switch t := tok.(type) {
	case int:
		if t >= 0 {
			save := lx.pos
			if g, ok := lx.next(); ok {
				if gen, isInt := g.(int); isInt && gen >= 0 {
					if r, ok := lx.next(); ok && r == keyword("R") {
						return ref{t, gen}, true
					}
				}
			}
			lx.pos = save
		}
		return t, true
	case keyword:
		switch t {
		case "true":
			return true, true
		case "false":
			return false, true
		case "null":
			return nil, true
		}
		return t, true
	case delim:
		switch t {
		case dArrayOpen:
			return lx.array(depth + 1), true
		case dDictOpen:
			return lx.dict(depth + 1), true
		}
		return t, true
	}
	return tok, true
}

func (lx *lexer) array(depth int) array {
	var out array
	for {
		tok, ok := lx.next()
		if !ok {
			return out
		}
		if tok == dArrayClose {
			return out
		}
		if tok == dDictClose {
			// Malformed: dictionary closes inside an array.
			lx.pos -= 2
			return out
		}
		if depth > maxNesting {
			continue
		}
		v, _ := lx.objectFrom(tok, depth)
		if k, isKw := v.(keyword); isKw && (k == "endobj" || k == "stream" || k == "obj") {
			lx.pos -= len(k)
			return out
		}
		out = append(out, v)
	}
}

func (lx *lexer) dict(depth int) dict {
	out := dict{}
	for {
		tok, ok := lx.next()
		if !ok {
			return out
		}
		if tok == dDictClose {
			return out
		}
		key, isName := tok.(name)
		if !isName {
			if k, isKw := tok.(keyword); isKw && (k == "endobj" || k == "stream" || k == "obj") {
				lx.pos -= len(k)
				return out
			}
			continue // skip junk keys
		}
		vtok, ok := lx.next()
		if !ok {
			return out
		}
		if vtok == dDictClose {
			out[key] = nil
			return out
		}
		if depth > maxNesting {
			continue
		}
		v, _ := lx.objectFrom(vtok, depth)
		if k, isKw := v.(keyword); isKw {
			if k == "endobj" || k == "stream" || k == "obj" {
				lx.pos -= len(k)
				return out
			}
			continue
		}
		if v != nil {
			out[key] = v
		}
	}
}

// hasPrefixAt reports whether b[i:] starts with s.
func hasPrefixAt(b []byte, i int, s string) bool {
	return i >= 0 && i+len(s) <= len(b) && string(b[i:i+len(s)]) == s
}

// indexFrom returns the index of sep in b at or after i, or -1.
func indexFrom(b []byte, i int, sep string) int {
	if i < 0 || i > len(b) {
		return -1
	}
	j := bytes.Index(b[i:], []byte(sep))
	if j < 0 {
		return -1
	}
	return i + j
}
