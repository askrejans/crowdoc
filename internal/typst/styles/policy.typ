// Policy — controlled corporate documents (policies, procedures, standards)
// with a document-control box and hierarchical clause numbering.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#1e3a5f"))
  let muted = cd-color(meta, "muted", rgb("#5f6368"))
  let theme = (accent: accent, link: accent, heading: cd-color(meta, "heading", rgb("#111111")), heading-font: meta.fonts.heading,
    justify: true, par-spacing: 0.8em, table-style: "grid", table-head-fill: cd-tint(meta, accent, 90%), callout-style: "box")
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 2.8cm, bottom: 2.4cm, left: 2.5cm, right: 2.5cm),
    header: context {
      set text(size: 8pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto, auto), column-gutter: 1.2em,
        text(weight: "bold", fill: accent, meta.short-title),
        if meta.version != none [#meta.terms.at("version", default: "Version") #meta.version],
        if meta.classification != none { upper(meta.classification) })
      v(-0.4em); line(length: 100%, stroke: 0.6pt + accent)
    },
    footer: context {
      set text(size: 8pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto), if meta.footer-left != none { meta.footer-left } else { meta.issuer }, cd-page-of(meta))
    },
  )
  show: cd-base.with(meta, theme: theme)
  set heading(numbering: if meta.number-sections { "1.1.1" } else { none })
  show heading.where(level: 1): it => block(above: 1.8em, below: 0.8em, sticky: true, {
    set text(font: meta.fonts.heading, size: 13pt, weight: "bold", fill: accent)
    if it.numbering != none { counter(heading).display(it.numbering); h(0.7em) }
    upper(it.body)
  })
  show heading.where(level: 2): it => block(above: 1.3em, below: 0.6em, sticky: true, text(font: meta.fonts.heading, size: 11pt, weight: "bold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
    it.body
  }))
  show heading.where(level: 3): it => block(above: 1em, below: 0.5em, sticky: true, text(font: meta.fonts.heading, size: meta.font-size, weight: "bold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.5em) }
    it.body
  }))

  let t = meta.terms
  text(font: meta.fonts.heading, size: 22pt, weight: "bold", meta.title)
  if meta.subtitle != none { linebreak(); text(size: 12pt, fill: muted, meta.subtitle) }
  v(0.8em)
  let x(k) = meta.extra.at(k, default: none)
  table(
    columns: (1fr, 1fr, 1fr, 1fr),
    stroke: 0.5pt + accent.lighten(60%),
    fill: (x, y) => if calc.even(x) { accent.lighten(94%) },
    inset: 6pt,
    text(weight: "bold", t.at("document-number", default: "Document no.")), [#x("document-number")],
    text(weight: "bold", t.at("version", default: "Version")), [#meta.version],
    text(weight: "bold", t.at("effective-date", default: "Effective")), [#x("effective-date")],
    text(weight: "bold", t.at("date", default: "Approved")), [#meta.date],
    text(weight: "bold", t.at("owner", default: "Owner")), [#cd-names(meta).join(", ")],
    text(weight: "bold", t.at("status", default: "Status")), [#meta.status],
  )
  if meta.summary != none { v(0.5em); callout(kind: "info", title: t.at("summary", default: "Purpose"), meta.summary) }
  if meta.toc { v(0.6em); outline(depth: 2) }
  body
}
