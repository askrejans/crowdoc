package cite

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// othersLiteral marks a truncated name list ("and others" in BibTeX).
const othersLiteral = "others"

func isOthers(n ast.Name) bool {
	return n.Literal == othersLiteral && n.Family == "" && n.Given == ""
}

// splitOthers removes a trailing "and others" marker.
func splitOthers(ns []ast.Name) ([]ast.Name, bool) {
	out := make([]ast.Name, 0, len(ns))
	others := false
	for _, n := range ns {
		if isOthers(n) {
			others = true
			continue
		}
		if n.Literal == "" && n.Family == "" && n.Given == "" {
			continue
		}
		out = append(out, n)
	}
	return out, others
}

// familyDisplay is the family name as shown, with its particle
// ("van Gogh", "d'Alembert"); institutions show their literal name.
func familyDisplay(n ast.Name) string {
	if n.Literal != "" {
		return n.Literal
	}
	fam := n.Family
	if fam == "" {
		fam = n.Given
	}
	if n.Particle == "" {
		return fam
	}
	if strings.HasSuffix(n.Particle, "'") || strings.HasSuffix(n.Particle, "’") || strings.HasSuffix(n.Particle, "-") {
		return n.Particle + fam
	}
	return n.Particle + " " + fam
}

// initials abbreviates given names: "John Alan" → "J. A.", "Jean-Paul" →
// "J.-P." (with periods), "JP" (Vancouver form without periods).
func initials(given string, period, space bool) string {
	var parts []string
	for _, word := range strings.FieldsFunc(given, func(r rune) bool { return unicode.IsSpace(r) || r == '.' }) {
		if isUpperRun(word) {
			// "JA" as written in MEDLINE-style data: one initial per letter.
			for _, r := range word {
				parts = append(parts, initial(r, period))
			}
			continue
		}
		var hs []string
		for _, sub := range strings.Split(word, "-") {
			r := firstLetter(sub)
			if r == 0 {
				continue
			}
			if unicode.IsLower(r) && len(hs) == 0 && utf8.RuneCountInString(sub) <= 3 && len(parts) > 0 {
				// Lowercase particles inside given names ("Maria de Paz") are dropped.
				continue
			}
			hs = append(hs, initial(r, period))
		}
		if len(hs) == 0 {
			continue
		}
		if period {
			parts = append(parts, strings.Join(hs, "-"))
		} else {
			parts = append(parts, strings.Join(hs, ""))
		}
	}
	sep := ""
	if space {
		sep = " "
	}
	return strings.Join(parts, sep)
}

func initial(r rune, period bool) string {
	s := string(unicode.ToUpper(r))
	if period {
		s += "."
	}
	return s
}

// isUpperRun reports whether word is two or three capitals ("JA", "JAK").
func isUpperRun(word string) bool {
	n := utf8.RuneCountInString(word)
	if n < 2 || n > 3 {
		return false
	}
	for _, r := range word {
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func firstLetter(s string) rune {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return r
		}
	}
	return 0
}

// nameInvertedInitials renders "Smith, J. A." (APA), "Smith, J.A." (Harvard).
func nameInvertedInitials(n ast.Name, space bool) string {
	if n.Literal != "" {
		return n.Literal
	}
	s := familyDisplay(n)
	if n.Family != "" {
		if ini := initials(n.Given, true, space); ini != "" {
			s += ", " + ini
		}
	}
	if n.Suffix != "" {
		s += ", " + n.Suffix
	}
	return s
}

// nameNaturalInitials renders "J. A. Smith" (IEEE, APA editors).
func nameNaturalInitials(n ast.Name) string {
	if n.Literal != "" {
		return n.Literal
	}
	s := familyDisplay(n)
	if n.Family != "" {
		if ini := initials(n.Given, true, true); ini != "" {
			s = ini + " " + s
		}
	}
	if n.Suffix != "" {
		s += ", " + n.Suffix
	}
	return s
}

// nameInvertedFull renders "Smith, John A." (Chicago, MLA first author).
func nameInvertedFull(n ast.Name) string {
	if n.Literal != "" {
		return n.Literal
	}
	s := familyDisplay(n)
	if n.Given != "" && n.Family != "" {
		s += ", " + n.Given
	}
	if n.Suffix != "" {
		s += ", " + n.Suffix
	}
	return s
}

