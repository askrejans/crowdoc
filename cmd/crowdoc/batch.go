package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/askrejans/crowdoc/v2"
)

// batchOutputs maps each input to its PDF under outDir, mirroring the tree.
// Inputs that would share an output (report.docx and report.odt) or write
// over an input (report.pdf) keep their format in the name: report-odt.pdf.
func batchOutputs(batchDir, outDir string, files []string) map[string]string {
	plain := func(f string) string {
		rel, _ := filepath.Rel(batchDir, f)
		return filepath.Join(outDir, strings.TrimSuffix(rel, filepath.Ext(rel))+".pdf")
	}
	key := func(p string) string {
		abs, _ := filepath.Abs(p)
		// Avoid collisions on the default Windows and macOS filesystems.
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			abs = strings.ToLower(abs)
		}
		return abs
	}
	count := map[string]int{}
	used := map[string]bool{}
	for _, f := range files {
		count[key(plain(f))]++
		used[key(f)] = true
	}
	// Allocate in a stable order, independent of directory traversal order.
	ordered := append([]string(nil), files...)
	sort.Strings(ordered)
	outs := make(map[string]string, len(files))
	for _, f := range ordered {
		out := plain(f)
		plainKey := key(out)
		if count[plainKey] > 1 || used[plainKey] {
			rel, _ := filepath.Rel(batchDir, f)
			ext := filepath.Ext(rel)
			out = filepath.Join(outDir, strings.TrimSuffix(rel, ext)+"-"+strings.ToLower(strings.TrimPrefix(ext, "."))+".pdf")
		}
		base := strings.TrimSuffix(out, ".pdf")
		for n := 2; used[key(out)] || (key(out) != plainKey && count[key(out)] > 0); n++ {
			out = fmt.Sprintf("%s-%d.pdf", base, n)
		}
		outs[f] = out
		used[key(out)] = true
	}
	return outs
}

// runBatch converts every supported file under c.batchDir in parallel,
// mirroring the directory layout under the output directory.
func runBatch(ctx context.Context, c *cli, opts crowdoc.Options, stdout, stderr io.Writer) error {
	outDir := c.batchOut
	if outDir == "" {
		outDir = filepath.Join(c.batchDir, "pdf")
	}
	absOut, _ := filepath.Abs(outDir)
	var files []string
	err := filepath.WalkDir(c.batchDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if abs, _ := filepath.Abs(path); (abs == absOut && path != c.batchDir) || (strings.HasPrefix(d.Name(), ".") && path != c.batchDir) {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~$") || name == "readme.md" || name == "claude.md" || name == "license.md" {
			return nil
		}
		if crowdoc.SupportedExtension(filepath.Ext(name)) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Fprintln(stdout, "No supported documents found in", c.batchDir)
		return nil
	}
	sort.Strings(files)
	jobs := c.jobs
	if jobs <= 0 {
		jobs = min(runtime.NumCPU(), 4)
	}
	jobs = min(jobs, len(files))
	if opts.Engine == nil {
		// Share one engine so the font list is computed once.
		path, err := crowdoc.FindTypst()
		if err != nil {
			return fmt.Errorf("%w\nRun `crowdoc engine install` first", err)
		}
		opts.Engine = crowdoc.NewTypstEngine(path, crowdoc.FontDirs(c.fontDirs), c.deterministic)
	}
	opts.WorkDir = "" // per-file temporary directories

	outputs := batchOutputs(c.batchDir, outDir, files)

	fmt.Fprintf(stdout, "Converting %d files with %d workers…\n", len(files), jobs)
	start := time.Now()
	var mu sync.Mutex
	failed := 0
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
schedule:
	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break schedule
		}
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			defer func() { <-sem }()
			rel, _ := filepath.Rel(c.batchDir, f)
			out := outputs[f]
			res, err := crowdoc.ConvertFile(ctx, f, out, opts)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				fmt.Fprintf(stderr, "  ✗ %s: %v\n", rel, err)
				return
			}
			fmt.Fprintf(stdout, "  ✓ %s → %s (%s)\n", rel, out, res.Style)
			if !c.quiet {
				for _, w := range res.Warnings {
					fmt.Fprintf(stderr, "      warning: %s\n", w)
				}
			}
		}(f)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Done: %d/%d PDFs in %s (%s)\n", len(files)-failed, len(files), outDir, time.Since(start).Round(time.Millisecond))
	if failed > 0 {
		return fmt.Errorf("%d of %d conversions failed", failed, len(files))
	}
	return nil
}
