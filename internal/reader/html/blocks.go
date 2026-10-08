package html

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
)

var blockTags = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true, "body": true,
	"center": true, "details": true, "dialog": true, "dd": true, "div": true, "dl": true,
	"dt": true, "fieldset": true, "figcaption": true, "figure": true, "footer": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"header": true, "hgroup": true, "hr": true, "html": true, "legend": true, "li": true,
	"listing": true, "main": true, "menu": true, "dir": true, "nav": true, "ol": true,
	"p": true, "plaintext": true, "pre": true, "section": true, "summary": true,
	"table": true, "tbody": true, "thead": true, "tfoot": true, "tr": true, "td": true,
	"th": true, "caption": true, "ul": true, "search": true, "xmp": true,
}

// dropClasses mark navigation, editing and screen-reader-only chrome.
var dropClasses = map[string]bool{
	"mw-editsection": true, "noprint": true, "mw-jump-link": true, "navbox": true,
	"toc": true, "mw-empty-elt": true, "printfooter": true, "catlinks": true,
	"sr-only": true, "visually-hidden": true, "screen-reader-text": true,
	"skip-link": true, "mathjax_preview": true, "mathjax": true, "mathjax_display": true,
	"mathjax_svg": true, "mathjax_svg_display": true, "mathjax_chtml": true,
	"mjx_assistive_mathml": true, "footnotes-sep": true, "headerlink": true,
	"anchorjs-link": true, "mw-cite-backlink": true, "katex-html": true,
}

// skip reports whether n is dropped entirely.
func (c *conv) skip(n *html.Node) bool {
	switch n.Type {
	case html.TextNode:
		return false
	case html.ElementNode:
	default:
		return true
	}
	if c.skipped[n] {
		return true
	}
	switch n.Data {
	case "script":
		return !isMathScript(n)
	case "style", "noscript", "template", "head", "title", "meta", "link", "base",
		"button", "select", "textarea", "option", "optgroup", "datalist", "input",
		"form", "dialog", "map", "area", "param", "source", "track", "frameset",
		"noframes", "nav", "slot", "portal":
		return true
	case "iframe", "object", "embed", "applet", "frame":
		c.warn.Addf("html: embedded <%s> content dropped", n.Data)
		return true
	case "canvas":
		c.warn.Addf("html: <canvas> drawing dropped")
		return true
	case "svg":
		c.warn.Addf("html: inline SVG graphic dropped")
		return true
	case "mjx-container":
		return false
	case "header":
		if c.page && (isElem(n.Parent, "body") || hasToken(n, "role", "banner")) {
			return true
		}
	case "footer":
		if c.page && (isElem(n.Parent, "body") || hasToken(n, "role", "contentinfo")) {
			return true
		}
	case "aside":
		if hasToken(n, "role", "complementary") || hasClass(n, "sidebar") {
			return true
		}
	}
	if v, ok := attrVal(n, "hidden"); ok && v != "until-found" {
		return true
	}
	if strings.EqualFold(attr(n, "aria-hidden"), "true") {
		return true
	}
	if d := styleProp(n, "display"); d == "none" {
		return true
	}
	if v := styleProp(n, "visibility"); v == "hidden" {
		return true
	}
	for _, r := range tokens(n, "role") {
		switch r {
		case "navigation", "search", "doc-toc":
			return true
		}
	}
	for _, t := range epubType(n) {
		switch t {
		case "toc", "landmarks", "page-list", "pagebreak":
			return true
		}
	}
	for _, cl := range classes(n) {
		if dropClasses[cl] {
			return true
		}
		if c.inNote > 0 && (cl == "label" || cl == "fn-label") {
			return true
		}
	}
	if attr(n, "id") == "toc" {
		return true
	}
	if n.Data == "hr" && c.idx.containers[n.Parent] {
		return true
	}
	if id, ok := c.idx.isBody[n]; ok && c.idx.refs[id] {
		return true // emitted as a Note at its reference
	}
	return false
}

func attrVal(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return strings.ToLower(strings.TrimSpace(a.Val)), true
		}
	}
	return "", false
}

