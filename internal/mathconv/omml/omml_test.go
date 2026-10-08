package omml

import (
	"strings"
	"testing"
)

const nsDecl = `xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" ` +
	`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`

func oMath(body string) []byte {
	return []byte(`<m:oMath ` + nsDecl + `>` + body + `</m:oMath>`)
}

// r is a default (italic) math run.
func r(text string) string { return `<m:r><m:t xml:space="preserve">` + text + `</m:t></m:r>` }

// rs is a math run with an m:sty / m:scr style.
func rs(sty, scr, text string) string {
	pr := ""
	if scr != "" {
		pr += `<m:scr m:val="` + scr + `"/>`
	}
	if sty != "" {
		pr += `<m:sty m:val="` + sty + `"/>`
	}
	return `<m:r><m:rPr>` + pr + `</m:rPr><m:t>` + text + `</m:t></m:r>`
}

func e(body string) string { return `<m:e>` + body + `</m:e>` }

func TestConvert(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"plain", r("x+1"), `x+1`},
		{"fraction", `<m:f><m:num>` + r("a") + `</m:num><m:den>` + r("b") + `</m:den></m:f>`, `\frac{a}{b}`},
		{"linear fraction", `<m:f><m:fPr><m:type m:val="lin"/></m:fPr><m:num>` + r("a+b") + `</m:num><m:den>` + r("c") + `</m:den></m:f>`, `{a+b}/c`},
		{"skewed fraction", `<m:f><m:fPr><m:type m:val="skw"/></m:fPr><m:num>` + r("1") + `</m:num><m:den>` + r("2") + `</m:den></m:f>`, `{}^{1}\!/\!{}_{2}`},
		{"no-bar fraction", `<m:f><m:fPr><m:type m:val="noBar"/></m:fPr><m:num>` + r("n") + `</m:num><m:den>` + r("k") + `</m:den></m:f>`, `\genfrac{}{}{0pt}{}{n}{k}`},
		{"superscript", `<m:sSup>` + e(r("x")) + `<m:sup>` + r("2") + `</m:sup></m:sSup>`, `x^{2}`},
		{"subscript", `<m:sSub>` + e(r("a")) + `<m:sub>` + r("ij") + `</m:sub></m:sSub>`, `a_{ij}`},
		{"sub-superscript", `<m:sSubSup>` + e(r("x")) + `<m:sub>` + r("i") + `</m:sub><m:sup>` + r("2") + `</m:sup></m:sSubSup>`, `x_{i}^{2}`},
		{"nested script base is braced", `<m:sSup>` + e(`<m:sSub>`+e(r("x"))+`<m:sub>`+r("1")+`</m:sub></m:sSub>`) + `<m:sup>` + r("2") + `</m:sup></m:sSup>`, `{x_{1}}^{2}`},
		{"multi-token base is braced", `<m:sSup>` + e(r("ab")) + `<m:sup>` + r("n") + `</m:sup></m:sSup>`, `{ab}^{n}`},
		{"pre-script", `<m:sPre><m:sub>` + r("1") + `</m:sub><m:sup>` + r("2") + `</m:sup>` + e(r("X")) + `</m:sPre>`, `{}_{1}^{2}X`},
		{"nested exponent", `<m:sSup>` + e(r("e")) + `<m:sup><m:rad><m:radPr><m:degHide m:val="1"/></m:radPr><m:deg/>` + e(`<m:f><m:num>`+r("1")+`</m:num><m:den>`+r("2")+`</m:den></m:f>`) + `</m:rad></m:sup></m:sSup>`, `e^{\sqrt{\frac{1}{2}}}`},
		{"square root", `<m:rad><m:radPr><m:degHide m:val="on"/></m:radPr><m:deg/>` + e(r("x")) + `</m:rad>`, `\sqrt{x}`},
		{"cube root", `<m:rad><m:deg>` + r("3") + `</m:deg>` + e(r("x")) + `</m:rad>`, `\sqrt[3]{x}`},
		{"root with empty degree", `<m:rad><m:deg/>` + e(r("y")) + `</m:rad>`, `\sqrt{y}`},
		{"sum with limits", `<m:nary><m:naryPr><m:chr m:val="∑"/><m:limLoc m:val="undOvr"/></m:naryPr><m:sub>` + r("i=1") + `</m:sub><m:sup>` + r("n") + `</m:sup>` + e(`<m:sSub>`+e(r("x"))+`<m:sub>`+r("i")+`</m:sub></m:sSub>`) + `</m:nary>`, `\sum\limits_{i=1}^{n} x_{i}`},
		{"integral side limits", `<m:nary><m:naryPr><m:limLoc m:val="subSup"/></m:naryPr><m:sub>` + r("0") + `</m:sub><m:sup>` + r("1") + `</m:sup>` + e(r("f")+rs("p", "", "d")+r("x")) + `</m:nary>`, `\int_{0}^{1} f\mathrm{d}x`},
		{"sum side limits", `<m:nary><m:naryPr><m:chr m:val="∑"/><m:limLoc m:val="subSup"/></m:naryPr><m:sub>` + r("k") + `</m:sub><m:sup/>` + e(r("a")) + `</m:nary>`, `\sum\nolimits_{k} a`},
		{"hidden limits", `<m:nary><m:naryPr><m:chr m:val="∏"/><m:subHide m:val="1"/><m:supHide m:val="1"/></m:naryPr><m:sub>` + r("i") + `</m:sub><m:sup>` + r("n") + `</m:sup>` + e(r("p")) + `</m:nary>`, `\prod p`},
		{"contour and union", `<m:nary><m:naryPr><m:chr m:val="∮"/></m:naryPr><m:sub>` + r("C") + `</m:sub><m:sup/>` + e(r("F")) + `</m:nary><m:nary><m:naryPr><m:chr m:val="⋃"/><m:limLoc m:val="undOvr"/></m:naryPr><m:sub>` + r("i") + `</m:sub><m:sup/>` + e(r("A")) + `</m:nary>`, `\oint_{C} F\bigcup\limits_{i} A`},
		{"unknown n-ary operator", `<m:nary><m:naryPr><m:chr m:val="⊕"/></m:naryPr><m:sub/><m:sup/>` + e(r("x")) + `</m:nary>`, `\mathop{\oplus} x`},
		{"parentheses", `<m:d>` + e(r("x+1")) + `</m:d>`, `\left(x+1\right)`},
		{"brackets", `<m:d><m:dPr><m:begChr m:val="["/><m:endChr m:val="]"/></m:dPr>` + e(r("a")) + `</m:d>`, `\left[a\right]`},
		{"open brace only", `<m:d><m:dPr><m:begChr m:val="{"/><m:endChr m:val=""/></m:dPr>` + e(r("a")) + `</m:d>`, `\left\{a\right.`},
		{"angle brackets", `<m:d><m:dPr><m:begChr m:val="⟨"/><m:endChr m:val="⟩"/></m:dPr>` + e(r("u")) + `</m:d>`, `\left\langle u\right\rangle`},
		{"separator", `<m:d>` + e(r("a")) + e(r("b")) + `</m:d>`, `\left(a\mid b\right)`},
		{"comma separator", `<m:d><m:dPr><m:sepChr m:val=","/></m:dPr>` + e(r("a")) + e(r("b")) + `</m:d>`, `\left(a,b\right)`},
		{"no grow", `<m:d><m:dPr><m:grow m:val="0"/></m:dPr>` + e(r("a")) + `</m:d>`, `(a)`},
		{"pmatrix", `<m:d>` + e(matrix2()) + `</m:d>`, `\begin{pmatrix}1 & 0 \\ 0 & 1\end{pmatrix}`},
		{"bmatrix", `<m:d><m:dPr><m:begChr m:val="["/><m:endChr m:val="]"/></m:dPr>` + e(matrix2()) + `</m:d>`, `\begin{bmatrix}1 & 0 \\ 0 & 1\end{bmatrix}`},
		{"Bmatrix", `<m:d><m:dPr><m:begChr m:val="{"/><m:endChr m:val="}"/></m:dPr>` + e(matrix2()) + `</m:d>`, `\begin{Bmatrix}1 & 0 \\ 0 & 1\end{Bmatrix}`},
		{"vmatrix", `<m:d><m:dPr><m:begChr m:val="|"/><m:endChr m:val="|"/></m:dPr>` + e(matrix2()) + `</m:d>`, `\begin{vmatrix}1 & 0 \\ 0 & 1\end{vmatrix}`},
		{"Vmatrix", `<m:d><m:dPr><m:begChr m:val="‖"/><m:endChr m:val="‖"/></m:dPr>` + e(matrix2()) + `</m:d>`, `\begin{Vmatrix}1 & 0 \\ 0 & 1\end{Vmatrix}`},
		{"bare matrix", matrix2(), `\begin{matrix}1 & 0 \\ 0 & 1\end{matrix}`},
		{"matrix with other delimiters", `<m:d><m:dPr><m:begChr m:val="⌊"/><m:endChr m:val="⌋"/></m:dPr>` + e(matrix2()) + `</m:d>`, `\left\lfloor\begin{matrix}1 & 0 \\ 0 & 1\end{matrix}\right\rfloor`},
		{"function", `<m:func><m:fName>` + rs("p", "", "sin") + `</m:fName>` + e(r("x")) + `</m:func>`, `\sin x`},
		{"function squared", `<m:func><m:fName><m:sSup>` + e(rs("p", "", "cos")) + `<m:sup>` + r("2") + `</m:sup></m:sSup></m:fName>` + e(r("θ")) + `</m:func>`, `\cos^{2} \theta`},
		{"unknown function", `<m:func><m:fName>` + rs("p", "", "sgn") + `</m:fName>` + e(r("x")) + `</m:func>`, `\operatorname{sgn} x`},
		{"limit", `<m:func><m:fName><m:limLow>` + e(rs("p", "", "lim")) + `<m:lim>` + r("x→0") + `</m:lim></m:limLow></m:fName>` + e(r("f")) + `</m:func>`, `\lim_{x\to0} f`},
		{"operator name limit", `<m:limLow>` + e(rs("p", "", "argmax")) + `<m:lim>` + r("x") + `</m:lim></m:limLow>`, `\operatorname*{argmax}_{x}`},
		{"underset", `<m:limLow>` + e(r("A")) + `<m:lim>` + r("n") + `</m:lim></m:limLow>`, `\underset{n}{A}`},
		{"overset", `<m:limUpp>` + e(r("=")) + `<m:lim>` + rs("p", "", "def") + `</m:lim></m:limUpp>`, `\overset{\mathrm{def}}{=}`},
		{"hat", `<m:acc>` + e(r("x")) + `</m:acc>`, `\hat{x}`},
		{"wide hat", `<m:acc><m:accPr><m:chr m:val="̂"/></m:accPr>` + e(r("xy")) + `</m:acc>`, `\widehat{xy}`},
		{"vector", `<m:acc><m:accPr><m:chr m:val="⃗"/></m:accPr>` + e(r("v")) + `</m:acc>`, `\vec{v}`},
		{"long vector", `<m:acc><m:accPr><m:chr m:val="⃗"/></m:accPr>` + e(r("AB")) + `</m:acc>`, `\overrightarrow{AB}`},
		{"tilde", `<m:acc><m:accPr><m:chr m:val="̃"/></m:accPr>` + e(r("n")) + `</m:acc>`, `\tilde{n}`},
		{"bar accent", `<m:acc><m:accPr><m:chr m:val="̅"/></m:accPr>` + e(r("z")) + `</m:acc>`, `\bar{z}`},
		{"dots", `<m:acc><m:accPr><m:chr m:val="̇"/></m:accPr>` + e(r("x")) + `</m:acc><m:acc><m:accPr><m:chr m:val="̈"/></m:accPr>` + e(r("y")) + `</m:acc><m:acc><m:accPr><m:chr m:val="⃛"/></m:accPr>` + e(r("z")) + `</m:acc>`, `\dot{x}\ddot{y}\dddot{z}`},
		{"check acute grave breve ring", `<m:acc><m:accPr><m:chr m:val="̌"/></m:accPr>` + e(r("a")) + `</m:acc><m:acc><m:accPr><m:chr m:val="́"/></m:accPr>` + e(r("b")) + `</m:acc><m:acc><m:accPr><m:chr m:val="̀"/></m:accPr>` + e(r("c")) + `</m:acc><m:acc><m:accPr><m:chr m:val="̆"/></m:accPr>` + e(r("d")) + `</m:acc><m:acc><m:accPr><m:chr m:val="̊"/></m:accPr>` + e(r("A")) + `</m:acc>`, `\check{a}\acute{b}\grave{c}\breve{d}\mathring{A}`},
		{"unknown combining accent", `<m:acc><m:accPr><m:chr m:val="⃩"/></m:accPr>` + e(r("ab")) + `</m:acc>`, `ab`},
		{"unknown spacing accent", `<m:acc><m:accPr><m:chr m:val="★"/></m:accPr>` + e(r("x")) + `</m:acc>`, `\overset{\star}{x}`},
		{"greek accent base", `<m:acc>` + e(r("α")) + `</m:acc>`, `\hat{\alpha}`},
		{"overline", `<m:bar><m:barPr><m:pos m:val="top"/></m:barPr>` + e(r("AB")) + `</m:bar>`, `\overline{AB}`},
		{"underline", `<m:bar>` + e(r("x")) + `</m:bar>`, `\underline{x}`},
		{"underbrace", `<m:groupChr>` + e(r("a+b")) + `</m:groupChr>`, `\underbrace{a+b}`},
		{"underbrace with label", `<m:limLow>` + e(`<m:groupChr>`+e(r("a+b"))+`</m:groupChr>`) + `<m:lim>` + r("n") + `</m:lim></m:limLow>`, `\underbrace{a+b}_{n}`},
		{"overbrace with label", `<m:limUpp>` + e(`<m:groupChr><m:groupChrPr><m:chr m:val="⏞"/><m:pos m:val="top"/></m:groupChrPr>`+e(r("x"))+`</m:groupChr>`) + `<m:lim>` + r("k") + `</m:lim></m:limUpp>`, `\overbrace{x}^{k}`},
		{"arrow group", `<m:groupChr><m:groupChrPr><m:chr m:val="→"/><m:pos m:val="top"/></m:groupChrPr>` + e(r("AB")) + `</m:groupChr>`, `\overrightarrow{AB}`},
		{"box", `<m:box>` + e(r("x")) + `</m:box>`, `x`},
		{"border box", `<m:borderBox>` + e(r("E=m")+`<m:sSup>`+e(r("c"))+`<m:sup>`+r("2")+`</m:sup></m:sSup>`) + `</m:borderBox>`, `\boxed{E=mc^{2}}`},
		{"phantom", `<m:phant><m:phantPr><m:show m:val="0"/></m:phantPr>` + e(r("x")) + `</m:phant>`, `\phantom{x}`},
		{"horizontal phantom", `<m:phant><m:phantPr><m:show m:val="off"/><m:zeroAsc/><m:zeroDesc/></m:phantPr>` + e(r("x")) + `</m:phant>`, `\hphantom{x}`},
		{"visible phantom", `<m:phant>` + e(r("x")) + `</m:phant>`, `x`},
		{"aligned array", `<m:eqArr>` + e(r("x&amp;=1")) + e(r("y&amp;=2")) + `</m:eqArr>`, `\begin{aligned}x&=1 \\ y&=2\end{aligned}`},
		{"aligned via m:aln", `<m:eqArr>` + e(r("a")+`<m:r><m:rPr><m:aln/></m:rPr><m:t>=b</m:t></m:r>`) + e(r("c")+`<m:r><m:rPr><m:aln/></m:rPr><m:t>=d</m:t></m:r>`) + `</m:eqArr>`, `\begin{aligned}a&=b \\ c&=d\end{aligned}`},
		{"gathered array", `<m:eqArr>` + e(r("a=b")) + e(r("c=d")) + `</m:eqArr>`, `\begin{gathered}a=b \\ c=d\end{gathered}`},
		{"ampersand inside nested object is escaped", `<m:eqArr>` + e(`<m:f><m:num>`+r("&amp;")+`</m:num><m:den>`+r("2")+`</m:den></m:f>`) + `</m:eqArr>`, `\begin{gathered}\frac{\&}{2}\end{gathered}`},
		{"cases", `<m:d><m:dPr><m:begChr m:val="{"/><m:endChr m:val=""/></m:dPr>` + e(`<m:eqArr>`+e(r("x,&amp;x&gt;0"))+e(r("-x,&amp;x≤0"))+`</m:eqArr>`) + `</m:d>`, `\begin{cases}x,&x>0 \\ -x,&x\leq0\end{cases}`},
		{"tex specials escaped", r(`a_b#c%d$e{f}g~h^i\j`), `a\_b\#c\%d\$e\{f\}g\sim h\hat{}i\backslash j`},
		{"normal text", `<m:r><m:rPr><m:nor/></m:rPr><m:t xml:space="preserve">if x &gt; 0 &amp; 50% {ok}</m:t></m:r>`, `\text{if x > 0 \& 50\% \{ok\}}`},
		{"normal-text operator stays math", r("E") + `<m:r><m:rPr><m:lit/><m:nor/></m:rPr><m:t>=</m:t></m:r>` + r("m"), `E=m`},
		{"normal text with math symbols", `<m:r><m:rPr><m:nor/></m:rPr><m:t>a ≤ b_c ~ d^e \f</m:t></m:r>`, `\text{a $\leq$ b\_c \textasciitilde{} d\textasciicircum{}e \textbackslash{}f}`},
		{"word run inside math", `<w:r><w:t>speed</w:t></w:r>` + r("=v"), `\text{speed}=v`},
		{"unicode operators", r("α≤β≠γ, x→∞, a×b, ±1, ∂f/∂x, x∈A∖B, ∀ε∃δ"), `\alpha\leq\beta\neq\gamma,x\to\infty,a\times b,\pm1,\partial f/\partial x,x\in A\setminus B,\forall\varepsilon\exists\delta`},
		{"capital greek", r("ΔΩΑ"), `\Delta\Omega\mathrm{A}`},
		{"control word spacing", r("αx") + r("βy"), `\alpha x\beta y`},
		{"bold", rs("b", "", "v"), `\mathbf{v}`},
		{"bold greek", rs("b", "", "α"), `\boldsymbol{\alpha}`},
		{"bold italic", rs("bi", "", "x"), `\boldsymbol{x}`},
		{"italic is default", rs("i", "", "x"), `x`},
		{"double struck", rs("p", "double-struck", "R"), `\mathbb{R}`},
		{"script", rs("", "script", "L"), `\mathcal{L}`},
		{"fraktur", rs("", "fraktur", "g"), `\mathfrak{g}`},
		{"sans serif", rs("", "sans-serif", "x"), `\mathsf{x}`},
		{"monospace", rs("", "monospace", "x1"), `\mathtt{x}\mathtt{1}`},
		{"upright word", rs("p", "", "speed"), `\mathrm{speed}`},
		{"upright single letter", rs("p", "", "d"), `\mathrm{d}`},
		{"upright known function", rs("p", "", "max"), `\max`},
		{"upright digits stay plain", rs("p", "", "42"), `42`},
		{"bold digits", rs("b", "", "3.14"), `\mathbf{3.14}`},
		{"math alphanumerics", r("𝐱𝑦𝔤ℝ𝟏"), `\mathbf{x}y\mathfrak{g}\mathbb{R}\mathbf{1}`},
		{"bold greek alphanumeric", r("𝛂"), `\boldsymbol{\alpha}`},
		{"combining accent in text", r("x̂"), `\hat{x}`},
		{"non-ascii identifier", r("ā"), `\textit{ā}`},
		{"non-ascii upright word", rs("p", "", "Siedém"), `\text{Siedém}`},
		{"non-ascii bold word", rs("b", "", "žā"), `\textbf{žā}`},
		{"differential d", r("ⅆx"), `\mathrm{d}x`},
		{"empty limit", `<m:limUpp>` + e(r("x")) + `<m:lim/></m:limUpp>`, `x`},
		{"top parenthesis group", `<m:groupChr><m:groupChrPr><m:chr m:val="⏜"/><m:pos m:val="top"/></m:groupChrPr>` + e(r("AB")) + `</m:groupChr>`, `\overset{\frown}{AB}`},
		{"invisible operators dropped", r("f⁡(x)⁢y"), `f(x)y`},
		{"control characters dropped", r("a\u0090b"), `ab`},
		{"degree and prime", r("90° f′"), `90^{\circ}f'`},
		{"nbsp in math", r("a "), `a\ `},
		{"revisions", `<w:ins>` + r("a") + `</w:ins><w:del>` + r("b") + `</w:del>`, `a`},
		{"properties ignored", `<m:ctrlPr><w:rPr><w:i/></w:rPr></m:ctrlPr>` + r("q"), `q`},
		{"line break with alignment", r("y") + `<m:r><m:rPr><m:aln/></m:rPr><m:t>=a</m:t></m:r><m:r><m:rPr><m:brk/><m:aln/></m:rPr><m:t>=b</m:t></m:r>`, `\begin{aligned}y&=a \\ &=b\end{aligned}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, display, err := Convert(oMath(tt.body))
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if display {
				t.Errorf("display = true for inline oMath")
			}
			if got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func matrix2() string {
	row := func(a, b string) string { return `<m:mr>` + e(r(a)) + e(r(b)) + `</m:mr>` }
	return `<m:m><m:mPr><m:mcs><m:mc><m:mcPr><m:count m:val="2"/><m:mcJc m:val="center"/></m:mcPr></m:mc></m:mcs></m:mPr>` +
		row("1", "0") + row("0", "1") + `</m:m>`
}

func TestConvertMathPara(t *testing.T) {
	one := `<m:oMathPara ` + nsDecl + `><m:oMathParaPr><m:jc m:val="center"/></m:oMathParaPr><m:oMath>` + r("E=m") + `<m:sSup>` + e(r("c")) + `<m:sup>` + r("2") + `</m:sup></m:sSup></m:oMath></m:oMathPara>`
	got, display, err := Convert([]byte(one))
	if err != nil || !display || got != `E=mc^{2}` {
		t.Errorf("single: got %q display=%v err=%v", got, display, err)
	}

	two := `<m:oMathPara ` + nsDecl + `><m:oMath>` + r("a=1") + `</m:oMath><m:oMath>` + r("b=2") + `</m:oMath></m:oMathPara>`
	got, display, err = Convert([]byte(two))
	if err != nil || !display || got != `\begin{gathered}a=1 \\ b=2\end{gathered}` {
		t.Errorf("multi: got %q display=%v err=%v", got, display, err)
	}
}

func TestConvertFindsNestedMath(t *testing.T) {
	doc := `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><w:r><w:t>x</w:t></w:r>` +
		`<m:oMath>` + r("y") + `</m:oMath></w:p>`
	got, _, err := Convert([]byte(doc))
	if err != nil || got != "y" {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestConvertUndeclaredPrefixes(t *testing.T) {
	// Fragments cut out of a document often lose their xmlns declarations.
	got, _, err := Convert([]byte(`<m:oMath><m:f><m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>x</m:t></m:r></m:den></m:f></m:oMath>`))
	if err != nil || got != `\frac{1}{x}` {
		t.Errorf("got %q err=%v", got, err)
	}
}

func TestConvertErrors(t *testing.T) {
	for _, in := range []string{"", "not xml", `<a><b></a>`, `<w:p xmlns:w="x"><w:r/></w:p>`} {
		if _, _, err := Convert([]byte(in)); err == nil {
			t.Errorf("Convert(%q): expected error", in)
		}
	}
}

func TestDeepNestingDoesNotPanic(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<m:oMath ` + nsDecl + `>`)
	for i := 0; i < 5000; i++ {
		sb.WriteString(`<m:sSup><m:e>`)
	}
	sb.WriteString(r("x"))
	for i := 0; i < 5000; i++ {
		sb.WriteString(`</m:e><m:sup>` + r("2") + `</m:sup></m:sSup>`)
	}
	sb.WriteString(`</m:oMath>`)
	if _, _, err := Convert([]byte(sb.String())); err != nil {
		t.Fatalf("Convert: %v", err)
	}
}

