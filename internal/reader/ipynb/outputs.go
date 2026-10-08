package ipynb

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

type output struct {
	OutputType     string                     `json:"output_type"`
	Name           string                     `json:"name"`
	Stream         string                     `json:"stream"` // nbformat 3
	Text           json.RawMessage            `json:"text"`
	Data           map[string]json.RawMessage `json:"data"`
	Metadata       map[string]json.RawMessage `json:"metadata"`
	ExecutionCount json.RawMessage            `json:"execution_count"`
	PromptNumber   json.RawMessage            `json:"prompt_number"`
	Ename          string                     `json:"ename"`
	Evalue         string                     `json:"evalue"`
	Traceback      []string                   `json:"traceback"`
	raw            map[string]json.RawMessage
}

// v3Keys maps nbformat 3 output keys to MIME types.
var v3Keys = map[string]string{
	"png": "image/png", "jpeg": "image/jpeg", "svg": "image/svg+xml", "html": "text/html",
	"latex": "text/latex", "markdown": "text/markdown", "json": "application/json", "text": "text/plain",
}

func (r *reader) code(c *cell, m cellMeta) []ast.Block {
	var blocks []ast.Block
	count := execCount(c)
	if !m.hideInput {
		src := asText(c.Source)
		if len(c.Source) == 0 {
			src = asText(c.Input)
		}
		if src = trimCode(src); src != "" {
			cb := &ast.CodeBlock{Lang: r.lang, Text: src, Attr: ast.Attr{Classes: []string{"input"}}}
			if count != "" {
				cb.Attr.KV = map[string]string{"execution_count": count}
			}
			blocks = append(blocks, cb)
		}
	}
	if m.hideOutput || len(c.Outputs) == 0 {
		return blocks
	}
	var raws []json.RawMessage
	if json.Unmarshal(c.Outputs, &raws) != nil {
		r.warn.Addf("a code cell has malformed outputs")
		return blocks
	}
	outs := make([]*output, 0, len(raws))
	for _, raw := range raws {
		o := &output{}
		if json.Unmarshal(raw, o) != nil {
			r.warn.Addf("a malformed output was skipped")
			continue
		}
		_ = json.Unmarshal(raw, &o.raw)
		outs = append(outs, o)
	}
	for _, o := range mergeStreams(outs) {
		blocks = append(blocks, r.output(o)...)
	}
	return blocks
}

// mergeStreams joins consecutive writes to the same stream, as a terminal
// would show them (progress bars rewrite lines across writes).
func mergeStreams(outs []*output) []*output {
	var res []*output
	for _, o := range outs {
		if o.OutputType == "stream" && len(res) > 0 {
			prev := res[len(res)-1]
			if prev.OutputType == "stream" && prev.streamName() == o.streamName() {
				merged := *prev
				merged.Text, _ = json.Marshal(asText(prev.Text) + asText(o.Text))
				res[len(res)-1] = &merged
				continue
			}
		}
		res = append(res, o)
	}
	return res
}

func (o *output) streamName() string {
	if o.Name != "" {
		return o.Name
	}
	return o.Stream
}

func (r *reader) output(o *output) []ast.Block {
	switch o.OutputType {
	case "stream":
		text := cleanTerminal(asText(o.Text))
		if strings.TrimSpace(text) == "" {
			return nil
		}
		classes := []string{"output"}
		if o.streamName() == "stderr" {
			classes = append(classes, "stderr")
		}
		return []ast.Block{&ast.CodeBlock{Text: text, Attr: ast.Attr{Classes: classes}}}
	case "execute_result", "display_data", "pyout", "update_display_data":
		data, meta := o.Data, o.Metadata
		if data == nil {
			data = map[string]json.RawMessage{}
			for k, mime := range v3Keys {
				if v, ok := o.raw[k]; ok {
					data[mime] = v
				}
			}
		}
		count := ""
		for _, raw := range []json.RawMessage{o.ExecutionCount, o.PromptNumber} {
			var n int
			if len(raw) > 0 && json.Unmarshal(raw, &n) == nil && n > 0 {
				count = strconv.Itoa(n)
			}
		}
		return r.richOutput(data, meta, count)
	case "error", "pyerr":
		return []ast.Block{r.errorOutput(o)}
	}
	return nil
}

