// Package engine compiles typesetting bundles into PDF.
//
// The default engine runs the Typst compiler as a separate executable.
// Callers embedding crowdoc on platforms without process spawning (mobile,
// WebAssembly) implement Engine themselves and pass it to the converter.
package engine

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Diagnostic is a compiler message.
type Diagnostic struct {
	Severity string // "error" or "warning"
	File     string
	Line     int
	Column   int
	Message  string
}

func (d Diagnostic) String() string {
	if d.File != "" {
		return fmt.Sprintf("%s:%d:%d: %s: %s", d.File, d.Line, d.Column, d.Severity, d.Message)
	}
	return d.Severity + ": " + d.Message
}

// Job is one compilation.
type Job struct {
	// Files maps bundle-relative paths to contents.
	Files map[string][]byte
	// Main is the entry file (default "main.typ").
	Main string
	// PDFStandards are conformance targets such as "a-2b" or "ua-1".
	PDFStandards []string
	// WorkDir, when set, is used (and kept) instead of a temporary
	// directory. Watch mode reuses it between runs.
	WorkDir string
	// FontPaths are additional font directories for this job.
	FontPaths []string
}

// Result is a compiled PDF.
type Result struct {
	PDF         []byte
	Diagnostics []Diagnostic
	Duration    time.Duration
}

// Engine compiles bundles.
type Engine interface {
	Name() string
	Compile(ctx context.Context, job *Job) (*Result, error)
	// Fonts lists the font family names the engine can use (lower-case).
	Fonts(ctx context.Context) (map[string]bool, error)
}

// CompileError reports a failed compilation with its diagnostics.
type CompileError struct {
	Engine      string
	Diagnostics []Diagnostic
	Output      string
}

func (e *CompileError) Error() string {
	var msgs []string
	for _, d := range e.Diagnostics {
		if d.Severity == "error" {
			msgs = append(msgs, d.String())
		}
	}
	if len(msgs) == 0 {
		out := strings.TrimSpace(e.Output)
		if len(out) > 2000 {
			out = out[len(out)-2000:]
		}
		return e.Engine + " failed: " + out
	}
	if len(msgs) > 8 {
		msgs = append(msgs[:8], fmt.Sprintf("… and %d more", len(msgs)-8))
	}
	return e.Engine + " failed:\n" + strings.Join(msgs, "\n")
}

// ErrNotFound means the engine executable is not installed.
var ErrNotFound = errors.New("typesetting engine not found")

// ---------------------------------------------------------------------------
// Typst
// ---------------------------------------------------------------------------

// Typst runs the Typst compiler.
type Typst struct {
	// Path is the typst executable.
	Path string
	// FontPaths are extra font directories.
	FontPaths []string
	// IgnoreSystemFonts restricts fonts to FontPaths and Typst's embedded
	// fonts, for byte-identical output across machines.
	IgnoreSystemFonts bool

	fontsOnce sync.Once
	fonts     map[string]bool
	fontsErr  error
}

// Name implements Engine.
func (t *Typst) Name() string { return "typst" }

// Version returns the compiler version string.
func (t *Typst) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, t.Path, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *Typst) fontArgs(extra ...string) []string {
	var args []string
	seen := map[string]bool{}
	for _, p := range append(append([]string(nil), t.FontPaths...), extra...) {
		if seen[p] {
			continue
		}
		seen[p] = true
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			args = append(args, "--font-path", p)
		}
	}
	if t.IgnoreSystemFonts {
		args = append(args, "--ignore-system-fonts")
	}
	return args
}

// Fonts implements Engine. The list is computed once per Typst value.
func (t *Typst) Fonts(ctx context.Context) (map[string]bool, error) {
	t.fontsOnce.Do(func() {
		args := append([]string{"fonts"}, t.fontArgs()...)
		cmd := exec.CommandContext(ctx, t.Path, args...)
		cmd.Env = typstEnv()
		out, err := cmd.Output()
		if err != nil {
			t.fontsErr = fmt.Errorf("listing fonts: %w", err)
			return
		}
		t.fonts = map[string]bool{}
		sc := bufio.NewScanner(bytes.NewReader(out))
		for sc.Scan() {
			if name := strings.TrimSpace(sc.Text()); name != "" && !strings.HasPrefix(name, "├") && !strings.HasPrefix(name, "└") && !strings.HasPrefix(name, "│") {
				t.fonts[strings.ToLower(name)] = true
			}
		}
	})
	return t.fonts, t.fontsErr
}

// Compile implements Engine.
func (t *Typst) Compile(ctx context.Context, job *Job) (*Result, error) {
	start := time.Now()
	dir := job.WorkDir
	if dir == "" {
		tmp, err := os.MkdirTemp("", "crowdoc-*")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmp)
		dir = tmp
	} else if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := writeFiles(dir, job.Files); err != nil {
		return nil, err
	}
	main := job.Main
	if main == "" {
		main = "main.typ"
	}
	out := filepath.Join(dir, "output.pdf")
	_ = os.Remove(out)
	args := []string{"compile", "--root", dir, "--diagnostic-format", "short"}
	args = append(args, t.fontArgs(job.FontPaths...)...)
	if len(job.PDFStandards) > 0 {
		args = append(args, "--pdf-standard", strings.Join(job.PDFStandards, ","))
	}
	args = append(args, filepath.Join(dir, main), out)
	cmd := exec.CommandContext(ctx, t.Path, args...)
	cmd.Dir = dir
	cmd.Env = typstEnv()
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	runErr := cmd.Run()
	diags := parseDiagnostics(combined.String(), dir)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if runErr != nil {
		return nil, &CompileError{Engine: "typst", Diagnostics: diags, Output: combined.String()}
	}
	pdf, err := os.ReadFile(out)
	if err != nil {
		return nil, &CompileError{Engine: "typst", Diagnostics: diags, Output: "no PDF was produced"}
	}
	return &Result{PDF: pdf, Diagnostics: diags, Duration: time.Since(start)}, nil
}

// typstEnv keeps the compiler off the network and away from user package
// caches: documents never import packages.
func typstEnv() []string {
	env := os.Environ()
	out := env[:0:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, "TYPST_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func writeFiles(dir string, files map[string][]byte) error {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		clean := filepath.Clean(filepath.FromSlash(name))
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("invalid bundle path %q", name)
		}
		p := filepath.Join(dir, clean)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		if existing, err := os.ReadFile(p); err == nil && bytes.Equal(existing, files[name]) {
			continue // unchanged (watch mode)
		}
		if err := os.WriteFile(p, files[name], 0o600); err != nil {
			return err
		}
	}
	return nil
}

var diagRe = regexp.MustCompile(`^(.*?):(\d+):(\d+): (error|warning): (.*)$`)

func parseDiagnostics(out, dir string) []Diagnostic {
	var diags []Diagnostic
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if m := diagRe.FindStringSubmatch(line); m != nil {
			l, _ := strconv.Atoi(m[2])
			c, _ := strconv.Atoi(m[3])
			file := m[1]
			if rel, err := filepath.Rel(dir, file); err == nil && !strings.HasPrefix(rel, "..") {
				file = filepath.ToSlash(rel)
			}
			diags = append(diags, Diagnostic{Severity: m[4], File: file, Line: l, Column: c, Message: m[5]})
			continue
		}
		for _, sev := range []string{"error", "warning"} {
			if strings.HasPrefix(line, sev+": ") {
				diags = append(diags, Diagnostic{Severity: sev, Message: strings.TrimPrefix(line, sev+": ")})
			}
		}
	}
	return diags
}
