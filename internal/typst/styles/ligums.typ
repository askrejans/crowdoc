// Līgums — Latvian agreements and contracts: sober, monochrome, with place
// and date line and a two-party signature table.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let ink = cd-color(meta, "ink", rgb("#111111"))
  let muted = cd-color(meta, "muted", rgb("#555555"))
  let theme = (
    accent: ink,
    link: ink,
    heading: ink,
    par-spacing: 0.8em,
    leading: 0.66em,
    table-style: "grid",
    callout-style: "box",
    callout: (note: ink, info: ink, tip: ink, success: ink, important: ink, warning: ink, caution: ink, danger: ink, example: ink, abstract: ink),
  )

  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.5cm, bottom: 2.5cm, left: 3cm, right: 2cm),
    header: context {
      if not cd-skip-first(meta) {
        set text(size: 8pt, fill: muted)
        grid(columns: (1fr, auto),
          if meta.header-left != none { meta.header-left } else { meta.short-title },
          if meta.header-right != none { meta.header-right })
      }
    },
    footer: context {
      set text(size: 8pt, fill: muted)
      align(center, cd-page-of(meta))
    },
  )

  show: cd-base.with(meta, theme: theme)
  set heading(numbering: if meta.number-sections { "1.1." } else { none })

  show heading.where(level: 1): it => block(above: 1.6em, below: 0.8em, sticky: true, align(center, {
    set text(size: 10.5pt, weight: "bold")
    if it.numbering != none { counter(heading).display(it.numbering); h(0.4em) }
    upper(it.body)
  }))
  show heading.where(level: 2): it => block(above: 1.1em, below: 0.5em, sticky: true, text(weight: "bold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.4em) }
    it.body
  }))

  if meta.title-page {
    page(header: none, footer: none, {
      v(1fr)
      align(center, {
        cd-balanced(text(size: 20pt, weight: "bold", tracking: 0.04em, hyphenate: false, upper(meta.title)))
        if meta.subtitle != none { v(0.8em); text(size: 12pt, meta.subtitle) }
        if meta.summary != none { v(2em); block(width: 80%, text(style: "italic", meta.summary)) }
      })
      v(1.4fr)
      align(center, text(size: 9.5pt, fill: muted, cd-join((meta.issuer, meta.date), sep: [ · ])))
    })
    counter(page).update(1)
  }

  align(center, block(below: 1.2em, {
    cd-balanced(text(size: 14pt, weight: "bold", tracking: 0.05em, hyphenate: false, upper(meta.title)))
    if meta.subtitle != none and not meta.title-page { v(0.3em); text(meta.subtitle) }
  }))
  if meta.place != none or meta.date != none {
    block(below: 1.4em, grid(columns: (1fr, 1fr),
      align(left, if meta.place != none { meta.place }),
      align(right, if meta.date != none { meta.date })))
  }

  body

  if meta.signatures {
    let t = meta.terms
    let ps = if meta.parties.len() > 0 { meta.parties } else { ([], []) }
    v(2em)
    block(breakable: false, {
      align(center, text(weight: "bold", upper(t.at("signature", default: "Paraksti"))))
      v(1em)
      grid(
        columns: (1fr,) * calc.min(ps.len(), 2),
        column-gutter: 2.5em,
        row-gutter: 2.4em,
        ..ps.map(p => {
          if p != [] { text(weight: "bold", p) }
          v(3em)
          line(length: 100%, stroke: 0.5pt + ink)
          v(0.2em)
          text(size: 8.5pt, fill: muted, t.at("signature", default: "Paraksts"))
        }),
      )
    })
  }
}
