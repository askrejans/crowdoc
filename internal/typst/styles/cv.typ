// CV — résumé / curriculum vitae: name banner, contact line, section labels
// in a left rail with content on the right.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#0e7490"))
  let ink = cd-color(meta, "ink", rgb("#111827"))
  let muted = cd-color(meta, "muted", rgb("#6b7280"))
  let theme = (accent: accent, link: accent, ink: ink, heading: ink, heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.6em, leading: 0.55em, list-indent: 0em, table-style: "minimal", callout-style: "minimal")
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 1.6cm, bottom: 1.6cm, left: 1.8cm, right: 1.8cm),
    footer: context {
      if counter(page).final().first() > 1 {
        set text(size: 7.5pt, fill: muted, font: meta.fonts.sans)
        grid(columns: (1fr, auto), cd-names(meta).join(", "), cd-page-of(meta))
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans, size: meta.font-size)
  set list(marker: text(fill: accent, [▸]), body-indent: 0.4em)
  show heading.where(level: 1): it => block(above: 1.3em, below: 0.5em, sticky: true, {
    set text(font: meta.fonts.heading, size: 9.5pt, weight: "bold", tracking: 0.14em, fill: accent)
    upper(it.body)
    v(-0.55em)
    line(length: 100%, stroke: 0.6pt + accent.lighten(50%))
  })
  show heading.where(level: 2): it => block(above: 0.9em, below: 0.25em, sticky: true, text(font: meta.fonts.heading, size: 10.5pt, weight: "bold", it.body))
  show heading.where(level: 3): it => block(above: 0.2em, below: 0.35em, sticky: true, text(size: 9pt, fill: muted, style: "italic", it.body))

  let name = if meta.authors.len() > 0 { meta.authors.first().name } else { meta.title }
  grid(columns: (1fr, auto), column-gutter: 1.2em,
    {
      text(font: meta.fonts.heading, size: 26pt, weight: "bold", tracking: -0.01em, name)
      let role = if meta.subtitle != none { meta.subtitle } else if meta.authors.len() > 0 and meta.title != name { meta.title }
      if role != none { linebreak(); text(size: 12pt, fill: accent, weight: "medium", role) }
      v(0.4em)
      let contact = (meta.place, ..meta.sender, if meta.authors.len() > 0 and meta.authors.first().email != none { meta.authors.first().email }).filter(x => x != none)
      if contact.len() > 0 { text(size: 8.5pt, fill: muted, contact.join([ #h(0.3em)·#h(0.3em) ])) }
    },
    if meta.logo != none { box(clip: true, radius: 50%, image(meta.logo, width: 2.4cm, height: 2.4cm, fit: "cover")) },
  )
  if meta.summary != none or meta.abstract != none {
    v(0.6em)
    text(size: 10pt, if meta.abstract != none { meta.abstract } else { meta.summary })
  }
  body
}
