package ipynb

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

var pngBytes = func() []byte {
	b, _ := hex.DecodeString("89504e470d0a1a0a0000000d4948445200000001000000010806000000" +
		"1f15c4890000000d49444154789c6360000002000154a24f5d0000000049454e44ae426082")
	return b
}()

var pngB64 = base64.StdEncoding.EncodeToString(pngBytes)

func read(t *testing.T, nb string) (*ast.Document, []string) {
	t.Helper()
	doc, warns, err := Read(context.Background(), []byte(nb), rd.Options{Name: "nb.ipynb"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc.Resources == nil {
		t.Fatal("Resources not initialised")
	}
	return doc, warns
}

// nb builds an nbformat 4 notebook from cell JSON snippets.
func nb(meta string, cells ...string) string {
	if meta == "" {
		meta = `{"kernelspec": {"name": "python3", "language": "python"}, "language_info": {"name": "python"}}`
	}
	return `{"nbformat": 4, "nbformat_minor": 5, "metadata": ` + meta + `, "cells": [` + strings.Join(cells, ",") + `]}`
}

func code(src string, count int, outputs ...string) string {
	return `{"cell_type": "code", "execution_count": ` + itoa(count) + `, "metadata": {}, "source": ` + jsonStr(src) +
		`, "outputs": [` + strings.Join(outputs, ",") + `]}`
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func checkDump(t *testing.T, got []ast.Block, want string) {
	t.Helper()
	if d := dump(got); d != want {
		t.Errorf("blocks mismatch\n got:\n%s\nwant:\n%s", d, want)
	}
}

func TestMetadata(t *testing.T) {
	tests := []struct {
		name, meta, title, lang string
		authors                 []string
	}{
		{"objects", `{"title": "Analysis", "authors": [{"name": "Ada Lovelace"}, {"name": "Alan Turing"}], "language_info": {"name": "Python"}}`, "Analysis", "python", []string{"Ada Lovelace", "Alan Turing"}},
		{"strings", `{"authors": ["Grace Hopper"], "kernelspec": {"language": "R", "name": "ir"}}`, "", "r", []string{"Grace Hopper"}},
		{"kernel name only", `{"kernelspec": {"name": "julia-1.10"}}`, "", "julia", nil},
		{"author string", `{"author": "Jane Roe", "title": ["Multi", " line"]}`, "Multi line", "", []string{"Jane Roe"}},
		{"malformed language info", `{"title": "T", "language_info": "python"}`, "T", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, nb(tc.meta, code("x", 1)))
			if doc.Meta.Title != tc.title {
				t.Errorf("title = %q, want %q", doc.Meta.Title, tc.title)
			}
			var names []string
			for _, a := range doc.Meta.Authors {
				names = append(names, a.Name)
			}
			if strings.Join(names, ";") != strings.Join(tc.authors, ";") {
				t.Errorf("authors = %q, want %q", names, tc.authors)
			}
			if cb, ok := doc.Blocks[0].(*ast.CodeBlock); !ok || cb.Lang != tc.lang {
				t.Errorf("code language = %+v, want %q", doc.Blocks[0], tc.lang)
			}
		})
	}
}

func TestCodeCells(t *testing.T) {
	doc, warns := read(t, nb("",
		code("import numpy as np\n\n\n", 3,
			`{"output_type": "stream", "name": "stdout", "text": ["line 1\n", "line 2\n"]}`,
			`{"output_type": "stream", "name": "stdout", "text": "line 3\n"}`,
			`{"output_type": "stream", "name": "stderr", "text": "\u001b[33mwarning\u001b[0m: careful\n"}`,
			`{"output_type": "execute_result", "execution_count": 3, "data": {"text/plain": ["array([1, 2])"]}, "metadata": {}}`),
		code("", 4), // empty cells disappear
		code("for i in progress(range(3)): pass", 5,
			`{"output_type": "stream", "name": "stderr", "text": "  0%|          | 0/3\r 33%|###       | 1/3\r"}`,
			`{"output_type": "stream", "name": "stderr", "text": "100%|##########| 3/3\n"}`),
	))
	checkDump(t, doc.Blocks, strings.Join([]string{
		`Code(python.input execution_count=3) "import numpy as np"`,
		`Code(.output) "line 1\nline 2\nline 3"`,
		`Code(.output.stderr) "warning: careful"`,
		`Code(.output execution_count=3) "array([1, 2])"`,
		`Code(python.input execution_count=5) "for i in progress(range(3)): pass"`,
		`Code(.output.stderr) "100%|##########| 3/3"`,
	}, "\n"))
	if len(warns) != 0 {
		t.Errorf("warnings = %q", warns)
	}
}

func TestRichOutputs(t *testing.T) {
	tests := []struct {
		name, data, meta, want string
	}{
		{"png with width", `{"image/png": "` + pngB64 + `\n", "text/plain": "<Figure size 640x480 with 1 Axes>"}`, `{"image/png": {"width": 400}}`,
			"Fig ![](res:notebook/output.png 400px×)"},
		{"svg text", `{"image/svg+xml": ["<svg xmlns=\"http://www.w3.org/2000/svg\">", "</svg>"]}`, `{}`,
			"Fig ![](res:notebook/output.svg)"},
		{"symbolic latex", `{"text/latex": "$\\displaystyle x^{2} + 1$", "text/plain": "x**2 + 1"}`, `{}`, "Math x^{2} + 1"},
		{"display latex", `{"text/latex": "$$\\int_0^1 f$$"}`, `{}`, "Math \\int_0^1 f"},
		{"bracket latex", `{"text/latex": "\\[ a = b \\]"}`, `{}`, "Math a = b"},
		{"equation env", `{"text/latex": "\\begin{equation*}E=mc^2\\end{equation*}"}`, `{}`, "Math E=mc^2"},
		{"align env", `{"text/latex": "\\begin{align*}a&=1\\\\b&=2\\end{align*}"}`, `{}`, "Math \\begin{aligned}a&=1\\\\b&=2\\end{aligned}"},
		{"latex table is raw", `{"text/latex": "\\begin{tabular}{ll}a & b\\end{tabular}"}`, `{}`, `Raw(latex) "\\begin{tabular}{ll}a & b\\end{tabular}"`},
		{"two formulas are raw", `{"text/latex": "$a$ and $b$"}`, `{}`, `Raw(latex) "$a$ and $b$"`},
		{"json", `{"application/json": {"a": [1, 2]}}`, `{}`, "Code(json.output) \"{\\n  \\\"a\\\": [\\n    1,\\n    2\\n  ]\\n}\""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := `{"output_type": "display_data", "data": ` + tc.data + `, "metadata": ` + tc.meta + `}`
			doc, _ := read(t, nb("", code("x", 1, out)))
			checkDump(t, doc.Blocks[1:], tc.want)
		})
	}
}

