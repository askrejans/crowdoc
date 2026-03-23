package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// mdToLaTeX converts markdown body text to LaTeX.
// It handles paragraphs, lists, tables, code blocks, blockquotes, images,
// horizontal rules, and inline formatting.
func mdToLaTeX(s string, doc Document) string {
	lines := strings.Split(s, "\n")
	var result []string

	// List tracking
	var listStack []string // stack of "itemize" or "enumerate"
	inCodeBlock := false
	codeBlockLang := ""
	var codeLines []string
	inBlockquote := false
	var blockquoteLines []string

	closeAllLists := func() {
		for len(listStack) > 0 {
			last := listStack[len(listStack)-1]
			listStack = listStack[:len(listStack)-1]
			result = append(result, `\end{`+last+`}`)
		}
	}

	closeBlockquote := func() {
		if inBlockquote {
			content := strings.Join(blockquoteLines, "\n\n")
			result = append(result, `\begin{quotebox}`)
			result = append(result, inlineFormat(escapeLaTeX(content), doc))
			result = append(result, `\end{quotebox}`)
			inBlockquote = false
			blockquoteLines = nil
		}
	}

	// Determine the list depth of a line (how many leading spaces / indentation levels)
	listDepth := func(line string) int {
		spaces := 0
		for _, ch := range line {
			if ch == ' ' {
				spaces++
			} else if ch == '\t' {
				spaces += 4
			} else {
				break
			}
		}
		return spaces / 2 // 2 spaces per indent level
	}

	isListItem := func(trimmed string) (bool, string, string) {
		// Checkbox items
		if strings.HasPrefix(trimmed, "- [ ] ") {
			return true, "itemize", trimmed[6:]
		}
		if strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") {
			return true, "itemize", trimmed[6:]
		}
		// Unordered
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			return true, "itemize", trimmed[2:]
		}
		// Ordered
		if matched, _ := regexp.MatchString(`^\d+\.\s`, trimmed); matched {
			content := regexp.MustCompile(`^\d+\.\s`).ReplaceAllString(trimmed, "")
			return true, "enumerate", content
		}
		return false, "", ""
	}

	inTable := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// ── Code blocks (fenced) ──
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				closeAllLists()
				closeBlockquote()
				inCodeBlock = true
				codeBlockLang = strings.TrimPrefix(trimmed, "```")
				codeBlockLang = strings.TrimSpace(codeBlockLang)
				codeLines = nil
				continue
			} else {
				// End code block
				inCodeBlock = false
				renderCodeBlock(&result, codeBlockLang, codeLines)
				continue
			}
		}
		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// ── Table separator lines ──
		if regexp.MustCompile(`^\|[-\s|:]+\|$`).MatchString(trimmed) {
			continue
		}

		// ── Table rows ──
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			closeAllLists()
			closeBlockquote()
			if !inTable {
				inTable = true
				cells := parseTableRow(trimmed)
				ncols := len(cells)
				// Use X columns for auto-width
				colSpec := strings.Repeat("X", ncols)
				result = append(result, `\begin{tabularx}{\textwidth}{`+colSpec+`}`)
				result = append(result, `\toprule`)
				row := formatTableCells(cells, true, doc)
				result = append(result, row+` \\`)
				result = append(result, `\midrule`)
			} else {
				cells := parseTableRow(trimmed)
				row := formatTableCells(cells, false, doc)
				result = append(result, row+` \\`)
			}
			// Check if next line is end of table
			if i+1 >= len(lines) || !isTableRow(strings.TrimSpace(lines[i+1])) {
				result = append(result, `\bottomrule`)
				result = append(result, `\end{tabularx}`)
				result = append(result, `\vspace{0.5em}`)
				inTable = false
			}
			continue
		}

		// ── Blockquote ──
		if strings.HasPrefix(trimmed, "> ") || trimmed == ">" {
			closeAllLists()
			if !inBlockquote {
				inBlockquote = true
				blockquoteLines = nil
			}
			content := strings.TrimPrefix(trimmed, "> ")
			content = strings.TrimPrefix(content, ">")
			blockquoteLines = append(blockquoteLines, content)
			continue
		} else if inBlockquote {
			closeBlockquote()
		}

		// ── Images ──
		imgRe := regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]+)\)$`)
		if m := imgRe.FindStringSubmatch(trimmed); m != nil {
			closeAllLists()
			closeBlockquote()
			alt := m[1]
			imgPath := m[2]
			renderImage(&result, alt, imgPath, doc)
			continue
		}

		// ── Horizontal rule ──
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			closeAllLists()
			closeBlockquote()
			result = append(result, `\vspace{0.5em}\noindent\textcolor{rulecolor}{\rule{\textwidth}{0.4pt}}\vspace{0.5em}`)
			continue
		}

		// ── List items (with nesting support) ──
		isList, listType, content := isListItem(trimmed)
		if isList {
			closeBlockquote()
			depth := listDepth(line)

			// Adjust list stack to match depth
			targetDepth := depth + 1 // we need at least 1 level
			for len(listStack) > targetDepth {
				last := listStack[len(listStack)-1]
				listStack = listStack[:len(listStack)-1]
				result = append(result, `\end{`+last+`}`)
			}
			// Open new levels if needed
			for len(listStack) < targetDepth {
				if listType == "enumerate" {
					result = append(result, `\begin{enumerate}[leftmargin=1.5em, itemsep=0.3em]`)
				} else {
					result = append(result, `\begin{itemize}[leftmargin=1.5em, itemsep=0.3em]`)
				}
				listStack = append(listStack, listType)
			}
			// Check if this is at the right level but wrong type
			if len(listStack) > 0 && listStack[len(listStack)-1] != listType {
				last := listStack[len(listStack)-1]
				listStack[len(listStack)-1] = listType
				result = append(result, `\end{`+last+`}`)
				if listType == "enumerate" {
					result = append(result, `\begin{enumerate}[leftmargin=1.5em, itemsep=0.3em]`)
				} else {
					result = append(result, `\begin{itemize}[leftmargin=1.5em, itemsep=0.3em]`)
				}
			}

			// Checkbox decoration
			prefix := ""
			origTrimmed := strings.TrimSpace(line)
			if strings.HasPrefix(origTrimmed, "- [x] ") || strings.HasPrefix(origTrimmed, "- [X] ") {
				prefix = `$\boxtimes$ `
			} else if strings.HasPrefix(origTrimmed, "- [ ] ") {
				prefix = `$\square$ `
			}

			result = append(result, `  \item `+prefix+inlineFormat(escapeLaTeX(content), doc))
			continue
		}

		// ── Blank line ──
		if trimmed == "" {
			if len(listStack) > 0 {
				// Look ahead to see if list continues
				nextNonEmpty := ""
				for j := i + 1; j < len(lines); j++ {
					if strings.TrimSpace(lines[j]) != "" {
						nextNonEmpty = strings.TrimSpace(lines[j])
						break
					}
				}
				nextIsList, _, _ := isListItem(nextNonEmpty)
				if !nextIsList {
					closeAllLists()
				}
			}
			result = append(result, "")
			continue
		}

		// ── Regular paragraph text ──
		if len(listStack) > 0 {
			closeAllLists()
		}
		result = append(result, inlineFormat(escapeLaTeX(trimmed), doc))
	}

	closeAllLists()
	closeBlockquote()
	return strings.Join(result, "\n")
}

// renderCodeBlock outputs a LaTeX code listing.
func renderCodeBlock(result *[]string, lang string, lines []string) {
	code := strings.Join(lines, "\n")
	// Escape special LaTeX chars inside listings
	// lstlisting handles most escaping, but we pass through raw
	if lang == "" {
		lang = "text"
	}

	// Map common language names to listings-compatible names
	langMap := map[string]string{
		"go":         "Go",
		"golang":     "Go",
		"python":     "Python",
		"py":         "Python",
		"javascript": "Java",
		"js":         "Java",
		"typescript": "Java",
		"ts":         "Java",
		"java":       "Java",
		"c":          "C",
		"cpp":        "C++",
		"c++":        "C++",
		"csharp":     "C",
		"cs":         "C",
		"ruby":       "Ruby",
		"rb":         "Ruby",
		"rust":       "C",
		"bash":       "bash",
		"sh":         "bash",
		"shell":      "bash",
		"zsh":        "bash",
		"sql":        "SQL",
		"html":       "HTML",
		"xml":        "XML",
		"css":        "HTML",
		"json":       "Java",
		"yaml":       "bash",
		"yml":        "bash",
		"toml":       "bash",
		"markdown":   "bash",
		"md":         "bash",
		"tex":        "TeX",
		"latex":      "TeX",
		"r":          "R",
		"perl":       "Perl",
		"php":        "PHP",
		"lua":        "Lua",
		"make":       "make",
		"makefile":   "make",
		"dockerfile": "bash",
		"text":       "",
		"plain":      "",
	}

	lstLang := ""
	if mapped, ok := langMap[strings.ToLower(lang)]; ok {
		lstLang = mapped
	}

	langDirective := ""
	if lstLang != "" {
		langDirective = fmt.Sprintf("language=%s,", lstLang)
	}

	*result = append(*result, fmt.Sprintf(`\begin{lstlisting}[%sstyle=codestyle]`, langDirective))
	*result = append(*result, code)
	*result = append(*result, `\end{lstlisting}`)
}

// renderImage outputs a LaTeX figure for an image.
func renderImage(result *[]string, alt, imgPath string, doc Document) {
	// Resolve relative paths
	if !filepath.IsAbs(imgPath) && doc.InputDir != "" {
		imgPath = filepath.Join(doc.InputDir, imgPath)
	}

	*result = append(*result, `\begin{figure}[htbp]`)
	*result = append(*result, `\centering`)
	*result = append(*result, fmt.Sprintf(`\includegraphics[max width=\textwidth, max height=0.4\textheight]{%s}`, imgPath))
	if alt != "" {
		*result = append(*result, fmt.Sprintf(`\caption{%s}`, escapeLaTeX(alt)))
	}
	*result = append(*result, `\end{figure}`)
}

// escapeLaTeX escapes special LaTeX characters in a string.
func escapeLaTeX(s string) string {
	// Order matters: backslash first
	s = strings.ReplaceAll(s, `\`, `\textbackslash{}`)
	s = strings.ReplaceAll(s, `&`, `\&`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `$`, `\$`)
	s = strings.ReplaceAll(s, `#`, `\#`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	s = strings.ReplaceAll(s, `{`, `\{`)
	s = strings.ReplaceAll(s, `}`, `\}`)
	s = strings.ReplaceAll(s, `~`, `\textasciitilde{}`)
	s = strings.ReplaceAll(s, `^`, `\textasciicircum{}`)
	return s
}

