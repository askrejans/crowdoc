package layout

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// style is the formatting of a piece of text.
type style struct {
	link                         string
	sup, sub, code, bold, italic bool
}

func (a style) and(b style) style {
	out := style{sup: a.sup && b.sup, sub: a.sub && b.sub, code: a.code && b.code,
		bold: a.bold && b.bold, italic: a.italic && b.italic}
	if a.link == b.link {
		out.link = a.link
	}
	return out
}

type tok struct {
	text string
	st   style
	sep  bool
	brk  bool // hard line break before the token
	note *ast.Note
}

// inlineOpts suppresses emphasis implied by the block (headings are bold,
// captions often italic).
type inlineOpts struct {
	noBold, noItalic bool
	noNotes          bool
	// flow, when set, enables hard line breaks: a line that ended early
	// although the next word would have fitted was broken on purpose.
	flow *flow
}

// inlines converts the words of consecutive lines into inline nodes:
// spaces from gaps, lines joined with de-hyphenation, styles grouped,
// footnote markers replaced by notes.
func (d *doc) inlines(lines []*line, o inlineOpts) []ast.Inline {
	var toks []tok
	for li, l := range lines {
		for wi, w := range l.words {
			sep, brk := false, false
			switch {
			case wi > 0:
				sep = spaceBefore(l.words[wi-1], w)
			case li > 0 && len(toks) > 0:
				toks, sep = d.joinLines(toks, w)
				// A line that ends a column is not broken on purpose.
				if f := lines[li-1].fl; f != nil && o.flow != nil && l.fl == f {
					brk = sep && forcedBreak(lines[li-1], w, f)
				}
			}
			t := tok{text: w.text, st: d.styleOf(w, o), sep: sep && !brk, brk: brk}
			if text, it, bd := mathText(w.text); text != w.text {
				t.text = text
				t.st.italic = t.st.italic || it && !o.noItalic && !t.st.code
				t.st.bold = t.st.bold || bd && !o.noBold && !t.st.code
			}
			if fn := d.noteOf[w]; fn != nil && !o.noNotes {
				t.note = d.noteNode(fn)
			}
			toks = append(toks, t)
		}
	}
	for i := range toks {
		if strings.ContainsRune(toks[i].text, '­') {
			toks[i].text = strings.ReplaceAll(toks[i].text, "­", "")
		}
	}
	return buildInlines(toks)
}

// forcedBreak reports whether line l ended although word next would have
// fitted after it.
func forcedBreak(l *line, next *word, f *flow) bool {
	return short(l, f) && l.x1+0.4*next.size+next.w() < f.right-0.5*next.size
}

func (d *doc) styleOf(w *word, o inlineOpts) style {
	st := style{link: w.link, sup: w.sup, sub: w.sub, code: w.mono && !d.bodyMono,
		bold: w.bold && !o.noBold, italic: w.italic && !o.noItalic}
	if st.code {
		st.bold, st.italic = false, false
	}
	return st
}

// joinLines joins the last word of a line — possibly several tokens in
// different styles — with the first word of the next line, removing a
// hyphenation hyphen, and reports whether a space separates them.
func (d *doc) joinLines(toks []tok, next *word) ([]tok, bool) {
	last := len(toks) - 1
	start := last
	for start > 0 && !toks[start].sep && !toks[start].brk && toks[start-1].note == nil {
		start--
	}
	var sb strings.Builder
	for _, t := range toks[start:] {
		sb.WriteString(t.text)
	}
	pt := sb.String()
	lt := &toks[last]
	switch {
	case strings.HasSuffix(lt.text, "\u00AD"):
		lt.text = strings.TrimSuffix(lt.text, "\u00AD")
	case endsHyphen(pt) && lt.note == nil && !lt.st.sup:
		if d.dehyphenate(pt, next.text) {
			_, size := utf8.DecodeLastRuneInString(lt.text)
			lt.text = lt.text[:len(lt.text)-size]
		}
	case looksLikeURL(pt) && strings.ContainsAny(string(lastRune(pt)), "/._=&?#"):
	default:
		return toks, true
	}
	if lt.text == "" && lt.note == nil {
		toks = toks[:last]
	}
	return toks, false
}

