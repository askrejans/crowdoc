// Package pipeline orchestrates a conversion: format detection, reading,
// metadata, normalisation, citations, language, fonts, images and Typst
// rendering. The public crowdoc package is a thin, documented facade over
// it.
package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/engine"
)

// Source is the document to read (see crowdoc.Source).
type Source struct {
	Path      string
	Name      string
	Data      []byte
	Format    Format
	BaseDir   string
	Untrusted bool
	// Document, when set, is used instead of reading Path/Data.
	Document *ast.Document
}

// Options configures a conversion (see crowdoc.Options).
type Options struct {
	Style              string
	TemplatePath       string
	Template           string
	Meta               map[string]any
	Bibliography       []string
	BibliographyData   map[string][]byte
	CitationStyle      string
	PDFStandards       []string
	Engine             engine.Engine
	FontDirs           []string
	DeterministicFonts bool
	AllowRemoteImages  bool
	UnsafeRaw          bool
	WorkDir            string
	Logger             *slog.Logger
	Now                time.Time
}

// Rendered is a document turned into a Typst project.
type Rendered struct {
	Document     *ast.Document
	Style        string
	Files        map[string][]byte
	PDFStandards []string
	FontDirs     []string
	Warnings     []string
	ParseTime    time.Duration
	RenderTime   time.Duration
}

// Prepared is a read and normalised document.
type Prepared = prepared

// Doc returns the document tree.
func (p *prepared) Doc() *ast.Document { return p.doc }

// Warnings returns the problems found while reading.
func (p *prepared) Warnings() []string { return p.warnings }

// BaseDir returns the directory relative resources are resolved against.
func (p *prepared) BaseDir() string { return p.baseDir }

// Format returns the detected input format.
func (p *prepared) Format() Format { return p.format }

// StyleName returns the chosen style.
func (p *prepared) StyleName() string { return p.style.Name }

// ErrNoInput is returned when a source has neither a path nor data.
var ErrNoInput = errors.New("no input: set Source.Path or Source.Data")

// Prepare reads and normalises src.
func Prepare(ctx context.Context, src Source, opts Options) (*Prepared, error) {
	return prepare(ctx, src, opts)
}

// Render reads src and renders the Typst project.
func Render(ctx context.Context, src Source, opts Options) (*Rendered, error) {
	return render(ctx, src, opts)
}

// Language decides the document language of a prepared document.
func Language(p *Prepared, opts Options) string {
	lang, _ := documentLanguage(p, opts)
	return lang
}

// DefaultEngine returns the shared Typst engine for these options.
func DefaultEngine(opts Options) (engine.Engine, error) { return sharedTypst(opts) }

func logf(l *slog.Logger, msg string, args ...any) {
	if l != nil {
		l.Debug(msg, args...)
	}
}

// Version is the library version (re-exported as crowdoc.Version).
const Version = "2.1.0"
