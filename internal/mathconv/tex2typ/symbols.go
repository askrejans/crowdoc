package tex2typ

// symClass is the TeX math class of a symbol, used for context decisions
// such as \dots and limit placement.
type symClass uint8

const (
	clsOrd symClass = iota
	clsBin
	clsRel
	clsOp // large operator
	clsOpen
	clsClose
	clsPunct
)

// sym describes a control sequence that maps to a single Typst symbol.
type sym struct {
	typ string   // Typst name in the math scope; empty: emit ch literally
	ch  rune     // the character the command denotes
	cls symClass // TeX class
}

// symbols maps LaTeX control words (and control symbols) to Typst symbols.
// Every name is verified against the Typst release in the validation test,
// including that it resolves to ch without a deprecation warning.
var symbols = map[string]sym{
	// Greek lowercase.
	"alpha": {"alpha", 'α', clsOrd}, "beta": {"beta", 'β', clsOrd},
	"gamma": {"gamma", 'γ', clsOrd}, "delta": {"delta", 'δ', clsOrd},
	"epsilon": {"epsilon.alt", 'ϵ', clsOrd}, "varepsilon": {"epsilon", 'ε', clsOrd},
	"zeta": {"zeta", 'ζ', clsOrd}, "eta": {"eta", 'η', clsOrd},
	"theta": {"theta", 'θ', clsOrd}, "vartheta": {"theta.alt", 'ϑ', clsOrd},
	"iota": {"iota", 'ι', clsOrd}, "kappa": {"kappa", 'κ', clsOrd},
	"varkappa": {"kappa.alt", 'ϰ', clsOrd}, "lambda": {"lambda", 'λ', clsOrd},
	"mu": {"mu", 'μ', clsOrd}, "nu": {"nu", 'ν', clsOrd}, "xi": {"xi", 'ξ', clsOrd},
	"omicron": {"omicron", 'ο', clsOrd}, "pi": {"pi", 'π', clsOrd},
	"varpi": {"pi.alt", 'ϖ', clsOrd}, "rho": {"rho", 'ρ', clsOrd},
	"varrho": {"rho.alt", 'ϱ', clsOrd}, "sigma": {"sigma", 'σ', clsOrd},
	"varsigma": {"sigma.alt", 'ς', clsOrd}, "tau": {"tau", 'τ', clsOrd},
	"upsilon": {"upsilon", 'υ', clsOrd}, "phi": {"phi.alt", 'ϕ', clsOrd},
	"varphi": {"phi", 'φ', clsOrd}, "chi": {"chi", 'χ', clsOrd},
	"psi": {"psi", 'ψ', clsOrd}, "omega": {"omega", 'ω', clsOrd},
	"digamma": {"digamma", 'ϝ', clsOrd},
	// Greek uppercase (including the Latin-looking ones some converters emit).
	"Alpha": {"Alpha", 'Α', clsOrd}, "Beta": {"Beta", 'Β', clsOrd},
	"Gamma": {"Gamma", 'Γ', clsOrd}, "Delta": {"Delta", 'Δ', clsOrd},
	"Epsilon": {"Epsilon", 'Ε', clsOrd}, "Zeta": {"Zeta", 'Ζ', clsOrd},
	"Eta": {"Eta", 'Η', clsOrd}, "Theta": {"Theta", 'Θ', clsOrd},
	"Iota": {"Iota", 'Ι', clsOrd}, "Kappa": {"Kappa", 'Κ', clsOrd},
	"Lambda": {"Lambda", 'Λ', clsOrd}, "Mu": {"Mu", 'Μ', clsOrd},
	"Nu": {"Nu", 'Ν', clsOrd}, "Xi": {"Xi", 'Ξ', clsOrd},
	"Omicron": {"Omicron", 'Ο', clsOrd}, "Pi": {"Pi", 'Π', clsOrd},
	"Rho": {"Rho", 'Ρ', clsOrd}, "Sigma": {"Sigma", 'Σ', clsOrd},
	"Tau": {"Tau", 'Τ', clsOrd}, "Upsilon": {"Upsilon", 'Υ', clsOrd},
	"Phi": {"Phi", 'Φ', clsOrd}, "Chi": {"Chi", 'Χ', clsOrd},
	"Psi": {"Psi", 'Ψ', clsOrd}, "Omega": {"Omega", 'Ω', clsOrd},
	"Digamma": {"Digamma", 'Ϝ', clsOrd},
	// Hebrew.
	"aleph": {"aleph", 'א', clsOrd}, "beth": {"beth", 'ב', clsOrd},
	"gimel": {"gimel", 'ג', clsOrd}, "daleth": {"daleth", 'ד', clsOrd},

	// Relations.
	"leq": {"lt.eq", '≤', clsRel}, "le": {"lt.eq", '≤', clsRel},
	"geq": {"gt.eq", '≥', clsRel}, "ge": {"gt.eq", '≥', clsRel},
	"leqq": {"lt.equiv", '≦', clsRel}, "geqq": {"gt.equiv", '≧', clsRel},
	"leqslant": {"lt.eq.slant", '⩽', clsRel}, "geqslant": {"gt.eq.slant", '⩾', clsRel},
	"neq": {"eq.not", '≠', clsRel}, "ne": {"eq.not", '≠', clsRel},
	"lt": {"lt", '<', clsRel}, "gt": {"gt", '>', clsRel},
	"approx": {"approx", '≈', clsRel}, "approxeq": {"approx.eq", '≊', clsRel},
	"thickapprox": {"approx", '≈', clsRel}, "napprox": {"approx.not", '≉', clsRel},
	"equiv": {"equiv", '≡', clsRel}, "nequiv": {"equiv.not", '≢', clsRel},
	"sim": {"tilde.op", '∼', clsRel}, "thicksim": {"tilde.op", '∼', clsRel},
	"nsim": {"tilde.not", '≁', clsRel}, "simeq": {"tilde.eq", '≃', clsRel},
	"cong": {"tilde.equiv", '≅', clsRel}, "ncong": {"tilde.equiv.not", '≇', clsRel},
	"backsim": {"tilde.rev", '∽', clsRel}, "backsimeq": {"tilde.eq.rev", '⋍', clsRel},
	"eqsim":  {"minus.tilde", '≂', clsRel},
	"propto": {"prop", '∝', clsRel}, "varpropto": {"prop", '∝', clsRel},
	"ll": {"lt.double", '≪', clsRel}, "gg": {"gt.double", '≫', clsRel},
	"lll": {"lt.triple", '⋘', clsRel}, "ggg": {"gt.triple", '⋙', clsRel},
	"llless": {"lt.triple", '⋘', clsRel}, "gggtr": {"gt.triple", '⋙', clsRel},
	"prec": {"prec", '≺', clsRel}, "succ": {"succ", '≻', clsRel},
	"preceq": {"prec.eq", '⪯', clsRel}, "succeq": {"succ.eq", '⪰', clsRel},
	"preccurlyeq": {"prec.curly.eq", '≼', clsRel}, "succcurlyeq": {"succ.curly.eq", '≽', clsRel},
	"precsim": {"prec.tilde", '≾', clsRel}, "succsim": {"succ.tilde", '≿', clsRel},
	"precapprox": {"prec.approx", '⪷', clsRel}, "succapprox": {"succ.approx", '⪸', clsRel},
	"precnapprox": {"prec.napprox", '⪹', clsRel}, "succnapprox": {"succ.napprox", '⪺', clsRel},
	"precneqq": {"prec.nequiv", '⪵', clsRel}, "succneqq": {"succ.nequiv", '⪶', clsRel},
	"precnsim": {"prec.ntilde", '⋨', clsRel}, "succnsim": {"succ.ntilde", '⋩', clsRel},
	"nprec": {"prec.not", '⊀', clsRel}, "nsucc": {"succ.not", '⊁', clsRel},
	"npreceq": {"prec.curly.eq.not", '⋠', clsRel}, "nsucceq": {"succ.curly.eq.not", '⋡', clsRel},
	"parallel": {"parallel", '∥', clsRel}, "nparallel": {"parallel.not", '∦', clsRel},
	"shortparallel": {"parallel", '∥', clsRel}, "nshortparallel": {"parallel.not", '∦', clsRel},
	"perp": {"perp", '⟂', clsRel}, "mid": {"divides", '∣', clsRel},
	"nmid": {"divides.not", '∤', clsRel}, "shortmid": {"divides", '∣', clsRel},
	"nshortmid": {"divides.not", '∤', clsRel},
	"models":    {"models", '⊧', clsRel}, "vdash": {"tack.r", '⊢', clsRel},
	"dashv": {"tack.l", '⊣', clsRel}, "vDash": {"tack.rr", '⊨', clsRel},
	"Vdash": {"forces", '⊩', clsRel}, "nvdash": {"tack.r.not", '⊬', clsRel},
	"nvDash": {"tack.rr.not", '⊭', clsRel}, "nVdash": {"forces.not", '⊮', clsRel},
	"Vvdash": {"", '⊪', clsRel}, "nVDash": {"", '⊯', clsRel},
	"asymp": {"asymp", '≍', clsRel}, "doteq": {"eq.dot", '≐', clsRel},
	"doteqdot": {"eq.dots", '≑', clsRel}, "Doteq": {"eq.dots", '≑', clsRel},
	"fallingdotseq": {"eq.dots.down", '≒', clsRel}, "risingdotseq": {"eq.dots.up", '≓', clsRel},
	"triangleq": {"eq.delta", '≜', clsRel}, "measeq": {"eq.m", '≞', clsRel},
	"questeq": {"eq.quest", '≟', clsRel}, "eqdef": {"eq.def", '≝', clsRel},
	"coloneqq": {"colon.eq", '≔', clsRel}, "coloneq": {"colon.eq", '≔', clsRel},
	"Coloneqq": {"colon.double.eq", '⩴', clsRel}, "eqqcolon": {"eq.colon", '≕', clsRel},
	"eqcolon": {"eq.colon", '≕', clsRel},
	"eqcirc":  {"", '≖', clsRel}, "circeq": {"", '≗', clsRel},
	"bumpeq": {"", '≏', clsRel}, "Bumpeq": {"", '≎', clsRel},
	"between": {"", '≬', clsRel}, "pitchfork": {"", '⋔', clsRel},
	"bowtie": {"bowtie.stroked", '⋈', clsRel}, "Join": {"bowtie.big", '⨝', clsRel},
	"smile": {"smile", '⌣', clsRel}, "frown": {"frown", '⌢', clsRel},
	"smallsmile": {"smile", '⌣', clsRel}, "smallfrown": {"frown", '⌢', clsRel},
	"lessgtr": {"lt.gt", '≶', clsRel}, "gtrless": {"gt.lt", '≷', clsRel},
	"lesseqgtr": {"lt.eq.gt", '⋚', clsRel}, "gtreqless": {"gt.eq.lt", '⋛', clsRel},
	"lesseqqgtr": {"", '⪋', clsRel}, "gtreqqless": {"", '⪌', clsRel},
	"lesssim": {"lt.tilde", '≲', clsRel}, "gtrsim": {"gt.tilde", '≳', clsRel},
	"lessapprox": {"lt.approx", '⪅', clsRel}, "gtrapprox": {"gt.approx", '⪆', clsRel},
	"lnapprox": {"lt.napprox", '⪉', clsRel}, "gnapprox": {"gt.napprox", '⪊', clsRel},
	"lneq": {"lt.neq", '⪇', clsRel}, "gneq": {"gt.neq", '⪈', clsRel},
	"lneqq": {"lt.nequiv", '≨', clsRel}, "gneqq": {"gt.nequiv", '≩', clsRel},
	"lvertneqq": {"lt.nequiv", '≨', clsRel}, "gvertneqq": {"gt.nequiv", '≩', clsRel},
	"lnsim": {"lt.ntilde", '⋦', clsRel}, "gnsim": {"gt.ntilde", '⋧', clsRel},
	"nless": {"lt.not", '≮', clsRel}, "ngtr": {"gt.not", '≯', clsRel},
	"nleq": {"lt.eq.not", '≰', clsRel}, "ngeq": {"gt.eq.not", '≱', clsRel},
	"nleqslant": {"lt.eq.not", '≰', clsRel}, "ngeqslant": {"gt.eq.not", '≱', clsRel},
	"lessdot": {"lt.dot", '⋖', clsRel}, "gtrdot": {"gt.dot", '⋗', clsRel},
	"eqslantless": {"", '⪕', clsRel}, "eqslantgtr": {"", '⪖', clsRel},
	"vartriangleleft": {"lt.closed", '⊲', clsRel}, "vartriangleright": {"gt.closed", '⊳', clsRel},
	"lhd": {"lt.closed", '⊲', clsBin}, "rhd": {"gt.closed", '⊳', clsBin},
	"unlhd": {"lt.closed.eq", '⊴', clsBin}, "unrhd": {"gt.closed.eq", '⊵', clsBin},
	"trianglelefteq": {"lt.closed.eq", '⊴', clsRel}, "trianglerighteq": {"gt.closed.eq", '⊵', clsRel},
	"ntriangleleft": {"lt.closed.not", '⋪', clsRel}, "ntriangleright": {"gt.closed.not", '⋫', clsRel},
	"ntrianglelefteq": {"lt.closed.eq.not", '⋬', clsRel}, "ntrianglerighteq": {"gt.closed.eq.not", '⋭', clsRel},
	"therefore": {"therefore", '∴', clsRel}, "because": {"because", '∵', clsRel},
	// Sets.
	"in": {"in", '∈', clsRel}, "notin": {"in.not", '∉', clsRel},
	"ni": {"in.rev", '∋', clsRel}, "owns": {"in.rev", '∋', clsRel},
	"notni":  {"in.rev.not", '∌', clsRel},
	"subset": {"subset", '⊂', clsRel}, "supset": {"supset", '⊃', clsRel},
	"subseteq": {"subset.eq", '⊆', clsRel}, "supseteq": {"supset.eq", '⊇', clsRel},
	"subsetneq": {"subset.neq", '⊊', clsRel}, "supsetneq": {"supset.neq", '⊋', clsRel},
	"varsubsetneq": {"subset.neq", '⊊', clsRel}, "varsupsetneq": {"supset.neq", '⊋', clsRel},
	"nsubset": {"subset.not", '⊄', clsRel}, "nsupset": {"supset.not", '⊅', clsRel},
	"nsubseteq": {"subset.eq.not", '⊈', clsRel}, "nsupseteq": {"supset.eq.not", '⊉', clsRel},
	"subseteqq": {"subset.equiv", '⫅', clsRel}, "supseteqq": {"supset.equiv", '⫆', clsRel},
	"subsetneqq": {"subset.nequiv", '⫋', clsRel}, "supsetneqq": {"supset.nequiv", '⫌', clsRel},
	"sqsubset": {"subset.sq", '⊏', clsRel}, "sqsupset": {"supset.sq", '⊐', clsRel},
	"sqsubseteq": {"subset.eq.sq", '⊑', clsRel}, "sqsupseteq": {"supset.eq.sq", '⊒', clsRel},
	"nsqsubseteq": {"subset.eq.sq.not", '⋢', clsRel}, "nsqsupseteq": {"supset.eq.sq.not", '⋣', clsRel},
	"Subset": {"subset.double", '⋐', clsRel}, "Supset": {"supset.double", '⋑', clsRel},

	// Binary operators.
	"pm": {"plus.minus", '±', clsBin}, "mp": {"minus.plus", '∓', clsBin},
	"times": {"times", '×', clsBin}, "div": {"div", '÷', clsBin},
	"cdot": {"dot.op", '⋅', clsBin}, "centerdot": {"dot.op", '⋅', clsBin},
	"circ": {"compose", '∘', clsBin}, "bullet": {"bullet.op", '∙', clsBin},
	"star": {"star.op", '⋆', clsBin}, "ast": {"ast.op", '∗', clsBin},
	"oplus": {"plus.o", '⊕', clsBin}, "ominus": {"minus.o", '⊖', clsBin},
	"otimes": {"times.o", '⊗', clsBin}, "oslash": {"slash.o", '⊘', clsBin},
	"odot": {"dot.o", '⊙', clsBin}, "circledast": {"ast.op.o", '⊛', clsBin},
	"circledcirc": {"compose.o", '⊚', clsBin}, "circleddash": {"", '⊝', clsBin},
	"boxplus": {"plus.square", '⊞', clsBin}, "boxminus": {"minus.square", '⊟', clsBin},
	"boxtimes": {"times.square", '⊠', clsBin}, "boxdot": {"dot.square", '⊡', clsBin},
	"wedge": {"and", '∧', clsBin}, "land": {"and", '∧', clsBin},
	"vee": {"or", '∨', clsBin}, "lor": {"or", '∨', clsBin},
	"cap": {"inter", '∩', clsBin}, "cup": {"union", '∪', clsBin},
	"setminus": {"without", '∖', clsBin}, "smallsetminus": {"without", '∖', clsBin},
	"sqcap": {"inter.sq", '⊓', clsBin}, "sqcup": {"union.sq", '⊔', clsBin},
	"uplus": {"union.plus", '⊎', clsBin}, "amalg": {"", '⨿', clsBin},
	"dagger": {"dagger", '†', clsBin}, "ddagger": {"dagger.double", '‡', clsBin},
	"wr": {"wreath", '≀', clsBin}, "diamond": {"diamond.stroked.small", '⋄', clsBin},
	"bigtriangleup":   {"triangle.stroked.t", '△', clsBin},
	"bigtriangledown": {"triangle.stroked.b", '▽', clsBin},
	"triangleleft":    {"triangle.stroked.l", '◁', clsBin},
	"triangleright":   {"triangle.stroked.r", '▷', clsBin},
	"bigcirc":         {"circle.stroked.big", '◯', clsBin},
	"intercal":        {"", '⊺', clsBin}, "barwedge": {"", '⊼', clsBin},
	"veebar": {"", '⊻', clsBin}, "doublebarwedge": {"", '⩞', clsBin},
	"curlywedge": {"and.curly", '⋏', clsBin}, "curlyvee": {"or.curly", '⋎', clsBin},
	"ltimes": {"times.l", '⋉', clsBin}, "rtimes": {"times.r", '⋊', clsBin},
	"leftthreetimes":  {"times.three.l", '⋋', clsBin},
	"rightthreetimes": {"times.three.r", '⋌', clsBin},
	"divideontimes":   {"times.div", '⋇', clsBin}, "dotplus": {"plus.dot", '∔', clsBin},
	"Cap": {"inter.double", '⋒', clsBin}, "Cup": {"union.double", '⋓', clsBin},
	"doublecap": {"inter.double", '⋒', clsBin}, "doublecup": {"union.double", '⋓', clsBin},

	// Arrows (relations).
	"to": {"arrow.r", '→', clsRel}, "rightarrow": {"arrow.r", '→', clsRel},
	"gets": {"arrow.l", '←', clsRel}, "leftarrow": {"arrow.l", '←', clsRel},
	"leftrightarrow": {"arrow.l.r", '↔', clsRel},
	"Rightarrow":     {"arrow.r.double", '⇒', clsRel}, "Leftarrow": {"arrow.l.double", '⇐', clsRel},
	"Leftrightarrow": {"arrow.l.r.double", '⇔', clsRel},
	"longrightarrow": {"arrow.r.long", '⟶', clsRel}, "longleftarrow": {"arrow.l.long", '⟵', clsRel},
	"longleftrightarrow": {"arrow.l.r.long", '⟷', clsRel},
	"Longrightarrow":     {"arrow.r.double.long", '⟹', clsRel},
	"Longleftarrow":      {"arrow.l.double.long", '⟸', clsRel},
	"Longleftrightarrow": {"arrow.l.r.double.long", '⟺', clsRel},
	"mapsto":             {"arrow.r.bar", '↦', clsRel}, "longmapsto": {"arrow.r.long.bar", '⟼', clsRel},
	"mapsfrom": {"arrow.l.bar", '↤', clsRel}, "longmapsfrom": {"arrow.l.long.bar", '⟻', clsRel},
	"uparrow": {"arrow.t", '↑', clsRel}, "downarrow": {"arrow.b", '↓', clsRel},
	"updownarrow": {"arrow.t.b", '↕', clsRel},
	"Uparrow":     {"arrow.t.double", '⇑', clsRel}, "Downarrow": {"arrow.b.double", '⇓', clsRel},
	"Updownarrow": {"arrow.t.b.double", '⇕', clsRel},
	"nearrow":     {"arrow.tr", '↗', clsRel}, "searrow": {"arrow.br", '↘', clsRel},
	"swarrow": {"arrow.bl", '↙', clsRel}, "nwarrow": {"arrow.tl", '↖', clsRel},
	"hookrightarrow": {"arrow.r.hook", '↪', clsRel}, "hookleftarrow": {"arrow.l.hook", '↩', clsRel},
	"rightharpoonup": {"harpoon.rt", '⇀', clsRel}, "rightharpoondown": {"harpoon.rb", '⇁', clsRel},
	"leftharpoonup": {"harpoon.lt", '↼', clsRel}, "leftharpoondown": {"harpoon.lb", '↽', clsRel},
	"upharpoonright": {"harpoon.tr", '↾', clsRel}, "upharpoonleft": {"harpoon.tl", '↿', clsRel},
	"downharpoonright": {"harpoon.br", '⇂', clsRel}, "downharpoonleft": {"harpoon.bl", '⇃', clsRel},
	"restriction":       {"harpoon.tr", '↾', clsRel},
	"rightleftharpoons": {"harpoons.rtlb", '⇌', clsRel},
	"leftrightharpoons": {"harpoons.ltrb", '⇋', clsRel},
	"rightrightarrows":  {"arrows.rr", '⇉', clsRel}, "leftleftarrows": {"arrows.ll", '⇇', clsRel},
	"rightleftarrows": {"arrows.rl", '⇄', clsRel}, "leftrightarrows": {"arrows.lr", '⇆', clsRel},
	"upuparrows": {"arrows.tt", '⇈', clsRel}, "downdownarrows": {"arrows.bb", '⇊', clsRel},
	"twoheadrightarrow": {"arrow.r.twohead", '↠', clsRel},
	"twoheadleftarrow":  {"arrow.l.twohead", '↞', clsRel},
	"rightarrowtail":    {"arrow.r.tail", '↣', clsRel}, "leftarrowtail": {"arrow.l.tail", '↢', clsRel},
	"looparrowright": {"arrow.r.loop", '↬', clsRel}, "looparrowleft": {"arrow.l.loop", '↫', clsRel},
	"curvearrowright": {"arrow.cw.half", '↷', clsRel}, "curvearrowleft": {"arrow.ccw.half", '↶', clsRel},
	"circlearrowright": {"arrow.cw", '↻', clsRel}, "circlearrowleft": {"arrow.ccw", '↺', clsRel},
	"rightsquigarrow": {"arrow.r.squiggly", '⇝', clsRel}, "leadsto": {"arrow.r.squiggly", '⇝', clsRel},
	"leftrightsquigarrow": {"arrow.l.r.wave", '↭', clsRel},
	"nrightarrow":         {"arrow.r.not", '↛', clsRel}, "nleftarrow": {"arrow.l.not", '↚', clsRel},
	"nRightarrow": {"arrow.r.double.not", '⇏', clsRel}, "nLeftarrow": {"arrow.l.double.not", '⇍', clsRel},
	"nleftrightarrow": {"arrow.l.r.not", '↮', clsRel},
	"nLeftrightarrow": {"arrow.l.r.double.not", '⇎', clsRel},
	"Rrightarrow":     {"arrow.r.triple", '⇛', clsRel}, "Lleftarrow": {"arrow.l.triple", '⇚', clsRel},
	"dashrightarrow": {"arrow.r.dashed", '⇢', clsRel}, "dashleftarrow": {"arrow.l.dashed", '⇠', clsRel},
	"Lsh": {"", '↰', clsRel}, "Rsh": {"", '↱', clsRel},
	"multimap": {"multimap", '⊸', clsRel},

	// Miscellaneous ordinary symbols.
	"infty": {"infinity", '∞', clsOrd}, "partial": {"partial", '∂', clsOrd},
	"nabla": {"nabla", '∇', clsOrd}, "hbar": {"planck", 'ħ', clsOrd},
	"hslash": {"planck", 'ħ', clsOrd}, "ell": {"ell", 'ℓ', clsOrd},
	"wp": {"pee", '℘', clsOrd}, "Re": {"Re", 'ℜ', clsOrd}, "Im": {"Im", 'ℑ', clsOrd},
	"mho": {"Omega.inv", '℧', clsOrd}, "eth": {"", 'ð', clsOrd},
	"Finv": {"", 'Ⅎ', clsOrd}, "Game": {"", '⅁', clsOrd}, "Bbbk": {"", '𝕜', clsOrd},
	"complement": {"complement", '∁', clsOrd},
	"imath":      {"dotless.i", 'ı', clsOrd}, "jmath": {"dotless.j", 'ȷ', clsOrd},
	"angle": {"angle", '∠', clsOrd}, "measuredangle": {"angle.arc", '∡', clsOrd},
	"sphericalangle": {"angle.spheric", '∢', clsOrd},
	"top":            {"top", '⊤', clsOrd}, "bot": {"bot", '⊥', clsOrd},
	"forall": {"forall", '∀', clsOrd}, "exists": {"exists", '∃', clsOrd},
	"nexists": {"exists.not", '∄', clsOrd},
	"neg":     {"not", '¬', clsOrd}, "lnot": {"not", '¬', clsOrd},
	"emptyset": {"emptyset.zero", '∅', clsOrd}, "varnothing": {"emptyset", '∅', clsOrd},
	"surd": {"", '√', clsOrd}, "prime": {"prime", '′', clsOrd},
	"backprime": {"prime.rev", '‵', clsOrd}, "degree": {"degree", '°', clsOrd},
	"checkmark": {"checkmark", '✓', clsOrd}, "maltese": {"maltese", '✠', clsOrd},
	"clubsuit":    {"suit.club.filled", '♣', clsOrd},
	"diamondsuit": {"suit.diamond.stroked", '♢', clsOrd},
	"heartsuit":   {"suit.heart.stroked", '♡', clsOrd},
	"spadesuit":   {"suit.spade.filled", '♠', clsOrd},
	"flat":        {"flat", '♭', clsOrd}, "natural": {"natural", '♮', clsOrd},
	"sharp": {"sharp", '♯', clsOrd},
	"dag":   {"dagger", '†', clsOrd}, "ddag": {"dagger.double", '‡', clsOrd},
	"S": {"section", '§', clsOrd}, "P": {"pilcrow", '¶', clsOrd},
	"copyright": {"copyright", '©', clsOrd}, "pounds": {"pound", '£', clsOrd},
	"yen": {"yen", '¥', clsOrd}, "euro": {"euro", '€', clsOrd},
	"circledR": {"", '®', clsOrd}, "circledS": {"", 'Ⓢ', clsOrd},
	"diagup": {"", '╱', clsOrd}, "diagdown": {"", '╲', clsOrd},
	"square": {"square.stroked", '□', clsOrd}, "Box": {"square.stroked", '□', clsOrd},
	"blacksquare": {"square.filled", '■', clsOrd}, "qed": {"qed", '∎', clsOrd},
	"Diamond": {"diamond.stroked", '◇', clsOrd},
	"lozenge": {"lozenge.stroked", '◊', clsOrd}, "blacklozenge": {"lozenge.filled", '⧫', clsOrd},
	"triangle":           {"triangle.stroked.t", '△', clsOrd},
	"triangledown":       {"triangle.stroked.small.b", '▿', clsOrd},
	"blacktriangle":      {"triangle.filled.small.t", '▴', clsOrd},
	"blacktriangledown":  {"triangle.filled.small.b", '▾', clsOrd},
	"blacktriangleleft":  {"triangle.filled.small.l", '◂', clsRel},
	"blacktriangleright": {"triangle.filled.small.r", '▸', clsRel},
	"bigstar":            {"star.filled", '★', clsOrd},
	"vdots":              {"dots.v", '⋮', clsOrd}, "ddots": {"dots.down", '⋱', clsOrd},
	"iddots": {"dots.up", '⋰', clsOrd}, "adots": {"dots.up", '⋰', clsOrd},
	"ldots": {"dots.h", '…', clsOrd}, "cdots": {"dots.h.c", '⋯', clsOrd},
	"dotsb": {"dots.h.c", '⋯', clsOrd}, "dotsm": {"dots.h.c", '⋯', clsOrd},
	"dotsi": {"dots.h.c", '⋯', clsOrd}, "dotsc": {"dots.h", '…', clsOrd},
	"dotso": {"dots.h", '…', clsOrd}, "hdots": {"dots.h", '…', clsOrd},
	"mathellipsis": {"dots.h", '…', clsOrd},
	"cdotp":        {"dot.c", '·', clsPunct},

	// Delimiters used outside \left … \right.
	"langle": {"chevron.l", '⟨', clsOpen}, "rangle": {"chevron.r", '⟩', clsClose},
	"lAngle": {"chevron.l.double", '⟪', clsOpen}, "rAngle": {"chevron.r.double", '⟫', clsClose},
	"lfloor": {"floor.l", '⌊', clsOpen}, "rfloor": {"floor.r", '⌋', clsClose},
	"lceil": {"ceil.l", '⌈', clsOpen}, "rceil": {"ceil.r", '⌉', clsClose},
	"lbrace": {"brace.l", '{', clsOpen}, "rbrace": {"brace.r", '}', clsClose},
	"{": {"brace.l", '{', clsOpen}, "}": {"brace.r", '}', clsClose},
	"lbrack": {"bracket.l", '[', clsOpen}, "rbrack": {"bracket.r", ']', clsClose},
	"lgroup": {"paren.l.flat", '⟮', clsOpen}, "rgroup": {"paren.r.flat", '⟯', clsClose},
	"llbracket": {"bracket.l.stroked", '⟦', clsOpen}, "rrbracket": {"bracket.r.stroked", '⟧', clsClose},
	"lmoustache": {"", '⎰', clsOpen}, "rmoustache": {"", '⎱', clsClose},
	"ulcorner": {"corner.l.t", '⌜', clsOpen}, "urcorner": {"corner.r.t", '⌝', clsClose},
	"llcorner": {"corner.l.b", '⌞', clsOpen}, "lrcorner": {"corner.r.b", '⌟', clsClose},
	"vert": {"bar.v", '|', clsOrd}, "Vert": {"bar.v.double", '‖', clsOrd},
	"|":     {"bar.v.double", '‖', clsOrd},
	"lvert": {"bar.v", '|', clsOpen}, "rvert": {"bar.v", '|', clsClose},
	"lVert": {"bar.v.double", '‖', clsOpen}, "rVert": {"bar.v.double", '‖', clsClose},
	"backslash": {"backslash", '\\', clsOrd},

	// Wikipedia (texvc) and MathJax aliases.
	"R": {"RR", 'ℝ', clsOrd}, "Reals": {"RR", 'ℝ', clsOrd}, "reals": {"RR", 'ℝ', clsOrd},
	"N": {"NN", 'ℕ', clsOrd}, "natnums": {"NN", 'ℕ', clsOrd},
	"Z": {"ZZ", 'ℤ', clsOrd}, "Q": {"QQ", 'ℚ', clsOrd},
	"C": {"CC", 'ℂ', clsOrd}, "Complex": {"CC", 'ℂ', clsOrd}, "cnums": {"CC", 'ℂ', clsOrd},
	"alef": {"aleph", 'א', clsOrd}, "alefsym": {"aleph", 'א', clsOrd},
	"and": {"and", '∧', clsBin}, "or": {"or", '∨', clsBin},
	"ang": {"angle", '∠', clsOrd}, "bull": {"bullet.op", '∙', clsBin},
	"clubs": {"suit.club.filled", '♣', clsOrd}, "diamonds": {"suit.diamond.stroked", '♢', clsOrd},
	"hearts": {"suit.heart.stroked", '♡', clsOrd}, "spades": {"suit.spade.filled", '♠', clsOrd},
	"Dagger": {"dagger.double", '‡', clsBin}, "empty": {"emptyset.zero", '∅', clsOrd},
	"O": {"emptyset.zero", '∅', clsOrd}, "exist": {"exists", '∃', clsOrd},
	"harr": {"arrow.l.r", '↔', clsRel}, "hArr": {"arrow.l.r.double", '⇔', clsRel},
	"Harr": {"arrow.l.r.double", '⇔', clsRel}, "lrarr": {"arrow.l.r", '↔', clsRel},
	"lrArr": {"arrow.l.r.double", '⇔', clsRel}, "Lrarr": {"arrow.l.r.double", '⇔', clsRel},
	"larr": {"arrow.l", '←', clsRel}, "lArr": {"arrow.l.double", '⇐', clsRel},
	"Larr": {"arrow.l.double", '⇐', clsRel}, "rarr": {"arrow.r", '→', clsRel},
	"rArr": {"arrow.r.double", '⇒', clsRel}, "Rarr": {"arrow.r.double", '⇒', clsRel},
	"uarr": {"arrow.t", '↑', clsRel}, "uArr": {"arrow.t.double", '⇑', clsRel},
	"Uarr": {"arrow.t.double", '⇑', clsRel}, "darr": {"arrow.b", '↓', clsRel},
	"dArr": {"arrow.b.double", '⇓', clsRel}, "Darr": {"arrow.b.double", '⇓', clsRel},
	"image": {"Im", 'ℑ', clsOrd}, "real": {"Re", 'ℜ', clsOrd},
	"infin": {"infinity", '∞', clsOrd}, "isin": {"in", '∈', clsRel},
	"part": {"partial", '∂', clsOrd}, "plusmn": {"plus.minus", '±', clsBin},
	"sdot": {"dot.op", '⋅', clsBin}, "sect": {"section", '§', clsOrd},
	"sub": {"subset", '⊂', clsRel}, "sube": {"subset.eq", '⊆', clsRel},
	"supe": {"supset.eq", '⊇', clsRel}, "thetasym": {"theta.alt", 'ϑ', clsOrd},
	"weierp": {"pee", '℘', clsOrd},
}

