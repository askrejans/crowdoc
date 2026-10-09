// Package typst renders an ast.Document as Typst markup and assembles a
// self-contained bundle (main.typ, the shared library, the chosen style and
// image assets) that the Typst compiler turns into PDF.
package typst

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// Asset describes an image prepared for the bundle.
type Asset struct {
	// Path is the bundle-relative path ("assets/img-1.png"); empty when
	// the image is unavailable.
	Path string
	// NaturalWidthPt/HeightPt are the intrinsic size in points (0 = unknown).
	NaturalWidthPt, NaturalHeightPt float64
	// Missing explains why the image could not be used.
	Missing string
}

// MathResult is the result of converting LaTeX math to Typst math.
type MathResult struct {
	Typst    string
	Label    string
	Warnings []string
}

// BibEntry is one formatted bibliography entry.
type BibEntry struct {
	ID      string
	Label   string
	Inlines []ast.Inline
}

// Hooks connect the writer to the rest of the pipeline.
type Hooks struct {
	// Image resolves an image source to a bundle asset.
	Image func(img *ast.Image) Asset
	// Math converts LaTeX math notation to Typst math markup.
	Math func(tex string) MathResult
	// Hyphenate inserts soft hyphens for languages the engine cannot
	// hyphenate itself; nil leaves text untouched.
	Hyphenate func(text string) string
	// Bibliography returns the formatted reference list and whether labels
	// are numeric.
	Bibliography func() (entries []BibEntry, numeric bool, title string)
}

// writer turns AST nodes into Typst markup.
type writer struct {
	sb        strings.Builder
	hooks     Hooks
	labels    map[string]bool
	unsafe    bool
	warnings  []string
	linksNote bool
	bibDone   bool
}

func (w *writer) warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	for _, existing := range w.warnings {
		if existing == msg {
			return
		}
	}
	w.warnings = append(w.warnings, msg)
}

// ---------------------------------------------------------------------------
// Blocks
// ---------------------------------------------------------------------------

