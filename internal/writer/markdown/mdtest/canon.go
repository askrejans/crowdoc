// Package mdtest summarises documents for Markdown round-trip tests: two
// documents that read the same have equal summaries.
//
// The summary keeps structure and text but ignores differences Markdown
// cannot or need not preserve: paragraphs versus plain blocks, soft line
// breaks versus spaces, white space at the edges of formatting, list
// tightness, image paths (only file names are compared) and white space
// inside mathematics.
package mdtest

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/askrejans/crowdoc/v2/ast"
)

// Summary renders the document body and the metadata the Markdown writer
// carries.
func Summary(doc *ast.Document) string {
	var sb strings.Builder
	m := doc.Meta
	if !m.TitleFromName {
		fmt.Fprintf(&sb, "title: %s\n", m.Title)
	}
	fmt.Fprintf(&sb, "subtitle: %s\nauthors: %v\ndate: %s\nkeywords: %v\n", m.Subtitle, m.AuthorNames(), m.Date, m.Keywords)
	if len(m.Abstract) > 0 {
		sb.WriteString("abstract:\n" + Blocks(m.Abstract))
	}
	sb.WriteString(Blocks(doc.Blocks))
	return sb.String()
}

// Blocks summarises blocks one per line.
func Blocks(blocks []ast.Block) string {
	var sb strings.Builder
	for _, b := range blocks {
		block(&sb, b, 0)
	}
	return sb.String()
}

func indent(sb *strings.Builder, lvl int) { sb.WriteString(strings.Repeat("  ", lvl)) }

