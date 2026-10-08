package crowdoc

import "github.com/askrejans/crowdoc/v2/internal/pipeline"

// Format identifies an input format.
type Format = pipeline.Format

// Supported input formats.
const (
	FormatMarkdown = pipeline.FormatMarkdown
	FormatText     = pipeline.FormatText
	FormatHTML     = pipeline.FormatHTML
	FormatEPUB     = pipeline.FormatEPUB
	FormatDOCX     = pipeline.FormatDOCX
	FormatODT      = pipeline.FormatODT
	FormatRTF      = pipeline.FormatRTF
	FormatNotebook = pipeline.FormatNotebook
	FormatCSV      = pipeline.FormatCSV
	FormatTSV      = pipeline.FormatTSV
	FormatXLSX     = pipeline.FormatXLSX
	FormatODS      = pipeline.FormatODS
	FormatPDF      = pipeline.FormatPDF
)

// FormatInfo describes an input format.
type FormatInfo = pipeline.FormatInfo

// Formats lists the supported input formats.
func Formats() []FormatInfo { return pipeline.Formats() }

// SupportedExtension reports whether files with this extension can be read.
func SupportedExtension(ext string) bool { return pipeline.SupportedExtension(ext) }

// DetectFormat identifies the format from the file name and content.
// Content wins over a misleading extension for binary containers.
func DetectFormat(name string, data []byte) (Format, error) { return pipeline.DetectFormat(name, data) }