// richOutput renders the best representation of a MIME bundle, falling back
// to the next one when a representation yields nothing.
func (r *reader) richOutput(data, meta map[string]json.RawMessage, count string) []ast.Block {
	for _, it := range imageTypes {
		raw, ok := data[it.mime]
		if !ok {
			continue
		}
		img := imageData(it.mime, asText(raw))
		if img == nil {
			continue
		}
		image := &ast.Image{Src: r.store("output", it.mime, img)}
		if w := imageWidth(meta[it.mime]); w != "" {
			image.Width = w
		}
		return []ast.Block{&ast.Figure{Image: image}}
	}
	if raw, ok := data["text/latex"]; ok {
		if b := latexBlock(asText(raw)); b != nil {
			return []ast.Block{b}
		}
	}
	if raw, ok := data["text/markdown"]; ok {
		if b := r.markdown(asText(raw)); len(b) > 0 {
			return b
		}
	}
	if raw, ok := data["text/html"]; ok {
		if b := r.html(asText(raw)); len(b) > 0 {
			return b
		}
	}
	if raw, ok := data["application/json"]; ok {
		var buf bytes.Buffer
		text := strings.TrimSpace(string(raw))
		if json.Indent(&buf, raw, "", "  ") == nil {
			text = buf.String()
		}
		return []ast.Block{outputBlock("json", text, count)}
	}
	for mime := range data {
		if strings.Contains(mime, "jupyter.widget") {
			r.warn.Addf("interactive widget output cannot be rendered and was dropped")
			return nil
		}
	}
	if raw, ok := data["text/plain"]; ok {
		if text := cleanTerminal(asText(raw)); strings.TrimSpace(text) != "" {
			return []ast.Block{outputBlock("", text, count)}
		}
		return nil
	}
	if len(data) > 0 {
		mimes := slices.Sorted(maps.Keys(data))
		r.warn.Addf("output of type %s cannot be rendered and was dropped", mimes[0])
	}
	return nil
}

func outputBlock(lang, text, count string) *ast.CodeBlock {
	cb := &ast.CodeBlock{Lang: lang, Text: text, Attr: ast.Attr{Classes: []string{"output"}}}
	if count != "" {
		cb.Attr.KV = map[string]string{"execution_count": count}
	}
	return cb
}