func TestHTMLFallbackWithoutConverter(t *testing.T) {
	old := HTMLFragment
	defer func() { HTMLFragment = old }()
	HTMLFragment = nil
	out := `{"output_type": "display_data", "data": {"text/html": "<table><tr><td>1</td></tr></table>", "text/plain": "   a\n0  1"}, "metadata": {}}`
	doc, _ := read(t, nb("", code("df", 1, out)))
	checkDump(t, doc.Blocks[1:], `Code(.output) "   a\n0  1"`)
}

func TestDataFrameHTML(t *testing.T) {
	frame := `<div><style scoped>.dataframe tbody tr th {vertical-align: top;}</style><table border="1" class="dataframe"><thead><tr style="text-align: right;"><th></th><th>name</th><th>value</th></tr></thead>` +
		`<tbody><tr><th>0</th><td>alpha</td><td>1.5</td></tr><tr><th>1</th><td>beta</td><td>2.25</td></tr></tbody></table></div>`
	out := `{"output_type": "execute_result", "execution_count": 4, "data": {"text/html": ` + jsonStr(frame) + `, "text/plain": "    name  value"}, "metadata": {}}`
	doc, _ := read(t, nb("", code("df", 4, out)))
	if len(doc.Blocks) != 2 {
		t.Fatalf("blocks = %s", dump(doc.Blocks))
	}
	tbl, ok := doc.Blocks[1].(*ast.Table)
	if !ok || len(tbl.Body) != 2 || strings.Contains(dump(doc.Blocks), "vertical-align") {
		t.Fatalf("data frame not converted to a table: %s", dump(doc.Blocks))
	}
}

