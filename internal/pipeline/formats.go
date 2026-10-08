package pipeline

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/docx"
	"github.com/askrejans/crowdoc/v2/internal/reader/epub"
	"github.com/askrejans/crowdoc/v2/internal/reader/html"
	"github.com/askrejans/crowdoc/v2/internal/reader/ipynb"
	"github.com/askrejans/crowdoc/v2/internal/reader/markdown"
	"github.com/askrejans/crowdoc/v2/internal/reader/odt"
	"github.com/askrejans/crowdoc/v2/internal/reader/pdf"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
	"github.com/askrejans/crowdoc/v2/internal/reader/rtf"
	"github.com/askrejans/crowdoc/v2/internal/reader/table"
	"github.com/askrejans/crowdoc/v2/internal/reader/text"
)

// Format identifies an input format.
type Format string

// Supported input formats.
const (
	FormatMarkdown Format = "markdown"
	FormatText     Format = "text"
	FormatHTML     Format = "html"
	FormatEPUB     Format = "epub"
	FormatDOCX     Format = "docx"
	FormatODT      Format = "odt"
	FormatRTF      Format = "rtf"
	FormatNotebook Format = "ipynb"
	FormatCSV      Format = "csv"
	FormatTSV      Format = "tsv"
	FormatXLSX     Format = "xlsx"
	FormatODS      Format = "ods"
	FormatPDF      Format = "pdf"
)

// FormatInfo describes an input format.
type FormatInfo struct {
	Format      Format
	Name        string
	Extensions  []string
	Description string
}

type readerFunc func(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error)

type formatEntry struct {
	info FormatInfo
	read readerFunc
}

var formatTable []formatEntry

func registerFormat(info FormatInfo, read readerFunc) {
	formatTable = append(formatTable, formatEntry{info: info, read: read})
}

func init() {
	registerFormat(FormatInfo{FormatMarkdown, "Markdown", []string{".md", ".markdown", ".mdown", ".mkd", ".mdx", ".qmd", ".rmd"},
		"CommonMark/GFM with YAML frontmatter, footnotes, math, citations, cross-references, callouts and tables"}, markdown.Read)
	registerFormat(FormatInfo{FormatDOCX, "Word", []string{".docx", ".docm", ".dotx"},
		"Word documents: styles, lists, tables, images, footnotes, equations, Zotero/Mendeley/Word citations, tracked changes"}, docx.Read)
	registerFormat(FormatInfo{FormatODT, "OpenDocument text", []string{".odt", ".fodt", ".ott"},
		"OpenDocument text: styles, lists, tables, frames with captions, notes, formulas, bibliography marks"}, odt.Read)
	registerFormat(FormatInfo{FormatRTF, "RTF", []string{".rtf"},
		"Rich Text Format: styles, lists, tables, pictures, footnotes, hyperlinks, code pages"}, rtf.Read)
	registerFormat(FormatInfo{FormatHTML, "HTML", []string{".html", ".htm", ".xhtml"},
		"Web pages: main content extraction, tables, figures, footnotes, MathML/KaTeX math, callouts"}, html.Read)
	registerFormat(FormatInfo{FormatEPUB, "EPUB", []string{".epub"},
		"EPUB 2 and 3 books: spine order, images, cross-chapter links, footnotes"}, epub.Read)
	registerFormat(FormatInfo{FormatNotebook, "Jupyter notebook", []string{".ipynb"},
		"Notebooks: Markdown cells, code, outputs (images, tables, LaTeX), attachments"}, ipynb.Read)
	registerFormat(FormatInfo{FormatCSV, "CSV", []string{".csv"},
		"Comma/semicolon/tab/pipe separated values with delimiter and encoding detection"}, table.ReadCSV)
	registerFormat(FormatInfo{FormatTSV, "TSV", []string{".tsv", ".tab"},
		"Tab-separated values"}, table.ReadTSV)
	registerFormat(FormatInfo{FormatXLSX, "Excel", []string{".xlsx", ".xlsm"},
		"Excel workbooks: all visible sheets, number/date formats, merged cells, charts as data, images"}, table.ReadXLSX)
	registerFormat(FormatInfo{FormatODS, "OpenDocument spreadsheet", []string{".ods", ".fods"},
		"OpenDocument spreadsheets: sheets, value types, spans"}, table.ReadODS)
	registerFormat(FormatInfo{FormatText, "Plain text", []string{".txt", ".text"},
		"Plain text with heading, list, code and link detection"}, text.Read)

	registerFormat(FormatInfo{FormatPDF, "PDF", []string{".pdf"},
		"Text-based PDFs: layout reconstruction (columns, headings, lists, tables, footnotes, figures, de-hyphenation)"}, pdf.Read)

	// Raw HTML inside Markdown is converted by the HTML reader.
	markdown.HTMLFragment = html.Fragment
}

