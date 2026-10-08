package mathml

// symbols maps Unicode characters to LaTeX math-mode commands. Characters
// not listed here are emitted as they are (the engines use unicode-math).
var symbols = map[rune]string{
	// Greek
	'α': `\alpha`, 'β': `\beta`, 'γ': `\gamma`, 'δ': `\delta`, 'ε': `\varepsilon`,
	'ϵ': `\epsilon`, 'ζ': `\zeta`, 'η': `\eta`, 'θ': `\theta`, 'ϑ': `\vartheta`,
	'ι': `\iota`, 'κ': `\kappa`, 'ϰ': `\varkappa`, 'λ': `\lambda`, 'μ': `\mu`, 'µ': `\mu`,
	'ν': `\nu`, 'ξ': `\xi`, 'ο': `o`, 'π': `\pi`, 'ϖ': `\varpi`, 'ρ': `\rho`,
	'ϱ': `\varrho`, 'σ': `\sigma`, 'ς': `\varsigma`, 'τ': `\tau`, 'υ': `\upsilon`,
	'φ': `\varphi`, 'ϕ': `\phi`, 'χ': `\chi`, 'ψ': `\psi`, 'ω': `\omega`,
	'Α': `A`, 'Β': `B`, 'Γ': `\Gamma`, 'Δ': `\Delta`, 'Ε': `E`, 'Ζ': `Z`, 'Η': `H`,
	'Θ': `\Theta`, 'Ι': `I`, 'Κ': `K`, 'Λ': `\Lambda`, 'Μ': `M`, 'Ν': `N`, 'Ξ': `\Xi`,
	'Ο': `O`, 'Π': `\Pi`, 'Ρ': `P`, 'Σ': `\Sigma`, 'Τ': `T`, 'Υ': `\Upsilon`,
	'Φ': `\Phi`, 'Χ': `X`, 'Ψ': `\Psi`, 'Ω': `\Omega`, '\u2126': `\Omega`,
	'ϒ': `\Upsilon`, 'ϝ': `\digamma`,

	// Binary operators
	'±': `\pm`, '∓': `\mp`, '×': `\times`, '÷': `\div`, '·': `\cdot`, '⋅': `\cdot`,
	'∗': `\ast`, '∘': `\circ`, '∙': `\bullet`, '•': `\bullet`, '⊕': `\oplus`,
	'⊖': `\ominus`, '⊗': `\otimes`, '⊘': `\oslash`, '⊙': `\odot`, '∧': `\wedge`,
	'∨': `\vee`, '∩': `\cap`, '∪': `\cup`, '∖': `\setminus`, '⊔': `\sqcup`,
	'⊓': `\sqcap`, '⊎': `\uplus`, '†': `\dagger`, '‡': `\ddagger`, '⋆': `\star`,
	'≀': `\wr`, '−': `-`, '⁄': `/`, '∕': `/`, '⋉': `\ltimes`, '⋊': `\rtimes`,
	'⊞': `\boxplus`, '⊠': `\boxtimes`, '⊡': `\boxdot`, '⊟': `\boxminus`,
	'⋯': `\cdots`, '…': `\ldots`, '⋮': `\vdots`, '⋱': `\ddots`,
	'′': `'`, '″': `''`, '‴': `'''`,

	// Relations
	'≤': `\leq`, '≥': `\geq`, '≠': `\neq`, '≈': `\approx`, '≡': `\equiv`,
	'≅': `\cong`, '∼': `\sim`, '≃': `\simeq`, '∝': `\propto`, '≪': `\ll`,
	'≫': `\gg`, '≺': `\prec`, '≻': `\succ`, '⪯': `\preceq`, '⪰': `\succeq`,
	'∈': `\in`, '∉': `\notin`, '∋': `\ni`, '∊': `\in`, '⊂': `\subset`, '⊃': `\supset`,
	'⊆': `\subseteq`, '⊇': `\supseteq`, '⊊': `\subsetneq`, '⊋': `\supsetneq`,
	'⊄': `\not\subset`, '⊅': `\not\supset`, '⊥': `\perp`, '∥': `\parallel`,
	'∦': `\nparallel`, '∣': `\mid`, '∤': `\nmid`, '⊢': `\vdash`, '⊣': `\dashv`,
	'⊨': `\models`, '≔': `:=`, '≜': `\triangleq`, '≐': `\doteq`, '≍': `\asymp`,
	'⩽': `\leqslant`, '⩾': `\geqslant`, '≦': `\leqq`, '≧': `\geqq`,
	'≲': `\lesssim`, '≳': `\gtrsim`, '∽': `\backsim`, '≉': `\not\approx`,
	'≢': `\not\equiv`, '⊏': `\sqsubset`, '⊐': `\sqsupset`, '⊑': `\sqsubseteq`,
	'⊒': `\sqsupseteq`, '≮': `\not<`, '≯': `\not>`, '≰': `\nleq`, '≱': `\ngeq`,
	'⋈': `\bowtie`, '⌣': `\smile`, '⌢': `\frown`,

	// Arrows
	'→': `\to`, '←': `\leftarrow`, '↔': `\leftrightarrow`, '⇒': `\Rightarrow`,
	'⇐': `\Leftarrow`, '⇔': `\Leftrightarrow`, '↦': `\mapsto`, '↑': `\uparrow`,
	'↓': `\downarrow`, '↕': `\updownarrow`, '⇑': `\Uparrow`, '⇓': `\Downarrow`,
	'⟶': `\longrightarrow`, '⟵': `\longleftarrow`, '⟷': `\longleftrightarrow`,
	'⟹': `\Longrightarrow`, '⟸': `\Longleftarrow`, '⟺': `\Longleftrightarrow`,
	'⟼': `\longmapsto`, '↪': `\hookrightarrow`, '↩': `\hookleftarrow`,
	'⇀': `\rightharpoonup`, '↼': `\leftharpoonup`, '⇌': `\rightleftharpoons`,
	'↗': `\nearrow`, '↘': `\searrow`, '↖': `\nwarrow`, '↙': `\swarrow`,
	'⇝': `\rightsquigarrow`, '↠': `\twoheadrightarrow`, '↣': `\rightarrowtail`,
	'⇄': `\rightleftarrows`, '⇉': `\rightrightarrows`,

	// Miscellaneous symbols
	'∞': `\infty`, '∂': `\partial`, '∇': `\nabla`, '∅': `\emptyset`, '⌀': `\emptyset`,
	'∃': `\exists`, '∄': `\nexists`, '∀': `\forall`, '¬': `\neg`, 'ℓ': `\ell`,
	'ℏ': `\hbar`, 'ℜ': `\Re`, 'ℑ': `\Im`, 'ℵ': `\aleph`, 'ℶ': `\beth`, '℘': `\wp`,
	'°': `^{\circ}`, '∠': `\angle`, '∡': `\measuredangle`, '△': `\triangle`,
	'□': `\square`, '◻': `\square`, '∎': `\blacksquare`, '■': `\blacksquare`,
	'⊤': `\top`, '√': `\surd`, '∴': `\therefore`, '∵': `\because`, '⋄': `\diamond`,
	'♠': `\spadesuit`, '♣': `\clubsuit`, '♥': `\heartsuit`, '♦': `\diamondsuit`,
	'⟨': `\langle`, '⟩': `\rangle`, '〈': `\langle`, '〉': `\rangle`,
	'\u2329': `\langle`, '\u232A': `\rangle`, '⌈': `\lceil`, '⌉': `\rceil`, '⌊': `\lfloor`, '⌋': `\rfloor`, '‖': `\|`,
	'ℝ': `\mathbb{R}`, 'ℕ': `\mathbb{N}`, 'ℤ': `\mathbb{Z}`, 'ℚ': `\mathbb{Q}`,
	'ℂ': `\mathbb{C}`, 'ℙ': `\mathbb{P}`, 'ℍ': `\mathbb{H}`,
	'ⅆ': `\mathrm{d}`, 'ⅇ': `\mathrm{e}`, 'ⅈ': `\mathrm{i}`, 'ⅉ': `\mathrm{j}`,
	'ℎ': `h`, 'ı': `\imath`, 'ȷ': `\jmath`,

	// Large operators
	'∑': `\sum`, '∏': `\prod`, '∐': `\coprod`, '∫': `\int`, '∬': `\iint`,
	'∭': `\iiint`, '∮': `\oint`, '⋃': `\bigcup`, '⋂': `\bigcap`, '⨁': `\bigoplus`,
	'⨂': `\bigotimes`, '⨀': `\bigodot`, '⋁': `\bigvee`, '⋀': `\bigwedge`,
	'⨄': `\biguplus`, '⨆': `\bigsqcup`,

	// TeX special characters
	'#': `\#`, '$': `\$`, '%': `\%`, '&': `\&`, '_': `\_`, '{': `\{`, '}': `\}`,
	'\\': `\backslash`, '~': `\sim`, '^': `\hat{}`,

	// Spaces and invisible operators
	'\u00A0': `~`, '\u2002': `\enspace`, '\u2003': `\quad`, '\u2004': `\;`,
	'\u2005': `\:`, '\u2006': `\,`, '\u2009': `\,`, '\u200A': `\,`, '\u202F': `\,`,
	'\u205F': `\:`, '\u200B': ``, '\u2061': ``, '\u2062': ``, '\u2063': ``, '\u2064': ``,
	'\uFEFF': ``,
}

