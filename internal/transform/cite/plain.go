package cite

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// referenceTitles are headings that introduce a reference section,
// compared case-insensitively after numbering and trailing punctuation
// are removed.
var referenceTitles = map[string]bool{
	"references": true, "reference list": true, "list of references": true, "bibliography": true,
	"works cited": true, "literature": true, "literature cited": true, "cited literature": true,
	"sources": true, "list of sources": true, "references and notes": true, "notes and references": true,
	"selected bibliography": true, "further reading": true,
	// Latvian
	"literatūra": true, "izmantotā literatūra": true, "izmantotie avoti": true, "avoti": true,
	"literatūras saraksts": true, "izmantotās literatūras saraksts": true, "avotu saraksts": true,
	"izmantotās literatūras un avotu saraksts": true, "izmantotā literatūra un avoti": true,
	"bibliogrāfija": true, "atsauces": true,
	// German
	"literaturverzeichnis": true, "quellen": true, "quellenverzeichnis": true, "literatur": true,
	"bibliografie": true, "literatur- und quellenverzeichnis": true,
	// French
	"bibliographie": true, "références": true, "références bibliographiques": true, "sources et bibliographie": true,
	// Spanish, Italian, Portuguese
	"bibliografía": true, "referencias": true, "referencias bibliográficas": true, "bibliografia": true,
	"riferimenti": true, "riferimenti bibliografici": true, "referências": true,
	"referências bibliográficas": true, "fontes": true,
	// Dutch, Scandinavian, Finnish, Estonian, Lithuanian, Polish
	"literatuur": true, "literatuurlijst": true, "referenties": true, "bronnen": true,
	"bibliografi": true, "referenser": true, "litteratur": true, "källor": true, "kilder": true,
	"litteraturliste": true, "kirjallisuus": true, "lähteet": true, "kirjandus": true,
	"kasutatud kirjandus": true, "viited": true, "literatūros sąrašas": true, "šaltiniai": true,
	"naudota literatūra": true, "bibliografija": true, "piśmiennictwo": true, "źródła": true,
	"literatura": true, "bibliografia załącznikowa": true,
	// Russian, Ukrainian
	"источники": true, "литература": true, "список литературы": true,
	"список использованной литературы": true, "библиография": true, "список источников": true,
	"использованная литература": true, "джерела": true, "список використаних джерел": true,
}

// LinkPlainReferences handles documents without structured references.
// It finds a reference section — a heading titled "References",
// "Bibliography", "Literatūra", "Literaturverzeichnis" … followed by a list
// or a run of paragraphs — and replaces those entries with one
// ast.ReferenceList. Numbered entries ("[1] …", "1. …", "1) …", ordered
// lists) keep their labels; author-date entries are unlabelled. In-text
// numeric citations matching the labels ("[1]", "[1, 3]", "[1–3]",
// "[2,4-6]") are then wrapped in links to "#ref-N", keeping the original
// characters. Code, math, links and URLs are left alone. It returns the
// number of entries found and does nothing when the document already has
// structured citations or no entries follow the heading.
func LinkPlainReferences(doc *ast.Document) int {
	if doc == nil || hasStructuredRefs(doc) {
		return 0
	}
	blocks, entries, ok := convertSection(doc.Blocks)
	if !ok {
		return 0
	}
	doc.Blocks = blocks
	labels := map[string]bool{}
	for _, e := range entries {
		if e.Label != "" {
			labels[e.Label] = true
		}
	}
	if len(labels) > 0 {
		linkBlocks(doc.Meta.Abstract, labels)
		linkBlocks(doc.Blocks, labels)
	}
	return len(entries)
}

func hasStructuredRefs(doc *ast.Document) bool {
	found := false
	check := func(in ast.Inline) {
		if _, ok := in.(*ast.Cite); ok {
			found = true
		}
	}
	ast.WalkInlines(doc.Meta.Abstract, check)
	ast.WalkInlines(doc.Blocks, check)
	ast.WalkBlocks(doc.Blocks, func(b ast.Block) bool {
		switch b.(type) {
		case *ast.ReferenceList, *ast.Bibliography:
			found = true
		}
		return !found
	})
	return found
}

