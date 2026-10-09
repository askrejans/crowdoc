// Modern — contemporary sans-serif layout with bold headlines and a
// strong accent colour. For articles, guides, briefings and portfolios.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#6a00ff"))
  let ink = cd-color(meta, "ink", rgb("#101522"))
  let muted = cd-color(meta, "muted", rgb("#5b6475"))
  let theme = (accent: accent, link: accent, ink: ink, heading: ink, heading-font: meta.fonts.heading,
    justify: false, par-spacing: 1em, leading: 0.66em, table-style: "striped", table-head-fill: cd-tint(meta, accent, 90%),
    code-bg: rgb("#f3f2f8"), callout-style: "box", radius: 8pt)
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.4cm, bottom: 2.4cm, left: 2.4cm, right: 2.4cm),
    columns: meta.columns,
    header: context {
      if here().page() > 1 {
        set text(size: 8pt, fill: muted, font: meta.fonts.sans, weight: "medium")
        grid(columns: (1fr, auto), meta.short-title, box(width: 0.6em, height: 0.6em, fill: accent, radius: 50%))
      }
    },
    footer: context {
      set text(size: 8pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto), if meta.footer-left != none { meta.footer-left } else { meta.issuer }, str(counter(page).get().first()))
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans)
  show heading.where(level: 1): it => block(above: 2em, below: 0.8em, sticky: true, {
    set text(font: meta.fonts.heading, size: 19pt, weight: "bold", tracking: -0.01em)
    if it.numbering != none { text(fill: accent, counter(heading).display("01")); h(0.5em) }
    it.body
  })
  show heading.where(level: 2): it => block(above: 1.5em, below: 0.6em, sticky: true, text(font: meta.fonts.heading, size: 13.5pt, weight: "bold", it.body))
  show heading.where(level: 3): it => block(above: 1.2em, below: 0.5em, sticky: true, text(font: meta.fonts.heading, size: 11pt, weight: "bold", fill: accent, it.body))

  block(below: 2em, {
    if meta.title != none {
      block(width: 2.2cm, height: 0.35cm, fill: accent)
      v(0.8em)
      text(font: meta.fonts.heading, size: 34pt, weight: "bold", tracking: -0.02em, hyphenate: false, meta.title)
    }
    if meta.subtitle != none { v(0.4em); text(size: 15pt, fill: muted, meta.subtitle) }
    if meta.summary != none { v(1em); block(width: 90%, text(size: 12pt, fill: cd-tint(meta, ink, 10%), meta.summary)) }
    let bits = (cd-names(meta).join(", "), meta.date).filter(x => x != none and x != "")
    if bits.len() > 0 { v(1em); text(size: 9pt, weight: "medium", fill: muted, upper(bits.join[ · ])) }
  })
  if meta.abstract != none { callout(kind: "abstract", title: none, meta.abstract) }
  if meta.toc { cd-outlines(meta) }
  body
}
