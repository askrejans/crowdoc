package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/markdown"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
	"github.com/askrejans/crowdoc/v2/internal/support/media"
	"github.com/askrejans/crowdoc/v2/internal/transform/metamap"
	"github.com/askrejans/crowdoc/v2/internal/transform/normalize"
	"github.com/askrejans/crowdoc/v2/internal/typst"
)

type prepared struct {
	doc      *ast.Document
	warnings []string
	format   Format
	baseDir  string
	name     string
	style    *typst.Style
	src      Source
	// bibliography is the formatted reference list (nil when none).
	bibliography *bibResult
}

// prepare reads and normalises the source.
func prepare(ctx context.Context, src Source, opts Options) (*prepared, error) {
	p := &prepared{src: src}
	data := src.Data
	if src.Path != "" {
		if data == nil {
			b, err := os.ReadFile(src.Path)
			if err != nil {
				return nil, fmt.Errorf("reading input: %w", err)
			}
			data = b
		}
		if src.Name == "" {
			src.Name = filepath.Base(src.Path)
		}
		if src.BaseDir == "" {
			if abs, err := filepath.Abs(filepath.Dir(src.Path)); err == nil {
				src.BaseDir = abs
			}
		}
	}
	if data == nil && src.Document == nil {
		return nil, ErrNoInput
	}
	p.name, p.baseDir, p.src = src.Name, src.BaseDir, src
	if src.Document != nil {
		return p.finish(ctx, src.Document, nil, FormatMarkdown, opts)
	}

	format := src.Format
	if format == "" {
		f, err := DetectFormat(src.Name, data)
		if err != nil {
			return nil, err
		}
		format = f
	}
	entry, ok := lookupFormat(format)
	if !ok {
		return nil, fmt.Errorf("input format %q is not supported", format)
	}
	p.format = format

	doc, warns, err := entry.read(ctx, data, rd.Options{Name: src.Name})
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", entry.info.Name, err)
	}
	return p.finish(ctx, doc, warns, format, opts)
}

// finish applies caller metadata, chooses the style, normalises and
// resolves citations.
func (p *prepared) finish(ctx context.Context, doc *ast.Document, warns []string, format Format, opts Options) (*prepared, error) {
	start := time.Now()
	src := p.src
	p.format = format
	if doc.Resources == nil {
		doc.Resources = ast.NewResources()
	}
	p.warnings = append(p.warnings, warns...)

	// Caller metadata overrides the document's own.
	if len(opts.Meta) > 0 {
		parse := func(s string) []ast.Block {
			b, _ := markdown.Fragment(ctx, s, doc.Resources)
			return b
		}
		p.warnings = append(p.warnings, metamap.Apply(&doc.Meta, opts.Meta, parse)...)
	}

	p.style = chooseStyle(doc, format, opts)
	p.warnings = append(p.warnings, normalize.Document(doc, normalize.Options{
		SourceName:      src.Name,
		FallbackTitle:   rd.TitleFromFilename(firstNonEmptyStr(src.Name, "Document")),
		ExtractAbstract: p.style.Abstract,
	})...)
	p.doc = doc
	p.warnings = append(p.warnings, processCitations(ctx, p, opts)...)
	logf(opts.Logger, "parsed", "format", format, "blocks", len(doc.Blocks), "elapsed", time.Since(start))
	return p, nil
}

func render(ctx context.Context, src Source, opts Options) (*Rendered, error) {
	t0 := time.Now()
	p, err := prepare(ctx, src, opts)
	if err != nil {
		return nil, err
	}
	res := &Rendered{Document: p.doc, Style: p.style.Name, ParseTime: time.Since(t0)}
	t1 := time.Now()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	doc := p.doc
	lang, langWarn := documentLanguage(p, opts)
	doc.Meta.Lang = lang
	if langWarn != "" {
		p.warnings = append(p.warnings, langWarn)
	}
	loc := localeFor(lang)

	resolver := media.NewResolver(ctx, doc.Resources, media.Options{
		BaseDir:         p.baseDir,
		AllowLocalFiles: p.baseDir != "",
		RestrictToBase:  src.Untrusted,
		AllowRemote:     opts.AllowRemoteImages && !src.Untrusted,
	})

	var tmpl string
	switch {
	case opts.Template != "":
		tmpl = opts.Template
	case opts.TemplatePath != "":
		b, err := os.ReadFile(opts.TemplatePath)
		if err != nil {
			return nil, fmt.Errorf("reading template: %w", err)
		}
		tmpl = string(b)
	}

	fonts, fontWarns, fontDirsNeeded := resolveFonts(ctx, doc, p.style, opts)
	p.warnings = append(p.warnings, fontWarns...)
	bib := p.bibliography
	bundle, err := typst.Render(doc, typst.Options{
		Style:          p.style,
		TemplateSource: tmpl,
		Lang:           loc.typstLang,
		Region:         loc.typstRegion,
		Terms:          loc.terms,
		Labels:         loc.labels,
		PageOfFmt:      loc.pageOf,
		Fonts:          fonts,
		Unsafe:         opts.UnsafeRaw && !src.Untrusted,
		Generator:      "crowdoc " + Version,
		Now:            nowOr(opts.Now),
		LocalDate:      loc.longDate,
		Hooks: typst.Hooks{
			Image: func(img *ast.Image) typst.Asset {
				a := resolver.Resolve(img.Src)
				return typst.Asset{Path: a.Path, NaturalWidthPt: a.NaturalWidthPt, NaturalHeightPt: a.NaturalHeightPt, Missing: a.Missing}
			},
			Math:      mathHook,
			Hyphenate: hyphenator(lang),
			Bibliography: func() ([]typst.BibEntry, bool, string) {
				if bib == nil {
					return nil, false, ""
				}
				return bib.entries, bib.numeric, doc.Meta.ReferenceSectionTitle
			},
		},
	})
	if err != nil {
		return nil, err
	}
	files := bundle.Files
	for k, v := range resolver.Files() {
		files[k] = v
	}
	standards := append([]string(nil), opts.PDFStandards...)
	if doc.Meta.PDFA && !containsPrefix(standards, "a-") {
		standards = append(standards, "a-2b")
	}
	sort.Strings(standards)
	res.Files, res.PDFStandards, res.FontDirs = files, standards, fontDirsNeeded
	res.Warnings = append(p.warnings, bundle.Warnings...)
	res.RenderTime = time.Since(t1)
	return res, nil
}

func containsPrefix(list []string, prefix string) bool {
	for _, s := range list {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func nowOr(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// chooseStyle picks the style: explicit option, document metadata, then a
// match on the document type or title.
func chooseStyle(doc *ast.Document, format Format, opts Options) *typst.Style {
	for _, name := range []string{opts.Style, doc.Meta.Style} {
		if name == "" {
			continue
		}
		if s, ok := typst.LookupStyle(name); ok {
			return s
		}
	}
	name := styleForType(doc.Meta.DocType, doc.Meta.Title, format)
	// Scholarly signals beat the generic default.
	if name == "report" && doc.Meta.DocType == "" && (len(doc.Meta.Abstract) > 0 || len(doc.Meta.Bibliography) > 0) {
		name = "article"
	}
	if s, ok := typst.LookupStyle(name); ok {
		return s
	}
	s, _ := typst.LookupStyle("report")
	return s
}