// isBlock reports whether n starts block-level content in a flow.
func (c *conv) isBlock(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if blockTags[n.Data] {
		return true
	}
	switch n.Data {
	case "math", "svg", "img", "picture", "video", "audio", "br", "code", "kbd",
		"mjx-container", "script":
		return false
	}
	return c.containsBlock(n)
}

func (c *conv) containsBlock(n *html.Node) bool {
	if v, ok := c.memo[n]; ok {
		return v
	}
	found := false
	for ch := n.FirstChild; ch != nil && !found; ch = ch.NextSibling {
		if ch.Type != html.ElementNode || c.skip(ch) {
			continue
		}
		found = blockTags[ch.Data] || ch.Data != "math" && ch.Data != "svg" && c.containsBlock(ch)
	}
	c.memo[n] = found
	return found
}

// blocks converts the children of parent. plain selects ast.Plain for bare
// inline content (tight list items, table cells).
func (c *conv) blocks(parent *html.Node, plain bool) []ast.Block {
	if c.depth > maxDepth {
		return nil
	}
	c.depth++
	defer func() { c.depth-- }()
	var out []ast.Block
	var buf []ast.Inline
	flush := func() {
		if len(buf) > 0 {
			out = append(out, c.paragraphs(buf, plain)...)
			buf = nil
		}
	}
	for n := parent.FirstChild; n != nil; n = n.NextSibling {
		if c.ctx.Err() != nil {
			break
		}
		if c.skip(n) {
			continue
		}
		if c.isBlock(n) {
			flush()
			out = append(out, c.block(n, plain)...)
			continue
		}
		buf = append(buf, c.inline(n)...)
	}
	flush()
	return mergeCodeParas(out)
}

// mergeCodeParas turns runs of paragraphs written entirely in monospace
// (office exports style code that way) into one code block.
func mergeCodeParas(blocks []ast.Block) []ast.Block {
	var out []ast.Block
	var lines []string
	flush := func() {
		if len(lines) > 0 {
			out = append(out, &ast.CodeBlock{Text: strings.Join(lines, "\n")})
			lines = nil
		}
	}
	for _, b := range blocks {
		if p, ok := b.(*ast.Para); ok {
			if text, ok := codeOnly(p.Inlines); ok {
				lines = append(lines, text)
				continue
			}
		}
		flush()
		out = append(out, b)
	}
	flush()
	return out
}

// codeOnly returns the text of inlines made only of code and spaces.
func codeOnly(ins []ast.Inline) (string, bool) {
	var sb strings.Builder
	code := false
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Code:
			sb.WriteString(strings.ReplaceAll(n.Text, "\u00a0", " "))
			code = true
		case *ast.Text:
			if strings.TrimSpace(n.Value) != "" {
				return "", false
			}
			sb.WriteString(strings.ReplaceAll(n.Value, "\u00a0", " "))
		case *ast.LineBreak:
			sb.WriteByte('\n')
		default:
			return "", false
		}
	}
	return sb.String(), code
}

// paragraphs turns collected inline content into paragraphs, splitting it
// around display math.
func (c *conv) paragraphs(ins []ast.Inline, plain bool) []ast.Block {
	var out []ast.Block
	var seg []ast.Inline
	emit := func() {
		norm := normalize(seg)
		seg = nil
		// Paragraphs of nothing but (non-breaking) spaces are spacers.
		if len(norm) > 0 && (hasContent(norm) || hasAnchor(norm)) {
			out = append(out, c.para(norm, plain))
		}
	}
	for _, in := range ins {
		if m, ok := in.(*ast.Math); ok && c.display[m] {
			emit()
			out = append(out, &ast.MathBlock{TeX: m.TeX})
			continue
		}
		seg = append(seg, in)
	}
	emit()
	return out
}

func (c *conv) para(ins []ast.Inline, plain bool) ast.Block {
	if plain {
		return &ast.Plain{Inlines: ins}
	}
	if img := soleImage(ins); img != nil {
		return &ast.Figure{Image: img}
	}
	if len(ins) == 1 {
		if m, ok := ins[0].(*ast.Math); ok {
			return &ast.MathBlock{TeX: m.TeX}
		}
	}
	return &ast.Para{Inlines: ins}
}