// bigOps are large operators; their scripts may become limits.
var bigOps = map[string]sym{
	"sum": {"sum", '∑', clsOp}, "prod": {"product", '∏', clsOp},
	"coprod": {"product.co", '∐', clsOp},
	"int":    {"integral", '∫', clsOp}, "intop": {"integral", '∫', clsOp},
	"smallint": {"integral", '∫', clsOp},
	"iint":     {"integral.double", '∬', clsOp}, "iiint": {"integral.triple", '∭', clsOp},
	"iiiint": {"integral.quad", '⨌', clsOp}, "oint": {"integral.cont", '∮', clsOp},
	"oiint": {"integral.surf", '∯', clsOp}, "oiiint": {"integral.vol", '∰', clsOp},
	"varointclockwise": {"integral.cont.cw", '∲', clsOp},
	"ointctrclockwise": {"integral.cont.ccw", '∳', clsOp},
	"fint":             {"integral.slash", '⨏', clsOp},
	"bigcup":           {"union.big", '⋃', clsOp}, "bigcap": {"inter.big", '⋂', clsOp},
	"bigoplus": {"plus.o.big", '⨁', clsOp}, "bigotimes": {"times.o.big", '⨂', clsOp},
	"bigodot": {"dot.o.big", '⨀', clsOp}, "bigvee": {"or.big", '⋁', clsOp},
	"bigwedge": {"and.big", '⋀', clsOp}, "biguplus": {"union.plus.big", '⨄', clsOp},
	"bigsqcup": {"union.sq.big", '⨆', clsOp}, "bigsqcap": {"inter.sq.big", '⨅', clsOp},
}

