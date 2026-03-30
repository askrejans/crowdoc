# crowdoc -- Development Guide

## Project Structure

```
crowdoc/
  main.go           CLI entry point, argument parsing, format dispatch
  parser.go         Markdown parsing: frontmatter extraction, section splitting
  converter.go      Markdown-to-LaTeX conversion: inline formatting, lists, tables, code, images
  renderer.go       LaTeX template execution and PDF compilation (lualatex/xelatex)
  styles.go         All LaTeX template strings (legal, technical, report, minimal, letter, academic, invoice, memo)
  csv_reader.go     CSV file parsing with auto-delimiter detection
  xlsx_reader.go    XLSX (Excel) parsing via stdlib archive/zip + encoding/xml
  txt_reader.go     Plain text parsing with heading/list detection
  html_reader.go    HTML-to-markdown conversion with tag-based parser
  watcher.go        File watch mode (polling-based, no external deps)
  go.mod            Go module definition
  examples/         Sample files for all styles and formats
```

## Build and Run

```bash
go build -o crowdoc .
./crowdoc examples/technical-doc.md
./crowdoc --style legal examples/legal-agreement.md
```

## Architecture

### Flow
1. `main.go` parses CLI arguments, detects input format from file extension
2. Format readers (csv/xlsx/txt/html) convert their input to markdown, then `parser.go` extracts YAML frontmatter into a `Document` struct and splits body into `Section` structs. Markdown files go directly to the parser.
3. `renderer.go` selects the style template from `styles.go`, executes it with the `Document` data using Go `text/template`
4. `converter.go` provides the `mdToLaTeX` function called from templates -- converts markdown body text to LaTeX (lists, tables, code blocks, images, inline formatting)
5. `renderer.go` writes the `.tex` file to a temp directory and runs `lualatex` twice (for TOC/references), falling back to `xelatex`

### Supported Input Formats
- `.md`, `.markdown` -- Full markdown with YAML frontmatter
- `.csv` -- Auto-detects delimiter (comma/semicolon/tab), first row as header
- `.xlsx` -- Parsed via stdlib `archive/zip` + `encoding/xml`, each sheet becomes a section
- `.txt` -- First line as title, ALL CAPS lines as headings, indented blocks as code
- `.html`, `.htm` -- Tag-based parser converts to markdown intermediate format
- `.xls` -- Not supported (old binary format); shows error suggesting .xlsx

### Key Design Decisions
- **Zero Go dependencies** -- only stdlib. XLSX parsed via archive/zip + encoding/xml. HTML parsed with regex-based converter. Watch mode uses polling instead of fsnotify.
- **All templates are Go string constants** -- no external template files to ship.
- **Shared template components** -- font setup, packages, and common environments are defined as constants and concatenated into each style template.
- **Graceful font fallback** -- uses `\IfFontExistsTF` to chain preferred fonts with system defaults.
- **Two-pass compilation** -- ensures table of contents and cross-references are resolved.

## How to Add a New Style

1. Define a new template constant in `styles.go` (e.g., `const academicTemplate = ...`)
2. Use the shared preamble constants (`sharedFontSetup`, `sharedPackages`, etc.) for consistency
3. Add the style to the `getStyleTemplate()` switch statement
4. Add it to the `printStyleList()` function in `main.go`
5. Optionally map document types to it in `styleFromDocType()` in `parser.go`

### Template Pattern
Every style template must:
- Accept `{{.FontSize}}` for the document class
- Accept `{{or .MarginTop "default"}}` for custom margins
- Use `{{escapeLaTeX .Title}}` for all user-provided text
- Use `{{mdToLaTeX .Content}}` for section body content
- Use `{{if .ShouldShowTOC}}` for conditional TOC
- Use `{{if not .NoTitlePage}}` for conditional title page
- Define all required colors: headingcolor, rulecolor, accentcolor, medgray, codebg, codekey, codestring, codecomment, quotecolor, quotebg, statusgreen, statusamber, statusblue
- Include `\usepackage{lastpage}` and use `\pageref{LastPage}` for page counts

## How to Add a New Input Format

1. Create a `<format>_reader.go` file with a `parse<Format>File(path, inputDir string) (Document, error)` function
2. The reader should convert its input to a markdown string and call `parseMarkdown()` to get a `Document`
3. Add the file extension to `supportedExts` in `main.go`
4. Add the format to `detectFormat()` switch in `main.go`
5. Add the dispatch case in `convertFile()` switch in `main.go`
6. Write tests in `<format>_reader_test.go`

## LaTeX Requirements
- LuaLaTeX (preferred) or XeLaTeX
- Packages used: fontspec, unicode-math, geometry, microtype, setspace, parskip, enumitem, amssymb, tabularx, booktabs, longtable, graphicx, adjustbox, listings, mdframed, amsmath, tocloft, hyperref, url, xcolor, titlesec, fancyhdr, lastpage

## Cross-Platform Support

crowdoc runs on macOS, Linux, and Windows. The binary auto-detects `lualatex`/`xelatex` via `exec.LookPath()` which searches PATH on all platforms. Font fallback chains (`\IfFontExistsTF`) ensure documents render with system defaults (Latin Modern) when premium fonts are unavailable.

LaTeX distributions by platform:
- **macOS**: MacTeX (`brew install --cask mactex-no-gui`)
- **Linux**: TeX Live (`apt install texlive-full` / `dnf install texlive-scheme-full` / `pacman -S texlive-most`)
- **Windows**: MiKTeX (https://miktex.org) or TeX Live (https://tug.org/texlive)

## Testing

Convert the example files and verify PDF output:
```bash
go build -o crowdoc . && ./crowdoc --batch examples/
```
