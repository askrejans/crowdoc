package hyph

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const softHyphen = '\u00ad'

// hyphenator binds a pattern set to reusable scratch buffers. It is not
// safe for concurrent use; create one per call.
type hyphenator struct {
	l *lang
	p *patternSet
	s scratch
}

func newHyphenator(l *lang) *hyphenator {
	return &hyphenator{l: l, p: l.load()}
}

// analyse folds piece, which must consist of letters and combining marks
// only, and stores its break positions in h.s.breaks. It reports whether the
// piece was hyphenatable at all (letters of the language's script).
func (h *hyphenator) analyse(piece string) bool {
	s := &h.s
	s.letters, s.breakOK, s.offset, s.runeOff = s.letters[:0], s.breakOK[:0], s.offset[:0], s.runeOff[:0]
	s.breaks = s.breaks[:0]

	marks := false
	for i, r := range piece {
		switch {
		case isMark(r):
			if i == 0 {
				return false
			}
			marks = true
		case !h.inScript(r):
			return false
		}
	}
	if marks {
		h.foldClusters(piece)
	} else {
		ri := 0
		for i, r := range piece {
			s.letters = append(s.letters, h.fold(r))
			s.breakOK = append(s.breakOK, true)
			s.offset = append(s.offset, i)
			s.runeOff = append(s.runeOff, ri)
			ri++
		}
	}
	s.breakOK = append(s.breakOK, true) // gap after the last letter
	h.p.hyphenate(s)
	return true
}

// foldClusters handles words carrying combining marks: each base letter and
// its marks are composed to NFC before matching, and breaks are only allowed
// between such clusters, so decomposed input hyphenates like composed input.
func (h *hyphenator) foldClusters(piece string) {
	s := &h.s
	ri := 0
	for i := 0; i < len(piece); {
		_, size := utf8.DecodeRuneInString(piece[i:])
		j, rj := i+size, ri+1
		for j < len(piece) {
			r, sz := utf8.DecodeRuneInString(piece[j:])
			if !isMark(r) {
				break
			}
			j += sz
			rj++
		}
		first := true
		for _, r := range norm.NFC.String(piece[i:j]) {
			s.letters = append(s.letters, h.fold(r))
			s.breakOK = append(s.breakOK, first)
			s.offset = append(s.offset, i)
			s.runeOff = append(s.runeOff, ri)
			first = false
		}
		i, ri = j, rj
	}
}

func (h *hyphenator) fold(r rune) rune {
	if r < utf8.RuneSelf {
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
	} else {
		r = unicode.ToLower(r)
	}
	if h.l.fold != nil {
		r = h.l.fold(r)
	}
	return r
}

func (h *hyphenator) inScript(r rune) bool {
	if r < utf8.RuneSelf {
		return h.l.script == unicode.Latin && ('a' <= r && r <= 'z' || 'A' <= r && r <= 'Z')
	}
	return unicode.Is(h.l.script, r) && (unicode.IsLetter(r) || isMark(r))
}

func isMark(r rune) bool {
	return r >= 0x300 && unicode.Is(unicode.M, r)
}

func isApostrophe(r rune) bool {
	return r == '\'' || r == '\u2019' || r == '\u02bc'
}

// isBlockingRune reports runes that make a whole word off-limits: digits,
// underscores, explicit or soft hyphens and joiners mark identifiers,
// compounds the author already divided, or deliberate shaping.
func isBlockingRune(r rune) bool {
	switch r {
	case '_', '-', softHyphen, '\u2010', '\u2011', '\u200c', '\u200d', '\u2060': // hyphen, non-breaking hyphen, ZWNJ, ZWJ, word joiner
		return true
	}
	return r >= '0' && r <= '9' || r >= utf8.RuneSelf && unicode.IsNumber(r)
}

// isWordRune reports runes that belong to a word token.
func isWordRune(r rune) bool {
	if r < utf8.RuneSelf {
		return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' ||
			r == '_' || r == '-' || r == '\''
	}
	return unicode.IsLetter(r) || isMark(r) || isApostrophe(r) || isBlockingRune(r)
}

