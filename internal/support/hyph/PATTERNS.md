# Embedded hyphenation patterns

This package hyphenates only the languages that Typst 0.15 does not hyphenate
itself but that are normally hyphenated in print. All pattern files are
embedded **unmodified** (byte for byte as distributed) under `patterns/`; their
original copyright and licence headers are kept in `LICENSES/`. Patterns are
compiled lazily, once per language, on first use.

Only files whose licence is compatible with GPL-3.0 are embedded.

| Language | Tags served | File | Source | Copyright | Licence used here | Hyphenmin (left/right) |
|---|---|---|---|---|---|---|
| Latvian | `lv` | `hyph-lv.pat.txt` | hyph-utf8 (TeX Live 2026), v0.3 2005-09-14 | © 2004–2005 Janis Vilims | LGPL-2.1 or GPL-2.0-or-later (dual) | 2 / 2 |
| Romanian | `ro`, `mo` | `hyph_ro_RO.dic` | rospell project, Hyphen dictionary v3.3.6 | Adrian Stoica, Lucian Constantin | GPL-2.0-or-later | 2 / 2 |
| Macedonian | `mk` | `hyph-mk.pat.txt` | hyph-utf8 (TeX Live 2026), v0.1 (2006, revised 2020) | © 2006 Vasil Taneski; Stojan Trajanovski | GPL (no version stated, so any version may be chosen) | 2 / 2 |
| Irish | `ga` | `hyph-ga.pat.txt`, `hyph-ga.hyp.txt` (exceptions) | hyph-utf8 (TeX Live 2026), v1.0 2004-01-23 | © 2004–2015 Kevin P. Scannell | GPL-2.0-or-later or MIT (dual) | 2 / 3 |
| Basque | `eu` | `hyph-eu.pat.txt` | hyph-utf8 (TeX Live 2026), June 2008 | © 1997, 2008 Juan M. Aguirregabiria | Permissive notice licence (Unicode-style, see `LICENSES/hyph-eu.txt`) | 2 / 2 |
| Croatian patterns, used for Montenegrin, Serbian (Latin script) and Bosnian | `cnr`, `sr-Latn` (also `sr-ME`, `sh`), `bs`, `hr` | `hyph-hr.pat.txt` | hyph-utf8 (TeX Live 2026) | © 1994, 1996, 2011, 2015 Igor Marinović | Permissive notice licence (Unicode-style), chosen from its LPPL-1.0-or-later / permissive dual licence | 2 / 2 |

The Basque and Croatian notice licences require that the copyright and
permission notice accompany the data and its documentation: the full notices
are in `LICENSES/hyph-eu.txt` and `LICENSES/hyph-hr.txt`, which are part of
this documentation. The files have not been modified.

Romanian rospell patterns are distributed in the Hyphen (libhnj) dictionary
layout: a charset line, `%` comments and patterns already expanded for that
library's automaton. Applying Liang's algorithm (maximum value over all
matching patterns) to the expanded set gives the same result as the original
patterns, and the parser skips the header lines.

## Patterns deliberately not embedded

| Candidate | Reason |
|---|---|
| hyph-utf8 `hyph-ro` (Adrian Rezus, 1995–1996) | No licence statement at all (`licence: [None]`), so no permission to redistribute; replaced by the GPL-2.0-or-later rospell patterns. |
| hyph-utf8 `hyph-sh-latn`, `hyph-sh-cyrl` (Serbo-Croatian, Dejan Muhamedagić) | LPPL-1.0-or-later only, which is not GPL-compatible. The Croatian patterns, which follow the same syllabification rules, serve the Latin-script varieties instead. |
| hyph-utf8 `hyph-sr-cyrl` | GPL, but unnecessary: Typst hyphenates Cyrillic Serbian itself. |
| Luxembourgish (`lb`) | No hyphenation patterns exist in hyph-utf8; text is left unhyphenated. |
| Japanese, Chinese, Korean (`ja`, `zh`, `ko`) | Not hyphenated in print: lines break between characters (CJK) or between syllable blocks / words (Korean) following UAX #14 rules, which the typesetter applies itself. |
| Arabic, Hebrew (`ar`, `he`) | Words are not divided at line ends; Arabic justifies by elongation (kashida) and spacing, Hebrew by spacing. |

## Verification

* **Algorithm.** `testdata/reference.txt` holds about 1,900 words, mostly
  random letter strings, hyphenated by LuaTeX's `\showhyphens` with the same
  hyph-utf8 patterns (Latvian, Irish with its exceptions, Basque, Macedonian,
  Croatian) and, for Romanian, by the Hyphen library algorithm with the same
  dictionary. The package reproduces every break exactly; during development
  the comparison ran on 7,500 TeX words and 3,000 Romanian words without a
  single difference.
* **Typst coverage** (`TypstHyphenates`). Each language code was typeset by
  the Typst 0.15.1 binary with `#set text(lang: …, hyphenate: true)` in a
  1.5 cm justified column of long native words, and the PDF text was checked
  for hyphenated line ends. Typst hyphenates: af, be, bg, ca, cs, da, de, el,
  en, es, et, fi, fr, hr, hu, is, it, ka, ku, la, lt, mn, nb/nn/no, nl, pl, pt,
  ru, sk, sl, sq, sr (Cyrillic script only), sv, tk, tr, uk. It does **not**
  hyphenate lv, lb, mk, ro, eu, ga, gl, cnr, bs, Serbian in Latin script, or
  any three-letter code (deu, nob, …). Typst emits no warning for any of these
  codes.
* **Soft hyphens in Typst.** A Latvian paragraph processed by
  `InsertSoftHyphens` and typeset in a 4.2 cm justified column breaks words at
  the inserted positions, prints a hyphen only at line ends, and keeps the URL
  and e-mail address intact; without soft hyphens the same paragraph shows
  wide rivers of space.
