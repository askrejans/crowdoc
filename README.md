# crowdoc

**Universal document-to-PDF converter with beautiful LaTeX typography.**

Transform Markdown, CSV, Excel, HTML, or plain text files into professionally typeset PDFs. Whether you're writing technical documentation, formatting spreadsheet data, converting web articles, or producing legal agreements, crowdoc produces stunning output with zero configuration.

## Features

- **Multi-format input** -- Markdown, CSV, XLSX (Excel), HTML, and plain text
- **8 built-in styles** -- legal, technical, report, minimal, letter, academic, invoice, memo
- **Full Markdown support** -- headings, bold/italic, lists, tables, code blocks, images, blockquotes, footnotes, links
- **Spreadsheet support** -- CSV and XLSX files rendered as formatted tables with auto-detected delimiters
- **HTML conversion** -- headings, tables, lists, code blocks, links, and inline formatting preserved
- **Code blocks** with syntax highlighting via LaTeX `listings`
- **Math support** -- inline `$E=mc^2$` and display `$$\sum_{i=1}^n$$`
- **Auto-detected styling** -- crowdoc picks the right style based on your content
- **Table of contents** -- auto-generated for documents with 3+ sections
- **Image embedding** -- `![caption](path.png)` rendered with captions
- **Frontmatter control** -- fine-tune every aspect via YAML metadata
- **Batch conversion** -- convert entire directories at once (all supported formats)
- **Watch mode** -- regenerate on file save
- **Cross-platform** -- macOS, Linux, Windows (auto-detects LaTeX engine)
- **Zero Go dependencies** -- single binary, just needs LaTeX

## Installation

### From source

```bash
go install github.com/askrejans/crowdoc@latest
```

### Build locally

```bash
git clone https://github.com/askrejans/crowdoc.git
cd crowdoc
go build -o crowdoc .
```

### Requirements

A LaTeX distribution with LuaLaTeX (preferred) or XeLaTeX. crowdoc auto-detects whichever engine is available on your PATH.

**macOS:**
```bash
brew install --cask mactex-no-gui
```

**Ubuntu / Debian:**
```bash
sudo apt install texlive-full
```

**Fedora:**
```bash
sudo dnf install texlive-scheme-full
```

**Arch Linux:**
```bash
sudo pacman -S texlive-most
```

**Windows:**

