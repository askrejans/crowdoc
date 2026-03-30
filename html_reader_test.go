package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── extractTag ─────────────────────────────────────────────────────────────

func TestExtractTag_Title(t *testing.T) {
	html := `<html><head><title>My Page</title></head></html>`
	got := extractTag(html, "title")
	if got != "My Page" {
		t.Errorf("got %q, want 'My Page'", got)
	}
}

func TestExtractTag_Body(t *testing.T) {
	html := `<html><body><p>Hello</p></body></html>`
	got := extractTag(html, "body")
	if !strings.Contains(got, "Hello") {
		t.Error("should extract body content")
	}
}

func TestExtractTag_Missing(t *testing.T) {
	html := `<html><body>text</body></html>`
	got := extractTag(html, "title")
	if got != "" {
		t.Errorf("should return empty for missing tag, got %q", got)
	}
}

func TestExtractTag_WithAttributes(t *testing.T) {
	html := `<div class="main" id="content">inside</div>`
	got := extractTag(html, "div")
	if got != "inside" {
		t.Errorf("got %q, want 'inside'", got)
	}
}

// ─── removeTagBlock ─────────────────────────────────────────────────────────

func TestRemoveTagBlock_Script(t *testing.T) {
	html := `<p>text</p><script>alert('x')</script><p>more</p>`
	got := removeTagBlock(html, "script")
	if strings.Contains(got, "alert") {
		t.Error("should remove script block")
	}
	if !strings.Contains(got, "text") || !strings.Contains(got, "more") {
		t.Error("should preserve non-script content")
	}
}

func TestRemoveTagBlock_Style(t *testing.T) {
	html := `<style>body{color:red}</style><p>content</p>`
	got := removeTagBlock(html, "style")
	if strings.Contains(got, "color:red") {
		t.Error("should remove style block")
	}
}

// ─── stripTags ──────────────────────────────────────────────────────────────

func TestStripTags_Basic(t *testing.T) {
	got := stripTags("<p>Hello <b>world</b></p>")
	if got != "Hello world" {
		t.Errorf("got %q, want 'Hello world'", got)
	}
}

func TestStripTags_SelfClosing(t *testing.T) {
	got := stripTags("line<br/>break")
	if got != "linebreak" {
		t.Errorf("got %q", got)
	}
}

func TestStripTags_NoTags(t *testing.T) {
	got := stripTags("plain text")
	if got != "plain text" {
		t.Errorf("got %q", got)
	}
}

// ─── cleanText ──────────────────────────────────────────────────────────────

func TestCleanText_CollapseWhitespace(t *testing.T) {
	got := cleanText("  hello   world  \n  foo  ")
	if got != "hello world foo" {
		t.Errorf("got %q", got)
	}
}

