# fonts

Package `fonts` curates open-licensed typefaces and typeface pairings for the
typesetting engines, installs them on demand and finds families that are
already available.

The font files are **not** part of this repository. `Install` downloads them
once into a cache directory and verifies every file against the size and
SHA-256 pinned in `catalog.go`; afterwards everything works offline. With
nothing installed, every pairing still renders, because each fallback chain
ends in a family Typst embeds (Libertinus Serif, New Computer Modern, New
Computer Modern Math, DejaVu Sans Mono).

## Directories

- `DefaultDir()`: `$CROWDOC_FONTS`, otherwise the user cache directory's
  `crowdoc/fonts` (for example `~/.cache/crowdoc/fonts`,
  `~/Library/Caches/crowdoc/fonts`, `%LocalAppData%\crowdoc\fonts`). Each
  family gets a subdirectory such as `eb-garamond/`.
- `SearchDirs(extra...)`: the existing directories to pass to the engine
  (`typst --font-path`), in priority order: `extra`, the cache directory, then
  TeX Live's `fonts/opentype` and `fonts/truetype` trees (located with
  `kpsewhich -var-value=TEXMFDIST`, or well-known install paths when TeX is not
  on `PATH`).
- `Scan(dirs)` reads the family names of all fonts below those directories and
  caches them in the user cache directory (`crowdoc/font-scan.json`, keyed by
  path, size and modification time).

## Family names

Names follow Typst, which uses name ID 1 with trailing style words trimmed
(plus a small table of exceptions) and deliberately ignores name ID 16. `Scan`
ports that logic so `Resolve` keeps exactly the names Typst will find. Two
variable fonts therefore carry their default optical size in the name:
**DM Sans 9pt** and **Newsreader 16pt**.

## Sets

`Install(ctx, dir, sets, families, progress)` installs whole sets and/or
individual families; with neither it installs `core`, and `all` selects every
set. The CJK and emoji families are large and only needed for those scripts,
so they are separate.

| Set | Families | Fonts on disk | Download |
|-----|---------:|--------------:|---------:|
| core | 23 | 25.4 MB | 33.3 MB |
| extended | 36 | 44.2 MB | 44.2 MB |
| cjk | 8 | 129.1 MB | 129.1 MB |
| emoji | 1 | 25.3 MB | 25.3 MB |

"Download" counts the Libertinus release archive once (9.4 MB for 0.9 MB of
fonts).

## Sources and licences

Every family is licensed under the **SIL Open Font License 1.1** (verified
from each upstream `OFL.txt`/`METADATA.pb`). The OFL permits use, embedding in
documents, modification and redistribution, including bundling with
software, provided the licence text travels with the fonts, the fonts are not
sold on their own, and modified versions do not use the Reserved Font Names.

Downloads are pinned for reproducibility:

- **google/fonts** repository at commit
  `2eb0b48d5f760f62e286216f0859a8c540dbc1bd` (2026-10-08), fetched from
  `raw.githubusercontent.com`. Variable fonts are used where available
  (`Inter[opsz,wght].ttf` and its italic); static families ship Regular,
  Italic, SemiBold (or Medium) and Bold weights only.
- **Libertinus 7.051** official release archive
  (`alerque/libertinus`, `Libertinus-7.051.zip`, SHA-256
  `4d9be29b5cb380c35af8ba967abcc752ad1e07be1f738a9789c33e0dd7478c92`) for
  Libertinus Sans, Mono and Math: the google/fonts conversions of Libertinus
  Sans and Math lack their GSUB and MATH tables, i.e. no ligatures, small caps
  or math layout. Libertinus Serif is not downloaded because Typst embeds it.
- **Fira Math 0.3.4** release asset (`firamath/firamath`).
- **Garamond-Math** from its upstream repository at commit
  `42b7c154422ae45e6f834654648da4ac467c656b` (binary last rebuilt 2020-05-29;
  TeX Live ships the slightly newer 2022-01-03 CTAN build).
- **IBM Plex Math 1.000** from `IBM/plex` at commit
  `763c36ef9117782905ae010056dfbe8fd2653a25`.

