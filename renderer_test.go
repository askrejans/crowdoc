package main

import (
	"strings"
	"testing"
)

// ─── renderLaTeX ─────────────────────────────────────────────────────────────

func TestRenderLaTeX_BasicDocument(t *testing.T) {
	doc := Document{
		Title:          "Test Title",
		Version:        "1.0",
		Status:         "DRAFT",
		Classification: "CONFIDENTIAL",
		Style:          "report",
		FontSize:       11,
		Sections: []Section{
			{Level: 2, Title: "Introduction", Content: "Hello world.\n"},
		},
		Footnotes: make(map[string]string),
	}

	latex := renderLaTeX(doc)

	if latex == "" {
		t.Fatal("renderLaTeX returned empty string")
	}
	if !strings.Contains(latex, `\documentclass`) {
		t.Error("should contain documentclass")
	}
	if !strings.Contains(latex, "Test Title") {
		t.Error("should contain title")
	}
	if !strings.Contains(latex, "Hello world") {
		t.Error("should contain section content")
	}
}

func TestRenderLaTeX_AllStyles(t *testing.T) {
	styles := []string{"legal", "technical", "report", "minimal", "letter"}

	for _, style := range styles {
		t.Run(style, func(t *testing.T) {
			doc := Document{
				Title:          "Test",
				Version:        "1.0",
				Status:         "DRAFT",
				Classification: "CONFIDENTIAL",
				Style:          style,
				FontSize:       11,
				GeneratedDate:  "2026-01-01",
				Footnotes:      make(map[string]string),
				Sections: []Section{
					{Level: 2, Title: "Section", Content: "Content.\n"},
				},
			}

			latex := renderLaTeX(doc)
			if latex == "" {
				t.Errorf("renderLaTeX for style %q returned empty", style)
			}
			if !strings.Contains(latex, `\begin{document}`) {
				t.Errorf("style %q missing \\begin{document}", style)
			}
			if !strings.Contains(latex, `\end{document}`) {
				t.Errorf("style %q missing \\end{document}", style)
			}
		})
	}
}

func TestRenderLaTeX_WithTOC(t *testing.T) {
	toc := true
	doc := Document{
		Title:          "Test",
		Style:          "report",
		FontSize:       11,
		TOC:            &toc,
		Classification: "CONFIDENTIAL",
		Status:         "DRAFT",
		Version:        "1.0",
		GeneratedDate:  "2026-01-01",
		Footnotes:      make(map[string]string),
		Sections: []Section{
			{Level: 2, Title: "A", Content: "a\n"},
			{Level: 2, Title: "B", Content: "b\n"},
			{Level: 2, Title: "C", Content: "c\n"},
		},
	}

	latex := renderLaTeX(doc)
	if !strings.Contains(latex, `\tableofcontents`) {
		t.Error("should contain tableofcontents when TOC is enabled")
	}
}

func TestRenderLaTeX_NoTitlePage(t *testing.T) {
	doc := Document{
		Title:          "Test",
		Style:          "report",
		FontSize:       11,
		NoTitlePage:    true,
		Classification: "CONFIDENTIAL",
		Status:         "DRAFT",
		Version:        "1.0",
		GeneratedDate:  "2026-01-01",
		Footnotes:      make(map[string]string),
		Sections: []Section{
			{Level: 2, Title: "A", Content: "a\n"},
		},
	}

	latex := renderLaTeX(doc)
	// When NoTitlePage is true, the conditional in the template should skip the title page
	if latex == "" {
		t.Error("should produce output even with no title page")
	}
}

func TestRenderLaTeX_CustomMargins(t *testing.T) {
	doc := Document{
		Title:          "Test",
		Style:          "report",
		FontSize:       11,
		MarginTop:      "3cm",
		MarginBottom:   "3cm",
		MarginLeft:     "2cm",
		MarginRight:    "2cm",
		Classification: "CONFIDENTIAL",
		Status:         "DRAFT",
		Version:        "1.0",
		GeneratedDate:  "2026-01-01",
		Footnotes:      make(map[string]string),
	}

	latex := renderLaTeX(doc)
	if !strings.Contains(latex, "3cm") {
		t.Error("should include custom margins")
	}
}

func TestRenderLaTeX_CustomHeaderFooter(t *testing.T) {
	doc := Document{
		Title:          "Test",
		Style:          "report",
		FontSize:       11,
		HeaderLeft:     "My Header",
		FooterRight:    "Page Footer",
		Classification: "CONFIDENTIAL",
		Status:         "DRAFT",
		Version:        "1.0",
		GeneratedDate:  "2026-01-01",
		Footnotes:      make(map[string]string),
	}

	latex := renderLaTeX(doc)
	if !strings.Contains(latex, "My Header") {
		t.Error("should include custom header")
	}
}

