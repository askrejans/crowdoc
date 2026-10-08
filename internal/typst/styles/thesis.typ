// Thesis — bachelor's, master's and doctoral theses: institutional title
// page, roman-numbered front matter, chapters, chapter-based numbering.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#1a1a1a"))
  let muted = cd-color(meta, "muted", rgb("#555555"))
  let theme = (accent: accent, link: rgb("#1a1a1a"), heading: cd-color(meta, "heading", rgb("#111111")), first-line-indent: 1.25em,
    par-spacing: 0.65em, leading: if meta.leading != none { meta.leading } else { 0.95em },
    table-style: "booktabs", callout-style: "minimal")
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 2.5cm, bottom: 2.5cm, left: 3.5cm, right: 2cm),
    numbering: "i",
    footer: context { set text(size: 9.5pt); align(center, counter(page).display()) },
  )
  show: cd-base.with(meta, theme: theme)
  set heading(numbering: if meta.number-sections { "1.1" } else { none })
  show heading.where(level: 1): it => {
    pagebreak(weak: true)
    block(above: 0pt, below: 1.6em, {
      v(2cm)
      set text(size: 18pt, weight: "bold")
      if it.numbering != none {
        text(size: 11pt, weight: "regular", fill: muted, tracking: 0.1em, upper(meta.terms.at("chapter", default: "Chapter")) + [ ] + counter(heading).display("1"))
        linebreak()
        v(0.3em)
      }
      it.body
    })
  }
  show heading.where(level: 2): it => block(above: 1.5em, below: 0.7em, sticky: true, text(size: 12.5pt, weight: "bold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.7em) }
    it.body
  }))
  show heading.where(level: 3): it => block(above: 1.2em, below: 0.6em, sticky: true, text(size: 11pt, weight: "bold", {
    if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
    it.body
  }))
  show heading.where(level: 4): it => block(above: 1em, below: 0.4em, sticky: true, text(size: meta.font-size, weight: "bold", style: "italic", it.body))
  show outline.entry.where(level: 1): it => { v(0.6em, weak: true); strong(it) }
  show: cd-chapter-numbering

  let t = meta.terms
  // Title page
  page(numbering: none, footer: none, {
    set par(justify: false, first-line-indent: 0pt)
    align(center, {
      set text(size: 12pt)
      if meta.logo != none { image(meta.logo, height: 2.2cm); v(0.6em) }
      if meta.institution != none { text(weight: "bold", upper(meta.institution)); linebreak() }
      if meta.faculty != none { meta.faculty; linebreak() }
      if meta.department != none { meta.department }
    })
    v(1fr)
    align(center, {
      let names = cd-names(meta)
      if names.len() > 0 { text(size: 13pt, names.join(", ")); v(1.4em) }
      cd-balanced(text(size: 21pt, weight: "bold", hyphenate: false, upper(meta.title)))
      if meta.subtitle != none { v(0.6em); text(size: 14pt, meta.subtitle) }
      v(1.4em)
      text(size: 12pt, if meta.degree != none { meta.degree } else if meta.doc-type != "" { upper(meta.doc-type.first()) + meta.doc-type.slice(1) } else { t.at("thesis", default: "Thesis") })
    })
    v(1fr)
    if meta.supervisor != none {
      align(right, block(width: 60%, align(left, {
        set text(size: 11pt)
        text(weight: "bold", t.at("supervisor", default: "Supervisor") + [:]); linebreak()
        meta.supervisor
      })))
    }
    v(1.5cm)
    align(center, text(size: 12pt, cd-join((meta.location, if meta.date != none { meta.date }), sep: [ ])))
  })
  counter(page).update(1)
  if meta.abstract != none {
    heading(level: 1, numbering: none, outlined: false, t.at("abstract", default: "Abstract"))
    meta.abstract
    if meta.keywords.len() > 0 { v(1em); set par(first-line-indent: 0pt); cd-keywords-line(meta) }
  }
  if meta.toc {
    pagebreak(weak: true)
    outline(depth: 3)
  }
  if meta.lof { pagebreak(weak: true); outline(title: t.at("list-of-figures", default: "List of Figures"), target: figure.where(kind: image)) }
  if meta.lot { pagebreak(weak: true); outline(title: t.at("list-of-tables", default: "List of Tables"), target: figure.where(kind: table)) }
  pagebreak(weak: true)
  set page(numbering: "1")
  counter(page).update(1)
  body
}