Install [MiKTeX](https://miktex.org/download) or [TeX Live](https://tug.org/texlive/windows.html). Both provide `lualatex` and `xelatex`. MiKTeX auto-installs missing LaTeX packages on first use. For TeX Live, use the full installer.

**Recommended fonts** (optional -- graceful fallbacks to Latin Modern built in):

| Font | macOS | Ubuntu/Debian | Windows |
|------|-------|---------------|---------|
| EB Garamond | `brew install --cask font-eb-garamond` | `sudo apt install fonts-ebgaramond` | [Google Fonts](https://fonts.google.com/specimen/EB+Garamond) |
| Inter | `brew install --cask font-inter` | `sudo apt install fonts-inter` | [Google Fonts](https://fonts.google.com/specimen/Inter) |
| JetBrains Mono | `brew install --cask font-jetbrains-mono` | `sudo apt install fonts-jetbrains-mono` | [JetBrains](https://www.jetbrains.com/lp/mono/) |

## Usage

### Single file

```bash
crowdoc document.md                    # Markdown → PDF
crowdoc report.csv                     # CSV → PDF (formatted table)
crowdoc data.xlsx                      # Excel → PDF (all sheets)
crowdoc article.html                   # HTML → PDF
crowdoc notes.txt                      # Plain text → PDF
crowdoc document.md output.pdf         # Custom output path
crowdoc --style technical spec.md      # Force a style
```

### Batch conversion

```bash
crowdoc --batch docs/                  # Convert all supported files to docs/pdf/
crowdoc --batch docs/ output/          # Custom output directory
```

Batch mode processes all supported formats: `.md`, `.csv`, `.xlsx`, `.txt`, `.html`.

### Watch mode

```bash
crowdoc --watch document.md            # Regenerates on every save
```

### Options

```
  -s, --style <name>     Style preset (legal, technical, report, minimal, letter, academic, invoice, memo)
  -b, --batch <dir>      Batch convert all supported files in directory
  -w, --watch            Watch file for changes and regenerate
      --toc              Force table of contents
      --no-toc           Disable table of contents
      --no-title-page    Skip the title page
      --no-signatures    Skip signature blocks (legal style)
      --font-size <n>    Base font size: 10, 11, or 12
  -v, --version          Show version
      --list-styles      Show available styles
  -h, --help             Show help
```

## Styles

### `legal`
Gold accents, formal typography, signature blocks. Designed for contracts, NDAs, and legal agreements. Auto-detected for documents with "agreement", "contract", or "NDA" in the title.
([source](examples/legal-agreement.md) | [pdf](examples/legal-agreement.pdf))

### `technical`
Sans-serif body text, wider margins for code blocks, GitHub-inspired color palette. Ideal for API docs, specifications, and technical guides.
([source](examples/technical-doc.md) | [pdf](examples/technical-doc.pdf))

### `report`
Professional cover page with dark header band, serif body, clean section formatting. Great for business reports, proposals, and analyses.
([source](examples/business-report.md) | [pdf](examples/business-report.pdf))

### `minimal`
No title page, no frills. Clean serif typography with subtle formatting. Perfect for notes, essays, and general writing.
([source](examples/minimal-notes.md) | [pdf](examples/minimal-notes.pdf))

### `letter`
Formal business letter layout with sender/recipient blocks, date, and subject line. Includes signature area.
([source](examples/business-letter.md) | [pdf](examples/business-letter.pdf))

### `academic`
Double-spaced serif typography with abstract block, numbered sections, and running headers. Designed for research papers, theses, and journal articles. Auto-detected for documents with "paper", "thesis", "research", or "study" in the title.
([source](examples/academic-paper.md) | [pdf](examples/academic-paper.pdf))

### `invoice`
Bold invoice header with number/date/status, clean sans-serif body optimized for tables and line items. Auto-detected for documents with "invoice", "bill", or "receipt" in the title.
([source](examples/invoice-sample.md) | [pdf](examples/invoice-sample.pdf))

### `memo`
Structured TO/FROM/DATE/RE header block with rose accent color and sans-serif typography. Auto-detected for documents with "memo", "memorandum", or "notice" in the title.
([source](examples/memo-internal.md) | [pdf](examples/memo-internal.pdf))

## Input Formats

### Markdown (`.md`, `.markdown`)
Full Markdown support with YAML frontmatter, sections, code blocks, tables, math, images, and footnotes.

### CSV (`.csv`)
Auto-detects delimiter (comma, semicolon, tab). First row becomes table header. Title derived from filename. Default style: `report`.
([source](examples/quarterly-sales.csv) | [pdf](examples/quarterly-sales.pdf))

### Excel (`.xlsx`)
Reads all sheets — each becomes a section with a formatted table. Supports shared strings, numbers, and booleans. Parsed using Go stdlib only (no external dependencies). Old `.xls` binary format is not supported.
([source](examples/employee-data.xlsx) | [pdf](examples/employee-data.pdf))

### HTML (`.html`, `.htm`)
Converts headings, paragraphs, tables, lists, code blocks, blockquotes, links, images, and inline formatting (bold, italic, code, strikethrough). Strips `<script>`, `<style>`, `<nav>`, and `<footer>` blocks. Title extracted from `<title>` or first `<h1>`.
([source](examples/article-webpage.html) | [pdf](examples/article-webpage.pdf))

### Plain Text (`.txt`)
First line becomes title. ALL CAPS lines promoted to section headings. Indented blocks (4 spaces or tab) become code blocks. Bullet and numbered lists preserved. Default style: `minimal`.
([source](examples/plain-readme.txt) | [pdf](examples/plain-readme.pdf))

## Frontmatter Reference

Control document metadata and rendering with YAML frontmatter:

```yaml
---
title: My Document
subtitle: A comprehensive guide
date: 2026-03-23
version: 2.0
status: FINAL
type: technical
style: report
summary: Brief description for the cover page.
author: Jane Smith
language: en
classification: INTERNAL
toc: true
signatures: false
logo: assets/logo.png
font-size: 11
header-left: Custom Header
header-right: Confidential
footer-left: Draft v2
footer-right: Acme Corp
margin-top: 2.5cm
margin-bottom: 2.5cm
margin-left: 3cm
margin-right: 3cm
---
```

All fields are optional. Sensible defaults are applied for everything.

## Markdown Features

### Inline formatting
- `**bold**` and `*italic*`
- `` `inline code` ``
- `[link text](url)` -- rendered with footnote URLs
- `[^1]` footnotes with `[^1]: definition` at end of file

### Code blocks
````markdown
```python
def hello():
    print("Hello, world!")
```
````

### Images
```markdown
![Architecture diagram](diagrams/arch.png)
```
Images are auto-sized to fit the text width with alt text as caption.

### Math
- Inline: `$E = mc^2$`
- Display: `$$\int_0^\infty e^{-x^2} dx = \frac{\sqrt{\pi}}{2}$$`

### Tables
```markdown
| Feature  | Status |
|----------|--------|
| Tables   | Done   |
| Images   | Done   |
```

### Blockquotes
```markdown
> This will render with a styled left border
> and light background.
```

## License

GPL-3.0 License. Copyright 2026 Arvis Skrējāns.

See [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome. Please:

1. Fork the repository
2. Create a feature branch
3. Write clean, tested Go code
4. Submit a pull request

For bug reports and feature requests, open an issue on GitHub.

## AI Training Opt-Out

This repository and its contents are **not licensed for use in training AI/ML models**. This opt-out is declared via:

- `robots.txt` — blocks known AI training crawlers (GPTBot, CCBot, Google-Extended, etc.)
- `ai.txt` — Spawning.ai AI training opt-out declaration
- `.ai-training-opt-out` — explicit opt-out marker file
- **GPL-3.0 license** — derivative works (including trained models) must be released under the same license

---

Built by [Arvis Skrējāns](https://github.com/askrejans).
