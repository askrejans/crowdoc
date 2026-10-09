// Command crowdoc converts documents into beautifully typeset PDF.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/askrejans/crowdoc/v2"
	"github.com/askrejans/crowdoc/v2/internal/transform/metamap"
)

type cli struct {
	input, output string
	style         string
	template      string
	batch         bool
	batchDir      string
	batchOut      string
	watch         bool
	jobs          int
	meta          map[string]any
	bibs          []string
	pdfStandards  []string
	fontDirs      []string
	fetchImages   bool
	unsafeRaw     bool
	deterministic bool
	workDir       string
	emitSource    bool
	markdown      bool
	sourceDir     string
	timeout       time.Duration
	quiet         bool
	verbose       bool
	enginePath    string
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "crowdoc:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "engine":
			return runEngine(ctx, args[1:], stdout)
		case "fonts":
			return runFonts(ctx, args[1:], stdout, stderr)
		case "style", "styles":
			return runStyle(args[1:], stdout)
		}
	}
	c, info, err := parseArgs(args)
	if err != nil {
		return err
	}
	switch info {
	case "help":
		printUsage(stdout)
		return nil
	case "version":
		fmt.Fprintf(stdout, "crowdoc %s (Typst %s)\n", crowdoc.Version, crowdoc.TypstVersion)
		return nil
	case "styles":
		printStyles(stdout)
		return nil
	case "formats":
		printFormats(stdout)
		return nil
	case "citation-styles":
		printCitationStyles(stdout)
		return nil
	case "fonts":
		printPairings(stdout)
		return nil
	case "languages":
		printLanguages(stdout)
		return nil
	case "colors":
		printColors(stdout)
		return nil
	}
	opts, err := c.options(stderr)
	if err != nil {
		return err
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	switch {
	case c.batch:
		return runBatch(ctx, c, opts, stdout, stderr)
	case c.input == "":
		printUsage(stdout)
		return nil
	case c.watch:
		return runWatch(ctx, c, opts, stdout, stderr)
	}
	return convertOne(ctx, c, c.input, c.output, opts, stdout, stderr)
}