// InsertSoftHyphens returns text with U+00AD inserted at every permitted
// break of every word written in tag's language, or text unchanged when no
// patterns exist for tag. Words shorter than the language's left+right
// hyphenmin, URLs, e-mail addresses, file paths, and words containing
// digits, underscores, hyphens or soft hyphens are left intact, as are words
// with letters from another script. Every other byte of text is preserved.
func InsertSoftHyphens(text, tag string) string {
	l := resolve(tag)
	if l == nil || text == "" {
		return text
	}
	w := softWriter{text: text, h: newHyphenator(l)}
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		j := i + size
		for j < len(text) {
			r, size = utf8.DecodeRuneInString(text[j:])
			if unicode.IsSpace(r) {
				break
			}
			j += size
		}
		if !isURLish(text[i:j]) {
			w.chunk(i, j)
		}
		i = j
	}
	if w.b.Len() == 0 {
		return text
	}
	w.b.WriteString(text[w.done:])
	return w.b.String()
}

// softWriter copies text lazily, only materialising a new string once the
// first soft hyphen is inserted.
type softWriter struct {
	text string
	h    *hyphenator
	b    strings.Builder
	done int // bytes of text already copied to b
}

func (w *softWriter) insertAt(off int) {
	if w.b.Len() == 0 {
		w.b.Grow(len(w.text) + len(w.text)/8)
	}
	w.b.WriteString(w.text[w.done:off])
	w.b.WriteRune(softHyphen)
	w.done = off
}

// chunk hyphenates the words inside the whitespace-free span text[i:j].
func (w *softWriter) chunk(i, j int) {
	for k := i; k < j; {
		r, size := utf8.DecodeRuneInString(w.text[k:])
		if !isWordRune(r) {
			k += size
			continue
		}
		end, blocked := k, false
		for end < j {
			r, size = utf8.DecodeRuneInString(w.text[end:])
			if !isWordRune(r) {
				break
			}
			blocked = blocked || isBlockingRune(r)
			end += size
		}
		if !blocked {
			w.word(k, end)
		}
		k = end
	}
}

// word hyphenates text[i:j], a run of letters, marks and apostrophes; the
// parts between apostrophes are treated as separate words.
func (w *softWriter) word(i, j int) {
	start := i
	for k := i; k < j; {
		r, size := utf8.DecodeRuneInString(w.text[k:])
		if isApostrophe(r) {
			w.piece(start, k)
			start = k + size
		}
		k += size
	}
	w.piece(start, j)
}

func (w *softWriter) piece(i, j int) {
	if j-i < 2 || !w.h.analyse(w.text[i:j]) {
		return
	}
	for _, m := range w.h.s.breaks {
		w.insertAt(i + w.h.s.offset[m])
	}
}

const (
	openPunct  = "([{<\"'«‹„“‚‘¿¡"
	closePunct = ")]}>\"'»›“”’‘.,;:!?…"
)

// isURLish reports whether a whitespace-delimited chunk is a URL, e-mail
// address, domain name or file path, all of which must never be divided.
func isURLish(chunk string) bool {
	if strings.Contains(chunk, "://") || strings.ContainsRune(chunk, '@') || strings.ContainsRune(chunk, '\\') {
		return true
	}
	core := strings.TrimRight(strings.TrimLeft(chunk, openPunct), closePunct)
	if core == "" {
		return false
	}
	if len(core) > 4 && strings.EqualFold(core[:4], "www.") {
		return true
	}
	if core[0] == '/' || core[0] == '~' || strings.HasPrefix(core, "./") || strings.HasPrefix(core, "../") {
		return true
	}
	host := core
	if k := strings.IndexAny(host, "/?#"); k >= 0 {
		host = host[:k]
	}
	if k := strings.LastIndexByte(host, ':'); k >= 0 && k+1 < len(host) && allDigits(host[k+1:]) {
		host = host[:k]
	}
	dot := strings.LastIndexByte(host, '.')
	if dot <= 0 {
		return false
	}
	tld := host[dot+1:]
	if len(tld) < 2 || !allASCIILetters(tld) {
		return false
	}
	for label := range strings.SplitSeq(host[:dot], ".") {
		if label == "" {
			return false
		}
		for _, r := range label {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && !isMark(r) {
				return false
			}
		}
	}
	return true
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

func allASCIILetters(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i] | 0x20
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}
