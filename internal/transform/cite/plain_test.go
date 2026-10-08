package cite

import (
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

func text(s string) *ast.Text { return &ast.Text{Value: s} }

func heading(level int, s string) *ast.Heading {
	return &ast.Heading{Level: level, Inlines: []ast.Inline{text(s)}}
}

func refList(t *testing.T, blocks []ast.Block) *ast.ReferenceList {
	t.Helper()
	var found *ast.ReferenceList
	ast.WalkBlocks(blocks, func(b ast.Block) bool {
		if rl, ok := b.(*ast.ReferenceList); ok && found == nil {
			found = rl
		}
		return true
	})
	if found == nil {
		t.Fatal("no ReferenceList produced")
	}
	return found
}

func entriesMD(rl *ast.ReferenceList) []string {
	var out []string
	for _, e := range rl.Entries {
		out = append(out, e.ID+"|"+e.Label+"|"+md(e.Inlines))
	}
	return out
}

func equalStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %d items %q, want %d %q", what, len(got), got, len(want), want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d]:\n got: %s\nwant: %s", what, i, got[i], want[i])
		}
	}
}

func TestLinkPlainReferencesNumbered(t *testing.T) {
	body := para(text("Earlier work [1] and later studies [2, 3] or [1–3], also [2,4-6] and [1--2]. Not [7] or [2023]."))
	note := para(text("Intro"), &ast.Note{Blocks: []ast.Block{para(text("See [4]."))}})
	cell := &ast.Table{Body: []ast.Row{{Cells: []ast.Cell{{Blocks: []ast.Block{&ast.Plain{Inlines: []ast.Inline{&ast.Strong{Inlines: []ast.Inline{text("[5]")}}}}}}}}}}
	code := &ast.CodeBlock{Text: "x = a[1]"}
	inlineCode := para(&ast.Code{Text: "arr[1]"}, text(" see http://example.org/page[1] and "), &ast.Math{TeX: "x_[1]"},
		&ast.Link{URL: "https://x.org", Inlines: []ast.Inline{text("[2]")}})
	doc := &ast.Document{Blocks: []ast.Block{
		heading(1, "Introduction"), body, note, cell, code, inlineCode,
		heading(1, "7. References"),
		para(text("[1] Smith, J. (2020). First work. "), &ast.Emph{Inlines: []ast.Inline{text("Journal")}}, text(".")),
		para(text("[2] Lee, K. Second work.")),
		para(text("Continued: Example Press, 2019.")),
		para(text("[3] Third, 2018."), &ast.LineBreak{}, text("[4] Fourth, 2017."), &ast.SoftBreak{}, text("[5] Fifth, 2016.")),
		para(text("[6] Sixth, 2015.")),
		heading(1, "Appendix"),
		para(text("Appendix text [1].")),
	}}
	n := LinkPlainReferences(doc)
	if n != 6 {
		t.Fatalf("found %d entries, want 6", n)
	}
	rl := refList(t, doc.Blocks)
	equalStrings(t, "entries", entriesMD(rl), []string{
		"ref-1|1|Smith, J. (2020). First work. _Journal_.",
		"ref-2|2|Lee, K. Second work. Continued: Example Press, 2019.",
		"ref-3|3|Third, 2018.",
		"ref-4|4|Fourth, 2017.",
		"ref-5|5|Fifth, 2016.",
		"ref-6|6|Sixth, 2015.",
	})
	if _, ok := doc.Blocks[7].(*ast.ReferenceList); !ok || len(doc.Blocks) != 10 {
		t.Errorf("reference paragraphs not replaced in place: %d blocks", len(doc.Blocks))
	}
	if h, ok := doc.Blocks[8].(*ast.Heading); !ok || ast.PlainText(h.Inlines) != "Appendix" {
		t.Errorf("following section disturbed")
	}
	want := "Earlier work [[1](#ref-1)] and later studies [[2](#ref-2), [3](#ref-3)] or [[1](#ref-1)–[3](#ref-3)], " +
		"also [[2](#ref-2),[4](#ref-4)-[6](#ref-6)] and [[1](#ref-1)--[2](#ref-2)]. Not [7] or [2023]."
	if got := md(body.Inlines); got != want {
		t.Errorf("body:\n got: %s\nwant: %s", got, want)
	}
	if got := md(note.Inlines); got != "Intro^[See [[4](#ref-4)].]" {
		t.Errorf("footnote: %s", got)
	}
	if got := md(ast.InlinesOf(cell.Body[0].Cells[0].Blocks[0])[0]); got != "*[[5](#ref-5)]*" {
		t.Errorf("table cell: %s", got)
	}
	if code.Text != "x = a[1]" {
		t.Errorf("code block changed")
	}
	if got := md(inlineCode.Inlines); got != "`arr[1]` see http://example.org/page[1] and $x_[1]$[[2]](https://x.org)" {
		t.Errorf("code/url/math/link touched: %s", got)
	}
	if got := md(doc.Blocks[9].(*ast.Para).Inlines); got != "Appendix text [[1](#ref-1)]." {
		t.Errorf("appendix: %s", got)
	}
}