// functions are multi-letter identifiers typeset as upright operators.
var functions = map[string]string{
	"sin": `\sin`, "cos": `\cos`, "tan": `\tan`, "cot": `\cot`, "sec": `\sec`,
	"csc": `\csc`, "arcsin": `\arcsin`, "arccos": `\arccos`, "arctan": `\arctan`,
	"sinh": `\sinh`, "cosh": `\cosh`, "tanh": `\tanh`, "coth": `\coth`,
	"log": `\log`, "ln": `\ln`, "lg": `\lg`, "exp": `\exp`, "lim": `\lim`,
	"limsup": `\limsup`, "lim sup": `\limsup`, "liminf": `\liminf`, "lim inf": `\liminf`,
	"max": `\max`, "min": `\min`, "sup": `\sup`, "inf": `\inf`, "det": `\det`,
	"dim": `\dim`, "ker": `\ker`, "deg": `\deg`, "gcd": `\gcd`, "hom": `\hom`,
	"arg": `\arg`, "Pr": `\Pr`, "mod": `\bmod`,
}

// limitOps take their scripts as limits in munder/mover constructs.
var limitOps = map[string]bool{
	`\sum`: true, `\prod`: true, `\coprod`: true, `\int`: true, `\iint`: true,
	`\iiint`: true, `\oint`: true, `\bigcup`: true, `\bigcap`: true,
	`\bigoplus`: true, `\bigotimes`: true, `\bigodot`: true, `\bigvee`: true,
	`\bigwedge`: true, `\biguplus`: true, `\bigsqcup`: true, `\lim`: true,
	`\limsup`: true, `\liminf`: true, `\max`: true, `\min`: true, `\sup`: true,
	`\inf`: true, `\det`: true, `\gcd`: true, `\Pr`: true,
}

