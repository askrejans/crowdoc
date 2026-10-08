// Report — a professional report with a cover page.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#1f4e79"))
  let theme = (
    accent: accent,
    link: accent,
    heading: cd-color(meta, "heading", rgb("#14213d")),
    heading-font: meta.fonts.heading,
    table-style: "booktabs",
    callout-style: "bar",
  )
  let muted = cd-color(meta, "muted", rgb("#5f6368"))
  let margins = (
    top: if meta.margins.top != auto { meta.margins.top } else { 2.6cm },
    bottom: if meta.margins.bottom != auto { meta.margins.bottom } else { 2.4cm },
    left: if meta.margins.left != auto { meta.margins.left } else { 2.5cm },
    right: if meta.margins.right != auto { meta.margins.right } else { 2.5cm },
  )

  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: margins,
    columns: meta.columns,
    header: context {
      if here().page() > 1 or not meta.title-page {
        set text(size: 8.5pt, fill: muted, font: meta.fonts.sans)
        grid(
          columns: (1fr, auto),
          if meta.header-left != none { meta.header-left } else { meta.short-title },
          if meta.header-right != none { meta.header-right }
          else if meta.classification != none { upper(meta.classification) }
          else if meta.organization != none { meta.organization },
        )
        v(-0.45em)
        line(length: 100%, stroke: 0.4pt + muted.lighten(50%))
      }
    },
    footer: context {
      if here().page() > 1 or not meta.title-page {
        set text(size: 8.5pt, fill: muted, font: meta.fonts.sans)
        grid(
          columns: (1fr, auto, 1fr),
          align(left, if meta.footer-left != none { meta.footer-left } else if meta.status != none { upper(meta.status) }),
          cd-page-of(meta),
          align(right, if meta.footer-right != none { meta.footer-right } else if meta.date != none { meta.date }),
        )
      }
    },
  )

  show: cd-base.with(meta, theme: theme)

  show heading.where(level: 1): it => {
    set text(size: 15pt, font: meta.fonts.heading, weight: "semibold")
    block(above: 1.9em, below: 0.9em, {
      if it.numbering != none {
        text(fill: accent)[#counter(heading).display(it.numbering)]
        h(0.6em)
      }
      it.body
    })
  }
  show heading.where(level: 2): it => {
    set text(size: 12pt, font: meta.fonts.heading, weight: "semibold")
    block(above: 1.5em, below: 0.7em, {
      if it.numbering != none {
        text(fill: accent)[#counter(heading).display(it.numbering)]
        h(0.5em)
      }
      it.body
    })
  }
  show heading.where(level: 3): it => {
    set text(size: 10.5pt, font: meta.fonts.heading, weight: "semibold")
    block(above: 1.3em, below: 0.6em, it)
  }
  show heading.where(level: 4): it => block(above: 1.1em, below: 0.5em, text(size: meta.font-size, style: "italic", weight: "regular", it.body))

  // Cover page
  if meta.title-page {
    page(margin: (top: 3cm, bottom: 2.5cm, left: 2.8cm, right: 2.8cm), header: none, footer: none, columns: 1, {
      set par(justify: false)
      grid(
        columns: (1fr, auto),
        align(left + horizon, {
          set text(font: meta.fonts.sans, size: 9pt, fill: muted, tracking: 0.06em)
          if meta.organization != none { upper(meta.organization) }
        }),
        if meta.logo != none { image(meta.logo, height: 1.4cm) },
      )
      v(1fr)
      block(width: 100%, {
        line(length: 3.2cm, stroke: 2.5pt + accent)
        v(1.1em)
        text(font: meta.fonts.heading, size: 30pt, weight: "bold", fill: cd-color(meta, "heading", rgb("#14213d")), hyphenate: false, meta.title)
        if meta.subtitle != none {
          v(0.6em)
          text(font: meta.fonts.heading, size: 15pt, fill: muted, meta.subtitle)
        }
        if meta.summary != none {
          v(1.6em)
          block(width: 85%, text(size: 11pt, fill: rgb("#3c4043"), meta.summary))
        }
      })
      v(1.2fr)
      set text(font: meta.fonts.sans, size: 9pt)
      let field(label, value) = if value != none {
        (text(fill: muted, tracking: 0.04em, upper(label)), value)
      } else { () }
      let t = meta.terms
      let names = meta.authors.map(a => a.name)
      grid(
        columns: (auto, 1fr),
        column-gutter: 1.6em,
        row-gutter: 0.75em,
        ..field(t.at("prepared-by", default: "Prepared by"), if names.len() > 0 { names.join(", ") } else { none }),
        ..field(t.at("date", default: "Date"), meta.date),
        ..field(t.at("version", default: "Version"), meta.version),
        ..field(t.at("status", default: "Status"), meta.status),
        ..field(t.at("classification", default: "Classification"), meta.classification),
      )
    })
    counter(page).update(1)
  } else {
    block(below: 1.6em, {
      text(font: meta.fonts.heading, size: 22pt, weight: "bold", fill: cd-color(meta, "heading", rgb("#14213d")), meta.title)
      if meta.subtitle != none { linebreak(); text(size: 13pt, fill: muted, meta.subtitle) }
      let bits = (meta.authors.map(a => a.name).join(", "), meta.date).filter(x => x != none and x != "")
      if bits.len() > 0 { v(0.4em); text(size: 9.5pt, fill: muted, bits.join[ · ]) }
    })
  }

  if meta.abstract != none {
    callout(kind: "abstract", title: meta.terms.at("abstract", default: "Abstract"), meta.abstract)
  }
  if meta.toc or meta.lof or meta.lot {
    cd-outlines(meta, pagebreak-after: meta.title-page)
  }
  body

  if meta.signatures { cd-signatures(parties: meta.parties) }
}