func TestCleanText_Empty(t *testing.T) {
	got := cleanText("  ")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ─── convertHeadings ────────────────────────────────────────────────────────

func TestConvertHeadings_H1toH6(t *testing.T) {
	html := `<h1>One</h1><h2>Two</h2><h3>Three</h3><h4>Four</h4><h5>Five</h5><h6>Six</h6>`
	got := convertHeadings(html)

	if !strings.Contains(got, "# One") {
		t.Error("should convert h1")
	}
	if !strings.Contains(got, "## Two") {
		t.Error("should convert h2")
	}
	if !strings.Contains(got, "### Three") {
		t.Error("should convert h3")
	}
	if !strings.Contains(got, "#### Four") {
		t.Error("should convert h4")
	}
	if !strings.Contains(got, "##### Five") {
		t.Error("should convert h5")
	}
	if !strings.Contains(got, "###### Six") {
		t.Error("should convert h6")
	}
}

func TestConvertHeadings_WithAttributes(t *testing.T) {
	html := `<h2 class="title" id="intro">Introduction</h2>`
	got := convertHeadings(html)
	if !strings.Contains(got, "## Introduction") {
		t.Errorf("should handle attributes, got: %s", got)
	}
}

// ─── convertInlineFormatting ────────────────────────────────────────────────

func TestConvertInlineFormatting_Bold(t *testing.T) {
	tests := []string{
		"<strong>bold</strong>",
		"<b>bold</b>",
	}
	for _, input := range tests {
		got := convertInlineFormatting(input)
		if !strings.Contains(got, "**bold**") {
			t.Errorf("input %q → %q, want **bold**", input, got)
		}
	}
}

func TestConvertInlineFormatting_Italic(t *testing.T) {
	tests := []string{
		"<em>italic</em>",
		"<i>italic</i>",
	}
	for _, input := range tests {
		got := convertInlineFormatting(input)
		if !strings.Contains(got, "*italic*") {
			t.Errorf("input %q → %q, want *italic*", input, got)
		}
	}
}

func TestConvertInlineFormatting_Code(t *testing.T) {
	got := convertInlineFormatting("<code>foo()</code>")
	if !strings.Contains(got, "`foo()`") {
		t.Errorf("got %q, want backtick-wrapped", got)
	}
}

func TestConvertInlineFormatting_Strikethrough(t *testing.T) {
	got := convertInlineFormatting("<del>removed</del>")
	if !strings.Contains(got, "~~removed~~") {
		t.Errorf("got %q", got)
	}
}

func TestConvertInlineFormatting_LineBreak(t *testing.T) {
	got := convertInlineFormatting("line<br>break")
	if !strings.Contains(got, "line\nbreak") {
		t.Errorf("got %q", got)
	}
}

// ─── convertLinks ───────────────────────────────────────────────────────────

func TestConvertLinks_Basic(t *testing.T) {
	got := convertLinks(`<a href="https://example.com">Example</a>`)
	if !strings.Contains(got, "[Example](https://example.com)") {
		t.Errorf("got %q", got)
	}
}

func TestConvertLinks_EmptyHref(t *testing.T) {
	got := convertLinks(`<a href="">Text</a>`)
	if strings.Contains(got, "[Text]()") {
		t.Error("should not create link with empty href")
	}
}

func TestConvertLinks_HashOnly(t *testing.T) {
	got := convertLinks(`<a href="#">Skip</a>`)
	if strings.Contains(got, "(#)") {
		t.Error("should not create link for hash-only href")
	}
}

// ─── convertImages ──────────────────────────────────────────────────────────

func TestConvertImages_Basic(t *testing.T) {
	got := convertImages(`<img src="photo.jpg" alt="My Photo" />`)
	if !strings.Contains(got, "![My Photo](photo.jpg)") {
		t.Errorf("got %q", got)
	}
}

func TestConvertImages_NoAlt(t *testing.T) {
	got := convertImages(`<img src="img.png" />`)
	if !strings.Contains(got, "![](img.png)") {
		t.Errorf("got %q", got)
	}
}

// ─── convertCodeBlocks ──────────────────────────────────────────────────────

func TestConvertCodeBlocks_Pre(t *testing.T) {
	html := `<pre><code>hello world</code></pre>`
	got := convertCodeBlocks(html)
	if !strings.Contains(got, "```") {
		t.Error("should convert to fenced code block")
	}
	if !strings.Contains(got, "hello world") {
		t.Error("should preserve code content")
	}
}

func TestConvertCodeBlocks_WithLanguage(t *testing.T) {
	html := `<pre><code class="language-python">print('hi')</code></pre>`
	got := convertCodeBlocks(html)
	if !strings.Contains(got, "```python") {
		t.Errorf("should detect language class, got: %s", got)
	}
}

// ─── convertTables ──────────────────────────────────────────────────────────

func TestConvertTables_BasicTable(t *testing.T) {
	html := `<table>
		<tr><th>Name</th><th>Age</th></tr>
		<tr><td>Alice</td><td>30</td></tr>
		<tr><td>Bob</td><td>25</td></tr>
	</table>`
	got := convertTables(html)

	if !strings.Contains(got, "| Name | Age |") {
		t.Error("should convert header row")
	}
	if !strings.Contains(got, "| --- | --- |") {
		t.Error("should add separator")
	}
	if !strings.Contains(got, "| Alice | 30 |") {
		t.Error("should convert data rows")
	}
}

func TestConvertTables_NoHeader(t *testing.T) {
	html := `<table>
		<tr><td>A</td><td>B</td></tr>
		<tr><td>C</td><td>D</td></tr>
	</table>`
	got := convertTables(html)

	// First row becomes header-like, separator follows
	if !strings.Contains(got, "| --- | --- |") {
		t.Error("should add separator even without th")
	}
}

// ─── convertLists ───────────────────────────────────────────────────────────

func TestConvertLists_Unordered(t *testing.T) {
	html := `<ul><li>Apple</li><li>Banana</li></ul>`
	got := convertLists(html)

	if !strings.Contains(got, "- Apple") {
		t.Error("should convert to markdown unordered list")
	}
	if !strings.Contains(got, "- Banana") {
		t.Error("should convert all items")
	}
}

func TestConvertLists_Ordered(t *testing.T) {
	html := `<ol><li>First</li><li>Second</li></ol>`
	got := convertLists(html)

	if !strings.Contains(got, "1. First") {
		t.Error("should convert to markdown ordered list")
	}
	if !strings.Contains(got, "2. Second") {
		t.Error("should number items")
	}
}

// ─── convertBlockquotes ────────────────────────────────────────────────────

func TestConvertBlockquotes(t *testing.T) {
	html := `<blockquote>Important note here</blockquote>`
	got := convertBlockquotes(html)
	if !strings.Contains(got, "> Important note here") {
		t.Errorf("got %q", got)
	}
}

// ─── convertParagraphs ─────────────────────────────────────────────────────

func TestConvertParagraphs(t *testing.T) {
	html := `<p>First paragraph.</p><p>Second paragraph.</p>`
	got := convertParagraphs(html)

	if !strings.Contains(got, "First paragraph.") {
		t.Error("should preserve paragraph text")
	}
	if !strings.Contains(got, "Second paragraph.") {
		t.Error("should preserve all paragraphs")
	}
}

// ─── convertHR ──────────────────────────────────────────────────────────────

func TestConvertHR(t *testing.T) {
	tests := []string{"<hr>", "<hr/>", "<hr />", "<HR>"}
	for _, input := range tests {
		got := convertHR(input)
		if !strings.Contains(got, "---") {
			t.Errorf("input %q should convert to ---, got %q", input, got)
		}
	}
}

// ─── htmlToMarkdown (integration) ───────────────────────────────────────────

func TestHtmlToMarkdown_FullDocument(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
	<title>Test Article</title>
	<style>body{margin:0}</style>
</head>
<body>
	<h1>Test Article</h1>
	<p>This is the <strong>first</strong> paragraph with <em>emphasis</em>.</p>
	<h2>Section One</h2>
	<p>Content here with <code>inline code</code>.</p>
	<ul>
		<li>Item A</li>
		<li>Item B</li>
	</ul>
	<h2>Section Two</h2>
	<table>
		<tr><th>Key</th><th>Value</th></tr>
		<tr><td>Name</td><td>Test</td></tr>
	</table>
	<blockquote>A wise quote.</blockquote>
	<script>alert('ignored')</script>
</body>
</html>`

	md := htmlToMarkdown(html, "article.html")

	if !strings.Contains(md, "title: Test Article") {
		t.Error("should extract title")
	}
	if strings.Contains(md, "alert") {
		t.Error("should strip script blocks")
	}
	if strings.Contains(md, "margin:0") {
		t.Error("should strip style blocks")
	}
	if !strings.Contains(md, "**first**") {
		t.Error("should convert strong to bold")
	}
	if !strings.Contains(md, "*emphasis*") {
		t.Error("should convert em to italic")
	}
	if !strings.Contains(md, "- Item A") {
		t.Error("should convert list items")
	}
	if !strings.Contains(md, "| Key | Value |") {
		t.Error("should convert tables")
	}
	if !strings.Contains(md, "> A wise quote.") {
		t.Error("should convert blockquotes")
	}
}

func TestHtmlToMarkdown_NoTitle(t *testing.T) {
	html := `<p>Just some content</p>`
	md := htmlToMarkdown(html, "untitled-page.html")

	if !strings.Contains(md, "title: Untitled Page") {
		t.Error("should fallback to filename for title")
	}
}

func TestHtmlToMarkdown_H1AsTitle(t *testing.T) {
	html := `<h1>My Heading</h1><p>text</p>`
	md := htmlToMarkdown(html, "page.html")

	if !strings.Contains(md, "title: My Heading") {
		t.Error("should use h1 as title when no <title>")
	}
}

// ─── parseHTMLFile ──────────────────────────────────────────────────────────

func TestParseHTMLFile_Basic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "page.html")
	os.WriteFile(path, []byte(`<html><head><title>Test</title></head><body><h2>Hello</h2><p>World</p></body></html>`), 0644)

	doc, err := parseHTMLFile(path, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.Title != "Test" {
		t.Errorf("title = %q, want 'Test'", doc.Title)
	}
}

func TestParseHTMLFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.html")
	os.WriteFile(path, []byte(""), 0644)

	_, err := parseHTMLFile(path, dir)
	if err == nil {
		t.Error("should return error for empty file")
	}
}

func TestParseHTMLFile_NonExistent(t *testing.T) {
	_, err := parseHTMLFile("/nonexistent.html", "/tmp")
	if err == nil {
		t.Error("should return error for non-existent file")
	}
}

// ─── cleanWhitespace ────────────────────────────────────────────────────────

func TestCleanWhitespace_CollapseBlankLines(t *testing.T) {
	input := "line1\n\n\n\n\nline2\n"
	got := cleanWhitespace(input)
	if strings.Contains(got, "\n\n\n") {
		t.Error("should collapse 3+ newlines into 2")
	}
	if !strings.Contains(got, "line1") || !strings.Contains(got, "line2") {
		t.Error("should preserve content")
	}
}

func TestCleanWhitespace_TrailingSpaces(t *testing.T) {
	input := "hello   \nworld   \n"
	got := cleanWhitespace(input)
	lines := strings.Split(got, "\n")
	for _, line := range lines {
		if strings.HasSuffix(line, " ") {
			t.Errorf("should trim trailing spaces: %q", line)
		}
	}
}
