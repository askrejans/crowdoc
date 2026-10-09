package typst

import "testing"

func TestStyleNamesAndAliasesAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, s := range builtin {
		for _, n := range append([]string{s.Name}, s.Aliases...) {
			if prev, ok := seen[n]; ok {
				t.Errorf("%q is used by both %s and %s", n, prev, s.Name)
			}
			seen[n] = s.Name
		}
	}
}

// Plain-text styles print only what is in the document: no contents,
// section numbers, title page or signature lines unless asked for.
func TestTextStylesAddNothing(t *testing.T) {
	for _, name := range []string{"page", "clean", "typewriter", "largeprint", "notebook"} {
		s, ok := LookupStyle(name)
		if !ok {
			t.Fatalf("%s not registered", name)
		}
		if s.Category != "text" || s.TOC != "never" || s.NumberSections || s.TitlePage || s.Signatures != "never" {
			t.Errorf("%s: %+v", name, s)
		}
		if s.HasHeader() == s.HasFooter() {
			t.Errorf("%s: header %v footer %v; want exactly one piece of furniture", name, s.HasHeader(), s.HasFooter())
		}
	}
	for _, name := range []string{"personal", "formal"} {
		if s, ok := LookupStyle(name); !ok || s.Category != "correspondence" {
			t.Errorf("%s not registered as correspondence", name)
		}
	}
}
