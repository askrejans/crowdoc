package markdown

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/internal/writer/markdown/mdtest"
)

// FuzzRoundTrip reads arbitrary Markdown, writes it and reads the result:
// the writer must not panic and the second reading must equal the first.
//
//	go test -fuzz FuzzRoundTrip ./internal/writer/markdown
func FuzzRoundTrip(f *testing.F) {
	f.Add(sampleMarkdown)
	files, _ := filepath.Glob("../../../examples/*/*.md")
	for _, p := range files {
		if b, err := os.ReadFile(p); err == nil {
			f.Add(string(b))
		}
	}
	for _, s := range []string{
		"*a **b** c*", "a_b_c __d__", "~~x~~ ~y~ ^z^ ==w==", "[a](<b c> \"t\") <http://x.y>", "x\\\ny  \nz",
		"> [!TIP] t\n> b", "::: {.note #n title=\"T\"}\n::: tip\nx\n:::\n:::", "| a | b |\n|:-|-:|\n| `\\|` | $|x|$ |",
		"Term\n: def\n\n  more", "1. a\n\n   b\n2. c", "- [ ] x\n  - [x] y", "$$\na\n$$ {#eq:x}", "@x [p. 3] and [-@y; see @z, ch. 2]",
		"<u>u</u> <span class=\"smallcaps\">s</span> <kbd>k</kbd>", "![c](i.png){#fig:a width=50% fig-alt=\"a\"}", "\\newpage", "| a\n|  b",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// Invalid UTF-8 is decoded as Windows-1252, which the second
		// reading cannot repeat.
		if len(src) > 2000 || !utf8.ValidString(src) {
			return
		}
		doc := read(t, src)
		res, err := Write(context.Background(), doc, Options{})
		if err != nil {
			t.Fatal(err)
		}
		back := read(t, res.Text)
		mdtest.ClearAutoIDs(doc.Blocks, back.Blocks)
		if want, got := mdtest.Summary(doc), mdtest.Summary(back); want != got {
			t.Errorf("round trip differs\n%s\nmarkdown: %q", firstDiff(want, got), res.Text)
		}
	})
}
