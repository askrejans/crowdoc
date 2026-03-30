package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── isAllCapsHeading ───────────────────────────────────────────────────────

func TestIsAllCapsHeading_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"INTRODUCTION", true},
		{"SECTION ONE", true},
		{"PART 2: OVERVIEW", true},
		{"ABC", true},
	}
	for _, tt := range tests {
		if got := isAllCapsHeading(tt.input); got != tt.want {
			t.Errorf("isAllCapsHeading(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsAllCapsHeading_Invalid(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Introduction", false},   // has lowercase
		{"hello world", false},    // all lowercase
		{"AB", false},             // too short
		{"", false},               // empty
		{"123 456", false},        // no letters
	}
	for _, tt := range tests {
		if got := isAllCapsHeading(tt.input); got != tt.want {
			t.Errorf("isAllCapsHeading(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ─── isUnderline ────────────────────────────────────────────────────────────

func TestIsUnderline_Equals(t *testing.T) {
	if !isUnderline("======", '=') {
		t.Error("should detect equals underline")
	}
}

func TestIsUnderline_Dashes(t *testing.T) {
	if !isUnderline("------", '-') {
		t.Error("should detect dash underline")
	}
}

func TestIsUnderline_TooShort(t *testing.T) {
	if isUnderline("==", '=') {
		t.Error("should reject underline shorter than 3")
	}
}

func TestIsUnderline_Mixed(t *testing.T) {
	if isUnderline("==-==", '=') {
		t.Error("should reject mixed characters")
	}
}

// ─── isTxtListItem ──────────────────────────────────────────────────────────

func TestIsTxtListItem_Bullet(t *testing.T) {
	tests := []string{"- item", "* item", "+ item"}
	for _, input := range tests {
		if !isTxtListItem(input) {
			t.Errorf("should detect %q as list item", input)
		}
	}
}

func TestIsTxtListItem_Numbered(t *testing.T) {
	tests := []string{"1. first", "2. second", "10. tenth", "3) third"}
	for _, input := range tests {
		if !isTxtListItem(input) {
			t.Errorf("should detect %q as numbered list item", input)
		}
	}
}

func TestIsTxtListItem_NotList(t *testing.T) {
	tests := []string{"hello world", "---", "123", ""}
	for _, input := range tests {
		if isTxtListItem(input) {
			t.Errorf("should not detect %q as list item", input)
		}
	}
}

// ─── txtToMarkdown ──────────────────────────────────────────────────────────

func TestTxtToMarkdown_BasicParagraphs(t *testing.T) {
	input := "My Document Title\n\nFirst paragraph of text.\n\nSecond paragraph of text.\n"
	md := txtToMarkdown(input, "doc.txt")

	if !strings.Contains(md, "title: My Document Title") {
		t.Error("should use first line as title")
	}
	if !strings.Contains(md, "style: minimal") {
		t.Error("should default to minimal style")
	}
	if !strings.Contains(md, "First paragraph") {
		t.Error("should contain first paragraph")
	}
	if !strings.Contains(md, "Second paragraph") {
		t.Error("should contain second paragraph")
	}
}

func TestTxtToMarkdown_AllCapsHeadings(t *testing.T) {
	input := "Title\n\nINTRODUCTION\n\nSome text here.\n\nCONCLUSION\n\nFinal text.\n"
	md := txtToMarkdown(input, "doc.txt")

	if !strings.Contains(md, "## INTRODUCTION") {
		t.Error("should promote ALL CAPS to headings")
	}
	if !strings.Contains(md, "## CONCLUSION") {
		t.Error("should promote ALL CAPS to headings")
	}
}

func TestTxtToMarkdown_IndentedCodeBlock(t *testing.T) {
	input := "Title\n\nSome text:\n\n    def hello():\n        print('world')\n\nMore text.\n"
	md := txtToMarkdown(input, "doc.txt")

	if !strings.Contains(md, "```") {
		t.Error("should convert indented blocks to code blocks")
	}
	if !strings.Contains(md, "def hello():") {
		t.Error("should preserve code content")
	}
}

func TestTxtToMarkdown_ListItems(t *testing.T) {
	input := "Title\n\n- First item\n- Second item\n- Third item\n"
	md := txtToMarkdown(input, "doc.txt")

	if !strings.Contains(md, "- First item") {
		t.Error("should preserve list items")
	}
}

func TestTxtToMarkdown_EmptyContent(t *testing.T) {
	md := txtToMarkdown("", "empty.txt")

	if !strings.Contains(md, "title: Empty") {
		t.Error("should derive title from filename when content is empty")
	}
}

func TestTxtToMarkdown_UnderlineHeading(t *testing.T) {
	input := "Title\n\nSection One\n===========\n\nText here.\n"
	md := txtToMarkdown(input, "doc.txt")

	// Underline should be skipped (not rendered as content)
	if strings.Contains(md, "===========") {
		t.Error("should skip underline characters")
	}
}

// ─── parseTXTFile ───────────────────────────────────────────────────────────

func TestParseTXTFile_Basic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	os.WriteFile(path, []byte("Meeting Notes\n\nDiscussed project timeline.\nAgreed on milestones.\n"), 0644)

	doc, err := parseTXTFile(path, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.Title != "Meeting Notes" {
		t.Errorf("title = %q, want 'Meeting Notes'", doc.Title)
	}
	if doc.Style != "minimal" {
		t.Errorf("style = %q, want 'minimal'", doc.Style)
	}
}

func TestParseTXTFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	os.WriteFile(path, []byte(""), 0644)

	_, err := parseTXTFile(path, dir)
	if err == nil {
		t.Error("should return error for empty file")
	}
}

func TestParseTXTFile_WhitespaceOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blank.txt")
	os.WriteFile(path, []byte("   \n\n   \n"), 0644)

	_, err := parseTXTFile(path, dir)
	if err == nil {
		t.Error("should return error for whitespace-only file")
	}
}

func TestParseTXTFile_NonExistent(t *testing.T) {
	_, err := parseTXTFile("/nonexistent.txt", "/tmp")
	if err == nil {
		t.Error("should return error for non-existent file")
	}
}
