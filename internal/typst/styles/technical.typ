// Technical — specifications, API guides and engineering documentation.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#0969da"))
  let ink = cd-color(meta, "ink", rgb("#1f2328"))
  let muted = cd-color(meta, "muted", rgb("#59636e"))
  let theme = (
    accent: accent, link: accent, ink: ink, heading: ink, heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.9em, leading: 0.62em,
    code-bg: rgb("#f6f8fa"), code-border: 0.5pt + rgb("#d1d9e0"), code-size: 0.84em,
    table-style: "grid", table-head-fill: rgb("#f6f8fa"), rule: rgb("#d1d9e0"),
    callout-style: "bar", radius: 6pt,
  )
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.4cm, bottom: 2.2cm, left: 2.2cm, right: 2.2cm),
    columns: meta.columns,
    header: context {
      if not cd-skip-first(meta) {
        set text(size: 8pt, fill: muted, font: meta.fonts.sans)
        grid(columns: (1fr, auto),
          if meta.header-left != none { meta.header-left } else { meta.short-title },
          if meta.header-right != none { meta.header-right } else if meta.version != none [v#meta.version])
        v(-0.4em)
        line(length: 100%, stroke: 0.4pt + rgb("#d1d9e0"))
      }
    },
    footer: context {
      if not cd-skip-first(meta) {
        set text(size: 8pt, fill: muted, font: meta.fonts.sans)
        grid(columns: (1fr, auto, 1fr),
          align(left, if meta.footer-left != none { meta.footer-left } else if meta.classification != none { upper(meta.classification) }),
          cd-page-of(meta),
          align(right, if meta.footer-right != none { meta.footer-right } else if meta.status != none { upper(meta.status) }))
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans, size: meta.font-size)
  show heading.where(level: 1): it => {
    block(above: 2em, below: 0.9em, sticky: true, {
      set text(font: meta.fonts.heading, size: 16pt, weight: "semibold")
      if it.numbering != none { text(fill: accent, counter(heading).display(it.numbering)); h(0.6em) }
      it.body
      v(-0.35em)
      line(length: 100%, stroke: 0.6pt + rgb("#d1d9e0"))
    })
  }
  show heading.where(level: 2): it => block(above: 1.5em, below: 0.6em, sticky: true, {
    set text(font: meta.fonts.heading, size: 12.5pt, weight: "semibold")
    if it.numbering != none { text(fill: accent, counter(heading).display(it.numbering)); h(0.5em) }
    it.body
  })
  show heading.where(level: 3): it => block(above: 1.2em, below: 0.5em, sticky: true, text(font: meta.fonts.heading, size: 10.5pt, weight: "semibold", it.body))
  show heading.where(level: 4): it => block(above: 1em, below: 0.4em, sticky: true, text(font: meta.fonts.heading, size: meta.font-size, weight: "semibold", fill: muted, it.body))

  if meta.title-page {
    page(header: none, footer: none, margin: (x: 2.4cm, top: 2.6cm, bottom: 2.2cm), {
      set text(font: meta.fonts.sans)
      grid(columns: (1fr, auto),
        if meta.organization != none { text(size: 9pt, weight: "semibold", fill: muted, meta.organization) },
        if meta.logo != none { image(meta.logo, height: 1.2cm) })
      v(1fr)
      box(fill: accent, inset: (x: 6pt, y: 3pt), radius: 3pt, text(size: 8pt, weight: "bold", fill: white, tracking: 0.08em,
        upper(if meta.doc-type != "" { meta.doc-type } else { meta.terms.at("technical", default: "Documentation") })))
      v(0.8em)
      text(font: meta.fonts.heading, size: 30pt, weight: "bold", fill: ink, hyphenate: false, meta.title)
      if meta.subtitle != none { v(0.5em); text(size: 14pt, fill: muted, meta.subtitle) }
      if meta.summary != none { v(1.4em); block(width: 88%, text(size: 10.5pt, fill: cd-tint(meta, ink, 15%), meta.summary)) }
      v(1.3fr)
      let t = meta.terms
      block(width: 100%, inset: 12pt, radius: 6pt, fill: rgb("#f6f8fa"), stroke: 0.5pt + rgb("#d1d9e0"), {
        set text(size: 8.5pt)
        grid(columns: (1fr, 1fr, 1fr, 1fr), column-gutter: 1em,
          ..(
            (t.at("version", default: "Version"), meta.version),
            (t.at("status", default: "Status"), meta.status),
            (t.at("date", default: "Date"), meta.date),
            (t.at("author", default: "Author"), cd-names(meta).join(", ")),
          ).map(((l, v)) => [#text(fill: muted, size: 7.5pt, tracking: 0.05em, upper(l))\ #if v != none and v != "" { text(weight: "medium", v) } else [—]]))
      })
    })
    counter(page).update(1)
  } else {
    block(below: 1.5em, {
      text(font: meta.fonts.heading, size: 22pt, weight: "bold", meta.title)
      if meta.subtitle != none { linebreak(); text(size: 12pt, fill: muted, meta.subtitle) }
    })
  }
  if meta.abstract != none { callout(kind: "abstract", title: meta.terms.at("abstract", default: "Abstract"), meta.abstract) }
  if meta.toc or meta.lof or meta.lot { cd-outlines(meta, pagebreak-after: meta.title-page) }
  body
}