// functions are operator names Typst provides under the same name.
var functions = map[string]bool{
	"sin": true, "cos": true, "tan": true, "cot": true, "sec": true, "csc": true,
	"arcsin": true, "arccos": true, "arctan": true, "sinh": true, "cosh": true,
	"tanh": true, "coth": true, "sech": true, "csch": true, "log": true, "ln": true,
	"lg": true, "exp": true, "det": true, "dim": true, "ker": true, "hom": true,
	"arg": true, "deg": true, "gcd": true, "lcm": true, "lim": true, "liminf": true,
	"limsup": true, "max": true, "min": true, "sup": true, "inf": true, "Pr": true,
	"tr": true,
}

// extraFunctions are operator names Typst lacks; they become op("…").
// The value tells whether scripts go below/above in display style.
var extraFunctions = map[string]bool{
	"arccot": false, "arcsec": false, "arccsc": false, "sgn": false,
	"injlim": true, "projlim": true, "varinjlim": true, "varprojlim": true,
}

// accents maps accent commands to the Typst accent symbol.
var accents = map[string]string{
	"hat": "hat", "widehat": "hat", "check": "caron", "widecheck": "caron",
	"tilde": "tilde", "widetilde": "tilde", "acute": "acute", "grave": "grave",
	"dot": "dot", "ddot": "dot.double", "dddot": "dot.triple", "ddddot": "dot.quad",
	"breve": "breve", "bar": "macron", "widebar": "macron", "vec": "arrow",
	"overrightarrow": "arrow", "overleftarrow": "arrow.l",
	"overleftrightarrow": "arrow.l.r", "mathring": "circle",
}

