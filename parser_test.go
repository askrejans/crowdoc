package main

import (
	"strings"
	"testing"
)

// ─── parseMarkdown ───────────────────────────────────────────────────────────

func TestParseMarkdown_FullFrontmatter(t *testing.T) {
	md := `---
title: Test Document
subtitle: A Subtitle
date: 2026-01-15
version: 2.0
status: final
type: technical
style: minimal
summary: A brief summary
author: John Doe
language: lv
classification: internal
signatures: true
toc: true
font-size: 12
header-left: Left Header
header-right: Right Header
footer-left: Left Footer
footer-right: Right Footer
margin-top: 3cm
margin-bottom: 3cm
margin-left: 2cm
margin-right: 2cm
logo: logo.png
---

## Section One

Content here.
`
	doc := parseMarkdown(md, "/tmp")

	if doc.Title != "Test Document" {
		t.Errorf("Title = %q, want %q", doc.Title, "Test Document")
	}
	if doc.Subtitle != "A Subtitle" {
		t.Errorf("Subtitle = %q, want %q", doc.Subtitle, "A Subtitle")
	}
	if doc.Date != "2026-01-15" {
		t.Errorf("Date = %q, want %q", doc.Date, "2026-01-15")
	}
	if doc.Version != "2.0" {
		t.Errorf("Version = %q, want %q", doc.Version, "2.0")
	}
	if doc.Status != "FINAL" {
		t.Errorf("Status = %q, want %q", doc.Status, "FINAL")
	}
	if doc.DocType != "technical" {
		t.Errorf("DocType = %q, want %q", doc.DocType, "technical")
	}
	if doc.Style != "minimal" {
		t.Errorf("Style = %q, want %q", doc.Style, "minimal")
	}
	if doc.Summary != "A brief summary" {
		t.Errorf("Summary = %q, want %q", doc.Summary, "A brief summary")
	}
	if doc.Author != "John Doe" {
		t.Errorf("Author = %q, want %q", doc.Author, "John Doe")
	}
	if doc.Language != "lv" {
		t.Errorf("Language = %q, want %q", doc.Language, "lv")
	}
	if doc.Classification != "INTERNAL" {
		t.Errorf("Classification = %q, want %q", doc.Classification, "INTERNAL")
	}
	if !doc.HasSignatures {
		t.Error("HasSignatures should be true")
	}
	if doc.TOC == nil || !*doc.TOC {
		t.Error("TOC should be true")
	}
	if doc.FontSize != 12 {
		t.Errorf("FontSize = %d, want 12", doc.FontSize)
	}
	if doc.HeaderLeft != "Left Header" {
		t.Errorf("HeaderLeft = %q", doc.HeaderLeft)
	}
	if doc.HeaderRight != "Right Header" {
		t.Errorf("HeaderRight = %q", doc.HeaderRight)
	}
	if doc.FooterLeft != "Left Footer" {
		t.Errorf("FooterLeft = %q", doc.FooterLeft)
	}
	if doc.FooterRight != "Right Footer" {
		t.Errorf("FooterRight = %q", doc.FooterRight)
	}
	if doc.MarginTop != "3cm" {
		t.Errorf("MarginTop = %q", doc.MarginTop)
	}
	if doc.MarginBottom != "3cm" {
		t.Errorf("MarginBottom = %q", doc.MarginBottom)
	}
	if doc.MarginLeft != "2cm" {
		t.Errorf("MarginLeft = %q", doc.MarginLeft)
	}
	if doc.MarginRight != "2cm" {
		t.Errorf("MarginRight = %q", doc.MarginRight)
	}
	if doc.Logo != "logo.png" {
		t.Errorf("Logo = %q", doc.Logo)
	}
}

