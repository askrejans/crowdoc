# Using crowdoc as a Go library

```sh
go get github.com/askrejans/crowdoc/v2
```

```go
import "github.com/askrejans/crowdoc/v2"
```

The package converts a source document into PDF in one call, or stops after
any stage so a host application can show an outline, preview the Typst
source, or compile with its own embedded engine.

```
Source ──Parse──▶ *ast.Document ──Render──▶ Bundle (Typst project) ──Engine──▶ PDF
          │                         │                                   │
       Inspect                 WriteBundle                         Convert / ConvertFile
```

## Convert

```go
res, err := crowdoc.ConvertFile(ctx, "report.docx", "report.pdf", crowdoc.Options{
	Style: "report",
	Meta: map[string]any{
		"colors": "oxford",       // colour scheme
		"fonts":  "source",       // typeface pairing
		"lang":   "lv",
		"toc":    true,
	},
	Bibliography: []string{"refs.bib"},
	PDFStandards: []string{"a-2b"}, // archival PDF/A
})
if err != nil {
	var ce *crowdoc.CompileError // typesetting diagnostics
	...
}
for _, w := range res.Warnings { log.Println(w) } // missing images, unknown keys, …
```

`Convert` takes a `Source` instead of paths:

```go
res, err := crowdoc.Convert(ctx, crowdoc.Source{
	Name: "notes.md",          // selects the reader and the fallback title
	Data: markdownBytes,
	BaseDir: "/srv/uploads/42", // where relative images and .bib files live
	Untrusted: true,            // uploads: no files outside BaseDir, no network
}, crowdoc.Options{})
pdf := res.PDF
```

`Untrusted` sources may only read files inside `BaseDir` (no absolute paths,
`..` or symbolic links leaving it), never fetch remote images, and never pass
raw Typst through. Use it for anything a third party supplied.

## Options

| Field | Purpose |
|---|---|
| `Style` | Built-in style name (`crowdoc.Styles()`) |
| `TemplatePath`, `Template` | Custom Typst template defining `template(meta, body)` (start from `crowdoc.ExportStyle`) |
| `Meta` | Frontmatter overrides — any key from [authoring.md](authoring.md) |
| `Bibliography`, `BibliographyData` | Extra bibliography files by path or by content |
| `CitationStyle` | `apa`, `chicago`, `ieee`, `harvard`, `vancouver`, `mla` |
| `PDFStandards` | `a-1b` … `a-4`, `ua-1` |
| `Engine` | Custom engine (see below); nil runs the Typst executable |
| `FontDirs`, `DeterministicFonts` | Extra font directories; ignore system fonts for identical output everywhere |
| `AllowRemoteImages` | Download `http(s)` images (trusted sources only) |
| `UnsafeRaw` | Pass ```` ```{=typst} ```` blocks through (trusted sources only) |
| `WorkDir` | Keep the compilation directory; reused between runs for fast rebuilds |
| `Logger`, `Now` | `log/slog` diagnostics; fixed clock for reproducible output |

## Inspect and parse

```go
outline, _ := crowdoc.Inspect(ctx, crowdoc.Source{Path: "thesis.docx"}, crowdoc.Options{})
fmt.Println(outline.Title, outline.Language, outline.Counts.Figures, outline.Citations)
for _, h := range outline.Headings { fmt.Println(strings.Repeat("  ", h.Level-1) + h.Text) }