func hasAnchor(ins []ast.Inline) bool {
	for _, in := range ins {
		if sp, ok := in.(*ast.Span); ok && sp.Attr.ID != "" {
			return true
		}
	}
	return false
}

// soleImage returns the image when ins holds nothing else (a linked image
// counts too).
func soleImage(ins []ast.Inline) *ast.Image {
	if len(ins) != 1 {
		return nil
	}
	switch n := ins[0].(type) {
	case *ast.Image:
		return n
	case *ast.Link:
		if len(n.Inlines) == 1 {
			img, _ := n.Inlines[0].(*ast.Image)
			return img
		}
	}
	return nil
}

// nodeBlocks converts a single node as block content.
func (c *conv) nodeBlocks(n *html.Node, plain bool) []ast.Block {
	if c.skip(n) {
		return nil
	}
	if c.isBlock(n) {
		return c.block(n, plain)
	}
	return c.paragraphs(c.inline(n), plain)
}

func (c *conv) block(n *html.Node, plain bool) []ast.Block {
	var out []ast.Block
	ownsID := false
	switch n.Data {
	case "p":
		if kind := calloutKind(n); kind != "" {
			out = c.callout(n, kind)
			break
		}
		out = c.paragraphs(c.inlineChildren(n), false)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		out, ownsID = c.heading(n), true
	case "hr":
		out = []ast.Block{&ast.HorizontalRule{}}
	case "pre", "xmp", "listing", "plaintext":
		out, ownsID = c.codeBlock(n), true
	case "blockquote":
		out = c.blockquote(n)
	case "ul", "ol", "menu", "dir":
		out = c.list(n)
	case "dl":
		out = c.defList(n)
	case "table":
		out, ownsID = c.table(n), true
	case "figure":
		out, ownsID = c.figure(n), true
	case "details":
		out = c.details(n)
	case "address":
		out = c.address(n)
	case "center":
		if bs := c.blocks(n, plain); len(bs) > 0 {
			out = []ast.Block{&ast.Div{Attr: ast.Attr{Classes: []string{"center"}}, Blocks: bs}}
		}
	case "div", "aside", "section":
		if kind := calloutKind(n); kind != "" {
			out = c.callout(n, kind)
			break
		}
		if n.Data == "div" {
			if ins, ok := c.specialSpan(n); ok {
				out = c.paragraphs(ins, plain)
				break
			}
			if isFigureDiv(n) {
				out, ownsID = c.figure(n), true
				break
			}
		}
		out = c.blocks(n, plain)
	default:
		out = c.blocks(n, plain)
	}
	if !ownsID {
		if id := c.keptID(n); id != "" {
			out = attachID(out, id)
		}
	}
	return out
}

// attachID makes id point at the first of blocks.
func attachID(blocks []ast.Block, id string) []ast.Block {
	if len(blocks) == 0 {
		return blocks
	}
	anchor := &ast.Span{Attr: ast.Attr{ID: id}}
	switch b := blocks[0].(type) {
	case *ast.Heading:
		if b.Attr.ID == "" {
			b.Attr.ID = id
		} else {
			b.Inlines = append([]ast.Inline{anchor}, b.Inlines...)
		}
		return blocks
	case *ast.Para:
		b.Inlines = append([]ast.Inline{anchor}, b.Inlines...)
		return blocks
	case *ast.Plain:
		b.Inlines = append([]ast.Inline{anchor}, b.Inlines...)
		return blocks
	case *ast.Table:
		if b.Attr.ID == "" {
			b.Attr.ID = id
			return blocks
		}
	case *ast.Figure:
		if b.Attr.ID == "" {
			b.Attr.ID = id
			return blocks
		}
	case *ast.CodeBlock:
		if b.Attr.ID == "" {
			b.Attr.ID = id
			return blocks
		}
	case *ast.Div:
		if b.Attr.ID == "" {
			b.Attr.ID = id
			return blocks
		}
	}
	return []ast.Block{&ast.Div{Attr: ast.Attr{ID: id}, Blocks: blocks}}
}

