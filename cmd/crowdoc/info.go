package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/askrejans/crowdoc/v2"
)

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `crowdoc %s — beautifully typeset PDFs from any document
https://github.com/askrejans/crowdoc

Usage:
  crowdoc [options] <input> [output.pdf]     Convert one document
  crowdoc --batch <dir> [outdir] [options]   Convert every document in a folder (parallel)
  crowdoc --watch <input> [options]          Rebuild on every save
  crowdoc engine install|status              Install or check the Typst engine (~15 MB)
  crowdoc fonts install [sets…]|list         Install curated open-source font sets
  crowdoc style export <name> [dir]          Copy a style's Typst source for customising

Inputs: Markdown, Word (.docx), OpenDocument (.odt, .fodt), RTF, HTML, EPUB,
Jupyter (.ipynb), CSV/TSV, Excel (.xlsx), OpenDocument spreadsheets (.ods), text,
and PDFs with a text layer (re-typeset to report.typeset.pdf).

Style and layout:
  -s, --style <name>         Style (see --list-styles)
  -t, --template <file.typ>  Custom Typst template defining template(meta, body)
      --fonts <pairing>      Typeface pairing (see --list-fonts)
      --font / --sans-font / --mono-font / --math-font <family>
      --font-size <pt>       Base size, 8–14
      --paper <a4|letter|a5|legal|…>    --landscape    --columns <1|2>
      --line-spacing <single|onehalf|double|1.25>
      --margin <2.5cm>       --logo <file>
      --colors <scheme>      Colour scheme (see --list-colors)
      --color role=#hex      Override one colour role (repeatable); --accent <#hex>
      --toc / --no-toc       --lof  --lot        --number-sections / --no-number-sections
      --title-page / --no-title-page             --signatures / --no-signatures
      --header-left/--header-right/--footer-left/--footer-right <text>

Metadata:
      --title --subtitle --author "A; B" --organization --date --status
      --classification --summary --language <code> --version-label <text>
  -M, --meta key=value       Any frontmatter key (repeatable)

Citations:
      --bib <file>           .bib, CSL .json/.yaml, .ris, .nbib (repeatable)
      --csl <style>          apa, chicago, ieee, harvard, vancouver, mla (see --list-citation-styles)

Output:
  -o, --output <file.pdf>    Output path (default: input name with .pdf)
      --pdfa                 Archival PDF/A-2b      --pdf-standard <a-2b,ua-1,…>
      --accessible           Tagged, accessible PDF/UA-1
      --typst                Write the Typst project instead of a PDF (--source-dir <dir>)
      --markdown             Write clean Markdown (+ images/) instead of a PDF; also when -o ends in .md
  -j, --jobs <n>             Parallel conversions in batch mode (default: CPUs, max 4)

Engine and resources:
      --font-dir <dir>       Extra font directory (repeatable)
      --deterministic-fonts  Ignore system fonts (identical output everywhere)
      --fetch-images         Allow downloading http(s) images
      --unsafe-raw           Pass raw Typst blocks through (trusted input only)
      --engine-path <typst>  Typst executable (default: $CROWDOC_TYPST, installed, $PATH)
      --workdir <dir>        Keep the compilation directory
      --timeout <30s>        Abort after a duration
  -q, --quiet   --verbose   -v, --version   -h, --help
      --list-styles --list-fonts --list-colors --list-formats --list-citation-styles --list-languages
`, crowdoc.Version)
}

func printStyles(w io.Writer) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	cat := ""
	for _, s := range crowdoc.Styles() {
		if s.Category != cat {
			cat = s.Category
			fmt.Fprintf(tw, "\n%s\n", strings.ToUpper(cat))
		}
		fmt.Fprintf(tw, "  %s\t%s\n", s.Name, s.Description)
	}
	tw.Flush()
	fmt.Fprintln(w, "\nUse: crowdoc --style <name> input.md   or   style: <name> in frontmatter")
}

func printFormats(w io.Writer) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	for _, f := range crowdoc.Formats() {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", f.Name, strings.Join(f.Extensions, " "), f.Description)
	}
	tw.Flush()
}

func printCitationStyles(w io.Writer) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	for _, s := range crowdoc.CitationStyles() {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", s.Name, s.Kind, s.Title)
	}
	tw.Flush()
}

func printPairings(w io.Writer) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	for _, p := range crowdoc.FontPairings() {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", p.Name, p.Title, p.Description)
	}
	tw.Flush()
	fmt.Fprintln(w, "\nUse: --fonts <name> or `fonts: <name>` in frontmatter. Install the faces with `crowdoc fonts install`.")
}

func printColors(w io.Writer) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	for _, p := range crowdoc.ColorSchemes() {
		fmt.Fprintf(tw, "  %s\t%s %s\t%s\n", p.Name, p.Colors["accent"], p.Colors["secondary"], p.Description)
	}
	tw.Flush()
	fmt.Fprintln(w, "\nUse: --colors <name>, or `colors: <name>` in frontmatter. Override single roles with")
	fmt.Fprintln(w, "--color accent=#6A00FF or `colors: {accent: \"#6A00FF\", page: \"#FBF8F1\"}`.")
	fmt.Fprintln(w, "Roles: accent, secondary, ink, heading, muted, link, rule, code-bg, table-head, stripe, quote, page")
}

func printLanguages(w io.Writer) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	for _, l := range crowdoc.Languages() {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", l.Tag, l.Name, l.NativeName)
	}
	tw.Flush()
}

func runEngine(ctx context.Context, args []string, stdout io.Writer) error {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "install":
		fmt.Fprintf(stdout, "Installing Typst %s…\n", crowdoc.TypstVersion)
		path, err := crowdoc.InstallTypst(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Installed:", path)
		return nil
	case "status":
		path, err := crowdoc.FindTypst()
		if err != nil {
			fmt.Fprintln(stdout, "Typst: not found —", err)
			return nil
		}
		fmt.Fprintf(stdout, "Typst: %s (crowdoc targets %s)\n", path, crowdoc.TypstVersion)
		return nil
	}
	return fmt.Errorf("unknown engine command %q (install, status)", sub)
}

func runFonts(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	switch sub {
	case "list":
		printPairings(stdout)
		return nil
	case "install":
		sets := args
		if len(sets) == 0 {
			sets = []string{"core"}
		}
		return crowdoc.InstallFonts(ctx, sets, func(family string, done, total int64) {
			if done == total {
				fmt.Fprintf(stdout, "  ✓ %s\n", family)
			}
		})
	}
	return fmt.Errorf("unknown fonts command %q (install, list)", sub)
}

func runStyle(args []string, stdout io.Writer) error {
	if len(args) < 2 || args[0] != "export" {
		return fmt.Errorf("usage: crowdoc style export <name> [dir]")
	}
	dir := "."
	if len(args) > 2 {
		dir = args[2]
	}
	paths, err := crowdoc.ExportStyle(args[1], dir)
	if err != nil {
		return err
	}
	for _, p := range paths {
		fmt.Fprintln(stdout, "  "+p)
	}
	fmt.Fprintln(stdout, "Edit the copy and use it with: crowdoc --template", paths[0], "input.md")
	return nil
}
