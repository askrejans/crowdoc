// Package markdown writes a document tree as Markdown in the dialect read by
// crowdoc's Markdown reader (CommonMark/GFM with Pandoc extensions), so that
// reading the output back yields the same document.
//
// The output is meant to be edited by people: paragraphs are not wrapped,
// lists, tables and fenced blocks are laid out the way people write them
// and characters are escaped only where the reader would otherwise give
// them a meaning. Constructs Markdown cannot express (row and column spans,
// multi-paragraph table cells) fall back to HTML that the reader converts
// back.
package markdown

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
	mdreader "github.com/askrejans/crowdoc/v2/internal/reader/markdown"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
	"github.com/askrejans/crowdoc/v2/internal/transform/normalize"
)

// Options configures the writer.
type Options struct {
	// NoFrontmatter omits the YAML frontmatter.
	NoFrontmatter bool
	// MediaDir is the relative directory extracted images are written to
	// ("images" when empty). It must be a clean relative path.
	MediaDir string
	// LoadImage returns the content of an image source (a "res:" name, a
	// data: URI or a local path). When it fails or is nil, the image keeps
	// its source. http(s) sources are never loaded.
	LoadImage func(src string) (name string, data []byte, err error)
}

// Result is a rendered document.
type Result struct {
	// Text is the Markdown, ending in exactly one newline.
	Text string
	// Files maps paths relative to the Markdown (MediaDir/…) to the
	// extracted images.
	Files    map[string][]byte
	Warnings []string
}

// Write renders doc as Markdown.
func Write(ctx context.Context, doc *ast.Document, o Options) (*Result, error) {
	if doc == nil {
		doc = &ast.Document{}
	}
	if o.MediaDir == "" {
		o.MediaDir = "images"
	}
	w := &writer{ctx: ctx, doc: doc, o: o, media: newMediaSet(o), warned: map[string]bool{}, needID: map[*ast.Heading]bool{}}

	// Heading identifiers are written only where reading the Markdown back
	// would not produce the same automatic identifier.
	orig := headings(doc.Blocks)
	text := ""
	for round := 0; ; round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		w.reset()
		text = w.document()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if w.allIDs {
			break
		}
		got := w.reparseHeadingIDs(text)
		if len(got) != len(orig) {
			w.allIDs = true
			continue
		}
		changed := false
		for i, h := range orig {
			if h.Attr.ID != "" && got[i] != h.Attr.ID && !w.needID[h] {
				w.needID[h] = true
				changed = true
			}
		}
		if !changed {
			break
		}
		if round >= 3 {
			w.allIDs = true
		}
	}
	return &Result{Text: text, Files: w.media.files, Warnings: w.warnings}, nil
}

type writer struct {
	ctx   context.Context
	doc   *ast.Document
	o     Options
	media *mediaSet

	warnings []string
	warned   map[string]bool

	// needID lists headings whose identifier must be written; allIDs
	// writes every identifier.
	needID map[*ast.Heading]bool
	allIDs bool

	// notes are the footnote bodies of the current note context.
	notes []string
	// htmlNotes numbers footnotes written inside HTML tables.
	htmlNotes int
}

func (w *writer) reset() {
	w.warnings = nil
	w.warned = map[string]bool{}
	w.notes = nil
	w.htmlNotes = 0
	w.media = newMediaSet(w.o)
}

func (w *writer) warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !w.warned[msg] {
		w.warned[msg] = true
		w.warnings = append(w.warnings, "markdown: "+msg)
	}
}

func (w *writer) document() string {
	var sb strings.Builder
	if !w.o.NoFrontmatter {
		sb.WriteString(w.frontmatter())
	}
	body := w.blocksWithNotes(w.doc.Blocks)
	if body != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		} else if strings.HasPrefix(body, "---") {
			// A rule at the very start would open a frontmatter block.
			body = "***" + body[3:]
		}
		sb.WriteString(body)
		sb.WriteString("\n")
	}
	out := sb.String()
	if out == "" {
		return "\n"
	}
	return out
}

