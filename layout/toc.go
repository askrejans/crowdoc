package layout

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// tocEntryRe matches a table-of-contents line: a title, a leader and a
	// page number.
	tocEntryRe = regexp.MustCompile(`^\S.*?\s*(?:(?:\.\s?){3,}|(?:·\s?){3,}|…{2,}|_{3,}|-{4,})\s*(?:\d{1,4}|[ivxlcdm]{1,6}|[IVXLCDM]{1,6})$`)
	tocTitleRe = regexp.MustCompile(`(?i)^(?:table of contents|contents|saturs|satura rādītājs|satura radītājs|inhalt|inhaltsverzeichnis|table des matières|sommaire|índice|indice|sommario|spis treści|obsah|sisukord|turinys|sisällys|sisällysluettelo|innehåll|innehållsförteckning|indholdsfortegnelse|innhold|inhoud|inhoudsopgave|оглавление|содержание|зміст)$`)
	lofTitleRe = regexp.MustCompile(`(?i)^(?:list of figures|attēlu saraksts|abbildungsverzeichnis|table des figures|índice de figuras)$`)
	lotTitleRe = regexp.MustCompile(`(?i)^(?:list of tables|tabulu saraksts|tabellenverzeichnis|liste des tableaux|índice de tablas)$`)
)

// isTOCEntry reports whether a line is a contents entry: a leader and a
// page number, or (after a contents title) a page number set far apart
// from the title.
func (d *doc) isTOCEntry(l *line, afterTitle bool) bool {
	if l.role() == RoleTOC {
		return true
	}
	if tocEntryRe.MatchString(strings.TrimSpace(l.text())) {
		return true
	}
	if !afterTitle || len(l.words) < 2 {
		return false
	}
	last := l.words[len(l.words)-1]
	prev := l.words[len(l.words)-2]
	return isPageRef(last.text) && last.x0-prev.x1 > 2.5*l.size
}

func isPageRef(s string) bool {
	if s == "" || len(s) > 6 {
		return false
	}
	digits := strings.IndexFunc(s, func(r rune) bool { return !unicode.IsDigit(r) }) < 0
	return digits || strings.Trim(strings.ToLower(s), "ivxlcdm") == ""
}

// removeContents deletes tables of contents (and lists of figures and
// tables) from the flows: the typesetter generates its own. It records in
// the metadata that the document had them.
func (d *doc) removeContents() {
	for _, f := range d.flows {
		items := f.items
		kept := items[:0:0]
		for i := 0; i < len(items); {
			l := items[i].l
			titled := l != nil && i+1 < len(items) && items[i+1].l != nil && d.contentsTitle(l) != ""
			start := i
			if titled {
				start = i + 1
			}
			j := start
			entries := 0
			for j < len(items) && items[j].l != nil {
				if d.isTOCEntry(items[j].l, titled) {
					entries++
					j++
					continue
				}
				// A wrapped entry title: a line followed by an entry.
				if entries > 0 && j+1 < len(items) && items[j+1].l != nil && d.isTOCEntry(items[j+1].l, titled) &&
					items[j+1].l.y0-items[j].l.y1 < items[j].l.size {
					j++
					continue
				}
				break
			}
			if entries >= 2 || titled && entries >= 1 {
				kind := "toc"
				if titled {
					kind = d.contentsTitle(l)
				}
				d.markContents(kind)
				i = j
				continue
			}
			kept = append(kept, items[i])
			i++
		}
		f.items = kept
	}
}

// contentsTitle returns "toc", "lof" or "lot" for the title line of a
// contents list, or "".
func (d *doc) contentsTitle(l *line) string {
	text := strings.TrimSpace(strings.TrimRight(l.text(), ":"))
	if len(l.words) > 6 || l.size < d.body*0.95 {
		return ""
	}
	switch {
	case tocTitleRe.MatchString(text):
		return "toc"
	case lofTitleRe.MatchString(text):
		return "lof"
	case lotTitleRe.MatchString(text):
		return "lot"
	}
	return ""
}

func (d *doc) markContents(kind string) {
	switch kind {
	case "lof":
		d.meta.LOF = true
	case "lot":
		d.meta.LOT = true
	default:
		t := true
		d.meta.TOC = &t
	}
}
