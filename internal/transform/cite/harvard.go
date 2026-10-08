package cite

import (
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// harvardEntry formats a reference list entry in Harvard author-date
// style (British conventions):
//
//	Smith, J.A. and Lee, K. (2020) ‘Title’, Journal, 12(3), pp. 45–67. Available at: https://doi.org/….
func harvardEntry(f *fmtr, e *entry) []ast.Inline {
	b := f.builder()
	titleFirst := len(e.names) == 0
	if titleFirst {
		f.harvardTitle(b, e)
	} else {
		b.text(f.harvardNames(e.names, e.others))
		switch e.role {
		case "editor":
			b.text(" (" + f.termN("editor", len(e.names)) + ")")
		case "translator":
			b.text(" (" + f.termN("translator", len(e.names)) + ")")
		}
	}
	b.space()
	b.text("(" + f.yearText(e) + ")")
	if !titleFirst && e.ref.Title != "" {
		b.space()
		f.harvardTitle(b, e)
	}
	f.harvardSource(b, e)
	f.harvardLink(b, e)
	b.punct(".")
	return b.result()
}

// harvardNames renders "Smith, J.A., Lee, K. and Brown, M.".
func (f *fmtr) harvardNames(ns []ast.Name, others bool) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = nameInvertedInitials(n, false)
	}
	n := len(parts)
	and := f.term("and")
	switch {
	case n == 0:
		return ""
	case others:
		return strings.Join(parts, ", ") + " " + f.term("et-al")
	case n == 1:
		return parts[0]
	}
	return strings.Join(parts[:n-1], ", ") + " " + and + " " + parts[n-1]
}

// harvardTitle writes the title — quoted for parts of a larger work,
// italic otherwise — followed by version and medium for data and software.
func (f *fmtr) harvardTitle(b *builder, e *entry) {
	r := e.ref
	switch {
	case r.Title == "":
		return
	case e.inContainer():
		f.quote(b, r.Title)
		return
	case e.kind == kLegal:
		b.text(r.Title)
	default:
		b.emph(r.Title)
	}
	if (e.kind == kDataset || e.kind == kSoftware) && r.Version != "" {
		b.text(" (" + f.term("version") + " " + r.Version + ")")
	}
	switch e.kind {
	case kDataset:
		b.text(" [" + firstNonEmpty(r.Genre, f.term("dataset")) + "]")
	case kSoftware:
		b.text(" [" + firstNonEmpty(r.Genre, f.term("software")) + "]")
	}
}

func (f *fmtr) harvardSource(b *builder, e *entry) {
	r := e.ref
	switch {
	case e.inContainer() && (e.kind == kJournal || e.kind == kMagazine || e.kind == kNewspaper || e.kind == kBlogPost):
		if len(e.names) > 0 {
			b.punct(",")
		}
		b.space()
		b.emph(r.ContainerTitle)
		if e.kind == kJournal {
			vi := r.Volume
			if r.Issue != "" {
				vi += "(" + r.Issue + ")"
			}
			if vi != "" {
				b.text(", " + vi)
			}
		} else if r.Issued.Month > 0 {
			b.text(", " + f.dayMonth(r.Issued, false))
		}
		if p := f.pagesLabelled(r.Page); p != "" {
			b.text(", " + p)
		}
		b.punct(".")
		return
	case e.inContainer():
		if len(e.names) > 0 {
			b.punct(",")
		}
		b.space()
		b.text(lowerFirst(f.term("in")) + " ")
		if eds, others := splitOthers(r.Editor); len(eds) > 0 {
			b.text(f.harvardNames(eds, others) + " (" + f.termN("editor", len(eds)) + ") ")
		}
		b.emph(r.ContainerTitle)
		b.punct(".")
		sentence(b, f.edition(r.Edition))
		if pp := placePublisher(r.PublisherPlace, r.Publisher); pp != "" {
			b.space()
			b.text(pp)
		}
		if p := f.pagesLabelled(r.Page); p != "" {
			b.punct(",")
			b.space()
			b.text(p)
		}
		b.punct(".")
		return
	}
	b.punct(".")
	if tr, others := splitOthers(r.Translator); len(tr) > 0 && e.role != "translator" {
		sentence(b, capFirst(f.term("translated-by"))+" "+f.harvardNames(tr, others))
	}
	sentence(b, f.edition(r.Edition))
	if r.Volume != "" && e.kind == kBook {
		sentence(b, capFirst(f.labelled("volume", r.Volume)))
	}
	switch e.kind {
	case kThesis:
		sentence(b, f.thesisLabel(r.Genre))
		sentence(b, r.Publisher)
		return
	case kConference:
		sentence(b, joinNonEmpty(", ", r.Event, r.EventPlace))
		return
	case kWebpage:
		return
	case kManuscript:
		sentence(b, firstNonEmpty(r.Genre, f.term("manuscript")))
		sentence(b, r.Publisher)
		return
	case kReport:
		if r.Number != "" {
			sentence(b, joinNonEmpty(" ", firstNonEmpty(r.Genre, f.term("report")), r.Number))
		}
	case kPatent:
		sentence(b, joinNonEmpty(" ", f.term("patent"), r.Number))
	case kStandard, kLegal, kGeneric, kPreprint:
		sentence(b, r.Number)
	}
	if pp := placePublisher(r.PublisherPlace, r.Publisher); pp != "" && (e.kind != kReport || !e.sameAsAuthor(pp)) {
		sentence(b, pp)
	}
}

// harvardLink writes "Available at: URL (Accessed: 1 May 2021)".
func (f *fmtr) harvardLink(b *builder, e *entry) {
	r := e.ref
	u := r.URL
	if r.DOI != "" {
		u = doiURL(r.DOI)
	}
	if u == "" {
		return
	}
	b.punct(".")
	b.space()
	b.text(f.term("available-at") + ": ")
	b.link(u, u)
	if !r.Accessed.IsZero() {
		b.text(" (" + f.term("accessed") + ": " + f.accessedDate(r.Accessed, false) + ")")
	}
}