// delimiters are the characters valid after \left / \right.
var delimiters = map[string]string{
	"(": "(", ")": ")", "[": "[", "]": "]", "{": `\{`, "}": `\}`, "|": "|",
	"‖": `\|`, "∥": `\|`, "⟨": `\langle`, "⟩": `\rangle`, "〈": `\langle`,
	"〉": `\rangle`, "<": `\langle`, ">": `\rangle`, "⌊": `\lfloor`, "⌋": `\rfloor`,
	"⌈": `\lceil`, "⌉": `\rceil`, "/": "/", `\`: `\backslash`, "∣": "|", "": ".",
	"\u2329": `\langle`, "\u232A": `\rangle`,
}

var openFences = map[string]bool{"(": true, "[": true, "{": true, "⟨": true, "〈": true, "⌊": true, "⌈": true, "|": true, "‖": true, "∥": true, "∣": true, "⟦": true, "\u2329": true}
var closeFences = map[string]bool{")": true, "]": true, "}": true, "⟩": true, "〉": true, "⌋": true, "⌉": true, "|": true, "‖": true, "∥": true, "∣": true, "⟧": true, "\u232A": true}

// matrixEnvs picks an amsmath matrix environment from its fences.
var matrixEnvs = map[string]string{
	"()": "pmatrix", "[]": "bmatrix", "{}": "Bmatrix", "||": "vmatrix",
	"∣∣": "vmatrix", "‖‖": "Vmatrix", "∥∥": "Vmatrix",
}

type accent struct {
	narrow, wide string
}

// overAccents are mover scripts that act as accents on the base.
var overAccents = map[string]accent{
	"^": {`\hat`, `\widehat`}, "ˆ": {`\hat`, `\widehat`}, "\u0302": {`\hat`, `\widehat`},
	"~": {`\tilde`, `\widetilde`}, "˜": {`\tilde`, `\widetilde`}, "\u0303": {`\tilde`, `\widetilde`},
	"∼": {`\tilde`, `\widetilde`},
	"¯": {`\bar`, `\overline`}, "‾": {`\bar`, `\overline`}, "\u0304": {`\bar`, `\overline`},
	"\u0305": {`\bar`, `\overline`}, "_": {`\bar`, `\overline`}, "―": {`\bar`, `\overline`},
	"→": {`\vec`, `\overrightarrow`}, "\u20D7": {`\vec`, `\overrightarrow`},
	"⟶": {`\overrightarrow`, `\overrightarrow`},
	"←": {`\overleftarrow`, `\overleftarrow`}, "\u20D6": {`\overleftarrow`, `\overleftarrow`},
	"↔": {`\overleftrightarrow`, `\overleftrightarrow`}, "\u20E1": {`\overleftrightarrow`, `\overleftrightarrow`},
	"˙": {`\dot`, `\dot`}, "\u0307": {`\dot`, `\dot`}, ".": {`\dot`, `\dot`},
	"¨": {`\ddot`, `\ddot`}, "\u0308": {`\ddot`, `\ddot`}, "..": {`\ddot`, `\ddot`},
	"\u20DB": {`\dddot`, `\dddot`}, "...": {`\dddot`, `\dddot`},
	"ˇ": {`\check`, `\check`}, "\u030C": {`\check`, `\check`},
	"˘": {`\breve`, `\breve`}, "\u0306": {`\breve`, `\breve`},
	"´": {`\acute`, `\acute`}, "\u0301": {`\acute`, `\acute`},
	"`": {`\grave`, `\grave`}, "\u0300": {`\grave`, `\grave`},
	"˚": {`\mathring`, `\mathring`}, "\u030A": {`\mathring`, `\mathring`},
	"⏞": {`\overbrace`, `\overbrace`}, "︷": {`\overbrace`, `\overbrace`},
	"⏜": {`\overbrace`, `\overbrace`}, "⎴": {`\overbrace`, `\overbrace`},
}