// delimiters maps the arguments of \left, \right, \middle and \big… to
// Typst symbols. "" means the null delimiter ".".
var delimChars = map[rune]string{
	'(': "paren.l", ')': "paren.r", '[': "bracket.l", ']': "bracket.r",
	'|': "bar.v", '.': "", '/': "slash", '<': "chevron.l", '>': "chevron.r",
	'⟨': "chevron.l", '⟩': "chevron.r", '‖': "bar.v.double",
	'⌊': "floor.l", '⌋': "floor.r", '⌈': "ceil.l", '⌉': "ceil.r",
}

var delimCmds = map[string]string{
	"{": "brace.l", "}": "brace.r", "lbrace": "brace.l", "rbrace": "brace.r",
	"lbrack": "bracket.l", "rbrack": "bracket.r",
	"langle": "chevron.l", "rangle": "chevron.r",
	"lAngle": "chevron.l.double", "rAngle": "chevron.r.double",
	"lfloor": "floor.l", "rfloor": "floor.r", "lceil": "ceil.l", "rceil": "ceil.r",
	"vert": "bar.v", "lvert": "bar.v", "rvert": "bar.v",
	"|": "bar.v.double", "Vert": "bar.v.double", "lVert": "bar.v.double", "rVert": "bar.v.double",
	"backslash": "backslash", "uparrow": "arrow.t", "downarrow": "arrow.b",
	"updownarrow": "arrow.t.b", "Uparrow": "arrow.t.double", "Downarrow": "arrow.b.double",
	"Updownarrow": "arrow.t.b.double", "lgroup": "paren.l.flat", "rgroup": "paren.r.flat",
	"llbracket": "bracket.l.stroked", "rrbracket": "bracket.r.stroked",
	"ulcorner": "corner.l.t", "urcorner": "corner.r.t",
	"llcorner": "corner.l.b", "lrcorner": "corner.r.b",
	"lt": "chevron.l", "gt": "chevron.r",
}

