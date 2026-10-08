package cite

import (
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// chicagoEntry formats a reference list entry in Chicago 17th edition
// author-date style:
//
//	Smith, John A., and Kim Lee. 2020. “Title.” Journal 12 (3): 45–67. https://doi.org/….
func chicagoEntry(f *fmtr, e *entry) []ast.Inline {
	b := f.builder()
	titleFirst := len(e.names) == 0
	if titleFirst {
		f.chicagoTitle(b, e)
	} else {
		b.text(f.chicagoNames(e.names, e.others, true))
		switch e.role {
		case "editor":
			b.text(", " + f.termN("editor", len(e.names)))
		case "translator":
			b.text(", " + f.termN("translator", len(e.names)))
		}
	}
	b.punct(".")
	b.space()
	b.text(f.yearText(e))
	b.punct(".")
	if !titleFirst && e.ref.Title != "" {
		b.space()
		f.chicagoTitle(b, e)
		b.punct(".")
	}
	f.chicagoSource(b, e)
	f.chicagoLink(b, e)
	return b.result()
}

// chicagoNames renders "Smith, John A., Kim Lee, and Mary Brown"; more
// than ten names are cut to seven followed by "et al.".
func (f *fmtr) chicagoNames(ns []ast.Name, others, invertFirst bool) string {
	limit := len(ns)
	etal := others
	if limit > 10 {
		limit, etal = 7, true
	}
	parts := make([]string, limit)
	for i := range limit {
		if i == 0 && invertFirst {
			parts[i] = nameInvertedFull(ns[i])
		} else {
			parts[i] = nameNaturalFull(ns[i])
		}
	}
	if etal {
		if limit == 1 && !invertFirst {
			return parts[0] + " " + f.term("et-al")
		}
		return strings.Join(parts, ", ") + ", " + f.term("et-al")
	}
	and := f.term("and")
	switch limit {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		if invertFirst {
			return parts[0] + ", " + and + " " + parts[1]
		}
		return parts[0] + " " + and + " " + parts[1]
	}
	return strings.Join(parts[:limit-1], ", ") + ", " + and + " " + parts[limit-1]
}

func (f *fmtr) chicagoTitle(b *builder, e *entry) {
	t := e.ref.Title
	switch {
	case t == "":
	case e.inContainer(), e.kind == kThesis, e.kind == kManuscript, e.kind == kWebpage, e.kind == kConference:
		f.quote(b, t)
	case e.kind == kLegal, e.kind == kPatent:
		b.text(t)
	default:
		b.emph(t)
	}
}

// sentence writes s as its own sentence.
func sentence(b *builder, s string) {
	if s == "" {
		return
	}
	b.space()
	b.text(s)
	b.punct(".")
}

func (f *fmtr) chicagoSource(b *builder, e *entry) {
	r := e.ref
	switch {
	case e.inContainer() && e.kind == kJournal:
		b.space()
		b.emph(r.ContainerTitle)
		switch {
		case r.Volume != "" && r.Issue != "":
			b.text(" " + r.Volume + " (" + r.Issue + ")")
		case r.Volume != "":
			b.text(" " + r.Volume)
		case r.Issue != "":
			b.text(", " + f.labelled("issue", r.Issue))
		}
		if p := f.pageRange(r.Page); p != "" {
			if r.Volume != "" || r.Issue != "" {
				b.text(": " + p)
			} else {
				b.text(", " + p)
			}
		}
		b.punct(".")
		return
	case e.inContainer() && (e.kind == kMagazine || e.kind == kNewspaper || e.kind == kBlogPost):
		b.space()
		b.emph(r.ContainerTitle)
		if r.Issued.Month > 0 {
			b.text(", " + f.date(r.Issued, false))
		}
		if p := f.pageRange(r.Page); p != "" {
			b.text(", " + p)
		}
		b.punct(".")
		return
	case e.inContainer():
		b.space()
		b.text(f.term("in") + " ")
		b.emph(r.ContainerTitle)
		if ed := f.edition(r.Edition); ed != "" {
			b.text(", " + ed)
		}
		if eds, _ := splitOthers(r.Editor); len(eds) > 0 {
			b.text(", " + f.term("edited-by") + " " + f.chicagoNames(eds, false, false))
		}
		if tr, _ := splitOthers(r.Translator); len(tr) > 0 && e.role != "translator" {
			b.text(", " + f.term("translated-by") + " " + f.chicagoNames(tr, false, false))
		}
		if r.Volume != "" {
			b.text(", " + f.labelled("volume", r.Volume))
		}
		if p := f.pageRange(r.Page); p != "" {
			b.text(", " + p)
		}
		b.punct(".")
		sentence(b, placePublisher(r.PublisherPlace, r.Publisher))
		return
	}
	switch e.kind {
	case kConference:
		if ev := joinNonEmpty(", ", r.Event, r.EventPlace); ev != "" {
			sentence(b, capFirst(f.term("presented-at"))+" "+ev)
		}
		return
	case kThesis:
		sentence(b, joinNonEmpty(", ", f.thesisLabel(r.Genre), r.Publisher))
		return
	case kWebpage:
		sentence(b, r.ContainerTitle)
		if r.Issued.Month > 0 {
			sentence(b, f.date(r.Issued, false))
		}
		return
	case kManuscript:
		sentence(b, joinNonEmpty(", ", firstNonEmpty(r.Genre, f.term("manuscript")), r.Publisher))
		return
	}
	if e.role == "author" {
		if eds, _ := splitOthers(r.Editor); len(eds) > 0 {
			sentence(b, capFirst(f.term("edited-by"))+" "+f.chicagoNames(eds, false, false))
		}
	}
	if tr, _ := splitOthers(r.Translator); len(tr) > 0 && e.role != "translator" {
		sentence(b, capFirst(f.term("translated-by"))+" "+f.chicagoNames(tr, false, false))
	}
	sentence(b, f.edition(r.Edition))
	if r.Volume != "" {
		sentence(b, capFirst(f.labelled("volume", r.Volume)))
	}
	sentence(b, r.CollectionTitle)
	switch e.kind {
	case kReport:
		if r.Number != "" {
			sentence(b, joinNonEmpty(" ", firstNonEmpty(r.Genre, f.term("report")), r.Number))
		} else {
			sentence(b, r.Genre)
		}
	case kPatent:
		sentence(b, joinNonEmpty(" ", f.term("patent"), r.Number))
	case kDataset, kSoftware:
		if r.Version != "" {
			sentence(b, f.term("version")+" "+r.Version)
		}
		sentence(b, r.Genre)
	case kStandard, kLegal:
		sentence(b, r.Number)
	case kGeneric, kPreprint:
		sentence(b, firstNonEmpty(r.Genre, r.Medium))
		sentence(b, r.Number)
	}
	sentence(b, placePublisher(r.PublisherPlace, r.Publisher))
}

func (f *fmtr) chicagoLink(b *builder, e *entry) {
	r := e.ref
	if r.DOI == "" && r.URL != "" && r.Issued.IsZero() && !r.Accessed.IsZero() {
		sentence(b, capFirst(f.term("accessed"))+" "+f.accessedDate(r.Accessed, false))
	}
	u := r.URL
	if r.DOI != "" {
		u = doiURL(r.DOI)
	}
	if u != "" {
		b.space()
		b.link(u, u)
		b.punct(".")
	}
}