func (c *cli) options(stderr io.Writer) (crowdoc.Options, error) {
	opts := crowdoc.Options{
		Style:              c.style,
		TemplatePath:       c.template,
		Meta:               c.meta,
		Bibliography:       c.bibs,
		PDFStandards:       c.pdfStandards,
		FontDirs:           c.fontDirs,
		AllowRemoteImages:  c.fetchImages,
		UnsafeRaw:          c.unsafeRaw,
		DeterministicFonts: c.deterministic,
		WorkDir:            c.workDir,
	}
	if c.verbose {
		opts.Logger = slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	if c.enginePath != "" {
		opts.Engine = crowdoc.NewTypstEngine(c.enginePath, crowdoc.FontDirs(c.fontDirs), c.deterministic)
	}
	return opts, nil
}

func convertOne(ctx context.Context, c *cli, in, out string, opts crowdoc.Options, stdout, stderr io.Writer) error {
	if c.emitSource {
		res, err := crowdoc.Render(ctx, crowdoc.Source{Path: in}, opts)
		if err != nil {
			return err
		}
		dir := c.sourceDir
		if dir == "" {
			dir = strings.TrimSuffix(in, extOf(in)) + "-typst"
		}
		if err := crowdoc.WriteBundle(res.Bundle, dir); err != nil {
			return err
		}
		printWarnings(stderr, res.Warnings, c.quiet)
		if !c.quiet {
			fmt.Fprintf(stdout, "  %s/ (Typst project)\n", dir)
		}
		return nil
	}
	if c.markdown || strings.EqualFold(extOf(out), ".md") {
		res, err := crowdoc.MarkdownFile(ctx, in, out, opts, crowdoc.MarkdownOptions{})
		if res != nil {
			printWarnings(stderr, res.Warnings, c.quiet)
		}
		if err != nil {
			return err
		}
		if out == "" {
			out = crowdoc.DefaultMarkdownPath(in)
		}
		if !c.quiet {
			fmt.Fprintf(stdout, "  %s (Markdown, %d images)\n", out, len(res.Files))
		}
		return nil
	}
	res, err := crowdoc.ConvertFile(ctx, in, out, opts)
	if res != nil {
		printWarnings(stderr, res.Warnings, c.quiet)
	}
	if err != nil {
		var nf interface{ Unwrap() error }
		if errors.As(err, &nf) && errors.Is(err, crowdoc.ErrEngineNotFound) {
			return fmt.Errorf("%w\nRun `crowdoc engine install` to download the Typst engine (one-time, about 15 MB)", err)
		}
		return err
	}
	if out == "" {
		out = crowdoc.DefaultOutputPath(in)
	}
	if !c.quiet {
		fmt.Fprintf(stdout, "  %s (%s, %d KB, %s)\n", out, res.Style, (len(res.PDF)+1023)/1024, (res.ParseTime + res.RenderTime + res.CompileTime).Round(time.Millisecond))
	}
	return nil
}

func extOf(p string) string {
	for i := len(p) - 1; i >= 0 && p[i] != '/' && p[i] != '\\'; i-- {
		if p[i] == '.' {
			return p[i:]
		}
	}
	return ""
}

func printWarnings(w io.Writer, warns []string, quiet bool) {
	if quiet {
		return
	}
	for _, x := range warns {
		fmt.Fprintln(w, "  warning:", x)
	}
}

// parseArgs keeps the v1 command-line contract (flags may appear before or
// after the positional input and output paths).
func parseArgs(args []string) (*cli, string, error) {
	c := &cli{meta: map[string]any{}}
	need := func(i *int, flag string) (string, error) {
		*i++
		if *i >= len(args) {
			return "", fmt.Errorf("%s requires a value", flag)
		}
		return args[*i], nil
	}
	set := func(key string, v any) { c.meta[key] = v }
	for i := 0; i < len(args); i++ {
		a := args[i]
		// Support --flag=value.
		if strings.HasPrefix(a, "--") && strings.Contains(a, "=") {
			k, v, _ := strings.Cut(a, "=")
			args = append(args[:i], append([]string{k, v}, args[i+1:]...)...)
			a = k
		}
		var v string
		var err error
		switch a {
		case "-h", "--help":
			return c, "help", nil
		case "-v", "--version":
			return c, "version", nil
		case "--list-styles":
			return c, "styles", nil
		case "--list-formats":
			return c, "formats", nil
		case "--list-citation-styles", "--list-csl":
			return c, "citation-styles", nil
		case "--list-fonts", "--list-pairings":
			return c, "fonts", nil
		case "--list-languages":
			return c, "languages", nil
		case "--list-colors", "--list-colours", "--list-palettes":
			return c, "colors", nil
		case "-s", "--style":
			c.style, err = need(&i, a)
		case "-t", "--template":
			c.template, err = need(&i, a)
		case "-o", "--output":
			c.output, err = need(&i, a)
		case "-b", "--batch":
			c.batch = true
			c.batchDir, err = need(&i, a)
			if err == nil && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				c.batchOut = args[i]
			}
		case "-w", "--watch":
			c.watch = true
		case "-j", "--jobs":
			if v, err = need(&i, a); err == nil {
				c.jobs, err = strconv.Atoi(v)
			}
		case "--toc":
			set("toc", true)
		case "--no-toc":
			set("toc", false)
		case "--lof":
			set("lof", true)
		case "--lot":
			set("lot", true)
		case "--no-title-page":
			set("title-page", false)
		case "--title-page":
			set("title-page", true)
		case "--signatures":
			set("signatures", true)
		case "--no-signatures":
			set("signatures", false)
		case "--number-sections":
			set("number-sections", true)
		case "--no-number-sections":
			set("number-sections", false)
		case "--landscape":
			set("landscape", true)
		case "--title", "--subtitle", "--author", "--date", "--status", "--classification", "--summary",
			"--paper", "--accent", "--font-size", "--columns", "--line-spacing", "--organization",
			"--version-label", "--margin", "--header-left", "--header-right", "--footer-left", "--footer-right", "--logo":
			if v, err = need(&i, a); err == nil {
				key := strings.TrimPrefix(a, "--")
				if key == "version-label" {
					key = "version"
				}
				if key == "author" {
					set(key, splitAuthors(v))
				} else {
					set(key, v)
				}
			}
		case "--language", "--lang":
			if v, err = need(&i, a); err == nil {
				set("lang", v)
			}
		case "--fonts", "--typeface":
			if v, err = need(&i, a); err == nil {
				set("fonts-preset", v)
			}
		case "--font", "--main-font":
			if v, err = need(&i, a); err == nil {
				set("mainfont", v)
			}
		case "--sans-font", "--heading-font":
			if v, err = need(&i, a); err == nil {
				set("sansfont", v)
			}
		case "--mono-font", "--code-font":
			if v, err = need(&i, a); err == nil {
				set("monofont", v)
			}
		case "--colors", "--colours", "--palette", "--color-scheme":
			if v, err = need(&i, a); err == nil {
				set("color-scheme", v)
			}
		case "--color", "--colour":
			if v, err = need(&i, a); err == nil {
				role, val, ok := strings.Cut(v, "=")
				if !ok {
					err = fmt.Errorf("--color expects role=#hex, e.g. accent=#6A00FF")
				} else {
					colors, _ := c.meta["colors"].(map[string]any)
					if colors == nil {
						colors = map[string]any{}
					}
					colors[strings.TrimSpace(role)] = strings.TrimSpace(val)
					c.meta["colors"] = colors
				}
			}
		case "--math-font":
			if v, err = need(&i, a); err == nil {
				set("mathfont", v)
			}
		case "--meta", "-M":
			if v, err = need(&i, a); err == nil {
				var key string
				var val any
				if key, val, err = metamap.ParseOverride(v); err == nil {
					set(key, val)
				}
			}
		case "--bib", "--bibliography":
			if v, err = need(&i, a); err == nil {
				c.bibs = append(c.bibs, v)
			}
		case "--csl", "--citation-style":
			if v, err = need(&i, a); err == nil {
				set("csl", v)
			}
		case "--pdfa":
			c.pdfStandards = append(c.pdfStandards, "a-2b")
		case "--pdf-standard":
			if v, err = need(&i, a); err == nil {
				c.pdfStandards = append(c.pdfStandards, strings.Split(v, ",")...)
			}
		case "--accessible", "--pdfua":
			c.pdfStandards = append(c.pdfStandards, "ua-1")
		case "--font-dir":
			if v, err = need(&i, a); err == nil {
				c.fontDirs = append(c.fontDirs, v)
			}
		case "--fetch-images":
			c.fetchImages = true
		case "--unsafe-raw":
			c.unsafeRaw = true
		case "--deterministic-fonts", "--ignore-system-fonts":
			c.deterministic = true
		case "--workdir", "--keep-workdir":
			c.workDir, err = need(&i, a)
		case "--markdown", "--md":
			c.markdown = true
		case "--typst", "--source", "--tex":
			c.emitSource = true
		case "--source-dir":
			c.emitSource = true
			c.sourceDir, err = need(&i, a)
		case "--timeout":
			if v, err = need(&i, a); err == nil {
				c.timeout, err = time.ParseDuration(v)
			}
		case "--engine-path", "--typst-path":
			c.enginePath, err = need(&i, a)
		case "-q", "--quiet":
			c.quiet = true
		case "--verbose":
			c.verbose = true
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				return nil, "", fmt.Errorf("unknown flag: %s (see crowdoc --help)", a)
			}
			switch {
			case c.input == "":
				c.input = a
			case c.output == "":
				c.output = a
			default:
				return nil, "", fmt.Errorf("unexpected argument: %s", a)
			}
		}
		if err != nil {
			return nil, "", err
		}
	}
	if fs, ok := c.meta["font-size"]; ok {
		if _, ok := metamap.FontSize(fs); !ok {
			return nil, "", fmt.Errorf("--font-size must be between 8 and 14")
		}
	}
	if p, ok := c.meta["fonts-preset"]; ok {
		delete(c.meta, "fonts-preset")
		c.meta["fonts"] = p
	}
	return c, "", nil
}

func splitAuthors(s string) []any {
	var out []any
	for _, p := range strings.Split(s, ";") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
