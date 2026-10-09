// Typewriter — the typed page: monospaced type at one size, ragged right,
// one-and-a-half line spacing, one-inch margins, underlined emphasis and
// the page number at the top. Prints only what is in the document.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let ink = cd-color(meta, "ink", rgb("#232220"))
  let muted = cd-color(meta, "muted", rgb("#6a6762"))
  let size = meta.font-size
  let lead = if meta.leading != none { meta.leading } else { 1.02em }
  let theme = (ink: ink, heading: cd-color(meta, "heading", ink), link: cd-color(meta, "link", ink), accent: cd-color(meta, "accent", ink),
    justify: false, first-line-indent: 0pt, leading: lead, par-spacing: lead + 0.75em, table-style: "minimal",
    callout-style: "minimal", code-bg: none, code-size: 1em, caption-size: 1em, footnote-size: 1em, table-size: 1em,
    list-indent: 0pt, hrule: "dots")
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    columns: meta.columns,
    margin: cd-margins(meta, top: 1in, bottom: 1in, left: 1in, right: 1in),
    header-ascent: 40%,
    header: context {
      set text(font: (..meta.fonts.mono, ..meta.fonts.main), size: size, fill: ink, number-type: "lining")
      let n = counter(page).get().first()
      let folio = if counter(page).final().first() > 1 and n > 1 { str(n) + "." }
      cd-running(meta.header-left, none, cd-join((meta.header-right, folio), sep: h(2em)))
    },
    // No footer of its own: only the document's footer texts.
    ..(footer: if meta.footer-left != none or meta.footer-right != none {
      set text(font: (..meta.fonts.mono, ..meta.fonts.main), size: size, fill: muted)
      cd-running(meta.footer-left, none, meta.footer-right)
    }),
  )
  show: cd-base.with(meta, theme: theme)
  // One face, one size; the serif face only fills glyphs the typewriter lacks.
  set text(font: (..meta.fonts.mono, ..meta.fonts.main), hyphenate: false, number-type: "lining")
  set par(linebreaks: "optimized")
  show raw: set text(font: (..meta.fonts.mono, ..meta.fonts.main), size: size)
  show raw.where(block: false): it => it
  show raw.where(block: true): it => pad(left: 2.5em, block(above: 1.2em, below: 1.2em, {
    set par(leading: 0.65em)
    it
  }))

  // Typed emphasis: underlining, and strike-twice bold.
  show emph: it => underline(offset: 0.2em, stroke: 0.06em + ink, evade: true, it.body)
  show strong: it => text(stroke: 0.035em + ink, it.body)
  show link: it => if type(it.dest) == str { underline(offset: 0.2em, stroke: 0.05em, it) } else { it }

  // Headings stay at the one size: capitals, then underlining.
  let num(it) = if it.numbering != none { counter(heading).display(it.numbering); h(1em) }
  show heading: set text(font: (..meta.fonts.mono, ..meta.fonts.main), weight: "regular")
  show heading.where(level: 1): it => block(above: lead + 1.6em, below: lead + 0.4em, sticky: true,
    text(size: size, stroke: 0.035em + ink, { num(it); upper(it.body) }))
  show heading.where(level: 2): it => block(above: lead + 1.2em, below: lead + 0.3em, sticky: true,
    text(size: size, { num(it); underline(offset: 0.2em, stroke: 0.06em + ink, evade: true, it.body) }))
  show heading: it => if it.level > 2 {
    block(above: lead + 1em, below: lead + 0.2em, sticky: true, text(size: size, { num(it); it.body }))
  } else { it }

  set list(marker: ([-], [-], [-]), body-indent: 1em)
  set enum(body-indent: 0.6em)
  show quote.where(block: true): it => pad(x: 2.5em, block(above: 1.2em, below: 1.2em, {
    set par(leading: 0.65em, spacing: 1.2em)
    it.body
    if it.attribution != none { align(end)[-- #it.attribution] }
  }))
  set footnote.entry(separator: line(length: 12em, stroke: 0.5pt + ink), gap: 0.5em)
  show footnote.entry: set par(leading: 0.65em)
  set table(stroke: (x, y) => if y == 0 { (top: 0.5pt + ink, bottom: 0.5pt + ink) } else { none })
  show table: it => { it; v(0pt) }
  show table.cell.where(y: 0): set text(weight: "regular")
  show figure.caption: set text(size: size)
  show figure: set block(above: lead + 1.2em, below: lead + 1.2em)

  // Title: centred capitals, as typed; only when the document has one.
  let byline = (cd-names(meta).join(", "), meta.date).filter(x => x != none)
  if meta.title != none {
    align(center, block(below: lead * 2 + 2em, {
      set par(leading: lead)
      text(size: size, upper(meta.title))
      if meta.subtitle != none { v(0.4em); text(size: size, meta.subtitle) }
      if byline.len() > 0 { v(lead + 0.8em); byline.join(linebreak()) }
    }))
  } else if byline.len() > 0 {
    block(below: lead * 2 + 1em, byline.join(linebreak()))
  }
  if meta.abstract != none { pad(x: 2.5em, bottom: lead + 1em, meta.abstract) }
  if meta.toc { outline(depth: 2); v(2em) }
  body
}