func TestLinkPlainReferencesOrderedList(t *testing.T) {
	list := &ast.List{Ordered: true, Start: 1, Items: []ast.ListItem{
		{Blocks: []ast.Block{&ast.Plain{Inlines: []ast.Inline{text("Bērziņš, J. Graudu glabāšana. Rīga, 2021.")}}}},
		{Blocks: []ast.Block{&ast.Plain{Inlines: []ast.Inline{text("Ozola, I. Augsne. 2019.")}}}},
	}}
	body := para(text("Kā norādīts [2], un [1, 2]."))
	doc := &ast.Document{Blocks: []ast.Block{body, heading(2, "Izmantotā literatūra:"), list}}
	if n := LinkPlainReferences(doc); n != 2 {
		t.Fatalf("found %d", n)
	}
	equalStrings(t, "entries", entriesMD(refList(t, doc.Blocks)), []string{
		"ref-1|1|Bērziņš, J. Graudu glabāšana. Rīga, 2021.",
		"ref-2|2|Ozola, I. Augsne. 2019.",
	})
	if got := md(body.Inlines); got != "Kā norādīts [[2](#ref-2)], un [[1](#ref-1), [2](#ref-2)]." {
		t.Errorf("body: %s", got)
	}
}

func TestLinkPlainReferencesAuthorDate(t *testing.T) {
	doc := &ast.Document{Blocks: []ast.Block{
		para(text("As shown by Smith (2020) [1]."), &ast.Note{Blocks: []ast.Block{para(text("x"))}}),
		heading(1, "BIBLIOGRAPHY"),
		para(text("Lee, K. (2019). Soil. Northfield Press."), &ast.LineBreak{}, text("Smith, J. (2020). Crops. Example Press.")),
		para(text("Ozols, P. (n.d.). Grain. https://example.org")),
		para(text("Closing remarks without any year.")),
	}}
	if n := LinkPlainReferences(doc); n != 3 {
		t.Fatalf("found %d", n)
	}
	equalStrings(t, "entries", entriesMD(refList(t, doc.Blocks)), []string{
		"ref-1||Lee, K. (2019). Soil. Northfield Press.",
		"ref-2||Smith, J. (2020). Crops. Example Press.",
		"ref-3||Ozols, P. (n.d.). Grain. https://example.org",
	})
	if p, ok := doc.Blocks[3].(*ast.Para); !ok || ast.PlainText(p.Inlines) != "Closing remarks without any year." {
		t.Errorf("trailing prose was consumed")
	}
	if got := md(doc.Blocks[0].(*ast.Para).Inlines); got != "As shown by Smith (2020) [1].^[x]" {
		t.Errorf("author-date list must not link numbers: %s", got)
	}
}

func TestLinkPlainReferencesBulletLabels(t *testing.T) {
	list := &ast.List{Items: []ast.ListItem{
		{Blocks: []ast.Block{para(text("[1] A, 2001."))}},
		{Blocks: []ast.Block{para(text("[2] B, 2002."))}},
	}}
	doc := &ast.Document{Blocks: []ast.Block{para(text("x [2]")), heading(1, "Literaturverzeichnis"), list}}
	if n := LinkPlainReferences(doc); n != 2 {
		t.Fatalf("found %d", n)
	}
	equalStrings(t, "entries", entriesMD(refList(t, doc.Blocks)), []string{"ref-1|1|A, 2001.", "ref-2|2|B, 2002."})
}