func block(sb *strings.Builder, b ast.Block, lvl int) {
	var ins []ast.Inline
	switch n := b.(type) {
	case *ast.Para:
		ins = Canon(n.Inlines)
	case *ast.Plain:
		ins = Canon(n.Inlines)
	}
	switch n := b.(type) {
	case *ast.Para, *ast.Plain:
		if len(ins) == 0 {
			return
		}
		// A paragraph holding only an image is a figure in Markdown.
		if img, ok := ins[0].(*ast.Image); ok && len(ins) == 1 {
			fig := &ast.Figure{Attr: ast.Attr{ID: img.Attr.ID}, Image: img}
			if img.Attr.ID != "" {
				i := *img
				i.Attr.ID = ""
				fig.Image = &i
			}
			b = fig
		}
		_ = n
	}
	indent(sb, lvl)
	switch n := b.(type) {
	case *ast.Para:
		sb.WriteString("P " + Inlines(n.Inlines))
	case *ast.Plain:
		sb.WriteString("P " + Inlines(n.Inlines))
	case *ast.Heading:
		fmt.Fprintf(sb, "H%d#%s", n.Level, n.Attr.ID)
		if n.Unnumbered || n.Attr.HasClass("unnumbered") {
			sb.WriteString("[-]")
		}
		// Markdown headings are one line.
		sb.WriteString(" " + Inlines(breaksToSpaces(n.Inlines)))
	case *ast.CodeBlock:
		fmt.Fprintf(sb, "Code[%s]#%s %q", strings.ToLower(n.Lang), n.Attr.ID, codeText(n.Text))
		if len(Canon(n.Caption)) > 0 {
			sb.WriteString(" cap=" + Inlines(n.Caption))
		}
	case *ast.MathBlock:
		fmt.Fprintf(sb, "Math#%s %s", n.Label, strings.Join(strings.Fields(n.TeX), " "))
	case *ast.RawBlock:
		fmt.Fprintf(sb, "Raw[%s] %q", n.Format, n.Text)
	case *ast.BlockQuote:
		sb.WriteString("Quote\n")
		children(sb, n.Blocks, lvl+1)
		return
	case *ast.List:
		if n.Ordered {
			start := max(n.Start, 1)
			fmt.Fprintf(sb, "OL(%d)\n", start)
		} else {
			sb.WriteString("UL\n")
		}
		for _, it := range n.Items {
			indent(sb, lvl+1)
			sb.WriteString("Item")
			task := it.Task
			if !startsWithText(it.Blocks) {
				task = ast.TaskNone // a checkbox needs text after it
			}
			switch task {
			case ast.TaskOpen:
				sb.WriteString("[ ]")
			case ast.TaskDone:
				sb.WriteString("[x]")
			}
			sb.WriteString("\n")
			children(sb, it.Blocks, lvl+2)
		}
		return
	case *ast.DefinitionList:
		sb.WriteString("DL\n")
		for _, it := range n.Items {
			indent(sb, lvl+1)
			sb.WriteString("Term " + Inlines(breaksToSpaces(it.Term)) + "\n")
			for _, d := range it.Definitions {
				indent(sb, lvl+2)
				sb.WriteString("Def\n")
				children(sb, d, lvl+3)
			}
		}
		return
	case *ast.Table:
		fmt.Fprintf(sb, "Table#%s", n.Attr.ID)
		if len(Canon(n.Caption)) > 0 {
			sb.WriteString(" cap=" + Inlines(n.Caption))
		}
		var al []string
		for _, c := range n.Cols {
			al = append(al, fmt.Sprint(int(c.Align)))
		}
		sb.WriteString(" cols=" + strings.Join(al, ",") + "\n")
		for _, part := range []struct {
			name string
			rows []ast.Row
		}{{"head", n.Head}, {"body", n.Body}, {"foot", n.Foot}} {
			for _, r := range part.rows {
				if part.name == "head" && emptyRow(r) {
					continue // a pipe table without a header has an empty one
				}
				indent(sb, lvl+1)
				sb.WriteString(part.name + "\n")
				for _, c := range r.Cells {
					indent(sb, lvl+2)
					fmt.Fprintf(sb, "Cell(%dx%d)\n", max(c.ColSpan, 1), max(c.RowSpan, 1))
					children(sb, c.Blocks, lvl+3)
				}
			}
		}
		return
	case *ast.Figure:
		fmt.Fprintf(sb, "Figure#%s", n.Attr.ID)
		if n.Image != nil {
			img := *n.Image
			if strings.Join(strings.Fields(img.Alt), " ") == strings.Join(strings.Fields(ast.PlainText(n.Caption)), " ") {
				img.Alt = "" // the caption serves as alt text
			}
			sb.WriteString(" " + image(&img))
		}
		sb.WriteString(" cap=" + Inlines(n.Caption))
	case *ast.HorizontalRule:
		sb.WriteString("HR")
	case *ast.PageBreak:
		sb.WriteString("PageBreak")
	case *ast.Div:
		fmt.Fprintf(sb, "Div%s title=%s\n", attr(n.Attr), Inlines(n.Title))
		children(sb, n.Blocks, lvl+1)
		return
	case *ast.LineBlock:
		sb.WriteString("LineBlock")
		for _, l := range n.Lines {
			// Indentation reads back as no-break spaces.
			line := Inlines(l)
			rest := strings.TrimLeft(line, " \u00a0")
			sb.WriteString(" | " + strings.Repeat("_", len([]rune(line))-len([]rune(rest))) + rest)
		}
	case *ast.Bibliography:
		sb.WriteString("Bibliography")
	case *ast.ReferenceList:
		sb.WriteString("ReferenceList\n")
		for _, e := range n.Entries {
			indent(sb, lvl+1)
			fmt.Fprintf(sb, "[%s] %s\n", e.Label, Inlines(e.Inlines))
		}
		return
	default:
		fmt.Fprintf(sb, "%T", b)
	}
	sb.WriteString("\n")
}

func startsWithText(blocks []ast.Block) bool {
	for _, b := range blocks {
		var ins []ast.Inline
		switch n := b.(type) {
		case *ast.Para:
			ins = n.Inlines
		case *ast.Plain:
			ins = n.Inlines
		case *ast.Figure:
			return true // a paragraph holding only an image
		default:
			return false
		}
		if c := Canon(ins); len(c) > 0 {
			return true
		}
	}
	return false
}

// codeText drops white space that only fills a line: inside list items
// and quotes such lines cannot be told from blank ones.
func codeText(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			lines[i] = ""
		}
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func emptyRow(r ast.Row) bool {
	for _, c := range r.Cells {
		if strings.TrimSpace(Blocks(c.Blocks)) != "" {
			return false
		}
	}
	return true
}

