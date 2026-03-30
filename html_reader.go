package main

import (
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
)

// parseHTMLFile reads an HTML file and converts it to a Document via markdown.
// Uses a simple tag-based parser — no external dependencies.
func parseHTMLFile(path, inputDir string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("failed to read HTML file: %w", err)
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return Document{}, fmt.Errorf("HTML file is empty")
	}

	md := htmlToMarkdown(content, path)
	return parseMarkdown(md, inputDir), nil
}

// htmlToMarkdown converts HTML content to markdown.
// Handles common elements: headings, paragraphs, lists, tables, code, links, images.
func htmlToMarkdown(content, path string) string {
	// Extract title from <title> tag
	title := extractTag(content, "title")
	if title == "" {
		// Try first <h1>
		title = extractTag(content, "h1")
	}
	if title == "" {
		title = titleFromFilename(path)
	}
	title = cleanText(title)

	// Get body content; if no <body>, use entire content
	body := extractTag(content, "body")
	if body == "" {
		body = content
	}

	// Remove script and style blocks
	body = removeTagBlock(body, "script")
	body = removeTagBlock(body, "style")
	body = removeTagBlock(body, "head")
	body = removeTagBlock(body, "nav")
	body = removeTagBlock(body, "footer")

	// Convert block elements
	body = convertHeadings(body)
	body = convertCodeBlocks(body)
	body = convertBlockquotes(body)
	body = convertTables(body)
	body = convertLists(body)
	body = convertParagraphs(body)
	body = convertHR(body)

	// Convert inline elements
	body = convertInlineFormatting(body)
	body = convertLinks(body)
	body = convertImages(body)

	// Strip remaining HTML tags
	body = stripTags(body)

	// Decode HTML entities
	body = html.UnescapeString(body)

	// Clean up excessive whitespace
	body = cleanWhitespace(body)

	var md strings.Builder
	md.WriteString(fmt.Sprintf("---\ntitle: %s\n---\n\n", title))
	md.WriteString(body)

	return md.String()
}

// --- Tag extraction ---

var (
	reTagOpen     = regexp.MustCompile(`(?i)<(\w+)(?:\s[^>]*)?>`)
	reTagClose    = regexp.MustCompile(`(?i)</(\w+)\s*>`)
	reSelfClosing = regexp.MustCompile(`(?i)<(\w+)\s[^>]*/\s*>`)
)

// extractTag gets the inner content of the first occurrence of a tag.
func extractTag(s, tag string) string {
	re := regexp.MustCompile(`(?is)<` + tag + `(?:\s[^>]*)?>(.+?)</` + tag + `\s*>`)
	m := re.FindStringSubmatch(s)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// removeTagBlock removes all instances of a tag and its content.
func removeTagBlock(s, tag string) string {
	re := regexp.MustCompile(`(?is)<` + tag + `(?:\s[^>]*)?>.*?</` + tag + `\s*>`)
	return re.ReplaceAllString(s, "")
}

// --- Block conversions ---

func convertHeadings(s string) string {
	for level := 6; level >= 1; level-- {
		tag := fmt.Sprintf("h%d", level)
		prefix := strings.Repeat("#", level)
		re := regexp.MustCompile(`(?is)<` + tag + `(?:\s[^>]*)?>(.+?)</` + tag + `\s*>`)
		s = re.ReplaceAllStringFunc(s, func(match string) string {
			inner := re.FindStringSubmatch(match)
			if len(inner) >= 2 {
				text := cleanText(stripTags(inner[1]))
				return "\n\n" + prefix + " " + text + "\n\n"
			}
			return match
		})
	}
	return s
}

func convertCodeBlocks(s string) string {
	// <pre><code class="language-go"> or <pre>
	re := regexp.MustCompile(`(?is)<pre(?:\s[^>]*)?>(?:<code(?:\s[^>]*)?>)?(.+?)(?:</code\s*>)?</pre\s*>`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		inner := re.FindStringSubmatch(match)
		if len(inner) >= 2 {
			code := html.UnescapeString(stripTags(inner[1]))
			code = strings.TrimSpace(code)

			// Try to extract language from class attribute
			lang := ""
			langRe := regexp.MustCompile(`(?i)class="[^"]*(?:language-|lang-)(\w+)`)
			if lm := langRe.FindStringSubmatch(match); len(lm) >= 2 {
				lang = lm[1]
			}

			return "\n\n```" + lang + "\n" + code + "\n```\n\n"
		}
		return match
	})

	// Inline <code> handled separately in convertInlineFormatting
	return s
}

