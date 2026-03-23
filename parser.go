package main

import (
	"strings"
	"time"
)

// Document represents a parsed markdown document with all metadata.
type Document struct {
	// Core metadata
	Title          string
	Subtitle       string
	Date           string
	Version        string
	Status         string // DRAFT, FINAL, TEMPLATE, ACTIVE
	DocType        string // agreement, policy, internal, plan, technical, report, document
	Style          string // legal, technical, report, minimal, letter
	Summary        string
	Author         string
	Language       string // en, lv
	Classification string // CONFIDENTIAL, INTERNAL, PUBLIC

	// Layout control
	TOC          *bool // nil = auto, true = force on, false = force off
	NoTitlePage  bool
	HasSignatures bool
	FontSize     int    // 10, 11, or 12

	// Custom header/footer
	HeaderLeft  string
	HeaderRight string
	FooterLeft  string
	FooterRight string

	// Custom margins (e.g., "2.5cm")
	MarginTop    string
	MarginBottom string
	MarginLeft   string
	MarginRight  string

	// Logo path (resolved to absolute)
	Logo string

	// Content
	Parties     []string
	Sections    []Section
	RawPreamble string
	Footnotes   map[string]string // footnote id -> text

	// Computed
	GeneratedDate string
	InputDir      string // directory of the input .md file for resolving paths
}

// Section represents a document section with its heading level and content.
type Section struct {
	Level   int
	Title   string
	Content string
}

// parseMarkdown parses a complete markdown document including frontmatter.
func parseMarkdown(md string, inputDir string) Document {
	doc := Document{
		Version:        "1.0",
		Status:         "DRAFT",
		Classification: "CONFIDENTIAL",
		HasSignatures:  false,
		GeneratedDate:  time.Now().Format("2006-01-02"),
		Author:         "",
		FontSize:       11,
		InputDir:       inputDir,
		Footnotes:      make(map[string]string),
	}

	lines := strings.Split(md, "\n")

	// Extract YAML frontmatter if present
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				frontmatter := strings.Join(lines[1:i], "\n")
				parseFrontmatter(frontmatter, &doc)
				lines = lines[i+1:]
				break
			}
		}
	}

	// Extract footnote definitions from body (e.g., [^1]: Some text)
	var bodyLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[^") && strings.Contains(trimmed, "]:") {
			// Parse footnote definition
			idx := strings.Index(trimmed, "]:")
			if idx > 2 {
				id := trimmed[2:idx]
				text := strings.TrimSpace(trimmed[idx+2:])
				doc.Footnotes[id] = text
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	// Parse the body into sections
	var currentSection *Section
	var preambleLines []string
	inSection := false

	for _, line := range bodyLines {
		if strings.HasPrefix(line, "# ") && doc.Title == "" {
			doc.Title = strings.TrimPrefix(line, "# ")
			continue
		}

		level := 0
		if strings.HasPrefix(line, "#### ") {
			level = 4
		} else if strings.HasPrefix(line, "### ") {
			level = 3
		} else if strings.HasPrefix(line, "## ") {
			level = 2
		} else if strings.HasPrefix(line, "# ") {
			level = 1
		}

		if level > 0 {
			if currentSection != nil {
				doc.Sections = append(doc.Sections, *currentSection)
			}
			title := strings.TrimLeft(line, "# ")
			currentSection = &Section{Level: level, Title: title}
			inSection = true
			continue
		}

		if inSection && currentSection != nil {
			currentSection.Content += line + "\n"
		} else {
			preambleLines = append(preambleLines, line)
		}
	}

	if currentSection != nil {
		doc.Sections = append(doc.Sections, *currentSection)
	}

	doc.RawPreamble = strings.Join(preambleLines, "\n")

	if doc.Title == "" {
		doc.Title = "Document"
	}

	// Auto-detect document type from content/title
	if doc.DocType == "" {
		doc.DocType = detectDocType(doc.Title)
	}

	// Auto-detect style from doctype if not explicitly set
	if doc.Style == "" {
		doc.Style = styleFromDocType(doc.DocType)
	}

	// Auto-detect if signatures needed (only for legal-type docs)
	if doc.Style == "legal" {
		titleLower := strings.ToLower(doc.Title)
		if strings.Contains(titleLower, "agreement") ||
			strings.Contains(titleLower, "contract") ||
			strings.Contains(titleLower, "nda") ||
			strings.Contains(titleLower, "ligums") ||
			strings.Contains(titleLower, "līgums") {
			doc.HasSignatures = true
		}
	}

	return doc
}

