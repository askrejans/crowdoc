// Package normalize applies format-independent clean-up to a parsed
// document: title inference, heading levels, abstract and keyword
// extraction, cross-reference resolution and manual-numbering detection.
package normalize

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/askrejans/crowdoc/v2/ast"
)

// Options controls normalisation.
type Options struct {
	// SourceName is the input file name, used as the last-resort title.
	SourceName string
	// FallbackTitle is used when neither metadata nor content name the
	// document (defaults to a title derived from SourceName).
	FallbackTitle string
	// ExtractAbstract moves a leading "Abstract" section into Meta.Abstract.
	ExtractAbstract bool
}

// Document normalises doc in place and returns warnings.
func Document(doc *ast.Document, o Options) []string {
	var warns []string
	doc.Blocks = dropEmpty(doc.Blocks)
	inferTitle(doc, o)
	shiftHeadings(doc.Blocks)
	if o.ExtractAbstract {
		extractAbstract(doc)
	}
	extractKeywords(doc)
	detectManualNumbering(doc)
	warns = append(warns, resolveCrossRefs(doc)...)
	ensureIDs(doc)
	return warns
}

// ---------------------------------------------------------------------------
// Title
// ---------------------------------------------------------------------------

func inferTitle(doc *ast.Document, o Options) {
	if len(doc.Blocks) > 0 {
		if h, ok := doc.Blocks[0].(*ast.Heading); ok {
			text := ast.PlainText(h.Inlines)
			switch {
			case doc.Meta.Title == "" && isOnlyAtLevel(doc.Blocks, h.Level) && minLevel(doc.Blocks) == h.Level:
				doc.Meta.Title = text
				doc.Blocks = doc.Blocks[1:]
				// "# Title" directly followed by a lone deeper heading and
				// no content is a subtitle only when it is the only one.
			case doc.Meta.Title != "" && strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(doc.Meta.Title)):
				doc.Blocks = doc.Blocks[1:]
			}
		}
	}
	if doc.Meta.Title == "" {
		doc.Meta.Title = o.FallbackTitle
		doc.Meta.TitleFromName = true
	}
	// A short, fully italic first paragraph under the title is a subtitle
	// (the usual shape of a standfirst in web pages and office files). An
	// untitled document keeps it as text.
	if doc.Meta.Subtitle == "" && !doc.Meta.TitleFromName && len(doc.Blocks) > 0 {
		if p, ok := doc.Blocks[0].(*ast.Para); ok {
			ins := ast.TrimInlines(p.Inlines)
			if len(ins) == 1 {
				if e, ok := ins[0].(*ast.Emph); ok {
					if text := ast.PlainText(e.Inlines); len([]rune(text)) <= 160 {
						doc.Meta.Subtitle = text
						doc.Blocks = doc.Blocks[1:]
					}
				}
			}
		}
	}
}

func isOnlyAtLevel(blocks []ast.Block, level int) bool {
	n := 0
	for _, b := range blocks {
		if h, ok := b.(*ast.Heading); ok && h.Level <= level {
			n++
		}
	}
	return n == 1
}

func minLevel(blocks []ast.Block) int {
	min := 0
	for _, b := range blocks {
		if h, ok := b.(*ast.Heading); ok && (min == 0 || h.Level < min) {
			min = h.Level
		}
	}
	return min
}

// shiftHeadings makes the highest heading level present level 1, so a
// document using "##" for its sections gets top-level sections.
func shiftHeadings(blocks []ast.Block) {
	min := 0
	ast.WalkBlocks(blocks, func(b ast.Block) bool {
		if h, ok := b.(*ast.Heading); ok && (min == 0 || h.Level < min) {
			min = h.Level
		}
		return true
	})
	if min <= 1 {
		return
	}
	ast.WalkBlocks(blocks, func(b ast.Block) bool {
		if h, ok := b.(*ast.Heading); ok {
			h.Level -= min - 1
		}
		return true
	})
}

// ---------------------------------------------------------------------------
// Abstract and keywords
// ---------------------------------------------------------------------------

