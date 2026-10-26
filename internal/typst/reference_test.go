package typst

import (
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

func TestPlainReferenceTitleUsesOnlyAdjacentReferenceHeading(t *testing.T) {
	references := func() *ast.ReferenceList {
		return &ast.ReferenceList{Entries: []ast.ReferenceEntry{{ID: "ref-1", Label: "1", Inlines: ast.Str("Smith. Example work. 2020.")}}}
	}
	heading := func(title string) *ast.Heading {
		return &ast.Heading{Level: 2, Inlines: ast.Str(title), Attr: ast.Attr{ID: "source-heading"}}
	}
	style, _ := LookupStyle("minimal")
	for _, test := range []struct {
		name       string
		meta       ast.Meta
		blocks     []ast.Block
		suppressed int
	}{
		{"matched title and anchor", ast.Meta{}, []ast.Block{heading("Selected bibliography"), references()}, 1},
		{"unrelated heading", ast.Meta{}, []ast.Block{heading("Methods"), references()}, 0},
		{"earlier heading separated by prose", ast.Meta{}, []ast.Block{heading("References"), &ast.Para{Inlines: ast.Str("Closing remarks.")}, references()}, 0},
		{"custom explicit title", ast.Meta{ReferenceSectionTitle: "Primary material"}, []ast.Block{heading("Primary material"), references()}, 1},
		{"nested section", ast.Meta{}, []ast.Block{&ast.Div{Blocks: []ast.Block{heading("Works cited"), references()}}}, 1},
		{"following list has no heading", ast.Meta{}, []ast.Block{heading("References"), references(), references()}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := &ast.Document{Meta: test.meta, Blocks: test.blocks}
			before := ast.Dump(doc.Blocks)
			bundle, err := Render(doc, Options{Style: style})
			if err != nil {
				t.Fatal(err)
			}
			main := string(bundle.Files["main.typ"])
			if got := strings.Count(main, ", show-title: false, numeric:"); got != test.suppressed {
				t.Fatalf("suppressed %d titles, want %d:\n%s", got, test.suppressed, main)
			}
			if !strings.Contains(main, "<source-heading>") {
				t.Fatal("source heading anchor was dropped")
			}
			if got := ast.Dump(doc.Blocks); got != before {
				t.Fatalf("render changed source structure:\n%s", got)
			}
		})
	}
}