// convertSection looks for the last reference heading followed by entries,
// searching top-level blocks first and then section containers.
func convertSection(blocks []ast.Block) ([]ast.Block, []ast.ReferenceEntry, bool) {
	for i := len(blocks) - 1; i >= 0; i-- {
		h, ok := blocks[i].(*ast.Heading)
		if !ok || !isReferenceTitle(ast.PlainText(h.Inlines)) {
			continue
		}
		entries, end := collectEntries(blocks[i+1:])
		if len(entries) == 0 {
			continue
		}
		out := make([]ast.Block, 0, len(blocks)-end+1)
		out = append(out, blocks[:i+1]...)
		out = append(out, &ast.ReferenceList{Entries: entries})
		out = append(out, blocks[i+1+end:]...)
		return out, entries, true
	}
	for i := len(blocks) - 1; i >= 0; i-- {
		if d, ok := blocks[i].(*ast.Div); ok {
			if inner, entries, ok := convertSection(d.Blocks); ok {
				d.Blocks = inner
				return blocks, entries, true
			}
		}
	}
	return blocks, nil, false
}

// isReferenceTitle matches heading text against the known titles,
// ignoring numbering ("7.", "7.1", "VII.", "A.") and a trailing colon.
func isReferenceTitle(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimRight(s, ":. \u00a0")
	if i := strings.IndexByte(s, ' '); i > 0 && isSectionNumber(s[:i]) {
		s = strings.TrimSpace(s[i+1:])
	}
	return referenceTitles[strings.Join(strings.Fields(s), " ")]
}

func isSectionNumber(w string) bool {
	w = strings.TrimRight(w, ".)")
	if w == "" {
		return false
	}
	digits := true
	for _, r := range w {
		if (r < '0' || r > '9') && r != '.' {
			digits = false
			break
		}
	}
	if digits {
		return true
	}
	roman := true
	for _, r := range w {
		if !strings.ContainsRune("ivxlcdm", r) {
			roman = false
			break
		}
	}
	return roman || (utf8.RuneCountInString(w) == 1 && unicode.IsLetter([]rune(w)[0]))
}

// candidate is one possible entry with its detected label.
type candidate struct {
	label string
	ins   []ast.Inline
}

// collectEntries turns the blocks after a reference heading into entries
// and reports how many blocks were consumed.
func collectEntries(blocks []ast.Block) ([]ast.ReferenceEntry, int) {
	var cands []candidate
	numeric := false
	accept := func(c candidate) bool {
		if len(c.ins) == 0 {
			return true
		}
		if len(cands) == 0 {
			numeric = c.label != ""
			if !numeric && !looksLikeReference(c.ins) {
				return false
			}
			cands = append(cands, c)
			return true
		}
		switch {
		case numeric && c.label == "":
			if !looksLikeReference(c.ins) && len(cands) > 0 {
				return false
			}
			prev := &cands[len(cands)-1]
			prev.ins = append(append(prev.ins, &ast.Text{Value: " "}), c.ins...)
		case !numeric && !looksLikeReference(c.ins):
			return false
		default:
			cands = append(cands, c)
		}
		return true
	}
	consumed := 0
loop:
	for _, b := range blocks {
		var cs []candidate
		switch n := b.(type) {
		case *ast.List:
			cs = listCandidates(n)
		case *ast.Para:
			cs = paragraphCandidates(n.Inlines)
		case *ast.Plain:
			cs = paragraphCandidates(n.Inlines)
		case *ast.LineBlock:
			for _, line := range n.Lines {
				label, ins := stripLabel(line)
				cs = append(cs, candidate{label, ins})
			}
		default:
			break loop
		}
		before := len(cands)
		for _, c := range cs {
			if !accept(c) {
				if len(cands) == before {
					break loop // nothing from this block was taken
				}
				break
			}
		}
		consumed++
	}
	if len(cands) == 0 {
		return nil, 0
	}
	entries := make([]ast.ReferenceEntry, 0, len(cands))
	taken := map[string]bool{}
	for i, c := range cands {
		id := "ref-" + strconv.Itoa(i+1)
		if numeric {
			id = EntryID(c.label)
		}
		base := id
		for k := 2; taken[id]; k++ {
			id = base + "-" + strconv.Itoa(k)
		}
		taken[id] = true
		label := ""
		if numeric {
			label = c.label
		}
		entries = append(entries, ast.ReferenceEntry{ID: id, Label: label, Inlines: ast.MergeText(c.ins)})
	}
	return entries, consumed
}

