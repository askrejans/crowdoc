// Package ipynb reads notebook documents (.ipynb in nbformat 4, and the
// older nbformat 3 layout) into the crowdoc AST.
//
// Markdown cells go through the Markdown reader, code cells become code
// blocks and their outputs are rendered by MIME type: images become figures,
// LaTeX becomes display math, HTML (data frame tables) goes through the HTML
// reader and plain text becomes output blocks.
package ipynb

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/html"
	"github.com/askrejans/crowdoc/v2/internal/reader/markdown"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// HTMLFragment converts HTML outputs (data frame tables, rich displays) and
// HTML raw cells into blocks. It is a variable so it can be replaced; when
// nil, HTML outputs fall back to their plain-text representation.
var HTMLFragment = html.Fragment

type notebook struct {
	NBFormat   int               `json:"nbformat"`
	Metadata   json.RawMessage   `json:"metadata"`
	Cells      []json.RawMessage `json:"cells"`
	Worksheets []struct {
		Cells []json.RawMessage `json:"cells"`
	} `json:"worksheets"`
}

type cell struct {
	CellType       string          `json:"cell_type"`
	Source         json.RawMessage `json:"source"`
	Input          json.RawMessage `json:"input"` // nbformat 3 code cells
	Metadata       json.RawMessage `json:"metadata"`
	Outputs        json.RawMessage `json:"outputs"`
	ExecutionCount json.RawMessage `json:"execution_count"`
	PromptNumber   json.RawMessage `json:"prompt_number"` // nbformat 3
	Attachments    json.RawMessage `json:"attachments"`
	Level          json.RawMessage `json:"level"` // nbformat 3 heading cells
}

type cellMeta struct {
	Tags []string `json:"tags"`
	View struct {
		SourceHidden  bool `json:"source_hidden"`
		OutputsHidden bool `json:"outputs_hidden"`
	} `json:"jupyter"`
	HideInput   bool   `json:"hide_input"`
	Format      string `json:"format"`
	RawMimetype string `json:"raw_mimetype"`
	hideCell    bool
	hideInput   bool
	hideOutput  bool
}

type reader struct {
	ctx    context.Context
	doc    *ast.Document
	warn   rd.Warnings
	lang   string
	images map[[32]byte]string
}

// Read parses a notebook document.
func Read(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	var nb notebook
	if err := json.Unmarshal(trimBOM(data), &nb); err != nil {
		return nil, nil, fmt.Errorf("not a valid notebook: %w", err)
	}
	cells := nb.Cells
	if len(cells) == 0 && len(nb.Worksheets) > 0 {
		cells = nb.Worksheets[0].Cells
	}
	if cells == nil && nb.Metadata == nil && nb.NBFormat == 0 {
		return nil, nil, errors.New("not a valid notebook: no cells")
	}
	r := &reader{ctx: ctx, doc: &ast.Document{Resources: ast.NewResources()}}
	r.meta(nb.Metadata)
	for i, raw := range cells {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		var c cell
		if err := json.Unmarshal(raw, &c); err != nil {
			r.warn.Addf("cell %d is malformed and was skipped", i+1)
			continue
		}
		r.doc.Blocks = append(r.doc.Blocks, r.cell(&c, i)...)
	}
	return r.doc, r.warn.List(), nil
}

func trimBOM(b []byte) []byte {
	return []byte(strings.TrimPrefix(string(b), "\ufeff"))
}

