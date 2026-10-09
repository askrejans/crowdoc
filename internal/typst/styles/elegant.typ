// Elegant — classical book typography: centred small-caps headings,
// old-style figures and ornaments. For essays, stories, speeches and poetry.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#7b2d26"))
  let ink = cd-color(meta, "ink", rgb("#20201c"))
  let muted = cd-color(meta, "muted", rgb("#6d6a63"))
  let theme = (accent: accent, link: accent, ink: ink, heading: ink, first-line-indent: 1.4em,
    par-spacing: 0.55em, leading: 0.7em, table-style: "booktabs", callout-style: "minimal", quote-bar: accent.lighten(50%),
    hrule: "ornament", ornament: [❧])
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 3cm, bottom: 3.2cm, left: 3.2cm, right: 3.2cm),
    header: context {
      if here().page() > 1 {
        set text(size: 8.5pt, fill: muted, tracking: 0.08em)
        align(center, smallcaps(lower(meta.short-title)))
      }
    },
    footer: context { set text(size: 9pt, fill: muted); align(center, [— #counter(page).get().first() —]) },
  )
  show: cd-base.with(meta, theme: theme)
  set text(number-type: "old-style")
  show table: set text(number-type: "lining")
  let ornament = text(fill: accent, size: 11pt, [❧])
  show heading.where(level: 1): it => block(above: 2.2em, below: 1.1em, sticky: true, align(center, {
    set text(size: 12.5pt, weight: "regular", tracking: 0.12em)
    if it.numbering != none { text(fill: accent, numbering("I", ..counter(heading).get())); linebreak() }
    smallcaps(lower(it.body))
  }))
  show heading.where(level: 2): it => block(above: 1.6em, below: 0.7em, sticky: true, align(center, text(size: meta.font-size, style: "italic", it.body)))
  show heading.where(level: 3): it => block(above: 1.2em, below: 0.5em, sticky: true, text(size: meta.font-size, style: "italic", it.body))
  show heading.where(level: 1): set heading(numbering: none)

  align(center, block(below: 2.4em, {
    let names = cd-names(meta)
    if meta.title != none {
      v(1.5cm)
      cd-balanced(text(size: 24pt, weight: "regular", tracking: 0.02em, hyphenate: false, meta.title))
      if meta.subtitle != none { v(0.6em); text(size: 13pt, style: "italic", fill: muted, meta.subtitle) }
      v(1em)
      ornament
      v(0.6em)
    }
    if names.len() > 0 { text(size: 11pt, tracking: 0.1em, smallcaps(lower(names.join(", ")))) }
    if meta.date != none { v(0.3em); text(size: 9.5pt, fill: muted, style: "italic", meta.date) }
  }))
  if meta.abstract != none { pad(x: 2.5em, bottom: 1.5em, text(style: "italic", meta.abstract)) }
  if meta.toc { outline(depth: 2); v(1em) }
  body
}