func listCandidates(l *ast.List) []candidate {
	start := l.Start
	if start == 0 {
		start = 1
	}
	var out []candidate
	for i, it := range l.Items {
		var ins []ast.Inline
		for _, b := range it.Blocks {
			var part []ast.Inline
			switch n := b.(type) {
			case *ast.Para:
				part = n.Inlines
			case *ast.Plain:
				part = n.Inlines
			default:
				continue
			}
			if len(ins) > 0 {
				ins = append(ins, &ast.Text{Value: " "})
			}
			ins = append(ins, part...)
		}
		label, rest := stripLabel(ins)
		if label == "" && l.Ordered && l.Style == ast.NumberDecimal {
			label = strconv.Itoa(start + i)
		}
		out = append(out, candidate{label, rest})
	}
	return out
}

// paragraphCandidates splits a paragraph into entries at line breaks
// followed by a label; unlabelled lines are split only when every line
// looks like a reference of its own.
func paragraphCandidates(ins []ast.Inline) []candidate {
	var lines [][]ast.Inline
	var cur []ast.Inline
	for _, in := range ins {
		switch in.(type) {
		case *ast.LineBreak, *ast.SoftBreak:
			lines = append(lines, cur)
			cur = nil
			continue
		}
		cur = append(cur, in)
	}
	lines = append(lines, cur)
	if len(lines) == 1 {
		label, rest := stripLabel(lines[0])
		return []candidate{{label, rest}}
	}
	labelled := false
	allRefs := true
	for _, l := range lines {
		if lab, _ := stripLabel(l); lab != "" {
			labelled = true
		}
		if !looksLikeReference(l) || !startsUpper(l) {
			allRefs = false
		}
	}
	var out []candidate
	switch {
	case labelled:
		for _, l := range lines {
			label, rest := stripLabel(l)
			if label == "" && len(out) > 0 {
				prev := &out[len(out)-1]
				prev.ins = append(append(prev.ins, &ast.Text{Value: " "}), ast.TrimInlines(l)...)
				continue
			}
			out = append(out, candidate{label, rest})
		}
	case allRefs:
		for _, l := range lines {
			out = append(out, candidate{"", ast.TrimInlines(l)})
		}
	default:
		// One entry wrapped over several lines.
		var joined []ast.Inline
		for i, l := range lines {
			if i > 0 {
				joined = append(joined, &ast.Text{Value: " "})
			}
			joined = append(joined, l...)
		}
		label, rest := stripLabel(joined)
		out = append(out, candidate{label, rest})
	}
	return out
}

func startsUpper(ins []ast.Inline) bool {
	r := firstLetter(ast.PlainText(ins))
	return r != 0 && unicode.IsUpper(r)
}

