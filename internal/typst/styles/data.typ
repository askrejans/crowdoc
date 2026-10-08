// Data — spreadsheet and CSV exports: compact headers, striped tables that
// repeat their header row on every page, landscape for wide tables.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#1f6f5c"))
  let muted = cd-color(meta, "muted", rgb("#6b7280"))
  let theme = (accent: accent, link: accent, heading: cd-color(meta, "heading", rgb("#111827")), heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.7em, table-style: "striped", table-head-fill: cd-tint(meta, accent, 86%),
    table-stripe: rgb("#f6f7f9"), table-size: 0.86em, callout-style: "box")
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 1.8cm, bottom: 1.8cm, left: 1.6cm, right: 1.6cm),
    header: context {
      set text(size: 7.5pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto), text(weight: "bold", meta.short-title), cd-join((meta.issuer, meta.date), sep: [ · ]))
      v(-0.4em); line(length: 100%, stroke: 0.4pt + accent.lighten(40%))
    },
    footer: context {
      set text(size: 7.5pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto), if meta.footer-left != none { meta.footer-left } else if meta.classification != none { upper(meta.classification) }, cd-page-of(meta))
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans, size: meta.font-size, number-type: "lining", number-width: "tabular")
  show table: set table(inset: (x: 5pt, y: 3.5pt))
  show table.cell.where(y: 0): set text(fill: accent.darken(30%))
  show heading.where(level: 1): it => block(above: 1.4em, below: 0.6em, sticky: true, text(font: meta.fonts.heading, size: 12pt, weight: "bold", fill: accent.darken(20%), it.body))
  show heading.where(level: 2): it => block(above: 1.1em, below: 0.5em, sticky: true, text(font: meta.fonts.heading, size: 10pt, weight: "bold", it.body))
  block(below: 1em, {
    text(font: meta.fonts.heading, size: 18pt, weight: "bold", meta.title)
    if meta.subtitle != none { linebreak(); text(size: 10pt, fill: muted, meta.subtitle) }
    if meta.summary != none { v(0.4em); text(size: 9.5pt, meta.summary) }
  })
  body
}
