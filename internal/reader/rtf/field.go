package rtf

import (
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// fieldRec is a \field being parsed.
type fieldRec struct {
	inst   []byte
	inInst *fieldRec // enclosing field whose instruction this field sits in
	images int       // picture count when the field started
	runs   int       // paragraph run count when the result started

	parsed   bool
	kind     string
	args     []string
	switches map[string]string
	flags    map[string]bool
	result   bool
}

func (f *fieldRec) parse() {
	if f.parsed {
		return
	}
	f.parsed = true
	f.switches = map[string]string{}
	f.flags = map[string]bool{}
	toks := splitInstr(string(f.inst))
	if len(toks) == 0 {
		return
	}
	f.kind = strings.ToUpper(toks[0].text)
	for i := 1; i < len(toks); i++ {
		t := toks[i]
		if !t.quoted && strings.HasPrefix(t.text, `\`) && len(t.text) > 1 {
			name := strings.ToLower(t.text[1:])
			switch name {
			case "l", "o", "t", "f", "m", "*", "s", "h":
				if name != "h" && i+1 < len(toks) {
					f.switches[name] = toks[i+1].text
					i++
					continue
				}
			}
			f.flags[name] = true
			continue
		}
		f.args = append(f.args, t.text)
	}
}

func (f *fieldRec) arg(i int) string {
	if i < len(f.args) {
		return f.args[i]
	}
	return ""
}

type instrTok struct {
	text   string
	quoted bool
}

// splitInstr splits a field instruction into words, honouring quotes and
// the field-code escapes \\ and \".
func splitInstr(s string) []instrTok {
	var out []instrTok
	for i := 0; i < len(s); {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			i++
			continue
		}
		if c == '"' || strings.HasPrefix(s[i:], "“") {
			open := 1
			if c != '"' {
				open = len("“")
			}
			var b strings.Builder
			j := i + open
			for j < len(s) && s[j] != '"' && !strings.HasPrefix(s[j:], "”") {
				if s[j] == '\\' && j+1 < len(s) && (s[j+1] == '\\' || s[j+1] == '"') {
					b.WriteByte(s[j+1])
					j += 2
					continue
				}
				b.WriteByte(s[j])
				j++
			}
			out = append(out, instrTok{text: b.String(), quoted: true})
			if j < len(s) && s[j] == '"' {
				j++
			} else if j < len(s) {
				j += len("”")
			}
			i = j
			continue
		}
		j := i
		for j < len(s) && s[j] != ' ' && s[j] != '\t' && s[j] != '\r' && s[j] != '\n' && s[j] != '"' {
			j++
		}
		out = append(out, instrTok{text: s[i:j]})
		i = j
	}
	return out
}

// fieldResult starts a \fldrslt group: hyperlinks turn the result into a
// link, tables of contents are dropped (the writer builds its own).
func (p *parser) fieldResult(st *state) {
	f := st.field
	st.dest = dNormal
	if f == nil {
		return
	}
	f.parse()
	f.result = true
	f.runs = len(p.cur().para.items)
	if f.inInst != nil {
		st.dest = dFldInst
		st.field = f.inInst
		return
	}
	switch f.kind {
	case "TOC", "INDEX", "TOA", "BIBLIOGRAPHY":
		if f.kind == "BIBLIOGRAPHY" {
			return // keep the rendered bibliography text
		}
		p.skip()
	case "HYPERLINK":
		url := strings.TrimSpace(f.arg(0))
		if a := f.switches["l"]; a != "" {
			if url == "" {
				id := sanitizeID(a)
				p.targets[id] = true
				url = "#" + id
			} else {
				url += "#" + a
			}
		}
		if url == "" {
			return
		}
		p.links = append(p.links, linkInfo{url: url, title: f.switches["o"]})
		st.chr.link = len(p.links)
	case "REF", "NOTEREF":
		if f.flags["h"] {
			if id := sanitizeID(f.arg(0)); id != "" {
				p.targets[id] = true
				p.links = append(p.links, linkInfo{url: "#" + id})
				st.chr.link = len(p.links)
			}
		}
	}
}

// endField handles fields whose meaning is not carried by a result:
// SYMBOL characters and linked (not embedded) pictures.
func (p *parser) endField(f *fieldRec) {
	if f == nil || f.inInst != nil {
		return
	}
	f.parse()
	emptyResult := !f.result || len(p.cur().para.items) == f.runs
	switch f.kind {
	case "SYMBOL":
		if !emptyResult {
			return
		}
		code, err := strconv.ParseInt(strings.TrimPrefix(strings.ToLower(f.arg(0)), "0x"), base(f.arg(0)), 32)
		if err != nil || code <= 0 {
			return
		}
		font := strings.ToLower(f.switches["f"])
		var r rune
		switch {
		case strings.Contains(font, "symbol") && code < 256:
			r = symbolRune(byte(code))
		case strings.Contains(font, "wingdings") && code < 256:
			r = wingdingsRune(byte(code))
		case code < 256:
			if d := []rune(p.dec.decode([]byte{byte(code)}, p.docCP)); len(d) > 0 {
				r = d[0]
			}
		default:
			r = rune(code)
		}
		if r > 0 {
			p.emit(string(r))
		}
	case "INCLUDEPICTURE":
		if p.images != f.images {
			return
		}
		url := strings.TrimSpace(f.arg(0))
		low := strings.ToLower(url)
		if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
			p.paraInline(&ast.Image{Src: url})
		} else if url != "" {
			p.warn.Addf("linked picture %q is not embedded in the document and was dropped", url)
		}
	}
}

func base(s string) int {
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		return 16
	}
	return 10
}
