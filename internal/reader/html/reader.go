// Package html reads HTML documents (web pages, exported articles, converter
// output, notebook HTML outputs) into the crowdoc AST.
//
// The page is parsed with an HTML5 parser; the main content is located
// (<main>, the dominant <article>, else <body> without site chrome) and
// converted structurally. MathML and client-side math renderings become math,
// footnote sections become notes, and data: images become resources.
package html

import (
	"bytes"
	"context"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// Read parses an HTML document.
func Read(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	root, err := ParseDocument(decode(data))
	if err != nil {
		return nil, nil, err
	}
	doc := &ast.Document{Resources: ast.NewResources()}
	fromTitleTag := readMeta(root, &doc.Meta)
	pageStyles(root).Apply(root)

	body := findElem(root, "body", 0)
	if body == nil {
		body = root
	}
	opts := ConvertOptions{Resources: doc.Resources, Limits: o.Limits}
	c := newConv(ctx, opts, true)
	c.ensureIndex(body)
	if tb := titleBlock(body); tb != nil {
		c.readTitleBlock(tb, &doc.Meta)
		c.skipped[tb] = true
	}
	content := contentRoot(body)
	c.page = content == body
	doc.Blocks = c.run(content)
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if fromTitleTag {
		stripTitleSuffix(doc.Blocks, &doc.Meta)
	}
	doc.Blocks = promoteTitle(doc.Blocks, &doc.Meta)
	PruneLinks(doc.Blocks, doc.Meta.Abstract)
	return doc, c.warn.List(), nil
}

// Fragment parses an HTML fragment (raw HTML in Markdown, notebook
// text/html outputs) into blocks; data: images are stored in res.
func Fragment(src string, res *ast.Resources) ([]ast.Block, []string) {
	if res == nil {
		res = ast.NewResources()
	}
	ctxNode := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(src), ctxNode)
	if err != nil {
		nodes, err = html.ParseFragment(strings.NewReader(flattenDeep(src, maxOpen)), ctxNode)
		if err != nil {
			return nil, []string{"html: " + err.Error()}
		}
	}
	for _, n := range nodes {
		ctxNode.AppendChild(n)
	}
	pageStyles(ctxNode).Apply(ctxNode)
	return ReadNode(context.Background(), ctxNode, ConvertOptions{Resources: res})
}

// maxOpen keeps nesting below the parser's limit of 512 open elements.
const maxOpen = 400

// ParseDocument parses an HTML document. Pathologically deep nesting,
// which the HTML parser refuses, is flattened first so the text survives.
func ParseDocument(src string) (*html.Node, error) {
	root, err := html.Parse(strings.NewReader(src))
	if err == nil {
		return root, nil
	}
	return html.Parse(strings.NewReader(flattenDeep(src, maxOpen)))
}

// flattenDeep drops start and end tags nested deeper than max, keeping
// all text.
func flattenDeep(src string, max int) string {
	z := html.NewTokenizer(strings.NewReader(src))
	var sb strings.Builder
	sb.Grow(len(src))
	depth, dropped := 0, 0
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return sb.String()
		case html.StartTagToken:
			if depth >= max {
				dropped++
				continue
			}
			depth++
		case html.EndTagToken:
			if dropped > 0 {
				dropped--
				continue
			}
			if depth > 0 {
				depth--
			}
		}
		sb.Write(z.Raw())
	}
}

// decode returns the document as UTF-8, honouring a BOM or <meta charset>
// when the bytes are not valid UTF-8 (windows-1252 otherwise).
func decode(data []byte) string {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if utf8.Valid(data) {
		return string(data)
	}
	enc, _, _ := charset.DetermineEncoding(data, "text/html")
	out, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return strings.ToValidUTF8(string(data), "\uFFFD")
	}
	return strings.TrimPrefix(string(out), "\uFEFF")
}

// contentRoot picks the element holding the main content.
func contentRoot(body *html.Node) *html.Node {
	if m := findElem(body, "main", 0); m != nil && textLen(m) > 0 {
		return m
	}
	if m := findByRole(body, "main", 0); m != nil && textLen(m) > 0 {
		return m
	}
	var articles []*html.Node
	collectElems(body, "article", &articles, 0)
	var bestA *html.Node
	bestLen := 0
	for _, a := range articles {
		if l := textLen(a); l > bestLen {
			bestA, bestLen = a, l
		}
	}
	// A listing page has many articles; only take one that dominates.
	if bestA != nil && (len(articles) == 1 || bestLen*10 >= textLen(body)*6) {
		return bestA
	}
	return body
}

func findByRole(n *html.Node, role string, depth int) *html.Node {
	if depth > maxDepth {
		return nil
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && hasToken(ch, "role", role) {
			return ch
		}
		if f := findByRole(ch, role, depth+1); f != nil {
			return f
		}
	}
	return nil
}

func collectElems(n *html.Node, tag string, out *[]*html.Node, depth int) {
	if depth > maxDepth {
		return
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, tag) {
			*out = append(*out, ch)
			continue // nested articles are part of the outer one
		}
		collectElems(ch, tag, out, depth+1)
	}
}

// titleBlock finds the title header written by standalone document
// converters (<header id="title-block-header">).
func titleBlock(body *html.Node) *html.Node {
	for ch := body.FirstChild; ch != nil; ch = ch.NextSibling {
		if isElem(ch, "header") && attr(ch, "id") == "title-block-header" {
			return ch
		}
	}
	return nil
}

