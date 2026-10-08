# crowdoc — development guide

Module `github.com/askrejans/crowdoc/v2` (Go 1.26). GPL-3.0. A general-purpose
open-source library and CLI: keep everything generic — no references to any
particular product, company deployment or service that uses it.

## Pipeline

```
reader ──▶ *ast.Document ──normalize──▶ citations (transform/cite) ──▶ typst writer + style ──▶ engine ──▶ PDF
```

| Package | Responsibility |
|---|---|
| `crowdoc` (root) | Public API only: `Convert`, `ConvertFile`, `Render`, `Parse`, `Inspect`, catalogues, format detection |
| `ast` | Format-neutral document model (public) |
| `layout` | Layout reconstruction from positioned text — PDF text layers, OCR word boxes (public) |
| `internal/pipeline` | Wiring: read → normalise → cite → render → engine; style choice, fonts, language |
| `internal/reader/rd` | Reader options, limits (`rd.OpenZip` enforces zip-bomb limits), helpers |
| `internal/reader/*` | One package per input format, each exporting `Read(ctx, data, rd.Options)` |
| `internal/transform/metamap` | Frontmatter / `--meta` / API metadata → `ast.Meta` |
| `internal/transform/normalize` | Title inference, heading levels, abstract/keywords, cross-references, manual numbering |
| `internal/transform/cite` | BibTeX/CSL/RIS/NBIB parsing and APA, Chicago, IEEE, Harvard, Vancouver, MLA formatting |
| `internal/mathconv/*` | OMML → LaTeX, MathML → LaTeX, LaTeX math → Typst math |
| `internal/typst` | Typst writer, styles (`styles/*.typ`), shared library (`assets/crowdoc.typ`), palettes |
| `internal/support/media` | Image resolution, sniffing, sizing, conversion, path confinement |
| `internal/support/fonts` | Font catalogue, pairings, installer, scanner (Typst-compatible family names) |
| `internal/support/locale` | 40-language captions, labels, dates, language detection |
| `internal/support/hyph` | Liang hyphenation for languages Typst does not hyphenate |
| `internal/engine` | Typst runner, diagnostics, pinned installer |
| `cmd/crowdoc` | CLI (keeps the v1 flags working) |

## Rules

- Readers never put markup escapes into `ast.Text`; only `ast.Math`/`MathBlock`
  carry LaTeX math. Writers escape.
- Untrusted input (`Source.Untrusted`): no files outside `BaseDir`, no network,
  no raw passthrough. Archives go through `rd.OpenZip`. Respect `ctx`.
- Dependencies must be permissive (MIT/BSD/Apache-2.0) and pure Go (no cgo).
- Styles: every `styles/*.typ` defines `template(meta, body)`, calls
  `cd-base`, reads colours through `cd-color(meta, role, default)` and fills
  through `cd-tint`, and uses explicit point sizes inside heading show rules.
  Register new styles in `internal/typst/styles.go`.
- Typst pitfalls: `color`, `left`, `right`, `top`, `terms`, `as` are builtins or
  keywords — never shadow them; quote dictionary keys that are keywords;
  a `;` right after an embedded call is consumed.

## Testing

```sh
go test ./...                                        # unit tests
CROWDOC_TYPST=$(which typst) go test ./...           # + compile and round-trip tests
go build -o crowdoc ./cmd/crowdoc && ./crowdoc --batch examples/ /tmp/out
```

The escaping round-trip test compiles random text through Typst's HTML export
and must stay green. Visual changes to styles should be checked by rendering
`examples/` and looking at the pages.
