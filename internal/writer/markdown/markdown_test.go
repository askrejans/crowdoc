package markdown

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	mdreader "github.com/askrejans/crowdoc/v2/internal/reader/markdown"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
	"github.com/askrejans/crowdoc/v2/internal/writer/markdown/mdtest"
)

func write(t testing.TB, doc *ast.Document) string {
	t.Helper()
	res, err := Write(context.Background(), doc, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return res.Text
}

func read(t testing.TB, text string) *ast.Document {
	t.Helper()
	doc, _, err := mdreader.Read(context.Background(), []byte(text), rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// roundTrip writes blocks, reads them back and compares the summaries.
func roundTrip(t testing.TB, blocks []ast.Block) (string, bool) {
	t.Helper()
	text := write(t, &ast.Document{Blocks: blocks})
	back := read(t, text)
	mdtest.ClearAutoIDs(blocks, back.Blocks)
	want, got := mdtest.Blocks(blocks), mdtest.Blocks(back.Blocks)
	if want != got {
		t.Errorf("round trip differs\n%s\nmarkdown: %q", firstDiff(want, got), text)
		return text, false
	}
	return text, true
}

// firstDiff shows the first differing line of two summaries.
func firstDiff(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(al) || i < len(bl); i++ {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y {
			return fmt.Sprintf("line %d\nwant: %q\ngot:  %q", i+1, x, y)
		}
	}
	return ""
}

func txt(s string) *ast.Text { return &ast.Text{Value: s} }

// pieces mixes letters with diacritics, CJK, right-to-left text, digits,
// emoji, white space and every ASCII punctuation character.
var pieces = func() []string {
	p := strings.Split("a b c x y z A Q Z ā č ē ģ ī ķ ļ ņ š ū ž é ñ ü 中 文 字 ש ל ו م ر ح 0 1 2 9 ✓ 😀", " ")
	for c := byte('!'); c <= '~'; c++ {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			p = append(p, string(c), string(c))
		}
	}
	return append(p, " ", " ", " ", "\t", " ", "http://", "https://x.org", "www.", "a@b.co", "@key", "--", "...", "&amp;", "&#91;",
		"Table:", "Listing:", "\\newpage", "[!NOTE]", "{-}", "{#id}", "[^1]", "^[", "$$", "\\[", "\\]", "\\(", "<b>", "<!--", "-->", "```", "~~~", ":::", "| ", "1.", "1)", "- ", "+ ", "* ", "# ", "> ")
}()

func randomText(r *rand.Rand) string {
	n := r.IntN(14)
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(pieces[r.IntN(len(pieces))])
	}
	return sb.String()
}

// TestEscapingRoundTrip writes random text in every inline and block
// context and checks the reader returns exactly the same text.
func TestEscapingRoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	contexts := []struct {
		name string
		make func(a, b, c string) []ast.Block
	}{
		{"para", func(a, b, c string) []ast.Block { return []ast.Block{&ast.Para{Inlines: []ast.Inline{txt(a + b + c)}}} }},
		{"emph", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.Para{Inlines: []ast.Inline{txt(a), &ast.Emph{Inlines: []ast.Inline{txt(b)}}, txt(c)}}}
		}},
		{"strong", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.Para{Inlines: []ast.Inline{txt(a), &ast.Strong{Inlines: []ast.Inline{txt(b)}}, txt(c)}}}
		}},
		{"nested", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.Para{Inlines: []ast.Inline{&ast.Strong{Inlines: []ast.Inline{txt(a), &ast.Emph{Inlines: []ast.Inline{txt(b)}}}}, &ast.Strike{Inlines: []ast.Inline{txt(c)}}}}}
		}},
		{"formatting", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.Para{Inlines: []ast.Inline{
				&ast.Superscript{Inlines: []ast.Inline{txt(a)}}, &ast.Subscript{Inlines: []ast.Inline{txt(b)}},
				&ast.Highlight{Inlines: []ast.Inline{txt(c)}}, &ast.Underline{Inlines: []ast.Inline{txt(a)}},
				&ast.SmallCaps{Inlines: []ast.Inline{txt(b)}}, txt(c)}}}
		}},
		{"link", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.Para{Inlines: []ast.Inline{txt(a), &ast.Link{URL: "https://example.org/" + strings.TrimSpace(c), Title: c, Inlines: []ast.Inline{txt(b)}}, txt(c)}}}
		}},
		{"code and math", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.Para{Inlines: []ast.Inline{txt(a), &ast.Code{Text: b}, txt(c), &ast.Math{TeX: "x^2"}, txt(a)}}}
		}},
		{"line breaks", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.Para{Inlines: []ast.Inline{txt(a), &ast.LineBreak{}, txt(b), &ast.LineBreak{}, txt(c)}}}
		}},
		{"heading", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.Heading{Level: 2, Inlines: []ast.Inline{txt(a + b + c)}}, &ast.Para{Inlines: []ast.Inline{txt(c)}}}
		}},
		{"table", func(a, b, c string) []ast.Block {
			cell := func(s string) ast.Cell {
				return ast.Cell{Blocks: []ast.Block{&ast.Plain{Inlines: []ast.Inline{txt(s)}}}}
			}
			return []ast.Block{&ast.Table{Caption: []ast.Inline{txt(c)}, Cols: []ast.ColSpec{{}, {}},
				Head: []ast.Row{{Cells: []ast.Cell{cell(a), cell(b)}}}, Body: []ast.Row{{Cells: []ast.Cell{cell(c), cell(a + b)}}}}}
		}},
		{"list", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.List{Tight: true, Items: []ast.ListItem{
				{Blocks: []ast.Block{&ast.Plain{Inlines: []ast.Inline{txt(a)}}}},
				{Blocks: []ast.Block{&ast.Plain{Inlines: []ast.Inline{txt(b)}}}},
			}}, &ast.Para{Inlines: []ast.Inline{txt(c)}}}
		}},
		{"quote and definitions", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.BlockQuote{Blocks: []ast.Block{&ast.Para{Inlines: []ast.Inline{txt(a)}}}},
				&ast.DefinitionList{Items: []ast.DefinitionItem{{Term: []ast.Inline{txt(b)}, Definitions: [][]ast.Block{{&ast.Plain{Inlines: []ast.Inline{txt(c)}}}}}}}}
		}},
		{"figure", func(a, b, c string) []ast.Block {
			// fig-alt attributes cannot hold braces or both kinds of quotes.
			clean := func(s string) string {
				return strings.Map(func(r rune) rune {
					if strings.ContainsRune("{}\"'", r) {
						return -1
					}
					return r
				}, s)
			}
			a, b = clean(a), clean(b)
			return []ast.Block{&ast.Figure{Image: &ast.Image{Src: "https://example.org/a.png", Alt: a}, Caption: []ast.Inline{txt(b)}}, &ast.Para{Inlines: []ast.Inline{&ast.Image{Src: "https://example.org/b.png", Alt: b}, txt(c)}}}
		}},
		{"note and cite", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.Para{Inlines: []ast.Inline{txt(a), &ast.Note{Blocks: []ast.Block{&ast.Para{Inlines: []ast.Inline{txt(b)}}}},
				&ast.Cite{Items: []ast.CiteItem{{Key: "doe2020"}}}, txt(c), &ast.Cite{Mode: ast.CiteNarrative, Items: []ast.CiteItem{{Key: "roe"}}}, txt(b)}}}
		}},
		{"line block", func(a, b, c string) []ast.Block {
			return []ast.Block{&ast.LineBlock{Lines: [][]ast.Inline{{txt(a)}, {txt(b)}, {txt(c)}}}}
		}},
	}
	n := 2500
	if testing.Short() {
		n = 300
	}
	for _, cx := range contexts {
		t.Run(cx.name, func(t *testing.T) {
			fails := 0
			for i := 0; i < n && fails < 5; i++ {
				a, b, c := randomText(r), randomText(r), randomText(r)
				if _, ok := roundTrip(t, cx.make(a, b, c)); !ok {
					fails++
				}
			}
		})
	}
}

