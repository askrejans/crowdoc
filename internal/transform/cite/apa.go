package cite

import (
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// apaEntry formats a reference list entry in APA 7th edition style:
//
//	Smith, J. A., & Lee, K. (2020). Title of article. Journal, 12(3), 45–67. https://doi.org/…
func apaEntry(f *fmtr, e *entry) []ast.Inline {
	b := f.builder()
	if e.kind == kLegal && len(e.names) == 0 {
		f.apaLegal(b, e)
		return b.result()
	}
	titleFirst := len(e.names) == 0
	if titleFirst {
		f.apaTitle(b, e)
	} else {
		b.text(f.apaNames(e.names, e.others))
		switch e.role {
		case "editor":
			b.text(" (" + f.termN("editor", len(e.names)) + ")")
		case "translator":
			b.text(" (" + f.termN("translator", len(e.names)) + ")")
		}
	}
	b.punct(".")
	b.space()
	b.text("(" + f.apaDate(e) + ")")
	b.punct(".")
	if !titleFirst && e.ref.Title != "" {
		b.space()
		f.apaTitle(b, e)
		b.punct(".")
	}
	f.apaSource(b, e)
	f.apaLink(b, e)
	return b.result()
}

// apaNames renders the author list: up to 20 names, then the first 19,
// an ellipsis and the last.
func (f *fmtr) apaNames(ns []ast.Name, others bool) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = nameInvertedInitials(n, true)
	}
	n := len(parts)
	switch {
	case others:
		return strings.Join(parts, ", ") + ", " + f.term("et-al")
	case n == 1:
		return parts[0]
	case n == 2:
		return parts[0] + ", & " + parts[1]
	case n <= 20:
		return strings.Join(parts[:n-1], ", ") + ", & " + parts[n-1]
	}
	return strings.Join(parts[:19], ", ") + ", … " + parts[n-1]
}

// apaNatList renders editors or translators as "A. Editor & B. Editor".
func (f *fmtr) apaNatList(ns []ast.Name) (string, int) {
	ns, others := splitOthers(ns)
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = nameNaturalInitials(n)
	}
	n := len(parts)
	switch {
	case n == 0:
		return "", 0
	case others:
		return strings.Join(parts, ", ") + " " + f.term("et-al"), n + 1
	case n == 1:
		return parts[0], 1
	case n == 2:
		return parts[0] + " & " + parts[1], 2
	}
	return strings.Join(parts[:n-1], ", ") + ", & " + parts[n-1], n
}

func (f *fmtr) apaDate(e *entry) string {
	y := f.yearText(e)
	d := e.ref.Issued
	if d.Year == 0 || d.Month == 0 {
		return y
	}
	switch e.kind {
	case kMagazine, kNewspaper, kBlogPost, kWebpage:
		return y + ", " + f.dayMonth(d, false)
	case kConference:
		if e.ref.ContainerTitle == "" {
			return y + ", " + f.dayMonth(d, false)
		}
	}
	return y
}

// apaTitle writes the title element: the title (italic for standalone
// works) with its parenthetical details and bracketed description.
func (f *fmtr) apaTitle(b *builder, e *entry) {
	r := e.ref
	if e.standalone() {
		b.emph(r.Title)
	} else {
		b.text(r.Title)
	}
	var roles []string
	if e.role == "author" && !e.inContainer() {
		if eds, n := f.apaNatList(r.Editor); eds != "" {
			roles = append(roles, eds+", "+f.termN("editor", n))
		}
	}
	if e.role != "translator" {
		if tr, n := f.apaNatList(r.Translator); tr != "" {
			roles = append(roles, tr+", "+f.termN("translator", n))
		}
	}
	var info []string
	if !e.inContainer() {
		info = append(info, f.edition(r.Edition))
		if r.Volume != "" && (e.kind == kBook || e.kind == kReport) {
			info = append(info, f.labelled("volume", r.Volume))
		}
	}
	if r.Number != "" {
		switch e.kind {
		case kReport:
			if r.Genre != "" {
				info = append(info, r.Genre+" "+f.term("number")+" "+r.Number)
			} else {
				info = append(info, f.term("report-no")+" "+r.Number)
			}
		case kPatent:
			info = append(info, f.term("patent")+" "+f.term("number")+" "+r.Number)
		case kStandard, kGeneric, kPreprint, kDataset:
			info = append(info, r.Number)
		}
	}
	if (e.kind == kDataset || e.kind == kSoftware) && r.Version != "" {
		info = append(info, f.term("version")+" "+r.Version)
	}
	if paren := joinNonEmpty("; ", append(roles, joinNonEmpty(", ", info...))...); paren != "" {
		b.space()
		b.text("(" + paren + ")")
	}
	if d := f.apaDescription(e); d != "" {
		b.space()
		b.text("[" + d + "]")
	}
}

