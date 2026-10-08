package docx

import (
	"encoding/xml"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// instr is a parsed field instruction.
type instr struct {
	typ  string // upper-case field type
	toks []token
	raw  string
}

type token struct {
	s      string
	sw     bool // a \switch
	quoted bool
}

func parseInstr(s string) instr {
	in := instr{raw: s, toks: tokenizeInstr(s)}
	if len(in.toks) > 0 {
		in.typ = strings.ToUpper(in.toks[0].s)
		in.toks = in.toks[1:]
	}
	return in
}

func tokenizeInstr(s string) []token {
	var out []token
	rs := []rune(s)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ' ':
			i++
		case r == '"' || r == '“' || r == '”':
			j := i + 1
			var sb strings.Builder
			for j < len(rs) && rs[j] != '"' && rs[j] != '”' && rs[j] != '“' {
				if rs[j] == '\\' && j+1 < len(rs) && (rs[j+1] == '"' || rs[j+1] == '\\') {
					j++
				}
				sb.WriteRune(rs[j])
				j++
			}
			out = append(out, token{s: sb.String(), quoted: true})
			i = j + 1
		case r == '\\' && i+1 < len(rs):
			j := i + 2
			out = append(out, token{s: string(rs[i:j]), sw: true})
			i = j
		default:
			j := i
			for j < len(rs) && rs[j] != ' ' && rs[j] != '\t' && rs[j] != '\n' && rs[j] != '\r' && rs[j] != '"' {
				j++
			}
			out = append(out, token{s: string(rs[i:j])})
			i = j
		}
	}
	return out
}

// args returns the positional arguments, skipping switches and the
// arguments switches consume.
func (in instr) args() []string {
	var out []string
	for i := 0; i < len(in.toks); i++ {
		t := in.toks[i]
		if t.sw {
			if switchTakesArg(in.toks, i) {
				i++
			}
			continue
		}
		out = append(out, t.s)
	}
	return out
}

func switchTakesArg(toks []token, i int) bool {
	return i+1 < len(toks) && !toks[i+1].sw
}

// switchArg returns the argument of switch name (e.g. `\l`).
func (in instr) switchArg(name string) string {
	for i, t := range in.toks {
		if t.sw && strings.EqualFold(t.s, name) && switchTakesArg(in.toks, i) {
			return in.toks[i+1].s
		}
	}
	return ""
}

func (in instr) hasSwitch(name string) bool {
	for _, t := range in.toks {
		if t.sw && strings.EqualFold(t.s, name) {
			return true
		}
	}
	return false
}

// resolveField decides, once the instruction is complete, what happens to
// the field result.
func (pb *paraBuilder) resolveField(f *frame) {
	f.inInstr = false
	in := parseInstr(f.instr.String())
	args := in.args()
	switch in.typ {
	case "HYPERLINK":
		anchor := strings.TrimSpace(in.switchArg(`\l`))
		target := ""
		if len(args) > 0 {
			target = safeURL(args[0])
		}
		switch {
		case target != "":
			if anchor != "" && !strings.Contains(target, "#") {
				target += "#" + anchor
			}
			f.mode, f.url = modeLink, target
		case anchor != "":
			f.mode, f.anchor = modeLink, anchor
		}
	case "REF":
		if in.hasSwitch(`\h`) && len(args) > 0 {
			f.mode, f.anchor = modeLink, args[0]
		}
	case "TOC", "TOA", "INDEX", "TA", "XE", "TC", "RD", "PRIVATE", "MACROBUTTON", "GOTOBUTTON", "ADVANCE":
		f.mode = modeDrop
	case "SEQ":
		if pb.props.kind == kindCaption {
			if len(args) > 0 {
				pb.seqIDs = append(pb.seqIDs, args[0])
			}
			f.mode = modeDrop
		}
	case "BIBLIOGRAPHY":
		pb.emitMark(&ast.Bibliography{})
		f.mode = modeDrop
	case "CITATION":
		if items := pb.r.builtinCitation(in); len(items) > 0 {
			f.mode, f.cite = modeCite, items
		}
	case "ADDIN":
		pb.addin(f, in)
	case "EQ":
		pb.r.warn.Addf("legacy EQ field equation was dropped")
	case "FORMCHECKBOX":
		pb.formCheckBox(f)
	}
}

