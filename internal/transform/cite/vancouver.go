package cite

import (
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// vancouverEntry formats a numbered reference following the NLM
// recommendations (Vancouver style):
//
//	Smith JA, Lee K. Title of article. Journal. 2020 Mar;12(3):45-67. doi:10.…
func vancouverEntry(f *fmtr, e *entry) []ast.Inline {
	r := e.ref
	b := f.builder()
	if len(e.names) > 0 {
		b.text(f.vancouverNames(e.names, e.others))
		if e.role == "editor" {
			b.text(", " + f.termN("editor", len(e.names)))
		}
		b.punct(".")
	}
	if r.Title != "" {
		b.space()
		b.text(r.Title)
		if m := f.vancouverMedium(e); m != "" {
			b.text(" [" + m + "]")
		}
		b.punct(".")
	}
	switch {
	case e.inContainer() && (e.kind == kJournal || e.kind == kMagazine || e.kind == kNewspaper || e.kind == kBlogPost):
		sentence(b, r.ContainerTitle)
		s := f.vancouverDate(r.Issued, e.kind != kJournal)
		if r.Volume != "" {
			s += ";" + r.Volume
		}
		if r.Issue != "" {
			s += "(" + r.Issue + ")"
		}
		if p := f.pageRange(r.Page); p != "" {
			s += ":" + p
		}
		sentence(b, strings.TrimPrefix(s, ";"))
	case e.inContainer():
		b.space()
		b.text(f.term("in") + " ")
		if eds, others := splitOthers(r.Editor); len(eds) > 0 {
			b.text(f.vancouverNames(eds, others) + ", " + f.termN("editor", len(eds)))
			b.punct(".")
			b.text(" ")
		}
		b.text(r.ContainerTitle)
		b.punct(".")
		sentence(b, f.edition(r.Edition))
		f.vancouverPublication(b, e)
		if p := f.pageRange(r.Page); p != "" {
			sentence(b, f.labelled("page", p))
		}
	default:
		sentence(b, f.edition(r.Edition))
		if e.kind == kConference {
			sentence(b, joinNonEmpty("; ", r.Event, r.EventPlace))
		}
		f.vancouverPublication(b, e)
		switch e.kind {
		case kReport:
			if r.Number != "" {
				sentence(b, f.term("report-no")+" "+r.Number)
			}
		case kPatent:
			sentence(b, joinNonEmpty(" ", f.term("patent"), r.Number))
		case kStandard, kGeneric, kPreprint:
			sentence(b, r.Number)
		}
	}
	f.vancouverLink(b, e)
	return b.result()
}

// vancouverNames renders "Smith JA, Lee K"; more than six names are cut
// to six followed by "et al.".
func (f *fmtr) vancouverNames(ns []ast.Name, others bool) string {
	limit := len(ns)
	if limit > 6 {
		limit, others = 6, true
	}
	parts := make([]string, limit)
	for i := range limit {
		parts[i] = nameVancouver(ns[i])
	}
	s := strings.Join(parts, ", ")
	if others {
		s += ", " + f.term("et-al")
	}
	return s
}

func (f *fmtr) vancouverMedium(e *entry) string {
	switch e.kind {
	case kWebpage:
		return f.term("internet")
	case kThesis:
		return f.thesisLabel(e.ref.Genre)
	case kDataset:
		return firstNonEmpty(e.ref.Genre, f.term("dataset"))
	case kSoftware:
		return firstNonEmpty(e.ref.Genre, f.term("software"))
	case kManuscript:
		return firstNonEmpty(e.ref.Genre, f.term("manuscript"))
	}
	return ""
}

// vancouverDate renders "2020 Mar 3" (day only when asked for).
func (f *fmtr) vancouverDate(d ast.Date, withDay bool) string {
	if d.Year == 0 {
		return d.Literal
	}
	s := strconv.Itoa(d.Year)
	if mn := f.monthName(d.Month, true); mn != "" {
		s += " " + mn
		if withDay && d.Day > 0 {
			s += " " + strconv.Itoa(d.Day)
		}
	}
	return s
}

// vancouverPublication writes "Place: Publisher; Year" with the
// "[cited …]" date for web pages.
func (f *fmtr) vancouverPublication(b *builder, e *entry) {
	r := e.ref
	pub := r.Publisher
	if pub == "" && e.kind == kWebpage {
		pub = r.ContainerTitle
	}
	d := yearOnly(r.Issued)
	if e.kind == kWebpage {
		d = f.vancouverDate(r.Issued, true)
	}
	s := joinNonEmpty("; ", placePublisher(r.PublisherPlace, pub), d)
	if s == "" {
		return
	}
	b.space()
	b.text(s)
	if e.kind == kWebpage && !r.Accessed.IsZero() {
		b.text(" [" + f.term("cited") + " " + f.vancouverDate(r.Accessed, true) + "]")
	}
	b.punct(".")
}

func (f *fmtr) vancouverLink(b *builder, e *entry) {
	r := e.ref
	switch {
	case r.DOI != "":
		b.space()
		b.text("doi:")
		b.link(doiURL(r.DOI), cleanDOI(r.DOI))
	case r.URL != "":
		if e.kind != kWebpage && !r.Accessed.IsZero() {
			b.space()
			b.text("[" + f.term("cited") + " " + f.vancouverDate(r.Accessed, true) + "]")
			b.punct(".")
		}
		b.space()
		b.text(f.term("available-from") + " ")
		b.link(r.URL, r.URL)
	}
}
