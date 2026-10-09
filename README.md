# crowdoc

**Beautifully typeset PDFs from any document.**

crowdoc turns Markdown, Word, OpenDocument, RTF, HTML, EPUB, Jupyter
notebooks, spreadsheets, plain text and text-based PDFs into documents that
look professionally typeset: proper hyphenation in 40 languages, real
small caps and old-style figures, numbered figures and tables, cross-references,
footnotes, mathematics, and citations formatted to APA, Chicago, IEEE, Harvard,
Vancouver or MLA.

It is a single Go binary and a Go library. Typesetting is done by
[Typst](https://typst.app) — a 15 MB engine instead of a multi-gigabyte TeX
installation — so a typical document is ready in a fraction of a second.

```sh
crowdoc report.docx                          # → report.pdf, style chosen from the content
crowdoc --style thesis --colors oxford --fonts classic thesis.md
crowdoc --batch notes/ pdf/                  # a whole folder, in parallel
crowdoc --watch paper.md                     # rebuild on every save
```

## Highlights

- **29 styles** for academic, publishing, business, legal, technical,
  correspondence and personal documents — from a two-column conference paper
  and an APA manuscript to invoices, letters for window envelopes, minutes,
  CVs and 16:9 slides.
- **34 colour schemes** (Oxford, Bordeaux, Emerald, Ink, Ocean, Forest, Cobalt,
  tinted-paper Newsprint, Ivory and Kraft, High contrast, dark Night and Dusk
  schemes for screens, …) plus per-role colour overrides, applied to every
  style.
- **23 typeface pairings** over 68 curated, openly licensed families (EB
  Garamond, Source Serif, STIX Two, IBM Plex, Inter, Literata, Libertinus, Noto
  for Arabic, Hebrew and CJK, …), downloaded on demand and verified by
  checksum. Without them, crowdoc still works with the faces built into Typst.
- **40 languages**: localised captions ("Figure 3", "3. attēls", "図3"), dates,
  quotation marks, page labels and hyphenation — including languages the
  engine does not hyphenate itself (Latvian, Romanian, Macedonian, Irish,
  Basque, Montenegrin).
- **Citations without biber**: BibTeX/BibLaTeX, CSL-JSON/YAML, RIS and PubMed
  files; Zotero, Mendeley and Word citations inside .docx files; numbered
  hand-written reference lists linked automatically.
- **Faithful readers**: Word styles, multi-level lists, merged table cells,
  images with captions, footnotes, equations (Word OMML and MathML become
  typeset math), tracked changes, EPUB chapters, notebook outputs, Excel
  dates and merged cells, and PDF layout reconstruction (columns, headings,
  lists, tables, de-hyphenation).
- **Images just work**: PNG, JPEG (with EXIF rotation), GIF, WebP, SVG, PDF,
  BMP and TIFF; HEIC on macOS; data URIs; natural sizing that never blows up
  small images.
- **Archival and accessible output**: PDF/A-1b … A-4 and tagged PDF/UA-1.
- **Safe for untrusted input**: uploads cannot read files outside their own
  directory, fetch URLs or inject raw markup; archives are size-limited.
- **A library first**: parse, inspect, render or convert from Go; hosts
  without process support (mobile, WebAssembly) render the Typst project and
  compile it with an embedded Typst.

## Installation

```sh
go install github.com/askrejans/crowdoc/v2/cmd/crowdoc@latest
crowdoc engine install        # downloads the pinned Typst release (≈15 MB, checksum-verified)
crowdoc fonts install         # optional: the "core" font set (≈33 MB); also extended, cjk, emoji
```

crowdoc also uses `typst` from `$PATH` or `$CROWDOC_TYPST`. TeX Live's font
collection is picked up automatically when it is installed.

v1 (the LaTeX-based converter) remains available as
`go install github.com/askrejans/crowdoc@v1.3.0`; its command-line flags keep
working in v2.

## Usage

```
crowdoc [options] <input> [output.pdf]
crowdoc --batch <dir> [outdir]
crowdoc --watch <input>
crowdoc engine install|status
crowdoc fonts install [core|extended|cjk|emoji|all]…|list
crowdoc style export <name> [dir]
```

| Option | |
|---|---|
| `-s, --style <name>` | Style (`--list-styles`) |
| `--colors <scheme>`, `--color role=#hex` | Colour scheme and overrides (`--list-colors`) |
| `--fonts <pairing>`, `--font`, `--sans-font`, `--mono-font`, `--math-font` | Typefaces (`--list-fonts`) |
| `--lang <code>` | Document language (`--list-languages`); detected when omitted |
| `--bib <file>`, `--csl <style>` | Bibliography and citation style (`--list-citation-styles`) |
| `--paper`, `--landscape`, `--columns`, `--font-size`, `--line-spacing`, `--margin` | Page layout |
| `--toc`, `--lof`, `--lot`, `--number-sections`, `--title-page`, `--signatures` (and `--no-…`) | Document parts |
| `--title`, `--subtitle`, `--author "A; B"`, `--date`, `--organization`, `--status`, `--classification`, `--summary` | Metadata |
| `-M, --meta key=value` | Any frontmatter key |
| `--pdfa`, `--accessible`, `--pdf-standard <list>` | PDF/A-2b, PDF/UA-1, others |
| `--typst`, `--source-dir <dir>` | Write the Typst project instead of a PDF |
| `-t, --template <file.typ>` | Custom template (start from `crowdoc style export`) |
| `-j, --jobs <n>` | Parallel conversions in batch mode |
| `--fetch-images`, `--deterministic-fonts`, `--font-dir`, `--workdir`, `--timeout`, `--engine-path` | Resources and engine |

## Styles

| Category | Styles |
|---|---|
| Academic | `article`, `paper` (two-column), `apa`, `essay` (MLA), `thesis`, `preprint`, `manuscript` (line-numbered), `notes` |
| Publishing | `book`, `elegant`, `newsletter`, `slides` |
| Business | `report`, `proposal`, `whitepaper`, `brief`, `invoice`, `minutes`, `policy`, `data` |
| Technical | `technical`, `manual` |
| Legal | `legal`, `ligums` (Latvian agreements) |
| Correspondence | `letter` (DIN 5008 window on A4), `memo` |
| General & personal | `minimal`, `modern`, `cv` |

The style is chosen from frontmatter `style:`, the document type or title
("Invoice …", "Līgums …", "Meeting minutes …") or the input format, and
defaults to `report`. Every style honours colour schemes and typeface
pairings.

## Input formats

| Format | Extensions | Notes |
|---|---|---|
| Markdown (CommonMark, GFM, Pandoc extensions) | `.md` `.markdown` `.mdown` `.mkd` `.mdx` `.qmd` `.rmd` | Frontmatter, citations, math, fenced divs, alerts |
| Word | `.docx` `.docm` `.dotx` | Styles, lists, tables, images, footnotes, OMML equations, citations, tracked changes |
| OpenDocument text | `.odt` `.fodt` `.ott` | Styles, lists, tables, images, notes, MathML formulas |
| RTF | `.rtf` | Formatting, tables, embedded pictures |
| HTML | `.html` `.htm` `.xhtml` | Article content, figures, tables, MathML |
| EPUB | `.epub` | Chapters in spine order, images |
| Jupyter notebook | `.ipynb` | Markdown cells, code, text and image outputs |
| CSV / TSV | `.csv` `.tsv` `.tab` | Delimiter and header detection |
| Excel | `.xlsx` `.xlsm` | Every sheet, number formats, dates, merged cells |
| OpenDocument spreadsheet | `.ods` `.fods` | Every sheet, merged cells |
| Plain text | `.txt` `.text` | Headings, lists and paragraphs recognised |
| PDF with a text layer | `.pdf` | Layout reconstruction: columns, headings, lists, tables, footnotes, de-hyphenation; scanned PDFs need OCR first |

The format is detected from the content when the extension is missing or
wrong.

See [docs/authoring.md](docs/authoring.md) for the Markdown features:
frontmatter, citations, cross-references, callouts, figures, tables and math.

## Library

```go
import "github.com/askrejans/crowdoc/v2"

res, err := crowdoc.ConvertFile(ctx, "thesis.docx", "thesis.pdf", crowdoc.Options{
	Style: "thesis",
	Meta:  map[string]any{"colors": "oxford", "fonts": "classic", "lang": "lv"},
})
```

`Parse`, `Inspect` (outline and statistics), `Render` (Typst project without
compiling), typesetting a document tree built in code (`Source.Document`),
custom engines and the catalogue functions are described in
[docs/library.md](docs/library.md). The `layout` package rebuilds documents
from positioned text — a PDF text layer or OCR word boxes.

## Licence

GPL-3.0. Copyright 2026 Arvis Skrējāns. See [LICENSE](LICENSE).

Dependencies are permissively licensed (MIT, BSD, Apache-2.0). Typst is
Apache-2.0 and runs as a separate program. Fonts are SIL Open Font Licence
1.1 and are downloaded separately; see
[internal/support/fonts/README.md](internal/support/fonts/README.md). Hyphenation patterns
are listed with their licences in
[internal/support/hyph/PATTERNS.md](internal/support/hyph/PATTERNS.md).

## AI training opt-out

This repository and its contents are not licensed for use in training AI/ML
models. The opt-out is declared via `robots.txt`, `ai.txt` and
`.ai-training-opt-out`.
