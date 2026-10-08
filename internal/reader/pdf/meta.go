package pdf

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/layout"
)

// docInfo is the metadata stored in the file itself.
type docInfo struct {
	title, subject, description string
	authors, keywords           []string
	lang                        string
}

func (f *file) info() docInfo {
	var di docInfo
	if info, ok := f.resolve(f.trailer["Info"]).(dict); ok {
		get := func(k name) string {
			s, _ := f.resolve(info[k]).(pdfString)
			return cleanMeta(textString(s))
		}
		di.title = get("Title")
		di.subject = get("Subject")
		if a := get("Author"); a != "" {
			di.authors = splitAuthors(a)
		}
		if k := get("Keywords"); k != "" {
			di.keywords = splitKeywords(k)
		}
	}
	root, _ := f.resolve(f.trailer["Root"]).(dict)
	if l, ok := f.resolve(root["Lang"]).(pdfString); ok {
		di.lang = cleanMeta(textString(l))
	}
	if ms, ok := f.resolve(root["Metadata"]).(*stream); ok {
		if data, _, _, err := f.decodeStream(ms, false); err == nil && len(data) < 4<<20 {
			x := parseXMP(data)
			if di.title == "" {
				di.title = x.title
			}
			if len(x.authors) > 0 {
				di.authors = x.authors
			}
			if len(di.keywords) == 0 {
				di.keywords = x.keywords
			}
			if di.lang == "" {
				di.lang = x.lang
			}
			if di.subject == "" {
				di.description = x.description
			}
		}
	}
	return di
}

func cleanMeta(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0xFEFF {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func splitKeywords(s string) []string {
	var out []string
	for _, k := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// splitAuthors splits an author string on ";" and "," (unless the commas
// separate "Family, Given" pairs) and " and ".
func splitAuthors(s string) []string {
	parts := strings.Split(s, ";")
	if len(parts) == 1 {
		comma := strings.Split(s, ",")
		multiWord := 0
		for _, p := range comma {
			if len(strings.Fields(p)) >= 2 {
				multiWord++
			}
		}
		if len(comma) > 1 && multiWord == len(comma) {
			parts = comma
		}
	}
	var out []string
	for _, p := range parts {
		for _, q := range andSplit.Split(p, -1) {
			if q = strings.TrimSpace(q); q != "" {
				out = append(out, q)
			}
		}
	}
	return out
}

var andSplit = regexp.MustCompile(`\s+(?:and|&|un|und|et|y|i|ir|ja)\s+`)

type xmpInfo struct {
	title, description string
	authors, keywords  []string
	lang               string
}

// parseXMP reads Dublin Core fields from an XMP packet.
func parseXMP(data []byte) xmpInfo {
	var x xmpInfo
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	var stack []string
	var text strings.Builder
	inLi := false
	for {
		tok, err := dec.Token()
		if err == io.EOF || err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, t.Name.Local)
			if t.Name.Local == "li" {
				inLi = true
				text.Reset()
			}
			if t.Name.Local == "Keywords" {
				text.Reset()
			}
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			parent := ""
			for i := len(stack) - 2; i >= 0; i-- {
				if s := stack[i]; s != "Alt" && s != "Seq" && s != "Bag" && s != "li" {
					parent = s
					break
				}
			}
			v := cleanMeta(text.String())
			switch {
			case t.Name.Local == "li" && inLi:
				inLi = false
				switch parent {
				case "title":
					if x.title == "" {
						x.title = v
					}
				case "creator":
					if v != "" {
						x.authors = append(x.authors, v)
					}
				case "subject":
					if v != "" {
						x.keywords = append(x.keywords, splitKeywords(v)...)
					}
				case "language":
					if x.lang == "" {
						x.lang = v
					}
				case "description":
					if x.description == "" {
						x.description = v
					}
				}
			case t.Name.Local == "Keywords" && len(x.keywords) == 0:
				x.keywords = splitKeywords(v)
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return x
}

// junkTitle recognises titles that are file names or generator boilerplate
// rather than real titles.
var junkTitle = regexp.MustCompile(`(?i)(\.(docx?|odt|rtf|pdf|tex|dvi|ps|indd|qxd|pages|html?|txt|xlsx?|pptx?)$)|^(untitled|title|document\d*|doc\d*|slide \d+|presentation\d*|no title)$|^[\w-]+ - [^ ]+\.\w{2,4}$`)

func usableTitle(t string) bool {
	return t != "" && !junkTitle.MatchString(strings.TrimSpace(t)) && len([]rune(t)) <= 300
}

// mergeMeta combines file metadata with the hints recovered from the page
// layout. A title found on the first page wins because it was removed from
// the body text.
func mergeMeta(di docInfo, hint ast.Meta) ast.Meta {
	m := hint
	if m.Title == "" && usableTitle(di.title) {
		m.Title = di.title
	}
	if len(di.authors) > 0 {
		names := make([]ast.Author, 0, len(di.authors))
		for _, a := range di.authors {
			names = append(names, ast.Author{Name: a})
		}
		if len(m.Authors) == len(names) {
			for i := range names {
				names[i].Affiliations = m.Authors[i].Affiliations
				names[i].Email = m.Authors[i].Email
			}
		}
		if len(m.Authors) == 0 || len(m.Authors) == len(names) {
			m.Authors = names
		}
	}
	if di.subject != "" {
		m.Subject = di.subject
		if m.Summary == "" {
			m.Summary = di.subject
		}
	} else if di.description != "" {
		m.Summary = di.description
	}
	if len(m.Keywords) == 0 {
		m.Keywords = di.keywords
	}
	if di.lang != "" {
		m.Lang = di.lang
	}
	return m
}

// links reads URI link annotations of a page in display coordinates.
func (f *file) links(p pageInfo, toDisplay func(x, y float64) (float64, float64)) []layout.Link {
	annots, _ := f.resolve(p.d["Annots"]).(array)
	var out []layout.Link
	for _, a := range annots {
		if len(out) >= 1000 {
			break
		}
		ad, _ := f.resolve(a).(dict)
		if ad == nil || ad["Subtype"] != name("Link") {
			continue
		}
		act, _ := f.resolve(ad["A"]).(dict)
		uri, _ := f.resolve(act["URI"]).(pdfString)
		u := strings.TrimSpace(textString(uri))
		if u == "" || !safeURL(u) {
			continue
		}
		if strings.HasPrefix(strings.ToLower(u), "www.") {
			u = "https://" + u
		}
		r, ok := rectFrom(f.resolveArray(ad["Rect"]))
		if !ok {
			continue
		}
		x0, y0 := toDisplay(r.x0, r.y0)
		x1, y1 := toDisplay(r.x1, r.y1)
		out = append(out, layout.Link{X: min(x0, x1), Y: min(y0, y1), W: abs64(x1 - x0), H: abs64(y1 - y0), URL: u})
	}
	return out
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func safeURL(u string) bool {
	l := strings.ToLower(u)
	for _, p := range []string{"http://", "https://", "mailto:", "ftp://", "tel:", "doi:"} {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return strings.HasPrefix(l, "www.")
}
