// Paper — two-column conference paper in the style of engineering
// proceedings: roman-numbered small-caps sections, "Abstract—" lead-in.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#00629b"))
  let theme = (accent: accent, link: accent, heading: cd-color(meta, "heading", rgb("#000000")), first-line-indent: 1em,
    par-spacing: 0.5em, leading: 0.55em, table-style: "booktabs", callout-style: "minimal",
    caption-size: 0.85em, table-size: 0.88em, code-size: 0.82em, list-indent: 0.4em)
  set page(
    paper: meta.paper,
    margin: cd-margins(meta, top: 1.9cm, bottom: 2.5cm, left: 1.6cm, right: 1.6cm),
    columns: 2,
    footer: context { set text(size: 8pt); align(center, str(counter(page).get().first())) },
  )
  set columns(gutter: 0.6cm)
  show: cd-base.with(meta, theme: theme)
  set heading(numbering: if meta.number-sections { "I.A.1)" } else { none })
  show heading.where(level: 1): it => block(above: 1.1em, below: 0.6em, sticky: true, align(center, text(size: meta.font-size, weight: "regular", {
    if it.numbering != none { numbering("I.", counter(heading).get().first()); h(0.5em) }
    smallcaps(it.body)
  })))
  show heading.where(level: 2): it => block(above: 0.9em, below: 0.5em, sticky: true, text(size: meta.font-size, weight: "regular", style: "italic", {
    if it.numbering != none { numbering("A.", counter(heading).get().at(1)); h(0.4em) }
    it.body
  }))
  show heading.where(level: 3): it => block(above: 0.7em, below: 0.4em, sticky: true, text(size: meta.font-size, style: "italic", it.body + [:]))
  show figure: set place(clearance: 1em)
  show figure.caption: set align(start)
  set figure.caption(separator: [. ])
  show figure.where(kind: table): set figure(supplement: upper(meta.terms.at("table", default: "Table")))
  show figure.where(kind: table): set figure(numbering: "I")
  show figure.where(kind: image): set figure(supplement: [Fig.])

  place(top + center, float: true, scope: "parent", clearance: 1.6em, {
    cd-balanced(text(size: 22pt, hyphenate: false, meta.title))
    if meta.subtitle != none { v(0.3em); text(size: 12pt, meta.subtitle) }
    v(1em)
    // One column per author, as in proceedings.
    let auths = meta.authors
    if auths.len() > 0 {
      grid(columns: (1fr,) * calc.min(auths.len(), 4), column-gutter: 1em, row-gutter: 1em,
        ..auths.map(a => align(center, {
          set text(size: 10pt)
          a.name
          for af in a.affiliations { linebreak(); text(size: 9pt, style: "italic", af) }
          if a.email != none { linebreak(); text(size: 8.5pt, raw(a.email)) }
        })))
    }
  })

  if meta.abstract != none {
    set text(size: 9pt, weight: "bold")
    set par(first-line-indent: 0pt)
    [_#meta.terms.at("abstract", default: "Abstract")_—#meta.abstract]
    if meta.keywords.len() > 0 {
      v(0.3em)
      [_#meta.terms.at("keywords", default: "Index Terms")_—#meta.keywords.join(", ")]
    }
    v(0.6em)
  }
  body
}
