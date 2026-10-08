// Newsletter — masthead with issue line and a multi-column body.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#b91c1c"))
  let ink = cd-color(meta, "ink", rgb("#171717"))
  let muted = cd-color(meta, "muted", rgb("#737373"))
  let theme = (accent: accent, link: accent, ink: ink, heading: ink, heading-font: meta.fonts.heading,
    justify: true, par-spacing: 0.6em, first-line-indent: 1em, leading: 0.58em, table-style: "booktabs",
    callout-style: "box", caption-size: 0.82em)
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 1.6cm, bottom: 1.8cm, left: 1.6cm, right: 1.6cm),
    footer: context {
      set text(size: 7.5pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto), cd-join((meta.issuer, meta.date), sep: [ · ]), str(counter(page).get().first()))
    },
  )
  show: cd-base.with(meta, theme: theme)
  show heading.where(level: 1): it => block(above: 1.2em, below: 0.5em, sticky: true, {
    set par(justify: false, first-line-indent: 0pt)
    text(font: meta.fonts.heading, size: 15pt, weight: "bold", hyphenate: false, it.body)
  })
  show heading.where(level: 2): it => block(above: 1em, below: 0.4em, sticky: true, text(font: meta.fonts.heading, size: 11pt, weight: "bold", fill: accent, it.body))
  show heading.where(level: 3): it => block(above: 0.8em, below: 0.3em, sticky: true, text(font: meta.fonts.heading, size: meta.font-size, weight: "bold", it.body))
  // Masthead
  block(width: 100%, below: 0.8em, {
    line(length: 100%, stroke: 2.5pt + ink)
    v(0.4em)
    grid(columns: (1fr, auto),
      text(font: meta.fonts.heading, size: 40pt, weight: "black", tracking: -0.02em, hyphenate: false, meta.title),
      align(right + bottom, if meta.logo != none { image(meta.logo, height: 1.4cm) }))
    v(0.1em)
    line(length: 100%, stroke: 0.6pt + ink)
    v(-0.5em)
    set text(size: 7.5pt, font: meta.fonts.sans, weight: "medium", tracking: 0.08em)
    set par(justify: false)
    grid(columns: (2fr, auto, 1fr), column-gutter: 1em,
      align(left, upper(if meta.subtitle != none { meta.subtitle } else { meta.issuer })),
      upper(if meta.version != none { meta.version }),
      align(right, upper(if meta.date != none { meta.date })))
    v(-0.5em)
    line(length: 100%, stroke: 0.6pt + ink)
  })
  if meta.summary != none { block(below: 0.8em, text(size: 12pt, style: "italic", meta.summary)) }
  columns(if meta.columns > 1 { meta.columns } else { 3 }, gutter: 1em, body)
}