// inlineFormat applies inline Markdown formatting to an already-escaped LaTeX string.
func inlineFormat(s string, doc Document) string {
	// Math: restore $...$ for inline math and $$...$$ for display math
	// We need to un-escape $ signs that are part of math delimiters
	// Display math: \$\$...\$\$
	displayMath := regexp.MustCompile(`\\\$\\\$(.+?)\\\$\\\$`)
	s = displayMath.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[4 : len(m)-4] // strip \$\$ from both ends
		// Un-escape common math characters
		inner = unescapeForMath(inner)
		return `\[` + inner + `\]`
	})

	// Inline math: \$...\$
	inlineMath := regexp.MustCompile(`\\\$([^\$]+?)\\\$`)
	s = inlineMath.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[2 : len(m)-2] // strip \$ from both ends
		inner = unescapeForMath(inner)
		return `$` + inner + `$`
	})

	// Bold: **text** or __text__
	bold := regexp.MustCompile(`\*\*(.+?)\*\*|\_\_(.+?)\_\_`)
	s = bold.ReplaceAllStringFunc(s, func(m string) string {
		// Remove the ** or __ delimiters
		if strings.HasPrefix(m, "**") {
			inner := m[2 : len(m)-2]
			return `\textbf{` + inner + `}`
		}
		// __ case: we have \_\_ due to escaping
		inner := m[4 : len(m)-4]
		return `\textbf{` + inner + `}`
	})

	// Italic: *text* or _text_ (but not inside words for _)
	italic := regexp.MustCompile(`\*(.+?)\*`)
	s = italic.ReplaceAllString(s, `\textit{$1}`)

	// Strikethrough: ~~text~~
	strike := regexp.MustCompile(`\~\~(.+?)\~\~`)
	s = strike.ReplaceAllStringFunc(s, func(m string) string {
		// We have \textasciitilde{}\textasciitilde{} due to escaping
		return m // strikethrough is complex with escaping, skip for now
	})

	// Inline code: `text` — must come after bold/italic to avoid conflicts
	// Use \verb for inline code when possible (handles special chars natively)
	// Fall back to \texttt with manual escaping if content contains |
	code := regexp.MustCompile("`([^`]+)`")
	s = code.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[1 : len(m)-1]
		// Un-escape LaTeX escapes back to raw chars for \verb
		inner = strings.ReplaceAll(inner, `\textbackslash{}`, `\`)
		inner = strings.ReplaceAll(inner, `\&`, `&`)
		inner = strings.ReplaceAll(inner, `\%`, `%`)
		inner = strings.ReplaceAll(inner, `\$`, `$`)
		inner = strings.ReplaceAll(inner, `\#`, `#`)
		inner = strings.ReplaceAll(inner, `\_`, `_`)
		inner = strings.ReplaceAll(inner, `\{`, `{`)
		inner = strings.ReplaceAll(inner, `\}`, `}`)
		inner = strings.ReplaceAll(inner, `\textasciitilde{}`, `~`)
		inner = strings.ReplaceAll(inner, `\textasciicircum{}`, `^`)
		// Choose a delimiter that's not in the content
		for _, delim := range []string{"|", "!", "@", "+"} {
			if !strings.Contains(inner, delim) {
				return `\verb` + delim + inner + delim
			}
		}
		// Fallback: use \texttt with escaped underscores
		inner = strings.ReplaceAll(inner, `_`, `\_`)
		inner = strings.ReplaceAll(inner, `#`, `\#`)
		inner = strings.ReplaceAll(inner, `$`, `\$`)
		inner = strings.ReplaceAll(inner, `%`, `\%`)
		inner = strings.ReplaceAll(inner, `&`, `\&`)
		return `\texttt{` + inner + `}`
	})

	// Links: [text](url) — render as text with footnote URL
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	s = linkRe.ReplaceAllStringFunc(s, func(m string) string {
		matches := linkRe.FindStringSubmatch(m)
		if len(matches) < 3 {
			return m
		}
		text := matches[1]
		url := matches[2]
		// Un-escape URL
		url = strings.ReplaceAll(url, `\_`, `_`)
		url = strings.ReplaceAll(url, `\#`, `#`)
		url = strings.ReplaceAll(url, `\%`, `%`)
		url = strings.ReplaceAll(url, `\&`, `&`)
		return fmt.Sprintf(`\href{%s}{%s}\footnote{\url{%s}}`, url, text, url)
	})

	// Footnote references: [^id] → LaTeX footnote
	fnRef := regexp.MustCompile(`\[\textasciicircum\{\}(\w+)\]`)
	s = fnRef.ReplaceAllStringFunc(s, func(m string) string {
		matches := fnRef.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		id := matches[1]
		if text, ok := doc.Footnotes[id]; ok {
			return `\footnote{` + escapeLaTeX(text) + `}`
		}
		return m
	})

	// Citation: [@key] → superscript reference
	citeRe := regexp.MustCompile(`\[@([^\]]+)\]`)
	s = citeRe.ReplaceAllString(s, `\textsuperscript{[$1]}`)

	return s
}

