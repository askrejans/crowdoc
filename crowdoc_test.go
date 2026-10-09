package crowdoc

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/askrejans/crowdoc/v2/ast"
)

func typstEngine(t *testing.T) Engine {
	t.Helper()
	p := os.Getenv("CROWDOC_TYPST")
	if p == "" {
		t.Skip("CROWDOC_TYPST not set")
	}
	return NewTypstEngine(p, nil, false)
}

func TestDetectFormat(t *testing.T) {
	zipWith := func(names ...string) []byte {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for _, n := range names {
			w, _ := zw.Create(n)
			if n == "mimetype" {
				w.Write([]byte("application/epub+zip"))
			}
		}
		zw.Close()
		return buf.Bytes()
	}
	cases := []struct {
		name string
		data []byte
		want Format
	}{
		{"x.md", []byte("# Hi"), FormatMarkdown},
		{"report.docx", zipWith("word/document.xml"), FormatDOCX},
		{"misnamed.bin", zipWith("word/document.xml"), FormatDOCX},
		{"book.zip", zipWith("mimetype", "META-INF/container.xml"), FormatEPUB},
		{"data.xlsx", zipWith("xl/workbook.xml"), FormatXLSX},
		{"x.txt", []byte(`{\rtf1\ansi hi}`), FormatRTF},
		{"page", []byte("<!DOCTYPE html><html>"), FormatHTML},
		{"n.json", []byte(`{"cells": [], "nbformat": 4}`), FormatNotebook},
		{"t.csv", []byte("a,b\n1,2"), FormatCSV},
		{"notes", []byte("plain words"), FormatMarkdown},
	}
	for _, c := range cases {
		got, err := DetectFormat(c.name, c.data)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}
	for _, legacy := range []string{"old.doc", "old.xls"} {
		if _, err := DetectFormat(legacy, []byte{0xD0, 0xCF, 0x11, 0xE0}); err == nil {
			t.Errorf("%s: legacy binary accepted", legacy)
		}
	}
}

