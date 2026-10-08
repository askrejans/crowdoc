package rtf

import (
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

func (p *parser) applyInfo() {
	m := &p.doc.Meta
	if m.Title == "" {
		m.Title = p.info["title"]
	}
	m.Subject = p.info["subject"]
	if a := p.info["author"]; a != "" {
		for _, name := range strings.Split(a, ";") {
			if name = strings.TrimSpace(name); name != "" {
				m.Authors = append(m.Authors, ast.Author{Name: name})
			}
		}
	}
	if k := p.info["keywords"]; k != "" {
		for _, kw := range strings.FieldsFunc(k, func(r rune) bool { return r == ',' || r == ';' }) {
			if kw = strings.TrimSpace(kw); kw != "" {
				m.Keywords = append(m.Keywords, kw)
			}
		}
	}
	m.Summary = p.info["doccomm"]
	m.Organization = p.info["company"]
	if y, mo, d := p.infoDate[0], p.infoDate[1], p.infoDate[2]; y > 0 {
		switch {
		case mo >= 1 && mo <= 12 && d >= 1 && d <= 31:
			m.Date = pad4(y) + "-" + pad2(mo) + "-" + pad2(d)
		case mo >= 1 && mo <= 12:
			m.Date = pad4(y) + "-" + pad2(mo)
		default:
			m.Date = pad4(y)
		}
	}
	m.Lang = p.language()
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func pad4(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

// language picks the language covering most of the text.
func (p *parser) language() string {
	best, bestN := 0, 0
	for lang, n := range p.langChars {
		if _, ok := lcidTags[lang]; ok && (n > bestN || n == bestN && lang < best) {
			best, bestN = lang, n
		}
	}
	if bestN == 0 {
		best = p.defLang
		if s := p.styles[0]; s != nil && s.lang != 0 {
			best = s.lang // some writers keep the language on the default style
		}
	}
	return lcidTags[best]
}

// pruneAnchors drops hidden bookmarks (names starting with "_", which word
// processors create for their own bookkeeping) unless something links to them.
func pruneAnchors(blocks []ast.Block, targets map[string]bool) []ast.Block {
	keep := func(ins []ast.Inline) []ast.Inline { return pruneInlines(ins, targets) }
	var visit func([]ast.Block)
	visit = func(bs []ast.Block) {
		for _, b := range bs {
			switch n := b.(type) {
			case *ast.Para:
				n.Inlines = keep(n.Inlines)
			case *ast.Plain:
				n.Inlines = keep(n.Inlines)
			case *ast.Heading:
				n.Inlines = keep(n.Inlines)
			case *ast.Table:
				n.Caption = keep(n.Caption)
			case *ast.Figure:
				n.Caption = keep(n.Caption)
			case *ast.Div:
				n.Title = keep(n.Title)
			case *ast.LineBlock:
				for i := range n.Lines {
					n.Lines[i] = keep(n.Lines[i])
				}
			}
			visit(ast.Children(b))
		}
	}
	visit(blocks)
	return blocks
}

func pruneInlines(ins []ast.Inline, targets map[string]bool) []ast.Inline {
	out := ins[:0]
	for _, in := range ins {
		if id, ok := anchorID(in); ok && strings.HasPrefix(id, "_") && !targets[id] {
			continue
		}
		switch n := in.(type) {
		case *ast.Note:
			n.Blocks = pruneAnchors(n.Blocks, targets)
		case *ast.Strong:
			n.Inlines = pruneInlines(n.Inlines, targets)
		case *ast.Emph:
			n.Inlines = pruneInlines(n.Inlines, targets)
		case *ast.Underline:
			n.Inlines = pruneInlines(n.Inlines, targets)
		case *ast.Strike:
			n.Inlines = pruneInlines(n.Inlines, targets)
		case *ast.Superscript:
			n.Inlines = pruneInlines(n.Inlines, targets)
		case *ast.Subscript:
			n.Inlines = pruneInlines(n.Inlines, targets)
		case *ast.SmallCaps:
			n.Inlines = pruneInlines(n.Inlines, targets)
		case *ast.Highlight:
			n.Inlines = pruneInlines(n.Inlines, targets)
		case *ast.Link:
			n.Inlines = pruneInlines(n.Inlines, targets)
		}
		out = append(out, in)
	}
	return out
}