func TestImageResource(t *testing.T) {
	out := `{"output_type": "display_data", "data": {"image/png": "` + pngB64 + `"}, "metadata": {}}`
	doc, _ := read(t, nb("", code("a", 1, out), code("b", 2, out)))
	if doc.Resources.Len() != 1 {
		t.Errorf("identical images should share a resource: %v", doc.Resources.Names())
	}
	r, ok := doc.Resources.Get("res:notebook/output.png")
	if !ok || r.MediaType != "image/png" || !bytes.Equal(r.Data, pngBytes) {
		t.Fatalf("resource = %+v", r)
	}
}

func TestHTMLOutputUsesConverter(t *testing.T) {
	old := HTMLFragment
	defer func() { HTMLFragment = old }()
	var got string
	HTMLFragment = func(src string, res *ast.Resources) ([]ast.Block, []string) {
		got = src
		if strings.Contains(src, "<script") {
			return nil, nil
		}
		return []ast.Block{&ast.Table{Body: []ast.Row{{Cells: []ast.Cell{{Blocks: []ast.Block{&ast.Plain{Inlines: ast.Str("1")}}}}}}}}, []string{"html note"}
	}
	frame := `<div><style scoped>.dataframe tbody tr th {vertical-align: top;}</style><table border="1" class="dataframe"><thead><tr><th></th><th>a</th></tr></thead><tbody><tr><th>0</th><td>1</td></tr></tbody></table></div>`
	doc, warns := read(t, nb("", code("df", 2,
		`{"output_type": "execute_result", "execution_count": 2, "data": {"text/html": `+jsonStr(frame)+`, "text/plain": "   a\n0  1"}, "metadata": {}}`,
		`{"output_type": "display_data", "data": {"text/html": "<script>plot()</script>", "text/plain": "<Plot>"}, "metadata": {}}`)))
	checkDump(t, doc.Blocks[1:], "Table[]{[1]}\nCode(.output) \"<Plot>\"")
	if !strings.Contains(got, "<script>") || len(warns) != 1 || warns[0] != "html note" {
		t.Errorf("converter input %q, warnings %q", got, warns)
	}
}

func TestErrorOutput(t *testing.T) {
	tb := `["\u001b[0;31m---------------------------------------------------------------------------\u001b[0m", "\u001b[0;31mZeroDivisionError\u001b[0m                         Traceback (most recent call last)", "Cell \u001b[0;32mIn[1], line 1\u001b[0m\n\u001b[0;32m----> 1\u001b[0m \u001b[38;5;241;43m1\u001b[39;49m\u001b[38;5;241;43m/\u001b[39;49m\u001b[38;5;241;43m0\u001b[39;49m\n", "\u001b[0;31mZeroDivisionError\u001b[0m: division by zero"]`
	doc, _ := read(t, nb("", code("1/0", 1, `{"output_type": "error", "ename": "ZeroDivisionError", "evalue": "division by zero", "traceback": `+tb+`}`)))
	cb := doc.Blocks[1].(*ast.CodeBlock)
	if !cb.Attr.HasClass("error") || !cb.Attr.HasClass("output") {
		t.Errorf("classes = %v", cb.Attr.Classes)
	}
	if strings.Contains(cb.Text, "\x1b") || strings.Count(cb.Text, "ZeroDivisionError: division by zero") != 1 || !strings.Contains(cb.Text, "----> 1 1/0") {
		t.Errorf("traceback = %q", cb.Text)
	}
	doc, _ = read(t, nb("", code("raise X", 1, `{"output_type": "error", "ename": "X", "evalue": "boom", "traceback": []}`)))
	checkDump(t, doc.Blocks[1:], `Code(.output.error) "X: boom"`)
}

