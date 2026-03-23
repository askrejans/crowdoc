package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const version = "1.0.0"

// CLI flags parsed from os.Args
type options struct {
	inputPath   string
	outputPath  string
	style       string
	batch       bool
	batchDir    string
	batchOutDir string
	watch       bool
	listStyles  bool
	showVersion bool
	toc         *bool // nil = auto, true = force on, false = force off
	noTitlePage bool
	noSignatures bool
	fontSize    int
	outputTeX   bool // --tex: also output the intermediate .tex file
}

func main() {
	opts := parseArgs()

	if opts.showVersion {
		fmt.Printf("crowdoc v%s\n", version)
		return
	}

	if opts.listStyles {
		printStyleList()
		return
	}

	if opts.batch {
		runBatch(opts)
		return
	}

	if opts.inputPath == "" {
		printUsage()
		os.Exit(0)
	}

	if opts.watch {
		watchAndConvert(opts)
		return
	}

	if err := convertFile(opts); err != nil {
		fatal("%v", err)
	}
}

func parseArgs() options {
	opts := options{}
	args := os.Args[1:]

	if len(args) == 0 {
		return opts
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "--version" || arg == "-v":
			opts.showVersion = true
			return opts
		case arg == "--list-styles":
			opts.listStyles = true
			return opts
		case arg == "--style" || arg == "-s":
			i++
			if i >= len(args) {
				fatal("--style requires an argument")
			}
			opts.style = args[i]
		case arg == "--batch" || arg == "-b":
			opts.batch = true
			i++
			if i >= len(args) {
				fatal("--batch requires a directory argument")
			}
			opts.batchDir = args[i]
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				opts.batchOutDir = args[i]
			}
		case arg == "--watch" || arg == "-w":
			opts.watch = true
		case arg == "--toc":
			t := true
			opts.toc = &t
		case arg == "--no-toc":
			f := false
			opts.toc = &f
		case arg == "--no-title-page":
			opts.noTitlePage = true
		case arg == "--no-signatures":
			opts.noSignatures = true
		case arg == "--tex":
			opts.outputTeX = true
		case arg == "--font-size":
			i++
			if i >= len(args) {
				fatal("--font-size requires an argument (10, 11, or 12)")
			}
			switch args[i] {
			case "10":
				opts.fontSize = 10
			case "11":
				opts.fontSize = 11
			case "12":
				opts.fontSize = 12
			default:
				fatal("--font-size must be 10, 11, or 12")
			}
		case strings.HasPrefix(arg, "-"):
			fatal("unknown flag: %s", arg)
		default:
			if opts.inputPath == "" {
				opts.inputPath = arg
			} else if opts.outputPath == "" {
				opts.outputPath = arg
			} else {
				fatal("unexpected argument: %s", arg)
			}
		}
		i++
	}

	return opts
}

func convertFile(opts options) error {
	mdBytes, err := os.ReadFile(opts.inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	inputDir, _ := filepath.Abs(filepath.Dir(opts.inputPath))
	doc := parseMarkdown(string(mdBytes), inputDir)

	// Apply CLI overrides
	if opts.style != "" {
		doc.Style = opts.style
	}
	if opts.toc != nil {
		doc.TOC = opts.toc
	}
	if opts.noTitlePage {
		doc.NoTitlePage = true
	}
	if opts.noSignatures {
		doc.HasSignatures = false
	}
	if opts.fontSize > 0 {
		doc.FontSize = opts.fontSize
	}

	outputPath := opts.outputPath
	if outputPath == "" {
		ext := filepath.Ext(opts.inputPath)
		outputPath = strings.TrimSuffix(opts.inputPath, ext) + ".pdf"
	}

	if err := renderAndCompile(doc, outputPath); err != nil {
		return err
	}

	// Optionally output intermediate .tex file
	if opts.outputTeX {
		texOut := strings.TrimSuffix(outputPath, ".pdf") + ".tex"
		latex := renderLaTeX(doc)
		if err := os.WriteFile(texOut, []byte(latex), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write .tex file: %v\n", err)
		} else {
			fmt.Printf("  %s (tex)\n", texOut)
		}
	}

	fmt.Printf("  %s\n", outputPath)
	return nil
}

func runBatch(opts options) {
	inputDir := opts.batchDir
	outputDir := opts.batchOutDir
	if outputDir == "" {
		outputDir = filepath.Join(inputDir, "pdf")
	}

	var files []string
	filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			base := strings.ToLower(filepath.Base(path))
			if base == "readme.md" || base == "claude.md" {
				return nil
			}
			files = append(files, path)
		}
		return nil
	})

	if len(files) == 0 {
		fmt.Println("No .md files found in", inputDir)
		return
	}

	fmt.Printf("Converting %d files...\n", len(files))
	success := 0
	for idx, f := range files {
		rel, _ := filepath.Rel(inputDir, f)
		outPath := filepath.Join(outputDir, strings.TrimSuffix(rel, ".md")+".pdf")
		fmt.Printf("  [%d/%d] %s", idx+1, len(files), rel)

		batchOpts := opts
		batchOpts.inputPath = f
		batchOpts.outputPath = outPath
		if err := convertFile(batchOpts); err != nil {
			fmt.Fprintf(os.Stderr, " ERROR: %v\n", err)
			continue
		}
		success++
	}
	fmt.Printf("\nDone: %d/%d PDFs generated in %s\n", success, len(files), outputDir)
}

func printStyleList() {
	fmt.Printf("crowdoc v%s — available styles:\n\n", version)
	fmt.Println("  legal      Gold accents, signature blocks, formal headers. For contracts and agreements.")
	fmt.Println("  technical  Wide margins, code-friendly, clean sans-serif. For technical documentation.")
	fmt.Println("  report     Professional cover page, business typography. For reports and proposals.")
	fmt.Println("  minimal    Clean and simple, no frills. Great for general documents.")
	fmt.Println("  letter     Formal business letter format with sender/recipient blocks.")
	fmt.Println()
	fmt.Println("Usage: crowdoc --style <name> input.md")
	fmt.Println("Or set `style: <name>` in frontmatter.")
}

func printUsage() {
	fmt.Printf(`crowdoc v%s — Universal Markdown-to-PDF converter
https://github.com/askrejans/crowdoc

Usage:
  crowdoc <input.md> [output.pdf]          Convert a single file
  crowdoc --batch <dir/> [outdir/]         Batch convert directory
  crowdoc --watch <input.md>               Watch and regenerate on change
  crowdoc --style <style> <input.md>       Convert with style override
  crowdoc --list-styles                    Show available styles
  crowdoc --version                        Show version

Options:
  -s, --style <name>     Style: legal, technical, report, minimal, letter
  -b, --batch <dir>      Batch convert all .md files in directory
  -w, --watch            Watch file for changes and regenerate
      --toc              Force table of contents
      --no-toc           Disable table of contents
      --no-title-page    Skip the title page
      --no-signatures    Skip signature blocks
      --tex              Also output intermediate .tex file
      --font-size <n>    Base font size: 10, 11, or 12 (default: 11)
  -v, --version          Show version
  -h, --help             Show this help

Frontmatter (YAML between --- markers):
  title, subtitle, date, version, status, type, style, summary, author,
  language, classification, signatures, toc, logo, font-size,
  header-left, header-right, footer-left, footer-right,
  margin-top, margin-bottom, margin-left, margin-right

Example:
  ---
  title: Project Specification
  style: technical
  toc: true
  ---

Requirements:
  LaTeX (LuaLaTeX): brew install --cask mactex-no-gui
`, version)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "crowdoc: "+format+"\n", args...)
	os.Exit(1)
}
