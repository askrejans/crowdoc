package normalize

import (
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

func h(level int, text string) *ast.Heading {
	return &ast.Heading{Level: level, Inlines: ast.Str(text)}
}

func p(text string) *ast.Para { return &ast.Para{Inlines: ast.Str(text)} }

func TestTitleInference(t *testing.T) {
	cases := []struct {
		name      string
		meta      string
		blocks    []ast.Block
		wantTitle string
		wantFirst string
	}{
		{"single h1 becomes title", "", []ast.Block{h(1, "Report"), h(2, "Intro"), p("x")}, "Report", "H1 Intro"},
		{"several h1 keep headings", "", []ast.Block{h(1, "One"), p("x"), h(1, "Two")}, "fallback", "H1 One"},
		{"duplicate of frontmatter title removed", "Report", []ast.Block{h(1, "report"), p("x")}, "Report", "Para x"},
		{"different heading kept", "Report", []ast.Block{h(1, "Summary"), p("x")}, "Report", "H1 Summary"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := &ast.Document{Meta: ast.Meta{Title: c.meta}, Blocks: c.blocks}
			Document(doc, Options{FallbackTitle: "fallback"})
			if doc.Meta.Title != c.wantTitle {
				t.Fatalf("title %q, want %q", doc.Meta.Title, c.wantTitle)
			}
			if got := strings.SplitN(ast.Dump(doc.Blocks), "\n", 2)[0]; got != c.wantFirst {
				t.Fatalf("first block %q, want %q", got, c.wantFirst)
			}
		})
	}
}

func TestAbstractKeywordsAndNumbering(t *testing.T) {
	doc := &ast.Document{Blocks: []ast.Block{
		h(2, "Anotācija"), p("Īss kopsavilkums."), p("Atslēgvārdi: augsne; mikrobioms."),
		h(2, "1. Ievads"), p("x"), h(2, "2. Metodes"), h(3, "2.1. Paraugi"),
	}}
	Document(doc, Options{ExtractAbstract: true, FallbackTitle: "T"})
	if ast.BlocksText(doc.Meta.Abstract) != "Īss kopsavilkums." {
		t.Fatalf("abstract %q", ast.BlocksText(doc.Meta.Abstract))
	}
	if strings.Join(doc.Meta.Keywords, "|") != "augsne|mikrobioms" {
		t.Fatalf("keywords %q", doc.Meta.Keywords)
	}
	if doc.Meta.NumberSections == nil || *doc.Meta.NumberSections {
		t.Fatal("manual numbering should disable automatic numbering")
	}
	if first := doc.Blocks[0].(*ast.Heading); first.Level != 1 {
		t.Fatalf("headings not shifted to level 1: %d", first.Level)
	}
}

func TestCrossReferences(t *testing.T) {
	cite := func(mode ast.CiteMode, keys ...string) *ast.Cite {
		c := &ast.Cite{Mode: mode}
		for _, k := range keys {
			c.Items = append(c.Items, ast.CiteItem{Key: k})
		}
		return c
	}
	doc := &ast.Document{Blocks: []ast.Block{
		&ast.Figure{Attr: ast.Attr{ID: "fig:a"}, Image: &ast.Image{Src: "a.png"}},
		&ast.Para{Inlines: []ast.Inline{cite(ast.CiteNarrative, "fig:a"), &ast.Text{Value: " "}, cite(ast.CiteNarrative, "fig:missing"), &ast.Text{Value: " "}, cite(ast.CiteParenthetical, "doe2020")}},
	}}
	warns := Document(doc, Options{FallbackTitle: "T"})
	got := ast.Dump(doc.Blocks)
	if !strings.Contains(got, "{ref fig:a}") || !strings.Contains(got, "**??**") || !strings.Contains(got, "{cite @doe2020}") {
		t.Fatalf("got %s", got)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "fig:missing") {
		t.Fatalf("warnings %v", warns)
	}
}

func TestSanitizeAndUniqueIDs(t *testing.T) {
	if got := SanitizeID(" 2. Ievads / Overview! "); got != "id-2.-Ievads--Overview" {
		t.Fatalf("SanitizeID: %q", got)
	}
	doc := &ast.Document{Blocks: []ast.Block{
		&ast.Heading{Level: 1, Inlines: ast.Str("A"), Attr: ast.Attr{ID: "intro"}},
		&ast.Heading{Level: 1, Inlines: ast.Str("B"), Attr: ast.Attr{ID: "intro"}},
	}}
	Document(doc, Options{FallbackTitle: "T"})
	if doc.Blocks[1].(*ast.Heading).Attr.ID != "intro-2" {
		t.Fatalf("duplicate id not made unique: %s", ast.Dump(doc.Blocks))
	}
}
