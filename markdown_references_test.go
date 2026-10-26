package crowdoc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

const retainedBib = `@book{external,
  title = {Retained study},
  author = {Smith, Alice},
  year = {2024},
  publisher = {Example Press}
}`

func TestMarkdownEmbedsExternalBibliography(t *testing.T) {
	for _, cited := range []bool{true, false} {
		name := "cited"
		if !cited {
			name = "available for later editing"
		}
		t.Run(name, func(t *testing.T) {
			input := t.TempDir()
			if err := os.WriteFile(filepath.Join(input, "references.bib"), []byte(retainedBib), 0o600); err != nil {
				t.Fatal(err)
			}
			body := "A note for later editing."
			if cited {
				body = "Earlier work [@external] supports this result."
			}
			ctx := context.Background()
			res, err := Markdown(ctx, Source{Name: "paper.md", BaseDir: input, Untrusted: true,
				Data: []byte("---\ntitle: Portable research\nbibliography: references.bib\n---\n\n" + body)}, Options{}, MarkdownOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Warnings) != 0 || strings.Contains(res.Text, "bibliography:") || !strings.Contains(res.Text, "Retained study") {
				t.Fatalf("export did not embed its bibliography: %v\n%s", res.Warnings, res.Text)
			}
			out := filepath.Join(t.TempDir(), "paper.md")
			if err := WriteMarkdown(res, out); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(input); err != nil {
				t.Fatal(err)
			}
			again, err := Markdown(ctx, Source{Path: out, Untrusted: true}, Options{}, MarkdownOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(again.Warnings) != 0 || again.Text != res.Text {
				t.Fatalf("portable export changed: %v, warnings %v\n%s", err, again.Warnings, again.Text)
			}
			portable := again.Text
			if !cited {
				portable += "\nA later citation [@external].\n"
			}
			rendered, err := Render(ctx, Source{Name: "portable.md", Data: []byte(portable), Untrusted: true}, Options{})
			if err != nil {
				t.Fatal(err)
			}
			for _, warning := range rendered.Warnings {
				if strings.Contains(warning, "bibliography") || strings.Contains(warning, "citation key") {
					t.Fatalf("reference depended on removed source files: %s", warning)
				}
			}
			if !strings.Contains(string(rendered.Bundle.Files["main.typ"]), "Example Press") {
				t.Fatal("portable bibliography entry was not rendered")
			}
		})
	}
}

func TestMarkdownMergesCallerAndInlineReferences(t *testing.T) {
	ctx := context.Background()
	res, err := Markdown(ctx, Source{Name: "paper.md", Untrusted: true, Data: []byte(`---
title: Research
bibliography: references.bib
references:
  - id: external
    type: book
    title: Superseded inline definition
  - id: inline
    type: book
    title: Inline work
---

Earlier work [@external; @inline].
`)}, Options{BibliographyData: map[string][]byte{"references.bib": []byte(retainedBib)}}, MarkdownOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, "bibliography:") || strings.Contains(res.Text, "Superseded inline definition") ||
		!strings.Contains(res.Text, "Retained study") || !strings.Contains(res.Text, "Inline work") {
		t.Fatalf("caller precedence or inline references were lost:\n%s", res.Text)
	}
	again, err := Markdown(ctx, Source{Name: "roundtrip.md", Data: []byte(res.Text), Untrusted: true}, Options{}, MarkdownOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Warnings) != 0 || again.Text != res.Text {
		t.Fatalf("embedded merge is not portable: %v, %v", err, again.Warnings)
	}
}

func TestMarkdownPortableBibliographyRenders(t *testing.T) {
	eng := typstEngine(t)
	ctx := context.Background()
	exported, err := Markdown(ctx, Source{Name: "paper.md", Untrusted: true,
		Data: []byte("# Research\n\nEarlier work [@external] supports this result.\n")},
		Options{BibliographyData: map[string][]byte{"references.bib": []byte(retainedBib)}}, MarkdownOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Convert(ctx, Source{Name: "portable.md", Data: []byte(exported.Text), Untrusted: true}, Options{Style: "minimal", Engine: eng})
	if err != nil {
		t.Fatal(err)
	}
	back, _, err := Parse(ctx, Source{Name: "portable.pdf", Data: rendered.PDF}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	text := ast.BlocksText(back.Blocks)
	ast.WalkBlocks(back.Blocks, func(block ast.Block) bool {
		if refs, ok := block.(*ast.ReferenceList); ok {
			for _, entry := range refs.Entries {
				text += " " + ast.PlainText(entry.Inlines)
			}
		}
		return true
	})
	for _, expected := range []string{"Smith", "2024", "Retained study", "Example Press"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("portable PDF lost bibliography content %q: %s", expected, text)
		}
	}
}

func TestMarkdownKeepsUnresolvedBibliographyConfined(t *testing.T) {
	input := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.bib")
	if err := os.WriteFile(outside, []byte(retainedBib), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Markdown(context.Background(), Source{Name: "paper.md", BaseDir: input, Untrusted: true,
		Data: []byte("---\nbibliography: '" + filepath.ToSlash(outside) + "'\n---\n\nNote.")}, Options{}, MarkdownOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) == 0 || strings.Contains(res.Text, "Retained study") || !strings.Contains(res.Text, "bibliography:") {
		t.Fatalf("untrusted export read or discarded an inaccessible bibliography: %v\n%s", res.Warnings, res.Text)
	}
}
