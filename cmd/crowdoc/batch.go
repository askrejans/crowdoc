package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
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
	count := map[string]int{}
	inputs := map[string]bool{}
	for _, f := range files {
		count[plain(f)]++
		abs, _ := filepath.Abs(f)
		inputs[abs] = true
	}
	outs := make(map[string]string, len(files))
	for _, f := range files {
		out := plain(f)
		if abs, _ := filepath.Abs(out); count[out] > 1 || inputs[abs] {
			rel, _ := filepath.Rel(batchDir, f)
			ext := filepath.Ext(rel)
			out = filepath.Join(outDir, strings.TrimSuffix(rel, ext)+"-"+strings.ToLower(strings.TrimPrefix(ext, "."))+".pdf")
		}
		outs[f] = out
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
			return nil
		}
		if d.IsDir() {
			if abs, _ := filepath.Abs(path); abs == absOut || (strings.HasPrefix(d.Name(), ".") && path != c.batchDir) {
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
	for _, f := range files {
		wg.Add(1)
		sem <- struct{}{}
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
	fmt.Fprintf(stdout, "Done: %d/%d PDFs in %s (%s)\n", len(files)-failed, len(files), outDir, time.Since(start).Round(time.Millisecond))
	if failed > 0 {
		return fmt.Errorf("%d of %d conversions failed", failed, len(files))
	}
	return nil
}

// runWatch regenerates the PDF whenever the input (or a bibliography file)
// changes. The compilation directory is reused, so rebuilds are fast.
func runWatch(ctx context.Context, c *cli, opts crowdoc.Options, stdout, stderr io.Writer) error {
	if opts.WorkDir == "" {
		dir, err := os.MkdirTemp("", "crowdoc-watch-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		opts.WorkDir = dir
	}
	watched := append([]string{c.input}, c.bibs...)
	stamp := func() string {
		var sb strings.Builder
		for _, p := range watched {
			if fi, err := os.Stat(p); err == nil {
				fmt.Fprintf(&sb, "%s|%d|%d;", p, fi.ModTime().UnixNano(), fi.Size())
			}
		}
		return sb.String()
	}
	fmt.Fprintf(stdout, "Watching %s (Ctrl+C to stop)\n", c.input)
	last := ""
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s := stamp(); s != last {
			last = s
			if err := convertOne(ctx, c, c.input, c.output, opts, stdout, stderr); err != nil {
				fmt.Fprintln(stderr, "crowdoc:", err)
			} else if !c.quiet {
				fmt.Fprintf(stdout, "  updated at %s\n", time.Now().Format("15:04:05"))
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
