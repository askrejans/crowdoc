package main

import (
	"strings"
	"testing"
)

// ─── escapeLaTeX ─────────────────────────────────────────────────────────────

func TestEscapeLaTeX_AllSpecialChars(t *testing.T) {
	input := `\ & % $ # _ { } ~ ^`
	got := escapeLaTeX(input)

	if !strings.Contains(got, `\textbackslash{}`) {
		t.Error("should escape backslash")
	}
	if !strings.Contains(got, `\&`) {
		t.Error("should escape &")
	}
	if !strings.Contains(got, `\%`) {
		t.Error("should escape %")
	}
	if !strings.Contains(got, `\$`) {
		t.Error("should escape $")
	}
	if !strings.Contains(got, `\#`) {
		t.Error("should escape #")
	}
	if !strings.Contains(got, `\_`) {
		t.Error("should escape _")
	}
	if !strings.Contains(got, `\{`) {
		t.Error("should escape {")
	}
	if !strings.Contains(got, `\}`) {
		t.Error("should escape }")
	}
	if !strings.Contains(got, `\textasciitilde{}`) {
		t.Error("should escape ~")
	}
	if !strings.Contains(got, `\textasciicircum{}`) {
		t.Error("should escape ^")
	}
}

func TestEscapeLaTeX_EmptyString(t *testing.T) {
	if got := escapeLaTeX(""); got != "" {
		t.Errorf("escapeLaTeX(\"\") = %q", got)
	}
}

func TestEscapeLaTeX_NoSpecialChars(t *testing.T) {
	input := "Hello World 123"
	if got := escapeLaTeX(input); got != input {
		t.Errorf("escapeLaTeX(%q) = %q", input, got)
	}
}

func TestEscapeLaTeX_BackslashFirst(t *testing.T) {
	// Backslash must be escaped first to avoid double-escaping
	got := escapeLaTeX(`\&`)
	// Should be \textbackslash{}\& not \textbackslash{}\textbackslash{}&
	if !strings.HasPrefix(got, `\textbackslash{}`) {
		t.Errorf("got %q, backslash should be escaped first", got)
	}
}

func TestEscapeLaTeX_MultipleOccurrences(t *testing.T) {
	got := escapeLaTeX("$100 & $200")
	if strings.Count(got, `\$`) != 2 {
		t.Errorf("should escape both $ signs, got %q", got)
	}
}

// ─── unescapeForMath ─────────────────────────────────────────────────────────