func children(sb *strings.Builder, blocks []ast.Block, lvl int) {
	for _, b := range blocks {
		block(sb, b, lvl)
	}
}

func attr(a ast.Attr) string {
	var parts []string
	if a.ID != "" {
		parts = append(parts, "#"+a.ID)
	}
	for _, c := range a.Classes {
		parts = append(parts, "."+c)
	}
	var keys []string
	for k := range a.KV {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, k+"="+a.KV[k])
	}
	return "{" + strings.Join(parts, " ") + "}"
}

func image(img *ast.Image) string {
	src := img.Src
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		src = strings.ToLower(path.Base(strings.TrimPrefix(src, "res:")))
	}
	s := "img(" + src
	if alt := strings.Join(strings.Fields(img.Alt), " "); alt != "" {
		s += " alt=" + alt
	}
	if img.Title != "" {
		s += " title=" + img.Title
	}
	if img.Width != "" {
		s += " w=" + img.Width
	}
	if img.Height != "" {
		s += " h=" + img.Height
	}
	a := img.Attr
	a.ID = ""
	if a.KV != nil {
		kv := map[string]string{}
		for k, v := range a.KV {
			if k != "width" && k != "height" {
				kv[k] = v
			}
		}
		a.KV = kv
	}
	if len(a.Classes) > 0 || len(a.KV) > 0 {
		s += " " + attr(a)
	}
	return s + ")"
}

// Inlines summarises inlines after canonicalisation.
func Inlines(ins []ast.Inline) string {
	var sb strings.Builder
	inlines(&sb, Canon(ins))
	return sb.String()
}

// Canon canonicalises inline content: soft breaks become spaces, text is
// merged, white space at the edges of formatting moves outside it, empty
// formatting is dropped and the content is trimmed.
func Canon(ins []ast.Inline) []ast.Inline {
	return trim(canon(ins))
}

func canon(ins []ast.Inline) []ast.Inline {
	var out []ast.Inline
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.SoftBreak:
			out = append(out, &ast.Text{Value: " "})
		case *ast.Text:
			if v := controls.Replace(n.Value); v != "" {
				out = append(out, &ast.Text{Value: v})
			}
		case *ast.Emph, *ast.Strong, *ast.Strike, *ast.Underline, *ast.Superscript, *ast.Subscript, *ast.SmallCaps, *ast.Highlight:
			lead, core, trail := edges(canon(ast.InlineChildren(in)))
			out = append(out, lead...)
			if len(core) > 0 {
				out = append(out, rebuild(in, core))
			}
			out = append(out, trail...)
		case *ast.Span:
			lead, core, trail := edges(canon(n.Inlines))
			out = append(out, lead...)
			switch {
			case n.Attr.ID == "" && len(n.Attr.Classes) == 0 && len(n.Attr.KV) == 0:
				out = append(out, core...)
			case len(core) > 0 || n.Attr.ID != "":
				out = append(out, &ast.Span{Attr: n.Attr, Inlines: core})
			}
			out = append(out, trail...)
		case *ast.Link:
			l := *n
			l.Inlines = canon(n.Inlines)
			out = append(out, &l)
		case *ast.Code:
			if t := controls.Replace(n.Text); t != "" {
				out = append(out, &ast.Code{Text: t})
			}
		case *ast.Math:
			if t := strings.Join(strings.Fields(controls.Replace(n.TeX)), " "); t != "" {
				out = append(out, &ast.Math{TeX: t})
			}
		default:
			out = append(out, in)
		}
	}
	return merge(out)
}

// controls drops control characters Markdown cannot carry (line breaks in
// text are spaces).
var controls = func() *strings.Replacer {
	var pairs []string
	for c := rune(0); c < 0x20; c++ {
		switch c {
		case '\t':
		case '\n', '\r':
			pairs = append(pairs, string(c), " ")
		default:
			pairs = append(pairs, string(c), "")
		}
	}
	return strings.NewReplacer(append(pairs, "\x7f", "")...)
}()