func convertBlockquotes(s string) string {
	re := regexp.MustCompile(`(?is)<blockquote(?:\s[^>]*)?>(.+?)</blockquote\s*>`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		inner := re.FindStringSubmatch(match)
		if len(inner) >= 2 {
			text := cleanText(stripTags(inner[1]))
			lines := strings.Split(text, "\n")
			var quoted []string
			for _, line := range lines {
				quoted = append(quoted, "> "+strings.TrimSpace(line))
			}
			return "\n\n" + strings.Join(quoted, "\n") + "\n\n"
		}
		return match
	})
	return s
}

func convertTables(s string) string {
	reTable := regexp.MustCompile(`(?is)<table(?:\s[^>]*)?>(.+?)</table\s*>`)

	s = reTable.ReplaceAllStringFunc(s, func(match string) string {
		inner := reTable.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		tableHTML := inner[1]

		// Extract rows
		reRow := regexp.MustCompile(`(?is)<tr(?:\s[^>]*)?>(.+?)</tr\s*>`)
		rowMatches := reRow.FindAllStringSubmatch(tableHTML, -1)

		if len(rowMatches) == 0 {
			return match
		}

		var rows [][]string
		hasHeader := false

		for _, rm := range rowMatches {
			if len(rm) < 2 {
				continue
			}
			rowHTML := rm[1]

			// Check for header cells
			reCell := regexp.MustCompile(`(?is)<t[hd](?:\s[^>]*)?>(.+?)</t[hd]\s*>`)
			cellMatches := reCell.FindAllStringSubmatch(rowHTML, -1)

			var cells []string
			for _, cm := range cellMatches {
				if len(cm) >= 2 {
					cells = append(cells, cleanText(stripTags(cm[1])))
				}
			}

			if strings.Contains(rowHTML, "<th") {
				hasHeader = true
			}

			if len(cells) > 0 {
				rows = append(rows, cells)
			}
		}

		if len(rows) == 0 {
			return ""
		}

		// Build markdown table
		var md strings.Builder
		md.WriteString("\n\n")

		// Normalize column count
		maxCols := 0
		for _, row := range rows {
			if len(row) > maxCols {
				maxCols = len(row)
			}
		}

		// Header row
		header := padRow(rows[0], maxCols)
		md.WriteString("|")
		for _, cell := range header {
			md.WriteString(" " + cell + " |")
		}
		md.WriteString("\n|")
		for range header {
			md.WriteString(" --- |")
		}
		md.WriteString("\n")

		// Data rows (skip first if it was header)
		start := 0
		if hasHeader {
			start = 1
		}
		for _, row := range rows[start:] {
			padded := padRow(row, maxCols)
			md.WriteString("|")
			for _, cell := range padded {
				md.WriteString(" " + cell + " |")
			}
			md.WriteString("\n")
		}

		md.WriteString("\n")
		return md.String()
	})

	return s
}

func convertLists(s string) string {
	// Unordered lists
	reUL := regexp.MustCompile(`(?is)<ul(?:\s[^>]*)?>(.+?)</ul\s*>`)
	s = reUL.ReplaceAllStringFunc(s, func(match string) string {
		inner := reUL.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		return "\n" + convertListItems(inner[1], "- ") + "\n"
	})

	// Ordered lists
	reOL := regexp.MustCompile(`(?is)<ol(?:\s[^>]*)?>(.+?)</ol\s*>`)
	s = reOL.ReplaceAllStringFunc(s, func(match string) string {
		inner := reOL.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		return "\n" + convertListItemsNumbered(inner[1]) + "\n"
	})

	return s
}

