// Slides — 16:9 presentation: a title slide, then one slide per top-level
// heading (second-level headings start further slides).
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#6a00ff"))
  let ink = cd-color(meta, "ink", rgb("#101522"))
  let muted = cd-color(meta, "muted", rgb("#5b6475"))
  let theme = (accent: accent, link: accent, ink: ink, heading: ink, heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.8em, leading: 0.6em, table-style: "striped", callout-style: "box", radius: 8pt,
    code-size: 0.8em)
  set page(
    width: 33.867cm, height: 19.05cm,
    margin: (x: 2cm, top: 1.6cm, bottom: 1.4cm),
    footer: context {
      if here().page() > 1 {
        set text(size: 10pt, fill: muted, font: meta.fonts.sans)
        grid(columns: (1fr, auto), meta.short-title, [#counter(page).get().first() / #counter(page).final().first()])
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans, size: 20pt)
  set heading(numbering: none)
  set list(marker: text(fill: accent, [•]))
  show heading.where(level: 1): it => {
    pagebreak(weak: true)
    block(below: 0.8em, text(font: meta.fonts.heading, size: 34pt, weight: "bold", it.body))
  }
  show heading.where(level: 2): it => {
    pagebreak(weak: true)
    block(below: 0.6em, text(font: meta.fonts.heading, size: 28pt, weight: "bold", it.body))
  }
  show heading.where(level: 3): it => block(above: 0.8em, below: 0.4em, text(font: meta.fonts.heading, size: 22pt, weight: "bold", fill: accent, it.body))
  show figure: set image(height: 11cm)
  // Title slide
  if meta.title != none { page(footer: none, fill: ink, {
    set text(fill: white)
    v(1fr)
    block(width: 3cm, height: 0.35cm, fill: accent)
    v(0.6em)
    text(font: meta.fonts.heading, size: 48pt, weight: "bold", hyphenate: false, meta.title)
    if meta.subtitle != none { v(0.3em); text(size: 24pt, fill: white.darken(20%), meta.subtitle) }
    v(1fr)
    text(size: 16pt, fill: white.darken(25%), cd-join((cd-names(meta).join(", "), meta.date), sep: [ · ]))
  }) }
  body
}
