package omml

// symbols maps non-ASCII characters (and the TeX-special ASCII ones) that
// appear in math text to math-mode LaTeX. An empty value drops the character
// (invisible operators, zero-width spaces).
var symbols = map[rune]string{
	// Greek, lower case. U+03B5 is the "straight" epsilon users type; LaTeX
	// calls it \varepsilon. The same holds for phi.
	'α': `\alpha`, 'β': `\beta`, 'γ': `\gamma`, 'δ': `\delta`, 'ε': `\varepsilon`,
	'ϵ': `\epsilon`, 'ζ': `\zeta`, 'η': `\eta`, 'θ': `\theta`, 'ϑ': `\vartheta`,
	'ι': `\iota`, 'κ': `\kappa`, 'ϰ': `\varkappa`, 'λ': `\lambda`, 'μ': `\mu`,
	'µ': `\mu`, 'ν': `\nu`, 'ξ': `\xi`, 'ο': `o`, 'π': `\pi`, 'ϖ': `\varpi`,
	'ρ': `\rho`, 'ϱ': `\varrho`, 'σ': `\sigma`, 'ς': `\varsigma`, 'τ': `\tau`,
	'υ': `\upsilon`, 'φ': `\varphi`, 'ϕ': `\phi`, 'χ': `\chi`, 'ψ': `\psi`,
	'ω': `\omega`, 'ϝ': `\digamma`,

	// Greek, upper case. Letters identical to Latin ones are upright Latin.
	'Γ': `\Gamma`, 'Δ': `\Delta`, 'Θ': `\Theta`, 'Λ': `\Lambda`, 'Ξ': `\Xi`,
	'Π': `\Pi`, 'Σ': `\Sigma`, 'Υ': `\Upsilon`, 'ϒ': `\Upsilon`, 'Φ': `\Phi`,
	'Ψ': `\Psi`, 'Ω': `\Omega`, '\u2126': `\Omega`,
	'Α': `\mathrm{A}`, 'Β': `\mathrm{B}`, 'Ε': `\mathrm{E}`, 'Ζ': `\mathrm{Z}`,
	'Η': `\mathrm{H}`, 'Ι': `\mathrm{I}`, 'Κ': `\mathrm{K}`, 'Μ': `\mathrm{M}`,
	'Ν': `\mathrm{N}`, 'Ο': `\mathrm{O}`, 'Ρ': `\mathrm{P}`, 'Τ': `\mathrm{T}`,
	'Χ': `\mathrm{X}`,

	// Relations.
	'≤': `\leq`, '≥': `\geq`, '≦': `\leqq`, '≧': `\geqq`, '⩽': `\leqslant`,
	'⩾': `\geqslant`, '≠': `\neq`, '≈': `\approx`, '≊': `\approxeq`, '≡': `\equiv`,
	'≢': `\not\equiv`, '∼': `\sim`, '≃': `\simeq`, '≅': `\cong`, '≇': `\ncong`,
	'∝': `\propto`, '≪': `\ll`, '≫': `\gg`, '≺': `\prec`, '≻': `\succ`,
	'⪯': `\preceq`, '⪰': `\succeq`, '≼': `\preccurlyeq`, '≽': `\succcurlyeq`,
	'∈': `\in`, '∊': `\in`, '∉': `\notin`, '∋': `\ni`, '∍': `\ni`, '∌': `\not\ni`,
	'⊂': `\subset`, '⊃': `\supset`, '⊆': `\subseteq`, '⊇': `\supseteq`,
	'⊄': `\not\subset`, '⊅': `\not\supset`, '⊈': `\nsubseteq`, '⊉': `\nsupseteq`,
	'⊊': `\subsetneq`, '⊋': `\supsetneq`, '⊏': `\sqsubset`, '⊐': `\sqsupset`,
	'⊑': `\sqsubseteq`, '⊒': `\sqsupseteq`, '≐': `\doteq`, '≑': `\doteqdot`,
	'≍': `\asymp`, '≲': `\lesssim`, '≳': `\gtrsim`, '≮': `\not<`, '≯': `\not>`,
	'≰': `\nleq`, '≱': `\ngeq`, '⊢': `\vdash`, '⊣': `\dashv`, '⊨': `\models`,
	'⊥': `\perp`, '⟂': `\perp`, '∥': `\parallel`, '∦': `\nparallel`, '∣': `\mid`,
	'∤': `\nmid`, '≜': `\triangleq`, '≔': `:=`, '≕': `=:`, '⋈': `\bowtie`,
	'⊲': `\triangleleft`, '⊳': `\triangleright`, '⊴': `\trianglelefteq`,
	'⊵': `\trianglerighteq`, '∽': `\backsim`, '≀': `\wr`, '∷': `::`,

	// Binary operators.
	'±': `\pm`, '∓': `\mp`, '×': `\times`, '⨯': `\times`, '÷': `\div`,
	'·': `\cdot`, '⋅': `\cdot`, '∗': `\ast`, '∘': `\circ`, '∙': `\bullet`,
	'•': `\bullet`, '⊕': `\oplus`, '⊖': `\ominus`, '⊗': `\otimes`, '⊘': `\oslash`,
	'⊙': `\odot`, '∪': `\cup`, '∩': `\cap`, '⊎': `\uplus`, '⊓': `\sqcap`,
	'⊔': `\sqcup`, '∧': `\wedge`, '∨': `\vee`, '∖': `\setminus`, '†': `\dagger`,
	'‡': `\ddagger`, '⋆': `\star`, '★': `\star`, '⋄': `\diamond`, '◇': `\diamond`,
	'△': `\triangle`, '▵': `\triangle`, '▽': `\triangledown`, '∔': `\dotplus`,
	'⋉': `\ltimes`, '⋊': `\rtimes`, '⊻': `\veebar`, '−': `-`, '∕': `/`, '⁄': `/`,
	'∶': `:`, '‐': `-`, '‑': `-`, '‒': `-`, '–': `-`, '—': `-`,

	// Miscellaneous symbols.
	'∞': `\infty`, '∂': `\partial`, '∇': `\nabla`, '∅': `\emptyset`,
	'⌀': `\varnothing`, '∀': `\forall`, '∃': `\exists`, '∄': `\nexists`,
	'¬': `\neg`, '√': `\surd`, '∠': `\angle`, '∡': `\measuredangle`,
	'∢': `\sphericalangle`, '°': `^{\circ}`, '′': `'`, '″': `''`, '‴': `'''`,
	'…': `\ldots`, '⋯': `\cdots`, '⋮': `\vdots`, '⋱': `\ddots`, 'ℏ': `\hbar`,
	'ℓ': `\ell`, '℘': `\wp`, 'ℑ': `\Im`, 'ℜ': `\Re`, 'ℵ': `\aleph`, 'ℶ': `\beth`,
	'ℷ': `\gimel`, 'ℸ': `\daleth`, '∴': `\therefore`, '∵': `\because`,
	'□': `\square`, '■': `\blacksquare`, '◊': `\lozenge`, '♠': `\spadesuit`,
	'♡': `\heartsuit`, '♢': `\diamondsuit`, '♣': `\clubsuit`, '♭': `\flat`,
	'♮': `\natural`, '♯': `\sharp`, '⊤': `\top`, '∁': `\complement`, 'ð': `\eth`,
	'ı': `\imath`, 'ȷ': `\jmath`, '℧': `\mho`, '∎': `\blacksquare`, '‰': `\text{‰}`,

	// Arrows.
	'→': `\to`, '←': `\leftarrow`, '↔': `\leftrightarrow`, '↑': `\uparrow`,
	'↓': `\downarrow`, '↕': `\updownarrow`, '⇒': `\Rightarrow`, '⇐': `\Leftarrow`,
	'⇔': `\Leftrightarrow`, '⇑': `\Uparrow`, '⇓': `\Downarrow`, '⇕': `\Updownarrow`,
	'↦': `\mapsto`, '⟼': `\longmapsto`, '⟶': `\longrightarrow`,
	'⟵': `\longleftarrow`, '⟷': `\longleftrightarrow`, '⟹': `\Longrightarrow`,
	'⟸': `\Longleftarrow`, '⟺': `\Longleftrightarrow`, '↗': `\nearrow`,
	'↘': `\searrow`, '↙': `\swarrow`, '↖': `\nwarrow`, '↪': `\hookrightarrow`,
	'↩': `\hookleftarrow`, '⇀': `\rightharpoonup`, '⇁': `\rightharpoondown`,
	'↼': `\leftharpoonup`, '↽': `\leftharpoondown`, '⇌': `\rightleftharpoons`,
	'⇋': `\leftrightharpoons`, '↠': `\twoheadrightarrow`, '↞': `\twoheadleftarrow`,
	'⇄': `\rightleftarrows`, '⇆': `\leftrightarrows`, '⇉': `\rightrightarrows`,
	'⇇': `\leftleftarrows`, '↺': `\circlearrowleft`, '↻': `\circlearrowright`,
	'⇝': `\rightsquigarrow`, '↛': `\nrightarrow`, '↚': `\nleftarrow`,
	'⇏': `\nRightarrow`, '⇍': `\nLeftarrow`, '⇎': `\nLeftrightarrow`,
	'↮': `\nleftrightarrow`,

	// Large operators that show up as plain text.
	'∑': `\sum`, '∏': `\prod`, '∐': `\coprod`, '∫': `\int`, '∬': `\iint`,
	'∭': `\iiint`, '∮': `\oint`, '⋃': `\bigcup`, '⋂': `\bigcap`, '⋁': `\bigvee`,
	'⋀': `\bigwedge`, '⨁': `\bigoplus`, '⨂': `\bigotimes`, '⨀': `\bigodot`,
	'⨄': `\biguplus`, '⨆': `\bigsqcup`,

	// Delimiters.
	'⟨': `\langle`, '⟩': `\rangle`, '〈': `\langle`, '〉': `\rangle`, '⌈': `\lceil`,
	'⌉': `\rceil`, '⌊': `\lfloor`, '⌋': `\rfloor`, '‖': `\|`,

	// Spaces.
	'\u00a0': `\ `, '\u2000': `\enspace`, '\u2001': `\quad`, '\u2002': `\enspace`,
	'\u2003': `\quad`, '\u2004': `\;`, '\u2005': `\:`, '\u2006': `\,`,
	'\u2007': `\enspace`, '\u2008': `\,`, '\u2009': `\,`, '\u200a': `\,`,
	'\u205f': `\:`, '\u3000': `\qquad`,

	// Invisible characters (function application, invisible times, ...).
	'\u2061': "", '\u2062': "", '\u2063': "", '\u2064': "", '\u200b': "",
	'\u200c': "", '\u200d': "", '\ufeff': "", '\u00ad': "",

	// Letterlike symbols.
	'ℝ': `\mathbb{R}`, 'ℕ': `\mathbb{N}`, 'ℤ': `\mathbb{Z}`, 'ℚ': `\mathbb{Q}`,
	'ℂ': `\mathbb{C}`, 'ℙ': `\mathbb{P}`, 'ℍ': `\mathbb{H}`, 'ℬ': `\mathcal{B}`,
	'ℰ': `\mathcal{E}`, 'ℱ': `\mathcal{F}`, 'ℋ': `\mathcal{H}`, 'ℐ': `\mathcal{I}`,
	'ℒ': `\mathcal{L}`, 'ℳ': `\mathcal{M}`, 'ℛ': `\mathcal{R}`, 'ℭ': `\mathfrak{C}`,
	'ℌ': `\mathfrak{H}`, 'ℨ': `\mathfrak{Z}`, 'ℎ': `h`, 'ℯ': `e`, 'ℊ': `g`, 'ℴ': `o`,
	// Double-struck italic differentials and constants are set upright.
	'ⅅ': `\mathrm{D}`, 'ⅆ': `\mathrm{d}`, 'ⅇ': `\mathrm{e}`, 'ⅈ': `\mathrm{i}`, 'ⅉ': `\mathrm{j}`,
	'◦': `\circ`, '⨉': `\times`, '℃': `{}^{\circ}\mathrm{C}`, '⟮': `(`, '⟯': `)`,

	// ASCII characters that are special to TeX.
	'#': `\#`, '$': `\$`, '%': `\%`, '&': `\&`, '_': `\_`, '{': `\{`, '}': `\}`,
	'~': `\sim`, '^': `\hat{}`, '\\': `\backslash`,
}

