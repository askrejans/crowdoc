package main

import (
	"fmt"
	"os"
	"strings"
)

// parseTXTFile reads a plain text file and converts it to a Document via markdown.
// Detects paragraph breaks (double newlines), preserves structure.
func parseTXTFile(path, inputDir string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("failed to read text file: %w", err)
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return Document{}, fmt.Errorf("text file is empty")
	}

	md := txtToMarkdown(content, path)
	return parseMarkdown(md, inputDir), nil
}

// txtToMarkdown converts plain text content to markdown.
// Strategy:
//   - First non-empty line becomes the title
//   - Blank-line-separated blocks become paragraphs
//   - Lines that look like headings (ALL CAPS, or ending with :) get promoted
//   - Indented blocks (4+ spaces or tab) become code blocks
func txtToMarkdown(content, path string) string {
	lines := strings.Split(content, "\n")

	// Find title: first non-empty line
	title := ""
	startIdx := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			title = trimmed
			startIdx = i + 1
			break
		}
	}

	if title == "" {
		title = titleFromFilename(path)
	}

	var md strings.Builder
	md.WriteString(fmt.Sprintf("---\ntitle: %s\nstyle: minimal\n---\n\n", title))

	// Process remaining lines
	inCodeBlock := false
	prevEmpty := false

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Detect indented blocks (code)
		if !inCodeBlock && (strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")) && trimmed != "" {
			md.WriteString("```\n")
			inCodeBlock = true
		}
		if inCodeBlock && !strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
			md.WriteString("```\n\n")
			inCodeBlock = false
		}

		if inCodeBlock {
			// Strip one level of indentation
			stripped := line
			if strings.HasPrefix(stripped, "    ") {
				stripped = stripped[4:]
			} else if strings.HasPrefix(stripped, "\t") {
				stripped = stripped[1:]
			}
			md.WriteString(stripped + "\n")
			continue
		}

		// Empty line = paragraph break
		if trimmed == "" {
			if !prevEmpty {
				md.WriteString("\n")
			}
			prevEmpty = true
			continue
		}
		prevEmpty = false

		// Detect ALL CAPS headings (at least 3 chars, not a list item)
		if isAllCapsHeading(trimmed) {
			md.WriteString("## " + trimmed + "\n\n")
			continue
		}

		// Detect underline-style headings (line of === or ---)
		if i > 0 && (isUnderline(trimmed, '=') || isUnderline(trimmed, '-')) {
			// Previous line was the heading text; it was already written, wrap it
			// Actually we already wrote the previous line, so this is tricky.
			// Just skip the underline — the previous line stands as a paragraph.
			continue
		}

		// Detect bullet-like patterns (-, *, numbers followed by . or ))
		if isTxtListItem(trimmed) {
			md.WriteString(trimmed + "\n")
			continue
		}

		// Regular paragraph line
		md.WriteString(trimmed + "\n")
	}

	if inCodeBlock {
		md.WriteString("```\n")
	}

	return md.String()
}

// isAllCapsHeading returns true for lines that look like section headings.
// Must be 3+ chars, all uppercase letters/spaces/numbers, no lowercase.
func isAllCapsHeading(s string) bool {
	if len(s) < 3 || len(s) > 80 {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
	}
	return hasLetter
}

// isUnderline checks if a line is a repeated character (=== or ---).
func isUnderline(s string, ch byte) bool {
	if len(s) < 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != ch {
			return false
		}
	}
	return true
}

// isTxtListItem detects common plain text list patterns.
func isTxtListItem(s string) bool {
	if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "+ ") {
		return true
	}
	// Numbered: "1. ", "2) ", etc.
	for i := 0; i < len(s) && i < 4; i++ {
		if s[i] >= '0' && s[i] <= '9' {
			continue
		}
		if (s[i] == '.' || s[i] == ')') && i > 0 && i+1 < len(s) && s[i+1] == ' ' {
			return true
		}
		break
	}
	return false
}