// negations maps the symbol following \not to its negated Typst symbol.
var negations = map[string]string{
	"=": "eq.not", "<": "lt.not", ">": "gt.not",
	"in": "in.not", "ni": "in.rev.not", "owns": "in.rev.not",
	"equiv": "equiv.not", "sim": "tilde.not", "simeq": "tilde.eq.not",
	"cong": "tilde.equiv.not", "approx": "approx.not", "asymp": "asymp.not",
	"leq": "lt.eq.not", "le": "lt.eq.not", "geq": "gt.eq.not", "ge": "gt.eq.not",
	"subset": "subset.not", "supset": "supset.not",
	"subseteq": "subset.eq.not", "supseteq": "supset.eq.not",
	"sqsubseteq": "subset.eq.sq.not", "sqsupseteq": "supset.eq.sq.not",
	"mid": "divides.not", "parallel": "parallel.not",
	"prec": "prec.not", "succ": "succ.not",
	"preccurlyeq": "prec.curly.eq.not", "succcurlyeq": "succ.curly.eq.not",
	"exists": "exists.not", "vdash": "tack.r.not", "vDash": "tack.rr.not",
	"Vdash": "forces.not", "lessgtr": "lt.gt.not", "gtrless": "gt.lt.not",
	"lesssim": "lt.tilde.not", "gtrsim": "gt.tilde.not",
	"to": "arrow.r.not", "rightarrow": "arrow.r.not", "leftarrow": "arrow.l.not",
	"gets": "arrow.l.not", "leftrightarrow": "arrow.l.r.not",
	"Rightarrow": "arrow.r.double.not", "Leftarrow": "arrow.l.double.not",
	"Leftrightarrow":  "arrow.l.r.double.not",
	"vartriangleleft": "lt.closed.not", "vartriangleright": "gt.closed.not",
	"trianglelefteq": "lt.closed.eq.not", "trianglerighteq": "gt.closed.eq.not",
}

