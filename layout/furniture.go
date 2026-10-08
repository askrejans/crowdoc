package layout

import (
	"bytes"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// pageData is the working state of one page.
type pageData struct {
	idx    int
	w, h   float64
	words  []*word
	lines  []*line
	images []*pageImage
	links  []Link
}

type pageImage struct {
	x0, y0, x1, y1 float64
	data           []byte
	mediaType      string
	page           int
}

var pageNumberRe = regexp.MustCompile(`(?i)^(?:page|p\.|pg\.|lpp\.|lappuse|seite|s\.|str\.|стр\.|страница|psl\.|lk\.|sivu|sida|side|pagina|página|strona)?\s*[-–—(\[]?\s*(?:\d{1,4}|[ivxlcdm]{1,7})\s*[-–—)\]]?\s*(?:(?:/|of|no|iš|из|von|de|di|z|из|af|av)\s*\d{1,4})?\s*(?:\.\s*(?:lpp|lappuse|psl|lk|str|strana|oldal)\.?)?$`)

// isPageNumber reports whether s is a bare page number ("3", "- 3 -",
// "Page 3 of 9", "iv", "3. lpp.").
func isPageNumber(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 40 {
		return false
	}
	if strings.Trim(s, "0123456789") == "" {
		return len(s) <= 3
	}
	hasDigit := strings.IndexFunc(s, unicode.IsDigit) >= 0
	if !hasDigit {
		// Roman numerals alone, lower case only ("iv"), to avoid words.
		return len(s) <= 6 && strings.Trim(s, "ivxlcdm") == "" && strings.ToLower(s) == s
	}
	return pageNumberRe.MatchString(s)
}

// signature normalises furniture text for comparison across pages: case,
// digits and spacing are ignored.
func signature(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsDigit(r):
			sb.WriteByte('#')
		case unicode.IsSpace(r):
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

type furniturePiece struct {
	page  int
	words []*word
	sig   string
	top   bool
	y     float64
	text  string
	full  *line
}

// removeFurniture deletes running headers, footers, page numbers and
// repeated decorations (logos) from the pages. title holds the words of
// the document title, which may recur as a running head but is no
// furniture itself.
func removeFurniture(pages []*pageData, body float64, title []*word) {
	n := len(pages)
	var pieces []*furniturePiece
	for _, p := range pages {
		pieces = append(pieces, bandPieces(p, body)...)
	}
	bySig := map[string][]*furniturePiece{}
	for _, fp := range pieces {
		k := fp.sig
		if fp.top {
			k = "t:" + k
		} else {
			k = "b:" + k
		}
		bySig[k] = append(bySig[k], fp)
	}
	var titleText strings.Builder
	isTitle := map[*word]bool{}
	for _, w := range title {
		titleText.WriteString(w.text)
		isTitle[w] = true
	}
	titleSig := signature(titleText.String())
	remove := map[*word]bool{}
	for _, fp := range pieces {
		if isTitle[fp.words[0]] {
			continue
		}
		drop := false
		switch {
		case isPageNumber(fp.text):
			drop = true
		case len(fp.sig) >= 2 && fp.words[0].size <= body*1.15:
			// Running heads are set no larger than the text; a large line
			// repeated at the top of pages is a heading that opens each
			// page (a chapter or section per page).
			k := "b:" + fp.sig
			if fp.top {
				k = "t:" + fp.sig
			}
			pagesSeen := map[int]bool{}
			for _, o := range bySig[k] {
				if math.Abs(o.y-fp.y) <= 8 {
					pagesSeen[o.page] = true
				}
			}
			m := len(pagesSeen)
			drop = m >= 2 && (m >= 3 || m*4 >= n)
			// The document title repeated as a running head or foot.
			if !drop && len(titleSig) > 8 && len(fp.sig) >= 8 && strings.Contains(titleSig, fp.sig) && fp.words[0].size < body*1.2 {
				drop = true
			}
		}
		if !drop && (n > 1 || hasPageNumber(fp.full)) && isRunningHead(fp.full, pages[fp.page], body) {
			drop = true
		}
		if drop {
			for _, w := range fp.words {
				remove[w] = true
			}
		}
	}
	if len(remove) > 0 {
		for _, p := range pages {
			p.dropWords(remove)
		}
	}
	removeRepeatedImages(pages)
}

// hasPageNumber reports whether one of the line's pieces is a page number.
func hasPageNumber(l *line) bool {
	for _, ws := range splitAtGaps(l.words, 2.0) {
		if isPageNumber(wordsText(ws)) {
			return true
		}
	}
	return false
}

// bandPieces returns the text pieces of the first and last lines of a page
// that lie in the top or bottom margin band.
func bandPieces(p *pageData, body float64) []*furniturePiece {
	var out []*furniturePiece
	if len(p.lines) == 0 {
		return nil
	}
	topLimit := p.h * 0.14
	bottomLimit := p.h * 0.86
	add := func(l *line, top bool) {
		for _, ws := range splitAtGaps(l.words, 2.0) {
			if ws[0].role != RoleAuto {
				// Tagged content; furniture is tagged as an artifact.
				continue
			}
			text := wordsText(ws)
			out = append(out, &furniturePiece{page: p.idx, words: ws, sig: signature(text), top: top, y: ws[0].y0, text: text, full: l})
		}
	}
	for i := 0; i < len(p.lines) && i < 3; i++ {
		if l := p.lines[i]; l.y1 <= topLimit {
			add(l, true)
		}
	}
	for i := len(p.lines) - 1; i >= 0 && i >= len(p.lines)-3; i-- {
		if l := p.lines[i]; l.y0 >= bottomLimit {
			add(l, false)
		}
	}
	return out
}

// isRunningHead recognises the classic running head: the first or last
// line of a page, split into a left and a right part far apart, no larger
// than body text, and set off from the text block.
func isRunningHead(l *line, p *pageData, body float64) bool {
	parts := splitAtGaps(l.words, 2.0)
	if len(parts) < 2 || len(parts) > 3 || l.size > body*1.05 {
		return false
	}
	if parts[1][0].x0-parts[0][len(parts[0])-1].x1 < 0.25*p.w {
		return false
	}
	idx := -1
	for i, x := range p.lines {
		if x == l {
			idx = i
		}
	}
	switch idx {
	case 0:
		return len(p.lines) == 1 || p.lines[1].y0-l.y1 > 0.8*l.size
	case len(p.lines) - 1:
		return idx == 0 || l.y0-p.lines[idx-1].y1 > 0.8*l.size
	}
	return false
}

// splitAtGaps splits sorted words at horizontal gaps wider than k times
// the font size.
func splitAtGaps(ws []*word, k float64) [][]*word {
	if len(ws) == 0 {
		return nil
	}
	var out [][]*word
	start := 0
	for i := 1; i < len(ws); i++ {
		if ws[i].x0-ws[i-1].x1 > k*max(ws[i].size, ws[i-1].size) {
			out = append(out, ws[start:i])
			start = i
		}
	}
	return append(out, ws[start:])
}

func (p *pageData) dropWords(remove map[*word]bool) {
	words := p.words[:0]
	for _, w := range p.words {
		if !remove[w] {
			words = append(words, w)
		}
	}
	p.words = words
	lines := p.lines[:0]
	for _, l := range p.lines {
		ws := l.words[:0]
		for _, w := range l.words {
			if !remove[w] {
				ws = append(ws, w)
			}
		}
		if len(ws) == 0 {
			continue
		}
		if len(ws) != len(l.words) {
			l.words = ws
			l.recompute()
		}
		lines = append(lines, l)
	}
	p.lines = lines
}

// removeRepeatedImages drops images repeated at the same place on several
// pages (letterhead logos, decorations).
func removeRepeatedImages(pages []*pageData) {
	n := len(pages)
	if n < 2 {
		return
	}
	type key struct{ x, y, w, h int }
	count := map[key][]*pageImage{}
	for _, p := range pages {
		for _, im := range p.images {
			k := key{int(im.x0 / 4), int(im.y0 / 4), int((im.x1 - im.x0) / 4), int((im.y1 - im.y0) / 4)}
			count[k] = append(count[k], im)
		}
	}
	drop := map[*pageImage]bool{}
	for _, ims := range count {
		if len(ims) < 2 || len(ims)*4 < n {
			continue
		}
		same := 1
		for _, im := range ims[1:] {
			if bytes.Equal(im.data, ims[0].data) {
				same++
			}
		}
		if same >= 2 && (same >= 3 || same*4 >= n) {
			for _, im := range ims {
				if bytes.Equal(im.data, ims[0].data) {
					drop[im] = true
				}
			}
		}
	}
	if len(drop) == 0 {
		return
	}
	for _, p := range pages {
		ims := p.images[:0]
		for _, im := range p.images {
			if !drop[im] {
				ims = append(ims, im)
			}
		}
		p.images = ims
	}
}
