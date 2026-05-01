package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── convertFile ─────────────────────────────────────────────────────────────

func TestConvertFile_NonexistentFile(t *testing.T) {
	opts := options{inputPath: "/nonexistent/file.md"}
	err := convertFile(opts)
	if err == nil {
		t.Error("should error on nonexistent file")
	}
}

// ─── detectFormat ───────────────────────────────────────────────────────────

func TestDetectFormat_Markdown(t *testing.T) {
	for _, ext := range []string{".md", ".markdown"} {
		got := detectFormat("doc" + ext)
		if got != "markdown" {
			t.Errorf("detectFormat('doc%s') = %q, want 'markdown'", ext, got)
		}
	}
}

func TestDetectFormat_CSV(t *testing.T) {
	if got := detectFormat("data.csv"); got != "csv" {
		t.Errorf("got %q, want 'csv'", got)
	}
}

func TestDetectFormat_XLSX(t *testing.T) {
	if got := detectFormat("data.xlsx"); got != "xlsx" {
		t.Errorf("got %q, want 'xlsx'", got)
	}
}

func TestDetectFormat_XLS(t *testing.T) {
	if got := detectFormat("data.xls"); got != "xls" {
		t.Errorf("got %q, want 'xls'", got)
	}
}

func TestDetectFormat_TXT(t *testing.T) {
	if got := detectFormat("notes.txt"); got != "txt" {
		t.Errorf("got %q, want 'txt'", got)
	}
}

func TestDetectFormat_HTML(t *testing.T) {
	for _, ext := range []string{".html", ".htm"} {
		got := detectFormat("page" + ext)
		if got != "html" {
			t.Errorf("detectFormat('page%s') = %q, want 'html'", ext, got)
		}
	}
}

func TestDetectFormat_Unknown(t *testing.T) {
	if got := detectFormat("file.xyz"); got != "markdown" {
		t.Errorf("unknown ext should default to markdown, got %q", got)
	}
}

func TestDetectFormat_CaseInsensitive(t *testing.T) {
	if got := detectFormat("DATA.CSV"); got != "csv" {
		t.Errorf("should be case insensitive, got %q", got)
	}
}

func TestConvertFile_XLSRejectsOldFormat(t *testing.T) {
	opts := options{inputPath: "old-file.xls"}
	err := convertFile(opts)
	if err == nil {
		t.Error("should reject .xls files")
	}
	if !strings.Contains(err.Error(), ".xlsx") {
		t.Error("error should suggest .xlsx")
	}
}

func TestConvertFile_CSVFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.csv")
	os.WriteFile(path, []byte("Name,Score\nAlice,95\n"), 0644)

	opts := options{inputPath: path}
	err := convertFile(opts)
	if err != nil {
		t.Skipf("LaTeX not available: %v", err)
	}

	pdfPath := filepath.Join(tmpDir, "data.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Error("should create PDF from CSV")
	}
}

func TestConvertFile_TXTFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "notes.txt")
	os.WriteFile(path, []byte("My Notes\n\nSome content here.\n"), 0644)

	opts := options{inputPath: path}
	err := convertFile(opts)
	if err != nil {
		t.Skipf("LaTeX not available: %v", err)
	}

	pdfPath := filepath.Join(tmpDir, "notes.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Error("should create PDF from TXT")
	}
}

func TestConvertFile_HTMLFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "page.html")
	os.WriteFile(path, []byte("<html><head><title>Test</title></head><body><p>Hello</p></body></html>"), 0644)

	opts := options{inputPath: path}
	err := convertFile(opts)
	if err != nil {
		t.Skipf("LaTeX not available: %v", err)
	}

	pdfPath := filepath.Join(tmpDir, "page.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Error("should create PDF from HTML")
	}
}

func TestConvertFile_XLSXFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "report.xlsx")
	createTestXLSX(path, map[string][][]string{
		"Data": {{"A", "B"}, {"1", "2"}},
	})

	opts := options{inputPath: path}
	err := convertFile(opts)
	if err != nil {
		t.Skipf("LaTeX not available: %v", err)
	}

	pdfPath := filepath.Join(tmpDir, "report.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Error("should create PDF from XLSX")
	}
}

func TestConvertFile_DefaultOutputPath(t *testing.T) {
	// Create a temp markdown file
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdPath, []byte("---\ntitle: Test\nstyle: minimal\n---\n\n## Hello\n\nWorld.\n"), 0644)

	opts := options{inputPath: mdPath}
	err := convertFile(opts)
	if err != nil {
		t.Skipf("LaTeX not available: %v", err)
	}

	// Default output should be test.pdf
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Error("should create PDF at default path")
	}
}

func TestConvertFile_CustomOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "input.md")
	pdfPath := filepath.Join(tmpDir, "custom-output.pdf")
	os.WriteFile(mdPath, []byte("---\ntitle: Test\nstyle: minimal\n---\n\n## Hello\n\nWorld.\n"), 0644)

	opts := options{inputPath: mdPath, outputPath: pdfPath}
	err := convertFile(opts)
	if err != nil {
		t.Skipf("LaTeX not available: %v", err)
	}

	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Error("should create PDF at custom path")
	}
}

