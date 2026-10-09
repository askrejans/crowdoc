// Page — the classic printed page: justified serif text with first-line
// indents, a comfortable measure and a centred folio. Prints only what is
// in the document; a title, when there is one, is set modestly.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let ink = cd-color(meta, "ink", rgb("#1c1b19"))
  let muted = cd-color(meta, "muted", rgb("#6b675f"))
  let accent = cd-color(meta, "accent", ink)
  let size = meta.font-size
  let theme = (accent: accent, ink: ink, heading: cd-color(meta, "heading", ink), link: cd-color(meta, "link", ink),
    first-line-indent: 1.25em, par-spacing: 0.68em, leading: 0.68em, table-style: "booktabs", callout-style: "minimal",
    quote-bar: cd-color(meta, "rule", rgb("#cfcac0")), caption-size: 0.9em, footnote-size: 0.82em, hrule: "dots")
  let (pw, ph) = cd-page-dims(meta)
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    columns: meta.columns,
    margin: cd-measure-margins(meta, size * 31, min-side: pw * 0.12, top: ph * 0.11, bottom: ph * 0.13),
    // No running header of its own: only the document's header texts.
    ..(header: if meta.header-left != none or meta.header-right != none {
      set text(size: size * 0.8, fill: muted)
      cd-running(meta.header-left, none, meta.header-right)
    }),
    footer: context {
      set text(size: size * 0.82, fill: muted)
      let folio = if counter(page).final().first() > 1 { str(counter(page).get().first()) }
      cd-running(meta.footer-left, folio, meta.footer-right)
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(number-type: "old-style")
  show table: set text(number-type: "lining")
  set footnote.entry(separator: line(length: 2.4em, stroke: 0.5pt + muted), indent: 0pt)

  let num(it) = if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
  show heading.where(level: 1): it => block(above: 2.1em, below: 0.9em, sticky: true,
    text(size: size * 1.3, weight: "regular", { num(it); it.body }))
  show heading.where(level: 2): it => block(above: 1.7em, below: 0.7em, sticky: true,
    text(size: size, weight: "regular", tracking: 0.06em, { num(it); smallcaps(lower(it.body)) }))
  show heading: it => if it.level > 2 {
    block(above: 1.4em, below: 0.6em, sticky: true, text(size: size, weight: "regular", style: "italic", { num(it); it.body }))
  } else { it }

  // Block quotations: indented both sides, upright, a little smaller.
  show quote.where(block: true): it => pad(x: 1.6em, block(above: 1.1em, below: 1.1em, {
    set text(size: size * 0.94)
    set par(first-line-indent: 0pt)
    it.body
    if it.attribution != none { align(end, text(style: "italic")[— #it.attribution]) }
  }))
  show figure.caption: set text(style: "italic")
  show figure: set block(above: 1.8em, below: 1.8em)

  // Title block: only when the document has a real title.
  let names = cd-names(meta)
  let byline = (if names.len() > 0 { smallcaps(lower(names.join(", "))) }, if meta.date != none { text(style: "italic", meta.date) })
    .filter(x => x != none)
  if meta.title != none {
    align(center, block(below: 2.8em, {
      cd-balanced(text(size: size * 1.55, hyphenate: false, meta.title))
      if meta.subtitle != none { v(0.55em); cd-balanced(text(size: size * 1.06, style: "italic", fill: muted, meta.subtitle)) }
      if byline.len() > 0 { v(1.2em); text(size: size * 0.92, tracking: 0.04em, fill: muted, byline.join(h(1.2em))) }
    }))
  } else if byline.len() > 0 {
    align(center, block(below: 2.2em, text(size: size * 0.92, tracking: 0.04em, fill: muted, byline.join(h(1.2em)))))
  }
  if meta.abstract != none { pad(x: 1.6em, bottom: 1.6em, text(style: "italic", meta.abstract)) }
  if meta.toc { outline(depth: 2); v(1.5em) }
  body
}
