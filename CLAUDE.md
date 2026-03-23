# crowdoc -- Development Guide

## Project Structure

```
crowdoc/
  main.go        CLI entry point, argument parsing, dispatch
  parser.go      Markdown parsing: frontmatter extraction, section splitting
  converter.go   Markdown-to-LaTeX conversion: inline formatting, lists, tables, code, images
  renderer.go    LaTeX template execution and PDF compilation (lualatex/xelatex)
  styles.go      All LaTeX template strings for each style (legal, technical, report, minimal, letter)
  watcher.go     File watch mode (polling-based, no external deps)
  go.mod         Go module definition
  examples/      Sample markdown files demonstrating each style
```

## Build and Run

```bash
go build -o crowdoc .
./crowdoc examples/technical-doc.md
./crowdoc --style legal examples/legal-agreement.md
```

## Architecture

### Flow
1. `main.go` parses CLI arguments into an `options` struct
2. `parser.go` reads the markdown file, extracts YAML frontmatter into a `Document` struct, splits body into `Section` structs
3. `renderer.go` selects the style template from `styles.go`, executes it with the `Document` data using Go `text/template`
4. `converter.go` provides the `mdToLaTeX` function called from templates -- converts markdown body text to LaTeX (lists, tables, code blocks, images, inline formatting)
5. `renderer.go` writes the `.tex` file to a temp directory and runs `lualatex` twice (for TOC/references), falling back to `xelatex`

### Key Design Decisions
- **Zero Go dependencies** -- only stdlib. Watch mode uses polling instead of fsnotify.
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

## LaTeX Requirements
- LuaLaTeX (preferred) or XeLaTeX
- Packages used: fontspec, unicode-math, geometry, microtype, setspace, parskip, enumitem, amssymb, tabularx, booktabs, longtable, graphicx, adjustbox, listings, mdframed, amsmath, tocloft, hyperref, url, xcolor, titlesec, fancyhdr, lastpage

## Testing

Convert the example files and verify PDF output:
```bash
go build -o crowdoc . && ./crowdoc examples/technical-doc.md
go build -o crowdoc . && ./crowdoc examples/legal-agreement.md
go build -o crowdoc . && ./crowdoc examples/business-report.md
```
