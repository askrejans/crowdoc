package typst

import (
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
)

func TestEscapeMarkup(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a*b_c", `a\*b\_c`},
		{"#x $y$ @z <l>", `\#x \$y\$ \@z \<l\>`},
		{"- item", `\- item`},
		{"a - b", `a - b`},
		{"a--b", `a\-\-b`},
		{"http://x", `http:\/\/x`},
		{"1. not a list", `1\. not a list`},
		{"a [b] ~c", `a \[b\] \~c`},
		{"x\\y", `x\\y`},
	}
	for _, c := range cases {
		if got := escapeMarkup(c.in, true); got != c.want {
			t.Errorf("escapeMarkup(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTypstLength(t *testing.T) {
	for in, want := range map[string]string{"50%": "50%", "8cm": "8cm", "240px": "180pt", "300": "225pt", "120%": "100%", "bogus": "", "": ""} {
		if got := typstLength(in); got != want {
			t.Errorf("typstLength(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGridLayoutSpans(t *testing.T) {
	cell := func(cs, rs int) ast.Cell { return ast.Cell{ColSpan: cs, RowSpan: rs} }
	rows := []ast.Row{
		{Cells: []ast.Cell{cell(1, 2), cell(2, 1)}},
		{Cells: []ast.Cell{cell(1, 1), cell(1, 1)}},
		{Cells: []ast.Cell{cell(1, 1)}},
	}
	placed, cols := gridLayout(rows)
	if cols != 3 {
		t.Fatalf("cols = %d", cols)
	}
	if placed[1][0].col != 1 || placed[1][1].col != 2 {
		t.Fatalf("row 1 placed at %d,%d; want 1,2", placed[1][0].col, placed[1][1].col)
	}
	// A row span reaching past the table end is clamped.
	placed, _ = gridLayout([]ast.Row{{Cells: []ast.Cell{cell(1, 5)}}})
	if placed[0][0].cell.RowSpan != 1 {
		t.Fatalf("row span not clamped: %d", placed[0][0].cell.RowSpan)
	}
}

// TestEscapingRoundTrip writes random text full of markup characters,
// interleaved with formatting calls, compiles it with Typst's HTML export
// and checks the text survives unchanged. Set CROWDOC_TYPST to run it.
func TestEscapingRoundTrip(t *testing.T) {
	bin := os.Getenv("CROWDOC_TYPST")
	if bin == "" {
		t.Skip("CROWDOC_TYPST not set")
	}
	alphabet := []rune("ab z09.,;:!?-–—/\\*_`$#@<>[]{}()~=+'\"%&|^ āčēģķļņšūž")
	rng := rand.New(rand.NewSource(7))
	var paras []string
	var wants []string
	w := &writer{labels: map[string]bool{}}
	for i := 0; i < 300; i++ {
		var ins []ast.Inline
		var plain strings.Builder
		for j := 0; j < 1+rng.Intn(4); j++ {
			n := 1 + rng.Intn(12)
			var sb strings.Builder
			for k := 0; k < n; k++ {
				sb.WriteRune(alphabet[rng.Intn(len(alphabet))])
			}
			s := sb.String()
			if rng.Intn(3) == 0 {
				ins = append(ins, &ast.Strong{Inlines: []ast.Inline{&ast.Text{Value: s}}})
			} else {
				ins = append(ins, &ast.Text{Value: s})
			}
			plain.WriteString(s)
		}
		text := strings.Join(strings.Fields(plain.String()), " ")
		if text == "" {
			continue
		}
		paras = append(paras, "#block["+w.inlinesAtStart(ins)+"]")
		wants = append(wants, text)
	}
	dir := t.TempDir()
	if d := os.Getenv("CROWDOC_DEBUG_DIR"); d != "" {
		dir = d
	}
	src := "#set text(lang: \"en\")\n#set smartquote(enabled: false)\n" + strings.Join(paras, "\n\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "t.typ"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "compile", "--features", "html", "--format", "html", filepath.Join(dir, "t.typ"), filepath.Join(dir, "t.html")).CombinedOutput()
	if err != nil {
		t.Fatalf("typst: %v\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(dir, "t.html"))
	if err != nil {
		t.Fatal(err)
	}
	got := blockTexts(t, string(data))
	if len(got) != len(wants) {
		for i := range wants {
			if i >= len(got) || strings.Join(strings.Fields(got[i]), " ") != wants[i] {
				t.Fatalf("got %d blocks, want %d; first mismatch at %d:\nsrc  %s\nwant %q\ngot  %q", len(got), len(wants), i, paras[i], wants[i], got[min(i, len(got)-1)])
			}
		}
	}
	for i := range wants {
		// Typst turns "--", "---" and "..." into dashes and ellipses only
		// when unescaped; everything must come back verbatim.
		if g := strings.Join(strings.Fields(got[i]), " "); g != wants[i] {
			t.Errorf("block %d:\n got %q\nwant %q\nsrc  %s", i, g, wants[i], paras[i])
		}
	}
}

func blockTexts(t *testing.T, doc string) []string {
	t.Helper()
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	var text func(*html.Node) string
	text = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		var sb strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			sb.WriteString(text(c))
		}
		return sb.String()
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "div" || isDisplayBlock(n)) {
			out = append(out, text(n))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

// isDisplayBlock reports elements Typst's HTML export renders as blocks
// (a block holding a single strong becomes <strong style="display: block">).
func isDisplayBlock(n *html.Node) bool {
	for _, a := range n.Attr {
		if a.Key == "style" && strings.Contains(a.Val, "display: block") {
			return true
		}
	}
	return false
}
