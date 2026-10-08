package cite

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

type taggedField struct {
	tag, val string
}

var risTypes = map[string]string{
	"JOUR": "article-journal", "JFULL": "article-journal", "EJOUR": "article-journal",
	"ABST": "article-journal", "INPR": "article-journal", "SER": "article-journal",
	"MGZN": "article-magazine", "NEWS": "article-newspaper",
	"BOOK": "book", "EBOOK": "book", "EDBOOK": "book", "CTLG": "book", "PAMP": "pamphlet",
	"CHAP": "chapter", "ECHAP": "chapter", "ENCYC": "entry-encyclopedia", "DICT": "entry-dictionary",
	"CONF": "paper-conference", "CPAPER": "paper-conference",
	"THES": "thesis", "RPRT": "report", "GOVDOC": "report",
	"ELEC": "webpage", "WEB": "webpage", "ICOMM": "webpage", "BLOG": "post-weblog",
	"GEN": "document", "DATA": "dataset", "DBASE": "dataset", "AGGR": "dataset",
	"COMP": "software", "PAT": "patent", "STAND": "standard",
	"UNPB": "manuscript", "MANSCPT": "manuscript",
	"STAT": "legislation", "BILL": "legislation", "LEGAL": "legislation", "CASE": "legal_case",
	"PCOMM": "personal_communication", "MAP": "map", "SLIDE": "speech",
}

// ParseRIS parses RIS records (TY … ER). Records without an ID tag get a
// generated "<family><year>" key.
func ParseRIS(data []byte) ([]ast.Reference, []string) {
	var (
		refs  []ast.Reference
		warns []string
		typ   string
		cur   []taggedField
		open  bool
		start int
	)
	gen := &keyGen{}
	ids := map[string]bool{}
	flush := func() {
		r := risToRef(typ, cur)
		if r.ID == "" || ids[r.ID] {
			if r.ID != "" {
				warns = append(warns, fmt.Sprintf("ris: line %d: duplicate ID %q; generating a new key", start, r.ID))
			}
			r.ID = gen.next(&r)
		}
		ids[r.ID] = true
		gen.reserve(r.ID)
		refs = append(refs, r)
		cur, open = nil, false
	}
	lines := strings.Split(string(bytes.TrimPrefix(data, utf8BOM)), "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		tag, val, ok := risLine(line)
		if !ok {
			if open && len(cur) > 0 && strings.TrimSpace(line) != "" {
				last := &cur[len(cur)-1]
				last.val = strings.TrimSpace(last.val + " " + strings.TrimSpace(line))
			}
			continue
		}
		switch tag {
		case "TY":
			if open {
				warns = append(warns, fmt.Sprintf("ris: line %d: record without ER", start))
				flush()
			}
			typ, open, start = strings.ToUpper(val), true, i+1
		case "ER":
			if open {
				flush()
			}
		default:
			if !open {
				warns = append(warns, fmt.Sprintf("ris: line %d: %s outside a record ignored", i+1, tag))
				continue
			}
			cur = append(cur, taggedField{tag, val})
		}
	}
	if open {
		warns = append(warns, fmt.Sprintf("ris: line %d: record without ER", start))
		flush()
	}
	return refs, warns
}

// risLine splits "TY  - JOUR" into tag and value.
func risLine(line string) (string, string, bool) {
	if len(line) < 3 {
		return "", "", false
	}
	for i := range 2 {
		c := line[i]
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return "", "", false
		}
	}
	rest := strings.TrimLeft(line[2:], " ")
	if !strings.HasPrefix(rest, "-") {
		return "", "", false
	}
	return line[:2], strings.TrimSpace(rest[1:]), true
}

func risToRef(typ string, fields []taggedField) ast.Reference {
	r := ast.Reference{Type: risTypes[typ]}
	if r.Type == "" {
		r.Type = "document"
	}
	var sp, ep, py, da, sn string
	containers := map[string]string{}
	for _, f := range fields {
		v := f.val
		if v == "" {
			continue
		}
		switch f.tag {
		case "ID":
			r.ID = v
		case "AU", "A1":
			r.Author = append(r.Author, parseNameString(v))
		case "A2", "ED":
			r.Editor = append(r.Editor, parseNameString(v))
		case "A4":
			r.Translator = append(r.Translator, parseNameString(v))
		case "TI", "T1":
			r.Title = firstNonEmpty(r.Title, v)
		case "CT":
			if r.Title == "" {
				r.Title = v
			}
		case "T2", "JF", "JO", "BT", "JA", "J2", "J1":
			if containers[f.tag] == "" {
				containers[f.tag] = v
			}
		case "T3":
			r.CollectionTitle = v
		case "ST":
			r.ShortTitle = v
		case "PY", "Y1":
			py = firstNonEmpty(py, v)
		case "DA":
			da = v
		case "Y2":
			r.Accessed = parseDateString(v)
		case "VL":
			r.Volume = v
		case "IS":
			r.Issue = v
		case "SP":
			sp = v
		case "EP":
			ep = v
		case "PB":
			r.Publisher = v
		case "CY", "PP":
			r.PublisherPlace = v
		case "DO":
			r.DOI = cleanDOI(v)
		case "UR", "LK":
			r.URL = firstNonEmpty(r.URL, v)
		case "SN":
			sn = v
		case "ET":
			r.Edition = v
		case "AB", "N2":
			r.Abstract = firstNonEmpty(r.Abstract, v)
		case "N1":
			r.Note = firstNonEmpty(r.Note, v)
		case "LA":
			r.Language = v
		case "M3":
			r.Genre = v
		case "M1":
			r.Number = v
		}
	}
	for _, t := range []string{"T2", "JF", "JO", "BT", "J2", "JA", "J1"} {
		if c := containers[t]; c != "" {
			r.ContainerTitle = c
			break
		}
	}
	if r.Title == "" && (typ == "BOOK" || typ == "EBOOK" || typ == "EDBOOK") && containers["BT"] != "" {
		r.Title, r.ContainerTitle = containers["BT"], ""
	}
	switch {
	case sp != "" && ep != "" && !strings.ContainsAny(sp, "-–"):
		r.Page = sp + "–" + ep
	default:
		r.Page = normalizeRange(sp)
	}
	r.Page = normalizeRange(r.Page)
	r.Issued = parseDateString(py)
	if d := parseDateString(da); d.Year != 0 && (r.Issued.Year == 0 || (r.Issued.Year == d.Year && d.Month != 0)) {
		r.Issued = d
	}
	switch r.Type {
	case "book", "chapter", "thesis", "report", "pamphlet":
		r.ISBN = sn
	default:
		r.ISSN = sn
	}
	if r.Type == "report" && r.Number == "" && r.Issue != "" {
		r.Number, r.Issue = r.Issue, ""
	}
	return r
}

