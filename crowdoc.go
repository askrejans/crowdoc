// Package crowdoc converts documents — Markdown, Word, OpenDocument, RTF,
// HTML, EPUB, Jupyter notebooks, spreadsheets and plain text — into
// beautifully typeset PDF.
//
// The pipeline is: read the source into a format-neutral document tree
// (package ast), normalise it, resolve citations and images, render it with
// a style into a Typst project, and compile that project to PDF.
//
// Convert does everything in one call. Hosts that cannot start processes
// (mobile apps, WebAssembly) call Render to obtain the Typst project and
// compile it with an embedded Typst, or pass their own Engine.
package crowdoc

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/engine"
	"github.com/askrejans/crowdoc/v2/internal/pipeline"
)

// Version is the library version.
const Version = pipeline.Version

// Source is a document to convert. Provide Path, or Data with a Name.
type Source struct {
	// Path is a file to read. When set, Name and BaseDir default from it.
	Path string
	// Name is the file name used for format detection and the fallback
	// title ("report.docx").
	Name string
	// Data is the document content; read from Path when nil.
	Data []byte
	// Format forces the input format; detected when empty.
	Format Format
	// BaseDir resolves relative image and bibliography paths.
	BaseDir string
	// Untrusted marks content supplied by a third party (uploads to a
	// service). Local files outside the document and network access are
	// then refused regardless of other options.
	Untrusted bool
	// Document typesets a document tree built by the caller (for example
	// from the layout package) instead of reading Path or Data. It is
	// normalised in place.
	Document *ast.Document
}

// Options configures a conversion. The zero value is a sensible default.
type Options struct {
	// Style is a built-in style name ("report", "article", ...). Empty
	// selects the style named in the document, or one matching its type.
	Style string
	// TemplatePath or Template replaces the style with a custom Typst
	// template that defines `template(meta, body)`.
	TemplatePath string
	Template     string
	// Meta overrides document metadata using frontmatter keys
	// ("title", "author", "lang", "fonts", "paper", "toc", ...).
	Meta map[string]any
	// Bibliography lists additional bibliography files (.bib, .json,
	// .yaml, .ris, .nbib).
	Bibliography []string
	// BibliographyData supplies bibliography files by content
	// (file name → bytes), e.g. uploads.
	BibliographyData map[string][]byte
	// CitationStyle overrides the citation style (apa, ieee, chicago,
	// harvard, vancouver, mla).
	CitationStyle string
	// PDFStandards are conformance targets such as "a-2b" (archival) or
	// "ua-1" (accessible). Document metadata "pdfa: true" adds "a-2b".
	PDFStandards []string
	// Engine compiles the rendered project. Nil uses the Typst executable
	// (see FindTypst).
	Engine Engine
	// FontDirs adds font directories (installed font packs are included
	// automatically).
	FontDirs []string
	// DeterministicFonts ignores system fonts so output is identical on
	// every machine.
	DeterministicFonts bool
	// AllowRemoteImages permits downloading http(s) images.
	AllowRemoteImages bool
	// UnsafeRaw passes raw Typst blocks from the source through unchanged.
	// Only enable it for trusted input.
	UnsafeRaw bool
	// WorkDir keeps the compilation directory (useful for debugging and
	// incremental watch builds).
	WorkDir string
	// Logger receives progress and diagnostics (nil: silent).
	Logger *slog.Logger
	// Now overrides the current time (reproducible builds and tests).
	Now time.Time
}

// Engine compiles a rendered project to PDF. Implement it to plug in an
// embedded Typst (a Rust library, WebAssembly) or a remote compiler.
type Engine = engine.Engine

// EngineJob is a project handed to an [Engine]: its files, entry file, PDF
// standards and extra font directories.
type EngineJob = engine.Job

// EngineResult is what an [Engine] returns: the PDF and its diagnostics.
type EngineResult = engine.Result

// Diagnostic is a typesetting engine message.
type Diagnostic = engine.Diagnostic

// CompileError is returned when the engine rejects a project.
type CompileError = engine.CompileError

// ErrEngineNotFound is returned when no Typst executable is available.
var ErrEngineNotFound = engine.ErrNotFound

// Bundle is a self-contained Typst project.
type Bundle struct {
	// Main is the entry file name ("main.typ").
	Main string
	// Files maps project-relative paths to contents, including image assets.
	Files map[string][]byte
	// PDFStandards are the requested conformance targets.
	PDFStandards []string
	// FontDirs are font directories the project needs beyond the engine's
	// own (installed font packs, TeX Live subdirectories).
	FontDirs []string
}

// Result is the outcome of a conversion.
type Result struct {
	PDF      []byte
	Bundle   *Bundle
	Document *ast.Document
	// Style is the style that was used.
	Style string
	// Warnings lists recoverable problems (missing images, unknown
	// citation keys, unsupported content, ...).
	Warnings []string
	// Timings of the pipeline stages.
	ParseTime, RenderTime, CompileTime time.Duration
}

