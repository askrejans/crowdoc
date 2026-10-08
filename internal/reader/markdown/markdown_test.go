package markdown

import (
	"context"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func read(t *testing.T, src string) *ast.Document {
	t.Helper()
	doc, _, err := Read(context.Background(), []byte(src), rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func dump(t *testing.T, src string) string {
	t.Helper()
	return ast.Dump(read(t, src).Blocks)
}

func TestFeatures(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
		not       []string
	}{
		{"dollar amounts are not math", "It costs $5 and $10 today.", []string{"$5 and $10"}, []string{"Math"}},
		{"inline math", "Energy $E = mc^2$ here.", []string{"$E = mc^2$"}, nil},
		{"backslash math", `See \(a^2+b^2\) and \[x_i\] but \[1\] stays.`, []string{"$a^2+b^2$", "MathBlock \"x_i\"", "[1] stays"}, nil},
		{"display math with label", "$$\nE=mc^2\n$$ {#eq:e}", []string{`MathBlock "E=mc^2" #eq:e`}, nil},
		{"citations with locators", "As shown [see @doe2020, pp. 33-35; -@roe2019].", []string{"{cite @doe2020,page=33-35 pre=see -@roe2019}"}, nil},
		{"narrative citation with locator", "@smith2021 [chap. 2] argues.", []string{"{cite-n @smith2021,chapter=2}"}, nil},
		{"email is not a citation", "Mail me@example.com now.", []string{"[me@example.com](mailto:me@example.com)"}, []string{"cite"}},
		{"link is not a citation", "[@home](https://x.org)", []string{"[@home](https://x.org)"}, []string{"{cite"}},
		{"fenced div with title", "::: warning Read first\nBody\n:::", []string{"Div{.warning} title=Read first", "Para Body"}, nil},
		{"nested divs", ":::: note\nOuter\n\n::: tip\nInner\n:::\n\nAfter\n::::", []string{"Div{.note}", "  Div{.tip}", "    Para Inner", "  Para After"}, nil},
		{"quarto callout", "::: {.callout-important}\n## Heads up\nText\n:::", []string{"Div{.important} title=Heads up"}, nil},
		{"github alert", "> [!CAUTION]\n> Hot surface.", []string{"Div{.caution}", "Para Hot surface."}, nil},
		{"figure with attributes", "![A *rich* caption](a.png){#fig:a width=60%}", []string{`Figure{#fig:a} src="a.png" w=60% cap=A _rich_ caption`}, nil},
		{"inline image stays inline", "Icon ![i](i.png) here", []string{"Para Icon ![i](i.png) here"}, []string{"Figure"}},
		{"table caption after", "| a | b |\n|---|--:|\n| 1 | 2 |\n\nTable: Numbers {#tbl:n}", []string{"Table{#tbl:n} cols=2 cap=Numbers", "body: |1 |2"}, nil},
		{"table caption before", ": Before\n\n| a |\n|---|\n| 1 |", []string{"cap=Before"}, nil},
		{"footnotes", "Text[^n].\n\n[^n]: The *note*.", []string{"^[Para The _note_.]"}, nil},
		{"inline note", "Text^[inline]. And H^2^O.", []string{"^[Para inline]", "HSuperscript(2)O"}, nil},
		{"task list", "- [x] done\n- [ ] open", []string{"Item[x]", "Item[ ]"}, nil},
		{"inline html", "<sup>1</sup> <kbd>Ctrl</kbd> a<br>b <mark>m</mark>", []string{"Superscript(1)", "Span(Ctrl)", "a⏎b", "Highlight(m)"}, nil},
		{"smart punctuation", "a -- b --- c...", []string{"a – b — c…"}, nil},
		{"code keeps dashes", "`--flag` and\n\n```sh\nrm --force\n```", []string{"`--flag`", `"rm --force"`}, nil},
		{"raw typst block", "```{=typst}\n#lorem(3)\n```", []string{`Raw[typst] "#lorem(3)"`}, nil},
		{"line block", "| one\n|   two", []string{"LineBlock | one | \u00a0\u00a0two"}, nil},
		{"subscript vs strike", "H~2~O and ~~gone~~ and ~a b~", []string{"HSubscript(2)O", "Strike(gone)"}, []string{"Subscript(a"}},
		{"definition list", "Term\n: Definition", []string{"Term Term", "Plain Definition"}, nil},
		{"unnumbered heading", "## Preface {-}\n\n## Intro {#sec:intro .x}", []string{"H2{#preface}[-] Preface", "H2{#sec:intro .x} Intro"}, nil},
		{"reference list placeholder", "::: {#refs}\n:::", []string{"Bibliography"}, nil},
		{"code attributes and caption", "```python {#lst:x .numberLines}\nprint(1)\n```", []string{`Code[python]{#lst:x .numberLines} "print(1)"`}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dump(t, tc.src)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q in:\n%s", w, got)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(got, n) {
					t.Errorf("unexpected %q in:\n%s", n, got)
				}
			}
		})
	}
}

func TestFrontmatter(t *testing.T) {
	doc := read(t, `---
title: "A: B"
author:
  - name: Jane Doe
    affiliation: Faculty of Arts, University of Latvia, Riga
    corresponding: true
  - John Roe
keywords: [x, y]
abstract: |
  The *abstract*.
toc: yes
font-size: 11pt
colors: {scheme: oxford, accent: "#112233"}
client: Acme
---
Body`)
	m := doc.Meta
	if m.Title != "A: B" || len(m.Authors) != 2 || m.Authors[0].Affiliations[0] != "Faculty of Arts, University of Latvia, Riga" || !m.Authors[0].Corresponding {
		t.Fatalf("authors/title: %+v", m)
	}
	if m.TOC == nil || !*m.TOC || m.FontSize != 11 || len(m.Keywords) != 2 {
		t.Fatalf("layout: %+v", m)
	}
	if ast.Dump(m.Abstract) != "Para The _abstract_.\n" {
		t.Fatalf("abstract: %q", ast.Dump(m.Abstract))
	}
	if m.ColorScheme != "oxford" || m.Colors["accent"] != "#112233" || m.Extra["client"] != "Acme" {
		t.Fatalf("colors/extra: %+v %+v", m.Colors, m.Extra)
	}
}

func TestInvalidFrontmatterKeepsText(t *testing.T) {
	doc, warns, err := Read(context.Background(), []byte("---\n: : bad yaml [\n---\nText"), rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) == 0 || !strings.Contains(ast.Dump(doc.Blocks), "Text") {
		t.Fatalf("warns=%v blocks=%s", warns, ast.Dump(doc.Blocks))
	}
}

func TestDataURIImageBecomesResource(t *testing.T) {
	png := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	doc := read(t, "![dot]("+png+")")
	fig, ok := doc.Blocks[0].(*ast.Figure)
	if !ok || !strings.HasPrefix(fig.Image.Src, "res:") || doc.Resources.Len() != 1 {
		t.Fatalf("got %s", ast.Dump(doc.Blocks))
	}
}

func TestDecodeText(t *testing.T) {
	if got := DecodeText([]byte("\xef\xbb\xbfa\r\nb\rc")); got != "a\nb\nc" {
		t.Fatalf("%q", got)
	}
	if got := DecodeText([]byte{'c', 'a', 'f', 0xE9}); got != "café" {
		t.Fatalf("windows-1252 fallback: %q", got)
	}
	if got := DecodeText([]byte{0xFF, 0xFE, 'h', 0, 'i', 0}); got != "hi" {
		t.Fatalf("utf-16: %q", got)
	}
}