// fontCmds maps math alphabet commands to Typst variant functions. A second
// function wraps the first when LaTeX implies an upright shape.
type fontSpec struct {
	fn      string // outer function
	inner   string // optional inner function (upright for \mathbf, \mathsf)
	words   bool   // letter runs read as words (rendered as one string)
	doubled bool   // \mathbb: single capitals use the RR-style symbols
}

var fontCmds = map[string]fontSpec{
	"mathrm": {fn: "upright", words: true}, "mathup": {fn: "upright", words: true},
	"mathnormal": {}, "mathit": {fn: "italic", words: true},
	"mathbf":   {fn: "bold", inner: "upright", words: true},
	"mathbfup": {fn: "bold", inner: "upright", words: true},
	"mathbfit": {fn: "bold", inner: "italic"},
	"mathsf":   {fn: "sans", inner: "upright", words: true},
	"mathsfup": {fn: "sans", inner: "upright", words: true},
	"mathsfit": {fn: "sans", inner: "italic"},
	"mathtt":   {fn: "mono", words: true},
	"mathcal":  {fn: "cal"}, "mathscr": {fn: "scr"}, "mathfrak": {fn: "frak"},
	"mathbb": {fn: "bb", doubled: true}, "Bbb": {fn: "bb", doubled: true},
	"mathbbm": {fn: "bb", doubled: true}, "frak": {fn: "frak"},
	"boldsymbol": {fn: "bold"}, "bm": {fn: "bold"}, "pmb": {fn: "bold"},
	"symbf": {fn: "bold", inner: "upright", words: true}, "symrm": {fn: "upright", words: true},
	"symit": {fn: "italic"}, "symbfit": {fn: "bold", inner: "italic"},
	"symsf": {fn: "sans", inner: "upright"}, "symtt": {fn: "mono"},
	"symcal": {fn: "cal"}, "symscr": {fn: "scr"}, "symfrak": {fn: "frak"},
	"symbb": {fn: "bb", doubled: true},
}