// randomInlines builds a random inline sequence of every kind the writer
// handles.
func randomInlines(r *rand.Rand, depth int) []ast.Inline { return randomInlinesIn(r, depth, false) }

func randomInlinesIn(r *rand.Rand, depth int, inLink bool) []ast.Inline {
	n := 1 + r.IntN(4)
	var out []ast.Inline
	for i := 0; i < n; i++ {
		k := r.IntN(22)
		if depth >= 3 && k >= 8 {
			k = 0
		}
		kids := func() []ast.Inline { return randomInlinesIn(r, depth+1, inLink) }
		switch k {
		case 0, 1, 2, 3:
			out = append(out, txt(randomText(r)))
		case 4:
			out = append(out, &ast.Code{Text: randomText(r)})
		case 5:
			out = append(out, &ast.Math{TeX: []string{"x^2", `\alpha + \beta`, "a|b|c", `\{x\}`, "1"}[r.IntN(5)]})
		case 6:
			out = append(out, &ast.LineBreak{})
		case 7:
			out = append(out, &ast.SoftBreak{})
		case 8:
			out = append(out, &ast.Emph{Inlines: kids()})
		case 9:
			out = append(out, &ast.Strong{Inlines: kids()})
		case 10:
			out = append(out, &ast.Strike{Inlines: kids()})
		case 11:
			out = append(out, &ast.Superscript{Inlines: kids()})
		case 12:
			out = append(out, &ast.Subscript{Inlines: kids()})
		case 13:
			out = append(out, &ast.Highlight{Inlines: kids()})
		case 14:
			out = append(out, &ast.Underline{Inlines: kids()})
		case 15:
			out = append(out, &ast.SmallCaps{Inlines: kids()})
		case 16:
			out = append(out, &ast.Span{Attr: ast.Attr{ID: "s1", Classes: []string{"x"}, KV: map[string]string{"data-k": randomText(r)}}, Inlines: kids()})
		case 17:
			if !inLink {
				out = append(out, &ast.Link{URL: "https://example.org/" + randomText(r), Inlines: randomInlinesIn(r, depth+1, true)})
			}
		case 18:
			out = append(out, &ast.Image{Src: "https://example.org/i.png", Alt: randomText(r), Width: "50%"})
		case 19:
			if depth == 0 && !inLink {
				note := &ast.Note{Blocks: []ast.Block{&ast.Para{Inlines: randomInlines(r, 2)}}}
				if r.IntN(3) == 0 {
					note.Blocks = append(note.Blocks, &ast.List{Tight: true, Items: []ast.ListItem{{Blocks: []ast.Block{&ast.Plain{Inlines: randomInlines(r, 2)}}}}},
						&ast.CodeBlock{Text: "x = 1\n\ny = 2"})
				}
				out = append(out, note)
			}
		case 20:
			if inLink {
				continue // citations in link text are text
			}
			out = append(out, &ast.Cite{Mode: ast.CiteMode(r.IntN(2)), Items: []ast.CiteItem{{Key: "doe2020", Locator: "33", LocatorLabel: "page"}}})
		case 21:
			if inLink {
				continue
			}
			out = append(out, &ast.Ref{Target: "fig:x", Bare: r.IntN(2) == 0})
		}
	}
	return out
}