func TestToLaTeXNil(t *testing.T) {
	if s, d := ToLaTeX(nil); s != "" || d {
		t.Errorf("ToLaTeX(nil) = %q, %v", s, d)
	}
}

func TestOutputIsBalanced(t *testing.T) {
	// Hostile text must never unbalance braces or smuggle control sequences.
	body := r(`}\input{/etc/passwd}{`) + `<m:r><m:rPr><m:nor/></m:rPr><m:t>}\def\x{}{</m:t></m:r>`
	got, _, err := Convert(oMath(body))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, `\input`) || strings.Contains(got, `\def`) {
		t.Errorf("control sequence leaked: %s", got)
	}
	depth := 0
	for i := 0; i < len(got); i++ {
		switch got[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				t.Fatalf("unbalanced: %s", got)
			}
		}
	}
	if depth != 0 {
		t.Errorf("unbalanced: %s", got)
	}
}

func TestIsAtom(t *testing.T) {
	tests := map[string]bool{
		"x": true, "α": true, `\alpha`: true, `\mathrm{d}`: true, `{a+b}`: true,
		`\left(x\right)`: true, `\left(x\right)+1`: false, `x_{1}`: false, "ab": false,
		`\frac{a}{b}`: false, "": false, `\hat{x}`: true, `\left\{a\right.`: true,
	}
	for in, want := range tests {
		if got := isAtom(in); got != want {
			t.Errorf("isAtom(%q) = %v, want %v", in, got, want)
		}
	}
}
