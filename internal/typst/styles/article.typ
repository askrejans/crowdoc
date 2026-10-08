// Article — a classic single-column journal article.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#7a1f1f"))
  let ink = cd-color(meta, "ink", rgb("#1a1a1a"))
  let muted = cd-color(meta, "muted", rgb("#666666"))
  let theme = (
    accent: accent,
    link: accent.darken(10%),
    heading: ink,
    first-line-indent: 1.2em,
    par-spacing: 0.62em,
    leading: 0.6em,
    table-style: "booktabs",
    callout-style: "minimal",
    caption-size: 0.88em,
  )

  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.7cm, bottom: 2.6cm, left: 2.9cm, right: 2.9cm),
    columns: meta.columns,
    header: context {
      if here().page() > 1 {
        set text(size: 8pt, fill: muted)
        let lt = if meta.header-left != none { meta.header-left } else { smallcaps(meta.short-title) }
        let names = cd-names(meta)
        let rt = if meta.header-right != none { meta.header-right }
          else if names.len() > 2 { [#names.first() et al.] }
          else if names.len() > 0 { names.join[ & ] }
        if calc.even(here().page()) { grid(columns: (auto, 1fr), str(counter(page).get().first()), align(end, lt)) }
        else { grid(columns: (1fr, auto), rt, str(counter(page).get().first())) }
      }
    },
    footer: context {
      if here().page() == 1 {
        set text(size: 8pt, fill: muted)
        align(center, str(counter(page).get().first()))
      }
    },
  )

  show: cd-base.with(meta, theme: theme)
  set heading(numbering: if meta.number-sections { "1.1" } else { none })

  show heading.where(level: 1): it => {
    set text(size: 11.5pt, weight: "bold")
    block(above: 1.6em, below: 0.75em, {
      if it.numbering != none { counter(heading).display(it.numbering); h(0.7em) }
      it.body
    })
  }
  show heading.where(level: 2): it => {
    set text(size: 10.5pt, weight: "bold", style: "italic")
    block(above: 1.3em, below: 0.6em, {
      if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
      it.body
    })
  }
  show heading.where(level: 3): it => {
    set text(size: meta.font-size, weight: "regular", style: "italic")
    block(above: 1.1em, below: 0.5em, it.body)
  }
  show heading.where(level: 4): it => block(above: 1em, below: 0.4em, text(size: meta.font-size, style: "italic", it.body))

  // Title block
  block(width: 100%, below: 1.8em, {
    set par(justify: false, first-line-indent: 0pt)
    align(center, {
      cd-balanced(text(size: 17pt, weight: "bold", hyphenate: false, meta.title))
      if meta.subtitle != none {
        v(0.35em)
        text(size: 12pt, style: "italic", meta.subtitle)
      }
      v(1.1em)
      cd-author-block(meta, size: 10.5pt)
      if meta.date != none {
        v(0.6em)
        text(size: 9pt, fill: muted, meta.date)
      }
    })
    if meta.abstract != none {
      v(1.4em)
      pad(x: 2.2em, {
        set text(size: 9.2pt)
        set par(first-line-indent: 0pt, justify: true)
        align(center, text(weight: "bold", size: 9.5pt, meta.terms.at("abstract", default: "Abstract")))
        v(0.2em)
        meta.abstract
        if meta.keywords.len() > 0 {
          v(0.5em)
          cd-keywords-line(meta)
        }
      })
    } else if meta.keywords.len() > 0 {
      pad(x: 2.2em, text(size: 9.2pt, cd-keywords-line(meta)))
    }
    if meta.toc {
      v(1em)
      pad(x: 2.2em, text(size: 9.5pt, outline(depth: 2)))
    }
  })

  body
  if meta.signatures { cd-signatures(parties: meta.parties) }
}