| Family (Typst name) | Set | Category | Scripts | Files | Size | Version | Licence | Source |
|---|---|---|---|---|---:|---|---|---|
| EB Garamond | core | serif | latin, greek, cyrillic, vietnamese | 2 (variable) | 1.6 MB | 1.003 | OFL-1.1 | google/fonts `ofl/ebgaramond` |
| Cormorant Garamond | extended | display | latin, cyrillic, vietnamese | 2 (variable) | 1.9 MB | 4.001 | OFL-1.1 | google/fonts `ofl/cormorantgaramond` |
| Crimson Pro | extended | serif | latin, vietnamese | 2 (variable) | 0.5 MB | 1.003 | OFL-1.1 | google/fonts `ofl/crimsonpro` |
| Libre Baskerville | extended | serif | latin | 2 (variable) | 0.3 MB | 2.005 | OFL-1.1 | google/fonts `ofl/librebaskerville` |
| Literata | extended | serif | latin, greek, cyrillic, vietnamese | 2 (variable) | 1.9 MB | 3.103 | OFL-1.1 | google/fonts `ofl/literata` |
| Lora | extended | serif | latin, cyrillic, vietnamese | 2 (variable) | 0.4 MB | 3.008 | OFL-1.1 | google/fonts `ofl/lora` |
| Merriweather | extended | serif | latin, cyrillic, vietnamese | 2 (variable) | 9.2 MB | 2.101 | OFL-1.1 | google/fonts `ofl/merriweather` |
| Source Serif 4 | core | serif | latin, greek, cyrillic, vietnamese | 2 (variable) | 2.1 MB | 4.004 | OFL-1.1 | google/fonts `ofl/sourceserif4` |
| PT Serif | extended | serif | latin, cyrillic | 4 | 1.4 MB | 1.000W OFL | OFL-1.1 | google/fonts `ofl/ptserif` |
| Spectral | extended | serif | latin, cyrillic, vietnamese | 6 | 1.6 MB | 2.005 | OFL-1.1 | google/fonts `ofl/spectral` |
| Alegreya | extended | serif | latin, greek, cyrillic, vietnamese | 2 (variable) | 0.9 MB | 2.009 | OFL-1.1 | google/fonts `ofl/alegreya` |
| Noto Serif | core | serif | latin, greek, cyrillic, vietnamese | 2 (variable) | 4.3 MB | 2.013 | OFL-1.1 | google/fonts `ofl/notoserif` |
| IBM Plex Serif | extended | serif | latin, cyrillic, vietnamese | 6 | 1.0 MB | 2.6 | OFL-1.1 | google/fonts `ofl/ibmplexserif` |
| STIX Two Text | core | serif | latin, greek, cyrillic, vietnamese | 2 (variable) | 0.9 MB | 2.13 b171 | OFL-1.1 | google/fonts `ofl/stixtwotext` |
| Charis SIL | extended | serif | latin, cyrillic, vietnamese | 4 | 3.1 MB | 6.101 | OFL-1.1 | google/fonts `ofl/charissil` |
| Newsreader 16pt | extended | serif | latin, vietnamese | 2 (variable) | 0.9 MB | 1.003 | OFL-1.1 | google/fonts `ofl/newsreader` |
| Fraunces | extended | display | latin, vietnamese | 2 (variable) | 0.8 MB | 1.000 | OFL-1.1 | google/fonts `ofl/fraunces` |
| Playfair Display | extended | display | latin, cyrillic, vietnamese | 2 (variable) | 0.6 MB | 1.203 | OFL-1.1 | google/fonts `ofl/playfairdisplay` |
| DM Serif Display | extended | display | latin | 2 | 0.1 MB | 5.200 | OFL-1.1 | google/fonts `ofl/dmserifdisplay` |
| Inter | core | sans | latin, greek, cyrillic, vietnamese | 2 (variable) | 1.8 MB | 4.001 | OFL-1.1 | google/fonts `ofl/inter` |
| Source Sans 3 | core | sans | latin, greek, cyrillic, vietnamese | 2 (variable) | 1.0 MB | 3.052 | OFL-1.1 | google/fonts `ofl/sourcesans3` |
| IBM Plex Sans | extended | sans | latin, greek, cyrillic, vietnamese | 2 (variable) | 1.1 MB | 3.201 | OFL-1.1 | google/fonts `ofl/ibmplexsans` |
| Fira Sans | extended | sans | latin, greek, cyrillic, vietnamese | 6 | 2.9 MB | 4.203 | OFL-1.1 | google/fonts `ofl/firasans` |
| Lato | extended | sans | latin, greek, cyrillic, vietnamese | 6 | 4.1 MB | 2.015 | OFL-1.1 | google/fonts `ofl/lato` |
| Open Sans | extended | sans | latin, greek, cyrillic, vietnamese, hebrew | 2 (variable) | 1.1 MB | 3.003 | OFL-1.1 | google/fonts `ofl/opensans` |
| Work Sans | extended | sans | latin, vietnamese | 2 (variable) | 0.7 MB | 2.012 | OFL-1.1 | google/fonts `ofl/worksans` |
| Libre Franklin | extended | sans | latin, cyrillic, vietnamese | 2 (variable) | 0.4 MB | 3.000 | OFL-1.1 | google/fonts `ofl/librefranklin` |
| Montserrat | extended | sans | latin, cyrillic, vietnamese | 2 (variable) | 1.5 MB | 9.000 | OFL-1.1 | google/fonts `ofl/montserrat` |
| Manrope | extended | sans | latin, greek, cyrillic, vietnamese | 1 (variable) | 0.2 MB | 4.505 | OFL-1.1 | google/fonts `ofl/manrope` |
| DM Sans 9pt | extended | sans | latin | 2 (variable) | 0.5 MB | 4.004 | OFL-1.1 | google/fonts `ofl/dmsans` |
| Noto Sans | core | sans | latin, greek, cyrillic, vietnamese | 2 (variable) | 4.4 MB | 2.015 | OFL-1.1 | google/fonts `ofl/notosans` |
| Atkinson Hyperlegible Next | core | sans | latin | 2 (variable) | 0.2 MB | 2.001 | OFL-1.1 | google/fonts `ofl/atkinsonhyperlegiblenext` |
| Figtree | extended | sans | latin | 2 (variable) | 0.1 MB | 2.002 | OFL-1.1 | google/fonts `ofl/figtree` |
| Outfit | extended | sans | latin | 1 (variable) | 0.1 MB | 1.100 | OFL-1.1 | google/fonts `ofl/outfit` |
| Space Grotesk | extended | sans | latin, vietnamese | 1 (variable) | 0.1 MB | 2.000 | OFL-1.1 | google/fonts `ofl/spacegrotesk` |
| Alegreya Sans | extended | sans | latin, greek, cyrillic, vietnamese | 6 | 1.6 MB | 2.004 | OFL-1.1 | google/fonts `ofl/alegreyasans` |
| Libertinus Sans | core | sans | latin, greek, cyrillic, vietnamese, hebrew | 3 | 0.9 MB | 7.051 | OFL-1.1 | Libertinus 7.051 release zip |
| PT Sans | extended | sans | latin, cyrillic | 4 | 1.7 MB | 2.003W OFL | OFL-1.1 | google/fonts `ofl/ptsans` |
| JetBrains Mono | core | mono | latin, greek, cyrillic, vietnamese | 2 (variable) | 0.4 MB | 2.211 | OFL-1.1 | google/fonts `ofl/jetbrainsmono` |
| Source Code Pro | core | mono | latin, greek, cyrillic, vietnamese | 2 (variable) | 0.4 MB | 1.026 | OFL-1.1 | google/fonts `ofl/sourcecodepro` |
| IBM Plex Mono | extended | mono | latin, cyrillic, vietnamese | 6 | 0.8 MB | 2.3 | OFL-1.1 | google/fonts `ofl/ibmplexmono` |
| Fira Code | extended | mono | latin, greek, cyrillic | 1 (variable) | 0.3 MB | 5.002 | OFL-1.1 | google/fonts `ofl/firacode` |
| Fira Mono | extended | mono | latin, greek, cyrillic | 3 | 0.6 MB | 3.206 | OFL-1.1 | google/fonts `ofl/firamono` |
| Noto Sans Mono | core | mono | latin, greek, cyrillic, vietnamese | 1 (variable) | 1.7 MB | 2.014 | OFL-1.1 | google/fonts `ofl/notosansmono` |
| Roboto Mono | extended | mono | latin, greek, cyrillic, vietnamese | 2 (variable) | 0.4 MB | 3.001 | OFL-1.1 | google/fonts `ofl/robotomono` |
| DM Mono | extended | mono | latin | 4 | 0.2 MB | 1.000 | OFL-1.1 | google/fonts `ofl/dmmono` |
| Libertinus Mono | core | mono | latin | 1 | 0.1 MB | 7.051 | OFL-1.1 | Libertinus 7.051 release zip |
| PT Mono | extended | mono | latin, cyrillic | 1 | 0.2 MB | 1.001W OFL | OFL-1.1 | google/fonts `ofl/ptmono` |
| Atkinson Hyperlegible Mono | core | mono | latin | 2 (variable) | 0.1 MB | 2.001 | OFL-1.1 | google/fonts `ofl/atkinsonhyperlegiblemono` |
| STIX Two Math | core | math | math | 1 | 1.5 MB | 2.12 b168a | OFL-1.1 | google/fonts `ofl/stixtwomath` |
| Libertinus Math | core | math | math | 1 | 0.6 MB | 7.051 | OFL-1.1 | Libertinus 7.051 release zip |
| Fira Math | core | math | math | 1 | 0.2 MB | 0.3.4 | OFL-1.1 | firamath 0.3.4 release |
| Garamond-Math | core | math | math | 1 | 0.8 MB | 2019-08-16 | OFL-1.1 | Garamond-Math @ `42b7c15` |
| IBM Plex Math | extended | math | math | 1 | 0.7 MB | 1.000 | OFL-1.1 | IBM/plex @ `763c36e` |
| Noto Sans Math | core | math | math | 1 | 1.0 MB | 3.000 | OFL-1.1 | google/fonts `ofl/notosansmath` |
| Noto Naskh Arabic | core | serif | arabic | 1 (variable) | 0.3 MB | 2.021 | OFL-1.1 | google/fonts `ofl/notonaskharabic` |
| Noto Sans Arabic | core | sans | arabic | 1 (variable) | 0.8 MB | 2.012 | OFL-1.1 | google/fonts `ofl/notosansarabic` |
| Noto Serif Hebrew | core | serif | hebrew | 1 (variable) | 0.2 MB | 2.004 | OFL-1.1 | google/fonts `ofl/notoserifhebrew` |
| Noto Sans Hebrew | core | sans | hebrew | 1 (variable) | 0.1 MB | 3.001 | OFL-1.1 | google/fonts `ofl/notosanshebrew` |
| Noto Serif SC | cjk | serif | chinese-simplified | 1 (variable) | 25.1 MB | 2.003-H1 | OFL-1.1 | google/fonts `ofl/notoserifsc` |
| Noto Serif TC | cjk | serif | chinese-traditional | 1 (variable) | 16.9 MB | 2.003-H1 | OFL-1.1 | google/fonts `ofl/notoseriftc` |
| Noto Serif JP | cjk | serif | japanese | 1 (variable) | 13.6 MB | 2.003-H1 | OFL-1.1 | google/fonts `ofl/notoserifjp` |
| Noto Serif KR | cjk | serif | korean | 1 (variable) | 23.8 MB | 2.003-H1 | OFL-1.1 | google/fonts `ofl/notoserifkr` |
| Noto Sans SC | cjk | sans | chinese-simplified | 1 (variable) | 17.8 MB | 2.004-H2 | OFL-1.1 | google/fonts `ofl/notosanssc` |
| Noto Sans TC | cjk | sans | chinese-traditional | 1 (variable) | 11.9 MB | 2.004-H2 | OFL-1.1 | google/fonts `ofl/notosanstc` |
| Noto Sans JP | cjk | sans | japanese | 1 (variable) | 9.6 MB | 2.004-H2 | OFL-1.1 | google/fonts `ofl/notosansjp` |
| Noto Sans KR | cjk | sans | korean | 1 (variable) | 10.4 MB | 2.004-H2 | OFL-1.1 | google/fonts `ofl/notosanskr` |
| Noto Color Emoji | emoji | emoji | emoji | 1 | 25.3 MB | 2.055 | OFL-1.1 | google/fonts `ofl/notocoloremoji` |

