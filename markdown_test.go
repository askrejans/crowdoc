package crowdoc

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/internal/writer/markdown/mdtest"
)

// exampleInputs lists every example document of a supported format.
func exampleInputs(t *testing.T) []string {
	t.Helper()
	var inputs []string
	filepath.WalkDir("examples", func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && (d.Name() == "assets" || d.Name() == "output") {
			return filepath.SkipDir
		}
		if err == nil && !d.IsDir() && SupportedExtension(filepath.Ext(p)) {
			inputs = append(inputs, p)
		}
		return nil
	})
	if len(inputs) < 10 {
		t.Fatalf("only %d example inputs", len(inputs))
	}
	return inputs
}

func TestMarkdownRoundTripExamples(t *testing.T) {
	ctx := context.Background()
	for _, f := range exampleInputs(t) {
		t.Run(filepath.Base(f), func(t *testing.T) {
			res, err := Markdown(ctx, Source{Path: f}, Options{}, MarkdownOptions{})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			out := filepath.Join(dir, "doc.md")
			if err := WriteMarkdown(res, out); err != nil {
				t.Fatal(err)
			}
			back, _, err := Parse(ctx, Source{Path: out}, Options{})
			if err != nil {
				t.Fatal(err)
			}
			mdtest.ClearAutoIDs(res.Document.Blocks, back.Blocks)
			want, got := mdtest.Summary(res.Document), mdtest.Summary(back)
			if want != got {
				t.Errorf("round trip differs\n%s\n--- markdown ---\n%s", diff(want, got), res.Text)
			}
			again, err := Markdown(ctx, Source{Path: out}, Options{}, MarkdownOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if again.Text != res.Text {
				t.Errorf("not idempotent\n%s", diff(res.Text, again.Text))
			}
		})
	}
}

// diff shows the first differing lines of two texts.
func diff(a, b string) string {
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
			return "line " + strconv.Itoa(i+1) + ":\n  want: " + x + "\n  got:  " + y
		}
	}
	return ""
}
