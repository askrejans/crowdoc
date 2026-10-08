package layout

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// word is a run of text without spaces, the unit of layout analysis.
type word struct {
	text               string
	x0, y0, x1, y1     float64
	size               float64
	bold, italic, mono bool
	sup, sub           bool
	link               string
	block              int
	role               Role
	em                 float64 // OCR: the font size estimated from the box, 0 if unknown
}

func (w *word) w() float64  { return w.x1 - w.x0 }
func (w *word) h() float64  { return w.y1 - w.y0 }
func (w *word) cx() float64 { return (w.x0 + w.x1) / 2 }
func (w *word) cy() float64 { return (w.y0 + w.y1) / 2 }

var ligatures = strings.NewReplacer(
	"\uFB00", "ff", "\uFB01", "fi", "\uFB02", "fl", "\uFB03", "ffi", "\uFB04", "ffl",
	"\uFB05", "st", "\uFB06", "st",
	"\u00A0", " ", "\u2002", " ", "\u2003", " ", "\u2007", " ", "\u2008", " ",
	"\u2009", " ", "\u200A", " ", "\u202F", " ", "\u205F", " ", "\u3000", " ",
	"\u200B", "", "\uFEFF", "", "\u2060", "", "\u2061", "", "\u2062", "", "\u2063", "",
)

// cleanText normalises run text: ligature glyphs, odd spaces, control
// characters, canonical composition.
func cleanText(s string) string {
	s = ligatures.Replace(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			return ' '
		case r < 0x20 || (r >= 0x7F && r < 0xA0):
			return -1
		}
		return r
	}, s)
	if !norm.NFC.IsNormalString(s) {
		s = norm.NFC.String(s)
	}
	return s
}

func finite(v ...float64) bool {
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}

// makeWords turns runs into words: text is cleaned, multi-word runs are
// split with proportional boxes, runs outside the page are dropped and
// overprinted duplicates (a common way of faking bold) are merged.
func makeWords(p Page) []*word {
	out := make([]*word, 0, len(p.Runs))
	for _, r := range p.Runs {
		if !finite(r.X, r.Y, r.W, r.H, r.FontSize) || r.H <= 0 || r.W < 0 || r.Role == RoleArtifact {
			continue
		}
		if p.Width > 0 && p.Height > 0 && (r.X > p.Width || r.Y > p.Height || r.X+r.W < 0 || r.Y+r.H < 0) {
			continue
		}
		text := cleanText(r.Text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		size := r.FontSize
		em := 0.0
		if size <= 0 {
			// OCR boxes: estimate the em size from the height of the ink,
			// which depends on the letters (x-height, ascenders,
			// descenders).
			if ratio := inkRatio(text); ratio > 0 {
				em = r.H / ratio
			}
			size = em
			if size <= 0 {
				size = r.H
			}
		}
		base := word{size: size, em: em, bold: r.Bold, italic: r.Italic, mono: r.Mono, y0: r.Y, y1: r.Y + r.H, block: r.Block, role: r.Role}
		fields := strings.Fields(text)
		if len(fields) == 1 && !strings.ContainsRune(text, ' ') {
			w := base
			w.text, w.x0, w.x1 = text, r.X, r.X+r.W
			out = append(out, &w)
			continue
		}
		// Distribute the run width over its characters, spaces included.
		total := utf8.RuneCountInString(text)
		if total == 0 {
			continue
		}
		per := r.W / float64(total)
		for _, f := range splitKeepOffsets(text) {
			w := base
			w.text = f.s
			w.x0 = r.X + per*float64(f.start)
			w.x1 = w.x0 + per*float64(utf8.RuneCountInString(f.s))
			out = append(out, &w)
		}
	}
	return dedupe(out)
}

type field struct {
	s     string
	start int // rune offset
}

func splitKeepOffsets(s string) []field {
	var out []field
	start, i := -1, 0
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) {
			if start >= 0 {
				out = append(out, field{sb.String(), start})
				sb.Reset()
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
			sb.WriteRune(r)
		}
		i++
	}
	if start >= 0 {
		out = append(out, field{sb.String(), start})
	}
	return out
}

// dedupe drops words drawn twice at (almost) the same place; the survivor
// becomes bold when the copies were offset, as overprinting simulates bold.
func dedupe(ws []*word) []*word {
	if len(ws) < 2 {
		return ws
	}
	idx := make([]int, len(ws))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool {
		wa, wb := ws[idx[a]], ws[idx[b]]
		if wa.text != wb.text {
			return wa.text < wb.text
		}
		return wa.x0 < wb.x0
	})
	drop := make([]bool, len(ws))
	for k := 0; k < len(idx); k++ {
		a := ws[idx[k]]
		if drop[idx[k]] {
			continue
		}
		for j := k + 1; j < len(idx); j++ {
			b := ws[idx[j]]
			if b.text != a.text || b.x0-a.x0 > 1.5 {
				break
			}
			if drop[idx[j]] || math.Abs(b.y0-a.y0) > 1.5 {
				continue
			}
			drop[idx[j]] = true
			if math.Abs(b.x0-a.x0) > 0.05 || math.Abs(b.y0-a.y0) > 0.05 {
				a.bold = true
			}
		}
	}
	out := ws[:0]
	for i, w := range ws {
		if !drop[i] {
			out = append(out, w)
		}
	}
	return out
}

// inkRatio estimates the height of the ink of text as a fraction of the
// font size: about 0.5 for x-height letters, 0.72 with capitals,
// ascenders or accents, plus 0.22 with descenders. It returns 0 for text
// without letters or digits, whose height says nothing about the size.
func inkRatio(text string) float64 {
	top, bottom, known := 0.5, 0.0, false
	for _, r := range norm.NFD.String(text) {
		switch {
		case unicode.Is(unicode.Mn, r):
			if r == 0x327 || r == 0x328 || r == 0x323 || r == 0x326 || r == 0x331 {
				bottom = max(bottom, 0.15) // cedilla, ogonek and other marks below
			} else {
				top = max(top, 0.7)
			}
		case unicode.IsUpper(r), unicode.IsDigit(r), strings.ContainsRune("bdfhklt", r):
			top, known = 0.72, true
		case strings.ContainsRune("gpqy", r):
			bottom, known = 0.22, true
		case r == 'j':
			top, bottom, known = max(top, 0.68), 0.22, true
		case r == 'i':
			top, known = max(top, 0.68), true
		case unicode.IsLetter(r):
			known = true
		}
	}
	if !known {
		return 0
	}
	return top + bottom
}