doc, warnings, err := crowdoc.Parse(ctx, src, opts) // the full *ast.Document
```

## Documents built in code

The `ast` package is public. Build or modify a tree and typeset it directly
with `Source.Document`; the reader is skipped, everything after it
(normalisation, citations, styles, languages) runs as usual:

```go
doc := &ast.Document{Blocks: []ast.Block{
	&ast.Heading{Level: 1, Inlines: ast.Str("Quarterly review")},
	&ast.Para{Inlines: []ast.Inline{
		&ast.Text{Value: "Revenue grew "},
		&ast.Strong{Inlines: ast.Str("12 %")},
		&ast.Text{Value: " on "},
		&ast.Math{TeX: `\frac{a}{b}`},
	}},
}}
res, err := crowdoc.Convert(ctx, crowdoc.Source{Name: "review", Document: doc}, crowdoc.Options{Style: "report"})
```

Images in such a tree are referenced by path (resolved against
`Source.BaseDir`) or stored in `doc.Resources` and referenced as `res:<name>`.

## Positioned text and OCR: the `layout` package

`github.com/askrejans/crowdoc/v2/layout` rebuilds a structured document —
columns in reading order, headings, paragraphs, lists, tables, figures with
captions, footnotes, quotations, code — from text placed on pages. The PDF
reader uses it for text-layer PDFs; it works the same on OCR output:

```go
pages := []layout.Page{{
	Width: 595, Height: 842, // points, origin top-left
	Runs: []layout.Run{
		{Text: "Annual", X: 72, Y: 80, W: 60, H: 22}, // one run per recognised word
		{Text: "report", X: 136, Y: 80, W: 62, H: 22},
		// …
	},
}}
doc, warnings := layout.Document(ctx, pages, layout.Options{Lang: "en"})
res, err := crowdoc.Convert(ctx, crowdoc.Source{Name: "scan", Document: doc}, crowdoc.Options{})
```

Leave `FontSize` at 0 for OCR words: sizes are then estimated per line and
headings are recognised from relative box heights and spacing. The package
documentation describes how to map Tesseract, hOCR and ALTO coordinates.

## Catalogues

`crowdoc.Styles()`, `ColorSchemes()`, `ColorRoles()`, `FontPairings()`,
`FontFamilies()`, `CitationStyles()`, `Languages()` and `Formats()` describe
everything selectable, with JSON tags for APIs and user interfaces.
`StyleSource(name)` and `ExportStyle(name, dir)` return a style's Typst source.

## Engines and embedding

The default engine runs the Typst executable (`crowdoc engine install`
downloads the pinned, checksum-verified release; `$CROWDOC_TYPST` or `$PATH`
work too). Desktop and server programs need nothing else.

Platforms that cannot start processes — iOS, Android, WebAssembly — split the
pipeline: Go renders, the host compiles.

```go
res, err := crowdoc.Render(ctx, src, opts) // no engine needed
// res.Bundle.Files: "main.typ", "style.typ", "crowdoc.typ", "assets/…"
// res.Bundle.PDFStandards, res.Bundle.FontDirs
```

Hand the bundle to Typst compiled into the host (the `typst` Rust crate with a
`World` serving `Bundle.Files` and the app's bundled fonts, or the Typst
WebAssembly build). Alternatively implement `crowdoc.Engine`:

```go
type Engine interface {
	Name() string
	Compile(ctx context.Context, job *crowdoc.EngineJob) (*crowdoc.EngineResult, error)
	Fonts(ctx context.Context) (map[string]bool, error) // families the engine can use
}
```

`EngineJob` carries `Files`, `Main`, `PDFStandards` and `FontPaths`;
`EngineResult` holds the `PDF`, `Diagnostics` and `Duration`. Return a
`*crowdoc.CompileError` when the project does not compile.

`Fonts` lets crowdoc choose fonts the host actually has, so the generated
project never asks for a missing family.

## Fonts

`crowdoc.InstallFonts(ctx, []string{"core"}, progress)` downloads the curated
open-licence families (68 families; sets `core`, `extended`, `cjk`, `emoji`)
into the user cache directory, verifying SHA-256 checksums.
`crowdoc.FontDirs(nil)` returns the directories crowdoc uses. TeX Live font
trees are found automatically; only the subdirectories a document needs are
given to the engine.

## Concurrency and performance

All functions are safe for concurrent use. A typical document renders in
about a millisecond and typesets in 100–300 ms. The first run on a machine
builds a font-name cache (about two seconds with TeX Live installed).
