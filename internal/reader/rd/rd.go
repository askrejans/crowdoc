// Package rd holds the options, limits and helpers shared by all format
// readers. It is a leaf package so readers can import it without cycles.
package rd

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// Options configures a reader.
type Options struct {
	// Name is the source file name; used for titles and format hints.
	Name string
	// Limits bound resource use for untrusted input.
	Limits Limits
}

// Limits bound decompression and resource extraction. Zero values select
// the defaults.
type Limits struct {
	// MaxUnpacked is the total number of bytes a reader may decompress
	// from an archive-based format (DOCX, ODT, XLSX, EPUB).
	MaxUnpacked int64
	// MaxEntry is the largest single archive entry a reader may read.
	MaxEntry int64
	// MaxTableCells caps the number of spreadsheet cells read.
	MaxTableCells int
}

const (
	defaultMaxUnpacked   = 1 << 30   // 1 GiB
	defaultMaxEntry      = 256 << 20 // 256 MiB
	defaultMaxTableCells = 2_000_000
)

// Normalized returns the limits with defaults applied.
func (l Limits) Normalized() Limits {
	if l.MaxUnpacked <= 0 {
		l.MaxUnpacked = defaultMaxUnpacked
	}
	if l.MaxEntry <= 0 {
		l.MaxEntry = defaultMaxEntry
	}
	if l.MaxTableCells <= 0 {
		l.MaxTableCells = defaultMaxTableCells
	}
	return l
}

// ErrLimit is returned when input exceeds a configured limit.
var ErrLimit = errors.New("input exceeds processing limits")

// Warnings collects non-fatal problems found while reading.
type Warnings struct{ list []string }

// Addf records a warning.
func (w *Warnings) Addf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	for _, existing := range w.list {
		if existing == msg {
			return
		}
	}
	w.list = append(w.list, msg)
}

// List returns the recorded warnings.
func (w *Warnings) List() []string { return w.list }

// Zip is a size-limited view of a ZIP-based document container.
type Zip struct {
	r      *zip.Reader
	files  map[string]*zip.File
	lower  map[string]*zip.File
	budget int64
	limits Limits
}

// OpenZip opens data as a ZIP container with the given limits.
func OpenZip(data []byte, limits Limits) (*Zip, error) {
	limits = limits.Normalized()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a valid ZIP container: %w", err)
	}
	z := &Zip{r: r, files: map[string]*zip.File{}, lower: map[string]*zip.File{}, budget: limits.MaxUnpacked, limits: limits}
	for _, f := range r.File {
		name := strings.TrimPrefix(strings.ReplaceAll(f.Name, `\`, "/"), "/")
		z.files[name] = f
		z.lower[strings.ToLower(name)] = f
	}
	return z, nil
}

// Has reports whether the container holds name (case-insensitive fallback).
func (z *Zip) Has(name string) bool {
	_, ok := z.lookup(name)
	return ok
}

// Names returns all entry names.
func (z *Zip) Names() []string {
	out := make([]string, 0, len(z.files))
	for k := range z.files {
		out = append(out, k)
	}
	return out
}

func (z *Zip) lookup(name string) (*zip.File, bool) {
	name = CleanZipPath(name)
	if f, ok := z.files[name]; ok {
		return f, true
	}
	f, ok := z.lower[strings.ToLower(name)]
	return f, ok
}

// Read returns the decompressed bytes of entry name, enforcing limits.
func (z *Zip) Read(name string) ([]byte, error) {
	f, ok := z.lookup(name)
	if !ok {
		return nil, fmt.Errorf("missing entry %q", name)
	}
	if f.UncompressedSize64 > uint64(z.limits.MaxEntry) {
		return nil, fmt.Errorf("%w: entry %q is too large", ErrLimit, name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	max := z.limits.MaxEntry
	if z.budget < max {
		max = z.budget
	}
	data, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%w: decompressed content is too large", ErrLimit)
	}
	z.budget -= int64(len(data))
	return data, nil
}

// CleanZipPath normalises an archive-internal path and resolves "..".
func CleanZipPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	p = path.Clean("/" + p)
	return strings.TrimPrefix(p, "/")
}

// ResolveZipPath resolves target relative to the directory of base (both
// archive-internal), e.g. ("word/document.xml", "media/a.png") ->
// "word/media/a.png". Absolute targets ("/word/x") are taken from the root.
func ResolveZipPath(base, target string) string {
	if strings.HasPrefix(target, "/") {
		return CleanZipPath(target)
	}
	return CleanZipPath(path.Join(path.Dir(base), target))
}

// TitleFromFilename derives a human title from a file name:
// "quarterly_sales-2026.csv" -> "Quarterly Sales 2026".
func TitleFromFilename(name string) string {
	base := filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.NewReplacer("_", " ", "-", " ", ".", " ").Replace(base)
	words := strings.Fields(base)
	for i, w := range words {
		r, size := utf8.DecodeRuneInString(w)
		if r != utf8.RuneError && unicode.IsLower(r) {
			words[i] = string(unicode.ToUpper(r)) + w[size:]
		}
	}
	if len(words) == 0 {
		return "Document"
	}
	return strings.Join(words, " ")
}

// MediaTypeFromName guesses an image media type from a file name.
func MediaTypeFromName(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg", ".jpe", ".jfif":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp", ".dib":
		return "image/bmp"
	case ".tif", ".tiff":
		return "image/tiff"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".emf":
		return "image/emf"
	case ".wmf":
		return "image/wmf"
	case ".heic", ".heif":
		return "image/heic"
	}
	return ""
}

// CleanText normalises whitespace in extracted text: tabs and runs of
// spaces collapse to one space; NBSP is preserved.
func CleanText(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	space := false
	for _, r := range s {
		switch r {
		case '\t', '\n', '\r', ' ', '\f', '\v':
			if !space {
				sb.WriteByte(' ')
				space = true
			}
			continue
		}
		if r == 0 || (r < 0x20) {
			continue
		}
		space = false
		sb.WriteRune(r)
	}
	return sb.String()
}

// Para returns a paragraph block for plain text, or nil when empty.
func Para(text string) ast.Block {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	return &ast.Para{Inlines: ast.Str(text)}
}