// imageWidth reads the display width (in CSS pixels) from output metadata.
func imageWidth(raw json.RawMessage) string {
	var m struct {
		Width json.Number `json:"width"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil || m.Width == "" {
		return ""
	}
	w, err := m.Width.Float64()
	if err != nil || w <= 0 {
		return ""
	}
	return strconv.FormatFloat(w, 'f', -1, 64) + "px"
}

func (r *reader) errorOutput(o *output) ast.Block {
	var lines []string
	for _, l := range o.Traceback {
		lines = append(lines, cleanTerminal(l))
	}
	text := strings.Join(lines, "\n")
	summary := strings.TrimSpace(cleanTerminal(o.Ename))
	if v := cleanTerminal(o.Evalue); v != "" {
		summary += ": " + v
	}
	if summary != "" && !strings.Contains(text, summary) {
		if text != "" {
			text += "\n"
		}
		text += summary
	}
	return &ast.CodeBlock{Text: strings.Trim(text, "\n"), Attr: ast.Attr{Classes: []string{"output", "error"}}}
}

// latexBlock turns a LaTeX output into display math when it is a single
// formula, or a raw LaTeX block otherwise (e.g. a tabular).
func latexBlock(s string) ast.Block {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if tex, ok := mathContent(s); ok {
		tex = strings.TrimSpace(tex)
		tex = strings.TrimSpace(strings.TrimPrefix(tex, `\displaystyle`))
		if tex == "" {
			return nil
		}
		return &ast.MathBlock{TeX: tex}
	}
	return &ast.RawBlock{Format: "latex", Text: s}
}

func mathContent(s string) (string, bool) {
	switch {
	case strings.HasPrefix(s, "$$") && strings.HasSuffix(s, "$$") && len(s) >= 4:
		inner := s[2 : len(s)-2]
		return inner, !strings.Contains(inner, "$$")
	case strings.HasPrefix(s, "$") && strings.HasSuffix(s, "$") && len(s) >= 2:
		inner := s[1 : len(s)-1]
		return inner, !hasUnescapedDollar(inner)
	case strings.HasPrefix(s, `\[`) && strings.HasSuffix(s, `\]`):
		inner := s[2 : len(s)-2]
		return inner, !strings.Contains(inner, `\]`)
	case strings.HasPrefix(s, `\(`) && strings.HasSuffix(s, `\)`):
		inner := s[2 : len(s)-2]
		return inner, !strings.Contains(inner, `\)`)
	}
	for _, env := range []string{"equation*", "equation", "displaymath"} {
		begin, end := `\begin{`+env+`}`, `\end{`+env+`}`
		if strings.HasPrefix(s, begin) && strings.HasSuffix(s, end) {
			inner := s[len(begin) : len(s)-len(end)]
			return inner, !strings.Contains(inner, begin)
		}
	}
	// Multi-line environments are valid display math once turned into their
	// "-ed" forms (align -> aligned).
	for _, env := range [][2]string{{"align*", "aligned"}, {"align", "aligned"}, {"gather*", "gathered"}, {"gather", "gathered"}} {
		begin, end := `\begin{`+env[0]+`}`, `\end{`+env[0]+`}`
		if strings.HasPrefix(s, begin) && strings.HasSuffix(s, end) {
			inner := s[len(begin) : len(s)-len(end)]
			if strings.Contains(inner, begin) {
				return "", false
			}
			return `\begin{` + env[1] + `}` + inner + `\end{` + env[1] + `}`, true
		}
	}
	return "", false
}

func hasUnescapedDollar(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '$':
			return true
		}
	}
	return false
}

// cleanTerminal strips ANSI escape sequences and applies carriage returns
// and backspaces the way a terminal displays them, so progress bars show
// only their final state.
func cleanTerminal(s string) string {
	s = stripANSI(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.ContainsAny(s, "\r\b") {
		return strings.TrimRight(s, "\n")
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.ContainsAny(line, "\r\b") {
			lines[i] = overlay(line)
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// overlay renders one terminal line: "\r" returns to column 0 and later
// text overwrites earlier text; "\b" moves one column back.
func overlay(line string) string {
	var buf []rune
	col := 0
	for _, r := range line {
		switch r {
		case '\r':
			col = 0
		case '\b':
			col = max(col-1, 0)
		default:
			if col < len(buf) {
				buf[col] = r
			} else {
				buf = append(buf, r)
			}
			col++
		}
	}
	return strings.TrimRight(string(buf), " ")
}

// stripANSI removes CSI, OSC and two-byte escape sequences.
func stripANSI(s string) string {
	if !strings.Contains(s, "\x1b") && !strings.Contains(s, "\u009b") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		csi := false
		switch {
		case c == 0x1b && i+1 < len(s) && s[i+1] == '[':
			i += 2
			csi = true
		case c == 0xc2 && i+1 < len(s) && s[i+1] == 0x9b: // U+009B single-character CSI
			i += 2
			csi = true
		case c == 0x1b && i+1 < len(s) && s[i+1] == ']':
			// OSC: terminated by BEL or ESC \
			i += 2
			for i < len(s) && s[i] != 0x07 && !(s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\') {
				i++
			}
			if i < len(s) && s[i] == 0x1b {
				i++
			}
			continue
		case c == 0x1b && i+2 < len(s) && (s[i+1] == '(' || s[i+1] == ')'):
			i += 2 // character set designation, e.g. ESC ( B
			continue
		case c == 0x1b:
			i++ // two-byte sequence such as ESC c
			continue
		default:
			b.WriteByte(c)
			continue
		}
		if csi {
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
				i++
			}
		}
	}
	return b.String()
}