func TestRenderLaTeX_SpecialCharsInTitle(t *testing.T) {
	doc := Document{
		Title:          "Price: 100% & $50 Discount",
		Style:          "report",
		FontSize:       11,
		Classification: "CONFIDENTIAL",
		Status:         "DRAFT",
		Version:        "1.0",
		GeneratedDate:  "2026-01-01",
		Footnotes:      make(map[string]string),
	}

	latex := renderLaTeX(doc)
	// Title should be escaped in the template via escapeLaTeX
	if strings.Contains(latex, "100%") && !strings.Contains(latex, `100\%`) {
		t.Error("special chars in title should be escaped")
	}
}

func TestRenderLaTeX_EmptySections(t *testing.T) {
	doc := Document{
		Title:          "Empty",
		Style:          "minimal",
		FontSize:       11,
		Classification: "CONFIDENTIAL",
		Status:         "DRAFT",
		Version:        "1.0",
		GeneratedDate:  "2026-01-01",
		Footnotes:      make(map[string]string),
		Sections:       []Section{},
	}

	latex := renderLaTeX(doc)
	if latex == "" {
		t.Error("should produce output even with no sections")
	}
}

func TestRenderLaTeX_FontSize10(t *testing.T) {
	doc := Document{
		Title:          "Test",
		Style:          "report",
		FontSize:       10,
		Classification: "CONFIDENTIAL",
		Status:         "DRAFT",
		Version:        "1.0",
		GeneratedDate:  "2026-01-01",
		Footnotes:      make(map[string]string),
	}

	latex := renderLaTeX(doc)
	if !strings.Contains(latex, "10pt") {
		t.Error("should use 10pt font size")
	}
}

func TestRenderLaTeX_FontSize12(t *testing.T) {
	doc := Document{
		Title:          "Test",
		Style:          "report",
		FontSize:       12,
		Classification: "CONFIDENTIAL",
		Status:         "DRAFT",
		Version:        "1.0",
		GeneratedDate:  "2026-01-01",
		Footnotes:      make(map[string]string),
	}

	latex := renderLaTeX(doc)
	if !strings.Contains(latex, "12pt") {
		t.Error("should use 12pt font size")
	}
}

// ─── extractLaTeXError ───────────────────────────────────────────────────────

func TestExtractLaTeXError_WithErrors(t *testing.T) {
	output := `This is pdfTeX, Version 3.14159265
entering extended mode
(/tmp/doc.tex
! Undefined control sequence.
l.42 \badcommand
! Missing $ inserted.
l.50 _
Some other text
`
	got := extractLaTeXError(output)
	if !strings.Contains(got, "Undefined control sequence") {
		t.Errorf("should extract error, got %q", got)
	}
	if !strings.Contains(got, "Missing") {
		t.Errorf("should extract missing error, got %q", got)
	}
}

func TestExtractLaTeXError_NoErrors(t *testing.T) {
	output := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\nline 11\nline 12"
	got := extractLaTeXError(output)
	// Should return last 10 lines as fallback
	if !strings.Contains(got, "line 12") {
		t.Errorf("should return last lines, got %q", got)
	}
	if strings.Contains(got, "line 1\n") {
		t.Errorf("should not include first lines, got %q", got)
	}
}

func TestExtractLaTeXError_EmptyOutput(t *testing.T) {
	got := extractLaTeXError("")
	if got == "" {
		// Empty is acceptable
		return
	}
}

func TestExtractLaTeXError_ShortOutput(t *testing.T) {
	got := extractLaTeXError("only one line")
	if got == "" {
		t.Error("should return something even for short output")
	}
}

func TestExtractLaTeXError_BangPrefix(t *testing.T) {
	output := "! LaTeX Error: File not found.\n! Emergency stop."
	got := extractLaTeXError(output)
	if !strings.Contains(got, "File not found") {
		t.Errorf("should capture ! lines, got %q", got)
	}
}

func TestExtractLaTeXError_MaxFiveLines(t *testing.T) {
	output := "! Error 1\n! Error 2\n! Error 3\n! Error 4\n! Error 5\n! Error 6\n! Error 7"
	got := extractLaTeXError(output)
	count := strings.Count(got, "! Error")
	if count > 5 {
		t.Errorf("should cap at 5 error lines, got %d", count)
	}
}