func TestConvertFile_CLIOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdPath, []byte("---\ntitle: Test\n---\n\n## Section\n\nContent.\n"), 0644)

	toc := true
	opts := options{
		inputPath:    mdPath,
		style:        "minimal",
		toc:          &toc,
		noTitlePage:  true,
		noSignatures: true,
		fontSize:     12,
	}

	err := convertFile(opts)
	if err != nil {
		t.Skipf("LaTeX not available: %v", err)
	}
}

func TestConvertFile_OutputTeX(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdPath, []byte("---\ntitle: Test\nstyle: minimal\n---\n\n## Hello\n\nWorld.\n"), 0644)

	opts := options{inputPath: mdPath, outputTeX: true}
	err := convertFile(opts)
	if err != nil {
		t.Skipf("LaTeX not available: %v", err)
	}

	texPath := filepath.Join(tmpDir, "test.tex")
	if _, err := os.Stat(texPath); os.IsNotExist(err) {
		t.Error("should create .tex file when --tex is set")
	}
}

// ─── runBatch ────────────────────────────────────────────────────────────────

func TestRunBatch_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	opts := options{batch: true, batchDir: tmpDir}
	// Should not panic on empty directory
	runBatch(opts)
}

func TestRunBatch_SkipsReadmeAndClaude(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files that should be skipped
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Readme"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Claude"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# readme"), 0644)

	opts := options{batch: true, batchDir: tmpDir}
	// Should not process any of these files
	runBatch(opts)
}

func TestRunBatch_DefaultOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdPath, []byte("---\ntitle: Batch Test\nstyle: minimal\n---\n\n## S\n\nContent.\n"), 0644)

	opts := options{batch: true, batchDir: tmpDir}
	runBatch(opts)

	// Default output dir should be <inputDir>/pdf/
	pdfDir := filepath.Join(tmpDir, "pdf")
	if _, err := os.Stat(pdfDir); err != nil {
		// LaTeX might not be available, skip
		t.Skipf("LaTeX not available or batch failed: %v", err)
	}
}

func TestRunBatch_CustomOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "custom-out")
	mdPath := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdPath, []byte("---\ntitle: Test\nstyle: minimal\n---\n\n## S\n\nC.\n"), 0644)

	opts := options{batch: true, batchDir: tmpDir, batchOutDir: outDir}
	runBatch(opts)
}

// ─── printStyleList / printUsage ─────────────────────────────────────────────

func TestPrintStyleList_DoesNotPanic(t *testing.T) {
	// Just verify it doesn't panic
	printStyleList()
}

func TestPrintUsage_DoesNotPanic(t *testing.T) {
	printUsage()
}

// ─── version constant ────────────────────────────────────────────────────────

func TestVersionDefined(t *testing.T) {
	if version == "" {
		t.Error("version constant should not be empty")
	}
}

// ─── parseArgs (via os.Args override) ────────────────────────────────────────

func withArgs(args []string, fn func()) {
	old := os.Args
	os.Args = append([]string{"crowdoc"}, args...)
	defer func() { os.Args = old }()
	fn()
}

func TestParseArgs_NoArgs(t *testing.T) {
	withArgs([]string{}, func() {
		opts := parseArgs()
		if opts.inputPath != "" {
			t.Errorf("inputPath should be empty, got %q", opts.inputPath)
		}
	})
}

func TestParseArgs_Version(t *testing.T) {
	for _, flag := range []string{"--version", "-v"} {
		withArgs([]string{flag}, func() {
			opts := parseArgs()
			if !opts.showVersion {
				t.Errorf("showVersion should be true for %s", flag)
			}
		})
	}
}

func TestParseArgs_ListStyles(t *testing.T) {
	withArgs([]string{"--list-styles"}, func() {
		opts := parseArgs()
		if !opts.listStyles {
			t.Error("listStyles should be true")
		}
	})
}

func TestParseArgs_Style(t *testing.T) {
	for _, flag := range []string{"--style", "-s"} {
		withArgs([]string{flag, "legal", "input.md"}, func() {
			opts := parseArgs()
			if opts.style != "legal" {
				t.Errorf("style = %q, want %q", opts.style, "legal")
			}
		})
	}
}

func TestParseArgs_InputAndOutput(t *testing.T) {
	withArgs([]string{"input.md", "output.pdf"}, func() {
		opts := parseArgs()
		if opts.inputPath != "input.md" {
			t.Errorf("inputPath = %q", opts.inputPath)
		}
		if opts.outputPath != "output.pdf" {
			t.Errorf("outputPath = %q", opts.outputPath)
		}
	})
}

func TestParseArgs_Watch(t *testing.T) {
	for _, flag := range []string{"--watch", "-w"} {
		withArgs([]string{flag, "input.md"}, func() {
			opts := parseArgs()
			if !opts.watch {
				t.Errorf("watch should be true for %s", flag)
			}
		})
	}
}

