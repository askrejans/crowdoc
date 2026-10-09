// Brief — a dense one- or two-page fact sheet / executive brief in two
// columns under a full-width title band.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#0f766e"))
  let ink = cd-color(meta, "ink", rgb("#111827"))
  let muted = cd-color(meta, "muted", rgb("#6b7280"))
  let theme = (accent: accent, link: accent, ink: ink, heading: accent.darken(20%), heading-font: meta.fonts.heading,
    justify: true, par-spacing: 0.7em, leading: 0.58em, table-style: "striped", table-size: 0.88em,
    callout-style: "box", caption-size: 0.82em, code-size: 0.8em)
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 1.6cm, bottom: 1.6cm, left: 1.6cm, right: 1.6cm),
    footer: context {
      set text(size: 7.5pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto), cd-join((meta.issuer, meta.date), sep: [ · ]), cd-page-of(meta))
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans, size: meta.font-size)
  show heading.where(level: 1): it => block(above: 1.2em, below: 0.5em, sticky: true, {
    set text(font: meta.fonts.heading, size: 11pt, weight: "bold", fill: accent.darken(15%))
    upper(it.body)
    v(-0.5em); line(length: 100%, stroke: 0.8pt + accent)
  })
  show heading.where(level: 2): it => block(above: 1em, below: 0.4em, sticky: true, text(font: meta.fonts.heading, size: 10pt, weight: "bold", it.body))
  show heading.where(level: 3): it => block(above: 0.8em, below: 0.3em, sticky: true, text(font: meta.fonts.heading, size: meta.font-size, weight: "bold", fill: muted, it.body))

  if meta.title != none or meta.logo != none { block(width: 100%, fill: accent, inset: (x: 14pt, y: 12pt), radius: 4pt, {
    set text(fill: white, font: meta.fonts.sans)
    grid(columns: (1fr, auto), align(horizon, {
      text(font: meta.fonts.heading, size: 20pt, weight: "bold", hyphenate: false, meta.title)
      if meta.subtitle != none { linebreak(); text(size: 10.5pt, meta.subtitle) }
    }), if meta.logo != none { image(meta.logo, height: 1.1cm) })
  }) }
  if meta.summary != none or meta.abstract != none {
    v(0.6em)
    block(text(size: 10.5pt, weight: "medium", if meta.abstract != none { meta.abstract } else { meta.summary }))
  }
  v(0.4em)
  columns(if meta.columns > 1 { meta.columns } else { 2 }, gutter: 1.2em, body)
}