func (c *conv) heading(n *html.Node) []ast.Block {
	c.inHeading++
	ins := normalize(c.inlineChildren(n))
	c.inHeading--
	id := c.opts.RewriteID(attr(n, "id"))
	if id == "" {
		id, ins = takeAnchor(ins)
	}
	if len(ins) == 0 {
		return nil
	}
	return []ast.Block{&ast.Heading{Level: int(n.Data[1] - '0'), Inlines: ins, Attr: ast.Attr{ID: id}}}
}

// takeAnchor removes a leading empty anchor span and returns its id.
func takeAnchor(ins []ast.Inline) (string, []ast.Inline) {
	for i, in := range ins {
		sp, ok := in.(*ast.Span)
		if !ok || len(sp.Inlines) > 0 || len(sp.Attr.Classes) > 0 {
			continue
		}
		if sp.Attr.ID != "" {
			out := append(append([]ast.Inline{}, ins[:i]...), ins[i+1:]...)
			return sp.Attr.ID, out
		}
	}
	return "", ins
}

func (c *conv) blockquote(n *html.Node) []ast.Block {
	if kind := calloutKind(n); kind != "" {
		return c.callout(n, kind)
	}
	var attribution []*html.Node
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, "footer", "cite") && !c.skip(ch) {
			attribution = append(attribution, ch)
			c.skipped[ch] = true
		}
	}
	blocks := c.blocks(n, false)
	for _, a := range attribution {
		delete(c.skipped, a)
		var ins []ast.Inline
		if a.Data == "cite" {
			ins = normalize([]ast.Inline{&ast.Emph{Inlines: c.inlineChildren(a)}})
		} else {
			ins = normalize(c.inlineChildren(a))
		}
		if len(ins) == 0 {
			continue
		}
		if t := ast.PlainText(ins); !strings.HasPrefix(t, "—") && !strings.HasPrefix(t, "–") && !strings.HasPrefix(t, "-") && !strings.HasPrefix(t, "―") {
			ins = append([]ast.Inline{&ast.Text{Value: "— "}}, ins...)
		}
		blocks = append(blocks, &ast.Para{Inlines: ins})
	}
	if len(blocks) == 0 {
		return nil
	}
	return []ast.Block{&ast.BlockQuote{Blocks: blocks}}
}

func (c *conv) details(n *html.Node) []ast.Block {
	var title []ast.Inline
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, "summary") {
			title = normalize(c.inlineChildren(ch))
			c.skipped[ch] = true
			defer delete(c.skipped, ch)
			break
		}
	}
	div := &ast.Div{Title: title, Blocks: c.blocks(n, false)}
	if kind := calloutKind(n); kind != "" {
		div.Attr.Classes = []string{kind}
	}
	if len(div.Blocks) == 0 && len(div.Title) == 0 {
		return nil
	}
	return []ast.Block{div}
}

func (c *conv) address(n *html.Node) []ast.Block {
	if c.containsBlock(n) {
		return c.blocks(n, false)
	}
	ins := normalize(c.inlineChildren(n))
	if len(ins) == 0 {
		return nil
	}
	lb := &ast.LineBlock{}
	var line []ast.Inline
	for _, in := range ins {
		if _, ok := in.(*ast.LineBreak); ok {
			lb.Lines = append(lb.Lines, line)
			line = nil
			continue
		}
		line = append(line, in)
	}
	lb.Lines = append(lb.Lines, line)
	return []ast.Block{lb}
}

// codeBlock converts <pre>, keeping whitespace exactly.
func (c *conv) codeBlock(n *html.Node) []ast.Block {
	var sb strings.Builder
	c.preText(&sb, n, 0)
	text := strings.ReplaceAll(sb.String(), "\r\n", "\n")
	text = strings.TrimPrefix(text, "\n")
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}
	cb := &ast.CodeBlock{Lang: codeLang(n), Text: text}
	cb.Attr.ID = c.keptID(n)
	return []ast.Block{cb}
}

var lineNumberClasses = map[string]bool{
	"lineno": true, "linenos": true, "ln": true, "lnt": true, "line-number": true,
	"line-numbers": true, "line-numbers-rows": true, "gutter": true, "linenodiv": true,
	"hljs-ln-numbers": true, "code-line-number": true,
}

