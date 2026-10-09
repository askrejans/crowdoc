// Clean — a contemporary sans-serif page: ragged right, block paragraphs,
// generous margins and a small page number. Prints only what is in the
// document; a title, when there is one, is set modestly.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let ink = cd-color(meta, "ink", rgb("#1d1f23"))
  let muted = cd-color(meta, "muted", rgb("#6e737c"))
  let accent = cd-color(meta, "accent", rgb("#2f5d8a"))
  let rule = cd-color(meta, "rule", rgb("#d5d7dc"))
  let size = meta.font-size
  let theme = (accent: accent, ink: ink, heading: cd-color(meta, "heading", ink), link: cd-color(meta, "link", accent),
    heading-font: meta.fonts.heading, justify: false, first-line-indent: 0pt, par-spacing: 1.5em, leading: 0.74em,
    table-style: "minimal", callout-style: "bar", quote-bar: cd-color(meta, "quote", rule), caption-size: 0.88em,
    footnote-size: 0.84em, code-size: 0.88em, list-indent: 0.2em, hrule: "line")
  let (pw, ph) = cd-page-dims(meta)
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    columns: meta.columns,
    margin: cd-measure-margins(meta, size * 34, min-side: pw * 0.13, top: ph * 0.1, bottom: ph * 0.11, start-share: 0.44),
    // No running header of its own: only the document's header texts.
    ..(header: if meta.header-left != none or meta.header-right != none {
      set text(font: meta.fonts.sans, size: size * 0.76, fill: muted)
      cd-running(meta.header-left, none, meta.header-right)
    }),
    footer: context {
      set text(font: meta.fonts.sans, size: size * 0.76, fill: muted, number-type: "lining")
      let folio = if counter(page).final().first() > 1 { str(counter(page).get().first()) }
      cd-running(meta.footer-left, none, cd-join((meta.footer-right, folio), sep: h(1.2em)))
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans, costs: (hyphenation: 300%))
  set par(linebreaks: "optimized")
  set table(stroke: (x, y) => if y == 0 { (bottom: 0.6pt + ink) } else { (bottom: 0.4pt + rule) }, inset: (x: 7pt, y: 5pt))
  show table.cell.where(y: 0): set text(weight: "semibold")
  set footnote.entry(separator: line(length: 3em, stroke: 0.5pt + rule))

  let num(it) = if it.numbering != none { text(fill: muted, counter(heading).display(it.numbering)); h(0.7em) }
  show heading: set text(font: meta.fonts.heading)
  show heading.where(level: 1): it => block(above: 2.2em, below: 0.85em, sticky: true,
    text(size: size * 1.38, weight: "semibold", tracking: -0.005em, { num(it); it.body }))
  show heading.where(level: 2): it => block(above: 1.8em, below: 0.7em, sticky: true,
    text(size: size * 1.1, weight: "semibold", { num(it); it.body }))
  show heading: it => if it.level > 2 {
    block(above: 1.5em, below: 0.6em, sticky: true, text(size: size, weight: "semibold", fill: muted, { num(it); it.body }))
  } else { it }

  show quote.where(block: true): it => block(above: 1.3em, below: 1.3em, inset: (left: 1.1em, y: 0.2em),
    stroke: (left: 1.5pt + cd-color(meta, "quote", rule)), {
      set text(fill: cd-tint(meta, ink, 22%))
      it.body
      if it.attribution != none { block(above: 0.6em, text(size: size * 0.9, fill: muted)[— #it.attribution]) }
    })
  show figure: set block(above: 1.8em, below: 1.8em)
  show figure.caption: set text(fill: muted)

  // Title block: only when the document has a real title.
  let byline = (cd-names(meta).join(", "), meta.date).filter(x => x != none)
  if meta.title != none {
    block(below: 2.6em, {
      cd-balanced(text(font: meta.fonts.heading, size: size * 1.95, weight: "semibold", tracking: -0.015em, hyphenate: false, meta.title), align-to: start)
      if meta.subtitle != none { v(0.5em); text(size: size * 1.15, fill: muted, meta.subtitle) }
      if byline.len() > 0 { v(1em); text(size: size * 0.85, fill: muted, byline.join(h(0.9em) + sym.dot.c + h(0.9em))) }
    })
  } else if byline.len() > 0 {
    block(below: 2em, text(size: size * 0.85, fill: muted, byline.join(h(0.9em) + sym.dot.c + h(0.9em))))
  }
  if meta.abstract != none { block(below: 1.8em, text(size: size * 1.05, fill: cd-tint(meta, ink, 15%), meta.abstract)) }
  if meta.toc { outline(depth: 2); v(1.5em) }
  body
}