// isGreekLetter reports Greek letters, which bold styles wrap in
// \boldsymbol (\mathbf does not embolden lower-case Greek).
func isGreekLetter(r rune) bool {
	return (r >= 0x0391 && r <= 0x03A9) || (r >= 0x03B1 && r <= 0x03C9) ||
		r == 'ϵ' || r == 'ϑ' || r == 'ϰ' || r == 'ϖ' || r == 'ϱ' || r == 'ϕ' || r == 'µ'
}

// naryOps maps n-ary operator characters to their LaTeX commands.
var naryOps = map[string]string{
	"∑": `\sum`, "∏": `\prod`, "∐": `\coprod`, "∫": `\int`, "∬": `\iint`,
	"∭": `\iiint`, "⨌": `\iiiint`, "∮": `\oint`, "∯": `\oiint`, "∰": `\oiiint`,
	"⋃": `\bigcup`, "⋂": `\bigcap`, "⋁": `\bigvee`, "⋀": `\bigwedge`,
	"⨁": `\bigoplus`, "⨂": `\bigotimes`, "⨀": `\bigodot`, "⨄": `\biguplus`,
	"⨆": `\bigsqcup`, "∪": `\bigcup`, "∩": `\bigcap`, "∨": `\bigvee`, "∧": `\bigwedge`,
}