func (c *conv) preText(sb *strings.Builder, n *html.Node, depth int) {
	if depth > maxDepth {
		return
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		switch ch.Type {
		case html.TextNode:
			sb.WriteString(ch.Data)
		case html.ElementNode:
			switch ch.Data {
			case "br":
				sb.WriteByte('\n')
				continue
			case "script", "style", "button", "template", "svg":
				continue
			}
			skip := false
			for _, cl := range classes(ch) {
				if lineNumberClasses[cl] {
					skip = true
				}
			}
			if skip || hasAttr(ch, "hidden") {
				continue
			}
			c.preText(sb, ch, depth+1)
			if (ch.Data == "div" || ch.Data == "p") && ch.NextSibling != nil && !strings.HasSuffix(sb.String(), "\n") {
				sb.WriteByte('\n')
			}
		}
	}
}

// codeLang finds a language hint on a <pre>, its <code> child or the
// wrapping highlighter elements.
func codeLang(pre *html.Node) string {
	cands := []*html.Node{pre}
	for ch := pre.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, "code") {
			cands = append([]*html.Node{ch}, cands...)
			break
		}
	}
	for p, i := pre.Parent, 0; p != nil && i < 3; p, i = p.Parent, i+1 {
		if p.Type == html.ElementNode {
			cands = append(cands, p)
		}
	}
	for _, n := range cands {
		if v := cleanLang(attr(n, "data-lang")); v != "" {
			return v
		}
		if v := cleanLang(attr(n, "data-language")); v != "" {
			return v
		}
		if l := langFromClass(attr(n, "class")); l != "" {
			return l
		}
	}
	return ""
}

func langFromClass(class string) string {
	toks := strings.Fields(strings.ToLower(class))
	sourceCode := false
	for i, t := range toks {
		switch {
		case strings.HasPrefix(t, "language-"):
			return cleanLang(t[len("language-"):])
		case strings.HasPrefix(t, "lang-"):
			return cleanLang(t[len("lang-"):])
		case strings.HasPrefix(t, "highlight-source-"):
			return cleanLang(t[len("highlight-source-"):])
		case strings.HasPrefix(t, "highlight-") && t != "highlight-text":
			return cleanLang(t[len("highlight-"):])
		case strings.HasPrefix(t, "brush:"):
			if v := cleanLang(t[len("brush:"):]); v != "" {
				return v
			}
			if i+1 < len(toks) {
				return cleanLang(toks[i+1])
			}
		case t == "sourcecode":
			sourceCode = true
		}
	}
	if sourceCode {
		for _, t := range toks {
			switch t {
			case "sourcecode", "numbersource", "number-lines", "numberlines", "hljs":
				continue
			}
			return cleanLang(t)
		}
	}
	return ""
}

func cleanLang(s string) string {
	s = strings.Trim(strings.ToLower(strings.TrimSpace(s)), ";,")
	if s == "" || s == "none" || s == "plaintext" || s == "text" || s == "nohighlight" || len(s) > 32 {
		return ""
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("+#_.-", r)) {
			return ""
		}
	}
	return s
}

// list converts ul/ol/menu.
func (c *conv) list(n *html.Node) []ast.Block {
	l := &ast.List{Ordered: n.Data == "ol"}
	if l.Ordered {
		if s, err := strconv.Atoi(strings.TrimSpace(attr(n, "start"))); err == nil && s > 0 {
			l.Start = s
		}
		l.Style = numberStyle(attr(n, "type"), styleProp(n, "list-style-type"))
	}
	first := true
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if c.ctx.Err() != nil {
			break
		}
		if c.skip(ch) {
			continue
		}
		if isElem(ch, "li") {
			if first && l.Ordered {
				if v, err := strconv.Atoi(strings.TrimSpace(attr(ch, "value"))); err == nil && v > 0 {
					l.Start = v
				}
			}
			first = false
			l.Items = append(l.Items, c.listItem(ch))
			continue
		}
		if ch.Type == html.TextNode && strings.TrimSpace(ch.Data) == "" {
			continue
		}
		// Stray content between items (often a nested list written as a
		// sibling of <li>) belongs to the previous item.
		bs := c.nodeBlocks(ch, true)
		if len(bs) == 0 {
			continue
		}
		if len(l.Items) == 0 {
			l.Items = append(l.Items, ast.ListItem{})
		}
		last := &l.Items[len(l.Items)-1]
		last.Blocks = append(last.Blocks, bs...)
	}
	if len(l.Items) == 0 {
		return nil
	}
	l.Tight = true
	for _, it := range l.Items {
		for _, b := range it.Blocks {
			if _, ok := b.(*ast.Para); ok {
				l.Tight = false
			}
		}
	}
	return []ast.Block{l}
}

