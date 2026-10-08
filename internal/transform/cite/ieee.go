package cite

import (
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// ieeeEntry formats a numbered reference in IEEE style:
//
//	J. A. Smith and K. Lee, “Title,” Journal, vol. 12, no. 3, pp. 45–67, Mar. 2020, doi: 10.….
func ieeeEntry(f *fmtr, e *entry) []ast.Inline {
	r := e.ref
	b := f.builder()
	if len(e.names) > 0 && e.kind != kStandard {
		b.text(f.ieeeNames(e.names, e.others))
		if e.role == "editor" {
			b.text(", " + f.termN("editor", len(e.names)))
		}
		b.punct(",")
		b.space()
	}
	year := f.ieeeDate(r.Issued, false)
	items := func(parts ...string) {
		for _, p := range parts {
			if p != "" {
				b.punct(",")
				b.space()
				b.text(p)
			}
		}
	}
	quotedTitle := func() {
		if r.Title != "" {
			f.quote(b, r.Title)
			b.punct(",")
		}
	}
	switch {
	case e.inContainer() && (e.kind == kJournal || e.kind == kMagazine || e.kind == kNewspaper || e.kind == kBlogPost):
		quotedTitle()
		b.space()
		b.emph(r.ContainerTitle)
		vol, iss := "", ""
		if r.Volume != "" {
			vol = f.labelled("volume", r.Volume)
		}
		if r.Issue != "" {
			iss = f.labelled("issue", r.Issue)
		}
		items(vol, iss, f.pagesLabelled(r.Page), f.ieeeDate(r.Issued, e.kind != kJournal))
	case e.inContainer() && e.kind == kConference:
		quotedTitle()
		b.space()
		b.text(lowerFirst(f.term("in")) + " ")
		b.emph(r.ContainerTitle)
		items(firstNonEmpty(r.EventPlace, r.PublisherPlace), yearOnly(r.Issued), f.pagesLabelled(r.Page))
	case e.inContainer():
		quotedTitle()
		b.space()
		b.text(lowerFirst(f.term("in")) + " ")
		b.emph(r.ContainerTitle)
		items(f.edition(r.Edition))
		if eds, others := splitOthers(r.Editor); len(eds) > 0 {
			items(f.ieeeNames(eds, others) + ", " + f.termN("editor", len(eds)))
		}
		if r.Volume != "" {
			items(f.labelled("volume", r.Volume))
		}
		b.punct(".")
		b.space()
		b.text(joinNonEmpty(", ", placePublisher(r.PublisherPlace, r.Publisher), year))
		items(f.pagesLabelled(r.Page))
	case e.kind == kThesis:
		quotedTitle()
		b.space()
		b.text(f.thesisLabel(r.Genre))
		items(r.Publisher, r.PublisherPlace, year)
	case e.kind == kReport:
		quotedTitle()
		label := ""
		if r.Number != "" {
			label = firstNonEmpty(r.Genre, f.term("report")) + " " + r.Number
		}
		b.space()
		b.text(joinNonEmpty(", ", r.Publisher, r.PublisherPlace, label, f.ieeeDate(r.Issued, false)))
	case e.kind == kWebpage || e.kind == kBlogPost:
		quotedTitle()
		items(r.ContainerTitle, f.ieeeDate(r.Issued, true))
	case e.kind == kPatent:
		quotedTitle()
		items(joinNonEmpty(" ", f.term("patent"), r.Number), f.ieeeDate(r.Issued, true))
	case e.kind == kStandard:
		b.emph(r.Title)
		items(r.Number, year)
	case e.kind == kManuscript:
		quotedTitle()
		items(f.term("unpublished"))
	case e.kind == kConference:
		quotedTitle()
		items(joinNonEmpty(" ", lowerFirst(f.term("presented-at")), r.Event), r.EventPlace, f.ieeeDate(r.Issued, true))
	case e.kind == kDataset || e.kind == kGeneric || e.kind == kPreprint:
		quotedTitle()
		items(r.Publisher, r.Number, year)
	default: // books, software, legislation
		b.emph(r.Title)
		if r.Version != "" {
			items(f.term("version") + " " + r.Version)
		}
		if r.Volume != "" {
			items(f.labelled("volume", r.Volume))
		}
		items(f.edition(r.Edition))
		b.punct(".")
		b.space()
		b.text(joinNonEmpty(", ", placePublisher(r.PublisherPlace, r.Publisher), year))
	}
	f.ieeeEnd(b, e)
	return b.result()
}

// ieeeNames renders "J. A. Smith, K. Lee, and M. Brown"; more than six
// authors become "J. A. Smith et al.".
func (f *fmtr) ieeeNames(ns []ast.Name, others bool) string {
	if len(ns) == 0 {
		return ""
	}
	if len(ns) > 6 || others {
		return nameNaturalInitials(ns[0]) + " " + f.term("et-al")
	}
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = nameNaturalInitials(n)
	}
	and := f.term("and")
	switch len(parts) {
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " " + and + " " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + ", " + and + " " + parts[len(parts)-1]
}

// ieeeDate renders "Mar. 2020" or, with day, "Mar. 3, 2020".
func (f *fmtr) ieeeDate(d ast.Date, withDay bool) string {
	if d.Year == 0 {
		return d.Literal
	}
	mn := f.monthName(d.Month, true)
	if mn == "" {
		return strconv.Itoa(d.Year)
	}
	day := 0
	if withDay {
		day = d.Day
	}
	return f.loc.fullDate(d.Year, day, f.loc.dayMonth(day, d.Month, mn, true, false), false)
}

func yearOnly(d ast.Date) string {
	if d.Year == 0 {
		return d.Literal
	}
	return strconv.Itoa(d.Year)
}

// ieeeEnd closes an entry with the DOI or the online locator.
func (f *fmtr) ieeeEnd(b *builder, e *entry) {
	r := e.ref
	if r.DOI != "" {
		b.punct(",")
		b.text(" doi: ")
		b.link(doiURL(r.DOI), cleanDOI(r.DOI))
		b.punct(".")
		return
	}
	b.punct(".")
	if r.URL == "" {
		return
	}
	if !r.Accessed.IsZero() {
		b.space()
		b.text(capFirst(f.term("accessed")) + ": " + f.ieeeDate(r.Accessed, true))
		b.punct(".")
	}
	b.space()
	b.text(f.term("online") + ". " + f.term("available-at") + ": ")
	b.link(r.URL, r.URL)
}
