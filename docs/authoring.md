# Writing documents for crowdoc

crowdoc reads Markdown — CommonMark with GitHub tables, task lists and alerts —
plus the Pandoc extensions below, and typesets it with a style, colour scheme
and typeface pairing. Other formats (Word, OpenDocument, RTF, HTML, EPUB,
notebooks, spreadsheets, text, PDF) are converted to the same document model,
so everything here applies to them as well.

Every feature degrades gracefully: unknown keys are kept for custom templates,
unresolved references and missing images are reported as warnings, and the
document still typesets.

## Frontmatter

```yaml
---
title: Soil Microbiome Diversity
subtitle: A comparative analysis
author:
  - name: Marta Liepa
    affiliation: University of Latvia
    email: marta@example.org
    orcid: 0000-0002-1825-0097
    corresponding: true
  - Jānis Ozols
date: 2026-03-15            # ISO dates are printed in the document language
lang: lv                    # 40 languages: captions, dates, quotes, hyphenation
style: article              # crowdoc --list-styles
colors: oxford              # crowdoc --list-colors, or a map (see below)
fonts: classic              # crowdoc --list-fonts
abstract: |
  One paragraph of *Markdown*.
keywords: [soil, microbiome]
bibliography: references.bib   # .bib, .json, .yaml, .ris, .nbib (or a list)
csl: apa                       # apa, chicago, ieee, harvard, vancouver, mla
toc: true
lof: true                      # list of figures
lot: true                      # list of tables
number-sections: true
paper: a4                      # a4, letter, legal, a5, b5, …
landscape: false
columns: 2
font-size: 11
line-spacing: onehalf          # single, onehalf, double or a factor
margin: 2.5cm                  # or margin-top/bottom/left/right
organization: Example Ltd
version: "1.2"
status: draft
classification: internal
logo: assets/logo.png
header-left: Custom running header
footer-right: Page footer text
pdfa: true                     # archival PDF/A-2b
---
```

### Colours

```yaml
colors: emerald                       # a named scheme
colors:                               # or individual roles
  scheme: oxford                      # optional base scheme
  accent: "#6A00FF"
  page: "#FBF8F1"
```

Roles: `accent`, `secondary`, `ink`, `heading`, `muted`, `link`, `rule`,
`code-bg`, `table-head`, `stripe`, `quote`, `page`.

### Typefaces

`fonts: <pairing>` selects a pairing; `font`, `sans-font`, `mono-font` and
`math-font` override single roles with a family name or another pairing's
name. Install the curated open-licence families with `crowdoc fonts install`
(`core`, `extended`, `cjk`, `emoji` sets).

### Letters, theses, agreements

Letters and memos use `recipient` (address lines), `sender`, `to`, `from`,
`cc`, `opening`, `closing` and `place`. Theses use `institution`, `faculty`,
`department`, `degree`, `supervisor` and `location`. Agreements use `parties`
and `signatures: true`. Any other key is passed to templates as
`meta.extra.<key>` (for example `attendees` in minutes, `course` and
`instructor` in essays, `running-head` in APA papers).

## Structure

- Headings `#` … `####`. A single top-level `# Title` becomes the document
  title. Add `{#sec:intro}` for a cross-reference label and `{-}` to leave a
  heading unnumbered.
- Footnotes: `text[^1]` with `[^1]: note`, or inline `^[note text]`.
- Callouts: `::: note`, `tip`, `info`, `important`, `warning`, `caution`,
  `danger`, `success`, `example`, `abstract` — closed by `:::`, optional title
  after the name (`::: warning Read first`). Quarto (`{.callout-tip}`) and
  GitHub alerts (`> [!NOTE]`) work too.
- Numbered theorem-like blocks: `::: theorem`, `lemma`, `corollary`,
  `proposition`, `definition`, `remark`, `proof`, `exercise`, `solution`.
- `::: appendix` restarts top-level numbering as A, B, C; `::: center` centres
  a block; `::: landscape` puts it on a landscape page.
- Task lists `- [ ]` / `- [x]`, definition lists (`Term` / `: definition`),
  `==highlight==`, `H^2^O`, line blocks (`| line`), and inline HTML such as
  `<sup>`, `<sub>`, `<kbd>`, `<mark>`, `<br>`.
- Typographic quotes follow the document language; `--`, `---` and `...`
  become en dash, em dash and ellipsis outside code.

## Figures, tables, equations and code

```markdown
![Caption with *formatting*.](figures/chart.png){#fig:chart width=80%}

| Region  | Farms |
|:--------|------:|
| Zemgale |    14 |

Table: Study sites. {#tbl:sites}

$$ E = mc^2 $$ {#eq:energy}

Listing: Fitting the model. {#lst:fit}
```r
model <- lm(y ~ x)
```
```

An image alone in a paragraph becomes a numbered figure (its alt text is the
caption). Images may be PNG, JPEG, GIF, WebP, SVG, PDF, BMP or TIFF (HEIC is
converted on macOS); `width`/`height` accept `%`, `cm`, `mm`, `in`, `pt` and
`px`. Without a width, images keep their natural size up to the text width.

Refer to labelled elements with `@fig:chart`, `@tbl:sites`, `@eq:energy`,
`@sec:intro` — rendered as "Figure 1", "1. attēls", "図1" … according to the
document language; `[-@fig:chart]` prints the number only.

Inline math `$x^2$` or `\(x^2\)`; display math `$$…$$` or `\[…\]` in LaTeX
notation. `$5 and $10` stays text.

Fenced code blocks are highlighted by language; add `.numberLines` for line
numbers. Wide tables shrink their type automatically and break across pages
with repeated headers.

## Citations

| Syntax | Result (APA) |
|---|---|
| `[@doe2020]` | (Doe, 2020) |
| `[@doe2020, p. 33]` | (Doe, 2020, p. 33) |
| `[see @doe2020; @roe2019, chap. 2]` | (see Doe, 2020; Roe, 2019, chap. 2) |
| `@doe2020 argues` | Doe (2020) argues |
| `@doe2020 [p. 4] argues` | Doe (2020, p. 4) argues |
| `[-@doe2020]` | (2020) |

The reference list goes at a final empty `# References` heading (its title is
kept), at a `::: {#refs}` block, or at the end. `nocite: ["@key", "@*"]` lists
uncited entries. References can also be given inline as CSL YAML under
`references:`.

Word documents keep their Zotero, Mendeley and built-in Word citations;
OpenDocument files keep their bibliography marks. Documents with a
hand-written numbered reference list get their `[1]`-style citations linked to
it.