## Pairings

A pairing names a main (body), sans, mono, math and optional heading face. Each
role is a fallback chain: the intended family, close substitutes, then the
embedded fallback. Math faces are chosen to match the text face. Every body
and sans face has a real italic (Typst never slants a roman). Only the chain
heads are listed here; see `pairings.go` for the full chains.

| Pairing | Aliases | Category | Main | Sans | Mono | Math | Heading |
|---|---|---|---|---|---|---|---|
| `classic` | garamond, ebgaramond, book | book | EB Garamond | Source Sans 3 | Source Code Pro | Garamond-Math | — |
| `elegant` | cormorant, spectral | book | Spectral | Lato | IBM Plex Mono | Libertinus Math | Cormorant Garamond |
| `libertine` | libertinus, linuxlibertine | academic | Libertinus Serif | Libertinus Sans | Libertinus Mono | Libertinus Math | — |
| `computer-modern` | cm, latex, tex, newcomputermodern | academic | New Computer Modern | New Computer Modern Sans | New Computer Modern Mono | New Computer Modern Math | — |
| `scientific` | stix, stixtwo, science | academic | STIX Two Text | Inter | JetBrains Mono | STIX Two Math | — |
| `source` | sourceserif, sourcesans | business | Source Serif 4 | Source Sans 3 | Source Code Pro | STIX Two Math | — |
| `plex` | ibmplex | business | IBM Plex Serif | IBM Plex Sans | IBM Plex Mono | IBM Plex Math | — |
| `modern` | inter, sans | modern | Inter | Inter | JetBrains Mono | Fira Math | — |
| `fira` | firasans | modern | Fira Sans | Fira Sans | Fira Mono | Fira Math | — |
| `charter` | charis, charissil, xcharter | business | Charis SIL | Fira Sans | Fira Mono | XCharter Math | — |
| `crimson` | crimsonpro | book | Crimson Pro | Lato | Source Code Pro | Libertinus Math | — |
| `literata` |  | book | Literata | Open Sans | Roboto Mono | STIX Two Math | — |
| `editorial` | playfair | business | Source Serif 4 | Source Sans 3 | Source Code Pro | STIX Two Math | Playfair Display |
| `baskerville` | librebaskerville | book | Libre Baskerville | Libre Franklin | IBM Plex Mono | STIX Two Math | — |
| `humanist` | alegreya | book | Alegreya | Alegreya Sans | Fira Mono | Libertinus Math | — |
| `noto` | multilingual | multilingual | Noto Serif | Noto Sans | Noto Sans Mono | STIX Two Math | — |
| `accessible` | atkinson, hyperlegible, legible | accessible | Atkinson Hyperlegible Next | Atkinson Hyperlegible Next | Atkinson Hyperlegible Mono | Fira Math | — |
| `geometric` | montserrat, figtree | modern | Figtree | Figtree | Roboto Mono | Noto Sans Math | Montserrat |
| `newsreader` | news | book | Newsreader 16pt | Work Sans | Roboto Mono | STIX Two Math | — |
| `pt` | ptserif | multilingual | PT Serif | PT Sans | PT Mono | STIX Two Math | — |
| `tech` | spacegrotesk, dmsans | modern | DM Sans 9pt | DM Sans 9pt | DM Mono | Fira Math | Space Grotesk |
| `warm` | fraunces, lora | book | Lora | Figtree | Fira Code | Libertinus Math | Fraunces |
| `magazine` | dmserif, merriweather | business | Merriweather | DM Sans 9pt | DM Mono | STIX Two Math | DM Serif Display |