// integralOps are the n-ary operators whose limits sit beside the sign by
// default; all others stack limits above and below.
var integralOps = map[string]bool{
	"∫": true, "∬": true, "∭": true, "⨌": true, "∮": true, "∯": true, "∰": true,
	"∱": true, "∲": true, "∳": true,
}

// accents maps accent characters (combining and spacing forms) to the
// narrow and wide LaTeX accent commands.
var accents = map[rune][2]string{
	'\u0302': {`\hat`, `\widehat`}, '^': {`\hat`, `\widehat`}, 'ˆ': {`\hat`, `\widehat`},
	'\u0303': {`\tilde`, `\widetilde`}, '~': {`\tilde`, `\widetilde`}, '˜': {`\tilde`, `\widetilde`},
	'\u0304': {`\bar`, `\overline`}, '\u0305': {`\bar`, `\overline`}, '¯': {`\bar`, `\overline`},
	'‾':      {`\bar`, `\overline`},
	'\u20d7': {`\vec`, `\overrightarrow`}, '\u20d1': {`\vec`, `\overrightarrow`},
	'→':      {`\vec`, `\overrightarrow`},
	'\u20d6': {`\overleftarrow`, `\overleftarrow`}, '←': {`\overleftarrow`, `\overleftarrow`},
	'\u20e1': {`\overleftrightarrow`, `\overleftrightarrow`},
	'↔':      {`\overleftrightarrow`, `\overleftrightarrow`},
	'\u0307': {`\dot`, `\dot`}, '˙': {`\dot`, `\dot`},
	'\u0308': {`\ddot`, `\ddot`}, '¨': {`\ddot`, `\ddot`},
	'\u20db': {`\dddot`, `\dddot`},
	'\u030c': {`\check`, `\check`}, 'ˇ': {`\check`, `\check`},
	'\u0301': {`\acute`, `\acute`}, '´': {`\acute`, `\acute`},
	'\u0300': {`\grave`, `\grave`}, '`': {`\grave`, `\grave`},
	'\u0306': {`\breve`, `\breve`}, '˘': {`\breve`, `\breve`},
	'\u030a': {`\mathring`, `\mathring`}, '˚': {`\mathring`, `\mathring`},
	'\u0332': {`\underline`, `\underline`}, '\u0333': {`\underline`, `\underline`},
}