func (w *writer) blocks(blocks []ast.Block) string {
	var parts []string
	for _, b := range blocks {
		if s := w.block(b); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (w *writer) block(b ast.Block) string {
	switch n := b.(type) {
	case *ast.Para:
		return w.inlinesAtStart(n.Inlines)
	case *ast.Plain:
		return w.inlinesAtStart(n.Inlines)
	case *ast.Heading:
		return w.heading(n)
	case *ast.CodeBlock:
		return w.codeBlock(n)
	case *ast.MathBlock:
		return w.mathBlock(n)
	case *ast.RawBlock:
		if n.Format == "typst" && w.unsafe {
			return n.Text
		}
		w.warn("raw %s block skipped", n.Format)
		return ""
	case *ast.BlockQuote:
		return "#quote(block: true)[" + w.blocks(n.Blocks) + "]"
	case *ast.List:
		return w.list(n)
	case *ast.DefinitionList:
		return w.definitionList(n)
	case *ast.Table:
		return w.table(n)
	case *ast.Figure:
		return w.figure(n)
	case *ast.HorizontalRule:
		return "#cd-hrule()"
	case *ast.PageBreak:
		return "#pagebreak(weak: true)"
	case *ast.Div:
		return w.div(n)
	case *ast.LineBlock:
		parts := make([]string, 0, len(n.Lines))
		for _, l := range n.Lines {
			parts = append(parts, content(w.inlinesAtStart(l)))
		}
		return "#cd-lines(" + strings.Join(parts, ", ") + ")"
	case *ast.Bibliography:
		return w.bibliography()
	case *ast.ReferenceList:
		entries := make([]BibEntry, 0, len(n.Entries))
		numeric := false
		for _, e := range n.Entries {
			label := e.Label
			if label != "" {
				numeric = true
				label = "[" + label + "]"
			}
			entries = append(entries, BibEntry{ID: e.ID, Label: label, Inlines: e.Inlines})
		}
		return w.bibList(entries, numeric, "", false)
	}
	return ""
}

func (w *writer) heading(h *ast.Heading) string {
	level := h.Level
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}
	body := w.inlines(ast.TrimInlines(h.Inlines), false)
	if h.Unnumbered {
		return fmt.Sprintf("#heading(level: %d, numbering: none)[%s]%s", level, body, labelSuffix(h.Attr.ID))
	}
	return strings.Repeat("=", level) + " " + body + labelSuffix(h.Attr.ID)
}

func (w *writer) codeBlock(cb *ast.CodeBlock) string {
	args := []string{"block: true"}
	if lang := typstLang(cb.Lang); lang != "" {
		args = append(args, "lang: "+str(lang))
	}
	args = append(args, str(strings.TrimRight(cb.Text, "\n")))
	raw := "raw(" + strings.Join(args, ", ") + ")"
	if cb.Attr.HasClass("numberLines") || cb.Attr.HasClass("number-lines") || cb.Attr.HasClass("numberlines") {
		raw = "cd-numbered(" + raw + ")"
	}
	if cb.Attr.HasClass("output") || cb.Attr.HasClass("stderr") || cb.Attr.HasClass("error") {
		return "#cd-output(" + str(strings.TrimRight(cb.Text, "\n")) + ", error: " + boolean(cb.Attr.HasClass("error") || cb.Attr.HasClass("stderr")) + ")"
	}
	if cb.Caption != nil || cb.Attr.ID != "" {
		cap := "none"
		if cb.Caption != nil {
			cap = content(w.inlinesAtStart(cb.Caption))
		}
		return "#figure(" + raw + ", caption: " + cap + ")" + labelSuffix(cb.Attr.ID)
	}
	return "#" + raw
}

// typstLang maps common info-string names to Typst/syntect names.
func typstLang(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch l {
	case "", "text", "plain", "plaintext", "txt", "none", "output", "console-output":
		return ""
	case "shell", "sh", "zsh", "console", "terminal", "shell-session":
		return "bash"
	case "golang":
		return "go"
	case "py", "python3", "ipython", "ipython3":
		return "python"
	case "js", "node", "mjs", "cjs":
		return "javascript"
	case "ts":
		return "typescript"
	case "yml":
		return "yaml"
	case "c++", "cxx", "hpp", "cc":
		return "cpp"
	case "cs", "c#", "csharp":
		return "cs"
	case "rb":
		return "ruby"
	case "rs":
		return "rust"
	case "kt", "kts":
		return "kotlin"
	case "md":
		return "markdown"
	case "ps1", "pwsh":
		return "powershell"
	case "tex", "latex":
		return "latex"
	case "dockerfile", "docker":
		return "dockerfile"
	case "make", "makefile":
		return "makefile"
	case "objc", "objective-c":
		return "objc"
	}
	return l
}

func (w *writer) mathBlock(m *ast.MathBlock) string {
	res := w.math(m.TeX)
	label := m.Label
	if label == "" {
		label = res.Label
	}
	eq := "#math.equation(block: true, alt: " + str(mathAlt(m.TeX)) + ", $ " + res.Typst + " $)"
	if label == "" {
		return eq
	}
	w.labels[label] = true
	return "#[#set math.equation(numbering: \"(1)\")\n" + eq + labelSuffix(label) + "]"
}

func (w *writer) math(tex string) MathResult {
	if w.hooks.Math == nil {
		return MathResult{Typst: `"` + strings.ReplaceAll(strings.ReplaceAll(tex, `\`, `\\`), `"`, `\"`) + `"`}
	}
	res := w.hooks.Math(tex)
	for _, x := range res.Warnings {
		w.warn("math: %s", x)
	}
	if strings.TrimSpace(res.Typst) == "" {
		res.Typst = `""`
	}
	return res
}

func (w *writer) list(l *ast.List) string {
	task := false
	for _, it := range l.Items {
		if it.Task != ast.TaskNone {
			task = true
			break
		}
	}
	if task {
		items := make([]string, 0, len(l.Items))
		for _, it := range l.Items {
			items = append(items, "("+boolean(it.Task == ast.TaskDone)+", "+content(w.itemBody(it.Blocks))+")")
		}
		return "#task-list(" + strings.Join(items, ", ") + ")"
	}
	var args []string
	fn := "list"
	if l.Ordered {
		fn = "enum"
		if l.Start > 1 || l.Start < 0 {
			args = append(args, "start: "+strconv.Itoa(l.Start))
		}
		switch l.Style {
		case ast.NumberLowerAlpha:
			args = append(args, `numbering: "a)"`)
		case ast.NumberUpperAlpha:
			args = append(args, `numbering: "A."`)
		case ast.NumberLowerRoman:
			args = append(args, `numbering: "i."`)
		case ast.NumberUpperRoman:
			args = append(args, `numbering: "I."`)
		}
	}
	args = append(args, "tight: "+boolean(l.Tight))
	for _, it := range l.Items {
		args = append(args, content(w.itemBody(it.Blocks)))
	}
	return "#" + fn + "(" + strings.Join(args, ", ") + ")"
}

func (w *writer) itemBody(blocks []ast.Block) string {
	return w.blocks(blocks)
}

func (w *writer) definitionList(dl *ast.DefinitionList) string {
	items := make([]string, 0, len(dl.Items))
	for _, it := range dl.Items {
		defs := make([]string, 0, len(it.Definitions))
		for _, d := range it.Definitions {
			defs = append(defs, w.blocks(d))
		}
		items = append(items, "terms.item("+content(w.inlinesAtStart(it.Term))+", "+content(strings.Join(defs, "\n\n"))+")")
	}
	return "#terms(" + strings.Join(items, ", ") + ")"
}

var calloutKinds = map[string]bool{
	"note": true, "tip": true, "info": true, "important": true, "warning": true, "caution": true,
	"danger": true, "success": true, "example": true, "abstract": true,
	"theorem": true, "lemma": true, "corollary": true, "proposition": true, "definition": true,
	"remark": true, "proof": true, "exercise": true, "solution": true,
}

func (w *writer) div(d *ast.Div) string {
	body := w.blocks(d.Blocks)
	for _, cl := range d.Attr.Classes {
		switch {
		case calloutKinds[cl]:
			title := "none"
			if len(d.Title) > 0 {
				title = content(w.inlinesAtStart(d.Title))
			}
			return "#callout(kind: " + str(cl) + ", title: " + title + ")[" + body + "]" + labelSuffix(d.Attr.ID)
		case cl == "appendix":
			return "#cd-appendix[" + body + "]"
		case cl == "center":
			return "#align(center)[" + body + "]"
		case cl == "landscape":
			return "#page(flipped: true)[" + body + "]"
		}
	}
	if len(d.Title) > 0 {
		body = "*" + w.inlinesAtStart(d.Title) + "*\n\n" + body
	}
	return body
}

// ---------------------------------------------------------------------------
// Figures and images
// ---------------------------------------------------------------------------

func (w *writer) figure(f *ast.Figure) string {
	if f.Image == nil {
		return ""
	}
	im := *f.Image
	if strings.TrimSpace(im.Alt) == "" && f.Caption != nil {
		im.Alt = strings.TrimSpace(ast.PlainText(f.Caption))
	}
	img := w.imageExpr(&im, true)
	if f.Caption == nil && f.Attr.ID == "" {
		return "#align(center, " + img + ")"
	}
	cap := "none"
	if f.Caption != nil {
		cap = content(w.inlinesAtStart(f.Caption))
	}
	return "#figure(" + img + ", caption: " + cap + ")" + labelSuffix(f.Attr.ID)
}

// imageExpr returns a Typst expression (code mode) for an image.
func (w *writer) imageExpr(img *ast.Image, block bool) string {
	var a Asset
	if w.hooks.Image != nil {
		a = w.hooks.Image(img)
	}
	alt := strings.TrimSpace(img.Alt)
	if alt == "" {
		alt = strings.TrimSpace(img.Title)
	}
	if alt == "" && a.Path != "" {
		alt = imageAltFromName(img.Src)
	}
	if a.Path == "" {
		if a.Missing != "" {
			w.warn("image %q: %s", img.Src, a.Missing)
		}
		label := alt
		if label == "" {
			label = img.Src
		}
		if !block {
			return "box(" + textContent("["+label+"]") + ")"
		}
		return "cd-missing-image(" + textContent(label) + ")"
	}
	args := []string{str(a.Path)}
	width, height := typstLength(img.Width), typstLength(img.Height)
	switch {
	case width != "" || height != "":
		if width != "" {
			args = append(args, "width: "+width)
		}
		if height != "" {
			args = append(args, "height: "+height)
		}
	case !block:
		args = append(args, "height: 1.1em")
	case a.NaturalWidthPt > 0:
		// Natural size, never wider than the column.
		alts := ""
		if alt != "" {
			alts = ", alt: " + str(alt)
		}
		return "cd-image(" + str(a.Path) + ", natural: " + ftoa(a.NaturalWidthPt) + "pt" + alts + ")"
	}
	if alt != "" {
		args = append(args, "alt: "+str(alt))
	}
	expr := "image(" + strings.Join(args, ", ") + ")"
	if !block {
		return "box(" + expr + ", baseline: 0.15em)"
	}
	return expr
}

// typstLength converts CSS-like lengths ("50%", "8cm", "240px") to Typst.
func typstLength(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" || v == "auto" {
		return ""
	}
	num := strings.TrimRight(v, "abcdefghijklmnopqrstuvwxyz%")
	unit := v[len(num):]
	f, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	if err != nil || f <= 0 || math.IsInf(f, 0) {
		return ""
	}
	switch unit {
	case "%":
		if f > 100 {
			f = 100
		}
		return ftoa(f) + "%"
	case "pt", "mm", "cm", "in", "em":
		return ftoa(f) + unit
	case "px", "":
		return ftoa(f*0.75) + "pt"
	case "pc":
		return ftoa(f*12) + "pt"
	case "bp":
		return ftoa(f) + "pt"
	case "vw", "fr":
		return ftoa(math.Min(f, 100)) + "%"
	}
	return ""
}

func ftoa(f float64) string {
	return strconv.FormatFloat(math.Round(f*100)/100, 'f', -1, 64)
}

// ---------------------------------------------------------------------------
// Bibliography
// ---------------------------------------------------------------------------

func (w *writer) bibliography() string {
	if w.bibDone || w.hooks.Bibliography == nil {
		return ""
	}
	entries, numeric, title := w.hooks.Bibliography()
	if len(entries) == 0 {
		return ""
	}
	w.bibDone = true
	return w.bibList(entries, numeric, title, true)
}

func (w *writer) bibList(entries []BibEntry, numeric bool, title string, _ bool) string {
	if len(entries) == 0 {
		return ""
	}
	items := make([]string, 0, len(entries))
	for i, e := range entries {
		id := e.ID
		if id == "" {
			id = "ref-" + strconv.Itoa(i+1)
		}
		w.labels[id] = true
		label := "none"
		if e.Label != "" {
			label = str(e.Label)
		}
		items = append(items, "(id: "+str(id)+", label: "+label+", body: "+content(w.inlinesAtStart(e.Inlines))+")")
	}
	t := "none"
	if title != "" {
		t = textContent(title)
	}
	return "#cd-bibliography(title: " + t + ", numeric: " + boolean(numeric) + ", (" + strings.Join(items, ", ") + ",))"
}

// ---------------------------------------------------------------------------
// Inlines
// ---------------------------------------------------------------------------

func (w *writer) inlinesAtStart(ins []ast.Inline) string { return w.inlines(ins, true) }

// inlines renders inline content. lineStart marks the first text as being at
// the start of a markup line.
func (w *writer) inlines(ins []ast.Inline, lineStart bool) string {
	var sb strings.Builder
	start := lineStart
	callEnded := false
	write := func(s string, isCall bool) {
		if s == "" {
			return
		}
		// An embedded expression continues with ".x", "(…)" or "[…]";
		// terminate it explicitly when such text follows.
		if callEnded {
			r, _ := utf8.DecodeRuneInString(s)
			// A ";" right after a call would be consumed as its terminator.
			if r == '.' || r == '(' || r == '[' || r == ';' {
				sb.WriteByte(';')
			}
		}
		sb.WriteString(s)
		callEnded = isCall
		start = false
	}
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			v := n.Value
			if w.hooks.Hyphenate != nil {
				v = w.hooks.Hyphenate(v)
			}
			write(escapeMarkup(v, start), false)
		case *ast.SoftBreak:
			write(" ", false)
		case *ast.LineBreak:
			write("#linebreak()", true)
		case *ast.Emph:
			w.wrapCall(write, "emph", n.Inlines)
		case *ast.Strong:
			w.wrapCall(write, "strong", n.Inlines)
		case *ast.Strike:
			w.wrapCall(write, "strike", n.Inlines)
		case *ast.Underline:
			w.wrapCall(write, "underline", n.Inlines)
		case *ast.Superscript:
			w.wrapCall(write, "super", n.Inlines)
		case *ast.Subscript:
			w.wrapCall(write, "sub", n.Inlines)
		case *ast.SmallCaps:
			w.wrapCall(write, "smallcaps", n.Inlines)
		case *ast.Highlight:
			w.wrapCall(write, "highlight", n.Inlines)
		case *ast.Code:
			write("#raw("+str(n.Text)+")", true)
		case *ast.Math:
			res := w.math(n.TeX)
			write("#math.equation(alt: "+str(mathAlt(n.TeX))+", $"+strings.TrimSpace(res.Typst)+"$)", true)
		case *ast.Link:
			write(w.link(n), true)
		case *ast.Image:
			write("#"+w.imageExpr(n, false), true)
		case *ast.Note:
			write("#footnote["+w.blocks(n.Blocks)+"]", true)
		case *ast.Cite:
			if n.Rendered != nil {
				write(w.inlines(n.Rendered, true), false)
			} else {
				write(w.inlines(n.Fallback, true), false)
			}
		case *ast.Ref:
			if !w.labels[n.Target] {
				w.warn("unresolved cross-reference %q", n.Target)
				write("#strong[??]", true)
				continue
			}
			if n.Bare {
				write("#cd-ref("+labelRef(n.Target)+", bare: true)", true)
			} else {
				write("#cd-ref("+labelRef(n.Target)+")", true)
			}
		case *ast.Span:
			write(w.span(n), true)
		case *ast.RawInline:
			if n.Format == "typst" && w.unsafe {
				write(n.Text, false)
			}
		}
	}
	return sb.String()
}

// wrapCall writes #fn[…] for formatted inlines. Typst trims spaces at the
// edges of a content block, so leading and trailing spaces are moved
// outside the call.
func (w *writer) wrapCall(write func(string, bool), fn string, ins []ast.Inline) {
	lead, body, trail := splitEdgeSpace(ins)
	if lead {
		write(" ", false)
	}
	if len(body) > 0 {
		write("#"+fn+"["+w.inlines(body, true)+"]", true)
	}
	if trail {
		write(" ", false)
	}
}

func splitEdgeSpace(ins []ast.Inline) (bool, []ast.Inline, bool) {
	trimmed := ast.TrimInlines(append([]ast.Inline(nil), ins...))
	if len(trimmed) == 0 {
		return len(ins) > 0, nil, false
	}
	lead, trail := false, false
	if len(ins) > 0 {
		switch n := ins[0].(type) {
		case *ast.Text:
			lead = strings.TrimLeft(n.Value, " \t\n") != n.Value
		case *ast.SoftBreak:
			lead = true
		}
		switch n := ins[len(ins)-1].(type) {
		case *ast.Text:
			trail = strings.TrimRight(n.Value, " \t\n") != n.Value
		case *ast.SoftBreak:
			trail = true
		}
	}
	return lead, trimmed, trail
}

func (w *writer) span(s *ast.Span) string {
	inner := w.inlines(s.Inlines, true)
	if s.Attr.ID != "" && w.labels[s.Attr.ID] {
		// An anchor (bookmark) that links can point at.
		anchor := "#metadata(none)" + labelSuffix(s.Attr.ID)
		if inner == "" {
			return anchor
		}
		return anchor + "#[" + inner + "]"
	}
	for _, cl := range s.Attr.Classes {
		switch cl {
		case "kbd":
			return "#kbd[" + inner + "]"
		case "smallcaps", "small-caps":
			return "#smallcaps[" + inner + "]"
		case "underline", "ul":
			return "#underline[" + inner + "]"
		case "mark", "highlight":
			return "#highlight[" + inner + "]"
		case "nowrap", "nobreak":
			return "#box[" + inner + "]"
		}
	}
	return "#[" + inner + "]"
}

func (w *writer) link(l *ast.Link) string {
	text := w.inlines(l.Inlines, true)
	u := strings.TrimSpace(l.URL)
	if strings.HasPrefix(u, "#") {
		id := u[1:]
		if w.labels[id] {
			return "#link(" + labelRef(id) + ")[" + text + "]"
		}
		return "#[" + text + "]"
	}
	if u == "" {
		return "#[" + text + "]"
	}
	if text == "" {
		text = escapeMarkup(u, false)
	}
	out := "#link(" + str(u) + ")[" + text + "]"
	if w.linksNote && !strings.HasPrefix(u, "mailto:") && ast.PlainText(l.Inlines) != u {
		out += "#footnote[#link(" + str(u) + ")]"
	}
	return out
}

// mathAlt is the accessible description of an equation: its LaTeX source,
// which screen readers and PDF/UA validators accept as alt text.
func mathAlt(tex string) string {
	t := strings.Join(strings.Fields(tex), " ")
	if t == "" {
		return "equation"
	}
	return t
}

// imageAltFromName turns "figures/soil-map_2024.png" into "soil map 2024",
// the last-resort alt text for an image nobody described.
func imageAltFromName(src string) string {
	name := src
	if i := strings.LastIndexAny(name, "/\\"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.IndexAny(name, "?#"); i >= 0 {
		name = name[:i]
	}
	if i := strings.LastIndex(name, "."); i > 0 {
		name = name[:i]
	}
	name = strings.Join(strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' || r == '.' || r == ' ' }), " ")
	if name == "" || strings.HasPrefix(src, "data:") {
		return "image"
	}
	return name
}