// addin handles fields inserted by reference-manager add-ins.
func (pb *paraBuilder) addin(f *frame, in instr) {
	args := in.args()
	sub := ""
	if len(args) > 0 {
		sub = strings.ToUpper(args[0])
	}
	upper := strings.ToUpper(in.raw)
	switch {
	case strings.Contains(upper, "ZOTERO_BIBL") || strings.Contains(upper, "CSL_BIBLIOGRAPHY") || sub == "EN.REFLIST":
		pb.emitMark(&ast.Bibliography{})
		f.mode = modeDrop
	case strings.Contains(upper, "CSL_CITATION") || sub == "ZOTERO_ITEM":
		if items, ok := pb.r.cslCitation(in.raw); ok {
			f.mode, f.cite = modeCite, items
		} else {
			pb.r.warn.Addf("citation field with unreadable data kept as plain text")
		}
	case sub == "EN.CITE":
		if items, ok := pb.r.recordCitation(in.raw); ok {
			f.mode, f.cite = modeCite, items
		} else {
			pb.r.warn.Addf("citation field without embedded record data kept as plain text")
		}
	}
}

// formCheckBox renders a legacy form check box, which has no field result.
func (pb *paraBuilder) formCheckBox(f *frame) {
	cb := f.ffData.child("checkBox")
	checked := onOff(cb.child("checked"))
	if cb.child("checked") == nil {
		checked = onOff(cb.child("default"))
	}
	mark := "☐"
	if checked {
		mark = "☒"
	}
	// The frame is already out of its instruction phase, so the mark is
	// emitted like ordinary field result text.
	pb.emit(seg{text: mark})
}

// safeURL returns u when it is an acceptable link target.
func safeURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	for _, r := range u {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	if i := strings.IndexByte(u, ':'); i > 0 {
		scheme := strings.ToLower(u[:i])
		valid := true
		for _, r := range scheme {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.') {
				valid = false
				break
			}
		}
		if valid && len(scheme) > 1 {
			switch scheme {
			case "http", "https", "mailto", "ftp", "ftps", "sftp", "tel", "news", "irc", "urn", "doi":
				return u
			}
			return ""
		}
	}
	return u
}

// scanReferences records the bookmark names that links and reference
// fields point at, ignoring the generated table of contents.
func (r *reader) scanReferences(data []byte) {
	dec := xml.NewDecoder(strings.NewReader(string(toUTF8(data))))
	dec.Strict = false
	dec.CharsetReader = charsetReader
	type fld struct {
		instr    strings.Builder
		toc      bool
		resolved bool
	}
	var fields []*fld
	var sdtTOC []bool
	inInstr := false
	inTOC := func() bool {
		for _, f := range fields {
			if f.toc {
				return true
			}
		}
		for _, t := range sdtTOC {
			if t {
				return true
			}
		}
		return false
	}
	record := func(s string) {
		in := parseInstr(s)
		switch in.typ {
		case "REF", "PAGEREF", "NOTEREF", "GOTOBUTTON":
			if a := in.args(); len(a) > 0 {
				r.referenced[a[0]] = true
			}
		case "HYPERLINK":
			if a := strings.TrimSpace(in.switchArg(`\l`)); a != "" && len(in.args()) == 0 {
				r.referenced[a] = true
			}
		}
	}
	resolve := func(f *fld) {
		if f.resolved {
			return
		}
		f.resolved = true
		s := f.instr.String()
		switch parseInstr(s).typ {
		case "TOC", "TOA", "INDEX":
			f.toc = true
		default:
			if !inTOC() {
				record(s)
			}
		}
	}
	for {
		tok, err := dec.RawToken()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "fldChar":
				switch rawAttr(t, "fldCharType") {
				case "begin":
					fields = append(fields, &fld{})
				case "separate":
					if len(fields) > 0 {
						resolve(fields[len(fields)-1])
					}
				case "end":
					if len(fields) > 0 {
						resolve(fields[len(fields)-1])
						fields = fields[:len(fields)-1]
					}
				}
			case "instrText":
				inInstr = true
			case "fldSimple":
				if !inTOC() {
					record(rawAttr(t, "instr"))
				}
			case "hyperlink":
				if a := strings.TrimSpace(rawAttr(t, "anchor")); a != "" && !inTOC() {
					r.referenced[a] = true
				}
			case "sdt":
				sdtTOC = append(sdtTOC, false)
			case "docPartGallery":
				if len(sdtTOC) > 0 && strings.Contains(strings.ToLower(rawAttr(t, "val")), "table of contents") {
					sdtTOC[len(sdtTOC)-1] = true
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "instrText":
				inInstr = false
			case "sdt":
				if len(sdtTOC) > 0 {
					sdtTOC = sdtTOC[:len(sdtTOC)-1]
				}
			}
		case xml.CharData:
			if inInstr && len(fields) > 0 {
				fields[len(fields)-1].instr.Write(t)
			}
		}
	}
}

