package cite

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// latexToUnicode converts the LaTeX markup found in BibTeX fields to plain
// Unicode: accents (\'e, \"{o}, {\v s}, \={a}, \c{k}), special letters
// (\o, \ss, \l), escaped characters, ligature dashes and quotes, and
// formatting commands, whose content is kept. Braces are removed and
// whitespace collapsed; inline math keeps its content without dollars.
func latexToUnicode(s string) string {
	if !strings.ContainsAny(s, "\\{}$~-`'\n\r\t") {
		return strings.TrimSpace(s)
	}
	t := &texReader{s: s}
	var sb strings.Builder
	sb.Grow(len(s))
	t.convert(&sb, false)
	return norm.NFC.String(collapseSpace(sb.String()))
}

// collapseSpace folds runs of ASCII whitespace into one space (keeping
// no-break spaces) and trims the ends.
func collapseSpace(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
			space = true
			continue
		}
		if space && sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		space = false
		sb.WriteRune(r)
	}
	return sb.String()
}

type texReader struct {
	s     string
	i     int
	depth int
}

// maxTexDepth bounds group nesting so hostile input cannot recurse
// without limit; text nested deeper is dropped.
const maxTexDepth = 200

var accentMarks = map[string]rune{
	"'": '\u0301', "`": '\u0300', "^": '\u0302', "\"": '\u0308', "~": '\u0303', "=": '\u0304',
	".": '\u0307', "u": '\u0306', "v": '\u030C', "H": '\u030B', "c": '\u0327', "k": '\u0328',
	"r": '\u030A', "d": '\u0323', "b": '\u0331', "t": '\u0361', "G": '\u030F',
	"textcommabelow": '\u0326',
}

var texLetters = map[string]string{
	"aa": "å", "AA": "Å", "ae": "æ", "AE": "Æ", "oe": "œ", "OE": "Œ", "o": "ø", "O": "Ø",
	"l": "ł", "L": "Ł", "ss": "ß", "SS": "SS", "i": "ı", "j": "ȷ", "dh": "ð", "DH": "Ð",
	"th": "þ", "TH": "Þ", "ng": "ŋ", "NG": "Ŋ", "dj": "đ", "DJ": "Đ",
}

var texSymbols = map[string]string{
	"textendash": "–", "textemdash": "—", "ldots": "…", "dots": "…", "textellipsis": "…",
	"textquoteleft": "‘", "textquoteright": "’", "textquotedblleft": "“", "textquotedblright": "”",
	"guillemotleft": "«", "guillemotright": "»", "guillemetleft": "«", "guillemetright": "»",
	"glqq": "„", "grqq": "“", "glq": "‚", "grq": "‘", "flqq": "«", "frqq": "»",
	"copyright": "©", "textcopyright": "©", "textregistered": "®", "texttrademark": "™",
	"textdegree": "°", "degree": "°", "S": "§", "textsection": "§", "P": "¶", "textparagraph": "¶",
	"pounds": "£", "textsterling": "£", "euro": "€", "texteuro": "€", "textbullet": "•",
	"textperiodcentered": "·", "textdagger": "†", "dag": "†", "textdaggerdbl": "‡", "ddag": "‡",
	"textexclamdown": "¡", "textquestiondown": "¿", "textasciitilde": "~", "textbackslash": "\\",
	"textunderscore": "_", "textasciicircum": "^", "textbar": "|", "textless": "<",
	"textgreater": ">", "textdollar": "$", "textpercent": "%", "textampersand": "&",
	"nobreakspace": "\u00a0", "quad": " ", "qquad": " ", "enspace": " ", "thinspace": " ",
	"LaTeX": "LaTeX", "TeX": "TeX", "BibTeX": "BibTeX", "LaTeXe": "LaTeX2e", "XeTeX": "XeTeX",
	"LuaTeX": "LuaTeX", "textordfeminine": "ª", "textordmasculine": "º", "textmu": "µ",
	"textonehalf": "½", "texttimes": "×", "textminus": "−", "textpm": "±",
}

// commands whose first argument is dropped and second kept.
var texSkipFirst = map[string]bool{
	"href": true, "foreignlanguage": true, "textcolor": true, "hyperlink": true,
	"hyperref": true, "colorbox": true,
}

// commands whose arguments are dropped entirely.
var texDropArg = map[string]bool{
	"noopsort": true, "hspace": true, "vspace": true, "label": true, "cite": true,
	"selectlanguage": true, "hyphenation": true, "index": true,
}

