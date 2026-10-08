package cite

import (
	"strings"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// builder accumulates formatted inlines. It knows the last character
// written so separators never double up ("Title?." → "Title?") and, for
// American typography, moves periods and commas inside closing quotes.
type builder struct {
	out []ast.Inline
	buf strings.Builder

	last rune // last rune written, 0 when empty

	// quoteClose is the closing quotation mark that currently ends buf;
	// inside is the last rune before it.
	quoteClose string
	inside     rune

	moveInside bool
}

func (b *builder) flush() {
	if b.buf.Len() > 0 {
		b.out = append(b.out, &ast.Text{Value: b.buf.String()})
		b.buf.Reset()
	}
}

// text appends plain text.
func (b *builder) text(s string) {
	if s == "" {
		return
	}
	b.buf.WriteString(s)
	r, _ := utf8.DecodeLastRuneInString(s)
	b.last = r
	b.quoteClose = ""
}

// space appends a single space unless the output is empty or already ends
// with whitespace.
func (b *builder) space() {
	if b.last == 0 || b.last == ' ' || b.last == '\u00a0' {
		return
	}
	b.text(" ")
}

// node appends a formatted inline (emphasis, link).
func (b *builder) node(in ast.Inline, plain string) {
	if plain == "" {
		return
	}
	b.flush()
	b.out = append(b.out, in)
	r, _ := utf8.DecodeLastRuneInString(plain)
	b.last = r
	b.quoteClose = ""
}

// emph appends italic text.
func (b *builder) emph(s string) {
	if s == "" {
		return
	}
	b.node(&ast.Emph{Inlines: []ast.Inline{&ast.Text{Value: s}}}, s)
}

// link appends a hyperlink whose visible text is text.
func (b *builder) link(url, text string) {
	if url == "" {
		b.text(text)
		return
	}
	if text == "" {
		text = url
	}
	b.node(&ast.Link{URL: url, Inlines: []ast.Inline{&ast.Text{Value: text}}}, text)
}

// quoted wraps the output of fill in quotation marks.
func (b *builder) quoted(open, close string, fill func()) {
	b.text(open)
	fill()
	b.inside = b.last
	b.text(close)
	b.quoteClose = close
}

// punct appends a punctuation mark followed by nothing; it is dropped
// when it would double the preceding mark, and moved inside a closing
// quotation mark for American typography.
func (b *builder) punct(p string) {
	if p == "" || b.last == 0 {
		return
	}
	b.trimSpace()
	prev := b.last
	if b.quoteClose != "" && b.moveInside {
		// American typography: the mark goes inside, next to the title's
		// own punctuation. British style keeps it after the quote.
		prev = b.inside
	}
	r, _ := utf8.DecodeRuneInString(p)
	switch r {
	case '.':
		if prev == '.' || prev == '?' || prev == '!' || prev == '…' {
			return
		}
		if prev == ',' && b.replaceComma() {
			return
		}
	case ',':
		if prev == '?' || prev == '!' || prev == ',' {
			return
		}
	case ';', ':':
		if prev == r {
			return
		}
	}
	if b.quoteClose != "" && b.moveInside && (r == '.' || r == ',') {
		s := b.buf.String()
		s = s[:len(s)-len(b.quoteClose)] + p + b.quoteClose
		b.buf.Reset()
		b.buf.WriteString(s)
		lr, _ := utf8.DecodeLastRuneInString(p)
		b.inside = lr
		return
	}
	b.text(p)
}

// replaceComma turns a trailing comma (possibly before a closing quote)
// into a period: “Title,” followed by a full stop becomes “Title.”.
func (b *builder) replaceComma() bool {
	s := b.buf.String()
	i := len(s) - 1
	if b.quoteClose != "" {
		i -= len(b.quoteClose)
	}
	if i < 0 || s[i] != ',' {
		return false
	}
	b.buf.Reset()
	b.buf.WriteString(s[:i] + "." + s[i+1:])
	if b.quoteClose != "" {
		b.inside = '.'
	} else {
		b.last = '.'
	}
	return true
}

// trimSpace removes trailing blanks from the pending text.
func (b *builder) trimSpace() {
	if b.last != ' ' {
		return
	}
	s := strings.TrimRight(b.buf.String(), " ")
	b.buf.Reset()
	b.buf.WriteString(s)
	if s == "" {
		b.last = lastRuneOf(b.out)
		return
	}
	b.last, _ = utf8.DecodeLastRuneInString(s)
}

func lastRuneOf(ins []ast.Inline) rune {
	if len(ins) == 0 {
		return 0
	}
	r, _ := utf8.DecodeLastRuneInString(ast.PlainText(ins[len(ins)-1:]))
	return r
}

// empty reports whether nothing has been written.
func (b *builder) empty() bool { return b.last == 0 }

// result returns the accumulated inlines with trailing blanks removed.
func (b *builder) result() []ast.Inline {
	b.trimSpace()
	b.flush()
	return b.out
}
