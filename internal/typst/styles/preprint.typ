// Preprint — the familiar look of e-print archives: Computer Modern style,
// narrow abstract, "Preprint" marker.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let theme = (accent: rgb("#000000"), link: rgb("#1f3d99"), heading: cd-color(meta, "heading", rgb("#000000")), first-line-indent: 1.5em,
    par-spacing: 0.55em, leading: 0.6em, table-style: "booktabs", callout-style: "minimal")
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 2.8cm, bottom: 2.8cm, left: 3.2cm, right: 3.2cm),
    columns: meta.columns,
    header: context {
      set text(size: 8.5pt, style: "italic")
      if here().page() == 1 {
        align(center, if meta.header-left != none { meta.header-left } else { meta.terms.at("preprint", default: "Preprint") + [. ] + if meta.status != none { meta.status } })
      } else {
        align(center, smallcaps(meta.short-title))
      }
    },
    footer: context { set text(size: 9pt); align(center, str(counter(page).get().first())) },
  )
  show: cd-base.with(meta, theme: theme)
  show heading.where(level: 1): it => block(above: 1.5em, below: 0.8em, sticky: true, text(size: 12pt, weight: "bold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.8em) }
    it.body
  }))
  show heading.where(level: 2): it => block(above: 1.2em, below: 0.6em, sticky: true, text(size: 10.5pt, weight: "bold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.7em) }
    it.body
  }))
  show heading.where(level: 3): it => block(above: 1em, below: 0.5em, sticky: true, text(size: meta.font-size, weight: "bold", it.body))

  align(center, block(below: 1.5em, {
    v(0.5em)
    cd-balanced(text(size: 17pt, weight: "bold", hyphenate: false, meta.title))
    if meta.subtitle != none { v(0.3em); text(size: 12pt, meta.subtitle) }
    v(1.4em)
    cd-author-block(meta, size: 10.5pt)
    if meta.date != none { v(0.8em); meta.date }
  }))
  if meta.abstract != none {
    pad(x: 3em, bottom: 1.2em, {
      align(center, text(size: 9.5pt, weight: "bold", meta.terms.at("abstract", default: "Abstract")))
      v(-0.2em)
      set text(size: 9.5pt)
      set par(first-line-indent: 0pt)
      meta.abstract
      if meta.keywords.len() > 0 { v(0.4em); cd-keywords-line(meta) }
    })
  }
  if meta.toc { outline(depth: 2); v(1em) }
  body
}