func convertListItems(html, prefix string) string {
	reLI := regexp.MustCompile(`(?is)<li(?:\s[^>]*)?>(.+?)</li\s*>`)
	matches := reLI.FindAllStringSubmatch(html, -1)

	var lines []string
	for _, m := range matches {
		if len(m) >= 2 {
			text := cleanText(stripTags(m[1]))
			lines = append(lines, prefix+text)
		}
	}
	return strings.Join(lines, "\n")
}

func convertListItemsNumbered(html string) string {
	reLI := regexp.MustCompile(`(?is)<li(?:\s[^>]*)?>(.+?)</li\s*>`)
	matches := reLI.FindAllStringSubmatch(html, -1)

	var lines []string
	for i, m := range matches {
		if len(m) >= 2 {
			text := cleanText(stripTags(m[1]))
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, text))
		}
	}
	return strings.Join(lines, "\n")
}

func convertParagraphs(s string) string {
	re := regexp.MustCompile(`(?is)<p(?:\s[^>]*)?>(.+?)</p\s*>`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		inner := re.FindStringSubmatch(match)
		if len(inner) >= 2 {
			text := strings.TrimSpace(inner[1])
			return "\n\n" + text + "\n\n"
		}
		return match
	})
	return s
}

func convertHR(s string) string {
	re := regexp.MustCompile(`(?i)<hr\s*/?\s*>`)
	return re.ReplaceAllString(s, "\n\n---\n\n")
}

// --- Inline conversions ---

func convertInlineFormatting(s string) string {
	// Bold
	reBold := regexp.MustCompile(`(?is)<(?:strong|b)(?:\s[^>]*)?>(.+?)</(?:strong|b)\s*>`)
	s = reBold.ReplaceAllString(s, "**$1**")

	// Italic
	reItalic := regexp.MustCompile(`(?is)<(?:em|i)(?:\s[^>]*)?>(.+?)</(?:em|i)\s*>`)
	s = reItalic.ReplaceAllString(s, "*$1*")

	// Inline code
	reCode := regexp.MustCompile(`(?is)<code(?:\s[^>]*)?>(.+?)</code\s*>`)
	s = reCode.ReplaceAllString(s, "`$1`")

	// Strikethrough
	reDel := regexp.MustCompile(`(?is)<(?:del|s|strike)(?:\s[^>]*)?>(.+?)</(?:del|s|strike)\s*>`)
	s = reDel.ReplaceAllString(s, "~~$1~~")

	// Line breaks
	reBR := regexp.MustCompile(`(?i)<br\s*/?\s*>`)
	s = reBR.ReplaceAllString(s, "\n")

	return s
}

func convertLinks(s string) string {
	re := regexp.MustCompile(`(?is)<a\s[^>]*href="([^"]*)"[^>]*>(.+?)</a\s*>`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		m := re.FindStringSubmatch(match)
		if len(m) >= 3 {
			href := m[1]
			text := cleanText(stripTags(m[2]))
			if href == "" || href == "#" {
				return text
			}
			return "[" + text + "](" + href + ")"
		}
		return match
	})
	return s
}

func convertImages(s string) string {
	re := regexp.MustCompile(`(?is)<img\s[^>]*src="([^"]*)"[^>]*/?\s*>`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		m := re.FindStringSubmatch(match)
		if len(m) >= 2 {
			src := m[1]
			alt := ""
			altRe := regexp.MustCompile(`(?i)alt="([^"]*)"`)
			if am := altRe.FindStringSubmatch(match); len(am) >= 2 {
				alt = am[1]
			}
			return "![" + alt + "](" + src + ")"
		}
		return match
	})
	return s
}

// --- Utilities ---

// stripTags removes all HTML tags from a string.
func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

// cleanText trims whitespace and collapses internal whitespace.
func cleanText(s string) string {
	s = strings.TrimSpace(s)
	// Collapse multiple spaces/newlines into single space
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, " ")
}

// cleanWhitespace normalizes excessive blank lines and trailing spaces.
func cleanWhitespace(s string) string {
	// Collapse 3+ newlines into 2
	re := regexp.MustCompile(`\n{3,}`)
	s = re.ReplaceAllString(s, "\n\n")

	// Trim trailing whitespace per line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}