// Formats lists the supported input formats.
func Formats() []FormatInfo {
	out := make([]FormatInfo, len(formatTable))
	for i, f := range formatTable {
		out[i] = f.info
	}
	return out
}

func lookupFormat(f Format) (formatEntry, bool) {
	for _, e := range formatTable {
		if e.info.Format == f {
			return e, true
		}
	}
	return formatEntry{}, false
}

// SupportedExtension reports whether files with this extension can be read.
func SupportedExtension(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range formatTable {
		for _, x := range e.info.Extensions {
			if x == ext {
				return true
			}
		}
	}
	return false
}

// DetectFormat identifies the format from the file name and content.
// Content wins over a misleading extension for binary containers.
func DetectFormat(name string, data []byte) (Format, error) {
	if f := sniffFormat(data); f != "" {
		return f, nil
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".xls":
		return "", fmt.Errorf("legacy .xls workbooks are not supported; save the file as .xlsx")
	case ".doc":
		return "", fmt.Errorf("legacy .doc files are not supported; save the file as .docx")
	}
	for _, e := range formatTable {
		for _, x := range e.info.Extensions {
			if x == ext {
				return e.info.Format, nil
			}
		}
	}
	if looksLikeText(data) {
		return FormatMarkdown, nil
	}
	return "", fmt.Errorf("unrecognised input format %q", ext)
}

func sniffFormat(data []byte) Format {
	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	trimmed := bytes.TrimLeft(bytes.TrimPrefix(head, []byte("\xef\xbb\xbf")), " \t\r\n")
	switch {
	case bytes.HasPrefix(data, []byte{0xD0, 0xCF, 0x11, 0xE0}):
		return "" // OLE2 (.doc/.xls) — let the extension produce a clear error
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		return sniffZip(data)
	case bytes.HasPrefix(trimmed, []byte("%PDF-")):
		return FormatPDF
	case bytes.HasPrefix(trimmed, []byte(`{\rtf`)):
		return FormatRTF
	case bytes.HasPrefix(trimmed, []byte("{")) && bytes.Contains(head, []byte(`"cells"`)) && bytes.Contains(data, []byte(`"nbformat"`)):
		return FormatNotebook
	}
	lower := bytes.ToLower(trimmed)
	if bytes.HasPrefix(lower, []byte("<!doctype html")) || bytes.HasPrefix(lower, []byte("<html")) {
		return FormatHTML
	}
	if bytes.HasPrefix(lower, []byte("<?xml")) && bytes.Contains(lower, []byte("office:document")) {
		if bytes.Contains(lower, []byte("office:spreadsheet")) {
			return FormatODS // flat OpenDocument spreadsheet (.fods)
		}
		return FormatODT // flat OpenDocument text (.fodt)
	}
	return ""
}

func sniffZip(data []byte) Format {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	names := map[string]*zip.File{}
	for _, f := range zr.File {
		names[f.Name] = f
	}
	if f, ok := names["mimetype"]; ok {
		if rc, err := f.Open(); err == nil {
			buf := make([]byte, 128)
			n, _ := rc.Read(buf)
			rc.Close()
			mt := strings.TrimSpace(string(buf[:n]))
			switch mt {
			case "application/epub+zip":
				return FormatEPUB
			case "application/vnd.oasis.opendocument.text":
				return FormatODT
			case "application/vnd.oasis.opendocument.spreadsheet":
				return FormatODS
			}
		}
	}
	switch {
	case names["word/document.xml"] != nil:
		return FormatDOCX
	case names["xl/workbook.xml"] != nil:
		return FormatXLSX
	}
	return ""
}

func looksLikeText(data []byte) bool {
	head := data
	if len(head) > 2048 {
		head = head[:2048]
	}
	return !bytes.Contains(head, []byte{0})
}