// functions are the operator names LaTeX defines as commands.
var functions = map[string]string{
	"sin": `\sin`, "cos": `\cos`, "tan": `\tan`, "cot": `\cot`, "sec": `\sec`,
	"csc": `\csc`, "arcsin": `\arcsin`, "arccos": `\arccos`, "arctan": `\arctan`,
	"sinh": `\sinh`, "cosh": `\cosh`, "tanh": `\tanh`, "coth": `\coth`,
	"log": `\log`, "ln": `\ln`, "lg": `\lg`, "exp": `\exp`, "lim": `\lim`,
	"liminf": `\liminf`, "limsup": `\limsup`, "max": `\max`, "min": `\min`,
	"sup": `\sup`, "inf": `\inf`, "det": `\det`, "dim": `\dim`, "ker": `\ker`,
	"deg": `\deg`, "gcd": `\gcd`, "hom": `\hom`, "arg": `\arg`, "Pr": `\Pr`,
	"tg": `\tan`, "ctg": `\cot`,
}

// limitOps take their subscript underneath in display style, so an m:limLow
// on them becomes a plain subscript.
var limitOps = map[string]bool{
	`\lim`: true, `\liminf`: true, `\limsup`: true, `\max`: true, `\min`: true,
	`\sup`: true, `\inf`: true, `\det`: true, `\gcd`: true, `\Pr`: true,
}

