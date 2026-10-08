package layout

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

func firstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

func lastRune(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

func hasLetter(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}

func startsLower(s string) bool {
	s = strings.TrimLeft(s, "\"'“‘«(„[")
	r := firstRune(s)
	return unicode.IsLower(r)
}

// endsSentence reports whether text ends with sentence-final punctuation
// (possibly followed by closing quotes or brackets).
func endsSentence(s string) bool {
	s = strings.TrimRight(s, " \"'”’»)]*")
	switch lastRune(s) {
	case '.', '!', '?', ':', ';', '…', '。':
		return true
	}
	return false
}

func endsHyphen(s string) bool {
	switch lastRune(s) {
	case '-', '‐', '­':
		return len(s) > 1
	}
	return false
}

var (
	bulletRunes = "•◦▪▫‣⁃∙·●○■□►▸▶➢➤✓✔❖◆◇–—*-❑❒➔→⦿⁌⁍"
	orderedRe   = regexp.MustCompile(`^(\(?)([0-9]{1,3}|[a-zA-Z]|[ivxlcdm]{1,6}|[IVXLCDM]{1,6})([.)])$`)
	parenRe     = regexp.MustCompile(`^\(([0-9]{1,3}|[a-zA-Z]|[ivxlcdm]{1,6})\)$`)
	numberingRe = regexp.MustCompile(`^(?:\d{1,3}(?:\.\d{1,3})*\.?|[IVXLC]{1,6}\.|[A-Z]\.(?:\d{1,3}\.?)*|§\s*\d+)$`)
	refLabelRe  = regexp.MustCompile(`^\[\d{1,4}\]$`)
)

// taskMarkers are check boxes that mark task-list items.
var taskMarkers = map[string]ast.TaskState{"☐": ast.TaskOpen, "□": ast.TaskOpen, "☑": ast.TaskDone, "☒": ast.TaskDone, "✅": ast.TaskDone}

// isBullet reports whether a word is a list bullet glyph.
func isBullet(s string) bool {
	if utf8.RuneCountInString(s) != 1 {
		return false
	}
	r := firstRune(s)
	if r >= 0xE000 && r <= 0xF8FF {
		return true // symbol-font bullets mapped to the private use area
	}
	return strings.ContainsRune(bulletRunes, r)
}

// marker describes a list marker at the start of a line.
type marker struct {
	ordered bool
	style   int // ast.NumberStyle value
	num     int
	text    string
	letter  string // raw letter/roman for ambiguity resolution
	x       float64
	textX   float64
	gap     float64 // space between marker and text
	words   int     // number of words making up the marker
	task    ast.TaskState
}

func romanValue(s string) int {
	vals := map[rune]int{'i': 1, 'v': 5, 'x': 10, 'l': 50, 'c': 100, 'd': 500, 'm': 1000}
	total, prev := 0, 0
	rs := []rune(strings.ToLower(s))
	for i := len(rs) - 1; i >= 0; i-- {
		v := vals[rs[i]]
		if v == 0 {
			return 0
		}
		if v < prev {
			total -= v
		} else {
			total += v
			prev = v
		}
	}
	return total
}

// parseMarker interprets a marker word ("1.", "a)", "(iv)", "•").
func parseMarker(s string) (marker, bool) {
	if t, ok := taskMarkers[s]; ok {
		return marker{text: s, task: t}, true
	}
	if isBullet(s) {
		return marker{text: s}, true
	}
	var body string
	if m := parenRe.FindStringSubmatch(s); m != nil {
		body = m[1]
	} else if m := orderedRe.FindStringSubmatch(s); m != nil {
		if m[1] == "(" {
			return marker{}, false
		}
		body = m[2]
	} else {
		return marker{}, false
	}
	mk := marker{ordered: true, text: s, letter: body}
	switch {
	case body[0] >= '0' && body[0] <= '9':
		n := 0
		for _, c := range body {
			n = n*10 + int(c-'0')
		}
		mk.num, mk.style = n, 0
	case len(body) == 1 && body != "i" && body != "I":
		if body[0] >= 'a' {
			mk.num, mk.style = int(body[0]-'a')+1, 1
		} else {
			mk.num, mk.style = int(body[0]-'A')+1, 2
		}
	default:
		v := romanValue(body)
		if v == 0 {
			return marker{}, false
		}
		mk.num = v
		if strings.ToLower(body) == body {
			mk.style = 3
		} else {
			mk.style = 4
		}
	}
	return mk, true
}

// numberingDepth returns the depth of a heading number ("2" → 1,
// "2.3" → 2, "IV." → 1, "A.1" → 2), or 0 when s is not a heading number.
func numberingDepth(s string) int {
	if !numberingRe.MatchString(s) {
		return 0
	}
	s = strings.TrimRight(s, ".")
	if strings.HasPrefix(s, "§") {
		return 1
	}
	return strings.Count(s, ".") + 1
}

var (
	figureLabelRe   = regexp.MustCompile(`(?i)^(?:figure|fig\.|figura|figur|abbildung|abb\.|attēls|att\.|рис\.|рисунок|kuva|joonis|pav\.|paveikslas|rysunek|rys\.|obrázek|obr\.|chart|graph|illustration|ilustrācija|diagram|diagramma|exhibit|image|picture|attēlā)\s*(?:\d+(?:[.\-–]\d+)*|[IVX]+)\s*[.:\-–—|]?\s*`)
	figureLabelLvRe = regexp.MustCompile(`(?i)^\d+(?:\.\d+)*\.?\s*(?:attēls|att\.)\s*[.:\-–—]?\s*`)
	tableLabelRe    = regexp.MustCompile(`(?i)^(?:table|tab\.|tabula|tabelle|tabla|tableau|tabel|taulukko|tabelis|lentelė|tabela|таблица|tabulka)\s*(?:\d+(?:[.\-–]\d+)*|[IVX]+)\s*[.:\-–—|]?\s*`)
	tableLabelLvRe  = regexp.MustCompile(`(?i)^\d+(?:\.\d+)*\.?\s*(?:tabula|tab\.)\s*[.:\-–—]?\s*`)
)

// captionLabel returns "figure" or "table" and the length of the label
// prefix when s starts with a caption label.
func captionLabel(s string) (string, int) {
	if m := figureLabelRe.FindStringIndex(s); m != nil {
		return "figure", m[1]
	}
	if m := figureLabelLvRe.FindStringIndex(s); m != nil {
		return "figure", m[1]
	}
	if m := tableLabelRe.FindStringIndex(s); m != nil {
		return "table", m[1]
	}
	if m := tableLabelLvRe.FindStringIndex(s); m != nil {
		return "table", m[1]
	}
	return "", 0
}

// wordKey normalises a word for frequency counting.
func wordKey(s string) string {
	s = strings.TrimFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' })
	s = strings.Trim(s, "-")
	return strings.ToLower(s)
}

// compoundPrefixes are first elements of hyphenated compounds that are
// rarely a hyphenation point inside a single word, so a line-final hyphen
// after them is kept when the document offers no other evidence.
var compoundPrefixes = map[string]bool{
	"self": true, "well": true, "non": true, "twenty": true, "thirty": true,
	"forty": true, "fifty": true, "sixty": true, "seventy": true, "eighty": true,
	"ninety": true,
}
