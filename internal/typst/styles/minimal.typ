// Minimal — quiet, generous typography for notes, essays and drafts.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#444444"))
  let muted = cd-color(meta, "muted", rgb("#777777"))
  let theme = (accent: accent, link: accent.darken(10%), heading: cd-color(meta, "heading", rgb("#111111")), link-underline: true,
    first-line-indent: 0pt, par-spacing: 1em, leading: 0.68em, table-style: "minimal", callout-style: "minimal")
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 3cm, bottom: 3cm, left: 3.4cm, right: 3.4cm),
    columns: meta.columns,
    footer: context { set text(size: 8.5pt, fill: muted); align(center, str(counter(page).get().first())) },
  )
  show: cd-base.with(meta, theme: theme)
  show heading.where(level: 1): it => block(above: 1.8em, below: 0.8em, sticky: true, text(size: 13pt, weight: "semibold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
    it.body
  }))
  show heading.where(level: 2): it => block(above: 1.4em, below: 0.6em, sticky: true, text(size: 11pt, weight: "semibold", it.body))
  show heading.where(level: 3): it => block(above: 1.2em, below: 0.5em, sticky: true, text(size: meta.font-size, style: "italic", it.body))

  block(below: 2em, {
    cd-balanced(text(size: 18pt, weight: "semibold", hyphenate: false, meta.title), align-to: left)
    if meta.subtitle != none { v(0.2em); text(size: 12pt, fill: muted, meta.subtitle) }
    let bits = (cd-names(meta).join(", "), meta.date).filter(x => x != none and x != "")
    if bits.len() > 0 { v(0.6em); text(size: 9.5pt, fill: muted, bits.join[ · ]) }
  })
  if meta.abstract != none { block(below: 1.5em, text(style: "italic", meta.abstract)) }
  if meta.toc { outline(depth: 2); v(1em) }
  body
}
