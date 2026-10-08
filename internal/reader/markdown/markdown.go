// Package markdown reads CommonMark/GFM Markdown with the Pandoc extensions
// people actually use: YAML frontmatter, footnotes (including ^[inline]
// notes), $math$ and \(math\), citations ([@key, p. 3]), cross-references
// (@fig:x), fenced divs (::: warning), GitHub alerts, attributes
// ({#id .class width=50%}), definition lists, task lists, tables with
// captions, ==highlight== and ^superscript^.
package markdown

import (
	"bytes"
	"context"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gtext "github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
	"github.com/askrejans/crowdoc/v2/internal/transform/metamap"
)

// HTMLFragment converts raw HTML blocks embedded in Markdown. It is wired to
// the HTML reader by the package that registers readers; when nil, HTML
// blocks are reduced to their text.
var HTMLFragment func(src string, res *ast.Resources) ([]ast.Block, []string)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.Linkify,
		extension.TaskList,
		extension.DefinitionList,
		extension.Footnote,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
		parser.WithHeadingAttribute(),
		parser.WithInlineParsers(
			util.Prioritized(mathParser{}, 90),
			util.Prioritized(citeParser{}, 150),
			util.Prioritized(subParser{}, 450),
			util.Prioritized(delimParser{highlightDelim}, 500),
			util.Prioritized(delimParser{superDelim}, 500),
		),
		parser.WithBlockParsers(
			util.Prioritized(divParser{}, 690),
		),
	),
)

// Read parses a Markdown document.
func Read(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	src := DecodeText(data)
	doc := &ast.Document{Resources: ast.NewResources()}
	var warns rd.Warnings

	fm, body, err := metamap.SplitFrontmatter(src)
	if err != nil {
		warns.Addf("%v; the block is treated as text", err)
	}
	if fm == nil {
		body = pandocTitleBlock(body, &doc.Meta)
	} else {
		parse := func(s string) []ast.Block {
			blocks, _ := Fragment(ctx, s, doc.Resources)
			return blocks
		}
		for _, w := range metamap.Apply(&doc.Meta, fm, parse) {
			warns.Addf("%s", w)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	blocks, w := convertSource(ctx, body, doc.Resources)
	for _, x := range w {
		warns.Addf("%s", x)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	doc.Blocks = blocks
	return doc, warns.List(), nil
}

// Fragment parses a Markdown fragment (no frontmatter) into blocks.
// Embedded data: images are stored in res.
func Fragment(ctx context.Context, src string, res *ast.Resources) ([]ast.Block, []string) {
	if res == nil {
		res = ast.NewResources()
	}
	return convertSource(ctx, normalizeNewlines(src), res)
}

func convertSource(ctx context.Context, src string, res *ast.Resources) ([]ast.Block, []string) {
	source := []byte(src)
	root := md.Parser().Parse(gtext.NewReader(source))
	c := newConverter(ctx, source, res)
	blocks := c.document(root)
	return blocks, c.warns.List()
}

// DecodeText turns raw bytes into normalised UTF-8 text: BOMs are removed,
// UTF-16 is decoded, invalid UTF-8 falls back to Windows-1252 and line
// endings become "\n".
func DecodeText(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}), bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		if out, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder().Bytes(data); err == nil {
			data = out
		}
	}
	if !utf8.Valid(data) {
		if out, err := charmap.Windows1252.NewDecoder().Bytes(data); err == nil {
			data = out
		}
	}
	return normalizeNewlines(string(data))
}

func normalizeNewlines(s string) string {
	if !strings.Contains(s, "\r") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// pandocTitleBlock handles the Pandoc "% Title / % Author / % Date" header.
func pandocTitleBlock(body string, m *ast.Meta) string {
	if !strings.HasPrefix(body, "% ") {
		return body
	}
	lines := strings.SplitAfter(body, "\n")
	var fields []string
	i := 0
	for ; i < len(lines) && i < 3; i++ {
		if !strings.HasPrefix(lines[i], "%") {
			break
		}
		fields = append(fields, strings.TrimSpace(strings.TrimPrefix(lines[i], "%")))
	}
	if len(fields) > 0 && fields[0] != "" {
		m.Title = fields[0]
	}
	if len(fields) > 1 && fields[1] != "" {
		for _, a := range strings.Split(fields[1], ";") {
			if a = strings.TrimSpace(a); a != "" {
				m.Authors = append(m.Authors, ast.Author{Name: a})
			}
		}
	}
	if len(fields) > 2 {
		m.Date = fields[2]
	}
	return strings.Join(lines[i:], "")
}