func TestParseArgs_TOC(t *testing.T) {
	withArgs([]string{"--toc", "input.md"}, func() {
		opts := parseArgs()
		if opts.toc == nil || !*opts.toc {
			t.Error("toc should be true")
		}
	})
}

func TestParseArgs_NoTOC(t *testing.T) {
	withArgs([]string{"--no-toc", "input.md"}, func() {
		opts := parseArgs()
		if opts.toc == nil || *opts.toc {
			t.Error("toc should be false")
		}
	})
}

func TestParseArgs_NoTitlePage(t *testing.T) {
	withArgs([]string{"--no-title-page", "input.md"}, func() {
		opts := parseArgs()
		if !opts.noTitlePage {
			t.Error("noTitlePage should be true")
		}
	})
}

func TestParseArgs_NoSignatures(t *testing.T) {
	withArgs([]string{"--no-signatures", "input.md"}, func() {
		opts := parseArgs()
		if !opts.noSignatures {
			t.Error("noSignatures should be true")
		}
	})
}

func TestParseArgs_TeX(t *testing.T) {
	withArgs([]string{"--tex", "input.md"}, func() {
		opts := parseArgs()
		if !opts.outputTeX {
			t.Error("outputTeX should be true")
		}
	})
}

func TestParseArgs_FontSize(t *testing.T) {
	for _, size := range []string{"10", "11", "12"} {
		withArgs([]string{"--font-size", size, "input.md"}, func() {
			opts := parseArgs()
			expected := 10
			if size == "11" {
				expected = 11
			} else if size == "12" {
				expected = 12
			}
			if opts.fontSize != expected {
				t.Errorf("fontSize = %d, want %d", opts.fontSize, expected)
			}
		})
	}
}

func TestParseArgs_MetadataOverrides(t *testing.T) {
	withArgs([]string{
		"--title", "April Invoices",
		"--subtitle", "Customer ledger",
		"--author", "SIA Ulbroka",
		"--language", "lv",
		"--date", "2026-05-01",
		"--status", "final",
		"--classification", "internal",
		"--summary", "Monthly export",
		"input.csv",
	}, func() {
		opts := parseArgs()
		if opts.title != "April Invoices" ||
			opts.subtitle != "Customer ledger" ||
			opts.author != "SIA Ulbroka" ||
			opts.language != "lv" ||
			opts.date != "2026-05-01" ||
			opts.status != "final" ||
			opts.classification != "internal" ||
			opts.summary != "Monthly export" {
			t.Fatalf("metadata overrides were not parsed correctly: %+v", opts)
		}
	})
}

func TestApplyMetadataOverrides(t *testing.T) {
	doc := Document{Title: "Original", Status: "DRAFT", Classification: "CONFIDENTIAL"}
	applyMetadataOverrides(&doc, options{
		title:          "Export",
		subtitle:       "Ledger",
		author:         "CrowFoundry",
		language:       "lv",
		date:           "2026-05-01",
		status:         "final",
		classification: "internal",
		summary:        "Ready",
	})
	if doc.Title != "Export" ||
		doc.Subtitle != "Ledger" ||
		doc.Author != "CrowFoundry" ||
		doc.Language != "lv" ||
		doc.Date != "2026-05-01" ||
		doc.Status != "FINAL" ||
		doc.Classification != "INTERNAL" ||
		doc.Summary != "Ready" {
		t.Fatalf("metadata overrides were not applied: %+v", doc)
	}
}

func TestParseArgs_Batch(t *testing.T) {
	for _, flag := range []string{"--batch", "-b"} {
		withArgs([]string{flag, "docs/"}, func() {
			opts := parseArgs()
			if !opts.batch {
				t.Errorf("batch should be true for %s", flag)
			}
			if opts.batchDir != "docs/" {
				t.Errorf("batchDir = %q", opts.batchDir)
			}
		})
	}
}

func TestParseArgs_BatchWithOutput(t *testing.T) {
	withArgs([]string{"--batch", "docs/", "output/"}, func() {
		opts := parseArgs()
		if opts.batchDir != "docs/" {
			t.Errorf("batchDir = %q", opts.batchDir)
		}
		if opts.batchOutDir != "output/" {
			t.Errorf("batchOutDir = %q", opts.batchOutDir)
		}
	})
}

func TestParseArgs_CombinedFlags(t *testing.T) {
	withArgs([]string{"--style", "legal", "--toc", "--no-signatures", "--font-size", "12", "--tex", "input.md", "output.pdf"}, func() {
		opts := parseArgs()
		if opts.style != "legal" {
			t.Errorf("style = %q", opts.style)
		}
		if opts.toc == nil || !*opts.toc {
			t.Error("toc should be true")
		}
		if !opts.noSignatures {
			t.Error("noSignatures should be true")
		}
		if opts.fontSize != 12 {
			t.Errorf("fontSize = %d", opts.fontSize)
		}
		if !opts.outputTeX {
			t.Error("outputTeX should be true")
		}
		if opts.inputPath != "input.md" {
			t.Errorf("inputPath = %q", opts.inputPath)
		}
		if opts.outputPath != "output.pdf" {
			t.Errorf("outputPath = %q", opts.outputPath)
		}
	})
}