// shouldShowTOC decides whether to render a table of contents.
func (d Document) ShouldShowTOC() bool {
	if d.TOC != nil {
		return *d.TOC
	}
	// Auto: show TOC if 3+ sections
	return len(d.Sections) >= 3
}

// detectDocType infers document type from the title.
func detectDocType(title string) string {
	t := strings.ToLower(title)
	switch {
	case strings.Contains(t, "agreement") || strings.Contains(t, "līgums") ||
		strings.Contains(t, "nda") || strings.Contains(t, "contract"):
		return "agreement"
	case strings.Contains(t, "policy") || strings.Contains(t, "privacy") ||
		strings.Contains(t, "terms") || strings.Contains(t, "noteikumi"):
		return "policy"
	case strings.Contains(t, "lēmums") || strings.Contains(t, "decision") ||
		strings.Contains(t, "protokol"):
		return "corporate"
	case strings.Contains(t, "spec") || strings.Contains(t, "technical") ||
		strings.Contains(t, "api") || strings.Contains(t, "architecture"):
		return "technical"
	case strings.Contains(t, "report") || strings.Contains(t, "analysis") ||
		strings.Contains(t, "review"):
		return "report"
	case strings.Contains(t, "letter") || strings.Contains(t, "correspondence"):
		return "letter"
	case strings.Contains(t, "plan") || strings.Contains(t, "budget") ||
		strings.Contains(t, "roadmap") || strings.Contains(t, "checklist"):
		return "internal"
	default:
		return "document"
	}
}

// styleFromDocType maps document type to a default style.
func styleFromDocType(docType string) string {
	switch docType {
	case "agreement", "policy", "corporate":
		return "legal"
	case "technical":
		return "technical"
	case "report", "internal":
		return "report"
	case "letter":
		return "letter"
	default:
		return "report"
	}
}

// parseFrontmatter extracts key-value pairs from YAML-like frontmatter.
func parseFrontmatter(fm string, doc *Document) {
	for _, line := range strings.Split(fm, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if val == "" {
			continue
		}
		switch strings.ToLower(key) {
		case "title":
			doc.Title = val
		case "subtitle":
			doc.Subtitle = val
		case "date":
			doc.Date = val
		case "version":
			doc.Version = val
		case "status":
			doc.Status = strings.ToUpper(val)
		case "type":
			doc.DocType = val
		case "style":
			doc.Style = val
		case "summary":
			doc.Summary = val
		case "author":
			doc.Author = val
		case "language", "lang":
			doc.Language = val
		case "classification":
			doc.Classification = strings.ToUpper(val)
		case "signatures":
			doc.HasSignatures = strings.ToLower(val) == "true" || val == "yes"
		case "toc":
			b := strings.ToLower(val) == "true" || val == "yes"
			doc.TOC = &b
		case "logo":
			doc.Logo = val
		case "font-size", "fontsize":
			switch val {
			case "10":
				doc.FontSize = 10
			case "12":
				doc.FontSize = 12
			default:
				doc.FontSize = 11
			}
		case "header-left":
			doc.HeaderLeft = val
		case "header-right":
			doc.HeaderRight = val
		case "footer-left":
			doc.FooterLeft = val
		case "footer-right":
			doc.FooterRight = val
		case "margin-top":
			doc.MarginTop = val
		case "margin-bottom":
			doc.MarginBottom = val
		case "margin-left":
			doc.MarginLeft = val
		case "margin-right":
			doc.MarginRight = val
		}
	}
}