func TestParseMarkdown_NoFrontmatter(t *testing.T) {
	md := `# My Title

Some preamble text.

## Section One

Content.
`
	doc := parseMarkdown(md, "/tmp")

	if doc.Title != "My Title" {
		t.Errorf("Title = %q, want %q", doc.Title, "My Title")
	}
	if doc.Version != "1.0" {
		t.Errorf("Version = %q, want %q", doc.Version, "1.0")
	}
	if doc.Status != "DRAFT" {
		t.Errorf("Status = %q, want %q", doc.Status, "DRAFT")
	}
	if len(doc.Sections) != 1 {
		t.Errorf("got %d sections, want 1", len(doc.Sections))
	}
}

func TestParseMarkdown_EmptyDocument(t *testing.T) {
	doc := parseMarkdown("", "/tmp")

	if doc.Title != "Document" {
		t.Errorf("Title = %q, want %q", doc.Title, "Document")
	}
	if doc.FontSize != 11 {
		t.Errorf("FontSize = %d, want 11", doc.FontSize)
	}
}

func TestParseMarkdown_OnlyFrontmatter(t *testing.T) {
	md := `---
title: Only Frontmatter
---
`
	doc := parseMarkdown(md, "/tmp")
	if doc.Title != "Only Frontmatter" {
		t.Errorf("Title = %q, want %q", doc.Title, "Only Frontmatter")
	}
	if len(doc.Sections) != 0 {
		t.Errorf("got %d sections, want 0", len(doc.Sections))
	}
}

func TestParseMarkdown_TitleFromH1(t *testing.T) {
	md := `# Title From Heading

## Section
Content.
`
	doc := parseMarkdown(md, "/tmp")
	if doc.Title != "Title From Heading" {
		t.Errorf("Title = %q, want %q", doc.Title, "Title From Heading")
	}
}

func TestParseMarkdown_FrontmatterTitleOverridesH1(t *testing.T) {
	md := `---
title: Frontmatter Title
---

# Heading Title

## Section
Content.
`
	doc := parseMarkdown(md, "/tmp")
	if doc.Title != "Frontmatter Title" {
		t.Errorf("Title = %q, want %q", doc.Title, "Frontmatter Title")
	}
}

func TestParseMarkdown_MultipleSectionLevels(t *testing.T) {
	md := `---
title: Test
---

## Level 2

### Level 3

#### Level 4

## Another Level 2
`
	doc := parseMarkdown(md, "/tmp")
	if len(doc.Sections) != 4 {
		t.Fatalf("got %d sections, want 4", len(doc.Sections))
	}
	if doc.Sections[0].Level != 2 {
		t.Errorf("Section[0].Level = %d, want 2", doc.Sections[0].Level)
	}
	if doc.Sections[1].Level != 3 {
		t.Errorf("Section[1].Level = %d, want 3", doc.Sections[1].Level)
	}
	if doc.Sections[2].Level != 4 {
		t.Errorf("Section[2].Level = %d, want 4", doc.Sections[2].Level)
	}
	if doc.Sections[3].Level != 2 {
		t.Errorf("Section[3].Level = %d, want 2", doc.Sections[3].Level)
	}
}

func TestParseMarkdown_Footnotes(t *testing.T) {
	md := `---
title: Footnote Test
---

## Section

See this[^1] and this[^note].

[^1]: First footnote text.
[^note]: Named footnote text.
`
	doc := parseMarkdown(md, "/tmp")
	if len(doc.Footnotes) != 2 {
		t.Fatalf("got %d footnotes, want 2", len(doc.Footnotes))
	}
	if doc.Footnotes["1"] != "First footnote text." {
		t.Errorf("Footnote[1] = %q", doc.Footnotes["1"])
	}
	if doc.Footnotes["note"] != "Named footnote text." {
		t.Errorf("Footnote[note] = %q", doc.Footnotes["note"])
	}
}

