// Formal — a block-format business letter without window-envelope
// geometry: sender block, date, recipient, subject, salutation, text,
// closing and signature, all flush with the text edge.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let ink = cd-color(meta, "ink", rgb("#1c1e22"))
  let muted = cd-color(meta, "muted", rgb("#63676e"))
  let accent = cd-color(meta, "accent", rgb("#2b4a6b"))
  let size = meta.font-size
  let theme = (accent: accent, ink: ink, heading: cd-color(meta, "heading", ink), link: cd-color(meta, "link", accent),
    heading-font: meta.fonts.sans, justify: false, first-line-indent: 0pt, leading: 0.66em, par-spacing: 1.3em,
    table-style: "booktabs", callout-style: "bar")
  let (pw, ph) = cd-page-dims(meta)
  let recipient = if meta.recipient.len() > 0 { meta.recipient } else if meta.to != none { (meta.to,) } else { () }
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-measure-margins(meta, size * 40, min-side: calc.min(2.5cm, pw * 0.12), top: calc.min(2.2cm, ph * 0.08), bottom: calc.min(2.5cm, ph * 0.09)),
    // Continuation pages: addressee, date and page, as typed on a second sheet.
    header: context {
      if here().page() > 1 {
        set text(font: meta.fonts.sans, size: size * 0.78, fill: muted, number-type: "lining")
        let first = if meta.header-left != none { meta.header-left } else if recipient.len() > 0 { recipient.first() }
        let last = if meta.header-right != none { meta.header-right } else { cd-join((meta.date, cd-page-of(meta)), sep: h(0.6em) + sym.dot.c + h(0.6em)) }
        cd-running(first, none, last)
      }
    },
    // No footer of its own: only the document's footer texts.
    ..(footer: if meta.footer-left != none or meta.footer-right != none {
      set text(font: meta.fonts.sans, size: size * 0.75, fill: muted)
      cd-running(meta.footer-left, none, meta.footer-right)
    }),
  )
  show: cd-base.with(meta, theme: theme)
  set par(linebreaks: "optimized")
  show heading: it => block(above: 1.3em, below: 0.55em, sticky: true, text(font: meta.fonts.sans, size: size, weight: "semibold", it.body))

  let lines(ls) = ls.join(linebreak())
  // Sender block: only what the document gives.
  if meta.organization != none or meta.sender.len() > 0 or meta.logo != none {
    block(below: 2.4em, {
      grid(columns: (1fr, auto), column-gutter: 1.5em,
        {
          set text(font: meta.fonts.sans)
          set par(leading: 0.5em)
          // The organisation heads the block; without one, the first line
          // of the sender's address (conventionally the name) does.
          let (head, rest) = if meta.organization != none { (meta.organization, meta.sender) }
            else if meta.sender.len() > 0 { (meta.sender.first(), meta.sender.slice(1)) } else { (none, ()) }
          if head != none {
            block(below: 0.5em, text(size: size * 1.2, weight: "semibold", fill: cd-color(meta, "heading", ink), head))
          }
          if rest.len() > 0 { text(size: size * 0.8, fill: muted, rest.join(h(0.5em) + sym.dot.c + h(0.5em))) }
        },
        if meta.logo != none { align(end + horizon, image(meta.logo, height: 1.2cm, alt: "logo")) },
      )
      v(0.7em)
      line(length: 100%, stroke: 0.5pt + accent)
    })
  }

  let dp = cd-date-place(meta)
  if dp != none { block(below: 1.8em, dp) }
  if recipient.len() > 0 { block(below: 2em, { set par(leading: 0.55em); lines(recipient) }) }
  if meta.title != none { block(below: 1.5em, text(weight: "bold", meta.title)) }
  if meta.opening != none { block(below: 1.05em, meta.opening) }

  body

  let name = if meta.from != none { meta.from } else if meta.authors.len() > 0 { meta.authors.first().name }
  let role = if meta.authors.len() > 0 and meta.authors.first().affiliations.len() > 0 { meta.authors.first().affiliations.first() }
  if meta.closing != none or meta.signatures {
    block(above: 1.8em, breakable: false, {
      if meta.closing != none { meta.closing } else { meta.terms.at("closing", default: "Sincerely,") }
      v(3.4em)
      if meta.signatures { line(length: 5.5cm, stroke: 0.5pt + ink); v(0.3em) }
      if name != none { block(below: 0.3em, name) }
      if role != none { text(size: size * 0.9, fill: muted, role) }
    })
  }
}