func numberStyle(typ, css string) ast.NumberStyle {
	switch typ {
	case "a":
		return ast.NumberLowerAlpha
	case "A":
		return ast.NumberUpperAlpha
	case "i":
		return ast.NumberLowerRoman
	case "I":
		return ast.NumberUpperRoman
	}
	switch css {
	case "lower-alpha", "lower-latin":
		return ast.NumberLowerAlpha
	case "upper-alpha", "upper-latin":
		return ast.NumberUpperAlpha
	case "lower-roman":
		return ast.NumberLowerRoman
	case "upper-roman":
		return ast.NumberUpperRoman
	}
	return ast.NumberDecimal
}

func (c *conv) listItem(li *html.Node) ast.ListItem {
	item := ast.ListItem{}
	if cb := taskCheckbox(li, 0); cb != nil {
		item.Task = ast.TaskOpen
		if hasAttr(cb, "checked") {
			item.Task = ast.TaskDone
		}
	}
	item.Blocks = c.blocks(li, true)
	return item
}

// taskCheckbox returns a checkbox input that starts the item's content.
func taskCheckbox(n *html.Node, depth int) *html.Node {
	if depth > 3 {
		return nil
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		switch ch.Type {
		case html.TextNode:
			if strings.TrimSpace(ch.Data) != "" {
				return nil
			}
		case html.ElementNode:
			if ch.Data == "input" {
				if strings.EqualFold(attr(ch, "type"), "checkbox") {
					return ch
				}
				return nil
			}
			return taskCheckbox(ch, depth+1)
		}
	}
	return nil
}

func (c *conv) defList(n *html.Node) []ast.Block {
	dl := &ast.DefinitionList{}
	termOpen := false // last item has a term but no definition yet
	var walk func(*html.Node, int)
	walk = func(p *html.Node, depth int) {
		for ch := p.FirstChild; ch != nil; ch = ch.NextSibling {
			if c.skip(ch) || ch.Type != html.ElementNode {
				if ch.Type == html.TextNode && strings.TrimSpace(ch.Data) != "" {
					c.addDefinition(dl, c.paragraphs(c.inline(ch), true))
					termOpen = false
				}
				continue
			}
			switch ch.Data {
			case "dt":
				term := normalize(c.inlineChildren(ch))
				if id := c.keptID(ch); id != "" {
					term = append([]ast.Inline{&ast.Span{Attr: ast.Attr{ID: id}}}, term...)
				}
				if termOpen && len(dl.Items) > 0 {
					last := &dl.Items[len(dl.Items)-1]
					last.Term = append(append(last.Term, &ast.LineBreak{}), term...)
				} else {
					dl.Items = append(dl.Items, ast.DefinitionItem{Term: term})
				}
				termOpen = true
			case "dd":
				c.addDefinition(dl, c.blocks(ch, true))
				termOpen = false
			case "div":
				if depth < 3 {
					walk(ch, depth+1)
					continue
				}
				fallthrough
			default:
				c.addDefinition(dl, c.nodeBlocks(ch, true))
				termOpen = false
			}
		}
	}
	walk(n, 0)
	if len(dl.Items) == 0 {
		return nil
	}
	return []ast.Block{dl}
}

func (c *conv) addDefinition(dl *ast.DefinitionList, blocks []ast.Block) {
	if len(blocks) == 0 {
		return
	}
	if len(dl.Items) == 0 {
		dl.Items = append(dl.Items, ast.DefinitionItem{})
	}
	last := &dl.Items[len(dl.Items)-1]
	last.Definitions = append(last.Definitions, blocks)
}