// underAccents are munder scripts that act as accents below the base.
var underAccents = map[string]accent{
	"_": {`\underline`, `\underline`}, "\u0332": {`\underline`, `\underline`},
	"¯": {`\underline`, `\underline`}, "‾": {`\underline`, `\underline`},
	"―": {`\underline`, `\underline`},
	"⏟": {`\underbrace`, `\underbrace`}, "︸": {`\underbrace`, `\underbrace`},
	"⏝": {`\underbrace`, `\underbrace`}, "⎵": {`\underbrace`, `\underbrace`},
	"→": {`\underrightarrow`, `\underrightarrow`}, "←": {`\underleftarrow`, `\underleftarrow`},
	"↔": {`\underleftrightarrow`, `\underleftrightarrow`},
}

// extensibleArrows become \xrightarrow-style commands when text is set over
// (or under) them.
var extensibleArrows = map[string]string{
	"→": `\xrightarrow`, "⟶": `\xrightarrow`, "←": `\xleftarrow`, "⟵": `\xleftarrow`,
}

// mathAlnumRanges describe the Mathematical Alphanumeric Symbols block
// (U+1D400–U+1D7FF): each range holds A–Z then a–z in one style.
var mathAlnumRanges = []struct {
	start   rune
	variant string
}{
	{0x1D400, "bold"}, {0x1D434, "italic"}, {0x1D468, "bold-italic"},
	{0x1D49C, "script"}, {0x1D4D0, "bold-script"}, {0x1D504, "fraktur"},
	{0x1D538, "double-struck"}, {0x1D56C, "bold-fraktur"}, {0x1D5A0, "sans-serif"},
	{0x1D5D4, "bold-sans-serif"}, {0x1D608, "sans-serif-italic"},
	{0x1D63C, "sans-serif-bold-italic"}, {0x1D670, "monospace"},
}

