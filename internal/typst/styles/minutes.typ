// Minutes — meeting minutes with a meeting details block; checkbox lists
// become action items.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#4338ca"))
  let muted = cd-color(meta, "muted", rgb("#6b7280"))
  let theme = (accent: accent, link: accent, heading: cd-color(meta, "heading", rgb("#111827")), heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.8em, table-style: "grid", table-head-fill: cd-tint(meta, accent, 90%), callout-style: "bar")
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 2.2cm, bottom: 2.2cm, left: 2.3cm, right: 2.3cm),
    footer: context {
      set text(size: 8pt, fill: muted, font: meta.fonts.sans)
      grid(columns: (1fr, auto), meta.short-title, cd-page-of(meta))
    },
  )
  show: cd-base.with(meta, theme: theme)
  set heading(numbering: if meta.number-sections { "1." } else { none })
  show heading.where(level: 1): it => block(above: 1.5em, below: 0.6em, sticky: true, {
    set text(font: meta.fonts.heading, size: 12pt, weight: "bold")
    if it.numbering != none { text(fill: accent, counter(heading).display(it.numbering)); h(0.5em) }
    it.body
  })
  show heading.where(level: 2): it => block(above: 1.1em, below: 0.4em, sticky: true, text(font: meta.fonts.heading, size: meta.font-size, weight: "bold", fill: accent.darken(10%), it.body))

  let t = meta.terms
  text(size: 9pt, weight: "bold", tracking: 0.14em, fill: accent, upper(t.at("minutes", default: "Minutes")))
  v(0.2em)
  text(font: meta.fonts.heading, size: 20pt, weight: "bold", meta.title)
  if meta.subtitle != none { linebreak(); text(size: 11pt, fill: muted, meta.subtitle) }
  v(0.6em)
  let attendees = meta.extra.at("attendees", default: none)
  let absent = meta.extra.at("absent", default: none)
  let chair = meta.extra.at("chair", default: none)
  let secretary = meta.extra.at("secretary", default: none)
  block(width: 100%, inset: 10pt, radius: 4pt, fill: cd-tint(meta, accent, 95%), stroke: 0.5pt + accent.lighten(70%), {
    set text(size: 9pt)
    cd-fields((
      (t.at("date", default: "Date"), meta.date),
      (t.at("place", default: "Place"), cd-join((meta.place, meta.location), sep: [, ])),
      (t.at("chair", default: "Chair"), chair),
      (t.at("secretary", default: "Minutes"), secretary),
      (t.at("attendees", default: "Attendees"), if attendees != none { attendees } else { cd-names(meta).join(", ") }),
      (t.at("absent", default: "Absent"), absent),
    ), label-style: (weight: "bold", fill: muted), row-gutter: 0.5em)
  })
  v(0.6em)
  body
}
