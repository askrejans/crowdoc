package mathml

import (
	"encoding/xml"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const ns = `xmlns="http://www.w3.org/1998/Math/MathML"`

func TestConvertXML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"identifiers and numbers", `<mrow><mi>x</mi><mo>+</mo><mn>2</mn></mrow>`, `x+2`},
		{"greek and relation", `<mrow><mi>α</mi><mo>≤</mo><mi>β</mi></mrow>`, `\alpha\leq\beta`},
		{"command followed by letter", `<mrow><mi>α</mi><mi>x</mi></mrow>`, `\alpha x`},
		{"function name", `<mrow><mi>sin</mi><mo>&#x2061;</mo><mi>x</mi></mrow>`, `\sin x`},
		{"multi-letter identifier", `<mi>rate</mi>`, `\mathrm{rate}`},
		{"unknown operator word", `<mo>tr</mo>`, `\operatorname{tr}`},
		{"bold variant", `<mi mathvariant="bold">v</mi>`, `\mathbf{v}`},
		{"bold greek", `<mi mathvariant="bold">μ</mi>`, `\boldsymbol{\mu}`},
		{"double-struck", `<mi mathvariant="double-struck">R</mi>`, `\mathbb{R}`},
		{"script", `<mi mathvariant="script">L</mi>`, `\mathcal{L}`},
		{"fraktur", `<mi mathvariant="fraktur">g</mi>`, `\mathfrak{g}`},
		{"sans-serif", `<mi mathvariant="sans-serif">A</mi>`, `\mathsf{A}`},
		{"monospace", `<mi mathvariant="monospace">x</mi>`, `\mathtt{x}`},
		{"normal", `<mi mathvariant="normal">d</mi>`, `\mathrm{d}`},
		{"bold-italic", `<mi mathvariant="bold-italic">x</mi>`, `\boldsymbol{x}`},
		{"math alphanumeric", `<mi>𝐱</mi>`, `\mathbf{x}`},
		{"blackboard letter", `<mi>ℝ</mi>`, `\mathbb{R}`},
		{"tex specials escaped", `<mi>#</mi><mo>%</mo><mi>&amp;</mi><mi>_</mi><mo>\</mo>`, `\#\%\&\_\backslash`},
		{"text", `<mtext>if x &gt; 0 &amp; 50%</mtext>`, `\text{if x > 0 \& 50\%}`},
		{"string literal", `<ms>abc</ms>`, `\text{"abc"}`},
		{"superscript", `<msup><mi>x</mi><mn>2</mn></msup>`, `x^{2}`},
		{"subscript braces base", `<msub><mrow><mi>a</mi><mo>+</mo><mi>b</mi></mrow><mi>i</mi></msub>`, `{a+b}_{i}`},
		{"subsup", `<msubsup><mi>x</mi><mi>i</mi><mn>2</mn></msubsup>`, `x_{i}^{2}`},
		{"prime", `<msup><mi>f</mi><mo>′</mo></msup>`, `f'`},
		{"fraction", `<mfrac><mn>1</mn><mi>n</mi></mfrac>`, `\frac{1}{n}`},
		{"no-line fraction", `<mfrac linethickness="0"><mi>a</mi><mi>b</mi></mfrac>`, `\genfrac{}{}{0pt}{}{a}{b}`},
		{"bevelled fraction", `<mfrac bevelled="true"><mn>1</mn><mn>2</mn></mfrac>`, `1/2`},
		{"sqrt", `<msqrt><mi>x</mi><mo>+</mo><mn>1</mn></msqrt>`, `\sqrt{x+1}`},
		{"root", `<mroot><mi>x</mi><mn>3</mn></mroot>`, `\sqrt[3]{x}`},
		{"sum with limits", `<munderover><mo>∑</mo><mrow><mi>i</mi><mo>=</mo><mn>1</mn></mrow><mi>n</mi></munderover><msub><mi>a</mi><mi>i</mi></msub>`, `\sum_{i=1}^{n}a_{i}`},
		{"integral", `<msubsup><mo>∫</mo><mn>0</mn><mi>∞</mi></msubsup><mi>f</mi>`, `\int_{0}^{\infty}f`},
		{"limit", `<munder><mi>lim</mi><mrow><mi>x</mi><mo>→</mo><mn>0</mn></mrow></munder>`, `\lim_{x\to0}`},
		{"hat accent", `<mover accent="true"><mi>x</mi><mo>^</mo></mover>`, `\hat{x}`},
		{"wide hat", `<mover><mrow><mi>x</mi><mi>y</mi></mrow><mo>ˆ</mo></mover>`, `\widehat{xy}`},
		{"tilde", `<mover><mi>a</mi><mo>~</mo></mover>`, `\tilde{a}`},
		{"bar and overline", `<mover><mi>x</mi><mo>¯</mo></mover><mover><mrow><mi>a</mi><mi>b</mi></mrow><mo>‾</mo></mover>`, `\bar{x}\overline{ab}`},
		{"vector", `<mover><mi>v</mi><mo>→</mo></mover>`, `\vec{v}`},
		{"long vector", `<mover><mrow><mi>A</mi><mi>B</mi></mrow><mo>→</mo></mover>`, `\overrightarrow{AB}`},
		{"dots", `<mover><mi>x</mi><mo>˙</mo></mover><mover><mi>y</mi><mo>¨</mo></mover>`, `\dot{x}\ddot{y}`},
		{"overbrace with label", `<mover><mover><mrow><mi>a</mi><mo>+</mo><mi>b</mi></mrow><mo>⏞</mo></mover><mtext>n</mtext></mover>`, `\overbrace{a+b}^{\text{n}}`},
		{"underbrace with label", `<munder><munder><mi>x</mi><mo>⏟</mo></munder><mn>3</mn></munder>`, `\underbrace{x}_{3}`},
		{"underline", `<munder><mi>x</mi><mo>_</mo></munder>`, `\underline{x}`},
		{"overset", `<mover><mo>=</mo><mtext>def</mtext></mover>`, `\overset{\text{def}}{=}`},
		{"arrow with text", `<mover><mo>→</mo><mi>f</mi></mover>`, `\xrightarrow{f}`},
		{"underset", `<munder><mi>x</mi><mi>y</mi></munder>`, `\underset{y}{x}`},
		{"plain parens stay plain", `<mrow><mo>(</mo><mi>x</mi><mo>)</mo></mrow>`, `(x)`},
		{"stretchy parens", `<mrow><mo>(</mo><mfrac><mn>1</mn><mn>2</mn></mfrac><mo>)</mo></mrow>`, `\left(\frac{1}{2}\right)`},
		{"explicit non-stretchy", `<mrow><mo stretchy="false">[</mo><mfrac><mn>1</mn><mn>2</mn></mfrac><mo stretchy="false">]</mo></mrow>`, `[\frac{1}{2}]`},
		{"unbalanced pair not wrapped", `<mrow><mo>(</mo><mfrac><mi>a</mi><mi>b</mi></mfrac><mo>)</mo><mo>(</mo><mi>c</mi><mo>)</mo></mrow>`, `(\frac{a}{b})(c)`},
		{"binomial", `<mrow><mo>(</mo><mfrac linethickness="0"><mi>n</mi><mi>k</mi></mfrac><mo>)</mo></mrow>`, `\binom{n}{k}`},
		{"pmatrix", `<mrow><mo>(</mo><mtable><mtr><mtd><mn>1</mn></mtd><mtd><mn>0</mn></mtd></mtr><mtr><mtd><mn>0</mn></mtd><mtd><mn>1</mn></mtd></mtr></mtable><mo>)</mo></mrow>`, `\begin{pmatrix} 1 & 0 \\ 0 & 1 \end{pmatrix}`},
		{"bmatrix", `<mrow><mo>[</mo><mtable><mtr><mtd><mi>a</mi></mtd></mtr></mtable><mo>]</mo></mrow>`, `\begin{bmatrix} a \end{bmatrix}`},
		{"vmatrix", `<mrow><mo>|</mo><mtable><mtr><mtd><mi>a</mi></mtd><mtd><mi>b</mi></mtd></mtr></mtable><mo>|</mo></mrow>`, `\begin{vmatrix} a & b \end{vmatrix}`},
		{"bare matrix", `<mtable><mtr><mtd><mi>a</mi></mtd><mtd><mi>b</mi></mtd></mtr></mtable>`, `\begin{matrix} a & b \end{matrix}`},
		{"cases", `<mrow><mo>{</mo><mtable><mtr><mtd><mn>1</mn></mtd><mtd><mi>x</mi><mo>&gt;</mo><mn>0</mn></mtd></mtr><mtr><mtd><mn>0</mn></mtd><mtd><mtext>otherwise</mtext></mtd></mtr></mtable></mrow>`, `\begin{cases} 1 & x>0 \\ 0 & \text{otherwise} \end{cases}`},
		{"aligned by columnalign", `<mtable columnalign="right left"><mtr><mtd><mi>x</mi></mtd><mtd><mo>=</mo><mn>1</mn></mtd></mtr><mtr><mtd><mi>y</mi></mtd><mtd><mo>=</mo><mn>2</mn></mtd></mtr></mtable>`, `\begin{aligned} x & =1 \\ y & =2 \end{aligned}`},
		{"aligned by relation column", `<mtable><mtr><mtd><mi>x</mi></mtd><mtd><mo>=</mo><mn>1</mn></mtd></mtr></mtable>`, `\begin{aligned} x & =1 \end{aligned}`},
		{"labeled row drops label", `<mtable columnalign="right left"><mlabeledtr><mtd><mtext>(1)</mtext></mtd><mtd><mi>a</mi></mtd><mtd><mo>=</mo><mi>b</mi></mtd></mlabeledtr></mtable>`, `\begin{aligned} a & =b \end{aligned}`},
		{"array alignment", `<mtable columnalign="left center"><mtr><mtd><mi>a</mi></mtd><mtd><mi>b</mi></mtd><mtd><mi>c</mi></mtd></mtr></mtable>`, `\begin{array}{lcc} a & b & c \end{array}`},
		{"single column left", `<mtable columnalign="left"><mtr><mtd><mi>a</mi></mtd></mtr><mtr><mtd><mi>b</mi></mtd></mtr></mtable>`, `\begin{aligned} &a \\ &b \end{aligned}`},
		{"single column centred", `<mtable><mtr><mtd><mi>a</mi></mtd></mtr></mtable>`, `\begin{gathered} a \end{gathered}`},
		{"mfenced default", `<mfenced><mi>a</mi><mi>b</mi></mfenced>`, `(a,b)`},
		{"mfenced custom", `<mfenced open="[" close=")" separators=";"><mi>a</mi><mi>b</mi><mi>c</mi></mfenced>`, `[a;b;c)`},
		{"mfenced braces escaped", `<mfenced open="{" close="}"><mi>x</mi></mfenced>`, `\{x\}`},
		{"mfenced stretchy", `<mfenced><mfrac><mi>a</mi><mi>b</mi></mfrac></mfenced>`, `\left(\frac{a}{b}\right)`},
		{"mfenced matrix", `<mfenced open="[" close="]"><mtable><mtr><mtd><mn>1</mn></mtd></mtr></mtable></mfenced>`, `\begin{bmatrix} 1 \end{bmatrix}`},
		{"boxed", `<menclose notation="box"><mi>E</mi></menclose>`, `\boxed{E}`},
		{"strike falls back to content", `<menclose notation="updiagonalstrike"><mi>x</mi></menclose>`, `x`},
		{"menclose top", `<menclose notation="top"><mi>x</mi></menclose>`, `\overline{x}`},
		{"phantom", `<mphantom><mi>x</mi></mphantom>`, `\phantom{x}`},
		{"mstyle displaystyle", `<mstyle displaystyle="true"><mfrac><mn>1</mn><mn>2</mn></mfrac></mstyle>`, `{\displaystyle \frac{1}{2}}`},
		{"mstyle variant inherits", `<mstyle mathvariant="bold"><mi>x</mi><mn>1</mn></mstyle>`, `\mathbf{x}\mathbf{1}`},
		{"padded error action", `<mpadded><merror><maction actiontype="toggle" selection="2"><mi>a</mi><mi>b</mi></maction></merror></mpadded>`, `b`},
		{"multiscripts", `<mmultiscripts><mi>X</mi><mi>a</mi><mi>b</mi><mprescripts/><mi>c</mi><mi>d</mi></mmultiscripts>`, `{}_{c}^{d}X_{a}^{b}`},
		{"multiscripts none", `<mmultiscripts><mi>R</mi><mi>i</mi><none/><none/><mi>j</mi></mmultiscripts>`, `R_{i}{}^{j}`},
		{"spaces", `<mi>a</mi><mspace width="1em"/><mi>b</mi><mspace width="0.167em"/><mi>c</mi><mspace width="thickmathspace"/><mi>d</mi>`, `a\quad b\,c\;d`},
		{"invisible operators", `<mi>a</mi><mo>&#x2062;</mo><mi>b</mi>`, `ab`},
		{"set membership", `<mi>x</mi><mo>∈</mo><mi>A</mi><mo>∪</mo><mi>B</mi><mo>⊂</mo><mi>C</mi>`, `x\in A\cup B\subset C`},
		{"arrows and logic", `<mi>p</mi><mo>⇒</mo><mi>q</mi><mo>⇔</mo><mo>¬</mo><mi>r</mi>`, `p\Rightarrow q\Leftrightarrow\neg r`},
		{"operators", `<mi>a</mi><mo>×</mo><mi>b</mi><mo>·</mo><mi>c</mi><mo>±</mo><mi>d</mi><mo>∓</mo><mi>e</mi><mo>≠</mo><mi>f</mi>`, `a\times b\cdot c\pm d\mp e\neq f`},
		{"calculus symbols", `<mo>∂</mo><mi>f</mi><mo>∇</mo><mi>g</mi>`, `\partial f\nabla g`},
		{"unicode minus", `<mi>a</mi><mo>−</mo><mi>b</mi>`, `a-b`},
		{"glyph alt", `<mglyph alt="z"/>`, `z`},
		{"semantics ignores non-tex annotation", `<semantics><mi>x</mi><annotation encoding="text/plain">x</annotation></semantics>`, `x`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, display, err := ConvertXML([]byte(`<math ` + ns + `>` + tt.in + `</math>`))
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if display {
				t.Errorf("display = true, want false")
			}
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestConvertXMLDisplayAndAnnotation(t *testing.T) {
	in := `<?xml version="1.0" encoding="UTF-8"?>
<math:math xmlns:math="http://www.w3.org/1998/Math/MathML" display="block">
 <math:semantics>
  <math:mrow><math:mi>E</math:mi><math:mo>=</math:mo><math:mi>m</math:mi><math:msup><math:mi>c</math:mi><math:mn>2</math:mn></math:msup></math:mrow>
  <math:annotation encoding="application/x-tex">  E = mc^2 </math:annotation>
 </math:semantics>
</math:math>`
	got, display, err := ConvertXML([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if !display {
		t.Error("display = false, want true")
	}
	if got != "E = mc^2" {
		t.Errorf("got %q", got)
	}
}

func TestTeXEncodings(t *testing.T) {
	for enc, want := range map[string]bool{
		"application/x-tex": true, "TeX": true, "LaTeX": true, "text/x-latex": true,
		"application/x-latex; charset=utf-8": true, "text/plain": false, "text/html": false,
		"formula-markup 5.0": false, "": false, "MathML-Content": false,
	} {
		if got := isTeXEncoding(enc); got != want {
			t.Errorf("isTeXEncoding(%q) = %v", enc, got)
		}
	}
}

func TestUnsafeAnnotationFallsBack(t *testing.T) {
	in := `<math ` + ns + `><semantics><mi>x</mi><annotation encoding="TeX">\input{/etc/passwd}</annotation></semantics></math>`
	got, _, err := ConvertXML([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if got != "x" {
		t.Errorf("got %q, want presentation fallback", got)
	}
}

func TestConvertXMLErrors(t *testing.T) {
	if _, _, err := ConvertXML([]byte("   ")); err == nil {
		t.Error("expected error for empty input")
	}
	// Truncated input must not panic and should keep what was parsed.
	got, _, _ := ConvertXML([]byte(`<math><mi>x</mi><mo>+`))
	if !strings.HasPrefix(got, "x") {
		t.Errorf("truncated input: got %q", got)
	}
}

func TestDeepNestingIsBounded(t *testing.T) {
	in := strings.Repeat("<mrow>", 5000) + "<mi>x</mi>" + strings.Repeat("</mrow>", 5000)
	if _, _, err := ConvertXML([]byte(`<math>` + in + `</math>`)); err != nil {
		t.Fatal(err)
	}
}

func TestConvertElementStreaming(t *testing.T) {
	src := `<p xmlns:math="http://www.w3.org/1998/Math/MathML">before <math:math><math:mi>y</math:mi></math:math> after</p>`
	d := xml.NewDecoder(strings.NewReader(src))
	var got string
	var tail strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "math" {
				got, _, _ = ConvertElement(d, t)
			}
		case xml.CharData:
			tail.Write(t)
		}
	}
	if got != "y" {
		t.Errorf("got %q", got)
	}
	if tail.String() != "before  after" {
		t.Errorf("decoder not positioned after </math>: %q", tail.String())
	}
}

func parseHTMLMath(t *testing.T, src string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	var find func(*html.Node) *html.Node
	find = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "math" {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if m := find(c); m != nil {
				return m
			}
		}
		return nil
	}
	m := find(doc)
	if m == nil {
		t.Fatal("no math element")
	}
	return m
}

func TestConvertHTML(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"annotation inside wrapper spans", `<span class="katex"><span class="katex-mathml"><math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mrow><mi>x</mi></mrow><annotation encoding="application/x-tex">\frac{a}{b}</annotation></semantics></math></span></span>`, `\frac{a}{b}`},
		{"alttext with displaystyle annotation", `<math alttext="{\displaystyle a^{2}}"><semantics><mrow class="MJX-TeXAtom-ORD"><mstyle displaystyle="true"><msup><mi>a</mi><mn>2</mn></msup></mstyle></mrow><annotation encoding="application/x-tex">{\displaystyle a^{2}}</annotation></semantics></math>`, `{\displaystyle a^{2}}`},
		{"presentation only", `<p>Let <math><mfrac><mi>π</mi><mn>2</mn></mfrac></math>.</p>`, `\frac{\pi}{2}`},
		{"entities", `<math><mi>x</mi><mo>&le;</mo><mi>&infin;</mi></math>`, `x\leq\infty`},
		{"alttext fallback", `<math alttext="x^2"></math>`, `x^2`},
		{"prose alttext ignored", `<math alttext="the variable x"></math>`, ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Convert(parseHTMLMath(t, tt.in)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
	if Convert(nil) != "" {
		t.Error("nil node should convert to empty string")
	}
}

func TestSafeTeX(t *testing.T) {
	tests := []struct {
		in   string
		safe bool
	}{
		{`\frac{a}{b} + \sqrt{x}`, true},
		{`\begin{aligned} a &= b \\ c &= d \end{aligned}`, true},
		{`\{ x \}`, true},
		{`\input{secret}`, false},
		{`\immediate\write18{rm -rf}`, false},
		{`\def\x{1}`, false},
		{`\catcode`, false},
		{`^^5cinput`, false},
		{`\directlua{os.exit()}`, false},
		{`\begin{filecontents}{x}`, false},
		{`a}{b`, false},
		{`{a`, false},
		{`\pdfstrcmp{a}{b}`, false},
	}
	for _, tt := range tests {
		if got := SafeTeX(tt.in); got != tt.safe {
			t.Errorf("SafeTeX(%q) = %v, want %v", tt.in, got, tt.safe)
		}
	}
}