func (c *conv) readTitleBlock(h *html.Node, m *ast.Meta) {
	var authors []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type != html.ElementNode {
				continue
			}
			text := normSpace(textContent(ch))
			switch {
			case hasClass(ch, "title") && m.Title == "":
				m.Title = text
			case hasClass(ch, "subtitle") && m.Subtitle == "":
				m.Subtitle = text
			case hasClass(ch, "author"):
				authors = append(authors, text)
			case hasClass(ch, "date") && m.Date == "":
				m.Date = text
			case hasClass(ch, "abstract"):
				var body []ast.Block
				for k := ch.FirstChild; k != nil; k = k.NextSibling {
					if hasClass(k, "abstract-title") {
						c.skipped[k] = true
					}
				}
				body = c.blocks(ch, false)
				if len(m.Abstract) == 0 {
					m.Abstract = body
				}
			default:
				walk(ch)
			}
		}
	}
	walk(h)
	if len(m.Authors) == 0 {
		for _, a := range authors {
			if a != "" {
				m.Authors = append(m.Authors, ast.Author{Name: a})
			}
		}
	}
}

var titleSeparators = []string{" | ", " - ", " – ", " — ", " · ", " :: ", " » ", " • ", " / "}

// stripTitleSuffix turns "Article | Site" into "Article" when the first
// top-level heading reads "Article".
func stripTitleSuffix(blocks []ast.Block, m *ast.Meta) {
	var h1 string
	for i, b := range blocks {
		if h, ok := b.(*ast.Heading); ok && h.Level == 1 {
			h1 = normSpace(ast.PlainText(h.Inlines))
			break
		}
		if i > 8 {
			return
		}
	}
	if h1 == "" {
		return
	}
	title := normSpace(m.Title)
	for _, sep := range titleSeparators {
		for i := strings.Index(title, sep); i > 0; {
			if left := strings.TrimSpace(title[:i]); strings.EqualFold(left, h1) {
				m.Title = left
				return
			}
			next := strings.Index(title[i+len(sep):], sep)
			if next < 0 {
				break
			}
			i += len(sep) + next
		}
	}
}

// promoteTitle removes a leading <h1> that repeats the title, or uses it
// as the title when the page has none and it is the only top heading.
func promoteTitle(blocks []ast.Block, m *ast.Meta) []ast.Block {
	if len(blocks) == 0 {
		return blocks
	}
	h, ok := blocks[0].(*ast.Heading)
	if !ok || h.Level != 1 {
		return blocks
	}
	text := ast.PlainText(h.Inlines)
	if m.Title != "" {
		if strings.EqualFold(normSpace(text), normSpace(m.Title)) {
			return blocks[1:]
		}
		return blocks
	}
	for _, b := range blocks[1:] {
		if o, ok := b.(*ast.Heading); ok && o.Level == 1 {
			return blocks
		}
	}
	m.Title = text
	return blocks[1:]
}

// ParseXHTML parses an XHTML document (EPUB content) with the HTML parser.
// XML self-closing tags such as <a id="p1"/> or <title/>, which HTML would
// leave open, are expanded first and CDATA sections become text.
func ParseXHTML(src string) (*html.Node, error) {
	return ParseDocument(expandSelfClosing(src))
}

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "param": true,
	"source": true, "track": true, "wbr": true, "keygen": true, "basefont": true,
	"bgsound": true, "frame": true,
}

func expandSelfClosing(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + len(s)/32)
	i := 0
	for i < len(s) {
		j := strings.IndexByte(s[i:], '<')
		if j < 0 {
			sb.WriteString(s[i:])
			break
		}
		j += i
		sb.WriteString(s[i:j])
		rest := s[j:]
		switch {
		case strings.HasPrefix(rest, "<!--"):
			end := strings.Index(rest[4:], "-->")
			if end < 0 {
				return sb.String()
			}
			i = j + 4 + end + 3
			continue
		case strings.HasPrefix(rest, "<![CDATA["):
			end := strings.Index(rest, "]]>")
			if end < 0 {
				sb.WriteString(html.EscapeString(rest[9:]))
				return sb.String()
			}
			sb.WriteString(html.EscapeString(rest[9:end]))
			i = j + end + 3
			continue
		}
		k := j + 1
		for k < len(s) && isNameByte(s[k]) {
			k++
		}
		name := s[j+1 : k]
		if name == "" {
			sb.WriteByte('<')
			i = j + 1
			continue
		}
		end, quote := k, byte(0)
		for ; end < len(s); end++ {
			c := s[end]
			if quote != 0 {
				if c == quote {
					quote = 0
				}
			} else if c == '"' || c == '\'' {
				quote = c
			} else if c == '>' {
				break
			}
		}
		if end >= len(s) {
			sb.WriteString(rest)
			break
		}
		tag := strings.TrimRight(s[j:end], " \t\r\n")
		local := strings.ToLower(name[strings.LastIndexByte(name, ':')+1:])
		if strings.HasSuffix(tag, "/") && !voidElements[local] {
			sb.WriteString(strings.TrimSuffix(tag, "/") + "></" + name + ">")
		} else {
			sb.WriteString(s[j : end+1])
		}
		i = end + 1
	}
	return sb.String()
}

func isNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == ':' || b == '-' || b == '_' || b == '.'
}