// blocksWithNotes renders blocks followed by the definitions of the
// footnotes they reference.
func (w *writer) blocksWithNotes(blocks []ast.Block) string {
	saved := w.notes
	w.notes = nil
	body := w.blocks(blocks, false)
	// Footnote bodies may reference further notes; those are appended as
	// they are rendered.
	var defs []string
	for i := 0; i < len(w.notes); i++ {
		defs = append(defs, w.notes[i])
	}
	w.notes = saved
	if len(defs) == 0 {
		return body
	}
	parts := []string{}
	if body != "" {
		parts = append(parts, body)
	}
	parts = append(parts, defs...)
	return strings.Join(parts, "\n\n")
}

// reparseHeadingIDs reads text back and returns the heading identifiers
// the reader and normaliser assign, in document order.
func (w *writer) reparseHeadingIDs(text string) []string {
	doc, _, err := mdreader.Read(w.ctx, []byte(text), rd.Options{})
	if err != nil {
		return nil
	}
	title := w.doc.Meta.Title
	if title == "" {
		title = "Document"
	}
	normalize.Document(doc, normalize.Options{FallbackTitle: title})
	var ids []string
	for _, h := range headings(doc.Blocks) {
		ids = append(ids, h.Attr.ID)
	}
	return ids
}

func headings(blocks []ast.Block) []*ast.Heading {
	var out []*ast.Heading
	ast.WalkBlocks(blocks, func(b ast.Block) bool {
		if h, ok := b.(*ast.Heading); ok {
			out = append(out, h)
		}
		return true
	})
	return out
}

// ---------------------------------------------------------------------------
// Blocks
// ---------------------------------------------------------------------------

// blocks renders a block sequence separated by blank lines. tight joins
// them with single newlines (tight list items).
func (w *writer) blocks(blocks []ast.Block, tight bool) string {
	var parts []string
	var prev ast.Block
	alt := false
	for _, b := range blocks {
		if w.ctx.Err() != nil {
			break
		}
		// Two lists of the same kind in a row would merge: alternate the
		// marker.
		if l, ok := b.(*ast.List); ok {
			pl, ok := prev.(*ast.List)
			alt = ok && pl.Ordered == l.Ordered && !alt
		}
		s := w.block(b, alt)
		if s == "" {
			continue
		}
		// Two definition lists in a row would merge.
		if _, ok := b.(*ast.DefinitionList); ok {
			if _, ok := prev.(*ast.DefinitionList); ok {
				parts = append(parts, "<!-- -->")
			}
		}
		parts = append(parts, s)
		prev = b
	}
	sep := "\n\n"
	if tight {
		sep = "\n"
	}
	return strings.Join(parts, sep)
}

func (w *writer) block(b ast.Block, alt bool) string {
	switch n := b.(type) {
	case *ast.Para:
		return w.para(n.Inlines)
	case *ast.Plain:
		return w.para(n.Inlines)
	case *ast.Heading:
		return w.heading(n)
	case *ast.CodeBlock:
		return w.codeBlock(n)
	case *ast.MathBlock:
		return w.mathBlock(n)
	case *ast.RawBlock:
		return w.rawBlock(n)
	case *ast.BlockQuote:
		inner := w.blocks(n.Blocks, false)
		return prefixLines(inner, "> ", ">")
	case *ast.List:
		return w.list(n, alt)
	case *ast.DefinitionList:
		return w.definitionList(n)
	case *ast.Table:
		return w.table(n)
	case *ast.Figure:
		return w.figure(n)
	case *ast.HorizontalRule:
		return "---"
	case *ast.PageBreak:
		return `\newpage`
	case *ast.Div:
		return w.div(n)
	case *ast.LineBlock:
		return w.lineBlock(n)
	case *ast.Bibliography:
		return "::: {#refs}\n:::"
	case *ast.ReferenceList:
		return w.referenceList(n)
	case nil:
		return ""
	}
	w.warn("unsupported block %T dropped", b)
	return ""
}

