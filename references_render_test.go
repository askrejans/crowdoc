package crowdoc

import (
	"context"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

const plainReferencesSource = `# Research note

## Introduction

Earlier work [1] supports this observation. See [the source list](#reading-list).

## Selected bibliography {#reading-list}

[1] Smith, A. (2020). An example work. Example Press.

## Appendix

Additional observations remain here.
`

func TestPlainReferenceHeadingSurvivesMarkdown(t *testing.T) {
	ctx := context.Background()
	source := Source{Name: "research.md", Data: []byte(plainReferencesSource)}
	res, err := Markdown(ctx, source, Options{}, MarkdownOptions{NoFrontmatter: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(res.Text, "Selected bibliography") != 1 || !strings.Contains(res.Text, "#reading-list") {
		t.Fatalf("reference heading or anchor changed:\n%s", res.Text)
	}
	back, _, err := Parse(ctx, Source{Name: "roundtrip.md", Data: []byte(res.Text)}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	headings, lists := 0, 0
	ast.WalkBlocks(back.Blocks, func(block ast.Block) bool {
		switch b := block.(type) {
		case *ast.Heading:
			if ast.PlainText(b.Inlines) == "Selected bibliography" {
				headings++
				if b.Attr.ID != "reading-list" {
					t.Errorf("reference heading anchor = %q", b.Attr.ID)
				}
			}
		case *ast.ReferenceList:
			lists++
		}
		return true
	})
	if headings != 1 || lists != 1 || !strings.Contains(ast.BlocksText(back.Blocks), "Additional observations remain here.") {
		t.Fatalf("reference section or appendix changed: %d headings, %d lists", headings, lists)
	}
}

func TestPlainReferenceHeadingRendersOnce(t *testing.T) {
	eng := typstEngine(t)
	ctx := context.Background()
	res, err := Convert(ctx, Source{Name: "research.md", Data: []byte(plainReferencesSource)}, Options{Style: "minimal", Engine: eng, Meta: map[string]any{"show-header": false, "show-footer": false}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(res.Bundle.Files["main.typ"]), ", show-title: false, numeric:") != 1 {
		t.Fatal("plain reference list did not reuse its source heading")
	}
	pdf, _, err := Parse(ctx, Source{Name: "research.pdf", Data: res.PDF}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	text := ast.BlocksText(pdf.Blocks)
	ast.WalkBlocks(pdf.Blocks, func(block ast.Block) bool {
		if refs, ok := block.(*ast.ReferenceList); ok {
			for _, entry := range refs.Entries {
				text += " " + ast.PlainText(entry.Inlines)
			}
		}
		return true
	})
	text = strings.ToLower(strings.Join(strings.Fields(text), " "))
	if strings.Count(text, "selected bibliography") != 1 || strings.Contains(text, "references") {
		t.Fatalf("rendered bibliography title duplicated or changed: %s", text)
	}
	if !strings.Contains(text, "additional observations remain here") || !strings.Contains(text, "an example work") {
		t.Fatalf("reference or following body disappeared: %s", text)
	}
}
