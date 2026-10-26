package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/askrejans/crowdoc/v2"
)

func TestBatchOutputsResolveSecondaryCollisions(t *testing.T) {
	files := []string{"in/r.md", "in/r.docx", "in/r-md.md", "in/r-md-2.pdf", "in/r-md-2-pdf.md", "in/r-docx.pdf"}
	got := batchOutputs("in", "in", files)
	used := map[string]bool{}
	for _, f := range files {
		used[f] = true
	}
	for _, f := range files {
		out := got[f]
		if used[out] {
			t.Fatalf("%s would overwrite another output or input: %s", f, out)
		}
		used[out] = true
	}
	if got["in/r-md.md"] != "in/r-md.pdf" {
		t.Fatalf("unambiguous name was taken: %v", got)
	}
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}
	if other := batchOutputs("in", "in", files); !reflect.DeepEqual(got, other) {
		t.Fatalf("output names depend on input order: %v vs %v", got, other)
	}
}

func TestBatchMissingInputDirectory(t *testing.T) {
	var out bytes.Buffer
	err := runBatch(context.Background(), &cli{batchDir: filepath.Join(t.TempDir(), "missing")}, crowdoc.Options{}, &out, &out)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want missing input error, got %v", err)
	}
}

type batchEngine struct {
	cancel context.CancelFunc
	calls  int
}

func (*batchEngine) Name() string                                   { return "test" }
func (*batchEngine) Fonts(context.Context) (map[string]bool, error) { return nil, nil }
func (e *batchEngine) Compile(ctx context.Context, _ *crowdoc.EngineJob) (*crowdoc.EngineResult, error) {
	e.calls++
	if e.cancel != nil {
		e.cancel()
		return nil, ctx.Err()
	}
	return &crowdoc.EngineResult{PDF: []byte("%PDF-test")}, nil
}

func TestBatchCancellationStopsScheduling(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# Test\n\nBody."), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng := &batchEngine{cancel: cancel}
	var out bytes.Buffer
	err := runBatch(ctx, &cli{batchDir: dir, jobs: 1}, crowdoc.Options{Engine: eng}, &out, &out)
	if !errors.Is(err, context.Canceled) || eng.calls != 1 {
		t.Fatalf("want cancellation after first compile, got %v (%d compiles)", err, eng.calls)
	}
}

func TestBatchCanWriteAlongsideInputs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("# Test\n\nBody."), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := runBatch(context.Background(), &cli{batchDir: dir, batchOut: dir, jobs: 1}, crowdoc.Options{Engine: &batchEngine{}}, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "note.pdf")); err != nil {
		t.Fatalf("input directory was skipped: %v", err)
	}
}