// para renders a paragraph. Paragraphs are never wrapped.
func (w *writer) para(ins []ast.Inline) string {
	// A paragraph holding only an image reads back as a figure; write it
	// as one, with the description as alternative text.
	if t := hoist(ins, 0); len(ast.TrimInlines(t)) == 1 {
		if img, ok := ast.TrimInlines(t)[0].(*ast.Image); ok {
			return w.figure(&ast.Figure{Image: img})
		}
	}
	return w.leaf(ins, ictx{lineStart: true, paraStart: true})
}

// leaf renders the inline content of a leaf block. "\[" starts display
// math when a "\]" follows in the same block, so such blocks escape
// brackets as entities instead.
func (w *writer) leaf(ins []ast.Inline, c ictx) string {
	notes := len(w.notes)
	out := w.inlines(ins, c)
	if i := strings.Index(out, `\[`); i >= 0 && strings.Contains(out[i:], `\]`) {
		w.notes = w.notes[:notes]
		c.bracketEntities = true
		out = w.inlines(ins, c)
	}
	return out
}

func (w *writer) heading(h *ast.Heading) string {
	level := h.Level
	if level < 1 {
		level = 1
	}
	if level > 6 {
		w.warn("heading level %d written as level 6", level)
		level = 6
	}
	var sb strings.Builder
	sb.WriteString(strings.Repeat("#", level))
	text := w.leaf(h.Inlines, ictx{heading: true})
	if text != "" {
		sb.WriteString(" ")
		sb.WriteString(text)
	}
	if h.Unnumbered && !h.Attr.HasClass("unnumbered") {
		sb.WriteString(" {-}")
	}
	a := h.Attr
	if !w.allIDs && !w.needID[h] {
		a.ID = ""
	}
	if attrs := w.attrString(a, attrGoldmark); attrs != "" {
		sb.WriteString(" ")
		sb.WriteString(attrs)
	} else if strings.HasSuffix(sb.String(), "}") {
		// Keep a trailing {…} (image attributes) out of the heading's.
		sb.WriteString(" {}")
	}
	return sb.String()
}