// mathDigitRanges hold 0–9 in one style each.
var mathDigitRanges = []struct {
	start   rune
	variant string
}{
	{0x1D7CE, "bold"}, {0x1D7D8, "double-struck"}, {0x1D7E2, "sans-serif"},
	{0x1D7EC, "bold-sans-serif"}, {0x1D7F6, "monospace"},
}

// dangerous lists control sequences that must never appear in TeX taken
// from untrusted input: file and shell access, catcode or macro tricks,
// and engine scripting.
var dangerous = map[string]bool{
	"input": true, "include": true, "InputIfFileExists": true, "IfFileExists": true,
	"write": true, "immediate": true, "openout": true, "openin": true, "read": true,
	"readline": true, "closeout": true, "closein": true, "newwrite": true,
	"newread": true, "catcode": true, "def": true, "edef": true, "gdef": true,
	"xdef": true, "let": true, "futurelet": true, "csname": true, "endcsname": true,
	"expandafter": true, "directlua": true, "luaexec": true, "luadirect": true,
	"latelua": true, "luacode": true, "special": true, "newcommand": true,
	"renewcommand": true, "providecommand": true, "DeclareRobustCommand": true,
	"DeclareMathOperator": true, "usepackage": true, "RequirePackage": true,
	"documentclass": true, "makeatletter": true, "makeatother": true,
	"jobname": true, "endinput": true, "output": true, "everypar": true,
	"everymath": true, "everydisplay": true, "everyhbox": true, "everyvbox": true,
	"shipout": true, "typeout": true, "message": true, "errmessage": true,
	"loop": true, "afterassignment": true, "aftergroup": true, "uppercase": true,
	"lowercase": true, "scantokens": true, "outer": true, "long": true,
	"global": true, "includegraphics": true, "verbatiminput": true,
	"lstinputlisting": true, "inputminted": true, "pdfprimitive": true,
	"primitive": true, "font": true, "openany": true, "filecontents": true,
	"ShellEscape": true, "write18": true, "pdfshellescape": true,
	"pdfobj": true, "pdfliteral": true, "pdfximage": true, "saveimageresource": true,
	"newenvironment": true, "renewenvironment": true, "halt": true,
	"batchmode": true, "nonstopmode": true, "scrollmode": true, "errorstopmode": true,
	"dump": true, "patterns": true, "hyphenation": true, "endlinechar": true,
	"newlinechar": true, "escapechar": true, "lccode": true, "uccode": true,
	"mathcode": true, "delcode": true, "sfcode": true, "count": true,
	"dimen": true, "skip": true, "toks": true, "box": true, "setbox": true,
	"chardef": true, "mathchardef": true, "countdef": true, "toksdef": true,
	"futurenonspacelet": true, "noexpand": true, "unexpanded": true,
	"detokenize": true, "string": true, "meaning": true, "romannumeral": true,
	"url": true, "href": true, "hyperref": true,
}