func looksLikeURL(s string) bool {
	return strings.Contains(s, "://") || strings.HasPrefix(s, "www.")
}

// dehyphenate decides whether a line-final hyphen is a hyphenation point
// (remove it) or part of a compound word (keep it).
func (d *doc) dehyphenate(prefix, suffix string) bool {
	p := strings.TrimRightFunc(prefix, func(r rune) bool { return r == '-' || r == '‐' })
	core := trailingLetters(p)
	sw := leadingLetters(suffix)
	if core == "" || sw == "" {
		return false
	}
	if !unicode.IsLower(firstRune(sw)) {
		return false
	}
	if len(core) != len(p) && strings.ContainsAny(p, "-‐0123456789/") {
		return false // "state-of-" + "the-art", "COVID-" etc.
	}
	if strings.ContainsAny(suffix, "-‐") && strings.Index(suffix, "-") < len(suffix)-1 {
		return false
	}
	joined := strings.ToLower(core + sw)
	hyph := strings.ToLower(core + "-" + sw)
	if d.freq[hyph] > 0 && d.freq[joined] == 0 {
		return false
	}
	if d.freq[joined] > 0 {
		return true
	}
	if utf8.RuneCountInString(core) == 1 {
		return false
	}
	return !compoundPrefixes[strings.ToLower(core)]
}

func trailingLetters(s string) string {
	i := len(s)
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		if !unicode.IsLetter(r) {
			break
		}
		i -= size
	}
	return s[i:]
}

func leadingLetters(s string) string {
	for i, r := range s {
		if !unicode.IsLetter(r) {
			return s[:i]
		}
	}
	return s
}

// buildInlines groups styled tokens into nested inline nodes.
func buildInlines(toks []tok) []ast.Inline {
	type atom struct {
		text string
		st   style
		note *ast.Note
		brk  bool
	}
	var atoms []atom
	add := func(text string, st style) {
		if n := len(atoms); n > 0 && atoms[n-1].note == nil && !atoms[n-1].brk && atoms[n-1].st == st {
			atoms[n-1].text += text
			return
		}
		atoms = append(atoms, atom{text: text, st: st})
	}
	var prev style
	for i, t := range toks {
		if t.sep && i > 0 {
			add(" ", prev.and(t.st))
		}
		if t.brk {
			atoms = append(atoms, atom{brk: true})
		}
		if t.note != nil {
			atoms = append(atoms, atom{note: t.note})
			prev = style{}
			continue
		}
		add(t.text, t.st)
		prev = t.st
	}
	out := make([]ast.Inline, 0, len(atoms))
	for _, a := range atoms {
		if a.brk {
			out = append(out, &ast.LineBreak{})
			continue
		}
		if a.note != nil {
			out = append(out, a.note)
			continue
		}
		out = append(out, wrap(a.text, a.st))
	}
	out = mergeAdjacent(out)
	out = hoistSpaces(out)
	return ast.TrimInlines(ast.MergeText(out))
}

func wrap(text string, st style) ast.Inline {
	var in ast.Inline
	if st.code {
		in = &ast.Code{Text: text}
	} else {
		in = &ast.Text{Value: text}
		if st.italic {
			in = &ast.Emph{Inlines: []ast.Inline{in}}
		}
		if st.bold {
			in = &ast.Strong{Inlines: []ast.Inline{in}}
		}
	}
	switch {
	case st.sup:
		in = &ast.Superscript{Inlines: []ast.Inline{in}}
	case st.sub:
		in = &ast.Subscript{Inlines: []ast.Inline{in}}
	}
	if st.link != "" {
		in = &ast.Link{URL: st.link, Inlines: []ast.Inline{in}}
	}
	return in
}