func (w *writer) codeBlock(cb *ast.CodeBlock) string {
	var sb strings.Builder
	if caption := w.leaf(cb.Caption, ictx{caption: true}); caption != "" {
		sb.WriteString("Listing: " + caption + "\n\n")
	}
	text := cleanCode(cb.Text)
	fence := strings.Repeat("`", max(3, longestRun(text, '`')+1))
	sb.WriteString(fence)
	lang := safeWord(cb.Lang)
	if lang == "" && cb.Lang != "" {
		w.warn("code block language %q dropped", cb.Lang)
	}
	sb.WriteString(lang)
	attr := cb.Attr
	if attr.KV != nil {
		attr.KV = copyKV(attr.KV)
		delete(attr.KV, "caption")
	}
	if s := w.attrString(attr, attrPandoc); s != "" {
		if lang != "" {
			sb.WriteString(" ")
		}
		sb.WriteString(s)
	}
	sb.WriteString("\n")
	if text != "" {
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	sb.WriteString(fence)
	return sb.String()
}

func cleanCode(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n")
}

func (w *writer) rawBlock(r *ast.RawBlock) string {
	format := safeWord(strings.ToLower(r.Format))
	if format == "" {
		w.warn("raw block without a format dropped")
		return ""
	}
	text := cleanCode(r.Text)
	fence := strings.Repeat("`", max(3, longestRun(text, '`')+1))
	return fence + "{=" + format + "}\n" + text + "\n" + fence
}

func (w *writer) mathBlock(m *ast.MathBlock) string {
	tex := strings.TrimSpace(cleanCode(m.TeX))
	if tex == "" {
		return ""
	}
	if strings.Contains(tex, "$$") {
		w.warn("display math containing $$ may not read back")
	}
	var sb strings.Builder
	lines := mathLines(tex)
	if startsBlock(lines) {
		sb.WriteString("$$ ") // keep a line such as "# …" off a line start
	} else {
		sb.WriteString("$$\n")
	}
	sb.WriteString(lines)
	sb.WriteString("\n$$")
	if m.Label != "" {
		sb.WriteString(" " + w.attrString(ast.Attr{ID: m.Label}, attrInline))
	}
	return sb.String()
}

// mathLines keeps display math inside one paragraph: blank lines are
// dropped and lines that would start another block are joined to the line
// before.
func mathLines(tex string) string {
	var out []string
	for _, line := range strings.Split(tex, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if len(out) > 0 && startsBlock(t) {
			last := out[len(out)-1]
			if !strings.Contains(last, "%") {
				out[len(out)-1] = last + " " + t
				continue
			}
			t = "{}" + t
		}
		out = append(out, t)
	}
	return strings.Join(out, "\n")
}

// startsBlock reports whether a line could open a block other than a
// paragraph continuation.
func startsBlock(t string) bool {
	switch t[0] {
	case '-', '+', '*', '#', '>', '|', ':', '<', '=', '_', '~', '`', '[':
		return true
	}
	i := 0
	for i < len(t) && i < 10 && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	return i > 0 && i < len(t) && (t[i] == '.' || t[i] == ')')
}

func (w *writer) lineBlock(lb *ast.LineBlock) string {
	if len(lb.Lines) < 2 {
		var ins []ast.Inline
		for _, l := range lb.Lines {
			ins = append(ins, l...)
		}
		return w.para(ins)
	}
	// The lines form one paragraph, so "\[" … "\]" may span them.
	render := func(c ictx) string {
		var lines []string
		for _, l := range lb.Lines {
			// Indentation is written as spaces; the reader keeps it as
			// no-break spaces.
			l = hoist(l, 0)
			indent := 0
			if len(l) > 0 {
				if t, ok := l[0].(*ast.Text); ok {
					rest := strings.TrimLeft(t.Value, " \u00a0")
					indent = utf8.RuneCountInString(t.Value) - utf8.RuneCountInString(rest)
					l = append([]ast.Inline{&ast.Text{Value: rest}}, l[1:]...)
				}
			}
			if s := w.inlines(l, c); s == "" {
				lines = append(lines, "|")
			} else {
				lines = append(lines, "| "+strings.Repeat(" ", indent)+s)
			}
		}
		return strings.Join(lines, "\n")
	}
	notes := len(w.notes)
	out := render(ictx{lineStart: true, lineBlock: true})
	if i := strings.Index(out, `\[`); i >= 0 && strings.Contains(out[i:], `\]`) {
		w.notes = w.notes[:notes]
		out = render(ictx{lineStart: true, lineBlock: true, bracketEntities: true})
	}
	return out
}

func (w *writer) figure(f *ast.Figure) string {
	if f.Image == nil {
		w.warn("figure without an image written as its caption")
		return w.para(f.Caption)
	}
	img := *f.Image
	img.Attr.ID = f.Attr.ID
	if len(f.Attr.Classes) > 0 && len(img.Attr.Classes) == 0 {
		img.Attr.Classes = f.Attr.Classes
	}
	caption := f.Caption
	plainCap := ast.PlainText(caption)
	alt := img.Alt
	if len(caption) == 0 && alt != "" || alt != "" && alt != plainCap {
		img.Attr.KV = copyKV(img.Attr.KV)
		img.Attr.KV["fig-alt"] = alt
	}
	c := ictx{lineStart: true, paraStart: true}
	notes := len(w.notes)
	s := w.image(&img, caption, true, c)
	if i := strings.Index(s, `\[`); i >= 0 && strings.Contains(s[i:], `\]`) {
		w.notes = w.notes[:notes]
		c.bracketEntities = true
		s = w.image(&img, caption, true, c)
	}
	return s
}

func (w *writer) referenceList(r *ast.ReferenceList) string {
	var parts []string
	for _, e := range r.Entries {
		ins := e.Inlines
		if e.Label != "" {
			ins = append([]ast.Inline{&ast.Text{Value: "[" + e.Label + "] "}}, ins...)
		}
		if s := w.para(ins); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

// ---------------------------------------------------------------------------
// Lists
// ---------------------------------------------------------------------------

func (w *writer) list(l *ast.List, alt bool) string {
	tight := l.Tight && listCanBeTight(l)
	if l.Ordered && l.Style != ast.NumberDecimal {
		w.warn("list numbering style written as decimal numbers")
	}
	start := l.Start
	if start < 1 {
		start = 1
	}
	if start > 999999000 {
		start = 999999000
	}
	var items []string
	marked := false
	for i, it := range l.Items {
		marker := "-"
		if alt {
			marker = "*"
		}
		if l.Ordered {
			sep := "."
			if alt {
				sep = ")"
			}
			marker = fmt.Sprintf("%d%s", start+i, sep)
		}
		item := w.listItem(it, marker, tight)
		if !tight && !marked && hasFigure(it.Blocks) {
			// The reader may take a list as tight (a lone item, a blank
			// line only before a table), and figures in tight lists read
			// as images: a comment after a blank line keeps it loose.
			item += "\n\n" + strings.Repeat(" ", len(marker)+1) + "<!-- -->"
			marked = true
		}
		items = append(items, item)
	}
	sep := "\n\n"
	if tight {
		sep = "\n"
	}
	return strings.Join(items, sep)
}

// listCanBeTight reports whether every item can be written without blank
// lines: one block, or a paragraph followed by one block that may
// interrupt it.
func listCanBeTight(l *ast.List) bool {
	for _, it := range l.Items {
		for _, b := range it.Blocks {
			if _, ok := b.(*ast.Figure); ok {
				return false // a tight item holds an image, not a figure
			}
		}
		switch len(it.Blocks) {
		case 0, 1:
			continue
		case 2:
			if !isText(it.Blocks[0]) {
				return false
			}
			switch n := it.Blocks[1].(type) {
			case *ast.List:
				if n.Ordered && n.Start > 1 {
					return false
				}
			case *ast.CodeBlock:
				if len(n.Caption) > 0 {
					return false // the caption paragraph needs a blank line
				}
			case *ast.BlockQuote:
			default:
				return false
			}
		default:
			return false
		}
	}
	return true
}

func hasFigure(blocks []ast.Block) bool {
	for _, b := range blocks {
		if _, ok := b.(*ast.Figure); ok {
			return true
		}
	}
	return false
}

func firstBlock(blocks []ast.Block) ast.Block {
	if len(blocks) == 0 {
		return nil
	}
	return blocks[0]
}

func isText(b ast.Block) bool {
	switch b.(type) {
	case *ast.Para, *ast.Plain:
		return true
	}
	return false
}

func (w *writer) listItem(it ast.ListItem, marker string, tight bool) string {
	blocks := it.Blocks
	for len(blocks) > 0 && isText(blocks[0]) && len(ast.TrimInlines(hoist(blockInlines(blocks[0]), 0))) == 0 {
		blocks = blocks[1:] // empty paragraphs are not written
	}
	task := ""
	switch it.Task {
	case ast.TaskOpen:
		task = "[ ] "
	case ast.TaskDone:
		task = "[x] "
	}
	if _, fig := firstBlock(blocks).(*ast.Figure); task != "" && !fig && (len(blocks) == 0 || !isText(blocks[0])) {
		w.warn("task list checkbox dropped from an item that does not start with text")
		task = ""
	}
	body := w.blocks(blocks, tight)
	if task != "" {
		body = task + body
	}
	// A rule as the first block would read as a rule made of the marker.
	if strings.HasPrefix(body, "---") && (marker == "-" || marker == "*") {
		body = "***" + body[3:]
		if marker == "*" {
			body = "___" + body[3:]
		}
	}
	if body == "" {
		// An empty item followed by a blank line would end every
		// enclosing block in the reader.
		return marker + " &nbsp;"
	}
	pad := strings.Repeat(" ", len(marker)+1)
	return marker + " " + indentRest(body, pad)
}

// ---------------------------------------------------------------------------
// Definition lists
// ---------------------------------------------------------------------------

func (w *writer) definitionList(dl *ast.DefinitionList) string {
	var items []string
	for i, it := range dl.Items {
		term := w.leaf(dropBreaks(it.Term), ictx{lineStart: true, paraStart: true, oneLine: true})
		if term == "" {
			term = "&nbsp;"
			w.warn("empty definition term written as a no-break space")
		}
		tight := true
		for _, d := range it.Definitions {
			if len(d) > 1 || len(d) == 1 && !isPlain(d[0]) {
				tight = false
			}
		}
		var sb strings.Builder
		sb.WriteString(term)
		for _, d := range it.Definitions {
			body := w.blocks(d, false)
			if tight {
				sb.WriteString("\n")
			} else {
				sb.WriteString("\n\n")
			}
			if body == "" {
				sb.WriteString(": &nbsp;")
				continue
			}
			sb.WriteString(": ")
			sb.WriteString(indentRest(body, "  "))
		}
		if len(it.Definitions) == 0 && i == len(dl.Items)-1 {
			sb.WriteString("\n: &nbsp;") // a term needs a definition
		}
		items = append(items, sb.String())
	}
	// A term without definitions shares the next term's (the terms are
	// written on consecutive lines).
	var sb strings.Builder
	for i, s := range items {
		if i > 0 {
			if len(dl.Items[i-1].Definitions) == 0 {
				sb.WriteString("\n")
			} else {
				sb.WriteString("\n\n")
			}
		}
		sb.WriteString(s)
	}
	return sb.String()
}

func isPlain(b ast.Block) bool {
	_, ok := b.(*ast.Plain)
	return ok
}

func dropBreaks(ins []ast.Inline) []ast.Inline {
	out := make([]ast.Inline, 0, len(ins))
	for _, in := range ins {
		switch in.(type) {
		case *ast.LineBreak, *ast.SoftBreak:
			out = append(out, &ast.Text{Value: " "})
		default:
			out = append(out, in)
		}
	}
	return ast.MergeText(out)
}

// ---------------------------------------------------------------------------
// Divs
// ---------------------------------------------------------------------------

func (w *writer) div(d *ast.Div) string {
	attr := d.Attr
	if attr.KV != nil {
		attr.KV = copyKV(attr.KV)
		delete(attr.KV, "title")
	}
	simple := len(attr.Classes) == 1 && attr.ID == "" && len(attr.KV) == 0 && simpleClass(attr.Classes[0])
	title, titleAttr := "", ""
	titleHeading := ""
	if plain, _ := plainInlines(d.Title); len(d.Title) > 0 && (plain != "" || len(ast.TrimInlines(d.Title)) > 0 && strings.TrimSpace(ast.PlainText(d.Title)) != "") {
		plain, isPlain := plainInlines(d.Title)
		if !isPlain && !isCallout(attr) {
			w.warn("formatting dropped from a box title")
			plain, isPlain = strings.Join(strings.Fields(ast.PlainText(d.Title)), " "), true
		}
		switch {
		case !isPlain:
			titleHeading = "#### " + w.leaf(d.Title, ictx{heading: true})
		case plainTitleOK(plain) && (simple || !strings.Contains(plain, "}") && !strings.HasPrefix(plain, "{")):
			title = plain
		case !strings.Contains(plain, "\n") && !(strings.Contains(plain, `"`) && strings.Contains(plain, "'")):
			titleAttr = plain
			simple = false
		case isCallout(attr):
			titleHeading = "#### " + w.leaf(d.Title, ictx{heading: true})
		default:
			w.warn("box title %q dropped", plain)
		}
	}
	var spec string
	if simple {
		spec = attr.Classes[0]
	} else {
		spec = w.attrString(attr, attrPandoc)
		if titleAttr != "" {
			// Written last: the value may hold braces.
			q := `"`
			if strings.Contains(titleAttr, `"`) {
				q = "'"
			}
			t := "title=" + q + titleAttr + q
			if spec == "" {
				spec = "{" + t + "}"
			} else {
				spec = spec[:len(spec)-1] + " " + t + "}"
			}
		}
		if spec == "" {
			spec = "{}"
		}
	}
	if title != "" {
		spec += " " + title
	}
	body := w.blocks(d.Blocks, false)
	if title == "" && titleHeading == "" && titleAttr == "" && isCallout(attr) && strings.HasPrefix(body, "#") {
		// Keep a leading heading out of the title position.
		body = "<!-- -->\n\n" + body
	}
	if titleHeading != "" {
		if body != "" {
			body = titleHeading + "\n\n" + body
		} else {
			body = titleHeading
		}
	}
	fence := strings.Repeat(":", 3+divDepth(d.Blocks))
	if body == "" {
		return fence + " " + spec + "\n" + fence
	}
	return fence + " " + spec + "\n" + body + "\n" + fence
}

// divDepth is the nesting depth of divs below blocks, so outer fences can
// be written longer than inner ones.
func divDepth(blocks []ast.Block) int {
	d := 0
	for _, b := range blocks {
		switch n := b.(type) {
		case *ast.Div:
			d = max(d, 1+divDepth(n.Blocks))
		case *ast.Bibliography:
			d = max(d, 1)
		default:
			d = max(d, divDepth(ast.Children(b)))
		}
	}
	return d
}

func isCallout(a ast.Attr) bool {
	for _, cl := range a.Classes {
		switch cl {
		case "note", "tip", "info", "important", "warning", "caution", "danger", "success", "example":
			return true
		}
	}
	return false
}

// plainTitleOK reports whether a title can follow the fence attributes as
// is: the reader takes the rest of the line verbatim (after the last "}")
// and drops trailing colons.
func plainTitleOK(s string) bool {
	return s != "" && !strings.Contains(s, "\n") && strings.TrimSpace(s) == s && !fenceTailRe.MatchString(" "+s)
}

// fenceTailRe matches the colons the reader drops from a fence line.
var fenceTailRe = regexp.MustCompile(`(?:\s+:+|:{3,})$`)

// simpleClass reports whether c can be written as "::: c": the reader
// lower-cases the word and trims braces and dots from its ends.
func simpleClass(c string) bool {
	return c != "" && c == strings.ToLower(c) && strings.Trim(c, "{}.") == c && !strings.ContainsAny(c, " \t\n\r") && !strings.HasPrefix(c, ":")
}

// plainInlines returns the text of inlines made only of text and spaces.
func plainInlines(ins []ast.Inline) (string, bool) {
	var sb strings.Builder
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			sb.WriteString(n.Value)
		case *ast.SoftBreak:
			sb.WriteByte(' ')
		default:
			return "", false
		}
	}
	return strings.TrimSpace(sb.String()), true
}

// ---------------------------------------------------------------------------
// Line helpers
// ---------------------------------------------------------------------------

// indentRest indents every line but the first; empty lines stay empty.
func indentRest(s, pad string) string {
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = pad + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// prefixLines prefixes every line; empty lines get emptyPrefix.
func prefixLines(s, prefix, emptyPrefix string) string {
	if s == "" {
		return emptyPrefix
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l == "" {
			lines[i] = emptyPrefix
		} else {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}

func longestRun(s string, c byte) int {
	best, cur := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			cur++
			best = max(best, cur)
		} else {
			cur = 0
		}
	}
	return best
}

func copyKV(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
