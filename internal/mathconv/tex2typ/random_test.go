package tex2typ

import (
	"math/rand/v2"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// fragments are the building blocks of randomly assembled formulas. They
// mix valid structure with fragments that unbalance it.
var fragments = []string{
	"x", "y", "ab", "2", "3.14", "+", "-", "=", "<", ">", ",", ";", ":", ".", "!", "|",
	"(", ")", "[", "]", "{", "}", "^", "_", "'", "&", `\\`, "~", "#", "%\n", "/", "@",
	`"`, "`", "$", `\$`, `\{`, `\}`, `\|`, `\,`, `\;`, `\!`, `\ `, `\quad`,
	`\alpha`, `\Gamma`, `\infty`, `\sum`, `\int`, `\lim`, `\sin`, `\cdot`, `\leq`,
	`\to`, `\langle`, `\rangle`, `\lfloor`, `\rceil`, `\ldots`, `\dots`, `\prime`,
	`\frac`, `\frac{a}{b}`, `\sqrt`, `\sqrt[3]`, `\binom`, `\over`, `\choose`,
	`\left(`, `\right)`, `\left.`, `\right|`, `\middle|`, `\left\{`, `\right\}`,
	`\big(`, `\Bigr]`, `\text{`, `\text{a b}`, `\mathrm{`, `\mathbf{x}`, `\mathbb{R}`,
	`\operatorname{f}`, `\operatorname*{argmax}`, `\hat`, `\vec{v}`, `\overline{`,
	`\overbrace{x}^{n}`, `\underbrace{`, `\boxed{`, `\phantom{`, `\color{red}`,
	`\textcolor{blue}{`, `\displaystyle`, `\rm`, `\limits`, `\nolimits`, `\not`,
	`\not=`, `\begin{pmatrix}`, `\end{pmatrix}`, `\begin{cases}`, `\end{cases}`,
	`\begin{aligned}`, `\end{aligned}`, `\begin{align}`, `\end{align}`,
	`\begin{array}{c|l}`, `\end{array}`, `\hline`, `\label{x}`, `\tag{1}`, `\nonumber`,
	`\substack{`, `\xrightarrow{`, `\hspace{1em}`, `\kern3mu`, `\unknown`, `\'`,
	`\mathop{`, `\mathrel{`, `\sideset{_a}{^b}`, `\stackrel{`, `\cancel{`, `α`, `∑`,
	`⟨`, `√`, "̂", `ℝ`, `日`, `\u`, `\\[2pt]`, `\intertext{`, `\pmod{`,
}

// commandPool holds every control sequence and environment the tables know.
var commandPool = func() []string {
	var out []string
	for _, keys := range [][]string{
		keysOf(symbols), keysOf(bigOps), keysOf(functions), keysOf(extraFunctions),
		keysOf(accents), keysOf(fontCmds), keysOf(fontSwitches), keysOf(styleCmds),
		keysOf(spaces), keysOf(xArrows), keysOf(bigDelims), keysOf(mathClasses),
		keysOf(textSymbols), keysOf(textAccents),
	} {
		for _, k := range sortedCopy(keys) {
			out = append(out, `\`+k)
		}
	}
	for _, env := range []string{"matrix", "pmatrix", "bmatrix", "Vmatrix", "smallmatrix", "pmatrix*",
		"array", "cases", "dcases", "rcases", "aligned", "alignedat", "split", "gathered",
		"align", "align*", "alignat", "gather", "multline", "eqnarray", "equation", "subarray", "foo"} {
		out = append(out, `\begin{`+env+`}`, `\end{`+env+`}`)
	}
	return out
}()

func sortedCopy(keys []string) []string {
	out := append([]string(nil), keys...)
	sort.Strings(out)
	return out
}

// randomFormula assembles a formula of up to n fragments.
func randomFormula(r *rand.Rand, n int) string {
	var b strings.Builder
	for i := r.IntN(n) + 1; i > 0; i-- {
		if r.IntN(3) == 0 {
			b.WriteString(commandPool[r.IntN(len(commandPool))])
		} else {
			b.WriteString(fragments[r.IntN(len(fragments))])
		}
		if r.IntN(3) == 0 {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func randomFormulas(count int) []string {
	seed := uint64(1)
	if s, err := strconv.ParseUint(os.Getenv("CROWDOC_RANDOM_SEED"), 10, 64); err == nil {
		seed = s
	}
	r := rand.New(rand.NewPCG(seed, 2))
	out := make([]string, count)
	for i := range out {
		out[i] = randomFormula(r, 24)
	}
	return out
}

func checkInvariants(t *testing.T, in string) {
	t.Helper()
	res := Convert(in)
	if !utf8.ValidString(res.Typst) {
		t.Fatalf("%q: invalid UTF-8 output %q", in, res.Typst)
	}
	trailing := len(res.Typst) - len(strings.TrimRight(res.Typst, `\`))
	if trailing%2 == 1 {
		t.Fatalf("%q: output ends with a lone backslash: %q", in, res.Typst)
	}
	if again := Convert(in); again.Typst != res.Typst {
		t.Fatalf("%q: conversion is not deterministic", in)
	}
}

func TestRandomFormulasKeepInvariants(t *testing.T) {
	for _, in := range randomFormulas(5000) {
		checkInvariants(t, in)
	}
}

func FuzzConvert(f *testing.F) {
	for _, s := range corpus[:60] {
		f.Add(s)
	}
	f.Add(`\left(\begin{array}{c|c} a & b \\ \hline c & d \end{array}\right)`)
	f.Add(`{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{x`)
	f.Fuzz(func(t *testing.T, in string) {
		checkInvariants(t, in)
	})
}

// TestPathologicalInputs checks that hostile nesting and repetition neither
// overflow the stack nor produce markup deeper than Typst accepts.
func TestPathologicalInputs(t *testing.T) {
	cases := map[string]string{
		"groups":        strings.Repeat("{", 100000) + "x" + strings.Repeat("}", 100000),
		"unclosed":      strings.Repeat("{", 100000),
		"fractions":     strings.Repeat(`\frac{`, 5000) + "x" + strings.Repeat("}{y}", 5000),
		"bare fracs":    strings.Repeat(`\frac`, 20000) + "xy",
		"sqrt chain":    strings.Repeat(`\sqrt`, 20000) + "x",
		"lefts":         strings.Repeat(`\left(`, 20000) + "x",
		"switches":      strings.Repeat(`\rm `, 20000) + "x",
		"over chain":    strings.Repeat(`a \over `, 20000) + "b",
		"colors":        strings.Repeat(`\color{red}`, 20000) + "x",
		"scripts":       "x" + strings.Repeat("^a", 20000),
		"primes":        "x" + strings.Repeat("'", 20000),
		"environments":  strings.Repeat(`\begin{pmatrix}`, 20000),
		"text":          strings.Repeat(`\text{\textbf{`, 20000),
		"text unbraced": strings.Repeat(`\textbf`, 20000) + "x",
		"text math":     strings.Repeat(`\text{$`, 20000),
		"nots":          strings.Repeat(`\not`, 20000) + "=",
		"accents":       strings.Repeat(`\hat`, 20000) + "x",
		"long flat":     strings.Repeat(`a+\alpha\cdot\frac{1}{2}`, 20000),
	}
	for name, in := range cases {
		res := Convert(in)
		if depth := nestingDepth(res.Typst); depth > 200 {
			t.Errorf("%s: output nests %d levels", name, depth)
		}
	}
}

// nestingDepth measures parenthesis/bracket nesting outside string literals.
func nestingDepth(s string) int {
	depth, max := 0, 0
	inStr := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			i++
		case c == '"':
			inStr = !inStr
		case inStr:
		case c == '(' || c == '[':
			if depth++; depth > max {
				max = depth
			}
		case c == ')' || c == ']':
			depth--
		}
	}
	return max
}