// unescapeForMath restores LaTeX-escaped characters back for use in math mode.
func unescapeForMath(s string) string {
	s = strings.ReplaceAll(s, `\_`, `_`)
	s = strings.ReplaceAll(s, `\{`, `{`)
	s = strings.ReplaceAll(s, `\}`, `}`)
	s = strings.ReplaceAll(s, `\textasciicircum{}`, `^`)
	s = strings.ReplaceAll(s, `\#`, `#`)
	s = strings.ReplaceAll(s, `\&`, `&`)
	return s
}

// parseTableRow splits a markdown table row into cells.
func parseTableRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

// isTableRow checks if a line looks like a markdown table row.
func isTableRow(line string) bool {
	if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
		return true
	}
	if regexp.MustCompile(`^\|[-\s|:]+\|$`).MatchString(line) {
		return true
	}
	return false
}

// formatTableCells formats cells for a LaTeX table row.
func formatTableCells(cells []string, header bool, doc Document) string {
	var formatted []string
	for _, c := range cells {
		escaped := inlineFormat(escapeLaTeX(c), doc)
		if header {
			formatted = append(formatted, `\textbf{`+escaped+`}`)
		} else {
			formatted = append(formatted, escaped)
		}
	}
	return strings.Join(formatted, " & ")
}

// sectionCmd returns the LaTeX sectioning command for a given heading level.
func sectionCmd(level int) string {
	switch level {
	case 1:
		return "section"
	case 2:
		return "subsection"
	case 3:
		return "subsubsection"
	default:
		return "paragraph"
	}
}
