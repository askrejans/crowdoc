package tex2typ

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// tableFormulas builds one formula per entry of every mapping table so that
// each Typst name the converter can emit is compiled at least once.
func tableFormulas() []string {
	var out []string
	add := func(format string, keys []string) {
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, fmt.Sprintf(format, k))
		}
	}
	add(`a \%s b`, keysOf(symbols))
	add(`\%s_{i=1}^{n} x_i`, keysOf(bigOps))
	add(`\%s x`, keysOf(functions))
	add(`\%s_{n} x`, keysOf(extraFunctions))
	add(`\%s{x} + \%[1]s{xy}`, keysOf(accents))
	add(`\%s{x} + \%[1]s{xy} + \%[1]s{R} + \%[1]s{\alpha} + \%[1]s{x+1}`, keysOf(fontCmds))
	add(`{\%s x} + {\%[1]s xyz}`, keysOf(fontSwitches))
	add(`\%s \frac{a}{b} \sum_i x_i`, keysOf(styleCmds))
	add(`a\%s b`, keysOf(spaces))
	add(`a \%s{f(x)} b \%[1]s[g]{} c \%[1]s[y]{x}`, keysOf(xArrows))
	add(`\%s( x \%[1]s] \%[1]s\| \%[1]s\langle \%[1]s.`, keysOf(bigDelims))
	add(`a \%s{b} c`, keysOf(mathClasses))
	add(`a \not%s b`, prefixed(keysOf(negations)))
	add(`\left\%s \frac{a}{b} \right\%[1]s`, keysOf(delimCmds))
	add(`\begin{%s} a & b \\ c & d \end{%[1]s}`, keysOf(matrixDelims))
	add(`\text{a\%s b}`, keysOf(textSymbols))
	add(`\text{\%s{e}} + \%[1]s{o}`, keysOf(textAccents))
	add(`\textcolor{%s}{x} + \color{%[1]s!40}{y}`, keysOf(namedColors))
	for r := range delimChars {
		out = append(out, `\left`+string(r)+` \frac{a}{b} \right`+string(r))
	}
	return out
}

func keysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// prefixed turns the keys of negations into LaTeX source (\in, =).
func prefixed(keys []string) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		if isASCIILetter(k[0]) {
			out[i] = `\` + k
		} else {
			out[i] = k
		}
	}
	return out
}

// TestTypstCompiles compiles every corpus formula, inline and display, plus
// formulas exercising every table entry, with a real Typst binary and fails
// on any error or warning. It also checks that each symbol name resolves to
// the intended character. Set CROWDOC_TYPST to the typst executable.
func TestTypstCompiles(t *testing.T) {
	bin := os.Getenv("CROWDOC_TYPST")
	if bin == "" {
		t.Skip("set CROWDOC_TYPST to a typst binary to validate the generated markup")
	}
	random := 3000
	if n, err := strconv.Atoi(os.Getenv("CROWDOC_TYPST_RANDOM")); err == nil {
		random = n
	}
	inputs := append(append(append([]string{}, corpus...), tableFormulas()...), randomFormulas(random)...)

	var doc strings.Builder
	line := 1
	emit := func(text, desc string) {
		doc.WriteString(text)
		line += strings.Count(text, "\n")
	}
	emit("#set page(width: 18cm, height: auto, margin: 1cm)\n", "preamble")
	for i, in := range inputs {
		r := Convert(in)
		desc := fmt.Sprintf("input %d %q\n    typst: %s", i, in, r.Typst)
		emit(fmt.Sprintf("#block[$ %s $]\n#block[x $%s$ y]\n", r.Typst, r.Typst), desc)
	}
	checkSymbols := func(m map[string]sym) {
		for _, k := range sortedKeys(m) {
			s := m[k]
			if s.typ == "" {
				continue
			}
			emit(fmt.Sprintf("#assert.eq(eval(%s, mode: \"math\").body.text.codepoints().first(), %s, message: %s)\n",
				quote(s.typ), quote(string(s.ch)), quote(`\`+k+` -> `+s.typ)), "symbol \\"+k)
		}
	}
	checkSymbols(symbols)
	checkSymbols(bigOps)

	dir := t.TempDir()
	src := filepath.Join(dir, "corpus.typ")
	if err := os.WriteFile(src, []byte(doc.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "compile", "--root", dir, src, filepath.Join(dir, "corpus.pdf")).CombinedOutput()
	text := string(out)
	if err == nil && !strings.Contains(text, "warning") && !strings.Contains(text, "error") {
		t.Logf("compiled %d formulas (%d lines) cleanly", len(inputs), line)
		return
	}
	// The whole document failed: bisect the formulas to report every
	// offender rather than only the first one Typst stops at.
	failures := 0
	var bisect func(idx []int)
	bisect = func(idx []int) {
		var b strings.Builder
		for _, i := range idx {
			r := Convert(inputs[i])
			fmt.Fprintf(&b, "#block[$ %s $]\n#block[x $%s$ y]\n", r.Typst, r.Typst)
		}
		path := filepath.Join(dir, "part.typ")
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(bin, "compile", "--root", dir, path, path+".pdf").CombinedOutput()
		if err == nil && len(out) == 0 {
			return
		}
		if len(idx) == 1 {
			failures++
			i := idx[0]
			t.Errorf("input %d %q\n    typst: %s\n%s", i, inputs[i], Convert(inputs[i]).Typst, truncate(string(out), 800))
			return
		}
		bisect(idx[:len(idx)/2])
		bisect(idx[len(idx)/2:])
	}
	all := make([]int, len(inputs))
	for i := range all {
		all[i] = i
	}
	bisect(all)
	if failures == 0 {
		t.Fatalf("typst reported problems (%v):\n%s", err, truncate(text, 6000))
	}
}

func sortedKeys[V any](m map[string]V) []string {
	k := keysOf(m)
	sort.Strings(k)
	return k
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…"
}