func TestParseMarkdown_Preamble(t *testing.T) {
	md := `# Title

This is preamble before any section.

## First Section

Content.
`
	doc := parseMarkdown(md, "/tmp")
	if !strings.Contains(doc.RawPreamble, "This is preamble") {
		t.Errorf("RawPreamble = %q, should contain preamble text", doc.RawPreamble)
	}
}

func TestParseMarkdown_UnclosedFrontmatter(t *testing.T) {
	md := `---
title: No Closing

# Heading
Content.
`
	// The --- never closes, so everything is treated as body
	doc := parseMarkdown(md, "/tmp")
	// Title should come from the frontmatter-like parsing or H1
	// Since there's no closing ---, frontmatter is not extracted
	if doc.Title == "" {
		t.Error("Title should not be empty")
	}
}

func TestParseMarkdown_AutoSignaturesForLegal(t *testing.T) {
	md := `---
title: Service Agreement
style: legal
---

## Terms
Content.
`
	doc := parseMarkdown(md, "/tmp")
	if !doc.HasSignatures {
		t.Error("HasSignatures should be true for legal agreement")
	}
}

func TestParseMarkdown_AutoSignaturesContract(t *testing.T) {
	md := `---
title: Employment Contract
style: legal
---

## Terms
`
	doc := parseMarkdown(md, "/tmp")
	if !doc.HasSignatures {
		t.Error("HasSignatures should be true for legal contract")
	}
}

func TestParseMarkdown_AutoSignaturesNDA(t *testing.T) {
	md := `---
title: NDA Document
style: legal
---

## Terms
`
	doc := parseMarkdown(md, "/tmp")
	if !doc.HasSignatures {
		t.Error("HasSignatures should be true for NDA")
	}
}

func TestParseMarkdown_AutoSignaturesLatvian(t *testing.T) {
	md := `---
title: Pakalpojumu līgums
style: legal
---

## Nosacījumi
`
	doc := parseMarkdown(md, "/tmp")
	if !doc.HasSignatures {
		t.Error("HasSignatures should be true for Latvian līgums")
	}
}

func TestParseMarkdown_NoSignaturesForNonLegal(t *testing.T) {
	md := `---
title: Service Agreement
style: technical
---

## Terms
`
	doc := parseMarkdown(md, "/tmp")
	if doc.HasSignatures {
		t.Error("HasSignatures should be false for non-legal style even with agreement title")
	}
}

func TestParseMarkdown_GeneratedDateSet(t *testing.T) {
	doc := parseMarkdown("# Test", "/tmp")
	if doc.GeneratedDate == "" {
		t.Error("GeneratedDate should be set")
	}
}

func TestParseMarkdown_InputDirSet(t *testing.T) {
	doc := parseMarkdown("# Test", "/my/dir")
	if doc.InputDir != "/my/dir" {
		t.Errorf("InputDir = %q, want %q", doc.InputDir, "/my/dir")
	}
}

// ─── parseFrontmatter ────────────────────────────────────────────────────────

func TestParseFrontmatter_EmptyValues(t *testing.T) {
	doc := Document{FontSize: 11}
	parseFrontmatter("title:\nsubtitle:\ndate:", &doc)
	// Empty values should be skipped
	if doc.Title != "" {
		t.Errorf("Title should be empty, got %q", doc.Title)
	}
}

func TestParseFrontmatter_LangAlias(t *testing.T) {
	doc := Document{FontSize: 11}
	parseFrontmatter("lang: lv", &doc)
	if doc.Language != "lv" {
		t.Errorf("Language = %q, want %q", doc.Language, "lv")
	}
}

func TestParseFrontmatter_FontSizeDefault(t *testing.T) {
	doc := Document{FontSize: 11}
	parseFrontmatter("font-size: 99", &doc)
	// Invalid font size defaults to 11
	if doc.FontSize != 11 {
		t.Errorf("FontSize = %d, want 11", doc.FontSize)
	}
}

