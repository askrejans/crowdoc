// Invoice — billing documents with an issuer header, invoice details and
// emphasised line-item tables.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#0f766e"))
  let muted = cd-color(meta, "muted", rgb("#5f6368"))
  let theme = (accent: accent, link: accent, heading: cd-color(meta, "heading", rgb("#1b1b1f")), heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.7em, table-style: "striped", table-head-fill: cd-tint(meta, accent, 88%),
    table-size: 0.95em, callout-style: "box")
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2cm, bottom: 2.2cm, left: 2.2cm, right: 2.2cm),
    footer: context {
      set text(size: 7.5pt, fill: muted, font: meta.fonts.sans)
      line(length: 100%, stroke: 0.4pt + muted.lighten(60%))
      grid(columns: (1fr, auto, 1fr),
        align(left, if meta.footer-left != none { meta.footer-left } else { meta.issuer }),
        cd-page-of(meta),
        align(right, if meta.footer-right != none { meta.footer-right }))
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans)
  show heading: it => block(above: 1.3em, below: 0.5em, sticky: true,
    text(size: 8pt, weight: "bold", tracking: 0.1em, fill: accent, upper(it.body)))
  // Totals rows (a row whose first cell is bold) stand out.
  show table: set table(stroke: (x, y) => if y == 0 { (bottom: 0.8pt + accent) } else { (bottom: 0.3pt + luma(220)) })

  let t = meta.terms
  grid(
    columns: (1fr, 6.5cm),
    column-gutter: 1.5em,
    align(left + top, {
      if meta.logo != none { image(meta.logo, height: 1.4cm); v(0.4em) }
      text(size: 15pt, weight: "bold", meta.issuer)
      if meta.subtitle != none { linebreak(); text(size: 8.5pt, fill: muted, meta.subtitle) }
      if meta.sender.len() > 0 { linebreak(); text(size: 8.5pt, fill: muted, meta.sender.join(linebreak())) }
    }),
    align(right + top, {
      text(size: 26pt, weight: "bold", fill: accent, tracking: 0.04em, upper(if meta.title != [] { meta.title } else { t.at("invoice", default: "Invoice") }))
      v(0.3em)
      set text(size: 9pt)
      align(right, cd-fields((
        (t.at("invoice-number", default: "No."), if meta.version != none { text(weight: "bold", meta.version) }),
        (t.at("issue-date", default: "Date"), meta.date),
        (t.at("status", default: "Status"), if meta.status != none { text(weight: "bold", fill: accent, meta.status) }),
      ), label-style: (fill: muted), gutter: 1em, row-gutter: 0.45em, compact: true))
    }),
  )
  v(0.8em)
  line(length: 100%, stroke: 1.5pt + accent)
  v(0.4em)
  if meta.recipient.len() > 0 {
    block(above: 1em, {
      text(size: 8pt, weight: "bold", tracking: 0.1em, fill: accent, upper(t.at("bill-to", default: "Bill to")))
      linebreak()
      meta.recipient.join(linebreak())
    })
  }
  body
  if meta.summary != none {
    v(1em)
    block(width: 100%, inset: 10pt, radius: 3pt, stroke: 0.6pt + accent.lighten(40%), fill: cd-tint(meta, accent, 95%), text(size: 9pt, meta.summary))
  }
  let legal = (meta.jurisdiction, meta.vat-breakdown, meta.legal-notice).filter(x => x != none)
  if legal.len() > 0 {
    v(1em)
    set text(size: 8pt, fill: muted)
    legal.join(linebreak())
  }
}
