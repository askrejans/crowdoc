package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// The v1 command line must keep working: flags before or after the
// positional paths, and the options other tools already pass.
func TestParseArgsV1Compatibility(t *testing.T) {
	c, info, err := parseArgs([]string{"--style", "report", "--toc", "--no-title-page", "--font-size", "12", "--template", "t.typ", "in.md", "out.pdf", "--title", "T", "--author", "A; B", "--language", "lv"})
	if err != nil || info != "" {
		t.Fatal(err, info)
	}
	if c.input != "in.md" || c.output != "out.pdf" || c.style != "report" || c.template != "t.typ" {
		t.Fatalf("%+v", c)
	}
	if c.meta["toc"] != true || c.meta["title-page"] != false || c.meta["font-size"] != "12" || c.meta["title"] != "T" || c.meta["lang"] != "lv" {
		t.Fatalf("meta %+v", c.meta)
	}
	if a, ok := c.meta["author"].([]any); !ok || len(a) != 2 {
		t.Fatalf("authors %+v", c.meta["author"])
	}
	c, _, _ = parseArgs([]string{"-b", "docs/", "out/", "-s", "minimal"})
	if !c.batch || c.batchDir != "docs/" || c.batchOut != "out/" || c.style != "minimal" {
		t.Fatalf("batch %+v", c)
	}
	if _, _, err := parseArgs([]string{"--font-size", "40", "x.md"}); err == nil {
		t.Fatal("invalid font size accepted")
	}
	if _, _, err := parseArgs([]string{"--bogus"}); err == nil {
		t.Fatal("unknown flag accepted")
	}
	c, _, _ = parseArgs([]string{"--meta", "keywords=[a, b]", "--colors", "oxford", "--color", "accent=#123456", "--meta=paper=letter", "x.md"})
	if c.meta["color-scheme"] != "oxford" || c.meta["paper"] != "letter" {
		t.Fatalf("meta %+v", c.meta)
	}
	if kw, ok := c.meta["keywords"].([]any); !ok || len(kw) != 2 {
		t.Fatalf("keywords %+v", c.meta["keywords"])
	}
}

func TestListings(t *testing.T) {
	for _, flag := range []string{"--list-styles", "--list-colors", "--list-fonts", "--list-formats", "--list-citation-styles", "--list-languages", "--version", "--help"} {
		var out, errb bytes.Buffer
		if err := run(context.Background(), []string{flag}, &out, &errb); err != nil {
			t.Fatalf("%s: %v", flag, err)
		}
		if strings.TrimSpace(out.String()) == "" {
			t.Fatalf("%s printed nothing", flag)
		}
	}
}

func TestBatchOutputsNeverCollide(t *testing.T) {
	got := batchOutputs("in", "in", []string{"in/a.md", "in/r.docx", "in/r.odt", "in/s.pdf", "in/sub/t.txt"})
	want := map[string]string{
		"in/a.md":      "in/a.pdf",
		"in/r.docx":    "in/r-docx.pdf",
		"in/r.odt":     "in/r-odt.pdf",
		"in/s.pdf":     "in/s-pdf.pdf",
		"in/sub/t.txt": "in/sub/t.pdf",
	}
	for in, w := range want {
		if got[in] != w {
			t.Errorf("%s → %s, want %s", in, got[in], w)
		}
	}
}
