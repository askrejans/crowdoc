package crowdoc

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/pipeline"
	"github.com/askrejans/crowdoc/v2/internal/support/media"
	mdwriter "github.com/askrejans/crowdoc/v2/internal/writer/markdown"
)

// MarkdownOptions configures Markdown output.
type MarkdownOptions struct {
	// NoFrontmatter omits the YAML frontmatter (title, authors, date, …).
	NoFrontmatter bool
	// MediaDir is the relative directory images are written to ("images"
	// when empty).
	MediaDir string
}

// MarkdownResult is a document rendered as Markdown.
type MarkdownResult struct {
	// Text is the Markdown, ending in one newline.
	Text string
	// Files holds the extracted images: path relative to the Markdown
	// (MediaDir/…) → content.
	Files    map[string][]byte
	Document *ast.Document
	Warnings []string
}

// Markdown reads src and renders it as Markdown that crowdoc reads back
// into the same document, for example to edit it and convert it again.
//
// The document goes through the same preparation as [Convert] (caller
// metadata, normalisation, citations), so the Markdown holds what the PDF
// would. Embedded images (and local images the source refers to) are
// returned in Files and linked relatively, so the Markdown and its files
// form a self-contained directory; remote images stay links. Untrusted
// sources follow the usual rules: no files outside BaseDir, no network.
func Markdown(ctx context.Context, src Source, opts Options, mopts MarkdownOptions) (*MarkdownResult, error) {
	dir, err := cleanMediaDir(mopts.MediaDir)
	if err != nil {
		return nil, err
	}
	p, err := pipeline.Prepare(ctx, src.internal(), opts.internal())
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	doc := p.Doc()
	base := p.BaseDir()
	resolver := media.NewResolver(ctx, doc.Resources, media.Options{
		BaseDir:         base,
		AllowLocalFiles: base != "",
		RestrictToBase:  src.Untrusted,
	})
	res, err := mdwriter.Write(ctx, doc, mdwriter.Options{
		NoFrontmatter: mopts.NoFrontmatter,
		MediaDir:      dir,
		LoadImage:     resolver.Load,
	})
	if err != nil {
		return nil, err
	}
	warns := append(append([]string(nil), p.Warnings()...), res.Warnings...)
	return &MarkdownResult{Text: res.Text, Files: res.Files, Document: doc, Warnings: warns}, nil
}

// cleanMediaDir validates a relative media directory.
func cleanMediaDir(d string) (string, error) {
	d = strings.TrimSpace(strings.ReplaceAll(d, `\`, "/"))
	if d == "" {
		return "images", nil
	}
	c := path.Clean(d)
	if path.IsAbs(c) || c == "." || c == ".." || strings.HasPrefix(c, "../") || filepath.VolumeName(d) != "" {
		return "", fmt.Errorf("media directory %q must be a relative path inside the output directory", d)
	}
	return c, nil
}

// MarkdownFile writes the Markdown for the file at in to out
// ([DefaultMarkdownPath] when empty) and its images next to it.
func MarkdownFile(ctx context.Context, in, out string, opts Options, mopts MarkdownOptions) (*MarkdownResult, error) {
	if out == "" {
		out = DefaultMarkdownPath(in)
	}
	if sameFile(in, out) {
		return nil, fmt.Errorf("output %s would overwrite the input", out)
	}
	res, err := Markdown(ctx, Source{Path: in}, opts, mopts)
	if err != nil {
		return nil, err
	}
	if err := WriteMarkdown(res, out); err != nil {
		return res, err
	}
	return res, nil
}

// WriteMarkdown writes a Markdown result to path and its files relative to
// the directory of path.
func WriteMarkdown(res *MarkdownResult, file string) error {
	dir := filepath.Dir(file)
	names := make([]string, 0, len(res.Files))
	for n := range res.Files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := filepath.Join(dir, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, res.Files[n], 0o644); err != nil {
			return err
		}
	}
	return writeAtomic(file, []byte(res.Text))
}

// DefaultMarkdownPath is the Markdown written for in when no output is
// given: the input name with a .md extension, or name.export.md when the
// input is itself Markdown.
func DefaultMarkdownPath(in string) string {
	ext := filepath.Ext(in)
	base := strings.TrimSuffix(in, ext)
	switch strings.ToLower(ext) {
	case ".md", ".markdown", ".mdown", ".mkd", ".mkdn", ".qmd", ".rmd":
		return base + ".export.md"
	}
	return base + ".md"
}
