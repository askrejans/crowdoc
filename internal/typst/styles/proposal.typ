// Proposal — client-facing business proposals and quotes: split cover with
// "prepared for / prepared by", executive summary box and clear pricing tables.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#c2410c"))
  let ink = cd-color(meta, "ink", rgb("#1c1917"))
  let muted = cd-color(meta, "muted", rgb("#6b6461"))
  let theme = (accent: accent, link: accent, ink: ink, heading: ink, heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.95em, table-style: "striped", table-head-fill: cd-tint(meta, accent, 88%), callout-style: "box", radius: 6pt)
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.4cm, bottom: 2.4cm, left: 2.5cm, right: 2.5cm),
    header: context {
      if not cd-skip-first(meta) {
        set text(size: 8pt, fill: muted, font: meta.fonts.sans)
        grid(columns: (1fr, auto), meta.short-title, meta.issuer)
        v(-0.4em); line(length: 100%, stroke: 0.4pt + accent.lighten(50%))
      }
    },
    footer: context {
      if not cd-skip-first(meta) {
        set text(size: 8pt, fill: muted, font: meta.fonts.sans)
        grid(columns: (1fr, auto), if meta.classification != none { upper(meta.classification) } else { meta.date }, cd-page-of(meta))
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  show heading.where(level: 1): it => block(above: 2em, below: 0.9em, sticky: true, {
    set text(font: meta.fonts.heading, size: 17pt, weight: "bold")
    if it.numbering != none { box(fill: accent, inset: (x: 6pt, y: 3pt), radius: 3pt, text(fill: white, size: 12pt, counter(heading).display("1"))); h(0.6em) }
    it.body
  })
  show heading.where(level: 2): it => block(above: 1.4em, below: 0.6em, sticky: true, text(font: meta.fonts.heading, size: 12.5pt, weight: "bold", it.body))
  show heading.where(level: 3): it => block(above: 1.1em, below: 0.5em, sticky: true, text(font: meta.fonts.heading, size: 10.5pt, weight: "bold", fill: accent.darken(10%), it.body))

  let t = meta.terms
  if meta.title-page {
    page(margin: 0pt, header: none, footer: none, {
      grid(columns: (38%, 62%), rows: 100%,
        block(width: 100%, height: 100%, fill: accent, inset: 1.8cm, {
          set text(fill: white, font: meta.fonts.sans)
          if meta.logo != none { image(meta.logo, width: 3cm) }
          v(1fr)
          set text(size: 9pt)
          let block-label(l) = text(size: 7.5pt, weight: "bold", tracking: 0.12em, upper(l))
          if meta.recipient.len() > 0 or meta.to != none {
            block-label(t.at("prepared-for", default: "Prepared for")); linebreak()
            if meta.to != none { meta.to } else { meta.recipient.join(linebreak()) }
            v(1.2em)
          }
          block-label(t.at("prepared-by", default: "Prepared by")); linebreak()
          cd-join((meta.organization, ..cd-names(meta).map(n => [#n])), sep: linebreak())
          v(1.2em)
          if meta.date != none { block-label(t.at("date", default: "Date")); linebreak(); meta.date }
        }),
        block(width: 100%, height: 100%, inset: (x: 1.8cm, y: 2.4cm), {
          v(1fr)
          text(size: 9pt, weight: "bold", fill: accent, tracking: 0.14em, upper(t.at("proposal", default: "Proposal")))
          v(0.6em)
          text(font: meta.fonts.heading, size: 30pt, weight: "bold", hyphenate: false, meta.title)
          if meta.subtitle != none { v(0.5em); text(size: 14pt, fill: muted, meta.subtitle) }
          v(1fr)
          if meta.version != none or meta.status != none {
            text(size: 8.5pt, fill: muted, cd-join((meta.version, meta.status), sep: [ · ]))
          }
        }),
      )
    })
    counter(page).update(1)
  } else {
    block(below: 1.5em, text(font: meta.fonts.heading, size: 24pt, weight: "bold", meta.title))
  }
  if meta.summary != none or meta.abstract != none {
    callout(kind: "abstract", title: t.at("summary", default: "Executive summary"), if meta.abstract != none { meta.abstract } else { meta.summary })
  }
  if meta.toc { cd-outlines(meta) }
  body
  if meta.signatures {
    cd-signatures(parties: meta.parties, sign-label: t.at("signature", default: "Signature"), date-label: t.at("date", default: "Date"))
  }
}