func TestParseFrontmatter_FontSize10(t *testing.T) {
	doc := Document{FontSize: 11}
	parseFrontmatter("font-size: 10", &doc)
	if doc.FontSize != 10 {
		t.Errorf("FontSize = %d, want 10", doc.FontSize)
	}
}

func TestParseFrontmatter_FontSizeAlias(t *testing.T) {
	doc := Document{FontSize: 11}
	parseFrontmatter("fontsize: 12", &doc)
	if doc.FontSize != 12 {
		t.Errorf("FontSize = %d, want 12", doc.FontSize)
	}
}

func TestParseFrontmatter_SignaturesYes(t *testing.T) {
	doc := Document{}
	parseFrontmatter("signatures: yes", &doc)
	if !doc.HasSignatures {
		t.Error("HasSignatures should be true for 'yes'")
	}
}

func TestParseFrontmatter_SignaturesFalse(t *testing.T) {
	doc := Document{}
	parseFrontmatter("signatures: false", &doc)
	if doc.HasSignatures {
		t.Error("HasSignatures should be false")
	}
}

func TestParseFrontmatter_TOCFalse(t *testing.T) {
	doc := Document{}
	parseFrontmatter("toc: false", &doc)
	if doc.TOC == nil {
		t.Fatal("TOC should not be nil")
	}
	if *doc.TOC {
		t.Error("TOC should be false")
	}
}

func TestParseFrontmatter_MalformedLines(t *testing.T) {
	doc := Document{FontSize: 11}
	parseFrontmatter("no-colon-here\n=invalid=\ntitle: Valid Title", &doc)
	if doc.Title != "Valid Title" {
		t.Errorf("Title = %q, want %q", doc.Title, "Valid Title")
	}
}

func TestParseFrontmatter_ColonInValue(t *testing.T) {
	doc := Document{}
	parseFrontmatter("title: Time: 12:30 PM", &doc)
	if doc.Title != "Time: 12:30 PM" {
		t.Errorf("Title = %q, want %q", doc.Title, "Time: 12:30 PM")
	}
}

func TestParseFrontmatter_CaseInsensitiveKeys(t *testing.T) {
	doc := Document{FontSize: 11}
	parseFrontmatter("Title: Test\nSTATUS: draft\nAUTHOR: Me", &doc)
	if doc.Title != "Test" {
		t.Errorf("Title = %q", doc.Title)
	}
	if doc.Status != "DRAFT" {
		t.Errorf("Status = %q", doc.Status)
	}
	if doc.Author != "Me" {
		t.Errorf("Author = %q", doc.Author)
	}
}

func TestParseFrontmatter_UnquotesValues(t *testing.T) {
	doc := Document{}
	parseFrontmatter(`title: "Quoted title"
subtitle: 'Quoted subtitle'
no-title-page: true`, &doc)
	if doc.Title != "Quoted title" {
		t.Fatalf("quoted title was not unquoted: %q", doc.Title)
	}
	if doc.Subtitle != "Quoted subtitle" {
		t.Fatalf("quoted subtitle was not unquoted: %q", doc.Subtitle)
	}
	if !doc.NoTitlePage {
		t.Fatal("no-title-page frontmatter should enable NoTitlePage")
	}
}

// ─── ShouldShowTOC ──────────────────────────────────────────────────────────

func TestShouldShowTOC_ForcedTrue(t *testing.T) {
	b := true
	doc := Document{TOC: &b}
	if !doc.ShouldShowTOC() {
		t.Error("ShouldShowTOC should return true when forced")
	}
}

func TestShouldShowTOC_ForcedFalse(t *testing.T) {
	b := false
	doc := Document{TOC: &b}
	if doc.ShouldShowTOC() {
		t.Error("ShouldShowTOC should return false when forced off")
	}
}

func TestShouldShowTOC_AutoWithFewSections(t *testing.T) {
	doc := Document{Sections: []Section{{Level: 2}, {Level: 2}}}
	if doc.ShouldShowTOC() {
		t.Error("ShouldShowTOC should return false with <3 sections")
	}
}