func merge(ins []ast.Inline) []ast.Inline {
	var out []ast.Inline
	for _, in := range ins {
		if t, ok := in.(*ast.Text); ok && len(out) > 0 {
			if p, ok := out[len(out)-1].(*ast.Text); ok {
				out[len(out)-1] = &ast.Text{Value: p.Value + t.Value}
				continue
			}
		}
		out = append(out, in)
	}
	return out
}

func edges(ins []ast.Inline) (lead, core, trail []ast.Inline) {
	core = ins
	for len(core) > 0 {
		switch n := core[0].(type) {
		case *ast.LineBreak:
			lead = append(lead, n)
			core = core[1:]
			continue
		case *ast.Text:
			v := strings.TrimLeftFunc(n.Value, unicode.IsSpace)
			if v != n.Value {
				lead = append(lead, &ast.Text{Value: n.Value[:len(n.Value)-len(v)]})
				if v == "" {
					core = core[1:]
					continue
				}
				core = append([]ast.Inline{&ast.Text{Value: v}}, core[1:]...)
			}
		}
		break
	}
	for len(core) > 0 {
		last := len(core) - 1
		switch n := core[last].(type) {
		case *ast.LineBreak:
			trail = append([]ast.Inline{n}, trail...)
			core = core[:last]
			continue
		case *ast.Text:
			v := strings.TrimRightFunc(n.Value, unicode.IsSpace)
			if v != n.Value {
				trail = append([]ast.Inline{&ast.Text{Value: n.Value[len(v):]}}, trail...)
				if v == "" {
					core = core[:last]
					continue
				}
				core = append(append([]ast.Inline{}, core[:last]...), &ast.Text{Value: v})
			}
		}
		break
	}
	return lead, merge(core), trail
}

func trim(ins []ast.Inline) []ast.Inline {
	ins = merge(ins)
	for len(ins) > 0 {
		if _, ok := ins[0].(*ast.LineBreak); ok {
			ins = ins[1:]
			continue
		}
		if t, ok := ins[0].(*ast.Text); ok {
			v := strings.TrimLeftFunc(t.Value, unicode.IsSpace)
			if v == "" {
				ins = ins[1:]
				continue
			}
			ins = append([]ast.Inline{&ast.Text{Value: v}}, ins[1:]...)
		}
		break
	}
	for len(ins) > 0 {
		last := len(ins) - 1
		if _, ok := ins[last].(*ast.LineBreak); ok {
			ins = ins[:last]
			continue
		}
		if t, ok := ins[last].(*ast.Text); ok {
			v := strings.TrimRightFunc(t.Value, unicode.IsSpace)
			if v == "" {
				ins = ins[:last]
				continue
			}
			ins = append(append([]ast.Inline{}, ins[:last]...), &ast.Text{Value: v})
		}
		break
	}
	return ins
}

func rebuild(in ast.Inline, kids []ast.Inline) ast.Inline {
	switch in.(type) {
	case *ast.Emph:
		return &ast.Emph{Inlines: kids}
	case *ast.Strong:
		return &ast.Strong{Inlines: kids}
	case *ast.Strike:
		return &ast.Strike{Inlines: kids}
	case *ast.Underline:
		return &ast.Underline{Inlines: kids}
	case *ast.Superscript:
		return &ast.Superscript{Inlines: kids}
	case *ast.Subscript:
		return &ast.Subscript{Inlines: kids}
	case *ast.SmallCaps:
		return &ast.SmallCaps{Inlines: kids}
	case *ast.Highlight:
		return &ast.Highlight{Inlines: kids}
	}
	return in
}

