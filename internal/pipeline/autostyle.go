package pipeline

import (
	"regexp"
	"strings"
)

// typeRules map document types and title keywords (in several languages)
// to styles, in priority order.
var typeRules = []struct {
	re    *regexp.Regexp
	style string
}{
	{regexp.MustCompile(`(?i)\b(līgums|ligums|vienošanās)\b`), "ligums"},
	{regexp.MustCompile(`(?i)agreement|contract|\bnda\b|vertrag|contrat|contrato|contratto|umowa|smlouva|sutartis|leping|sopimus|avtal|aftale|overeenkomst|szerződés|sözleşme|договор|угода|terms (of|and) (service|use)`), "legal"},
	{regexp.MustCompile(`(?i)invoice|rēķins|rekins|\bbill\b|receipt|rechnung|facture|factura|fattura|faktura|sąskaita|arve|lasku|счёт|счет-фактура|quotation|credit note`), "invoice"},
	{regexp.MustCompile(`(?i)\bmemo(randum)?\b|memorands|notice|announcement|rundschreiben`), "memo"},
	{regexp.MustCompile(`(?i)\bminutes\b|protokols|protokoll|protocol|procès-verbal|acta de|verbale`), "minutes"},
	{regexp.MustCompile(`(?i)\bletter\b|vēstule|correspondence|brief an|lettre|carta`), "letter"},
	{regexp.MustCompile(`(?i)thesis|dissertation|diplomdarbs|bakalaura darbs|maģistra darbs|promocijas darbs|doktorarbeit|masterarbeit|bachelorarbeit|mémoire|tesis|tesi`), "thesis"},
	{regexp.MustCompile(`(?i)\b(cv|résumé|resume|curriculum vitae|dzīvesgājums|lebenslauf)\b`), "cv"},
	{regexp.MustCompile(`(?i)proposal|piedāvājums|angebot|proposition|propuesta|proposta|statement of work`), "proposal"},
	{regexp.MustCompile(`(?i)white ?paper`), "whitepaper"},
	{regexp.MustCompile(`(?i)\bpolicy\b|procedure|politika|kārtība|noteikumi|richtlinie|politique|política`), "policy"},
	{regexp.MustCompile(`(?i)manual|handbook|user guide|rokasgrāmata|lietošanas instrukcija|handbuch|bedienungsanleitung`), "manual"},
	{regexp.MustCompile(`(?i)newsletter|bulletin|jaunumi`), "newsletter"},
	{regexp.MustCompile(`(?i)lecture|lekcija|study notes|vorlesung|cours`), "notes"},
	{regexp.MustCompile(`(?i)\bpaper\b|research|study|journal|article|pētījums|raksts|studie|étude|estudio`), "article"},
	{regexp.MustCompile(`(?i)spec(ification)?\b|technical|\bapi\b|architecture|tehniskā|technische`), "technical"},
	{regexp.MustCompile(`(?i)slides|presentation|prezentācija|präsentation`), "slides"},
	{regexp.MustCompile(`(?i)report|analysis|review|pārskats|ziņojums|bericht|rapport|informe|relazione`), "report"},
}

// styleForType picks a style from the declared document type, the title,
// or the input format.
func styleForType(docType, title string, f Format) string {
	for _, s := range []string{docType, title} {
		if strings.TrimSpace(s) == "" {
			continue
		}
		for _, r := range typeRules {
			if r.re.MatchString(s) {
				return r.style
			}
		}
	}
	switch f {
	case FormatCSV, FormatTSV, FormatXLSX, FormatODS:
		return "data"
	case FormatText:
		return "minimal"
	case FormatNotebook:
		return "notes"
	case FormatEPUB:
		return "book"
	}
	return "report"
}