// fontSwitches are the old-style declarations that apply to the rest of the
// current group, e.g. {\rm d}x.
var fontSwitches = map[string]string{
	"rm": "mathrm", "bf": "mathbf", "it": "mathit", "sf": "mathsf", "tt": "mathtt",
	"cal": "mathcal", "mit": "mathnormal", "bold": "mathbf", "boldmath": "boldsymbol",
}

// styleCmds map TeX style declarations to Typst size functions.
var styleCmds = map[string]string{
	"displaystyle": "display", "textstyle": "inline",
	"scriptstyle": "script", "scriptscriptstyle": "sscript",
}

// spaces maps spacing commands to Typst code.
var spaces = map[string]string{
	",": "thin", "thinspace": "thin", ":": "med", ">": "med", "medspace": "med",
	";": "thick", "thickspace": "thick", "quad": "quad", "qquad": "wide",
	" ": "space", "enspace": "#h(0.5em)", "enskip": "#h(0.5em)",
	"!": "#h(-0.1667em)", "negthinspace": "#h(-0.1667em)",
	"negmedspace": "#h(-0.2222em)", "negthickspace": "#h(-0.2778em)",
}

// xArrows are the extensible arrows of amsmath and mathtools.
var xArrows = map[string]string{
	"xrightarrow": "arrow.r", "xleftarrow": "arrow.l", "xleftrightarrow": "arrow.l.r",
	"xRightarrow": "arrow.r.double", "xLeftarrow": "arrow.l.double",
	"xLeftrightarrow": "arrow.l.r.double", "xmapsto": "arrow.r.bar",
	"xhookrightarrow": "arrow.r.hook", "xhookleftarrow": "arrow.l.hook",
	"xrightharpoonup": "harpoon.rt", "xrightharpoondown": "harpoon.rb",
	"xleftharpoonup": "harpoon.lt", "xleftharpoondown": "harpoon.lb",
	"xrightleftharpoons": "harpoons.rtlb", "xleftrightharpoons": "harpoons.ltrb",
	"xlongequal": "eq", "xtwoheadrightarrow": "arrow.r.twohead",
	"xtwoheadleftarrow": "arrow.l.twohead",
}

// bigDelims are \big-style delimiter size commands: height in em and the
// math class implied by the l/r/m suffix.
type bigSpec struct {
	size string
	cls  string // Typst class wrapper, "" for none
}