// randomBlocks builds a random block structure.
func randomBlocks(r *rand.Rand, depth int) []ast.Block {
	n := 1 + r.IntN(3)
	var out []ast.Block
	for i := 0; i < n; i++ {
		k := r.IntN(15)
		if depth >= 3 && k >= 4 {
			k = 0
		}
		switch k {
		case 0, 1:
			out = append(out, &ast.Para{Inlines: randomInlines(r, 0)})
		case 2:
			out = append(out, &ast.Heading{Level: 1 + r.IntN(3), Inlines: randomInlines(r, 1)})
		case 3:
			out = append(out, &ast.CodeBlock{Lang: "go", Text: randomText(r) + "\n```\n" + randomText(r)})
		case 4:
			out = append(out, &ast.BlockQuote{Blocks: randomBlocks(r, depth+1)})
		case 5, 6:
			l := &ast.List{Ordered: r.IntN(2) == 0, Start: r.IntN(12), Tight: r.IntN(2) == 0}
			for j := 0; j < 1+r.IntN(3); j++ {
				it := ast.ListItem{Blocks: randomBlocks(r, depth+1)}
				if _, ok := it.Blocks[0].(*ast.Para); ok {
					it.Task = ast.TaskState(r.IntN(3))
				}
				l.Items = append(l.Items, it)
			}
			out = append(out, l)
		case 7:
			// Titles cannot end in a run of fence colons.
			title := strings.TrimRight(randomText(r), ": \t")
			out = append(out, &ast.Div{Attr: ast.Attr{Classes: []string{[]string{"note", "warning", "theorem"}[r.IntN(3)]}}, Title: []ast.Inline{txt(title)}, Blocks: randomBlocks(r, depth+1)})
		case 8:
			cell := func() ast.Cell { return ast.Cell{Blocks: []ast.Block{&ast.Plain{Inlines: randomInlines(r, 1)}}} }
			t := &ast.Table{Cols: []ast.ColSpec{{Align: ast.AlignRight}, {}}, Head: []ast.Row{{Cells: []ast.Cell{cell(), cell()}}}, Body: []ast.Row{{Cells: []ast.Cell{cell(), cell()}}}}
			out = append(out, t)
		case 9:
			out = append(out, &ast.DefinitionList{Items: []ast.DefinitionItem{{Term: randomInlines(r, 1), Definitions: [][]ast.Block{randomBlocks(r, depth+1)}}}})
		case 10:
			out = append(out, &ast.MathBlock{TeX: "a\n- b\n= c", Label: "eq:x"}, &ast.HorizontalRule{}, &ast.PageBreak{})
		case 11:
			out = append(out, &ast.LineBlock{Lines: [][]ast.Inline{randomInlines(r, 1), randomInlines(r, 1)}})
		case 12:
			out = append(out, &ast.Figure{Attr: ast.Attr{ID: "fig:r"}, Image: &ast.Image{Src: "https://example.org/f.png", Alt: randomText(r), Width: "80%"}, Caption: randomInlines(r, 1)})
		case 13:
			out = append(out, &ast.CodeBlock{Lang: "python", Attr: ast.Attr{ID: "lst:r", Classes: []string{"numberLines"}}, Caption: randomInlines(r, 1), Text: "print(1)"})
		case 14:
			cell := func() ast.Cell { return ast.Cell{Blocks: []ast.Block{&ast.Plain{Inlines: randomInlines(r, 1)}}} }
			out = append(out, &ast.Table{Attr: ast.Attr{ID: "tbl:r"}, Caption: randomInlines(r, 1), Cols: []ast.ColSpec{{Align: ast.AlignCenter}, {Align: ast.AlignLeft}},
				Head: []ast.Row{{Cells: []ast.Cell{cell(), cell()}}}, Body: []ast.Row{{Cells: []ast.Cell{cell(), cell()}}, {Cells: []ast.Cell{cell(), cell()}}}})
		}
	}
	return out
}

// TestRandomDocumentsRoundTrip writes random block and inline structures
// and checks that they read back unchanged.
func TestRandomDocumentsRoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	n := 1500
	if testing.Short() {
		n = 200
	}
	fails := 0
	for i := 0; i < n && fails < 5; i++ {
		blocks := randomBlocks(r, 0)
		if _, ok := roundTrip(t, blocks); !ok {
			fails++
		}
	}
}