// meta fills document metadata from the notebook metadata.
func (r *reader) meta(raw json.RawMessage) {
	var m struct {
		Title        json.RawMessage `json:"title"`
		Authors      json.RawMessage `json:"authors"`
		Author       json.RawMessage `json:"author"`
		Language     string          `json:"language"`
		LanguageInfo struct {
			Name string `json:"name"`
		} `json:"language_info"`
		Kernelspec struct {
			Language string `json:"language"`
			Name     string `json:"name"`
		} `json:"kernelspec"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil {
		// Partial metadata is still useful; decode field by field.
		var loose map[string]json.RawMessage
		if json.Unmarshal(raw, &loose) != nil {
			return
		}
		m.Title, m.Authors, m.Author = loose["title"], loose["authors"], loose["author"]
	}
	r.doc.Meta.Title = strings.TrimSpace(asText(m.Title))
	for _, raw := range []json.RawMessage{m.Authors, m.Author} {
		for _, name := range authorNames(raw) {
			r.doc.Meta.Authors = append(r.doc.Meta.Authors, ast.Author{Name: name})
		}
	}
	lang := m.LanguageInfo.Name
	if lang == "" {
		lang = m.Kernelspec.Language
	}
	if lang == "" {
		lang = m.Language
	}
	if lang == "" {
		lang = languageFromKernel(m.Kernelspec.Name)
	}
	r.lang = strings.ToLower(strings.TrimSpace(lang))
}

func authorNames(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		list = []json.RawMessage{raw}
	}
	var out []string
	for _, item := range list {
		var obj struct {
			Name string `json:"name"`
		}
		name := asText(item)
		if name == "" && json.Unmarshal(item, &obj) == nil {
			name = obj.Name
		}
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func languageFromKernel(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.HasPrefix(n, "python"):
		return "python"
	case n == "ir", strings.HasPrefix(n, "ir"):
		return "r"
	case strings.HasPrefix(n, "julia"):
		return "julia"
	case strings.Contains(n, "scala"):
		return "scala"
	case strings.Contains(n, "javascript"), strings.Contains(n, "deno"):
		return "javascript"
	}
	return ""
}

// asText decodes a notebook string field, which may be a string or a list
// of line strings.
func asText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []string
	if json.Unmarshal(raw, &parts) == nil {
		return strings.Join(parts, "")
	}
	return ""
}

func execCount(c *cell) string {
	for _, raw := range []json.RawMessage{c.ExecutionCount, c.PromptNumber} {
		var n int
		if len(raw) > 0 && json.Unmarshal(raw, &n) == nil && n > 0 {
			return strconv.Itoa(n)
		}
	}
	return ""
}

func parseCellMeta(raw json.RawMessage) cellMeta {
	var m cellMeta
	if len(raw) > 0 && json.Unmarshal(raw, &m) != nil {
		// Tolerate odd metadata: pick the tags if they decode.
		var loose struct {
			Tags []string `json:"tags"`
		}
		_ = json.Unmarshal(raw, &loose)
		m = cellMeta{Tags: loose.Tags}
	}
	for _, t := range m.Tags {
		switch t {
		case "remove-cell", "hide-cell", "remove_cell":
			m.hideCell = true
		case "remove-input", "hide-input", "remove_input":
			m.hideInput = true
		case "remove-output", "hide-output", "remove_output":
			m.hideOutput = true
		}
	}
	if m.View.SourceHidden || m.HideInput {
		m.hideInput = true
	}
	if m.View.OutputsHidden {
		m.hideOutput = true
	}
	return m
}

func (r *reader) cell(c *cell, idx int) []ast.Block {
	m := parseCellMeta(c.Metadata)
	if m.hideCell {
		return nil
	}
	switch c.CellType {
	case "markdown":
		return r.markdown(r.withAttachments(asText(c.Source), c.Attachments, idx))
	case "code":
		return r.code(c, m)
	case "raw":
		return r.raw(asText(c.Source), m)
	case "heading":
		var level int
		_ = json.Unmarshal(c.Level, &level)
		text := strings.TrimSpace(asText(c.Source))
		if text == "" {
			return nil
		}
		return []ast.Block{&ast.Heading{Level: min(max(level, 1), 6), Inlines: ast.Str(text)}}
	}
	return nil
}

func (r *reader) markdown(src string) []ast.Block {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	blocks, warns := markdown.Fragment(r.ctx, src, r.doc.Resources)
	for _, w := range warns {
		r.warn.Addf("%s", w)
	}
	return blocks
}

func (r *reader) html(src string) []ast.Block {
	if HTMLFragment == nil || strings.TrimSpace(src) == "" {
		return nil
	}
	blocks, warns := HTMLFragment(src, r.doc.Resources)
	for _, w := range warns {
		r.warn.Addf("%s", w)
	}
	return blocks
}

// withAttachments stores a markdown cell's attachments as resources and
// points "attachment:name" references at them.
func (r *reader) withAttachments(src string, raw json.RawMessage, idx int) string {
	if len(raw) == 0 || !strings.Contains(src, "attachment:") {
		return src
	}
	var atts map[string]map[string]json.RawMessage
	if json.Unmarshal(raw, &atts) != nil {
		r.warn.Addf("cell %d has malformed attachments", idx+1)
		return src
	}
	for name, bundle := range atts {
		mt, data := pickImage(bundle)
		if data == nil {
			r.warn.Addf("attachment %q has no supported image data", name)
			continue
		}
		ref := r.store("attachments/"+safeName(name), mt, data)
		for _, form := range uniq(name, url.PathEscape(name), strings.ReplaceAll(name, " ", "%20")) {
			src = strings.ReplaceAll(src, "](attachment:"+form, "]("+ref)
			src = strings.ReplaceAll(src, `src="attachment:`+form+`"`, `src="`+ref+`"`)
			src = strings.ReplaceAll(src, `src='attachment:`+form+`'`, `src='`+ref+`'`)
		}
	}
	return src
}

func uniq(xs ...string) []string {
	var out []string
	for _, x := range xs {
		dup := false
		for _, y := range out {
			dup = dup || x == y
		}
		if !dup {
			out = append(out, x)
		}
	}
	return out
}

// safeName makes a resource name usable inside a Markdown link target.
func safeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "image"
	}
	return b.String()
}

var imageTypes = []struct{ mime, ext string }{
	{"image/png", ".png"},
	{"image/jpeg", ".jpg"},
	{"image/gif", ".gif"},
	{"image/svg+xml", ".svg"},
	{"image/webp", ".webp"},
}

// pickImage returns the best image representation in a MIME bundle.
func pickImage(bundle map[string]json.RawMessage) (string, []byte) {
	for _, it := range imageTypes {
		raw, ok := bundle[it.mime]
		if !ok {
			continue
		}
		if data := imageData(it.mime, asText(raw)); data != nil {
			return it.mime, data
		}
	}
	return "", nil
}

// imageData decodes base64 image payloads; SVG is usually stored as text.
func imageData(mime, s string) []byte {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if mime == "image/svg+xml" && strings.HasPrefix(s, "<") {
		return []byte(s)
	}
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	data, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(clean, "="))
	}
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

// store adds an image resource, reusing identical images.
func (r *reader) store(name, mime string, data []byte) string {
	sum := sha256.Sum256(data)
	if src, ok := r.images[sum]; ok {
		return src
	}
	if r.images == nil {
		r.images = map[[32]byte]string{}
	}
	if !strings.Contains(name, ".") {
		for _, it := range imageTypes {
			if it.mime == mime {
				name += it.ext
			}
		}
	}
	src := r.doc.Resources.Add("notebook/"+name, mime, data)
	r.images[sum] = src
	return src
}

func (r *reader) raw(src string, m cellMeta) []ast.Block {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	format := strings.ToLower(m.RawMimetype)
	if format == "" {
		format = strings.ToLower(m.Format)
	}
	switch format {
	case "text/latex", "latex", "application/x-latex":
		return []ast.Block{&ast.RawBlock{Format: "latex", Text: strings.TrimSpace(src)}}
	case "text/markdown", "text/x-markdown", "markdown":
		return r.markdown(src)
	case "text/html", "html":
		if b := r.html(src); len(b) > 0 {
			return b
		}
	}
	return []ast.Block{&ast.CodeBlock{Text: trimCode(src)}}
}

// trimCode drops leading and trailing blank lines but keeps indentation.
func trimCode(s string) string {
	s = strings.TrimRight(s, " \t\r\n")
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 || strings.TrimSpace(s[:i]) != "" {
			return s
		}
		s = s[i+1:]
	}
}