// convert writes the converted text until the end of input or, inside a
// group, the matching closing brace.
func (t *texReader) convert(sb *strings.Builder, inGroup bool) {
	t.depth++
	defer func() { t.depth-- }()
	if t.depth > maxTexDepth {
		t.i = len(t.s) // pathological nesting: drop the rest
		return
	}
	for t.i < len(t.s) {
		c := t.s[t.i]
		switch c {
		case '\\':
			t.command(sb)
		case '{':
			t.i++
			t.convert(sb, true)
		case '}':
			t.i++
			if inGroup {
				return
			}
		case '$':
			t.math(sb)
		case '~':
			sb.WriteString("\u00a0")
			t.i++
		case '-':
			n := 0
			for t.i < len(t.s) && t.s[t.i] == '-' {
				n++
				t.i++
			}
			switch n {
			case 1:
				sb.WriteByte('-')
			case 2:
				sb.WriteString("–")
			case 3:
				sb.WriteString("—")
			default:
				sb.WriteString(strings.Repeat("-", n))
			}
		case '`':
			if strings.HasPrefix(t.s[t.i:], "``") {
				sb.WriteString("“")
				t.i += 2
			} else {
				sb.WriteString("‘")
				t.i++
			}
		case '\'':
			if strings.HasPrefix(t.s[t.i:], "''") {
				sb.WriteString("”")
				t.i += 2
			} else {
				sb.WriteByte('\'')
				t.i++
			}
		default:
			r, size := utf8.DecodeRuneInString(t.s[t.i:])
			sb.WriteRune(r)
			t.i += size
		}
	}
}

func isASCIILetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

func (t *texReader) skipSpaces() {
	for t.i < len(t.s) && (t.s[t.i] == ' ' || t.s[t.i] == '\t' || t.s[t.i] == '\n' || t.s[t.i] == '\r') {
		t.i++
	}
}

// group reads a braced argument and returns its converted content; when
// no brace follows it returns "", false.
func (t *texReader) group() (string, bool) {
	save := t.i
	t.skipSpaces()
	if t.i >= len(t.s) || t.s[t.i] != '{' {
		t.i = save
		return "", false
	}
	t.i++
	var sb strings.Builder
	t.convert(&sb, true)
	return sb.String(), true
}

// rawGroup reads a braced argument verbatim (URLs).
func (t *texReader) rawGroup() (string, bool) {
	save := t.i
	t.skipSpaces()
	if t.i >= len(t.s) || t.s[t.i] != '{' {
		t.i = save
		return "", false
	}
	depth := 0
	start := t.i + 1
	for ; t.i < len(t.s); t.i++ {
		switch t.s[t.i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				out := t.s[start:t.i]
				t.i++
				return out, true
			}
		}
	}
	return t.s[start:], true
}

func (t *texReader) command(sb *strings.Builder) {
	t.i++ // backslash
	if t.i >= len(t.s) {
		return
	}
	c := t.s[t.i]
	if !isASCIILetter(c) {
		t.i++
		sym := string(c)
		if mark, ok := accentMarks[sym]; ok {
			sb.WriteString(applyAccent(t.accentArg(), mark))
			return
		}
		switch c {
		case '&', '%', '$', '#', '_', '{', '}', '[', ']', '|', '<', '>', '*', '+':
			sb.WriteByte(c)
		case ' ', '\t', '\n', '\r', '\\', ',', ';', ':':
			sb.WriteByte(' ')
		case '-', '/', '@', '!', '(', ')':
			// discretionary hyphen, italic correction, spacing commands
		default:
			t.i--
			r, size := utf8.DecodeRuneInString(t.s[t.i:])
			sb.WriteRune(r)
			t.i += size
		}
		return
	}
	start := t.i
	for t.i < len(t.s) && isASCIILetter(t.s[t.i]) {
		t.i++
	}
	name := t.s[start:t.i]
	if mark, ok := accentMarks[name]; ok {
		sb.WriteString(applyAccent(t.accentArg(), mark))
		return
	}
	if l, ok := texLetters[name]; ok {
		t.skipSpaces()
		sb.WriteString(l)
		return
	}
	if sym, ok := texSymbols[name]; ok {
		t.skipSpaces()
		sb.WriteString(sym)
		return
	}
	switch {
	case name == "url" || name == "path" || name == "nolinkurl":
		if raw, ok := t.rawGroup(); ok {
			sb.WriteString(raw)
		}
		return
	case name == "enquote" || name == "mkbibquote":
		if g, ok := t.group(); ok {
			sb.WriteString("“" + g + "”")
		}
		return
	case texSkipFirst[name]:
		t.rawGroup()
		if g, ok := t.group(); ok {
			sb.WriteString(g)
		}
		return
	case texDropArg[name]:
		t.rawGroup()
		return
	}
	// Formatting commands (\emph, \textbf, \mbox …) keep their argument;
	// declarations (\em, \bf) and unknown argument-less commands vanish.
	if g, ok := t.group(); ok {
		sb.WriteString(g)
		return
	}
	t.skipSpaces()
}