// nameNaturalFull renders "John A. Smith Jr.".
func nameNaturalFull(n ast.Name) string {
	if n.Literal != "" {
		return n.Literal
	}
	s := familyDisplay(n)
	if n.Given != "" && n.Family != "" {
		s = n.Given + " " + s
	}
	if n.Suffix != "" {
		s += " " + n.Suffix
	}
	return s
}

// nameVancouver renders "Smith JA" (NLM: initials without periods).
func nameVancouver(n ast.Name) string {
	if n.Literal != "" {
		return n.Literal
	}
	s := familyDisplay(n)
	if n.Family != "" {
		if ini := initials(n.Given, false, false); ini != "" {
			s += " " + ini
		}
	}
	if n.Suffix != "" {
		s += " " + strings.TrimSuffix(n.Suffix, ".")
	}
	return s
}

// nameSortKey returns the primary and secondary sort strings of a name:
// the family name without its particle ("van Gogh" sorts under G), then
// particle and given names.
func nameSortKey(n ast.Name) (string, string) {
	if n.Literal != "" {
		return n.Literal, ""
	}
	fam := n.Family
	if fam == "" {
		fam = n.Given
	}
	return fam, strings.TrimSpace(n.Given + " " + n.Particle)
}

var nameSuffixes = map[string]bool{
	"jr": true, "jr.": true, "sr": true, "sr.": true, "ii": true, "iii": true, "iv": true,
	"jun.": true, "sen.": true,
}

var institutionWords = []string{
	"organization", "organisation", "institute", "institut", "association", "university",
	"universit", "society", "committee", "council", "agency", "ministry", "department",
	"group", "ltd", "inc.", "inc", "gmbh", "centre", "center", "foundation", "office",
	"bureau", "commission", "consortium", "academy", "board", "service", "authority",
	"administration", "network", "project", "team", "laboratory", "programme", "program",
	"federation", "union", "company", "corporation", "ministrija", "universitāte", "institūts",
	"biedrība", "dienests", "pārvalde", "aģentūra", "padome", "komisija",
}

// looksInstitutional guesses whether an un-inverted name is an organisation.
func looksInstitutional(s string) bool {
	words := strings.Fields(s)
	if len(words) >= 5 {
		return true
	}
	low := strings.ToLower(s)
	for _, w := range strings.Fields(low) {
		w = strings.Trim(w, ",.;:()")
		for _, k := range institutionWords {
			if w == k || (len(k) > 5 && strings.HasPrefix(w, k)) {
				return true
			}
		}
	}
	return false
}

// parseNameString parses a personal name written as "Family, Given",
// "Family, Given, Suffix", "Family, Suffix, Given" or "Given Family".
// Organisation-like strings become literal names.
func parseNameString(s string) ast.Name {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ast.Name{}
	}
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		n := ast.Name{}
		n.Particle, n.Family = splitParticle(parts[0])
		switch {
		case len(parts) >= 3 && nameSuffixes[strings.ToLower(parts[1])]:
			n.Suffix, n.Given = parts[1], parts[2]
		case len(parts) >= 3:
			n.Given, n.Suffix = parts[1], parts[2]
		case len(parts) == 2:
			n.Given = parts[1]
		}
		if n.Family == "" {
			return ast.Name{Literal: s}
		}
		return n
	}
	if looksInstitutional(s) {
		return ast.Name{Literal: s}
	}
	words := strings.Fields(s)
	if len(words) == 1 {
		return ast.Name{Family: words[0]}
	}
	var suffix string
	if nameSuffixes[strings.ToLower(words[len(words)-1])] && len(words) > 2 {
		suffix = words[len(words)-1]
		words = words[:len(words)-1]
	}
	// Particles: lowercase words between the given names and the family name.
	i := len(words) - 1
	for i > 1 && isLowerWord(words[i-1]) {
		i--
	}
	return ast.Name{
		Given:    strings.Join(words[:min(i, len(words)-1)], " "),
		Particle: strings.Join(words[min(i, len(words)-1):len(words)-1], " "),
		Family:   words[len(words)-1],
		Suffix:   suffix,
	}
}

// splitParticle separates leading lowercase particles from a family name:
// "van der Berg" → ("van der", "Berg").
func splitParticle(fam string) (string, string) {
	words := strings.Fields(fam)
	i := 0
	for i < len(words)-1 && isLowerWord(words[i]) {
		i++
	}
	return strings.Join(words[:i], " "), strings.Join(words[i:], " ")
}

func isLowerWord(w string) bool {
	r := firstLetter(w)
	return r != 0 && unicode.IsLower(r)
}
