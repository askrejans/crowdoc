package tex2typ

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// textStyles maps text font commands met inside \text to Typst functions.
var textStyles = map[string]string{
	"textit": "italic", "textsl": "italic", "emph": "italic", "textbf": "bold",
	"textsf": "sans", "texttt": "mono", "textrm": "", "textnormal": "", "textup": "",
	"textmd": "", "text": "", "mbox": "", "hbox": "", "textsc": "",
}

// parseTextArg parses the argument of \text-like commands into strings
// (and math for nested $…$), wrapped in style when given.
func (p *parser) parseTextArg(style string) *node {
	p.skipSpace()
	if p.depth >= maxDepth {
		p.skipItem()
		p.warn("formula nested too deeply; inner content dropped")
		return seq()
	}
	p.depth++
	defer func() { p.depth-- }()
	t := p.peek()
	var content *node
	switch t.kind {
	case tOpen:
		p.next()
		content = p.parseText()
	case tEOF, tClose:
		p.warn("missing argument")
		return seq()
	default:
		var tb textBuilder
		p.textToken(&tb, p.next())
		tb.flush()
		content = seq(tb.items...)
	}
	if style == "" || content.isEmpty() {
		return content
	}
	return call(style, content.unwrap())
}

// textBuilder accumulates text-mode output: runs of characters become one
// string; nested math and styled spans become their own items.
type textBuilder struct {
	buf   strings.Builder
	items []*node
}

func (b *textBuilder) write(s string) { b.buf.WriteString(s) }

func (b *textBuilder) flush() {
	if b.buf.Len() > 0 {
		// Compose accents (\'e → é): not every font positions combining marks.
		b.items = append(b.items, strAtom(norm.NFC.String(b.buf.String())))
		b.buf.Reset()
	}
}

func (b *textBuilder) add(n *node) {
	b.flush()
	if n != nil {
		b.items = append(b.items, n)
	}
}

// parseText reads text up to the '}' matching an already consumed '{'.
func (p *parser) parseText() *node {
	var tb textBuilder
	for depth := 1; ; {
		t := p.next()
		switch t.kind {
		case tEOF:
			p.warn("missing '}' inserted")
			tb.flush()
			return seq(tb.items...)
		case tOpen:
			depth++
			continue
		case tClose:
			if depth--; depth == 0 {
				tb.flush()
				return seq(tb.items...)
			}
			continue
		}
		p.textToken(&tb, t)
	}
}

// textToken converts one text-mode token.
func (p *parser) textToken(tb *textBuilder, t token) {
	switch t.kind {
	case tSpace:
		tb.write(" ")
	case tTilde:
		tb.write(" ")
	case tSup:
		tb.write("^")
	case tSub:
		tb.write("_")
	case tAlign:
		tb.write("&")
	case tHash:
		tb.write("#")
	case tPrime:
		if p.peek().kind == tPrime {
			p.next()
			tb.write("”")
		} else {
			tb.write("’")
		}
	case tOpen, tClose:
		// Braces only group in text; they are handled by the caller.
	case tChar:
		p.textChar(tb, t)
	case tCmd:
		p.textCommand(tb, t.val)
	}
}

func (p *parser) textChar(tb *textBuilder, t token) {
	switch t.r {
	case '-':
		if n := p.peek(); n.kind == tChar && n.r == '-' {
			p.next()
			if n := p.peek(); n.kind == tChar && n.r == '-' {
				p.next()
				tb.write("—")
				return
			}
			tb.write("–")
			return
		}
	case '`':
		if n := p.peek(); n.kind == tChar && n.r == '`' {
			p.next()
			tb.write("“")
			return
		}
		tb.write("‘")
		return
	case '$':
		tb.flush()
		if p.depth >= maxDepth {
			p.warn("formula nested too deeply; inner content dropped")
			return
		}
		p.depth++
		m := p.parseList(stopDollar | stopClose)
		p.depth--
		if n := p.peek(); n.kind == tChar && n.r == '$' {
			p.next()
		}
		tb.add(m)
		return
	}
	tb.write(t.val)
}

func (p *parser) textCommand(tb *textBuilder, name string) {
	if s, ok := textSymbols[name]; ok {
		tb.write(s)
		return
	}
	if mark, ok := textAccents[name]; ok {
		s := []rune(p.rawArg())
		if len(s) > 0 {
			tb.write(string(s[0]) + string(mark) + string(s[1:]))
		}
		return
	}
	if style, ok := textStyles[name]; ok {
		tb.add(p.parseTextArg(style))
		return
	}
	switch name {
	case "(":
		tb.flush()
		if p.depth >= maxDepth {
			p.warn("formula nested too deeply; inner content dropped")
			return
		}
		p.depth++
		m := p.parseList(stopParen | stopClose)
		p.depth--
		if p.peekCmd(")") {
			p.next()
		}
		tb.add(m)
		return
	case "\\", "newline", "linebreak":
		tb.write(" ")
		return
	case "textcolor":
		col := p.parseColor()
		tb.add(colored(col, p.parseTextArg("")))
		return
	case "hspace", "hskip", "kern":
		p.consumeStar()
		if d := p.parseDimen(); d != "" {
			tb.add(codeAtom("#h(" + d + ")"))
		}
		return
	case "label", "ref", "eqref", "footnote", "index", "cite":
		p.rawArg()
		p.warn(`\` + name + ` inside text dropped`)
		return
	}
	if s, ok := symbols[name]; ok {
		tb.write(string(s.ch))
		return
	}
	if !isASCIILetter(name[0]) {
		tb.write(name)
		return
	}
	p.warn(`unknown text command \` + name + ` dropped`)
}
