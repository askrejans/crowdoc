package html

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
)

// readMeta fills m from <html lang>, <title> and <meta> tags (plain,
// Dublin Core, scholarly citation_* tags, Open Graph). It reports
// whether the title came from the <title> element.
func readMeta(root *html.Node, m *ast.Meta) (fromTitleTag bool) {
	if h := findElem(root, "html", 0); h != nil {
		m.Lang = strings.TrimSpace(attr(h, "lang"))
		if m.Lang == "" {
			m.Lang = strings.TrimSpace(attr(h, "xml:lang"))
		}
	}
	head := findElem(root, "head", 0)
	if head == nil {
		return false
	}
	var titleTag string
	vals := map[string][]string{}
	var citeAuthors []ast.Author
	for n := head.FirstChild; n != nil; n = n.NextSibling {
		switch {
		case isElem(n, "title") && titleTag == "":
			titleTag = normSpace(textContent(n))
		case isElem(n, "meta"):
			key := strings.ToLower(strings.TrimSpace(attr(n, "name")))
			if key == "" {
				key = strings.ToLower(strings.TrimSpace(attr(n, "property")))
			}
			if key == "" {
				key = strings.ToLower(strings.TrimSpace(attr(n, "http-equiv")))
			}
			val := normSpace(attr(n, "content"))
			if key == "" || val == "" {
				continue
			}
			switch key {
			case "citation_author":
				citeAuthors = append(citeAuthors, ast.Author{Name: displayName(val)})
			case "citation_author_institution":
				if k := len(citeAuthors); k > 0 {
					citeAuthors[k-1].Affiliations = append(citeAuthors[k-1].Affiliations, val)
				}
			case "citation_author_email":
				if k := len(citeAuthors); k > 0 {
					citeAuthors[k-1].Email = val
				}
			case "citation_author_orcid":
				if k := len(citeAuthors); k > 0 {
					citeAuthors[k-1].ORCID = strings.TrimPrefix(strings.TrimPrefix(val, "https://orcid.org/"), "http://orcid.org/")
				}
			}
			vals[key] = append(vals[key], val)
		}
	}
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := vals[k]; len(v) > 0 {
				return v[0]
			}
		}
		return ""
	}

	if m.Title = first("citation_title", "dc.title", "dcterms.title"); m.Title == "" {
		if titleTag != "" {
			m.Title, fromTitleTag = titleTag, true
		} else {
			m.Title = first("og:title", "twitter:title")
		}
	}

	switch {
	case len(citeAuthors) > 0:
		m.Authors = citeAuthors
	case len(vals["dc.creator"])+len(vals["dcterms.creator"]) > 0:
		for _, v := range append(vals["dc.creator"], vals["dcterms.creator"]...) {
			m.Authors = append(m.Authors, ast.Author{Name: displayName(v)})
		}
	default:
		for _, v := range vals["author"] {
			for _, a := range strings.Split(v, ";") {
				if a = strings.TrimSpace(a); a != "" {
					m.Authors = append(m.Authors, ast.Author{Name: a})
				}
			}
		}
		if len(m.Authors) == 0 {
			if a := first("article:author"); a != "" && !strings.Contains(a, "://") {
				m.Authors = []ast.Author{{Name: a}}
			}
		}
	}

	m.Summary = first("description", "dc.description", "dcterms.description", "dcterms.abstract", "citation_abstract", "og:description", "twitter:description")
	m.Subject = first("subject")
	seen := map[string]bool{}
	for _, k := range []string{"keywords", "citation_keywords", "dc.subject", "dcterms.subject", "news_keywords"} {
		for _, v := range vals[k] {
			for _, kw := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' }) {
				kw = strings.TrimSpace(kw)
				if kw != "" && !seen[strings.ToLower(kw)] {
					seen[strings.ToLower(kw)] = true
					m.Keywords = append(m.Keywords, kw)
				}
			}
		}
	}
	m.Date = normDate(first("citation_publication_date", "citation_date", "dc.date", "dcterms.date",
		"dcterms.issued", "dcterms.created", "date", "article:published_time", "citation_online_date"))
	if m.Lang == "" {
		m.Lang = first("dc.language", "dcterms.language", "content-language", "citation_language", "language")
		if m.Lang == "" {
			m.Lang = strings.ReplaceAll(first("og:locale"), "_", "-")
		}
	}
	m.Organization = first("dc.publisher", "dcterms.publisher", "citation_publisher",
		"citation_technical_report_institution", "citation_dissertation_institution")
	extra := map[string]string{
		"journal": first("citation_journal_title", "citation_conference_title", "prism.publicationname"),
		"doi":     first("citation_doi", "dc.identifier.doi", "prism.doi"),
		"volume":  first("citation_volume", "prism.volume"),
		"issue":   first("citation_issue", "prism.number"),
		"pages":   joinPages(first("citation_firstpage", "prism.startingpage"), first("citation_lastpage", "prism.endingpage")),
		"issn":    first("citation_issn", "prism.issn"),
		"isbn":    first("citation_isbn"),
	}
	for k, v := range extra {
		if v == "" {
			continue
		}
		if m.Extra == nil {
			m.Extra = map[string]any{}
		}
		m.Extra[k] = v
	}
	return fromTitleTag
}

// displayName turns "Family, Given" into "Given Family".
func displayName(s string) string {
	s = normSpace(s)
	if strings.Count(s, ",") != 1 {
		return s
	}
	family, given, _ := strings.Cut(s, ",")
	family, given = strings.TrimSpace(family), strings.TrimSpace(given)
	if family == "" || given == "" {
		return s
	}
	return given + " " + family
}

func joinPages(first, last string) string {
	switch {
	case first == "":
		return ""
	case last == "" || last == first:
		return first
	}
	return first + "–" + last
}

// normDate keeps the date part of timestamps and turns 2021/03/04 into
// 2021-03-04.
func normDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 && s[4] == '/' && s[7] == '/' {
		s = strings.ReplaceAll(s[:10], "/", "-") + s[10:]
	}
	if len(s) > 10 && s[4] == '-' && s[7] == '-' && (s[10] == 'T' || s[10] == ' ') {
		s = s[:10]
	}
	return s
}