// delimTeX converts an m:d delimiter character to its \left/\right form.
func delimTeX(s string) string {
	switch s {
	case "":
		return "."
	case "(", ")", "[", "]", "|", "/":
		return s
	case "{":
		return `\{`
	case "}":
		return `\}`
	case "⟨", "〈", "<", "❬":
		return `\langle`
	case "⟩", "〉", ">", "❭":
		return `\rangle`
	case "‖", "∥":
		return `\|`
	case "⌊":
		return `\lfloor`
	case "⌋":
		return `\rfloor`
	case "⌈":
		return `\lceil`
	case "⌉":
		return `\rceil`
	case "\\":
		return `\backslash`
	case "↑":
		return `\uparrow`
	case "↓":
		return `\downarrow`
	case "↕":
		return `\updownarrow`
	case "⇑":
		return `\Uparrow`
	case "⇓":
		return `\Downarrow`
	case "⟦", "〚":
		return "["
	case "⟧", "〛":
		return "]"
	}
	return "."
}

// matrixEnv returns the amsmath matrix environment drawn with the given
// delimiters, or "" when there is none.
func matrixEnv(beg, end string) string {
	switch {
	case beg == "(" && end == ")":
		return "pmatrix"
	case beg == "[" && end == "]":
		return "bmatrix"
	case beg == "{" && end == "}":
		return "Bmatrix"
	case beg == "|" && end == "|":
		return "vmatrix"
	case (beg == "‖" || beg == "∥") && (end == "‖" || end == "∥"):
		return "Vmatrix"
	}
	return ""
}

// mathAlnum decodes the Mathematical Alphanumeric Symbols block
// (U+1D400–U+1D7FF) into a style command and the plain base character.
func mathAlnum(r rune) (style string, base rune, ok bool) {
	const latin = 52
	latinStyles := [...]string{
		`\mathbf`, ``, `\boldsymbol`, `\mathcal`, `\mathcal`, `\mathfrak`,
		`\mathbb`, `\mathfrak`, `\mathsf`, `\mathsf`, `\mathsf`, `\mathsf`, `\mathtt`,
	}
	switch {
	case r >= 0x1D400 && r < 0x1D400+latin*13:
		off := int(r - 0x1D400)
		idx := off % latin
		style = latinStyles[off/latin]
		if idx < 26 {
			return style, rune('A' + idx), true
		}
		return style, rune('a' + idx - 26), true
	case r >= 0x1D6A8 && r < 0x1D6A8+58*5:
		// Five Greek alphabets of 58 characters each: bold, italic, bold
		// italic, sans bold, sans bold italic.
		const greek = "ΑΒΓΔΕΖΗΘΙΚΛΜΝΞΟΠΡϴΣΤΥΦΧΨΩ∇αβγδεζηθικλμνξοπρςστυφχψω∂ϵϑϰϕϱϖ"
		off := int(r - 0x1D6A8)
		styles := [...]string{`\boldsymbol`, ``, `\boldsymbol`, `\boldsymbol`, `\boldsymbol`}
		letters := []rune(greek)
		idx := off % 58
		if idx >= len(letters) {
			return "", 0, false
		}
		b := letters[idx]
		if b == 'ϴ' {
			b = 'Θ'
		}
		return styles[off/58], b, true
	case r >= 0x1D7CE && r <= 0x1D7FF:
		digitStyles := [...]string{`\mathbf`, `\mathbb`, `\mathsf`, `\mathsf`, `\mathtt`}
		off := int(r - 0x1D7CE)
		return digitStyles[off/10], rune('0' + off%10), true
	}
	return "", 0, false
}
