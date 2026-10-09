// Large print — accessibility first: large sans-serif type, high contrast,
// generous leading and spacing, flush-left lines without hyphenation or
// justification, and nothing set smaller than 14 pt. Prints only what is in
// the document.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let ink = cd-color(meta, "ink", rgb("#000000"))
  let accent = cd-color(meta, "accent", ink)
  // Large print stays large: the body never drops below 16 pt and no other
  // text below 14 pt, whatever size the document asks for.
  let size = calc.max(meta.font-size, 16pt)
  let small = calc.max(14pt, size * 0.86)
  let lead = if meta.leading != none { calc.max(meta.leading, 0.8em) } else { 0.82em }
  let theme = (accent: accent, ink: ink, heading: cd-color(meta, "heading", ink), muted: ink, link: cd-color(meta, "link", ink),
    link-underline: true, heading-font: meta.fonts.heading, justify: false, first-line-indent: 0pt, leading: lead,
    par-spacing: lead + 0.85em, table-style: "grid", rule: cd-color(meta, "ink", ink), callout-style: "box",
    caption-size: small, footnote-size: small, table-size: small, code-size: small, list-indent: 0.2em, radius: 4pt)
  let (pw, ph) = cd-page-dims(meta)
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    columns: meta.columns,
    margin: cd-measure-margins(meta, size * 36, min-side: calc.min(2cm, pw * 0.1), top: 2cm, bottom: 2.4cm),
    // No running header of its own: only the document's header texts.
    ..(header: if meta.header-left != none or meta.header-right != none {
      set text(font: meta.fonts.sans, size: small, fill: ink)
      cd-running(meta.header-left, none, meta.header-right)
    }),
    footer: context {
      set text(font: meta.fonts.sans, size: small, fill: ink, number-type: "lining")
      let folio = if counter(page).final().first() > 1 { cd-page-of(meta) }
      cd-running(meta.footer-left, folio, meta.footer-right)
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans, size: size, hyphenate: false, number-type: "lining")
  // No hyphenation at all, not even the soft hyphens added for languages
  // the engine cannot hyphenate.
  show "\u{ad}": none
  set par(linebreaks: "optimized")
  set super(typographic: false, size: 0.8em)
  show emph: set text(weight: "medium")
  show strong: set text(weight: "bold")
  set list(marker: ([•], [–], [•]), body-indent: 0.6em, spacing: lead + 0.5em)
  set enum(body-indent: 0.6em, spacing: lead + 0.5em)
  set table(inset: (x: 0.55em, y: 0.45em), stroke: 0.75pt + ink)
  show table.cell.where(y: 0): set text(weight: "bold")
  set footnote.entry(separator: line(length: 4em, stroke: 1pt + ink), gap: 0.6em)
  show figure: set block(above: 1.8em, below: 1.8em)

  let num(it) = if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
  show heading: set text(font: meta.fonts.heading, weight: "bold", hyphenate: false)
  show heading: set par(leading: 0.55em)
  show heading.where(level: 1): it => block(above: 2em, below: 0.9em, sticky: true,
    text(size: size * 1.6, { num(it); it.body }))
  show heading.where(level: 2): it => block(above: 1.8em, below: 0.75em, sticky: true,
    text(size: size * 1.3, { num(it); it.body }))
  show heading: it => if it.level > 2 {
    block(above: 1.6em, below: 0.6em, sticky: true, text(size: size * 1.1, { num(it); it.body }))
  } else { it }

  show quote.where(block: true): it => block(above: 1.4em, below: 1.4em, inset: (left: 1em, y: 0.2em),
    stroke: (left: 3pt + accent), {
      it.body
      if it.attribution != none { block(above: 0.6em)[— #it.attribution] }
    })

  // Title block: only when the document has a real title.
  let byline = (cd-names(meta).join(", "), meta.date).filter(x => x != none)
  if meta.title != none {
    block(below: 2em, {
      set par(leading: 0.5em, spacing: 0.95em)
      par(text(font: meta.fonts.heading, size: size * 1.9, weight: "bold", hyphenate: false, meta.title))
      if meta.subtitle != none { par(text(font: meta.fonts.heading, size: size * 1.25, meta.subtitle)) }
      if byline.len() > 0 { block(above: 1.2em, byline.join(linebreak())) }
    })
  } else if byline.len() > 0 {
    block(below: 1.6em, byline.join(linebreak()))
  }
  if meta.abstract != none { block(below: 1.6em, meta.abstract) }
  if meta.toc { outline(depth: 2); v(1.5em) }
  body
}