func TestHiddenCells(t *testing.T) {
	out := `{"output_type": "stream", "name": "stdout", "text": "out"}`
	cell := func(meta string) string {
		return `{"cell_type": "code", "execution_count": null, "metadata": ` + meta + `, "source": "src", "outputs": [` + out + `]}`
	}
	tests := []struct{ meta, want string }{
		{`{"tags": ["remove-input"]}`, `Code(.output) "out"`},
		{`{"tags": ["hide-output"]}`, `Code(python.input) "src"`},
		{`{"tags": ["remove-cell"]}`, ``},
		{`{"jupyter": {"source_hidden": true}}`, `Code(.output) "out"`},
		{`{"jupyter": {"outputs_hidden": true}}`, `Code(python.input) "src"`},
		{`{"hide_input": true}`, `Code(.output) "out"`},
		{`{"tags": "not-a-list"}`, "Code(python.input) \"src\"\nCode(.output) \"out\""},
	}
	for _, tc := range tests {
		t.Run(tc.meta, func(t *testing.T) {
			doc, _ := read(t, nb("", cell(tc.meta)))
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestMarkdownCells(t *testing.T) {
	md := `{"cell_type": "markdown", "metadata": {}, "source": ["# Results\n", "\n", "See ![chart](attachment:my%20chart.png) and <img src=\"attachment:my chart.png\">."],
	  "attachments": {"my chart.png": {"image/png": "` + pngB64 + `"}}}`
	doc, warns := read(t, nb("", md, `{"cell_type": "markdown", "metadata": {"tags": ["remove-cell"]}, "source": "hidden"}`))
	if len(doc.Blocks) < 2 {
		t.Fatalf("blocks = %s", dump(doc.Blocks))
	}
	if h, ok := doc.Blocks[0].(*ast.Heading); !ok || ast.PlainText(h.Inlines) != "Results" {
		t.Errorf("first block = %s", dump(doc.Blocks[:1]))
	}
	if strings.Contains(dump(doc.Blocks), "attachment:") || strings.Contains(dump(doc.Blocks), "hidden") {
		t.Errorf("attachment references not rewritten:\n%s", dump(doc.Blocks))
	}
	if _, ok := doc.Resources.Get("res:notebook/attachments/my_chart.png"); !ok {
		t.Errorf("attachment resource missing: %v", doc.Resources.Names())
	}
	found := false
	ast.WalkInlines(doc.Blocks, func(in ast.Inline) {
		if img, ok := in.(*ast.Image); ok && img.Src == "res:notebook/attachments/my_chart.png" {
			found = true
		}
	})
	if !found {
		t.Errorf("no image points at the attachment:\n%s", dump(doc.Blocks))
	}
	_ = warns
}

func TestMarkdownOutput(t *testing.T) {
	doc, _ := read(t, nb("", code("Markdown('**hi**')", 1, `{"output_type": "display_data", "data": {"text/markdown": "**hi**", "text/plain": "<Markdown>"}, "metadata": {}}`)))
	checkDump(t, doc.Blocks[1:], "P **hi**")
}

func TestRawCells(t *testing.T) {
	raw := func(meta, src string) string {
		return `{"cell_type": "raw", "metadata": ` + meta + `, "source": ` + jsonStr(src) + `}`
	}
	doc, _ := read(t, nb("",
		raw(`{"raw_mimetype": "text/latex"}`, `\newpage`),
		raw(`{"format": "text/markdown"}`, "*em*"),
		raw(`{}`, "\n  plain raw\n\n"),
		raw(`{"raw_mimetype": "text/restructuredtext"}`, ".. note:: x"),
	))
	checkDump(t, doc.Blocks, strings.Join([]string{
		`Raw(latex) "\\newpage"`,
		"P *em*",
		`Code() "  plain raw"`,
		`Code() ".. note:: x"`,
	}, "\n"))
}

func TestWidgetOutputDropped(t *testing.T) {
	doc, warns := read(t, nb("", code("slider", 1, `{"output_type": "display_data", "data": {"application/vnd.jupyter.widget-view+json": {"model_id": "x"}, "text/plain": "IntSlider(value=0)"}, "metadata": {}}`)))
	if len(doc.Blocks) != 1 || len(warns) != 1 {
		t.Errorf("blocks = %s, warnings = %q", dump(doc.Blocks), warns)
	}
}

func TestNBFormat3(t *testing.T) {
	src := `{"nbformat": 3, "nbformat_minor": 0, "metadata": {"name": "old"}, "worksheets": [{"cells": [
	  {"cell_type": "heading", "level": 2, "metadata": {}, "source": ["Old style"]},
	  {"cell_type": "code", "language": "python", "input": ["print(1)"], "prompt_number": 7, "metadata": {}, "outputs": [
	    {"output_type": "stream", "stream": "stdout", "text": ["1\n"]},
	    {"output_type": "pyout", "prompt_number": 7, "metadata": {}, "png": "` + pngB64 + `", "text": ["<Figure>"]},
	    {"output_type": "pyerr", "ename": "E", "evalue": "v", "traceback": ["E: v"]}
	  ]}
	]}]}`
	doc, _ := read(t, src)
	checkDump(t, doc.Blocks, strings.Join([]string{
		"H2 Old style",
		`Code(.input execution_count=7) "print(1)"`,
		`Code(.output) "1"`,
		"Fig ![](res:notebook/output.png)",
		`Code(.output.error) "E: v"`,
	}, "\n"))
}

func TestMalformedNotebooks(t *testing.T) {
	if _, _, err := Read(context.Background(), []byte("{not json"), rd.Options{}); err == nil {
		t.Error("expected error for invalid JSON")
	}
	if _, _, err := Read(context.Background(), []byte(`[1,2]`), rd.Options{}); err == nil {
		t.Error("expected error for a non-object")
	}
	if _, _, err := Read(context.Background(), []byte(`{}`), rd.Options{}); err == nil {
		t.Error("expected error for an empty object")
	}
	doc, warns := read(t, `{"nbformat": 4, "metadata": {}, "cells": [
	  42,
	  {"cell_type": "code", "source": 5, "outputs": "nope"},
	  {"cell_type": "code", "source": "ok", "outputs": [7, {"output_type": "stream", "name": "stdout", "text": "fine"}]},
	  {"cell_type": "markdown", "source": "![x](attachment:a.png)", "attachments": {"a.png": {"image/png": "!!!notbase64"}}},
	  {"cell_type": "code", "source": "img", "outputs": [{"output_type": "display_data", "data": {"image/png": "!!!", "text/plain": "fallback"}}]},
	  {"cell_type": "unknown", "source": "?"}
	]}`)
	checkDump(t, doc.Blocks[:2], "Code(.input) \"ok\"\nCode(.output) \"fine\"")
	if !strings.Contains(dump(doc.Blocks[2:3]), "](attachment:a.png)") {
		t.Errorf("undecodable attachment should stay unresolved: %s", dump(doc.Blocks[2:3]))
	}
	if got := dump(doc.Blocks[3:]); got != "Code(.input) \"img\"\nCode(.output) \"fallback\"" {
		t.Errorf("tail = %s", got)
	}
	if len(warns) < 3 {
		t.Errorf("expected warnings for malformed cells, got %q", warns)
	}
}

func TestContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := Read(ctx, []byte(nb("", code("x", 1))), rd.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestCleanTerminal(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain\n", "plain"},
		{"\x1b[1;31mred\x1b[0m text", "red text"},
		{"\x1b]8;;http://x\x07link\x1b]8;;\x07", "link"},
		{"\x1b(Bcharset", "charset"},
		{"10%\r50%\r100%", "100%"},
		{"abcdef\rXY", "XYcdef"},
		{"progress 100%\r", "progress 100%"},
		{"a\r\nb", "a\nb"},
		{"ab\bc", "ac"},
		{"line1\nloading\rdone\nline3\n\n", "line1\ndoneing\nline3"}, // as a terminal shows it
		{"\x1b[", ""},
		{"\u009b31mcsi", "csi"},
	}
	for _, tc := range tests {
		if got := cleanTerminal(tc.in); got != tc.want {
			t.Errorf("cleanTerminal(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func FuzzRead(f *testing.F) {
	f.Add([]byte(nb("", code("x", 1, `{"output_type": "stream", "name": "stdout", "text": "\u001b[31mhi\r"}`))))
	f.Add([]byte(`{"nbformat": 3, "worksheets": [{"cells": [{"cell_type": "heading", "level": 9, "source": "x"}]}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, _, err := Read(context.Background(), data, rd.Options{})
		if err == nil && (doc == nil || doc.Resources == nil) {
			t.Fatal("nil document without error")
		}
	})
}
