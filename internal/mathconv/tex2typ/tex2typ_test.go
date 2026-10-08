package tex2typ

import (
	"strings"
	"testing"
)

func TestConvertOutput(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		// Identifiers, numbers and operators.
		{"letters are separate variables", `xy`, `x y`},
		{"names next to letters", `abc + \alpha\beta`, `a b c+alpha beta`},
		{"numbers stay together", `3.14159x`, `3.14159 x`},
		{"unbraced script takes one digit", `x_12`, `x_1 2`},
		{"braced script", `x_{i+1}^{2}`, `x_(i+1)^2`},
		{"script order", `x^{a}_{b}`, `x_b^a`},
		{"quadratic formula", `x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}`, `x=frac(-b plus.minus sqrt(b^2-4 a c), 2 a)`},
		{"greek variants", `\epsilon \varepsilon \phi \varphi \vartheta`, `epsilon.alt epsilon phi.alt phi theta.alt`},
		{"italic capital greek", `\varGamma`, `italic(Gamma)`},
		{"unicode letters separated", `αβ ä`, `α β ä`},
		{"combining accent", "x̂", `hat(x)`},
		{"shorthand characters stay apart", `a <= b -> c`, `a< =b- >c`},

		// Escaping of characters that are syntax in Typst.
		{"slash is not a fraction", `a/b`, `a\/b`},
		{"double slash is not a comment", `a // b`, `a\/\/b`},
		{"typst specials", `x # y $ z @ w`, `x\#y\$z\@w`},
		{"brackets do not pair", `f(x) + [a, b)`, `f\(x\)+\[a,b\)`},
		{"punctuation in arguments", `\frac{a,b;c:d}{(e]}`, `frac(a\,b\;c\:d, \(e\])`},
		{"opening unicode bracket escaped", `\frac{⟨a}{b}`, `frac(\⟨a, b)`},
		{"radical sign is not root syntax", `√2`, `\√2`},
		{"dot after a name", `\alpha.\beta`, `alpha .beta`},
		{"semicolon after code", `\hspace{1em};`, `#h(1em)\;`},
		{"bars are ordinary", `|x| + \|y\|`, `class("normal", bar.v)x class("normal", bar.v)+class("normal", bar.v.double)y class("normal", bar.v.double)`},

		// Text.
		{"text keeps its spaces", `\text{if } x \ge 0`, `"if "x gt.eq 0`},
		{"text escapes quotes", `\text{a "b" \textbackslash}`, `"a \"b\" \\"`},
		{"text with nested math", `\text{for $x$ real}`, `"for "x" real"`},
		{"text styles", `\textbf{b}\textit{i}`, `bold("b")italic("i")`},
		{"text ligatures and accents", `\text{x -- y \'e}`, `"x – y é"`},
		{"math text accent", `\"a \'x`, `ä acute(x)`},

		// Fonts.
		{"upright single letter", `\mathrm{d}x`, `upright(d)x`},
		{"upright word", `\mathrm{max}`, `"max"`},
		{"bold is upright", `\mathbf{x} + \mathbf{AB} + \boldsymbol{\alpha}`, `bold(upright(x))+bold("AB")+bold(alpha)`},
		{"double struck", `\mathbb{R} \mathbb{Rx} \mathcal{L}`, `RR bb(R x)cal(L)`},
		{"old font switch", `{\rm d}x`, `upright(d)x`},
		{"operatorname", `\operatorname{sgn} x`, `op("sgn")x`},
		{"operatorname with limits", `\operatorname*{argmax}_x`, `op("argmax", limits: #true)_x`},

		// Big operators and limits.
		{"operators", `\sum_{i=1}^n \int_0^1 \lim_{x\to 0}`, `sum_(i=1)^n integral_0^1 lim_(x arrow.r 0)`},
		{"limits control", `\sum\limits_i \int\nolimits_0^1`, `limits(sum)_i scripts(integral)_0^1`},
		{"limits on a non-operator ignored", `x\limits^2`, `x^2`},
		{"primes", `f'(x) + f''`, `f'\(x\)+f''`},
		{"prime superscript", `f^{\prime\prime}`, `f''`},
		{"prime before subscript", `x_1'`, `x'_1`},
		{"degree", `90^\circ`, `90 degree`},
		{"prescript", `{}^{14}C`, `""^14 C`},
		{"text subscript", `x_\text{max}`, `x_"max"`},

		// Structures.
		{"display fraction", `\dfrac{1}{2}`, `display(frac(1, 2))`},
		{"tex fraction", `{a \over b} {n \choose k}`, `frac(a, b)binom(n, k)`},
		{"empty argument", `\frac{a}{}`, `frac(a, "")`},
		{"root", `\sqrt[3]{x}`, `root(3, x)`},
		{"braces", `\overbrace{a+b}^{n} \underbrace{c}_{m}`, `overbrace(a+b, n)underbrace(c, m)`},
		{"accents", `\hat{x} \widehat{xy} \vec{v} \overleftarrow{AB} \bar{x} \ddot{x}`, `hat(x)hat(x y)arrow(v)accent(A B, arrow.l)macron(x)accent(x, dot.double)`},
		{"overset", `\overset{!}{=}`, `limits(=)^(!)`},
		{"stacked limit", `\sum_{\substack{a \\ b}}`, "sum_(a \\\nb)"},
		{"boxed", `\boxed{x}`, `#box(stroke: 0.5pt, inset: 3pt, text(top-edge: "bounds", bottom-edge: "bounds", $display(x)$))`},
		{"phantom", `\phantom{x}`, `#hide($x$)`},
		{"colour", `\color{red} x + y`, `text(x+y, fill: #rgb("#FF0000"))`},
		{"colour mix", `\textcolor{red!50}{x}`, `text(x, fill: #rgb("#FF8080"))`},
		{"spacing", `a\,b\:c\;d\!e\quad f\qquad g\ h~i`, `a thin b med c thick d#h(-0.1667em)e quad f wide g space h space.nobreak i`},
		{"lengths", `\hspace{1em} \kern-3mu`, `#h(1em)#h(-0.1667em)`},
		{"negations", `\not= \not\in \not\preceq`, "eq.not in.not⪯̸"},
		{"modulo", `a \equiv b \pmod{n}`, `a equiv b quad\(mod n\)`},
		{"extensible arrow", `\xrightarrow[b]{a}`, `attach(stretch(arrow.r), t: a, b: b)`},
		{"implies has extra space", `\iff`, `thick arrow.l.r.double.long thick`},
		{"dots follow context", `a + \dots + b, \dots`, `a+dots.h.c+b,dots.h`},
		{"class commands", `\mathrel{R} \mathbin{\#}`, `class("relation", R)class("binary", \#)`},
		{"group makes an ordinary atom", `1{,}5`, `1 class("normal", \,)5`},

		// Delimiters.
		{"left right", `\left( \frac{a}{b} \right)`, `lr(paren.l frac(a, b)paren.r)`},
		{"one-sided", `\left. f \right|_0^1`, `lr(f bar.v)_0^1`},
		{"middle", `\left\{ x \middle| x > 0 \right\}`, `lr(brace.l x class("normal", mid(bar.v))x>0 brace.r)`},
		{"big delimiters", `\big( x \bigr)`, `class("normal", lr(paren.l, size: #1.2em))x class("closing", lr(paren.r, size: #1.2em))`},
		{"missing right", `\left( x`, `lr(paren.l x)`},

		// Environments.
		{"pmatrix", `\begin{pmatrix} a & b \\ c & d \end{pmatrix}`, `mat(a, b; c, d)`},
		{"empty cells", `\begin{bmatrix} 1 & \\ & 1 \end{bmatrix}`, `mat(delim: "[", 1, ""; "", 1)`},
		{"array rules", `\begin{array}{c|c} a & b \\ \hline c & d \end{array}`, `mat(delim: #none, augment: #(vline: (1,), hline: (1,)), a, b; c, d)`},
		{"array alignment", `\begin{array}{lr} a & b \end{array}`, `mat(delim: #none, &a, b&)`},
		{"cases", `\begin{cases} 1 & x > 0 \\ 0 & \text{else} \end{cases}`, `cases(1&quad x>0, 0&quad"else")`},
		{"rcases", `\begin{rcases} a & b \end{rcases}`, `cases(reverse: #true, a&quad b)`},
		{"align at the top", `\begin{align} a &= b \\ &= c \end{align}`, "a&=b \\\n&=c"},
		{"align pairs", `\begin{align} a &= b & c &= d \end{align}`, `a&=b wide&c&=d`},
		{"nested aligned", `x = \begin{aligned} a &= b \\ c &= d \end{aligned}`, `x=mat(delim: #none, a&=b; c&=d)`},
		{"gather", `\begin{gather} a \\ b \end{gather}`, "a \\\nb"},
		{"eqnarray", `\begin{eqnarray} a &=& b \end{eqnarray}`, `a&=b`},
		{"equation wrapper", `\begin{equation} \begin{split} a &= b \\ &= c \end{split} \end{equation}`, "a&=b \\\n&=c"},
		{"top-level line break", `a = b \\ c = d \\`, "a=b \\\nc=d"},
		{"smallmatrix", `\begin{smallmatrix} a \end{smallmatrix}`, `text(mat(delim: #none, a), size: #0.7em)`},

		// Malformed input.
		{"unknown command", `\foo{x}`, `op("foo")x`},
		{"unbalanced closing brace", `x}`, `x`},
		{"unclosed group", `{x`, `x`},
		{"missing fraction arguments", `\frac`, `frac("", "")`},
		{"lone backslash", `a \`, `a\\`},
		{"empty", ``, ``},
		{"comment only", `% nothing`, ``},
		{"invalid UTF-8", "\xff", "�"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Convert(tt.in).Typst; got != tt.want {
				t.Errorf("Convert(%q)\n got: %s\nwant: %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestConvertMetadata(t *testing.T) {
	tests := []struct {
		in       string
		label    string
		numbered bool
		warning  string // substring of one warning, "" for none
	}{
		{`E = mc^2`, "", true, ""},
		{`E = mc^2 \label{eq:energy}`, "eq:energy", true, ""},
		{`\label{a} x \label{b}`, "a", true, `\label{b}`},
		{`x \nonumber`, "", false, ""},
		{`x \notag`, "", false, ""},
		{`\begin{align*} a &= b \end{align*}`, "", false, ""},
		{`\begin{equation*} a \end{equation*}`, "", false, ""},
		{`\begin{align} a &= b \nonumber \\ c &= d \end{align}`, "", true, ""},
		{`\begin{align} a &= b \nonumber \\ c &= d \notag \end{align}`, "", false, ""},
		{`\begin{equation} a \label{eq:x} \end{equation}`, "eq:x", true, ""},
		{`\tag{3} x`, "", true, `\tag{3}`},
		{`\foo`, "", true, `unknown command \foo`},
		{`\begin{foo} a \end{foo}`, "", true, "unknown environment foo"},
		{`\left( x`, "", true, `missing \right`},
		{`\frac{a}`, "", true, "missing argument"},
		{`\color{nosuch} x`, "", true, "unknown colour"},
		{`\hspace{3furlongs}`, "", true, "unsupported length"},
		{`\begin{pmatrix} a \end{bmatrix}`, "", true, `\begin{pmatrix} ended by \end{bmatrix}`},
		{`\newcommand{\R}{\mathbb{R}}`, "", true, "macros are not expanded"},
		{`\begin{pmatrix} a \intertext{b} \end{pmatrix}`, "", true, "line break inside a matrix cell"},
	}
	for _, tt := range tests {
		r := Convert(tt.in)
		if r.Label != tt.label || r.Numbered != tt.numbered {
			t.Errorf("Convert(%q): label %q numbered %v, want %q %v", tt.in, r.Label, r.Numbered, tt.label, tt.numbered)
		}
		found := tt.warning == ""
		for _, w := range r.Warnings {
			if tt.warning != "" && strings.Contains(w, tt.warning) {
				found = true
			}
		}
		if !found {
			t.Errorf("Convert(%q): warnings %q lack %q", tt.in, r.Warnings, tt.warning)
		}
		if tt.warning == "" && len(r.Warnings) > 0 {
			t.Errorf("Convert(%q): unexpected warnings %q", tt.in, r.Warnings)
		}
	}
}

func TestWarningsAreDeduplicated(t *testing.T) {
	r := Convert(`\foo \foo \foo`)
	if len(r.Warnings) != 1 {
		t.Fatalf("warnings = %q, want one", r.Warnings)
	}
}

func TestCorpusConverts(t *testing.T) {
	if len(corpus) < 400 {
		t.Fatalf("corpus has %d formulas, want at least 400", len(corpus))
	}
	for _, in := range corpus {
		checkInvariants(t, in)
	}
}

func BenchmarkConvert(b *testing.B) {
	in := strings.Join(corpus[:200], ` \quad `)
	b.SetBytes(int64(len(in)))
	for b.Loop() {
		Convert(in)
	}
}

// TestLinearTime guards against quadratic behaviour: ten times the input
// must not take much more than ten times as long.
func TestLinearTime(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	unit := strings.Join(corpus[:100], ` + `)
	measure := func(n int) float64 {
		in := strings.Repeat(unit+" + ", n)
		res := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				Convert(in)
			}
		})
		return float64(res.NsPerOp())
	}
	small, large := measure(2), measure(20)
	if ratio := large / small; ratio > 25 {
		t.Errorf("10x input took %.1fx longer", ratio)
	}
}
