package locale

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// typstString quotes s as a Typst string literal.
func typstString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// TestTypstAcceptsLanguages compiles a small document per language with the
// Typst binary named by CROWDOC_TYPST, checking that every TypstLang and
// TypstRegion value is accepted without errors or warnings and that the
// localised strings typeset.
func TestTypstAcceptsLanguages(t *testing.T) {
	bin := os.Getenv("CROWDOC_TYPST")
	if bin == "" {
		t.Skip("set CROWDOC_TYPST to a Typst binary to run")
	}
	dir := t.TempDir()
	for _, terms := range everyTerms() {
		t.Run(terms.Tag, func(t *testing.T) {
			t.Parallel()
			args := "lang: " + typstString(terms.TypstLang)
			if terms.TypstRegion != "" {
				args += ", region: " + typstString(terms.TypstRegion)
			}
			if terms.RTL {
				args += ", dir: rtl"
			}
			var src strings.Builder
			src.WriteString("#set text(" + args + ")\n")
			src.WriteString("#set page(width: 12cm, height: auto)\n")
			src.WriteString("#set par(justify: true)\n")
			for _, s := range []string{
				terms.Contents, terms.CaptionPrefix(terms.Figure, "1") + terms.Abstract,
				FormatDate(time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), terms.Tag, true),
				FormatPageOf(3, 12, terms.Tag),
				terms.QuoteOpen + terms.Months[0] + terms.QuoteClose,
			} {
				src.WriteString("#text(" + typstString(s) + ")\n\n")
			}
			in := filepath.Join(dir, terms.Tag+".typ")
			if err := os.WriteFile(in, []byte(src.String()), 0o644); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, "compile", in, filepath.Join(dir, terms.Tag+".pdf"))
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("typst failed: %v\n%s", err, stderr.String())
			}
			if out := stderr.String(); strings.Contains(out, "warning") {
				t.Errorf("typst warned:\n%s", out)
			}
		})
	}
}
