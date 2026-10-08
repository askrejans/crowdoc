package odt

import (
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// bibTypes maps text:bibliography-type values to CSL types.
var bibTypes = map[string]string{
	"article":       "article-journal",
	"book":          "book",
	"booklet":       "pamphlet",
	"inbook":        "chapter",
	"incollection":  "chapter",
	"inproceedings": "paper-conference",
	"conference":    "paper-conference",
	"proceedings":   "book",
	"phdthesis":     "thesis",
	"mastersthesis": "thesis",
	"techreport":    "report",
	"manual":        "manual",
	"misc":          "misc",
	"www":           "webpage",
	"unpublished":   "manuscript",
	"email":         "personal_communication",
	"journal":       "periodical",
}

// citation converts a text:bibliography-mark into a citation and records
// its bibliography entry.
func (r *reader) citation(n *node) ast.Inline {
	id := strings.TrimSpace(n.attr("text:identifier"))
	visible := strings.TrimSpace(collapseODF(n.textContent()))
	if id == "" {
		if visible == "" {
			return nil
		}
		return &ast.Text{Value: visible}
	}
	if visible == "" {
		visible = "[" + id + "]"
	}
	if !r.refIDs[id] {
		r.refIDs[id] = true
		r.doc.References = append(r.doc.References, bibReference(n, id))
	}
	return &ast.Cite{
		Items:    []ast.CiteItem{{Key: id}},
		Fallback: []ast.Inline{&ast.Text{Value: visible}},
	}
}

func bibReference(n *node, id string) ast.Reference {
	get := func(k string) string { return strings.TrimSpace(n.attr("text:" + k)) }
	typ := strings.ToLower(get("bibliography-type"))
	ref := ast.Reference{
		ID:        id,
		Type:      bibTypes[typ],
		Title:     get("title"),
		Publisher: get("publisher"),
		Edition:   get("edition"),
		Volume:    get("volume"),
		Page:      get("pages"),
		URL:       get("url"),
		ISBN:      get("isbn"),
		Note:      get("note"),
		Author:    splitNames(get("author")),
		Editor:    splitNames(get("editor")),

		PublisherPlace:  get("address"),
		CollectionTitle: get("series"),
	}
	if ref.Type == "" {
		ref.Type = "misc"
	}
	switch typ {
	case "article":
		ref.ContainerTitle = get("journal")
		ref.Issue = get("number")
	case "inbook", "incollection", "inproceedings", "conference":
		ref.ContainerTitle = get("booktitle")
		ref.Number = get("number")
	default:
		ref.ContainerTitle = get("journal")
		if ref.ContainerTitle == "" {
			ref.ContainerTitle = get("booktitle")
		}
		ref.Number = get("number")
	}
	switch typ {
	case "phdthesis":
		ref.Genre = "PhD thesis"
	case "mastersthesis":
		ref.Genre = "Master's thesis"
	case "techreport":
		ref.Genre = get("report-type")
	}
	if ref.Publisher == "" {
		for _, k := range []string{"school", "institution", "organizations"} {
			if v := get(k); v != "" {
				ref.Publisher = v
				break
			}
		}
	}
	if hp := get("howpublished"); hp != "" {
		if ref.Note != "" {
			ref.Note += "; "
		}
		ref.Note += hp
	}
	if y, err := strconv.Atoi(get("year")); err == nil && y > 0 {
		ref.Issued.Year = y
		ref.Issued.Month = monthNumber(get("month"))
	} else if v := get("year"); v != "" {
		ref.Issued.Literal = v
	}
	if len(ref.Author) == 0 && ref.Type != "webpage" {
		if org := get("organizations"); org != "" {
			ref.Author = []ast.Name{{Literal: org}}
		}
	}
	return ref
}

var months = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

func monthNumber(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	if v, err := strconv.Atoi(s); err == nil && v >= 1 && v <= 12 {
		return v
	}
	if len(s) >= 3 {
		return months[s[:3]]
	}
	return 0
}

// splitNames parses "Last, First; Last2, First2" or "First Last and First2
// Last2".
func splitNames(s string) []ast.Name {
	if s == "" {
		return nil
	}
	var parts []string
	if strings.Contains(s, ";") {
		parts = strings.Split(s, ";")
	} else {
		parts = strings.Split(s, " and ")
	}
	var out []ast.Name
	for _, p := range parts {
		if p = strings.Join(strings.Fields(p), " "); p == "" {
			continue
		}
		if family, given, ok := strings.Cut(p, ","); ok {
			out = append(out, ast.Name{Family: strings.TrimSpace(family), Given: strings.TrimSpace(given)})
			continue
		}
		if i := strings.LastIndexByte(p, ' '); i > 0 {
			out = append(out, ast.Name{Family: p[i+1:], Given: p[:i]})
			continue
		}
		out = append(out, ast.Name{Family: p})
	}
	return out
}
