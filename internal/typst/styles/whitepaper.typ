// Whitepaper — authoritative long-form thought leadership with a bold
// cover band, large lead paragraph and accent section markers.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#0b5cad"))
  let ink = cd-color(meta, "ink", rgb("#13171f"))
  let muted = cd-color(meta, "muted", rgb("#5b6475"))
  let theme = (accent: accent, link: accent, ink: ink, heading: ink, heading-font: meta.fonts.heading,
    justify: true, par-spacing: 0.95em, leading: 0.64em, table-style: "striped", table-head-fill: cd-tint(meta, accent, 88%),
    callout-style: "bar", radius: 3pt)
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.5cm, bottom: 2.4cm, left: 2.6cm, right: 2.6cm),
    columns: meta.columns,
    header: context {
      if not cd-skip-first(meta) {
        set text(size: 7.5pt, fill: muted, font: meta.fonts.sans, tracking: 0.08em)
        grid(columns: (1fr, auto), upper(meta.short-title), if meta.organization != none { upper(meta.organization) })
      }
    },
    footer: context {
      if not cd-skip-first(meta) {
        set text(size: 8pt, fill: muted, font: meta.fonts.sans)
        grid(columns: (auto, 1fr), box(fill: accent, inset: (x: 5pt, y: 2pt), text(fill: white, weight: "bold", str(counter(page).get().first()))), h(1fr))
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  show heading.where(level: 1): it => block(above: 2.2em, below: 1em, sticky: true, {
    set text(font: meta.fonts.heading, size: 18pt, weight: "bold")
    block(width: 1.2cm, height: 3pt, fill: accent)
    v(0.3em)
    if it.numbering != none { text(fill: accent, counter(heading).display(it.numbering)); h(0.5em) }
    it.body
  })
  show heading.where(level: 2): it => block(above: 1.5em, below: 0.6em, sticky: true, text(font: meta.fonts.heading, size: 12.5pt, weight: "bold", fill: accent.darken(15%), it.body))
  show heading.where(level: 3): it => block(above: 1.2em, below: 0.5em, sticky: true, text(font: meta.fonts.heading, size: 10.5pt, weight: "bold", it.body))

  if meta.title-page {
    page(margin: 0pt, header: none, footer: none, {
      block(width: 100%, height: 58%, fill: accent, inset: (x: 2.6cm, top: 2.4cm, bottom: 2cm), {
        set text(fill: white, font: meta.fonts.sans)
        grid(columns: (1fr, auto),
          text(size: 9pt, weight: "bold", tracking: 0.12em, upper(meta.terms.at("whitepaper", default: "White paper"))),
          if meta.logo != none { image(meta.logo, height: 1.2cm) })
        v(1fr)
        text(font: meta.fonts.heading, size: 34pt, weight: "bold", hyphenate: false, meta.title)
        if meta.subtitle != none { v(0.6em); text(size: 15pt, fill: white.darken(8%), meta.subtitle) }
      })
      block(inset: (x: 2.6cm, top: 1.4cm), {
        if meta.summary != none { block(width: 92%, text(size: 13pt, fill: cd-tint(meta, ink, 10%), meta.summary)); v(1.2cm) }
        set text(size: 9pt, font: meta.fonts.sans)
        cd-fields((
          (meta.terms.at("author", default: "Author"), cd-names(meta).join(", ")),
          (meta.terms.at("date", default: "Date"), meta.date),
          (meta.terms.at("version", default: "Version"), meta.version),
        ), label-style: (fill: muted, weight: "bold", tracking: 0.06em))
      })
    })
    counter(page).update(1)
  } else {
    block(below: 1.6em, { text(font: meta.fonts.heading, size: 26pt, weight: "bold", fill: accent, meta.title); if meta.subtitle != none { linebreak(); text(size: 13pt, fill: muted, meta.subtitle) } })
  }
  if meta.abstract != none { block(below: 1.5em, text(size: 12pt, meta.abstract)) }
  if meta.toc { cd-outlines(meta, pagebreak-after: meta.title-page) }
  body
}