func TestRenderWithoutEngine(t *testing.T) {
	res, err := Render(context.Background(), Source{Path: "examples/general/showcase.md"}, Options{Style: "article", Now: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"main.typ", "style.typ", "crowdoc.typ"} {
		if len(res.Bundle.Files[f]) == 0 {
			t.Errorf("bundle lacks %s", f)
		}
	}
	assets := 0
	for name := range res.Bundle.Files {
		if strings.HasPrefix(name, "assets/") {
			assets++
		}
	}
	if assets != 3 {
		t.Errorf("assets = %d, want 3 images", assets)
	}
	main := string(res.Bundle.Files["main.typ"])
	for _, want := range []string{"#cd-bibliography(", "#figure(", "#callout(kind: \"warning\"", "#task-list(", "#cd-ref(<fig:revenue>)"} {
		if !strings.Contains(main, want) {
			t.Errorf("main.typ lacks %s", want)
		}
	}
}

func TestUntrustedSourceIsConfined(t *testing.T) {
	dir := t.TempDir()
	src := "---\nbibliography: /etc/passwd\nlogo: /etc/hosts\n---\n# T\n\n![x](/etc/hosts) ![y](../../../etc/hosts) [@a]\n\n```{=typst}\n#read(\"/etc/hosts\")\n```\n"
	res, err := Render(context.Background(), Source{Name: "u.md", Data: []byte(src), BaseDir: dir, Untrusted: true}, Options{UnsafeRaw: true})
	if err != nil {
		t.Fatal(err)
	}
	for name := range res.Bundle.Files {
		if strings.HasPrefix(name, "assets/") {
			t.Fatalf("untrusted document pulled in %s", name)
		}
	}
	if strings.Contains(string(res.Bundle.Files["main.typ"]), "#read(") {
		t.Fatal("raw Typst passed through for an untrusted source")
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "not accessible") {
		t.Fatalf("no warning about the bibliography path: %v", res.Warnings)
	}
}

func TestInspect(t *testing.T) {
	o, err := Inspect(context.Background(), Source{Path: "examples/academic/academic-paper.md"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if o.Title == "" || o.Language != "en" || o.Counts.Figures != 1 || o.Counts.Tables != 1 || len(o.Citations) < 5 || o.Style != "article" {
		t.Fatalf("%+v", o)
	}
}

func TestConvertFileRefusesOverwritingInput(t *testing.T) {
	p := filepath.Join(t.TempDir(), "same.md")
	os.WriteFile(p, []byte("# x"), 0o600)
	if _, err := ConvertFile(context.Background(), p, p, Options{}); err == nil {
		t.Fatal("input overwritten")
	}
}

func TestDefaultOutputPath(t *testing.T) {
	for in, want := range map[string]string{"a/notes.md": "a/notes.pdf", "paper.PDF": "paper.typeset.pdf", "README": "README.pdf"} {
		if got := DefaultOutputPath(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}

func TestEveryStyleCompiles(t *testing.T) {
	eng := typstEngine(t)
	for _, s := range Styles() {
		s := s
		t.Run(s.Name, func(t *testing.T) {
			t.Parallel()
			res, err := Convert(context.Background(), Source{Path: "examples/general/showcase.md"}, Options{Style: s.Name, Engine: eng})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(res.PDF, []byte("%PDF-")) {
				t.Fatal("no PDF")
			}
			for _, w := range res.Warnings {
				if strings.HasPrefix(w, "typesetting:") {
					t.Errorf("engine warning: %s", w)
				}
			}
		})
	}
}

func TestEveryColorSchemeCompiles(t *testing.T) {
	eng := typstEngine(t)
	for _, c := range ColorSchemes() {
		res, err := Convert(context.Background(), Source{Path: "examples/academic/academic-paper.md"}, Options{Style: "report", Engine: eng, Meta: map[string]any{"colors": c.Name}})
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if len(res.PDF) == 0 {
			t.Fatalf("%s: empty PDF", c.Name)
		}
	}
}

func TestEveryExampleInputConverts(t *testing.T) {
	eng := typstEngine(t)
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
	for _, in := range inputs {
		res, err := Convert(context.Background(), Source{Path: in}, Options{Engine: eng})
		if err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if len(res.PDF) < 1000 {
			t.Errorf("%s: suspiciously small PDF", in)
		}
	}
}

func TestPDFAAndLanguages(t *testing.T) {
	eng := typstEngine(t)
	for _, lang := range []string{"lv", "de", "ru", "ja", "ar", "el"} {
		res, err := Convert(context.Background(), Source{Name: "x.md", Data: []byte("# Virsraksts\n\n![a](missing.png)\n\nText.")}, Options{Engine: eng, Style: "article", PDFStandards: []string{"a-2b"}, Meta: map[string]any{"lang": lang}})
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if !bytes.Contains(res.PDF, []byte("pdfaid")) {
			t.Fatalf("%s: no PDF/A identification", lang)
		}
	}
}

func TestAccessiblePDFWithImagesAndMath(t *testing.T) {
	eng := typstEngine(t)
	for _, in := range []string{"examples/general/showcase.md", "examples/academic/academic-paper.md"} {
		res, err := Convert(context.Background(), Source{Path: in}, Options{Engine: eng, PDFStandards: []string{"ua-1"}})
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if !bytes.Contains(res.PDF, []byte("pdfuaid")) {
			t.Fatalf("%s: no PDF/UA identification", in)
		}
	}
}

func TestRenderDocumentTree(t *testing.T) {
	doc := &ast.Document{Blocks: []ast.Block{
		&ast.Heading{Level: 1, Inlines: ast.Str("Built in code")},
		&ast.Para{Inlines: []ast.Inline{&ast.Text{Value: "Hello "}, &ast.Strong{Inlines: ast.Str("tree")}, &ast.Math{TeX: `\frac{a}{b}`}}},
	}}
	res, err := Render(context.Background(), Source{Name: "notes", Document: doc}, Options{Style: "minimal"})
	if err != nil {
		t.Fatal(err)
	}
	main := string(res.Bundle.Files["main.typ"])
	if !strings.Contains(main, "#strong[tree]") || !strings.Contains(main, "frac(a, b)") || res.Document.Meta.Title != "Built in code" {
		t.Fatalf("unexpected render:\n%s", main)
	}
}

// Article and leaflet styles must work for untitled text (a scan, a pasted
// note) as well as for a full article, and stay accessible.
func TestArticleAndLeafletStyles(t *testing.T) {
	eng := typstEngine(t)
	sources := []Source{
		{Name: "Scan 9.10.26.md", Data: []byte("„Ābols“ ir 12 vārdu garš teikums, kas turpinās pietiekami ilgi, lai aptītu iniciāli vairākās rindās un vēl dažās rindās pēc tam.\n\nOtrā rindkopa.")},
		{Name: "note.md", Data: []byte("**Quick** reminder: the kitchen is closed on Thursday while the new dishwasher is installed, so please use the other one.\n\n- one\n- two")},
		{Name: "event.md", Data: []byte("# Open day\n\n*Workshops and talks*\n\nJoin us.[^1]\n\n## Highlights\n\n- Printing\n- Talks\n\n> Come and see.\n\n::: tip\nBook a seat.\n:::\n\n[^1]: Free entry.")},
	}
	for _, style := range []string{"magazine", "editorial", "newspaper", "blog", "leaflet", "flyer", "booklet"} {
		for _, src := range sources {
			res, err := Convert(context.Background(), src, Options{Style: style, Engine: eng, PDFStandards: []string{"ua-1"}})
			if err != nil {
				t.Fatalf("%s %s: %v", style, src.Name, err)
			}
			for _, w := range res.Warnings {
				if strings.HasPrefix(w, "typesetting:") {
					t.Errorf("%s %s: %s", style, src.Name, w)
				}
			}
		}
	}
}

// A booklet prints a cover only for a real title, not for a file name.
func TestBookletCoverNeedsRealTitle(t *testing.T) {
	for name, want := range map[string]string{"Scan 1.md": "title-from-name: true", "story.md": "title-from-name: false"} {
		data := "Plain text."
		if name == "story.md" {
			data = "# A story\n\nPlain text."
		}
		res, err := Render(context.Background(), Source{Name: name, Data: []byte(data)}, Options{Style: "booklet"})
		if err != nil {
			t.Fatal(err)
		}
		if main := string(res.Bundle.Files["main.typ"]); !strings.Contains(main, want) {
			t.Errorf("%s: missing %q", name, want)
		}
	}
}