func TestShouldShowTOC_AutoWithManySections(t *testing.T) {
	doc := Document{Sections: []Section{{Level: 2}, {Level: 2}, {Level: 2}}}
	if !doc.ShouldShowTOC() {
		t.Error("ShouldShowTOC should return true with >=3 sections")
	}
}

func TestShouldShowTOC_AutoEmpty(t *testing.T) {
	doc := Document{}
	if doc.ShouldShowTOC() {
		t.Error("ShouldShowTOC should return false with no sections")
	}
}

// ─── detectDocType ───────────────────────────────────────────────────────────

func TestDetectDocType(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		{"Service Agreement", "agreement"},
		{"NDA for Partners", "agreement"},
		{"Employment Contract", "agreement"},
		{"Pakalpojumu līgums", "agreement"},
		{"Privacy Policy", "policy"},
		{"Terms and Conditions", "policy"},
		{"Iekšējie noteikumi", "policy"},
		{"Board Decision", "corporate"},
		{"Lēmums par budžetu", "corporate"},
		{"Meeting Protokols", "corporate"},
		{"API Specification", "technical"},
		{"Technical Guide", "technical"},
		{"Architecture Overview", "technical"},
		{"Annual Report", "report"},
		{"Data Analysis", "report"},
		{"Security Review", "report"},
		{"Business Letter", "letter"},
		{"Correspondence Log", "letter"},
		{"Project Plan", "internal"},
		{"Q4 Budget", "internal"},
		{"Product Roadmap", "internal"},
		{"Release Checklist", "internal"},
		{"Monthly Invoice", "invoice"},
		{"Sales Receipt", "invoice"},
		{"Mēneša rēķins", "invoice"},
		{"Internal Memo", "memo"},
		{"Company Memorandum", "memo"},
		{"Office Notice", "memo"},
		{"Team Announcement", "memo"},
		{"Research Paper", "academic"},
		{"PhD Thesis", "academic"},
		{"Doctoral Dissertation", "academic"},
		{"Journal Submission", "academic"},
		{"Case Study", "academic"},
		{"Random Title", "document"},
		{"", "document"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := detectDocType(tt.title)
			if got != tt.expected {
				t.Errorf("detectDocType(%q) = %q, want %q", tt.title, got, tt.expected)
			}
		})
	}
}

// ─── styleFromDocType ────────────────────────────────────────────────────────

func TestStyleFromDocType(t *testing.T) {
	tests := []struct {
		docType  string
		expected string
	}{
		{"agreement", "legal"},
		{"policy", "legal"},
		{"corporate", "legal"},
		{"technical", "technical"},
		{"report", "report"},
		{"internal", "report"},
		{"letter", "letter"},
		{"academic", "academic"},
		{"invoice", "invoice"},
		{"memo", "memo"},
		{"document", "report"},
		{"unknown", "report"},
		{"", "report"},
	}

	for _, tt := range tests {
		t.Run(tt.docType, func(t *testing.T) {
			got := styleFromDocType(tt.docType)
			if got != tt.expected {
				t.Errorf("styleFromDocType(%q) = %q, want %q", tt.docType, got, tt.expected)
			}
		})
	}
}

// ─── Auto-detect style from title ────────────────────────────────────────────

func TestParseMarkdown_AutoStyleDetection(t *testing.T) {
	md := `# API Specification

## Endpoints
`
	doc := parseMarkdown(md, "/tmp")
	if doc.Style != "technical" {
		t.Errorf("Style = %q, want %q", doc.Style, "technical")
	}
}

func TestParseMarkdown_DefaultStyleIsReport(t *testing.T) {
	md := `# Something Random

## Section
`
	doc := parseMarkdown(md, "/tmp")
	if doc.Style != "report" {
		t.Errorf("Style = %q, want %q", doc.Style, "report")
	}
}
