// Magazine — a feature spread: display headline, standfirst and byline
// across the page, then two justified columns opening with a drop cap,
// small-caps section heads and pull quotes.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#a3242f"))
  let ink = cd-color(meta, "ink", rgb("#1c1a17"))
  let muted = cd-color(meta, "muted", rgb("#6f6a63"))
  let rule = cd-color(meta, "rule", rgb("#cfc9c0"))
  let display = cd-display-font(meta)
  let sans = meta.fonts.sans
  let cols = calc.max(meta.columns, 1)
  let theme = (accent: accent, link: accent, ink: ink, heading: ink, heading-font: display,
    justify: true, first-line-indent: 1em, par-spacing: 0.62em, leading: 0.62em, table-style: "booktabs",
    table-size: 0.86em, callout-style: "minimal", caption-size: 0.84em, code-size: 0.8em,
    hrule: "ornament", ornament: cd-lozenges(accent))
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.3cm, bottom: 2.3cm, left: 1.9cm, right: 1.9cm),
    header: context {
      if here().page() > 1 {
        set text(font: sans, size: 7.5pt, fill: muted, tracking: 0.08em)
        let left-bit = if meta.header-left != none { meta.header-left } else if meta.short-title != none { upper(meta.short-title) }
        let right-bit = if meta.header-right != none { meta.header-right } else if meta.issuer != none { upper(meta.issuer) }
        grid(columns: (1fr, auto), column-gutter: 1em, left-bit, right-bit)
        v(-0.45em)
        line(length: 100%, stroke: 0.4pt + rule)
      }
    },
    footer: context {
      set text(font: sans, size: 8pt, fill: muted)
      let n = counter(page).get().first()
      let num = text(weight: "semibold", fill: ink, str(n))
      grid(columns: (1fr, auto, 1fr),
        align(left, if meta.footer-left != none { meta.footer-left }),
        num,
        align(right, if meta.footer-right != none { meta.footer-right }))
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(number-type: "old-style")
  show table: set text(font: sans, number-type: "lining")

  // Section heads: letter-spaced small capitals with a short accent rule.
  show heading.where(level: 1): it => block(above: 1.7em, below: 0.75em, sticky: true, {
    set par(justify: false, first-line-indent: 0pt)
    pdf.artifact(block(above: 0pt, below: 0.55em, line(length: 1.6em, stroke: 1.2pt + accent)))
    text(font: sans, size: 9.5pt, weight: "semibold", tracking: 0.1em, fill: ink, upper(it.body))
  })
  show heading.where(level: 2): it => block(above: 1.3em, below: 0.5em, sticky: true,
    text(font: display, size: 11.5pt, weight: "regular", style: "italic", fill: accent, it.body))
  show heading.where(level: 3): it => block(above: 1em, below: 0.4em, sticky: true,
    text(font: sans, size: 8.5pt, weight: "bold", fill: ink, it.body))
  show heading.where(level: 4): it => block(above: 0.9em, below: 0.35em, sticky: true,
    text(size: meta.font-size, style: "italic", it.body))

  // Block quotes become pull quotes.
  show quote.where(block: true): it => block(width: 100%, above: 1.3em, below: 1.3em, breakable: false, inset: (y: 0.75em),
    stroke: (top: 1.2pt + accent, bottom: 0.4pt + rule), {
    set par(justify: false, first-line-indent: 0pt, leading: 0.5em)
    set text(font: display, size: 13.5pt, style: "italic", fill: accent, hyphenate: false)
    it.body
    if it.attribution != none {
      v(0.4em)
      text(font: sans, size: 7.5pt, style: "normal", tracking: 0.08em, fill: muted, upper(it.attribution))
    }
  })

  // Captions: italic text after a small spaced label.
  show figure.caption: it => {
    set par(justify: false, first-line-indent: 0pt, leading: 0.5em)
    set align(start)
    if it.numbering != none and it.supplement != none {
      let num = context it.counter.display(it.numbering)
      text(font: sans, size: 7pt, weight: "semibold", tracking: 0.1em, fill: accent, upper[#it.supplement #num])
      h(0.6em)
    }
    text(size: 0.84em, style: "italic", fill: muted, it.body)
  }
  show figure: set block(above: 1.2em, below: 1.2em)
  show footnote.entry: set par(first-line-indent: 0pt)

  // The opening spread: kicker, headline, standfirst and byline across the
  // columns.
  let names = cd-names(meta)
  let byline = cd-join((if names.len() > 0 { names.join(", ") }, meta.date), sep: [#h(0.6em)#text(fill: rule)[|]#h(0.6em)])
  let kicker = if meta.doc-type != "" { meta.doc-type } else { meta.organization }
  let standfirst = if meta.subtitle != none { meta.subtitle } else { meta.summary }
  let opening = if cd-has-title(meta) {
    block(width: 100%, below: 0pt, {
      set par(justify: false, first-line-indent: 0pt)
      if kicker != none {
        text(font: sans, size: 8pt, weight: "semibold", tracking: 0.14em, fill: accent, upper(kicker))
        v(0.9em)
      }
      cd-balanced(align-to: start, text(font: display, size: 38pt, weight: "regular", tracking: -0.01em,
        hyphenate: false, fill: cd-color(meta, "heading", ink), par(leading: 0.22em, meta.title)))
      if standfirst != none {
        v(0.9em)
        block(width: 82%, text(font: display, size: 14pt, style: "italic", fill: cd-tint(meta, ink, 18%), par(leading: 0.45em, standfirst)))
      }
      if meta.subtitle != none and meta.summary != none {
        v(0.6em)
        block(width: 82%, text(size: 10.5pt, fill: muted, meta.summary))
      }
      if byline != none {
        v(1.1em)
        text(font: sans, size: 8pt, weight: "semibold", tracking: 0.08em, fill: ink, upper(byline))
      }
      v(1em)
      line(length: 100%, stroke: 0.5pt + rule)
    })
  }
  let before = {
    if opening != none { block(below: 1.6em, opening) }
    if meta.abstract != none { block(below: 1.4em, width: 82%, text(style: "italic", meta.abstract)) }
    if meta.toc { block(below: 1.4em, cd-outlines(meta, toc-depth: 2)) }
  }
  let text-body = cd-with-dropcap(meta, cd-with-end-mark(body, square(size: 0.5em, fill: accent)),
    indent: 1em, lines: 3, font: display, fill: accent)
  cd-columns(cols, gutter: 0.6cm, before: before, text-body)
}