// stripLabel removes a leading "[1]", "1.", "1)" or "(1)" label.
func stripLabel(ins []ast.Inline) (string, []ast.Inline) {
	ins = ast.TrimInlines(ins)
	if len(ins) == 0 {
		return "", nil
	}
	switch first := ins[0].(type) {
	case *ast.Text:
		if label, n := leadingLabel(first.Value); label != "" {
			rest := strings.TrimLeft(first.Value[n:], " \t\u00a0")
			out := make([]ast.Inline, 0, len(ins))
			if rest != "" {
				out = append(out, &ast.Text{Value: rest})
			}
			return label, ast.TrimInlines(append(out, ins[1:]...))
		}
	case *ast.Strong, *ast.Emph, *ast.Span, *ast.Superscript:
		pt := ast.PlainText(ins[:1])
		if label, n := leadingLabel(pt); label != "" && strings.TrimSpace(pt[n:]) == "" {
			return label, ast.TrimInlines(ins[1:])
		}
	}
	return "", ins
}

// leadingLabel recognises a reference label at the start of s and returns
// it with the number of bytes it occupies.
func leadingLabel(s string) (string, int) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	open := byte(0)
	if i < len(s) && (s[i] == '[' || s[i] == '(') {
		open = s[i]
		i++
	}
	start := i
	for i < len(s) && i-start < 4 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start || (i < len(s) && s[i] >= '0' && s[i] <= '9') {
		return "", 0
	}
	num := s[start:i]
	switch {
	case open == '[' && i < len(s) && s[i] == ']':
		i++
	case open == '(' && i < len(s) && s[i] == ')':
		i++
	case open == 0 && i < len(s) && (s[i] == '.' || s[i] == ')'):
		i++
	default:
		return "", 0
	}
	// The label must be followed by white space (or end the text node);
	// "1.5 m" and "2020." are not labels.
	if i < len(s) && s[i] != ' ' && s[i] != '\t' && !strings.HasPrefix(s[i:], "\u00a0") {
		return "", 0
	}
	if n, _ := strconv.Atoi(num); n == 0 || (open == 0 && n > 999) {
		return "", 0
	}
	return strings.TrimLeft(num, "0"), i
}

// looksLikeReference requires a year, "n.d." or a URL/DOI so ordinary
// prose under a "Sources" heading is not mistaken for a reference list.
func looksLikeReference(ins []ast.Inline) bool {
	s := ast.PlainText(ins)
	if utf8.RuneCountInString(s) < 12 {
		return false
	}
	low := strings.ToLower(s)
	for _, k := range []string{"http://", "https://", "doi:", "doi.org", "n.d.", "b. g.", "s.d.", "o. j.", "s. f."} {
		if strings.Contains(low, k) {
			return true
		}
	}
	for i := 0; i+4 <= len(s); i++ {
		if (i == 0 || !isDigitByte(s[i-1])) && isYear(s[i:i+4]) && (i+4 == len(s) || !isDigitByte(s[i+4])) {
			return true
		}
	}
	return false
}

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }

func isYear(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1450 && n <= 2199
}

// ---------------------------------------------------------------------------
// In-text numeric citations
// ---------------------------------------------------------------------------

func linkBlocks(blocks []ast.Block, labels map[string]bool) {
	for _, b := range blocks {
		switch n := b.(type) {
		case *ast.Para:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Plain:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Heading:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.BlockQuote:
			linkBlocks(n.Blocks, labels)
		case *ast.Div:
			n.Title = linkInlines(n.Title, labels)
			linkBlocks(n.Blocks, labels)
		case *ast.List:
			for _, it := range n.Items {
				linkBlocks(it.Blocks, labels)
			}
		case *ast.DefinitionList:
			for i := range n.Items {
				n.Items[i].Term = linkInlines(n.Items[i].Term, labels)
				for _, d := range n.Items[i].Definitions {
					linkBlocks(d, labels)
				}
			}
		case *ast.Table:
			n.Caption = linkInlines(n.Caption, labels)
			for _, rows := range [][]ast.Row{n.Head, n.Body, n.Foot} {
				for _, r := range rows {
					for _, c := range r.Cells {
						linkBlocks(c.Blocks, labels)
					}
				}
			}
		case *ast.Figure:
			n.Caption = linkInlines(n.Caption, labels)
		case *ast.CodeBlock:
			n.Caption = linkInlines(n.Caption, labels)
		case *ast.LineBlock:
			for i := range n.Lines {
				n.Lines[i] = linkInlines(n.Lines[i], labels)
			}
		}
	}
}

