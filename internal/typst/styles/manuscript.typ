// Manuscript — submission-ready manuscript for peer review: double spacing,
// continuous line numbers, ragged right, 12 pt.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let theme = (accent: rgb("#000000"), link: rgb("#000000"), heading: cd-color(meta, "heading", rgb("#000000")), first-line-indent: 1.2em,
    justify: false, leading: 1.3em, par-spacing: 1.3em, table-style: "booktabs", callout-style: "minimal")
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 2.5cm, bottom: 2.5cm, left: 3cm, right: 2.5cm),
    footer: context { set text(size: 10pt); align(center, cd-page-of(meta)) },
  )
  set par.line(numbering: n => text(size: 7.5pt, fill: luma(120), str(n)), number-clearance: 1.2em)
  show: cd-base.with(meta, theme: theme)
  show heading.where(level: 1): it => block(above: 1.6em, below: 1em, sticky: true, text(size: meta.font-size, weight: "bold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
    upper(it.body)
  }))
  show heading.where(level: 2): it => block(above: 1.4em, below: 0.8em, sticky: true, text(size: meta.font-size, weight: "bold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
    it.body
  }))
  show heading.where(level: 3): it => block(above: 1.2em, below: 0.6em, sticky: true, text(size: meta.font-size, style: "italic", it.body))

  block(below: 2em, {
    set par(first-line-indent: 0pt)
    text(size: 14pt, weight: "bold", meta.title)
    if meta.subtitle != none { linebreak(); text(size: 12pt, meta.subtitle) }
    v(1em)
    cd-author-block(meta, size: 11pt, align-to: left)
    if meta.date != none { v(0.5em); meta.date }
  })
  if meta.abstract != none {
    text(weight: "bold", meta.terms.at("abstract", default: "Abstract"))
    parbreak()
    meta.abstract
    if meta.keywords.len() > 0 { parbreak(); cd-keywords-line(meta) }
  }
  body
}
