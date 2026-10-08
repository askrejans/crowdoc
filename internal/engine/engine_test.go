package engine

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseDiagnostics(t *testing.T) {
	out := "/tmp/x/main.typ:2:7: error: unknown variable: foo\n/tmp/x/style.typ:1:16: warning: unknown font family: nope\nerror: failed to load file\n  hint: something\n"
	d := parseDiagnostics(out, "/tmp/x")
	if len(d) != 3 {
		t.Fatalf("got %d diagnostics: %+v", len(d), d)
	}
	if d[0].File != "main.typ" || d[0].Line != 2 || d[0].Severity != "error" || d[1].Severity != "warning" || d[2].File != "" {
		t.Fatalf("%+v", d)
	}
	err := &CompileError{Engine: "typst", Diagnostics: d}
	if !strings.Contains(err.Error(), "main.typ:2:7: error: unknown variable: foo") || strings.Contains(err.Error(), "font") {
		t.Fatalf("error message: %s", err)
	}
}

func TestWriteFilesRejectsEscapes(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{"../x.typ", "/etc/x"} {
		if err := writeFiles(dir, map[string][]byte{bad: []byte("x")}); err == nil {
			t.Fatalf("%s accepted", bad)
		}
	}
	if err := writeFiles(dir, map[string][]byte{"assets/a.png": []byte("x")}); err != nil {
		t.Fatal(err)
	}
}

func TestTypstCompile(t *testing.T) {
	bin := os.Getenv("CROWDOC_TYPST")
	if bin == "" {
		t.Skip("CROWDOC_TYPST not set")
	}
	e := &Typst{Path: bin}
	res, err := e.Compile(context.Background(), &Job{Files: map[string][]byte{"main.typ": []byte("Hello *world*")}, PDFStandards: []string{"a-2b"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(res.PDF), "%PDF-") {
		t.Fatal("not a PDF")
	}
	_, err = e.Compile(context.Background(), &Job{Files: map[string][]byte{"main.typ": []byte("#undefined-function()")}})
	var ce *CompileError
	if !errors.As(err, &ce) || len(ce.Diagnostics) == 0 {
		t.Fatalf("want CompileError with diagnostics, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Compile(ctx, &Job{Files: map[string][]byte{"main.typ": []byte("x")}}); err == nil {
		t.Fatal("cancelled context accepted")
	}
}
