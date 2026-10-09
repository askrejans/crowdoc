package crowdoc

import (
	"context"
	"sort"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/pipeline"
)

// Outline summarises a parsed document without typesetting it.
type Outline struct {
	Title string `json:"title"`
	// TitleFromName reports that Title was derived from the file name
	// (the document names itself nowhere).
	TitleFromName bool          `json:"title_from_name,omitempty"`
	Subtitle      string        `json:"subtitle,omitempty"`
	Authors       []string      `json:"authors,omitempty"`
	Language      string        `json:"language,omitempty"`
	Format        Format        `json:"format"`
	Style         string        `json:"style"`
	Keywords      []string      `json:"keywords,omitempty"`
	Abstract      string        `json:"abstract,omitempty"`
	Headings      []HeadingInfo `json:"headings"`
	Counts        Counts        `json:"counts"`
	Citations     []string      `json:"citation_keys,omitempty"`
	Images        []string      `json:"images,omitempty"`
	Warnings      []string      `json:"warnings,omitempty"`
}

// HeadingInfo is one entry of the document outline.
type HeadingInfo struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	ID    string `json:"id,omitempty"`
}

// Counts tallies document elements.
type Counts struct {
	Words           int `json:"words"`
	Paragraphs      int `json:"paragraphs"`
	Headings        int `json:"headings"`
	Lists           int `json:"lists"`
	Tables          int `json:"tables"`
	Figures         int `json:"figures"`
	Images          int `json:"images"`
	Equations       int `json:"equations"`
	CodeBlocks      int `json:"code_blocks"`
	Footnotes       int `json:"footnotes"`
	Citations       int `json:"citations"`
	CrossReferences int `json:"cross_references"`
	Callouts        int `json:"callouts"`
}

// Inspect parses src and summarises its structure.
func Inspect(ctx context.Context, src Source, opts Options) (*Outline, error) {
	p, err := pipeline.Prepare(ctx, src.internal(), opts.internal())
	if err != nil {
		return nil, err
	}
	doc := p.Doc()
	o := &Outline{
		Title: doc.Meta.Title, TitleFromName: doc.Meta.TitleFromName, Subtitle: doc.Meta.Subtitle, Authors: doc.Meta.AuthorNames(),
		Format: p.Format(), Style: p.StyleName(), Keywords: doc.Meta.Keywords,
		Abstract: ast.BlocksText(doc.Meta.Abstract), Warnings: p.Warnings(),
		Language: pipeline.Language(p, opts.internal()),
	}
	keys := map[string]bool{}
	images := map[string]bool{}
	ast.WalkBlocks(doc.Blocks, func(b ast.Block) bool {
		switch n := b.(type) {
		case *ast.Heading:
			o.Counts.Headings++
			o.Headings = append(o.Headings, HeadingInfo{Level: n.Level, Text: ast.PlainText(n.Inlines), ID: n.Attr.ID})
		case *ast.Para:
			o.Counts.Paragraphs++
			o.Counts.Words += countWords(ast.PlainText(n.Inlines))
		case *ast.Plain:
			o.Counts.Words += countWords(ast.PlainText(n.Inlines))
		case *ast.List, *ast.DefinitionList:
			o.Counts.Lists++
		case *ast.Table:
			o.Counts.Tables++
		case *ast.Figure:
			o.Counts.Figures++
		case *ast.MathBlock:
			o.Counts.Equations++
		case *ast.CodeBlock:
			o.Counts.CodeBlocks++
		case *ast.Div:
			o.Counts.Callouts++
		}
		return true
	})
	ast.WalkInlines(doc.Blocks, func(in ast.Inline) {
		switch n := in.(type) {
		case *ast.Image:
			o.Counts.Images++
			images[n.Src] = true
		case *ast.Note:
			o.Counts.Footnotes++
		case *ast.Cite:
			o.Counts.Citations++
			for _, it := range n.Items {
				keys[it.Key] = true
			}
		case *ast.Ref:
			o.Counts.CrossReferences++
		case *ast.Math:
			o.Counts.Equations++
		}
	})
	o.Citations = sortedKeys(keys)
	o.Images = sortedKeys(images)
	return o, nil
}

func countWords(s string) int {
	n, in := 0, false
	for _, r := range s {
		space := r == ' ' || r == '\t' || r == '\n' || r == ' '
		if !space && !in {
			n++
		}
		in = !space
	}
	return n
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