func inlines(sb *strings.Builder, ins []ast.Inline) {
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			sb.WriteString(n.Value)
		case *ast.LineBreak:
			sb.WriteString("⏎")
		case *ast.Emph:
			wrap(sb, "em", n.Inlines)
		case *ast.Strong:
			wrap(sb, "strong", n.Inlines)
		case *ast.Strike:
			wrap(sb, "strike", n.Inlines)
		case *ast.Underline:
			wrap(sb, "u", n.Inlines)
		case *ast.Superscript:
			wrap(sb, "sup", n.Inlines)
		case *ast.Subscript:
			wrap(sb, "sub", n.Inlines)
		case *ast.SmallCaps:
			wrap(sb, "sc", n.Inlines)
		case *ast.Highlight:
			wrap(sb, "mark", n.Inlines)
		case *ast.Span:
			switch {
			case n.Attr.ID == "" && len(n.Attr.KV) == 0 && len(n.Attr.Classes) == 1 && n.Attr.Classes[0] == "smallcaps":
				wrap(sb, "sc", n.Inlines)
			case n.Attr.ID == "" && len(n.Attr.KV) == 0 && len(n.Attr.Classes) == 1 && n.Attr.Classes[0] == "underline":
				wrap(sb, "u", n.Inlines)
			case n.Attr.ID == "" && len(n.Attr.KV) == 0 && len(n.Attr.Classes) == 1 && n.Attr.Classes[0] == "mark":
				wrap(sb, "mark", n.Inlines)
			default:
				wrap(sb, "span"+attr(n.Attr), n.Inlines)
			}
		case *ast.Code:
			sb.WriteString("`" + strings.ReplaceAll(n.Text, "\n", " ") + "`")
		case *ast.Math:
			sb.WriteString("$" + strings.Join(strings.Fields(n.TeX), " ") + "$")
		case *ast.Link:
			sb.WriteString("<link " + controls.Replace(n.URL))
			if t := controls.Replace(n.Title); t != "" {
				sb.WriteString(" title=" + t)
			}
			sb.WriteString(">")
			inlines(sb, n.Inlines)
			sb.WriteString("</link>")
		case *ast.Image:
			sb.WriteString(image(n))
		case *ast.Note:
			sb.WriteString("<note>" + strings.ReplaceAll(strings.TrimSpace(Blocks(n.Blocks)), "\n", " / ") + "</note>")
		case *ast.Cite:
			// An unresolved cross-reference reads as a citation.
			if len(n.Items) == 1 && refPrefix(n.Items[0].Key) && n.Items[0].Prefix == "" && n.Items[0].Locator == "" && n.Items[0].Suffix == "" {
				fmt.Fprintf(sb, "<ref %s bare=%v>", n.Items[0].Key, n.Items[0].SuppressAuthor)
				continue
			}
			sb.WriteString("<cite")
			if n.Mode == ast.CiteNarrative {
				sb.WriteString("-n")
			}
			for _, it := range n.Items {
				label := it.LocatorLabel
				if label == "" {
					label = "page"
				}
				fmt.Fprintf(sb, " [%s|%v|%s|%s=%s|%s]", it.Prefix, it.SuppressAuthor, it.Key, label, it.Locator, it.Suffix)
			}
			sb.WriteString(">")
		case *ast.Ref:
			fmt.Fprintf(sb, "<ref %s bare=%v>", n.Target, n.Bare)
		case *ast.RawInline:
			fmt.Fprintf(sb, "<raw %s %q>", n.Format, n.Text)
		default:
			fmt.Fprintf(sb, "%T", in)
		}
	}
}

func breaksToSpaces(ins []ast.Inline) []ast.Inline {
	out := make([]ast.Inline, 0, len(ins))
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.LineBreak:
			out = append(out, &ast.Text{Value: " "})
		default:
			if kids := ast.InlineChildren(in); kids != nil && n != nil {
				if _, ok := in.(*ast.Cite); !ok {
					out = append(out, rebuildAny(in, breaksToSpaces(kids)))
					continue
				}
			}
			out = append(out, in)
		}
	}
	return out
}

func rebuildAny(in ast.Inline, kids []ast.Inline) ast.Inline {
	switch n := in.(type) {
	case *ast.Link:
		l := *n
		l.Inlines = kids
		return &l
	case *ast.Span:
		return &ast.Span{Attr: n.Attr, Inlines: kids}
	}
	return rebuild(in, kids)
}

func refPrefix(key string) bool {
	for _, p := range []string{"fig:", "tbl:", "sec:", "eq:", "lst:", "tab:", "chap:", "app:"} {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

func wrap(sb *strings.Builder, tag string, ins []ast.Inline) {
	sb.WriteString("<" + tag + ">")
	inlines(sb, ins)
	sb.WriteString("</" + tag + ">")
}