func TestLinkPlainReferencesNoop(t *testing.T) {
	tests := map[string]*ast.Document{
		"heading without entries": {Blocks: []ast.Block{para(text("see [1]")), heading(1, "References"), heading(1, "Next")}},
		"heading at the end":      {Blocks: []ast.Block{para(text("see [1]")), heading(1, "References")}},
		"prose under Sources":     {Blocks: []ast.Block{heading(1, "Sources"), para(text("We used many sources for this chapter."))}},
		"not a reference title":   {Blocks: []ast.Block{heading(1, "Results"), para(text("[1] Smith, 2020."))}},
		"structured citations": {Blocks: []ast.Block{
			para(&ast.Cite{Items: keys("x")}), heading(1, "References"), para(text("[1] Smith, 2020.")),
		}},
		"already converted": {Blocks: []ast.Block{
			heading(1, "References"), &ast.ReferenceList{}, para(text("[1] Smith, 2020.")),
		}},
		"code after heading": {Blocks: []ast.Block{heading(1, "References"), &ast.CodeBlock{Text: "[1] Smith, 2020."}}},
	}
	for name, doc := range tests {
		before := len(doc.Blocks)
		if n := LinkPlainReferences(doc); n != 0 || len(doc.Blocks) != before {
			t.Errorf("%s: converted %d entries", name, n)
		}
		if got := md(ast.InlinesOf(doc.Blocks[0])[0]); name == "heading without entries" && got != "see [1]" {
			t.Errorf("%s: text changed to %s", name, got)
		}
	}
	if LinkPlainReferences(nil) != 0 {
		t.Error("nil document")
	}
}

func TestLinkPlainReferencesInSection(t *testing.T) {
	section := &ast.Div{Blocks: []ast.Block{heading(2, "Список литературы"), para(text("1. Иванов И. И. Книга. 2010."))}}
	body := para(text("См. [1]."))
	doc := &ast.Document{Blocks: []ast.Block{body, section}}
	if n := LinkPlainReferences(doc); n != 1 {
		t.Fatalf("found %d", n)
	}
	equalStrings(t, "entries", entriesMD(refList(t, doc.Blocks)), []string{"ref-1|1|Иванов И. И. Книга. 2010."})
	if got := md(body.Inlines); got != "См. [[1](#ref-1)]." {
		t.Errorf("body: %s", got)
	}
}

func TestReferenceTitles(t *testing.T) {
	for _, s := range []string{
		"References", "REFERENCES", "7. References", "VII. Bibliography", "A. Works Cited", "References:",
		"Literatūra", "Izmantotā literatūra", "Izmantotie avoti", "Avoti", "Literaturverzeichnis", "Quellen",
		"Bibliographie", "Références", "Bibliografía", "Referencias", "Bibliografia", "Riferimenti",
		"Literatuur", "Bibliografi", "Kirjallisuus", "Источники", "Литература", "Список литературы",
		"4.2 Literature",
	} {
		if !isReferenceTitle(s) {
			t.Errorf("%q not recognised", s)
		}
	}
	for _, s := range []string{"Introduction", "Reference implementation", "1. Methods", "Source code"} {
		if isReferenceTitle(s) {
			t.Errorf("%q wrongly recognised", s)
		}
	}
}

func TestLeadingLabel(t *testing.T) {
	tests := []struct {
		in, label string
	}{
		{"[1] Smith", "1"}, {"[12]", "12"}, {"1. Smith", "1"}, {"3) Smith", "3"}, {"(4) Smith", "4"},
		{"  [5]\tSmith", "5"}, {"007. Bond", "7"},
		{"1.5 m", ""}, {"2020. Smith", ""}, {"[a] x", ""}, {"Smith [1]", ""}, {"[0] x", ""}, {"[12345] x", ""},
	}
	for _, tc := range tests {
		if got, _ := leadingLabel(tc.in); got != tc.label {
			t.Errorf("leadingLabel(%q) = %q, want %q", tc.in, got, tc.label)
		}
	}
}
