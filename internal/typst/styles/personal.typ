// Personal — a warm, simple letter: sender and date on the right-hand
// axis, the salutation, the text, and the closing and name on the same
// axis. No letterhead, no envelope window, no subject banner.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let ink = cd-color(meta, "ink", rgb("#22201c"))
  let muted = cd-color(meta, "muted", rgb("#6f695f"))
  let size = meta.font-size
  let theme = (ink: ink, heading: cd-color(meta, "heading", ink), link: cd-color(meta, "link", ink), accent: cd-color(meta, "accent", ink),
    justify: false, first-line-indent: 0pt, leading: 0.7em, par-spacing: 1.15em, table-style: "booktabs",
    callout-style: "minimal", hrule: "dots")
  let (pw, ph) = cd-page-dims(meta)
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-measure-margins(meta, size * 32, min-side: pw * 0.12, top: ph * 0.1, bottom: ph * 0.11),
    // No running header of its own: only the document's header texts.
    ..(header: if meta.header-left != none or meta.header-right != none {
      set text(size: size * 0.8, fill: muted)
      cd-running(meta.header-left, none, meta.header-right)
    }),
    footer: context {
      set text(size: size * 0.8, fill: muted)
      let n = counter(page).get().first()
      cd-running(meta.footer-left, if n > 1 { str(n) }, meta.footer-right)
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(costs: (hyphenation: 300%))
  set par(linebreaks: "optimized")
  show heading: it => block(above: 1.5em, below: 0.7em, sticky: true, text(size: size, weight: "regular", style: "italic", it.body))
  show quote.where(block: true): it => pad(x: 1.5em, block(above: 1.1em, below: 1.1em, {
    it.body
    if it.attribution != none { align(end, text(style: "italic")[— #it.attribution]) }
  }))

  // The writer's side of the page: sender, date, closing and name share
  // one axis a little right of centre.
  let axis = 54%
  let side(content) = pad(..if cd-rtl(meta) { (right: axis) } else { (left: axis) }, content)
  let lines(ls) = ls.join(linebreak())
  let name = if meta.from != none { meta.from } else if meta.authors.len() > 0 { meta.authors.first().name }
  let dp = cd-date-place(meta)

  if meta.sender.len() > 0 or dp != none {
    block(width: 100%, below: 2.2em, side({
      set par(spacing: 0.9em)
      if meta.sender.len() > 0 { par(lines(meta.sender)) }
      if dp != none { par(dp) }
    }))
  }
  let recipient = if meta.recipient.len() > 0 { lines(meta.recipient) } else if meta.to != none { meta.to }
  if recipient != none { block(below: 2em, recipient) }
  if meta.title != none { block(below: 1.4em, text(style: "italic", meta.title)) }
  if meta.opening != none { block(below: 1.15em, meta.opening) }

  body

  if meta.closing != none or meta.signatures {
    block(width: 100%, above: 2em, breakable: false, side({
      if meta.closing != none { meta.closing } else { meta.terms.at("closing", default: "Yours,") }
      v(if meta.signatures { 3.6em } else { 2.6em })
      if name != none { name }
    }))
  }
}