Use `LookupPairing(name)` (case, spaces, hyphens and underscores are ignored;
aliases accepted) and then either `Resolve(p.Chain(role), available)` or
`p.Fonts(role, lang, available)`. `Fonts` additionally inserts the installed
Arabic, Hebrew, CJK and emoji Noto faces: the document language's own script
goes before the embedded fallback (so Japanese text gets Noto Serif JP rather
than the Chinese glyph forms of Noto Serif SC), all others go last so quoted
foreign text still renders.

### Families used but not downloaded

- Embedded in Typst: Libertinus Serif, New Computer Modern, New Computer
  Modern Math, DejaVu Sans Mono.
- TeX Live only: New Computer Modern Sans and Mono, Latin Modern Sans and Mono
  (GUST Font License), XCharter and XCharter Math (Bitstream Charter licence),
  Source Serif Pro and Source Sans Pro (older names of Source Serif 4 and
  Source Sans 3). TeX Live also ships many catalog families (EB Garamond,
  Libertinus, STIX Two, Inter, JetBrains Mono, IBM Plex, Fira, Noto and more),
  which `SearchDirs` picks up without a download.
- Common system sans families (Helvetica Neue, Helvetica, Arial, DejaVu Sans,
  Liberation Sans, Noto Sans) sit in sans chains for callers that also scan
  system fonts.

### Known limitations of the faces

- Typst does not synthesise small caps: faces without an `smcp` feature (for
  example Inter, Figtree, DM Sans, Newsreader, PT Serif) render small caps as
  lowercase.
- Charis SIL and Merriweather contain a handful of Greek letters (for IPA and
  symbols) but not the full alphabet, so running Greek text mixes faces. For
  Greek documents prefer a pairing whose main face covers Greek: `classic`,
  `libertine`, `computer-modern`, `scientific`, `source`, `modern`, `fira`,
  `literata`, `humanist` or `noto`.
- Newsreader 1.003 draws capital Ģ (U+0122) with the comma above instead of
  below, which is wrong for Latvian.

## Updating the catalog

Bump `googleFontsCommit` (or the release pins), download every file, and
recompute sizes and SHA-256 sums from the downloaded bytes; never copy them
from elsewhere. Check family names with `typst fonts --font-path DIR
--ignore-system-fonts` and run the tests, including the networked one:
`CROWDOC_FONT_NETWORK=1 go test ./internal/fonts/`.