var abstractTitles = map[string]bool{
	"abstract": true, "anotācija": true, "kopsavilkums": true, "zusammenfassung": true, "kurzfassung": true,
	"résumé": true, "resume": true, "resumen": true, "riassunto": true, "resumo": true, "samenvatting": true,
	"streszczenie": true, "abstrakt": true, "tiivistelmä": true, "sammanfattning": true, "sammendrag": true,
	"resumé": true, "santrauka": true, "anotacija": true, "kokkuvõte": true, "аннотация": true, "реферат": true,
	"анотація": true, "περίληψη": true, "özet": true, "absztrakt": true, "rezumat": true, "sažetak": true,
	"povzetek": true, "abstracto": true, "要旨": true, "摘要": true, "초록": true, "ملخص": true, "תקציר": true,
}

func extractAbstract(doc *ast.Document) {
	if len(doc.Meta.Abstract) > 0 {
		return
	}
	for i, b := range doc.Blocks {
		h, ok := b.(*ast.Heading)
		if !ok {
			continue
		}
		if !abstractTitles[strings.ToLower(strings.Trim(ast.PlainText(h.Inlines), " .:"))] {
			return // the abstract must be the first section
		}
		end := i + 1
		for end < len(doc.Blocks) {
			if nh, ok := doc.Blocks[end].(*ast.Heading); ok && nh.Level <= h.Level {
				break
			}
			end++
		}
		doc.Meta.Abstract = append([]ast.Block(nil), doc.Blocks[i+1:end]...)
		doc.Blocks = append(doc.Blocks[:i:i], doc.Blocks[end:]...)
		return
	}
}

var keywordLabelRe = regexp.MustCompile(`(?i)^\s*(keywords|key words|index terms|atslēgvārdi|atslēgas vārdi|schlüsselwörter|stichwörter|mots[- ]clés|palabras clave|parole chiave|palavras[- ]chave|trefwoorden|słowa kluczowe|klíčová slova|kľúčové slová|avainsanat|nyckelord|nøgleord|nøkkelord|raktažodžiai|märksõnad|ключевые слова|ключові слова|λέξεις-κλειδιά|anahtar kelimeler|kulcsszavak|cuvinte cheie|ključne riječi|ključne besede)\s*[:—–-]\s*`)

// extractKeywords lifts a "Keywords: a, b, c" paragraph from the abstract
// or the beginning of the document.
func extractKeywords(doc *ast.Document) {
	if len(doc.Meta.Keywords) > 0 {
		return
	}
	try := func(blocks []ast.Block) []ast.Block {
		for i, b := range blocks {
			if i > 3 {
				break
			}
			p, ok := b.(*ast.Para)
			if !ok {
				continue
			}
			text := ast.PlainText(p.Inlines)
			loc := keywordLabelRe.FindStringIndex(text)
			if loc == nil {
				continue
			}
			for _, k := range strings.FieldsFunc(text[loc[1]:], func(r rune) bool { return r == ',' || r == ';' || r == '·' || r == '•' }) {
				if k = strings.TrimRight(strings.TrimSpace(k), "."); k != "" {
					doc.Meta.Keywords = append(doc.Meta.Keywords, k)
				}
			}
			return append(blocks[:i:i], blocks[i+1:]...)
		}
		return blocks
	}
	if len(doc.Meta.Abstract) > 0 {
		doc.Meta.Abstract = try(doc.Meta.Abstract)
	}
	if len(doc.Meta.Keywords) == 0 {
		doc.Blocks = try(doc.Blocks)
	}
}

// ---------------------------------------------------------------------------
// Manual numbering
// ---------------------------------------------------------------------------

var manualNumberRe = regexp.MustCompile(`^\s*(?:(?:\d+\.)+\d*|\d+\)|[IVXLC]+\.|[A-Z]\.|§\s*\d+|(?i:article|chapter|section|part|nodaļa|pants|punkts|artikel|kapitel|abschnitt)\s+[\dIVXLC]+\b|\d+\s*(?i:\.?\s*(?:pants|nodaļa|punkts)))`)

