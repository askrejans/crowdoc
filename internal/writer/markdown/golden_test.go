package markdown

import (
	"context"
	htmlreader "github.com/askrejans/crowdoc/v2/internal/reader/html"
	mdreader "github.com/askrejans/crowdoc/v2/internal/reader/markdown"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/writer/markdown/mdtest"
)

func para(ins ...ast.Inline) *ast.Para   { return &ast.Para{Inlines: ins} }
func plain(ins ...ast.Inline) *ast.Plain { return &ast.Plain{Inlines: ins} }

// sampleDocument uses every construct the writer supports.
func sampleDocument() *ast.Document {
	yes := true
	cell := func(s string) ast.Cell { return ast.Cell{Blocks: []ast.Block{plain(txt(s))}} }
	return &ast.Document{
		Meta: ast.Meta{
			Title: "Field notes: spring", Subtitle: "A #1 \"survey\"",
			Authors: []ast.Author{{Name: "Anna Kalniņa", Affiliations: []string{"Institute, Rīga"}, Email: "anna@example.org"}, {Name: "Jānis Bērziņš"}},
			Date:    "2026-05-01", Lang: "lv", Keywords: []string{"soil", "- moisture"},
			Abstract: []ast.Block{para(txt("Short "), &ast.Emph{Inlines: []ast.Inline{txt("abstract")}}, txt("."))},
			TOC:      &yes, ShowTitle: &yes, Status: "DRAFT",
			Extra: map[string]any{"client": "Acme: North"},
		},
		Blocks: []ast.Block{
			&ast.Heading{Level: 1, Inlines: []ast.Inline{txt("Introduction")}, Attr: ast.Attr{ID: "introduction"}},
			para(txt("Text with "), &ast.Strong{Inlines: []ast.Inline{txt("bold")}}, txt(", "), &ast.Emph{Inlines: []ast.Inline{txt("italic")}},
				txt(", "), &ast.Strike{Inlines: []ast.Inline{txt("struck")}}, txt(", "), &ast.Highlight{Inlines: []ast.Inline{txt("marked")}},
				txt(", H"), &ast.Subscript{Inlines: []ast.Inline{txt("2")}}, txt("O, x"), &ast.Superscript{Inlines: []ast.Inline{txt("2")}},
				txt(", "), &ast.Underline{Inlines: []ast.Inline{txt("underlined")}}, txt(", "), &ast.SmallCaps{Inlines: []ast.Inline{txt("Small Caps")}},
				txt(", "), &ast.Code{Text: "a `tick`"}, txt(", "), &ast.Math{TeX: `E = mc^2`}, txt(" and "),
				&ast.Link{URL: "https://example.org/a b", Title: "Example", Inlines: []ast.Inline{txt("a link")}}, txt(" or "),
				&ast.Link{URL: "https://example.org", Inlines: []ast.Inline{txt("https://example.org")}}, txt("."), &ast.LineBreak{},
				txt("Costs $5 * 2 = $10, see [1] and @home; a -- b..."),
				&ast.Note{Blocks: []ast.Block{para(txt("A note.")), para(txt("Second paragraph."))}},
			),
			para(txt("As "), &ast.Cite{Mode: ast.CiteNarrative, Items: []ast.CiteItem{{Key: "doe2020"}}}, txt(" shows "),
				&ast.Cite{Items: []ast.CiteItem{{Key: "roe2019", Prefix: "see", Locator: "33-35", LocatorLabel: "page"}, {Key: "poe", SuppressAuthor: true}}},
				txt("; compare "), &ast.Ref{Target: "fig:field"}, txt(" and "), &ast.Ref{Target: "tbl:farms", Bare: true}, txt(".")),
			&ast.Heading{Level: 2, Inlines: []ast.Inline{txt("Lists")}, Attr: ast.Attr{ID: "sec:lists"}, Unnumbered: true},
			&ast.List{Tight: true, Items: []ast.ListItem{
				{Blocks: []ast.Block{plain(txt("one"))}},
				{Blocks: []ast.Block{plain(txt("two")), &ast.List{Ordered: true, Start: 1, Tight: true, Items: []ast.ListItem{
					{Blocks: []ast.Block{plain(txt("three"))}}, {Blocks: []ast.Block{plain(txt("four"))}}}}}},
			}},
			&ast.List{Ordered: true, Start: 9, Items: []ast.ListItem{{Blocks: []ast.Block{para(txt("nine"))}}, {Blocks: []ast.Block{para(txt("ten"))}}}},
			&ast.List{Tight: true, Items: []ast.ListItem{{Task: ast.TaskDone, Blocks: []ast.Block{plain(txt("done"))}}, {Task: ast.TaskOpen, Blocks: []ast.Block{plain(txt("open"))}}}},
			&ast.DefinitionList{Items: []ast.DefinitionItem{{Term: []ast.Inline{txt("Term")}, Definitions: [][]ast.Block{{plain(txt("Definition."))}}}}},
			&ast.CodeBlock{Lang: "go", Attr: ast.Attr{ID: "lst:main"}, Caption: []ast.Inline{txt("Main")}, Text: "func main() {\n\t// ```\n}"},
			&ast.MathBlock{TeX: "\\int_0^1 x\\,dx\n= \\frac{1}{2}", Label: "eq:int"},
			&ast.BlockQuote{Blocks: []ast.Block{para(txt("Quoted.")), &ast.List{Items: []ast.ListItem{{Blocks: []ast.Block{para(txt("in quote"))}}}}}},
			&ast.Table{Attr: ast.Attr{ID: "tbl:farms"}, Caption: []ast.Inline{txt("Farms")}, Cols: []ast.ColSpec{{Align: ast.AlignLeft}, {Align: ast.AlignRight}},
				Head: []ast.Row{{Cells: []ast.Cell{cell("Farm"), cell("Fields")}}},
				Body: []ast.Row{{Cells: []ast.Cell{cell("Ozolkalni"), cell("18")}}, {Cells: []ast.Cell{cell("中文 | pipe"), cell("4")}}}},
			&ast.Table{Cols: []ast.ColSpec{{}, {}, {}}, Head: []ast.Row{
				{Cells: []ast.Cell{{Blocks: []ast.Block{plain(txt("Item"))}, RowSpan: 2}, {Blocks: []ast.Block{plain(txt("Price"))}, ColSpan: 2}}},
				{Cells: []ast.Cell{cell("Net"), cell("Gross")}}},
				Body: []ast.Row{{Cells: []ast.Cell{cell("Licence"), cell("9.99"), {Blocks: []ast.Block{plain(txt("12.09"), &ast.Note{Blocks: []ast.Block{para(txt("Incl. tax."))}})}}}}}},
			&ast.Figure{Attr: ast.Attr{ID: "fig:field"}, Image: &ast.Image{Src: "https://example.org/field.jpg", Alt: "A field", Width: "60%"}, Caption: []ast.Inline{txt("The "), &ast.Emph{Inlines: []ast.Inline{txt("field")}}}},
			&ast.Div{Attr: ast.Attr{Classes: []string{"warning"}}, Title: []ast.Inline{txt("Careful")}, Blocks: []ast.Block{para(txt("Hot surface."))}},
			&ast.Div{Attr: ast.Attr{Classes: []string{"note"}, ID: "n1"}, Blocks: []ast.Block{&ast.Div{Attr: ast.Attr{Classes: []string{"tip"}}, Blocks: []ast.Block{para(txt("Nested."))}}}},
			&ast.LineBlock{Lines: [][]ast.Inline{{txt("Roses are red,")}, {txt("  violets blue.")}}},
			&ast.PageBreak{},
			&ast.HorizontalRule{},
			&ast.RawBlock{Format: "typst", Text: "#v(1em)"},
			&ast.Heading{Level: 1, Inlines: []ast.Inline{txt("References")}, Attr: ast.Attr{ID: "references"}},
			&ast.Bibliography{},
		},
	}
}