func rawAttr(t xml.StartElement, local string) string {
	for _, a := range t.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Caption labels

// capKind classifies caption paragraphs. capUnstyled marks text that only
// looks like a caption ("Figure 2: ...") without a caption style.
type capKind uint8

const (
	capNone     capKind = 0
	capTable    capKind = 1
	capFigure   capKind = 2
	capOther    capKind = 4
	capUnstyled capKind = 8
)

func (k capKind) base() capKind { return k &^ capUnstyled }

// Caption labels in the languages documents commonly use. Entries ending
// in '.' are abbreviations.
var (
	tableLabels = []string{
		"table", "tabula", "tabelle", "tableau", "tabla", "tab.", "tabel", "tabell", "taulukko",
		"tabulka", "tabela", "tavola", "tabelul", "lentelė", "lentele", "tabelis", "таблица",
		"táblázat", "tablo", "πίνακας",
	}
	figureLabels = []string{
		"figure", "fig.", "fig", "attēls", "attels", "abbildung", "abb.", "figura", "figur",
		"illustration", "bild", "kuva", "obrázek", "obr.", "rysunek", "rys.", "pav.", "paveikslas",
		"joonis", "afbeelding", "рисунок", "рис.", "ábra", "şekil", "chart", "diagram",
		"diagramma", "grafiks", "grafik", "photo", "foto", "image", "plate", "exhibit", "scheme",
		"schema", "shēma",
	}
	labelFirstRe = regexp.MustCompile(`^(?i)\s*(` + labelAlternation() + `)(\s*[A-Z]?\d+(?:[.\-–]\d+)*[a-z]?)?(\s*[:.\-–—|)]+\s*)?`)
	numFirstRe   = regexp.MustCompile(`^(?i)\s*\d+(?:\.\d+)*\.?\s*(` + labelAlternation() + `)\.?(\s*[:.\-–—|)]+\s*|\s+|$)`)
)

// labelAlternation builds a regexp alternation of all labels, longest
// first so "tableau" is not cut short by "table".
func labelAlternation() string {
	all := append(append([]string{}, tableLabels...), figureLabels...)
	sort.Slice(all, func(i, j int) bool { return len(all[i]) > len(all[j]) })
	for i, l := range all {
		all[i] = regexp.QuoteMeta(l)
	}
	return strings.Join(all, "|")
}

// captionLabel recognises a leading caption label ("Table 3:", "Fig. 2.",
// "1. attēls.") and returns its kind and byte length (0 when absent). A
// label needs a number or a separator after it, so ordinary sentences
// starting with "Figure shows" are not mistaken for captions.
func captionLabel(s string) (capKind, int) {
	if !mayStartLabel(s) {
		return capNone, 0
	}
	if m := labelFirstRe.FindStringSubmatchIndex(s); m != nil && !letterAt(s, m[3]) {
		hasNum := m[4] >= 0 && m[5] > m[4]
		hasSep := m[6] >= 0 && m[7] > m[6]
		if hasNum || hasSep {
			end := m[1]
			for end < len(s) && s[end] == ' ' {
				end++
			}
			return labelKind(s[m[2]:m[3]]), end
		}
	}
	if m := numFirstRe.FindStringSubmatchIndex(s); m != nil && !letterAt(s, m[3]) {
		return labelKind(s[m[2]:m[3]]), m[1]
	}
	return capNone, 0
}

// labelInitials holds the lower-case first letters of all labels, so most
// paragraphs are rejected without running the regular expressions.
var labelInitials = func() map[rune]bool {
	m := map[rune]bool{}
	for _, l := range append(append([]string{}, tableLabels...), figureLabels...) {
		r, _ := utf8.DecodeRuneInString(l)
		m[r] = true
	}
	return m
}()

func mayStartLabel(s string) bool {
	s = strings.TrimLeft(s, " \t\u00a0")
	r, _ := utf8.DecodeRuneInString(s)
	return (r >= '0' && r <= '9') || labelInitials[unicode.ToLower(r)]
}

// letterAt reports whether a letter starts at byte offset i of s.
func letterAt(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return unicode.IsLetter(r)
}

// labelKind classifies a caption label or SEQ identifier.
func labelKind(label string) capKind {
	l := strings.ToLower(strings.TrimSpace(label))
	for _, t := range tableLabels {
		if l == t || l+"." == t {
			return capTable
		}
	}
	for _, f := range figureLabels {
		if l == f || l+"." == f {
			return capFigure
		}
	}
	return capOther
}

// stripCaption removes the leading label from caption inlines.
func stripCaption(ins []ast.Inline) []ast.Inline {
	lead, _ := leadingText(ins)
	if _, n := captionLabel(lead); n > 0 {
		ins = dropPrefix(ins, n)
	}
	return ast.TrimInlines(ins)
}
