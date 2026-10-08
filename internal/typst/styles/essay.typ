// Essay — humanities papers in the MLA manner: heading block at top left,
// centred title, double spacing, surname and page number top right.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let names = meta.authors.map(a => a.name)
  let surname = if meta.author-names.len() > 0 { meta.author-names.first().split(" ").last() }
  let theme = (accent: rgb("#000000"), link: rgb("#000000"), heading: cd-color(meta, "heading", rgb("#000000")), first-line-indent: 0.5in,
    justify: false, leading: if meta.leading != none { meta.leading } else { 1.4em }, par-spacing: if meta.leading != none { meta.leading } else { 1.4em },
    table-style: "booktabs", callout-style: "minimal")
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 1in, bottom: 1in, left: 1in, right: 1in),
    header: context { set text(size: meta.font-size); align(right, [#surname #counter(page).get().first()]) },
    header-ascent: 0.5in - 0.3em,
  )
  show: cd-base.with(meta, theme: theme)
  set heading(numbering: none)
  show heading: it => block(above: 1.4em, below: 1.4em, sticky: true, align(center, text(size: meta.font-size, weight: "bold", it.body)))
  {
    set par(first-line-indent: 0pt)
    if names.len() > 0 { names.join(", "); linebreak() }
    for k in ("instructor", "course") {
      let v = meta.extra.at(k, default: none)
      if v != none { v; linebreak() }
    }
    if meta.date != none { meta.date }
    parbreak()
    align(center, { meta.title; if meta.subtitle != none [: #meta.subtitle] })
  }
  body
}