// sampleMarkdown is the expected output (‵ stands for a backtick).
var sampleMarkdown = strings.ReplaceAll(`---
title: 'Field notes: spring'
subtitle: 'A #1 "survey"'
author:
  - name: Anna Kalniņa
    affiliation: Institute, Rīga
    email: anna@example.org
  - Jānis Bērziņš
date: "2026-05-01"
status: DRAFT
lang: lv
keywords:
  - soil
  - '- moisture'
abstract: |-
  Short *abstract*.
toc: true
show-title: true
client: 'Acme: North'
---

# Introduction

Text with **bold**, *italic*, ~~struck~~, ==marked==, H~2~O, x^2^, <u>underlined</u>, <span class="smallcaps">Small Caps</span>, ‵‵ a ‵tick‵ ‵‵, $E = mc^2$ and [a link](<https://example.org/a b> "Example") or <https://example.org>.\
Costs \$5 * 2 = \$10, see \[1] and \@home; a -\- b.\.\.[^1]

As @doe2020 shows [see @roe2019, p. 33-35; -@poe]; compare @fig:field and [-@tbl:farms].

## Lists {-} {#sec:lists}

- one
- two
  1. three
  2. four

9. nine

10. ten

- [x] done
- [ ] open

Term
: Definition.

Listing: Main

‵‵‵‵go {#lst:main}
func main() {
	// ‵‵‵
}
‵‵‵‵

$$
\int_0^1 x\,dx = \frac{1}{2}
$$ {#eq:int}

> Quoted.
>
> - in quote

Table: Farms {#tbl:farms}

| Farm         | Fields |
| :----------- | -----: |
| Ozolkalni    |     18 |
| 中文 \| pipe |      4 |

<table>
<thead>
<tr>
<th rowspan="2">Item</th>
<th colspan="2">Price</th>
</tr>
<tr>
<th>Net</th>
<th>Gross</th>
</tr>
</thead>
<tbody>
<tr>
<td>Licence</td>
<td>9.99</td>
<td>12.09<a href="#cd-note-1" role="doc-noteref">1</a></td>
</tr>
</tbody>
</table>
<aside id="cd-note-1" role="doc-footnote">
<p>Incl. tax.</p>
</aside>

![The *field*](https://example.org/field.jpg){#fig:field fig-alt="A field" width=60%}

::: warning Careful
Hot surface.
:::

:::: {#n1 .note}
::: tip
Nested.
:::
::::

| Roses are red,
|   violets blue.

\newpage

---

‵‵‵{=typst}
#v(1em)
‵‵‵

# References

::: {#refs}
:::

[^1]: A note.

    Second paragraph.
`, "‵", "`")