func TestUnescapeForMath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`\_`, `_`},
		{`\{`, `{`},
		{`\}`, `}`},
		{`\textasciicircum{}`, `^`},
		{`\#`, `#`},
		{`\&`, `&`},
		{`x\_1\textasciicircum{}2`, `x_1^2`},
		{`plain text`, `plain text`},
		{``, ``},
	}

	for _, tt := range tests {
		got := unescapeForMath(tt.input)
		if got != tt.expected {
			t.Errorf("unescapeForMath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// ─── sectionCmd ──────────────────────────────────────────────────────────────

func TestSectionCmd(t *testing.T) {
	tests := []struct {
		level    int
		expected string
	}{
		{1, "section"},
		{2, "subsection"},
		{3, "subsubsection"},
		{4, "paragraph"},
		{5, "paragraph"},
		{0, "paragraph"},
		{-1, "paragraph"},
		{99, "paragraph"},
	}

	for _, tt := range tests {
		got := sectionCmd(tt.level)
		if got != tt.expected {
			t.Errorf("sectionCmd(%d) = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

// ─── parseTableRow ───────────────────────────────────────────────────────────

func TestParseTableRow(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"| A | B | C |", []string{"A", "B", "C"}},
		{"| Single |", []string{"Single"}},
		{"|  Spaces  |  Here  |", []string{"Spaces", "Here"}},
		{"| | Empty | |", []string{"", "Empty", ""}},
	}

	for _, tt := range tests {
		got := parseTableRow(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("parseTableRow(%q) len = %d, want %d", tt.input, len(got), len(tt.expected))
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("parseTableRow(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

// ─── isTableRow ──────────────────────────────────────────────────────────────

func TestIsTableRow(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"| A | B |", true},
		{"|---|---|", true},
		{"| :--- | ---: |", true},
		{"not a table", false},
		{"| only start", false},
		{"only end |", false},
		{"", false},
		{"|single|", true},
	}

	for _, tt := range tests {
		got := isTableRow(tt.input)
		if got != tt.expected {
			t.Errorf("isTableRow(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// ─── formatTableCells ────────────────────────────────────────────────────────

func TestFormatTableCells_Header(t *testing.T) {
	doc := Document{}
	got := formatTableCells([]string{"Name", "Value"}, true, doc)
	if !strings.Contains(got, `\textbf{`) {
		t.Errorf("header cells should be bold, got %q", got)
	}
	if !strings.Contains(got, " & ") {
		t.Errorf("cells should be joined with &, got %q", got)
	}
}

func TestFormatTableCells_Body(t *testing.T) {
	doc := Document{}
	got := formatTableCells([]string{"A", "B"}, false, doc)
	if strings.Contains(got, `\textbf{`) {
		t.Errorf("body cells should not be bold, got %q", got)
	}
}

func TestFormatTableCells_SpecialChars(t *testing.T) {
	doc := Document{}
	got := formatTableCells([]string{"100%", "$50"}, false, doc)
	if !strings.Contains(got, `\%`) {
		t.Errorf("should escape %%, got %q", got)
	}
	if !strings.Contains(got, `\$`) {
		t.Errorf("should escape $, got %q", got)
	}
}

func TestFormatTableCells_Empty(t *testing.T) {
	doc := Document{}
	got := formatTableCells([]string{}, false, doc)
	if got != "" {
		t.Errorf("empty cells should produce empty string, got %q", got)
	}
}

// ─── renderCodeBlock ─────────────────────────────────────────────────────────

func TestRenderCodeBlock_WithLanguage(t *testing.T) {
	var result []string
	renderCodeBlock(&result, "go", []string{"package main", "", "func main() {}"})

	joined := strings.Join(result, "\n")
	if !strings.Contains(joined, "language=Go") {
		t.Errorf("should map 'go' to 'Go', got %q", joined)
	}
	if !strings.Contains(joined, `\begin{lstlisting}`) {
		t.Error("should contain lstlisting begin")
	}
	if !strings.Contains(joined, `\end{lstlisting}`) {
		t.Error("should contain lstlisting end")
	}
}

func TestRenderCodeBlock_EmptyLanguage(t *testing.T) {
	var result []string
	renderCodeBlock(&result, "", []string{"plain text"})

	joined := strings.Join(result, "\n")
	// Empty lang maps to "text" which maps to "" (no language directive)
	if strings.Contains(joined, "language=") {
		t.Errorf("empty lang should not have language directive, got %q", joined)
	}
}

func TestRenderCodeBlock_UnknownLanguage(t *testing.T) {
	var result []string
	renderCodeBlock(&result, "brainfuck", []string{"++++++++[>++++[>++>+++>+++>+<<<<-]"})

	joined := strings.Join(result, "\n")
	// Unknown language should not have language directive
	if strings.Contains(joined, "language=") {
		t.Errorf("unknown lang should not have language directive, got %q", joined)
	}
}

func TestRenderCodeBlock_LanguageMapping(t *testing.T) {
	tests := map[string]string{
		"python":     "Python",
		"py":         "Python",
		"golang":     "Go",
		"javascript": "Java",
		"bash":       "bash",
		"sh":         "bash",
		"sql":        "SQL",
		"html":       "HTML",
		"tex":        "TeX",
		"ruby":       "Ruby",
		"php":        "PHP",
		"lua":        "Lua",
		"r":          "R",
		"perl":       "Perl",
		"cpp":        "C++",
		"c++":        "C++",
		"dockerfile": "bash",
		"yaml":       "bash",
		"json":       "Java",
	}

	for input, expected := range tests {
		var result []string
		renderCodeBlock(&result, input, []string{"code"})
		joined := strings.Join(result, "\n")
		if !strings.Contains(joined, "language="+expected) {
			t.Errorf("renderCodeBlock lang=%q should map to %q, got %q", input, expected, joined)
		}
	}
}

func TestRenderCodeBlock_EmptyCode(t *testing.T) {
	var result []string
	renderCodeBlock(&result, "go", []string{})

	if len(result) != 3 {
		t.Errorf("expected 3 lines (begin, empty, end), got %d", len(result))
	}
}

// ─── renderImage ─────────────────────────────────────────────────────────────

func TestRenderImage_RelativePath(t *testing.T) {
	var result []string
	doc := Document{InputDir: "/my/docs"}
	renderImage(&result, "Alt text", "images/photo.png", doc)

	joined := strings.Join(result, "\n")
	if !strings.Contains(joined, "/my/docs/images/photo.png") {
		t.Errorf("should resolve relative path, got %q", joined)
	}
	if !strings.Contains(joined, `\caption{Alt text}`) {
		t.Error("should include caption")
	}
}

func TestRenderImage_AbsolutePath(t *testing.T) {
	var result []string
	doc := Document{InputDir: "/my/docs"}
	renderImage(&result, "Alt", "/absolute/path/img.png", doc)

	joined := strings.Join(result, "\n")
	if !strings.Contains(joined, "/absolute/path/img.png") {
		t.Errorf("should keep absolute path, got %q", joined)
	}
}

func TestRenderImage_EmptyAlt(t *testing.T) {
	var result []string
	doc := Document{InputDir: "/tmp"}
	renderImage(&result, "", "img.png", doc)

	joined := strings.Join(result, "\n")
	if strings.Contains(joined, `\caption`) {
		t.Error("should not include caption for empty alt")
	}
}

func TestRenderImage_EmptyInputDir(t *testing.T) {
	var result []string
	doc := Document{InputDir: ""}
	renderImage(&result, "Alt", "relative/img.png", doc)

	joined := strings.Join(result, "\n")
	if !strings.Contains(joined, "relative/img.png") {
		t.Errorf("should keep relative path when InputDir empty, got %q", joined)
	}
}

// ─── mdToLaTeX ───────────────────────────────────────────────────────────────

func TestMdToLaTeX_EmptyString(t *testing.T) {
	got := mdToLaTeX("", Document{})
	if strings.TrimSpace(got) != "" {
		t.Errorf("mdToLaTeX(\"\") = %q", got)
	}
}

func TestMdToLaTeX_PlainText(t *testing.T) {
	got := mdToLaTeX("Hello world", Document{})
	if !strings.Contains(got, "Hello world") {
		t.Errorf("should contain plain text, got %q", got)
	}
}

func TestMdToLaTeX_UnorderedList(t *testing.T) {
	input := "- Item one\n- Item two\n- Item three"
	got := mdToLaTeX(input, Document{})

	if !strings.Contains(got, `\begin{itemize}`) {
		t.Error("should open itemize")
	}
	if !strings.Contains(got, `\end{itemize}`) {
		t.Error("should close itemize")
	}
	if strings.Count(got, `\item`) != 3 {
		t.Errorf("should have 3 items, got %q", got)
	}
}

func TestMdToLaTeX_OrderedList(t *testing.T) {
	input := "1. First\n2. Second\n3. Third"
	got := mdToLaTeX(input, Document{})

	if !strings.Contains(got, `\begin{enumerate}`) {
		t.Error("should open enumerate")
	}
	if !strings.Contains(got, `\end{enumerate}`) {
		t.Error("should close enumerate")
	}
}

func TestMdToLaTeX_NestedList(t *testing.T) {
	input := "- Parent\n  - Child\n    - Grandchild\n- Another parent"
	got := mdToLaTeX(input, Document{})

	// Should have nested itemize environments
	if strings.Count(got, `\begin{itemize}`) < 2 {
		t.Errorf("should have nested itemize, got %q", got)
	}
}

func TestMdToLaTeX_CheckboxList(t *testing.T) {
	input := "- [ ] Unchecked\n- [x] Checked\n- [X] Also checked"
	got := mdToLaTeX(input, Document{})

	if !strings.Contains(got, `$\square$`) {
		t.Error("should have unchecked checkbox")
	}
	if !strings.Contains(got, `$\boxtimes$`) {
		t.Error("should have checked checkbox")
	}
}

func TestMdToLaTeX_AsteriskList(t *testing.T) {
	input := "* Item one\n* Item two"
	got := mdToLaTeX(input, Document{})
	if !strings.Contains(got, `\begin{itemize}`) {
		t.Error("should handle * list markers")
	}
}

func TestMdToLaTeX_PlusList(t *testing.T) {
	input := "+ Item one\n+ Item two"
	got := mdToLaTeX(input, Document{})
	if !strings.Contains(got, `\begin{itemize}`) {
		t.Error("should handle + list markers")
	}
}

func TestMdToLaTeX_CodeBlock(t *testing.T) {
	input := "```go\nfunc main() {}\n```"
	got := mdToLaTeX(input, Document{})

	if !strings.Contains(got, `\begin{lstlisting}`) {
		t.Error("should contain lstlisting")
	}
	if !strings.Contains(got, "func main()") {
		t.Error("should contain code content")
	}
}

func TestMdToLaTeX_CodeBlockNoLanguage(t *testing.T) {
	input := "```\nplain code\n```"
	got := mdToLaTeX(input, Document{})

	if !strings.Contains(got, `\begin{lstlisting}`) {
		t.Error("should contain lstlisting")
	}
}

func TestMdToLaTeX_Table(t *testing.T) {
	input := "| Header 1 | Header 2 |\n|----------|----------|\n| Cell 1   | Cell 2   |"
	got := mdToLaTeX(input, Document{})

	if !strings.Contains(got, `\begin{tabularx}`) {
		t.Error("should contain tabularx")
	}
	if !strings.Contains(got, `\toprule`) {
		t.Error("should contain toprule")
	}
	if !strings.Contains(got, `\midrule`) {
		t.Error("should contain midrule")
	}
	if !strings.Contains(got, `\bottomrule`) {
		t.Error("should contain bottomrule")
	}
}

func TestMdToLaTeX_Blockquote(t *testing.T) {
	input := "> This is a quote\n> Second line"
	got := mdToLaTeX(input, Document{})

	if !strings.Contains(got, `\begin{quotebox}`) {
		t.Error("should contain quotebox begin")
	}
	if !strings.Contains(got, `\end{quotebox}`) {
		t.Error("should contain quotebox end")
	}
}

func TestMdToLaTeX_EmptyBlockquote(t *testing.T) {
	input := ">\n> Text after empty"
	got := mdToLaTeX(input, Document{})
	if !strings.Contains(got, `\begin{quotebox}`) {
		t.Error("should handle empty blockquote marker")
	}
}

func TestMdToLaTeX_HorizontalRule(t *testing.T) {
	for _, rule := range []string{"---", "***", "___"} {
		got := mdToLaTeX(rule, Document{})
		if !strings.Contains(got, `\rule{\textwidth}`) {
			t.Errorf("should render horizontal rule for %q, got %q", rule, got)
		}
	}
}

func TestMdToLaTeX_Image(t *testing.T) {
	input := "![Alt text](image.png)"
	doc := Document{InputDir: "/tmp"}
	got := mdToLaTeX(input, doc)

	if !strings.Contains(got, `\includegraphics`) {
		t.Error("should contain includegraphics")
	}
	if !strings.Contains(got, `\caption{Alt text}`) {
		t.Error("should contain caption")
	}
}

func TestMdToLaTeX_InlineBold(t *testing.T) {
	input := "This is **bold** text"
	got := mdToLaTeX(input, Document{})
	if !strings.Contains(got, `\textbf{bold}`) {
		t.Errorf("should contain bold, got %q", got)
	}
}

func TestMdToLaTeX_InlineItalic(t *testing.T) {
	input := "This is *italic* text"
	got := mdToLaTeX(input, Document{})
	if !strings.Contains(got, `\textit{italic}`) {
		t.Errorf("should contain italic, got %q", got)
	}
}

func TestMdToLaTeX_InlineCode(t *testing.T) {
	input := "Use `fmt.Println` here"
	got := mdToLaTeX(input, Document{})
	if !strings.Contains(got, `\verb`) {
		t.Errorf("should contain verb for inline code, got %q", got)
	}
}

func TestMdToLaTeX_Link(t *testing.T) {
	input := "Visit [Google](https://google.com) today"
	got := mdToLaTeX(input, Document{})
	if !strings.Contains(got, `\href{`) {
		t.Errorf("should contain href, got %q", got)
	}
	if !strings.Contains(got, `\footnote{\url{`) {
		t.Errorf("should contain footnote URL, got %q", got)
	}
}

func TestMdToLaTeX_Citation(t *testing.T) {
	input := "As noted by [@smith2024]"
	got := mdToLaTeX(input, Document{})
	if !strings.Contains(got, `\textsuperscript{[smith2024]}`) {
		t.Errorf("should contain citation superscript, got %q", got)
	}
}

func TestMdToLaTeX_ListFollowedByParagraph(t *testing.T) {
	input := "- Item 1\n- Item 2\n\nParagraph after list."
	got := mdToLaTeX(input, Document{})

	if !strings.Contains(got, `\end{itemize}`) {
		t.Error("should close list before paragraph")
	}
	if !strings.Contains(got, "Paragraph after list.") {
		t.Error("should contain paragraph text")
	}
}

func TestMdToLaTeX_MixedContent(t *testing.T) {
	input := `Some text.

- List item 1
- List item 2

> A blockquote

More text.

` + "```\ncode\n```"

	got := mdToLaTeX(input, Document{})

	if !strings.Contains(got, `\begin{itemize}`) {
		t.Error("should contain list")
	}
	if !strings.Contains(got, `\begin{quotebox}`) {
		t.Error("should contain blockquote")
	}
	if !strings.Contains(got, `\begin{lstlisting}`) {
		t.Error("should contain code block")
	}
}

func TestMdToLaTeX_SpecialCharsInText(t *testing.T) {
	input := "Price is 100% of $50 & tax"
	got := mdToLaTeX(input, Document{})
	if !strings.Contains(got, `100\%`) {
		t.Errorf("should escape %%, got %q", got)
	}
	if !strings.Contains(got, `\$50`) {
		t.Errorf("should escape $, got %q", got)
	}
	if !strings.Contains(got, `\&`) {
		t.Errorf("should escape &, got %q", got)
	}
}

func TestMdToLaTeX_TableSeparatorSkipped(t *testing.T) {
	input := "|---|---|"
	got := mdToLaTeX(input, Document{})
	// Separator line should be skipped entirely
	if strings.Contains(got, "---") {
		t.Errorf("separator should be skipped, got %q", got)
	}
}

func TestMdToLaTeX_MultipleBlankLines(t *testing.T) {
	input := "Text\n\n\n\nMore text"
	got := mdToLaTeX(input, Document{})
	// Should not crash, just produce empty lines
	if !strings.Contains(got, "Text") || !strings.Contains(got, "More text") {
		t.Errorf("should contain both texts, got %q", got)
	}
}

// ─── inlineFormat ────────────────────────────────────────────────────────────

func TestInlineFormat_BoldDoubleAsterisk(t *testing.T) {
	input := escapeLaTeX("**bold text**")
	got := inlineFormat(input, Document{})
	if !strings.Contains(got, `\textbf{bold text}`) {
		t.Errorf("got %q", got)
	}
}

func TestInlineFormat_ItalicSingleAsterisk(t *testing.T) {
	input := escapeLaTeX("*italic*")
	got := inlineFormat(input, Document{})
	if !strings.Contains(got, `\textit{italic}`) {
		t.Errorf("got %q", got)
	}
}

func TestInlineFormat_InlineCodeWithSpecialChars(t *testing.T) {
	// Inline code should un-escape LaTeX chars back to raw
	input := escapeLaTeX("use `my_var` here")
	got := inlineFormat(input, Document{})
	if !strings.Contains(got, `\verb`) {
		t.Errorf("should use verb for inline code, got %q", got)
	}
}

func TestInlineFormat_InlineCodeFallback(t *testing.T) {
	// When all common delimiters are in the content, should fall back to \texttt
	input := "use `|!@+` here"
	got := inlineFormat(input, Document{})
	if !strings.Contains(got, `\texttt{`) {
		t.Errorf("should fall back to texttt, got %q", got)
	}
}

func TestInlineFormat_LinkWithUnderscores(t *testing.T) {
	input := escapeLaTeX("[link](https://example.com/my_page)")
	got := inlineFormat(input, Document{})
	if !strings.Contains(got, `\href{https://example.com/my_page}`) {
		t.Errorf("should unescape URL underscores, got %q", got)
	}
}

func TestInlineFormat_EmptyString(t *testing.T) {
	got := inlineFormat("", Document{})
	if got != "" {
		t.Errorf("inlineFormat(\"\") = %q", got)
	}
}

func TestInlineFormat_NoFormatting(t *testing.T) {
	input := "plain text no formatting"
	got := inlineFormat(input, Document{})
	if got != input {
		t.Errorf("should pass through plain text, got %q", got)
	}
}

func TestInlineFormat_DisplayMath(t *testing.T) {
	input := escapeLaTeX("$$x^2 + y^2 = z^2$$")
	got := inlineFormat(input, Document{})
	if !strings.Contains(got, `\[`) && !strings.Contains(got, `\]`) {
		t.Errorf("should convert display math, got %q", got)
	}
}

func TestInlineFormat_InlineMath(t *testing.T) {
	input := escapeLaTeX("The formula $E = mc^2$ is famous")
	got := inlineFormat(input, Document{})
	// After escaping and inline format, should have math delimiters
	if !strings.Contains(got, `$`) {
		t.Errorf("should contain math delimiters, got %q", got)
	}
}
