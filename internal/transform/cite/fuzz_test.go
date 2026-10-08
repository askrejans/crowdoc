package cite

import (
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

// The fuzz targets guard the "never panic on untrusted input" contract.
// Their seeds run with every `go test`; run `go test -fuzz=FuzzParse`
// for longer exploration.

func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		sampleBib, sampleCSLJSON, sampleRIS, sampleNBIB,
		"@article{k, author = {{\\\"O}zil and others}, title = {$x$ \\v{s}}}",
		"@string{a = b # c}", "@book{k, title = {" + strings.Repeat("{", 300) + "}}",
		"TY  - JOUR\nAU  - ,\nPY  - 99999\nER  -", "PMID- 1\nPG  - 9-1\nAU  - X",
		"references:\n- id: a\n  issued: {date-parts: [[x]]}", "[{\"author\": 5, \"issued\": [[\"2020\"]]}]",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, name := range []string{"a.bib", "a.json", "a.yaml", "a.ris", "a.nbib", "a.unknown"} {
			refs, _, err := Parse(name, []byte(s))
			if err != nil {
				continue
			}
			// Formatting arbitrary parsed data must not panic either.
			doc := &ast.Document{}
			for _, r := range refs {
				doc.Blocks = append(doc.Blocks, para(cite(0, ast.CiteItem{Key: r.ID, Locator: r.Page})))
			}
			for _, st := range []string{"apa", "ieee", "mla"} {
				if _, err := Process(doc, refs, Options{Style: st, Lang: "lv", NoCite: []string{"*"}}); err != nil {
					t.Fatal(err)
				}
			}
		}
	})
}

func FuzzLatexToUnicode(f *testing.F) {
	for _, seed := range []string{`\"{o}`, `{\c{k}}`, `$\alpha$`, `\href{a}{b}`, `\url{x`, `\`, `{`, `}`, `\'\`, "``''---"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		latexToUnicode(s)
		parseBibNames(s)
	})
}

func FuzzDatesAndLabels(f *testing.F) {
	for _, seed := range []string{"2020-05/2020-06", "May 1, 2020", "c. 1850", "[1, 3–5]", "1.", "[2,4-6]"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		parseDateString(s)
		leadingLabel(s)
		splitCitations(s, map[string]bool{"1": true, "2": true, "3": true, "4": true, "5": true, "6": true})
		normalizeRange(s)
		f := newFmtr(styles["chicago"], "en")
		f.pageRange(s)
		parseNameString(s)
		initials(s, true, true)
	})
}

func FuzzLinkPlainReferences(f *testing.F) {
	for _, seed := range []string{"[1] A, 2001.\n[2] B, 2002.", "1. x 2020\n2) y 2021", "(1) a 1999"} {
		f.Add(seed, "see [1–2] and [2,1]")
	}
	f.Fuzz(func(t *testing.T, refs, body string) {
		var ins []ast.Inline
		for i, line := range strings.Split(refs, "\n") {
			if i > 0 {
				ins = append(ins, &ast.LineBreak{})
			}
			ins = append(ins, text(line))
		}
		doc := &ast.Document{Blocks: []ast.Block{para(text(body)), heading(1, "References"), para(ins...)}}
		LinkPlainReferences(doc)
	})
}

func TestDeepNesting(t *testing.T) {
	deep := strings.Repeat("{", 100000) + "x" + strings.Repeat("}", 100000)
	if got := latexToUnicode(deep); got != "" {
		t.Errorf("pathological nesting produced %q", got)
	}
	refs, _ := ParseBibTeX([]byte("@book{k, title = " + deep + "}"))
	if len(refs) != 1 {
		t.Errorf("deeply nested entry not parsed: %d", len(refs))
	}
}