// detectManualNumbering turns automatic section numbering off when the
// source already numbers its headings ("1. Scope", "2.1 Terms",
// "3. pants"), avoiding "1 1. Scope".
func detectManualNumbering(doc *ast.Document) {
	if doc.Meta.NumberSections != nil {
		return
	}
	// Judge the top level: documents often number their sections by hand
	// but leave sub-headings unnumbered.
	total, numbered := 0, 0
	ast.WalkBlocks(doc.Blocks, func(b ast.Block) bool {
		if h, ok := b.(*ast.Heading); ok && h.Level == 1 {
			total++
			if manualNumberRe.MatchString(ast.PlainText(h.Inlines)) {
				numbered++
			}
		}
		return true
	})
	if numbered >= 2 && numbered*10 >= total*6 {
		f := false
		doc.Meta.NumberSections = &f
	}
}

// ---------------------------------------------------------------------------
// Cross-references
// ---------------------------------------------------------------------------

var refPrefixes = []string{"fig:", "tbl:", "sec:", "eq:", "lst:", "tab:", "chap:", "app:"}

func hasRefPrefix(key string) bool {
	for _, p := range refPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// resolveCrossRefs replaces citations whose keys are document labels with
// ast.Ref nodes.
func resolveCrossRefs(doc *ast.Document) []string {
	labels := map[string]bool{}
	ast.WalkBlocks(doc.Blocks, func(b ast.Block) bool {
		switch n := b.(type) {
		case *ast.Heading:
			labels[n.Attr.ID] = true
		case *ast.Figure:
			labels[n.Attr.ID] = true
		case *ast.Table:
			labels[n.Attr.ID] = true
		case *ast.MathBlock:
			labels[n.Label] = true
		case *ast.CodeBlock:
			labels[n.Attr.ID] = true
		case *ast.Div:
			labels[n.Attr.ID] = true
		}
		return true
	})
	delete(labels, "")
	var warns []string
	EachInlineSlice(doc, func(ins *[]ast.Inline) {
		out := (*ins)[:0:0]
		changed := false
		for _, in := range *ins {
			c, ok := in.(*ast.Cite)
			if !ok {
				out = append(out, in)
				continue
			}
			allLabels, anyPrefixed := true, false
			for _, it := range c.Items {
				if !labels[it.Key] {
					allLabels = false
				}
				if hasRefPrefix(it.Key) {
					anyPrefixed = true
				}
			}
			switch {
			case allLabels:
				changed = true
				for i, it := range c.Items {
					if i > 0 {
						sep := ", "
						if i == len(c.Items)-1 {
							sep = " & "
						}
						out = append(out, &ast.Text{Value: sep})
					}
					if it.Prefix != "" {
						out = append(out, &ast.Text{Value: it.Prefix + " "})
					}
					out = append(out, &ast.Ref{Target: it.Key, Bare: it.SuppressAuthor})
					if it.Suffix != "" {
						out = append(out, &ast.Text{Value: " " + it.Suffix})
					}
				}
			case anyPrefixed && c.Mode == ast.CiteNarrative:
				changed = true
				warns = append(warns, "unresolved cross-reference @"+c.Items[0].Key)
				out = append(out, &ast.Strong{Inlines: []ast.Inline{&ast.Text{Value: "??"}}})
			default:
				out = append(out, in)
			}
		}
		if changed {
			*ins = out
		}
	})
	return warns
}

// EachInlineSlice calls fn with a pointer to every inline slice in the
// document (paragraphs, headings, captions, cells, notes, metadata
// abstract), allowing in-place replacement.
func EachInlineSlice(doc *ast.Document, fn func(*[]ast.Inline)) {
	eachBlocks(doc.Meta.Abstract, fn)
	eachBlocks(doc.Blocks, fn)
}

func eachBlocks(blocks []ast.Block, fn func(*[]ast.Inline)) {
	for _, b := range blocks {
		switch n := b.(type) {
		case *ast.Para:
			eachInlines(&n.Inlines, fn)
		case *ast.Plain:
			eachInlines(&n.Inlines, fn)
		case *ast.Heading:
			eachInlines(&n.Inlines, fn)
		case *ast.CodeBlock:
			eachInlines(&n.Caption, fn)
		case *ast.BlockQuote:
			eachBlocks(n.Blocks, fn)
		case *ast.List:
			for i := range n.Items {
				eachBlocks(n.Items[i].Blocks, fn)
			}
		case *ast.DefinitionList:
			for i := range n.Items {
				eachInlines(&n.Items[i].Term, fn)
				for _, d := range n.Items[i].Definitions {
					eachBlocks(d, fn)
				}
			}
		case *ast.Table:
			eachInlines(&n.Caption, fn)
			for _, rows := range [][]ast.Row{n.Head, n.Body, n.Foot} {
				for _, r := range rows {
					for _, c := range r.Cells {
						eachBlocks(c.Blocks, fn)
					}
				}
			}
		case *ast.Figure:
			eachInlines(&n.Caption, fn)
		case *ast.Div:
			eachInlines(&n.Title, fn)
			eachBlocks(n.Blocks, fn)
		case *ast.LineBlock:
			for i := range n.Lines {
				eachInlines(&n.Lines[i], fn)
			}
		case *ast.ReferenceList:
			for i := range n.Entries {
				eachInlines(&n.Entries[i].Inlines, fn)
			}
		}
	}
}

func eachInlines(ins *[]ast.Inline, fn func(*[]ast.Inline)) {
	if ins == nil || len(*ins) == 0 {
		return
	}
	fn(ins)
	for _, in := range *ins {
		switch n := in.(type) {
		case *ast.Emph:
			eachInlines(&n.Inlines, fn)
		case *ast.Strong:
			eachInlines(&n.Inlines, fn)
		case *ast.Strike:
			eachInlines(&n.Inlines, fn)
		case *ast.Underline:
			eachInlines(&n.Inlines, fn)
		case *ast.Superscript:
			eachInlines(&n.Inlines, fn)
		case *ast.Subscript:
			eachInlines(&n.Inlines, fn)
		case *ast.SmallCaps:
			eachInlines(&n.Inlines, fn)
		case *ast.Highlight:
			eachInlines(&n.Inlines, fn)
		case *ast.Link:
			eachInlines(&n.Inlines, fn)
		case *ast.Span:
			eachInlines(&n.Inlines, fn)
		case *ast.Note:
			eachBlocks(n.Blocks, fn)
		}
	}
}

// ---------------------------------------------------------------------------
// Identifiers and clean-up
// ---------------------------------------------------------------------------

// ensureIDs makes heading identifiers unique so they can serve as labels.
func ensureIDs(doc *ast.Document) {
	seen := map[string]int{}
	ast.WalkBlocks(doc.Blocks, func(b ast.Block) bool {
		var id *string
		switch n := b.(type) {
		case *ast.Heading:
			id = &n.Attr.ID
		case *ast.Figure:
			id = &n.Attr.ID
		case *ast.Table:
			id = &n.Attr.ID
		}
		if id == nil || *id == "" {
			return true
		}
		*id = SanitizeID(*id)
		if n := seen[*id]; n > 0 {
			seen[*id] = n + 1
			*id = *id + "-" + itoa(n+1)
		} else {
			seen[*id] = 1
		}
		return true
	})
}

// SanitizeID keeps letters, digits and "-_:." so identifiers are valid
// labels in every output format.
func SanitizeID(s string) string {
	var sb strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ':' || r == '.':
			sb.WriteRune(r)
		case unicode.IsSpace(r):
			sb.WriteByte('-')
		}
	}
	out := strings.Trim(sb.String(), "-.:")
	if out != "" && !unicode.IsLetter([]rune(out)[0]) {
		out = "id-" + out
	}
	return out
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// dropEmpty removes empty paragraphs and collapses repeated rules.
func dropEmpty(blocks []ast.Block) []ast.Block {
	out := blocks[:0]
	for _, b := range blocks {
		switch n := b.(type) {
		case *ast.Para:
			if len(ast.TrimInlines(n.Inlines)) == 0 {
				continue
			}
		case *ast.Plain:
			if len(ast.TrimInlines(n.Inlines)) == 0 {
				continue
			}
		case *ast.HorizontalRule:
			if len(out) > 0 {
				if _, ok := out[len(out)-1].(*ast.HorizontalRule); ok {
					continue
				}
			}
		case *ast.PageBreak:
			if len(out) == 0 {
				continue
			}
			if _, ok := out[len(out)-1].(*ast.PageBreak); ok {
				continue
			}
		}
		out = append(out, b)
	}
	return out
}