func TestGolden(t *testing.T) {
	doc := sampleDocument()
	res, err := Write(context.Background(), doc, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != sampleMarkdown {
		t.Errorf("output differs:\n%s", firstDiff(sampleMarkdown, res.Text))
	}
	back := read(t, res.Text)
	mdtest.ClearAutoIDs(doc.Blocks, back.Blocks)
	if want, got := mdtest.Summary(doc), mdtest.Summary(back); want != got {
		t.Errorf("round trip differs\n%s", firstDiff(want, got))
	}
}

func TestTitleFromNameIsNotWritten(t *testing.T) {
	no := false
	doc := &ast.Document{Meta: ast.Meta{Title: "Report", TitleFromName: true, ShowTitle: &no}, Blocks: []ast.Block{para(txt("Body"))}}
	text := write(t, doc)
	if strings.Contains(text, "title: Report") || !strings.Contains(text, "show-title: false") {
		t.Errorf("frontmatter:\n%s", text)
	}
	if back := read(t, text); back.Meta.Title != "" || back.Meta.ShowTitle == nil || *back.Meta.ShowTitle {
		t.Errorf("meta: %+v", back.Meta)
	}
	text = write(t, &ast.Document{Meta: ast.Meta{Title: "Report", TitleFromName: true}, Blocks: []ast.Block{para(txt("Body"))}})
	if text != "Body\n" {
		t.Errorf("got %q", text)
	}
}

func TestPageBreakRoundTrip(t *testing.T) {
	blocks := []ast.Block{para(txt("a")), &ast.PageBreak{}, para(txt(`\newpage`))}
	text, _ := roundTrip(t, blocks)
	if text != "a\n\n\\newpage\n\n\\\\newpage\n" {
		t.Errorf("got %q", text)
	}
}

func init() { mdreader.HTMLFragment = htmlreader.Fragment }
