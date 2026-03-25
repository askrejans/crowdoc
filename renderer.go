package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// renderAndCompile generates LaTeX from the document and compiles it to PDF.
// customTemplatePath is optional; if non-empty, it overrides the built-in style template.
func renderAndCompile(doc Document, outputPath string, customTemplatePath ...string) error {
	tmplPath := ""
	if len(customTemplatePath) > 0 {
		tmplPath = customTemplatePath[0]
	}
	latex := renderLaTeX(doc, tmplPath)

	tmpDir, err := os.MkdirTemp("", "crowdoc-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if os.Getenv("CROWDOC_KEEP_TEX") == "" {
			os.RemoveAll(tmpDir)
		} else {
			fmt.Fprintf(os.Stderr, "crowdoc: temp dir kept at %s\n", tmpDir)
		}
	}()

	texPath := filepath.Join(tmpDir, "document.tex")
	if err := os.WriteFile(texPath, []byte(latex), 0644); err != nil {
		return fmt.Errorf("failed to write .tex file: %w", err)
	}

	// Compile with LuaLaTeX (two passes for TOC and references)
	if err := compileLaTeX(texPath, tmpDir); err != nil {
		return err
	}

	pdfPath := filepath.Join(tmpDir, "document.pdf")
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read generated PDF: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, pdfBytes, 0644); err != nil {
		return fmt.Errorf("failed to write output PDF: %w", err)
	}

	return nil
}

// renderLaTeX executes the style template with the document data.
// If customTemplatePath is non-empty, it reads that file instead of using the built-in style.
func renderLaTeX(doc Document, customTemplatePath ...string) string {
	var tmplStr string
	if len(customTemplatePath) > 0 && customTemplatePath[0] != "" {
		data, err := os.ReadFile(customTemplatePath[0])
		if err != nil {
			fatal("failed to read custom template %s: %v", customTemplatePath[0], err)
		}
		tmplStr = string(data)
	} else {
		tmplStr = getStyleTemplate(doc.Style)
	}

	funcMap := template.FuncMap{
		"escapeLaTeX": escapeLaTeX,
		"mdToLaTeX": func(s string) string {
			return mdToLaTeX(s, doc)
		},
		"sectionCmd":  sectionCmd,
		"hasContent":  func(s string) bool { return strings.TrimSpace(s) != "" },
		"statusColor": statusColor,
		"classIcon":   classIcon,
		"or": func(a, b string) string {
			if a != "" {
				return a
			}
			return b
		},
		"printf": fmt.Sprintf,
	}

	tmpl, err := template.New("latex").Delims("<<", ">>").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		fatal("template parse error: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, doc); err != nil {
		fatal("template execution failed: %v", err)
	}
	return buf.String()
}

// compileLaTeX runs LuaLaTeX on the .tex file, falling back to XeLaTeX.
func compileLaTeX(texPath, tmpDir string) error {
	engines := []string{"lualatex", "xelatex"}

	for _, engine := range engines {
		if _, err := exec.LookPath(engine); err != nil {
			continue
		}

		success := true
		var lastOutput string

		// Two passes for TOC and cross-references
		for pass := 0; pass < 2; pass++ {
			cmd := exec.Command(engine,
				"-interaction=nonstopmode",
				"-halt-on-error",
				"-output-directory="+tmpDir,
				texPath,
			)
			var out bytes.Buffer
			cmd.Stderr = &out
			cmd.Stdout = &out
			if err := cmd.Run(); err != nil {
				lastOutput = out.String()
				success = false
				break
			}
		}

		if success {
			return nil
		}

		// If first engine failed, try next
		if engine == engines[len(engines)-1] {
			// Extract the most useful error line from LaTeX output
			errMsg := extractLaTeXError(lastOutput)
			return fmt.Errorf("LaTeX compilation failed with %s:\n%s\n\nFull log saved to temp directory. Install MacTeX: brew install --cask mactex-no-gui", engine, errMsg)
		}
	}

	return fmt.Errorf("no LaTeX engine found. Install MacTeX: brew install --cask mactex-no-gui")
}

// extractLaTeXError pulls out the most relevant error lines from LaTeX output.
func extractLaTeXError(output string) string {
	lines := strings.Split(output, "\n")
	var errLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "!") || strings.Contains(line, "Error") ||
			strings.Contains(line, "Undefined control sequence") ||
			strings.Contains(line, "Missing") {
			errLines = append(errLines, line)
			if len(errLines) >= 5 {
				break
			}
		}
	}
	if len(errLines) == 0 {
		// Return last 10 lines as fallback
		start := len(lines) - 10
		if start < 0 {
			start = 0
		}
		return strings.Join(lines[start:], "\n")
	}
	return strings.Join(errLines, "\n")
}