func linkInlines(ins []ast.Inline, labels map[string]bool) []ast.Inline {
	if len(ins) == 0 {
		return ins
	}
	ins = ast.MergeText(ins)
	out := make([]ast.Inline, 0, len(ins))
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			if pieces := splitCitations(n.Value, labels); pieces != nil {
				out = append(out, pieces...)
				continue
			}
		case *ast.Emph:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Strong:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Underline:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Strike:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Superscript:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Subscript:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.SmallCaps:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Highlight:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Span:
			n.Inlines = linkInlines(n.Inlines, labels)
		case *ast.Note:
			linkBlocks(n.Blocks, labels)
		}
		out = append(out, in)
	}
	return out
}

// citeNum is one number inside a bracketed citation.
type citeNum struct {
	start, end int
	value      string
}

// splitCitations wraps the numbers of bracketed citations whose numbers
// are all known labels; it returns nil when s has none.
func splitCitations(s string, labels map[string]bool) []ast.Inline {
	if !strings.Contains(s, "[") {
		return nil
	}
	var out []ast.Inline
	last := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '[' || inURL(s, i) {
			continue
		}
		nums, end, ok := parseCiteGroup(s, i+1)
		if !ok {
			continue
		}
		valid := true
		for _, n := range nums {
			if !labels[n.value] {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		pos := last
		for _, n := range nums {
			if n.start > pos {
				out = append(out, &ast.Text{Value: s[pos:n.start]})
			}
			out = append(out, &ast.Link{URL: "#" + EntryID(n.value), Inlines: []ast.Inline{&ast.Text{Value: s[n.start:n.end]}}})
			pos = n.end
		}
		out = append(out, &ast.Text{Value: s[pos:end]})
		last = end
		i = end - 1
	}
	if out == nil {
		return nil
	}
	if last < len(s) {
		out = append(out, &ast.Text{Value: s[last:]})
	}
	return ast.MergeText(out)
}

// inURL reports whether position i lies inside a URL-like token.
func inURL(s string, i int) bool {
	start := strings.LastIndexAny(s[:i], " \t\n\u00a0") + 1
	tok := strings.ToLower(s[start:i])
	return strings.Contains(tok, "://") || strings.HasPrefix(tok, "www.")
}

// parseCiteGroup parses "1, 3–5]" starting after the "[" and returns the
// numbers and the index just past "]".
func parseCiteGroup(s string, i int) ([]citeNum, int, bool) {
	var nums []citeNum
	skip := func() {
		for i < len(s) && s[i] == ' ' {
			i++
		}
	}
	number := func() bool {
		start := i
		for i < len(s) && i-start < 4 && isDigitByte(s[i]) {
			i++
		}
		if i == start || (i < len(s) && isDigitByte(s[i])) {
			return false
		}
		v := strings.TrimLeft(s[start:i], "0")
		if v == "" {
			return false
		}
		nums = append(nums, citeNum{start, i, v})
		return true
	}
	for {
		skip()
		if !number() {
			return nil, 0, false
		}
		skip()
		if r, _ := utf8.DecodeRuneInString(s[i:]); isDash(r) {
			from := nums[len(nums)-1]
			for {
				r, size := utf8.DecodeRuneInString(s[i:])
				if !isDash(r) {
					break
				}
				i += size
			}
			skip()
			if !number() {
				return nil, 0, false
			}
			to, _ := strconv.Atoi(nums[len(nums)-1].value)
			fr, _ := strconv.Atoi(from.value)
			if to <= fr {
				return nil, 0, false
			}
			skip()
		}
		if i >= len(s) {
			return nil, 0, false
		}
		switch s[i] {
		case ',', ';':
			i++
		case ']':
			return nums, i + 1, true
		default:
			return nil, 0, false
		}
	}
}
