package cite

import (
	"github.com/askrejans/crowdoc/v2/ast"
)

// mlaEntry formats a works-cited entry in MLA 9th edition style:
//
//	Smith, John A., and Kim Lee. “Title.” Journal, vol. 12, no. 3, 2020, pp. 45–67.
func mlaEntry(f *fmtr, e *entry) []ast.Inline {
	r := e.ref
	b := f.builder()
	if len(e.names) > 0 {
		if e.sameNames {
			b.text("---") // repeated author(s) in a works-cited list
		} else {
			b.text(f.mlaNames(e.names, e.others))
		}
		switch e.role {
		case "editor":
			b.text(", " + f.termN("editor", len(e.names)))
		case "translator":
			b.text(", " + f.termN("translator", len(e.names)))
		}
		b.punct(".")
	}
	container := ""
	if e.inContainer() || (e.kind == kWebpage && r.ContainerTitle != "") {
		container = r.ContainerTitle
	}
	if r.Title != "" {
		b.space()
		switch {
		case container != "":
			f.quote(b, r.Title)
		case e.kind == kLegal:
			b.text(r.Title)
		default:
			b.emph(r.Title)
		}
		b.punct(".")
	}
	if e.kind == kThesis {
		sentence(b, f.date(r.Issued, true))
		sentence(b, joinNonEmpty(", ", r.Publisher, f.thesisLabel(r.Genre)))
		f.mlaLocation(b, e, true)
		return b.result()
	}
	// Elements of the container, separated by commas. Terms starting a
	// new sentence are capitalised ("Edited by", "Vol."); names and
	// identifiers are not.
	type element struct {
		text string
		term bool
	}
	var elems []element
	add := func(text string, term bool) {
		if text != "" {
			elems = append(elems, element{text, term})
		}
	}
	if eds, others := splitOthers(r.Editor); len(eds) > 0 && (container != "" || e.role == "author") {
		add(f.term("edited-by")+" "+f.mlaNatNames(eds, others), true)
	}
	if tr, others := splitOthers(r.Translator); len(tr) > 0 && e.role != "translator" {
		add(f.term("translated-by")+" "+f.mlaNatNames(tr, others), true)
	}
	if (e.kind == kDataset || e.kind == kSoftware) && r.Version != "" {
		add(f.term("version")+" "+r.Version, true)
	}
	add(f.edition(r.Edition), true)
	if r.Volume != "" {
		add(f.labelled("volume", r.Volume), true)
	}
	if r.Issue != "" {
		add(f.labelled("issue", r.Issue), true)
	}
	switch e.kind {
	case kReport, kStandard, kGeneric, kPreprint:
		if r.Number != "" && r.Number[0] >= '0' && r.Number[0] <= '9' {
			add(f.labelled("issue", r.Number), true)
		} else {
			add(r.Number, false)
		}
	case kPatent:
		add(joinNonEmpty(" ", f.term("patent"), r.Number), true)
	case kConference:
		if container == "" {
			add(r.Event, false)
		}
	}
	switch e.kind {
	case kJournal, kMagazine, kNewspaper, kBlogPost, kWebpage:
	default:
		add(r.Publisher, false)
	}
	add(f.date(r.Issued, true), false)
	if e.kind == kConference && container == "" {
		add(r.EventPlace, false)
	}
	add(f.pagesLabelled(r.Page), true)

	started := false
	if container != "" {
		b.space()
		b.emph(container)
		started = true
	}
	for _, el := range elems {
		if started {
			b.punct(",")
			b.space()
			b.text(el.text)
			continue
		}
		b.space()
		if el.term {
			b.text(capFirst(el.text))
		} else {
			b.text(el.text)
		}
		started = true
	}
	f.mlaLocation(b, e, !started)
	return b.result()
}

// mlaLocation ends an entry with the DOI or URL and, for undated online
// works, the access date.
func (f *fmtr) mlaLocation(b *builder, e *entry, newSentence bool) {
	r := e.ref
	u := r.URL
	if r.DOI != "" {
		u = doiURL(r.DOI)
	}
	if u != "" {
		if newSentence {
			b.punct(".")
		} else {
			b.punct(",")
		}
		b.space()
		b.link(u, u)
	}
	b.punct(".")
	if r.DOI == "" && r.URL != "" && r.Issued.IsZero() && !r.Accessed.IsZero() {
		sentence(b, capFirst(f.term("accessed"))+" "+f.accessedDate(r.Accessed, true))
	}
}

// mlaNames renders "Smith, John A.", "Smith, John A., and Kim Lee" or
// "Smith, John A., et al.".
func (f *fmtr) mlaNames(ns []ast.Name, others bool) string {
	switch {
	case len(ns) >= 3 || (others && len(ns) > 0):
		return nameInvertedFull(ns[0]) + ", " + f.term("et-al")
	case len(ns) == 2:
		return nameInvertedFull(ns[0]) + ", " + f.term("and") + " " + nameNaturalFull(ns[1])
	case len(ns) == 1:
		return nameInvertedFull(ns[0])
	}
	return ""
}

// mlaNatNames renders contributors in natural order: "Ilze Ozola and
// Pēteris Kalniņš", "Ilze Ozola et al.".
func (f *fmtr) mlaNatNames(ns []ast.Name, others bool) string {
	switch {
	case len(ns) >= 3 || (others && len(ns) > 0):
		return nameNaturalFull(ns[0]) + " " + f.term("et-al")
	case len(ns) == 2:
		return nameNaturalFull(ns[0]) + " " + f.term("and") + " " + nameNaturalFull(ns[1])
	case len(ns) == 1:
		return nameNaturalFull(ns[0])
	}
	return ""
}