func (f *fmtr) apaDescription(e *entry) string {
	r := e.ref
	switch e.kind {
	case kThesis:
		return joinNonEmpty(", ", f.thesisLabel(r.Genre), r.Publisher)
	case kDataset:
		return firstNonEmpty(r.Genre, f.term("dataset"))
	case kSoftware:
		return firstNonEmpty(r.Genre, f.term("software"))
	case kManuscript:
		return firstNonEmpty(r.Genre, f.term("manuscript"))
	case kConference:
		if r.ContainerTitle == "" {
			return firstNonEmpty(r.Genre, f.term("presentation"))
		}
	case kPreprint:
		return firstNonEmpty(r.Genre, f.term("preprint"))
	case kGeneric:
		return firstNonEmpty(r.Genre, r.Medium)
	}
	return ""
}

// apaSource writes where the work appeared: periodical, edited book,
// website or publisher.
func (f *fmtr) apaSource(b *builder, e *entry) {
	r := e.ref
	switch {
	case e.inContainer() && (e.kind == kJournal || e.kind == kMagazine || e.kind == kNewspaper || e.kind == kBlogPost):
		b.space()
		b.emph(r.ContainerTitle)
		if r.Volume != "" {
			b.text(", ")
			b.emph(r.Volume)
		}
		if r.Issue != "" {
			if r.Volume == "" {
				b.text(", ")
			}
			b.text("(" + r.Issue + ")")
		}
		if r.Page != "" {
			b.text(", " + f.pageRange(r.Page))
		}
		b.punct(".")
		return
	case e.inContainer():
		b.space()
		b.text(f.term("in") + " ")
		if eds, n := f.apaNatList(r.Editor); eds != "" {
			b.text(eds + " (" + f.termN("editor", n) + "), ")
		}
		b.emph(r.ContainerTitle)
		vol := ""
		if r.Volume != "" {
			vol = f.labelled("volume", r.Volume)
		}
		if info := joinNonEmpty(", ", f.edition(r.Edition), vol, f.pagesLabelled(r.Page)); info != "" {
			b.text(" (" + info + ")")
		}
		b.punct(".")
	case e.kind == kWebpage:
		if site := r.ContainerTitle; site != "" && !e.sameAsAuthor(site) {
			b.space()
			b.text(site)
			b.punct(".")
		}
		return
	case e.kind == kConference:
		if ev := joinNonEmpty(", ", r.Event, r.EventPlace); ev != "" {
			b.space()
			b.text(ev)
			b.punct(".")
		}
		return
	case e.kind == kThesis:
		return // the institution is part of the bracketed description
	}
	if pub := r.Publisher; pub != "" && !e.sameAsAuthor(pub) {
		b.space()
		b.text(pub)
		b.punct(".")
	}
}

func (f *fmtr) apaLink(b *builder, e *entry) {
	r := e.ref
	switch {
	case r.DOI != "":
		u := doiURL(r.DOI)
		b.space()
		b.link(u, u)
	case r.URL != "":
		b.space()
		if r.Issued.IsZero() && !r.Accessed.IsZero() {
			s := f.term("retrieved") + " " + f.accessedDate(r.Accessed, false) + ","
			if from := f.term("from"); from != "" {
				s += " " + from
			}
			b.text(s + " ")
		}
		b.link(r.URL, r.URL)
	}
}

// apaLegal formats legislation: "Name of Act, Number (Year). URL".
func (f *fmtr) apaLegal(b *builder, e *entry) {
	r := e.ref
	b.text(r.Title)
	if r.Number != "" {
		b.text(", " + r.Number)
	} else if r.ContainerTitle != "" {
		b.text(", " + r.ContainerTitle)
	}
	b.space()
	b.text("(" + f.yearText(e) + ")")
	b.punct(".")
	f.apaLink(b, e)
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