// accentArg reads the base character of an accent: "{o}", "o", "\i".
func (t *texReader) accentArg() string {
	t.skipSpaces()
	if t.i >= len(t.s) {
		return ""
	}
	switch t.s[t.i] {
	case '{':
		t.i++
		var sb strings.Builder
		t.convert(&sb, true)
		return sb.String()
	case '\\':
		var sb strings.Builder
		t.command(&sb)
		return sb.String()
	}
	r, size := utf8.DecodeRuneInString(t.s[t.i:])
	t.i += size
	return string(r)
}

func applyAccent(arg string, mark rune) string {
	if arg == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(arg)
	switch r {
	case 'ı':
		r = 'i'
	case 'ȷ':
		r = 'j'
	}
	return string(r) + string(mark) + arg[size:]
}

var mathSymbols = map[string]string{
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε", "varepsilon": "ε",
	"zeta": "ζ", "eta": "η", "theta": "θ", "vartheta": "ϑ", "iota": "ι", "kappa": "κ",
	"lambda": "λ", "mu": "μ", "nu": "ν", "xi": "ξ", "pi": "π", "rho": "ρ", "sigma": "σ",
	"tau": "τ", "upsilon": "υ", "phi": "φ", "varphi": "φ", "chi": "χ", "psi": "ψ", "omega": "ω",
	"Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ", "Pi": "Π", "Sigma": "Σ",
	"Upsilon": "Υ", "Phi": "Φ", "Psi": "Ψ", "Omega": "Ω",
	"pm": "±", "mp": "∓", "times": "×", "cdot": "·", "div": "÷", "leq": "≤", "le": "≤",
	"geq": "≥", "ge": "≥", "neq": "≠", "ne": "≠", "approx": "≈", "sim": "∼", "simeq": "≃",
	"equiv": "≡", "infty": "∞", "partial": "∂", "nabla": "∇", "sum": "∑", "prod": "∏",
	"int": "∫", "sqrt": "√", "to": "→", "rightarrow": "→", "leftarrow": "←",
	"leftrightarrow": "↔", "Rightarrow": "⇒", "in": "∈", "notin": "∉", "subset": "⊂",
	"subseteq": "⊆", "cup": "∪", "cap": "∩", "emptyset": "∅", "forall": "∀", "exists": "∃",
	"neg": "¬", "wedge": "∧", "vee": "∨", "circ": "∘", "degree": "°", "prime": "′",
	"ell": "ℓ", "hbar": "ℏ", "langle": "⟨", "rangle": "⟩", "ldots": "…", "cdots": "⋯",
}

// math converts $…$ content to readable text.
func (t *texReader) math(sb *strings.Builder) {
	t.i++
	display := t.i < len(t.s) && t.s[t.i] == '$'
	if display {
		t.i++
	}
	end := t.i
	for end < len(t.s) && (t.s[end] != '$' || t.s[end-1] == '\\') {
		end++
	}
	content := t.s[t.i:end]
	t.i = end
	for t.i < len(t.s) && t.s[t.i] == '$' {
		t.i++
	}
	var out strings.Builder
	for i := 0; i < len(content); {
		c := content[i]
		switch c {
		case '\\':
			j := i + 1
			for j < len(content) && isASCIILetter(content[j]) {
				j++
			}
			name := content[i+1 : j]
			if name == "" && j < len(content) {
				if content[j] != ',' && content[j] != ';' && content[j] != '!' {
					out.WriteByte(content[j])
				}
				j++
			} else if sym, ok := mathSymbols[name]; ok {
				out.WriteString(sym)
			} else if l, ok := texLetters[name]; ok {
				out.WriteString(l)
			} else if !strings.HasPrefix(name, "math") && name != "text" && name != "mbox" &&
				name != "left" && name != "right" {
				out.WriteString(name)
			}
			i = j
		case '{', '}':
			i++
		default:
			r, size := utf8.DecodeRuneInString(content[i:])
			out.WriteRune(r)
			i += size
		}
	}
	sb.WriteString(out.String())
}
