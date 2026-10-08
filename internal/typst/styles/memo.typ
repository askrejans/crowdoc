// Memo — an internal memorandum with a To/From/Date/Subject block.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#b23a48"))
  let muted = cd-color(meta, "muted", rgb("#5f6368"))
  let theme = (accent: accent, link: accent, heading: cd-color(meta, "heading", rgb("#1b1b1f")), heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.85em, callout-style: "bar", table-style: "striped")
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.3cm, bottom: 2.3cm, left: 2.4cm, right: 2.4cm),
    header: context {
      if here().page() > 1 {
        set text(size: 8pt, fill: muted, font: meta.fonts.sans)
        grid(columns: (1fr, auto), meta.short-title, if meta.classification != none { upper(meta.classification) })
      }
    },
    footer: context {
      set text(size: 8pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto), if meta.footer-left != none { meta.footer-left } else if meta.status != none { upper(meta.status) }, cd-page-of(meta))
    },
  )
  show: cd-base.with(meta, theme: theme)
  show heading.where(level: 1): it => block(above: 1.5em, below: 0.6em, sticky: true, text(font: meta.fonts.heading, size: 12pt, weight: "bold", it.body))
  show heading.where(level: 2): it => block(above: 1.2em, below: 0.5em, sticky: true, text(font: meta.fonts.heading, size: 10.5pt, weight: "bold", fill: accent.darken(10%), it.body))
  show heading.where(level: 3): it => block(above: 1em, below: 0.4em, sticky: true, text(font: meta.fonts.heading, weight: "semibold", it.body))

  let t = meta.terms
  grid(columns: (1fr, auto),
    text(font: meta.fonts.heading, size: 24pt, weight: "bold", tracking: 0.12em, fill: accent, upper(t.at("memorandum", default: "Memorandum"))),
    align(right + horizon, {
      if meta.logo != none { image(meta.logo, height: 1.1cm) }
      else if meta.classification != none {
        box(inset: (x: 6pt, y: 3pt), radius: 2pt, stroke: 0.6pt + accent, text(size: 8pt, weight: "bold", fill: accent, tracking: 0.08em, upper(meta.classification)))
      }
    }))
  v(0.6em)
  line(length: 100%, stroke: 1.2pt + accent)
  v(0.5em)
  let names = cd-names(meta)
  let to = if meta.to != none { meta.to } else { meta.subtitle }
  let from = if meta.from != none { meta.from } else if names.len() > 0 { names.join(", ") }
  set text(size: 1em)
  cd-fields((
    (t.at("to", default: "To") + ":", to),
    (t.at("from", default: "From") + ":", from),
    (t.at("cc", default: "CC") + ":", meta.cc),
    (t.at("date", default: "Date") + ":", meta.date),
    (t.at("subject", default: "Subject") + ":", text(weight: "bold", meta.title)),
  ), label-style: (weight: "bold", fill: muted, font: meta.fonts.heading, size: 0.9em), row-gutter: 0.55em)
  v(0.5em)
  line(length: 100%, stroke: 0.5pt + muted.lighten(50%))
  v(0.6em)
  if meta.summary != none {
    block(below: 1em, fill: cd-tint(meta, accent, 93%), inset: 10pt, radius: 3pt, width: 100%, text(weight: "medium", meta.summary))
  }
  body
  if meta.signatures { cd-signatures(parties: meta.parties) }
}