func (src Source) internal() pipeline.Source {
	return pipeline.Source{Path: src.Path, Name: src.Name, Data: src.Data, Format: src.Format, BaseDir: src.BaseDir, Untrusted: src.Untrusted, Document: src.Document}
}

func (o Options) internal() pipeline.Options {
	return pipeline.Options{
		Style: o.Style, TemplatePath: o.TemplatePath, Template: o.Template, Meta: o.Meta,
		Bibliography: o.Bibliography, BibliographyData: o.BibliographyData, CitationStyle: o.CitationStyle,
		PDFStandards: o.PDFStandards, Engine: o.Engine, FontDirs: o.FontDirs, DeterministicFonts: o.DeterministicFonts,
		AllowRemoteImages: o.AllowRemoteImages, UnsafeRaw: o.UnsafeRaw, WorkDir: o.WorkDir, Logger: o.Logger, Now: o.Now,
	}
}

func render(ctx context.Context, src Source, opts Options) (*Result, error) {
	r, err := pipeline.Render(ctx, src.internal(), opts.internal())
	if err != nil {
		return nil, err
	}
	return &Result{
		Document: r.Document, Style: r.Style, Warnings: r.Warnings, ParseTime: r.ParseTime, RenderTime: r.RenderTime,
		Bundle: &Bundle{Main: "main.typ", Files: r.Files, PDFStandards: r.PDFStandards, FontDirs: r.FontDirs},
	}, nil
}

// Convert reads src and produces a PDF.
func Convert(ctx context.Context, src Source, opts Options) (*Result, error) {
	res, err := render(ctx, src, opts)
	if err != nil {
		return nil, err
	}
	eng := opts.Engine
	if eng == nil {
		eng, err = pipeline.DefaultEngine(opts.internal())
		if err != nil {
			return nil, err
		}
	}
	start := time.Now()
	out, err := eng.Compile(ctx, &engine.Job{
		Files:        res.Bundle.Files,
		Main:         res.Bundle.Main,
		PDFStandards: res.Bundle.PDFStandards,
		WorkDir:      opts.WorkDir,
		FontPaths:    res.Bundle.FontDirs,
	})
	res.CompileTime = time.Since(start)
	if err != nil {
		return res, err
	}
	for _, d := range out.Diagnostics {
		if d.Severity == "warning" {
			res.Warnings = appendUnique(res.Warnings, "typesetting: "+d.Message)
		}
	}
	res.PDF = out.PDF
	if opts.Logger != nil {
		opts.Logger.Debug("converted", "style", res.Style, "parse", res.ParseTime, "render", res.RenderTime, "compile", res.CompileTime)
	}
	return res, nil
}

// ConvertFile converts the file at in and writes the PDF to out
// ([DefaultOutputPath] when empty). The PDF is written atomically.
func ConvertFile(ctx context.Context, in, out string, opts Options) (*Result, error) {
	if out == "" {
		out = DefaultOutputPath(in)
	}
	if sameFile(in, out) {
		return nil, fmt.Errorf("output %s would overwrite the input", out)
	}
	res, err := Convert(ctx, Source{Path: in}, opts)
	if err != nil {
		return res, err
	}
	if err := writeAtomic(out, res.PDF); err != nil {
		return res, err
	}
	return res, nil
}

// DefaultOutputPath is the PDF written for in when no output is given: the
// input name with a .pdf extension, or name.typeset.pdf when the input is
// itself a PDF.
func DefaultOutputPath(in string) string {
	ext := filepath.Ext(in)
	base := strings.TrimSuffix(in, ext)
	if strings.EqualFold(ext, ".pdf") {
		return base + ".typeset.pdf"
	}
	return base + ".pdf"
}

// Render reads src and returns the Typst project without compiling it.
func Render(ctx context.Context, src Source, opts Options) (*Result, error) {
	return render(ctx, src, opts)
}

// Parse reads src into a normalised document tree.
func Parse(ctx context.Context, src Source, opts Options) (*ast.Document, []string, error) {
	p, err := pipeline.Prepare(ctx, src.internal(), opts.internal())
	if err != nil {
		return nil, nil, err
	}
	return p.Doc(), p.Warnings(), nil
}

// FindTypst returns the path of the Typst executable crowdoc will use.
func FindTypst() (string, error) { return engine.FindTypst() }

// InstallTypst downloads and verifies the Typst release crowdoc is tested
// with into the crowdoc tools directory and returns its path.
func InstallTypst(ctx context.Context) (string, error) {
	return engine.InstallTypst(ctx, "", nil)
}

// TypstVersion is the Typst release crowdoc targets.
const TypstVersion = engine.TypstVersion

// NewTypstEngine returns an Engine running the given Typst executable.
func NewTypstEngine(path string, fontDirs []string, deterministic bool) Engine {
	return &engine.Typst{Path: path, FontPaths: fontDirs, IgnoreSystemFonts: deterministic}
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".crowdoc-*.pdf")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

func sameFile(a, b string) bool {
	fa, err1 := os.Stat(a)
	fb, err2 := os.Stat(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return os.SameFile(fa, fb)
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}
