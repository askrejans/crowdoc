// Letter — business correspondence. On A4 the recipient address sits in
// the DIN 5008 window position, so the letter fits standard window envelopes.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#24425f"))
  let muted = cd-color(meta, "muted", rgb("#5f6368"))
  let theme = (accent: accent, link: accent, heading: cd-color(meta, "heading", rgb("#1b1b1f")), heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.95em, leading: 0.62em, callout-style: "bar")
  let a4 = meta.paper == "a4"

  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 2.2cm, bottom: 2.4cm, left: if a4 { 2.5cm } else { 2.54cm }, right: 2cm),
    footer: context {
      set text(size: 7.5pt, fill: muted, font: meta.fonts.sans)
      line(length: 100%, stroke: 0.4pt + muted.lighten(60%))
      grid(columns: (1fr, auto, 1fr),
        align(left, if meta.footer-left != none { meta.footer-left } else if meta.organization != none { meta.organization }),
        if counter(page).final().first() > 1 { cd-page-of(meta) },
        align(right, if meta.footer-right != none { meta.footer-right }))
    },
  )
  show: cd-base.with(meta, theme: theme)
  show heading: it => block(above: 1.2em, below: 0.5em, sticky: true, text(size: meta.font-size, weight: "bold", font: meta.fonts.heading, it.body))

  let sender-name = if meta.organization != none { meta.organization } else if meta.authors.len() > 0 { meta.authors.first().name }
  let recipient = if meta.recipient.len() > 0 { meta.recipient.join(linebreak()) }
    else if meta.to != none { meta.to }
    else if meta.subtitle != none { meta.subtitle }
  let letterhead = grid(
    columns: (1fr, auto),
    align(left + top, {
      if meta.logo != none { image(meta.logo, height: 1.3cm); v(0.3em) }
      if sender-name != none { text(font: meta.fonts.heading, size: 13pt, weight: "bold", fill: accent, sender-name) }
    }),
    align(right + top, {
      set text(size: 8.5pt, fill: muted, font: meta.fonts.sans)
      set par(justify: false, leading: 0.5em)
      meta.sender.join(linebreak())
    }),
  )
  let address = {
    if sender-name != none {
      let ret = (sender-name, ..meta.sender.slice(0, calc.min(1, meta.sender.len())))
      text(size: 6.5pt, fill: muted, underline(stroke: 0.3pt, ret.join[ · ]))
      v(0.3em)
    }
    set par(justify: false, leading: 0.55em)
    if recipient != none { recipient }
  }
  if a4 {
    // DIN 5008 form B: the window starts 45 mm from the top and 20 mm from
    // the left edge; the letter text starts at about 98 mm.
    context {
      let mt = page.margin.top
      let ml = page.margin.left
      place(top + left, letterhead)
      place(top + left, dx: 2cm - ml, dy: 4.5cm - mt + 0.27cm, block(width: 8.5cm, height: 4.5cm, address))
      v(9.8cm - mt)
    }
  } else {
    letterhead
    v(1.2cm)
    address
    v(1cm)
  }

  // Date line and subject
  let dp = cd-date-place(meta)
  if dp != none { align(right, dp); v(0.8em) }
  block(below: 1.2em, text(weight: "bold", font: meta.fonts.heading, meta.title))
  if meta.opening != none { block(below: 0.9em, meta.opening) }

  body

  let closing = meta.closing
  if closing != none or meta.signatures {
    v(1em)
    block(breakable: false, {
      if closing != none { closing } else { meta.terms.at("closing", default: "Sincerely,") }
      v(2.2em)
      let names = cd-names(meta)
      if names.len() > 0 { names.join(linebreak()) }
    })
  }
}