// reserve marks an explicit id as used so generated keys do not collide.
func (g *keyGen) reserve(id string) {
	if g.used == nil {
		g.used = map[string]int{}
	}
	if g.used[id] == 0 {
		g.used[id] = 1
	}
}

// ParseNBIB parses PubMed MEDLINE (.nbib) records.
func ParseNBIB(data []byte) ([]ast.Reference, []string) {
	var (
		refs  []ast.Reference
		warns []string
		cur   []taggedField
	)
	gen := &keyGen{}
	flush := func() {
		if len(cur) == 0 {
			return
		}
		r := nbibToRef(cur)
		r.ID = gen.next(&r)
		refs = append(refs, r)
		cur = nil
	}
	for i, line := range strings.Split(string(bytes.TrimPrefix(data, utf8BOM)), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "      ") {
			if len(cur) > 0 {
				last := &cur[len(cur)-1]
				last.val += " " + strings.TrimSpace(line)
			}
			continue
		}
		if len(line) < 5 || line[4] != '-' {
			warns = append(warns, fmt.Sprintf("nbib: line %d: unrecognised line ignored", i+1))
			continue
		}
		tag := strings.TrimSpace(line[:4])
		if tag == "PMID" {
			flush()
		}
		cur = append(cur, taggedField{tag, strings.TrimSpace(line[5:])})
	}
	flush()
	return refs, warns
}

func nbibToRef(fields []taggedField) ast.Reference {
	r := ast.Reference{Type: "article-journal"}
	var au, fau, ed, fed []ast.Name
	var jt, ta, bti, pl string
	book := false
	for _, f := range fields {
		v := f.val
		switch f.tag {
		case "TI":
			r.Title = v
		case "BTI":
			bti = v
		case "AU":
			au = append(au, medlineName(v))
		case "FAU":
			fau = append(fau, parseNameString(v))
		case "CN":
			fau = append(fau, ast.Name{Literal: v})
			au = append(au, ast.Name{Literal: v})
		case "ED":
			ed = append(ed, medlineName(v))
		case "FED":
			fed = append(fed, parseNameString(v))
		case "DP":
			r.Issued = parseDateString(v)
		case "JT":
			jt = v
		case "TA":
			ta = v
		case "VI":
			r.Volume = v
		case "IP":
			r.Issue = v
		case "PG":
			r.Page = normalizeRange(expandPages(v))
		case "LID", "AID":
			if strings.HasSuffix(v, "[doi]") && r.DOI == "" {
				r.DOI = cleanDOI(strings.TrimSpace(strings.TrimSuffix(v, "[doi]")))
			}
		case "IS":
			if r.ISSN == "" {
				r.ISSN = strings.TrimSpace(strings.Split(v, "(")[0])
			}
		case "LA":
			r.Language = v
		case "AB":
			r.Abstract = v
		case "PB":
			r.Publisher = v
		case "PL":
			pl = v
		case "PT":
			if strings.EqualFold(v, "Book") || strings.EqualFold(v, "Book Chapter") {
				book = true
			}
		}
	}
	r.Title = strings.TrimSuffix(r.Title, ".")
	r.Author = au
	if len(fau) > 0 {
		r.Author = fau
	}
	r.Editor = ed
	if len(fed) > 0 {
		r.Editor = fed
	}
	r.ContainerTitle = firstNonEmpty(jt, ta)
	if book || bti != "" {
		switch {
		case r.Title != "" && bti != "":
			r.Type, r.ContainerTitle = "chapter", bti
		default:
			r.Type, r.Title, r.ContainerTitle = "book", firstNonEmpty(r.Title, bti), ""
		}
		r.PublisherPlace = pl
	}
	return r
}

// medlineName parses MEDLINE "Smith JA" names (family and initials).
func medlineName(s string) ast.Name {
	s = strings.TrimSpace(s)
	i := strings.LastIndexByte(s, ' ')
	if i < 0 {
		return ast.Name{Family: s}
	}
	fam, ini := s[:i], s[i+1:]
	var parts []string
	for _, r := range ini {
		parts = append(parts, string(r)+".")
	}
	return ast.Name{Family: fam, Given: strings.Join(parts, " ")}
}

// expandPages writes out MEDLINE's abbreviated page ranges: "284-7" → "284-287".
func expandPages(p string) string {
	a, b, ok := strings.Cut(p, "-")
	if !ok {
		return p
	}
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if len(b) < len(a) && allDigits(a) && allDigits(b) && b != "" {
		b = a[:len(a)-len(b)] + b
	}
	return a + "-" + b
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}
