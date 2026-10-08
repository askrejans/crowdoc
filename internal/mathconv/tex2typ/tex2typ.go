// Package tex2typ converts LaTeX math-mode source into Typst math markup.
//
// The converter understands the math subset of LaTeX, amsmath, amssymb,
// mathtools and the common MathJax/texvc extensions: symbols, fractions,
// roots, scripts, accents, fonts, delimiters, spacing, colours, matrices,
// cases and the multi-line alignment environments. Input is untrusted: the
// converter never panics, runs in linear time and always returns Typst that
// parses; anything it cannot represent is dropped or approximated and
// reported in Result.Warnings.
package tex2typ

import "strings"

// trimOutput removes surrounding space and line breaks at the very end: a
// trailing "\" would escape the closing "$" of the equation.
func trimOutput(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasSuffix(s, `\`) {
		n := len(s) - len(strings.TrimRight(s, `\`))
		if n%2 == 0 {
			break
		}
		s = strings.TrimSpace(s[:len(s)-1])
	}
	return s
}

// Result is the outcome of a conversion.
type Result struct {
	// Typst is the content to place between Typst `$ … $` delimiters
	// (without the delimiters).
	Typst string
	// Label is the first \label{…} found in the formula, removed from it.
	Label string
	// Numbered is false when the formula asked not to be numbered:
	// \nonumber, \notag or a starred environment such as align*.
	Numbered bool
	// Warnings lists unknown commands and dropped or approximated constructs.
	Warnings []string
}

// Convert converts LaTeX math-mode source (without $ delimiters) to Typst.
func Convert(tex string) Result {
	p := &parser{src: tex, toks: lex(tex)}
	root := trimBreaks(p.parseList(0))
	if u := root.unwrap(); u.kind == nRows {
		// A lone alignment environment is the whole formula: use Typst's
		// own line breaks and alignment points.
		u.top = true
	}
	w := renderer{warn: p.warn}
	w.b.Grow(len(tex) + len(tex)/2)
	w.render(root, ctx{})
	rows := p.rows
	if rows < 1 {
		rows = 1
	}
	return Result{
		Typst:    trimOutput(w.b.String()),
		Label:    p.label,
		Numbered: !p.starred && p.noNumbers < rows,
		Warnings: p.warns,
	}
}
