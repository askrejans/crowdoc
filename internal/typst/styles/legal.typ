// Legal — contracts, agreements, NDAs and policies.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#8a6d2f"))
  let ink = cd-color(meta, "ink", rgb("#16161d"))
  let muted = cd-color(meta, "muted", rgb("#6b6b70"))
  let theme = (
    accent: accent,
    link: ink,
    heading: ink,
    justify: true,
    par-spacing: 0.85em,
    leading: 0.68em,
    table-style: "grid",
    callout-style: "box",
    quote-bar: accent.lighten(40%),
  )

  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 3cm, bottom: 2.8cm, left: 3cm, right: 2.6cm),
    header: context {
      if not cd-skip-first(meta) {
        set text(size: 8pt, fill: muted, tracking: 0.04em)
        grid(
          columns: (1fr, auto),
          if meta.header-left != none { meta.header-left } else { smallcaps(meta.short-title) },
          if meta.header-right != none { meta.header-right } else if meta.classification != none { upper(meta.classification) },
        )
      }
    },
    footer: context {
      set text(size: 8pt, fill: muted)
      grid(
        columns: (1fr, auto, 1fr),
        align(left, if meta.footer-left != none { meta.footer-left } else if meta.version != none [v#meta.version]),
        { line(length: 2cm, stroke: 0.4pt + accent); v(-0.4em); cd-page-of(meta) },
        align(right, if meta.footer-right != none { meta.footer-right }),
      )
    },
  )

  show: cd-base.with(meta, theme: theme)
  set heading(numbering: if meta.number-sections { "1.1." } else { none })

  show heading.where(level: 1): it => {
    set text(size: 10.5pt, weight: "bold", tracking: 0.05em)
    block(above: 1.8em, below: 0.9em, sticky: true, {
      if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
      upper(it.body)
    })
  }
  show heading.where(level: 2): it => {
    set text(size: meta.font-size, weight: "bold")
    block(above: 1.2em, below: 0.6em, sticky: true, {
      if it.numbering != none { counter(heading).display(it.numbering); h(0.5em) }
      it.body
    })
  }
  show heading.where(level: 3): it => block(above: 1em, below: 0.5em, sticky: true, text(size: meta.font-size, style: "italic", it.body))

  // Title
  if meta.title-page {
    page(header: none, footer: none, {
      v(1fr)
      align(center, {
        if meta.logo != none { image(meta.logo, height: 1.3cm); v(1.5em) }
        line(length: 3cm, stroke: 0.8pt + accent)
        v(1.2em)
        cd-balanced(text(size: 22pt, weight: "bold", tracking: 0.03em, hyphenate: false, upper(meta.title)))
        if meta.subtitle != none { v(0.8em); text(size: 13pt, style: "italic", meta.subtitle) }
        v(1.2em)
        line(length: 3cm, stroke: 0.8pt + accent)
        if meta.summary != none {
          v(2em)
          block(width: 78%, text(size: 10pt, style: "italic", fill: muted, meta.summary))
        }
      })
      v(1.4fr)
      set text(size: 9pt)
      let t = meta.terms
      align(center, cd-fields((
        (t.at("date", default: "Date"), meta.date),
        (t.at("version", default: "Version"), meta.version),
        (t.at("status", default: "Status"), meta.status),
        (t.at("classification", default: "Classification"), meta.classification),
      ), label-style: (fill: muted, tracking: 0.04em)))
    })
    counter(page).update(1)
  } else {
    align(center, block(below: 1.8em, {
      cd-balanced(text(size: 16pt, weight: "bold", tracking: 0.04em, hyphenate: false, upper(meta.title)))
      if meta.subtitle != none { v(0.4em); text(style: "italic", meta.subtitle) }
      let dp = cd-date-place(meta)
      if dp != none { v(0.6em); text(size: 9.5pt, fill: muted, dp) }
      v(0.8em)
      line(length: 2.5cm, stroke: 0.6pt + accent)
    }))
  }

  if meta.parties.len() > 0 {
    let t = meta.terms
    block(below: 1.4em, {
      text(weight: "bold", upper(t.at("parties", default: "Parties")))
      v(0.4em)
      enum(numbering: "1.", ..meta.parties)
    })
  }

  if meta.toc { outline(depth: 2); v(1em) }
  body

  if meta.signatures {
    let t = meta.terms
    cd-signatures(
      parties: if meta.parties.len() > 0 { meta.parties } else { () },
      sign-label: t.at("signature", default: "Signature"),
      date-label: t.at("date", default: "Date"),
    )
  }
}