var bigDelims = map[string]bigSpec{
	"big": {"1.2em", "normal"}, "Big": {"1.8em", "normal"},
	"bigg": {"2.4em", "normal"}, "Bigg": {"3em", "normal"},
	"bigl": {"1.2em", "opening"}, "Bigl": {"1.8em", "opening"},
	"biggl": {"2.4em", "opening"}, "Biggl": {"3em", "opening"},
	"bigr": {"1.2em", "closing"}, "Bigr": {"1.8em", "closing"},
	"biggr": {"2.4em", "closing"}, "Biggr": {"3em", "closing"},
	"bigm": {"1.2em", "relation"}, "Bigm": {"1.8em", "relation"},
	"biggm": {"2.4em", "relation"}, "Biggm": {"3em", "relation"},
}

// mathClasses maps \mathbin and friends to Typst math classes.
var mathClasses = map[string]string{
	"mathbin": "binary", "mathrel": "relation", "mathord": "normal",
	"mathopen": "opening", "mathclose": "closing", "mathpunct": "punctuation",
	"mathinner": "normal",
}

// ignoredCmds have no visible effect in a converted formula.
var ignoredCmds = map[string]bool{
	"relax": true, "allowbreak": true, "nobreak": true, "displaybreak": true,
	"protect": true, "leavevmode": true, "nonscript": true, "limits": true,
	"nolimits": true, "displaylimits": true, "/": true, "qedhere": true,
	"hfill": true, "hfil": true, "centering": true, "noindent": true, "par": true,
	"hline": true, "hdashline": true, "toprule": true, "midrule": true,
	"bottomrule": true, "smallskip": true, "medskip": true, "bigskip": true,
	"tiny": true, "scriptsize": true, "footnotesize": true, "small": true,
	"normalsize": true, "large": true, "Large": true, "LARGE": true, "huge": true,
	"Huge": true,
}

// textSymbols are the text-mode control sequences understood inside \text.
var textSymbols = map[string]string{
	"%": "%", "&": "&", "$": "$", "#": "#", "_": "_", "{": "{", "}": "}",
	" ": " ", ",": " ", ";": " ", ":": " ", "!": "", "/": "",
	"-": "", "textbackslash": "\\", "backslash": "\\", "textasciitilde": "~",
	"textasciicircum": "^", "textbar": "|", "textless": "<", "textgreater": ">",
	"ldots": "…", "dots": "…", "textellipsis": "…", "textendash": "–",
	"textemdash": "—", "textquoteleft": "‘", "textquoteright": "’",
	"textquotedblleft": "“", "textquotedblright": "”", "textdegree": "°",
	"S": "§", "P": "¶", "copyright": "©", "textcopyright": "©",
	"textregistered": "®", "texttrademark": "™", "pounds": "£", "euro": "€",
	"quad": " ", "qquad": "  ", "enspace": " ",
	"thinspace": " ", "i": "ı", "j": "ȷ", "o": "ø", "O": "Ø", "ae": "æ",
	"AE": "Æ", "oe": "œ", "OE": "Œ", "aa": "å", "AA": "Å", "ss": "ß", "l": "ł",
	"L": "Ł", "TeX": "TeX", "LaTeX": "LaTeX", "textdagger": "†", "dag": "†",
	"textbullet": "•", "textperiodcentered": "·", "textunderscore": "_",
	"textdollar": "$", "textpercent": "%", "textampersand": "&", "textsection": "§",
}

// textAccents maps text-mode accent commands to combining characters.
var textAccents = map[string]rune{
	"'": '́', "`": '̀', "^": '̂', "\"": '̈', "~": '̃',
	"=": '̄', ".": '̇', "u": '̆', "v": '̌', "H": '̋',
	"c": '̧', "d": '̣', "b": '̱', "r": '̊', "k": '̨',
}

// namedColors are the xcolor base colours and common dvipsnames, as RGB hex.
var namedColors = map[string]string{
	"black": "000000", "white": "FFFFFF", "red": "FF0000", "green": "00FF00",
	"blue": "0000FF", "cyan": "00FFFF", "magenta": "FF00FF", "yellow": "FFFF00",
	"gray": "808080", "grey": "808080", "darkgray": "404040", "lightgray": "BFBFBF",
	"brown": "BF8040", "lime": "BFFF00", "olive": "808000", "orange": "FF8000",
	"pink": "FFBFBF", "purple": "BF0040", "teal": "008080", "violet": "800080",
	"navy": "000080", "maroon": "800000", "silver": "C0C0C0", "aqua": "00FFFF",
	"fuchsia": "FF00FF", "gold": "FFD700",
	"Apricot": "FBB982", "Aquamarine": "00B5BE", "Bittersweet": "C04F17",
	"Black": "221E1F", "Blue": "2D2F92", "BlueGreen": "00B3B8", "BlueViolet": "473992",
	"BrickRed": "B6321C", "Brown": "792500", "BurntOrange": "F7921D",
	"CadetBlue": "74729A", "CarnationPink": "F282B4", "Cerulean": "00A2E3",
	"CornflowerBlue": "41B0E4", "Cyan": "00AEEF", "Dandelion": "FDBC42",
	"DarkOrchid": "A4538A", "Emerald": "00A99D", "ForestGreen": "009B55",
	"Fuchsia": "8C368C", "Goldenrod": "FFDF42", "Gray": "949698", "Green": "00A64F",
	"GreenYellow": "DFE674", "JungleGreen": "00A99A", "Lavender": "F49EC4",
	"LimeGreen": "8DC73E", "Magenta": "EC008C", "Mahogany": "A9341F",
	"Maroon": "AF3235", "Melon": "F89E7B", "MidnightBlue": "006795",
	"Mulberry": "A93C93", "NavyBlue": "006EB8", "OliveGreen": "3C8031",
	"Orange": "F58137", "OrangeRed": "ED135A", "Orchid": "AF72B0", "Peach": "F7965A",
	"Periwinkle": "7977B8", "PineGreen": "008B72", "Plum": "92268F",
	"ProcessBlue": "00B0F0", "Purple": "99479B", "RawSienna": "974006",
	"Red": "ED1B23", "RedOrange": "F26035", "RedViolet": "A1246B",
	"Rhodamine": "EF559F", "RoyalBlue": "0071BC", "RoyalPurple": "613F99",
	"RubineRed": "ED017D", "Salmon": "F69289", "SeaGreen": "3FBC9D", "Sepia": "671800",
	"SkyBlue": "46C5DD", "SpringGreen": "C6DC67", "Tan": "DA9D76", "TealBlue": "00AEB3",
	"Thistle": "D883B7", "Turquoise": "00B4CE", "Violet": "58429B",
	"VioletRed": "EF58A0", "White": "FFFFFF", "WildStrawberry": "EE2967",
	"Yellow": "FFF200", "YellowGreen": "98CC70", "YellowOrange": "FAA21A",
}
