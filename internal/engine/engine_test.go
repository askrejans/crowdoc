package engine

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestWriteFilesConfinesSymlinks(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	secret := filepath.Join(outside, "secret.typ")
	if err := os.WriteFile(secret, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, link := range []struct{ target, name string }{
		{secret, "main.typ"},
		{outside, "assets"},
	} {
		if err := os.Symlink(link.target, filepath.Join(dir, link.name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	for _, name := range []string{"main.typ", "assets/secret.typ"} {
		if err := writeFiles(dir, map[string][]byte{name: []byte("overwrite")}); err == nil {
			t.Fatalf("bundle write escaped through %s", name)
		}
	}
	data, err := os.ReadFile(secret)
	if err != nil || string(data) != "original" {
		t.Fatalf("outside file was changed: %q, %v", data, err)
	}
}

func TestTypstRejectsEntryEscapeBeforeWriting(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")
	_, err := (&Typst{}).Compile(context.Background(), &Job{WorkDir: dir, Main: "../outside.typ"})
	if err == nil || !strings.Contains(err.Error(), "invalid entry path") {
		t.Fatalf("entry escape accepted: %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid job created work directory: %v", err)
	}
}

func TestTypstRelativeWorkDir(t *testing.T) {
	bin := os.Getenv("CROWDOC_TYPST")
	if bin == "" {
		t.Skip("CROWDOC_TYPST not set")
	}
	t.Chdir(t.TempDir())
	res, err := (&Typst{Path: bin}).Compile(context.Background(), &Job{
		WorkDir: "work", Files: map[string][]byte{"main.typ": []byte("Relative work directory")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(res.PDF, []byte("%PDF-")) {
		t.Fatal("not a PDF")
	}
}

func TestTypstFontPathsBecomeAbsolute(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir("fonts", 0o700); err != nil {
		t.Fatal(err)
	}
	args := (&Typst{FontPaths: []string{"fonts"}}).fontArgs()
	if len(args) != 2 || args[0] != "--font-path" || args[1] != filepath.Join(dir, "fonts") {
		t.Fatalf("font path would be resolved relative to the compile workdir: %v", args)
	}
}
