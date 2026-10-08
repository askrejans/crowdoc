// Notes — lecture and study notes: numbered theorem, definition and example
// boxes, wide margins for annotations, compact headings.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#2563eb"))
  let ink = cd-color(meta, "ink", rgb("#18181b"))
  let muted = cd-color(meta, "muted", rgb("#71717a"))
  let theme = (accent: accent, link: accent, ink: ink, heading: ink, heading-font: meta.fonts.heading,
    justify: true, par-spacing: 0.75em, table-style: "booktabs", callout-style: "box", radius: 4pt,
    callout: (note: accent, info: rgb("#0891b2"), tip: rgb("#16a34a"), success: rgb("#16a34a"), important: rgb("#7c3aed"),
      warning: rgb("#d97706"), caution: rgb("#ea580c"), danger: rgb("#dc2626"), example: rgb("#475569"), abstract: accent))
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 2.2cm, bottom: 2.2cm, left: 2.2cm, right: 4cm),
    header: context {
      set text(size: 8pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto), meta.short-title, cd-join((cd-names(meta).join(", "), meta.date), sep: [ · ]))
    },
    footer: context { set text(size: 8pt, fill: muted, font: meta.fonts.sans); align(right, str(counter(page).get().first())) },
  )
  show: cd-base.with(meta, theme: theme)
  set heading(numbering: if meta.number-sections { "1.1" } else { none })
  show heading.where(level: 1): it => block(above: 1.8em, below: 0.8em, sticky: true, {
    set text(font: meta.fonts.heading, size: 15pt, weight: "bold")
    if it.numbering != none { box(width: 1.6em, text(fill: accent, counter(heading).display("1"))) }
    it.body
  })
  show heading.where(level: 2): it => block(above: 1.3em, below: 0.6em, sticky: true, text(font: meta.fonts.heading, size: 11.5pt, weight: "bold", {
    if it.numbering != none { text(fill: accent, counter(heading).display(it.numbering)); h(0.5em) }
    it.body
  }))
  show heading.where(level: 3): it => block(above: 1em, below: 0.4em, sticky: true, text(font: meta.fonts.heading, size: meta.font-size, weight: "bold", it.body))
  block(below: 1.4em, {
    text(font: meta.fonts.heading, size: 22pt, weight: "bold", meta.title)
    if meta.subtitle != none { linebreak(); text(size: 12pt, fill: muted, meta.subtitle) }
    v(0.3em)
    line(length: 100%, stroke: 1pt + accent)
  })
  if meta.abstract != none { callout(kind: "abstract", title: meta.terms.at("summary", default: "Summary"), meta.abstract) }
  if meta.toc { cd-outlines(meta) }
  body
}
