// APA — manuscripts following the APA Publication Manual (7th edition):
// title page, double spacing, 0.5 in paragraph indents, APA heading levels
// and a References page with hanging indents.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let theme = (accent: rgb("#000000"), link: rgb("#000000"), heading: cd-color(meta, "heading", rgb("#000000")), first-line-indent: 0.5in,
    par-spacing: if meta.leading != none { meta.leading } else { 1.4em }, leading: if meta.leading != none { meta.leading } else { 1.4em },
    justify: false, table-style: "booktabs", callout-style: "minimal", caption-size: 1em, footnote-size: 0.9em)
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 1in, bottom: 1in, left: 1in, right: 1in),
    header: context {
      set text(size: meta.font-size)
      grid(columns: (1fr, auto),
        if meta.header-left != none { meta.header-left } else if meta.extra.at("running-head", default: none) != none { upper(meta.extra.at("running-head")) },
        str(counter(page).get().first()))
    },
    header-ascent: 0.5in - 0.3em,
  )
  show: cd-base.with(meta, theme: theme)
  set heading(numbering: none)
  show heading.where(level: 1): it => block(above: 1.4em, below: 1.4em, sticky: true, align(center, text(size: meta.font-size, weight: "bold", it.body)))
  show heading.where(level: 2): it => block(above: 1.4em, below: 1.4em, sticky: true, text(size: meta.font-size, weight: "bold", it.body))
  show heading.where(level: 3): it => block(above: 1.4em, below: 1.4em, sticky: true, text(size: meta.font-size, weight: "bold", style: "italic", it.body))
  show heading.where(level: 4): it => box(text(size: meta.font-size, weight: "bold", it.body + [. ]))
  show heading.where(level: 5): it => box(text(size: meta.font-size, weight: "bold", style: "italic", it.body + [. ]))
  // APA: table/figure number on its own line in bold, title in italic, above.
  set figure.caption(position: top, separator: [])
  show figure.caption: it => align(left, {
    text(weight: "bold")[#it.supplement #context it.counter.display(it.numbering)]
    linebreak()
    emph(it.body)
  })
  show figure: set align(left)
  set quote(block: true)
  show quote.where(block: true): it => pad(left: 0.5in, it.body)

  let t = meta.terms
  // Title page (student papers may also start with the title only)
  if meta.title-page { page({
    set par(first-line-indent: 0pt, justify: false)
    v(3 * 1.4em + 2em)
    align(center, {
      text(weight: "bold", meta.title)
      if meta.subtitle != none { [: ]; text(weight: "bold", meta.subtitle) }
      v(2em)
      cd-names(meta).join(", ", last: if cd-names(meta).len() > 2 { ", and " } else { " and " })
      linebreak()
      let affs = meta.authors.map(a => a.affiliations).flatten().dedup()
      if affs.len() > 0 { affs.join(linebreak()); linebreak() }
      for k in ("course", "instructor", "due-date") {
        let v = meta.extra.at(k, default: none)
        if v != none { v; linebreak() }
      }
      if meta.date != none { meta.date }
    })
    let note = meta.extra.at("author-note", default: none)
    if note != none {
      v(1fr)
      align(center, text(weight: "bold", t.at("author-note", default: "Author Note")))
      note
    }
  }) }
  if meta.abstract != none {
    page({
      align(center, text(weight: "bold", t.at("abstract", default: "Abstract")))
      set par(first-line-indent: 0pt)
      meta.abstract
      if meta.keywords.len() > 0 {
        v(0.4em)
        par(first-line-indent: 0.5in)[#emph(t.at("keywords", default: "Keywords") + [:]) #meta.keywords.join(", ")]
      }
    })
  }
  align(center, text(weight: "bold", meta.title))
  body
}
