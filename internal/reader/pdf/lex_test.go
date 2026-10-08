package pdf

import (
	"reflect"
	"strings"
	"testing"
)

func TestLexerTokens(t *testing.T) {
	tests := []struct {
		in   string
		want any
	}{
		{"42", 42},
		{"-17", -17},
		{"+4.5", 4.5},
		{".5", 0.5},
		{"-.25", -0.25},
		{"4.", 4.0},
		{"--5", -5},    // writer bug: doubled sign
		{"1.2.3", 1.2}, // garbage after a real
		{"5-", 5},      // trailing sign
		{"12abc", keyword("12abc")},
		{"99999999999", 99999999999.0}, // too large for an object number
		{"/Name", name("Name")},
		{"/A#20B", name("A B")},
		{"/#41#42", name("AB")},
		{"/Bad#G1", name("Bad#G1")},
		{"(plain)", pdfString("plain")},
		{`(a\(b\)c)`, pdfString("a(b)c")},
		{"(nested (paren) ok)", pdfString("nested (paren) ok")},
		{`(\101\102\60)`, pdfString("AB0")},
		{`(\n\r\t\b\f\\\q)`, pdfString("\n\r\t\b\f\\q")},
		{"(line\\\ncontinued)", pdfString("linecontinued")},
		{"(cr\r\nlf)", pdfString("cr\nlf")},
		{"(unterminated", pdfString("unterminated")},
		{"<48 65 6C 6c 6F>", pdfString("Hello")},
		{"<414>", pdfString("A@")}, // odd digit count: pad with 0
		{"<4x1>", pdfString("A")},  // junk inside a hex string is skipped
		{"true", true},
		{"false", false},
		{"null", nil},
		{"Tj", keyword("Tj")},
	}
	for _, tt := range tests {
		lx := newLexer([]byte(tt.in))
		got, ok := lx.object(0)
		if !ok {
			t.Errorf("%q: no token", tt.in)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: got %#v, want %#v", tt.in, got, tt.want)
		}
	}
}

func TestLexerObjects(t *testing.T) {
	lx := newLexer([]byte(`<< /Type /Page /Kids [1 0 R 2 0 R 3] /Rect [0 0 612.5 792] /Nested << /A (x) /B null >> % comment
	/Empty [] >>`))
	got, _ := lx.object(0)
	want := dict{
		"Type":   name("Page"),
		"Kids":   array{ref{1, 0}, ref{2, 0}, 3},
		"Rect":   array{0, 0, 612.5, 792},
		"Nested": dict{"A": pdfString("x")},
		"Empty":  array(nil),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}

	// A dictionary cut short by "endobj" stops there.
	lx = newLexer([]byte("<< /A 1 /B endobj"))
	got, _ = lx.object(0)
	if d, ok := got.(dict); !ok || d["A"] != 1 || len(d) != 1 {
		t.Errorf("truncated dict: %#v", got)
	}
}

func TestLexerDeepNesting(t *testing.T) {
	// Hostile nesting must neither overflow the stack nor loop.
	deep := strings.Repeat("[", 100000) + strings.Repeat("]", 100000)
	lx := newLexer([]byte(deep))
	if _, ok := lx.object(0); !ok {
		t.Fatal("no object")
	}
	deep = strings.Repeat("<< /A ", 50000)
	lx = newLexer([]byte(deep))
	lx.object(0)
}

func TestLexerContentStream(t *testing.T) {
	lx := newLexer([]byte("BT /F1 12 Tf 72 700 Td [(He) -20 (llo)] TJ ET"))
	var ops []keyword
	for {
		tok, ok := lx.next()
		if !ok {
			break
		}
		if k, isKw := tok.(keyword); isKw {
			ops = append(ops, k)
		}
	}
	if want := []keyword{"BT", "Tf", "Td", "TJ", "ET"}; !reflect.DeepEqual(ops, want) {
		t.Errorf("ops %v, want %v", ops, want)
	}
}