// mergeAdjacent merges neighbouring nodes of the same kind so that
// "Strong(a) Strong(b)" becomes "Strong(a b)".
func mergeAdjacent(ins []ast.Inline) []ast.Inline {
	out := make([]ast.Inline, 0, len(ins))
	for _, in := range ins {
		if n := len(out); n > 0 {
			if m := tryMerge(out[n-1], in); m != nil {
				out[n-1] = m
				continue
			}
		}
		out = append(out, in)
	}
	return out
}

func tryMerge(a, b ast.Inline) ast.Inline {
	switch x := a.(type) {
	case *ast.Text:
		if y, ok := b.(*ast.Text); ok {
			return &ast.Text{Value: x.Value + y.Value}
		}
	case *ast.Code:
		if y, ok := b.(*ast.Code); ok {
			return &ast.Code{Text: x.Text + y.Text}
		}
	case *ast.Strong:
		if y, ok := b.(*ast.Strong); ok {
			return &ast.Strong{Inlines: mergeAdjacent(append(append([]ast.Inline{}, x.Inlines...), y.Inlines...))}
		}
	case *ast.Emph:
		if y, ok := b.(*ast.Emph); ok {
			return &ast.Emph{Inlines: mergeAdjacent(append(append([]ast.Inline{}, x.Inlines...), y.Inlines...))}
		}
	case *ast.Superscript:
		if y, ok := b.(*ast.Superscript); ok {
			return &ast.Superscript{Inlines: mergeAdjacent(append(append([]ast.Inline{}, x.Inlines...), y.Inlines...))}
		}
	case *ast.Subscript:
		if y, ok := b.(*ast.Subscript); ok {
			return &ast.Subscript{Inlines: mergeAdjacent(append(append([]ast.Inline{}, x.Inlines...), y.Inlines...))}
		}
	case *ast.Link:
		if y, ok := b.(*ast.Link); ok && x.URL == y.URL {
			return &ast.Link{URL: x.URL, Inlines: mergeAdjacent(append(append([]ast.Inline{}, x.Inlines...), y.Inlines...))}
		}
	}
	return nil
}

// hoistSpaces moves leading and trailing spaces out of formatting
// containers ("**bold **" → "**bold** ").
func hoistSpaces(ins []ast.Inline) []ast.Inline {
	out := make([]ast.Inline, 0, len(ins))
	for _, in := range ins {
		children := containerChildren(in)
		if children == nil {
			out = append(out, in)
			continue
		}
		*children = hoistSpaces(*children)
		lead, trail := false, false
		if n := len(*children); n > 0 {
			if t, ok := (*children)[0].(*ast.Text); ok && strings.HasPrefix(t.Value, " ") {
				lead = true
				(*children)[0] = &ast.Text{Value: strings.TrimLeft(t.Value, " ")}
			}
			if t, ok := (*children)[len(*children)-1].(*ast.Text); ok && strings.HasSuffix(t.Value, " ") {
				trail = true
				(*children)[len(*children)-1] = &ast.Text{Value: strings.TrimRight(t.Value, " ")}
			}
		}
		*children = ast.MergeText(*children)
		if lead {
			out = append(out, &ast.Text{Value: " "})
		}
		if len(ast.TrimInlines(*children)) > 0 {
			out = append(out, in)
		}
		if trail {
			out = append(out, &ast.Text{Value: " "})
		}
	}
	return out
}

func containerChildren(in ast.Inline) *[]ast.Inline {
	switch x := in.(type) {
	case *ast.Strong:
		return &x.Inlines
	case *ast.Emph:
		return &x.Inlines
	case *ast.Link:
		return &x.Inlines
	case *ast.Superscript:
		return &x.Inlines
	case *ast.Subscript:
		return &x.Inlines
	}
	return nil
}
