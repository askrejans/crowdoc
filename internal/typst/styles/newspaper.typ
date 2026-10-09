// Newspaper — a broadsheet article: heavy and hairline rules, a bold
// headline across the page, deck and byline, then tightly set justified
// columns separated by column rules (three on A4 and letter, two on
// narrower pages).
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#1f1f1f"))
  let ink = cd-color(meta, "ink", rgb("#141414"))
  let heading-ink = cd-color(meta, "heading", ink)
  let muted = cd-color(meta, "muted", rgb("#5c5c5c"))
  let rule = cd-color(meta, "rule", rgb("#9a9a9a"))
  let display = cd-display-font(meta)
  let sans = meta.fonts.sans
  let (pw, _) = cd-paper-dims(meta.paper, flipped: meta.landscape)
  let scale = calc.min(1, pw / 210mm)
  let cols = calc.min(calc.max(meta.columns, 1), if pw < 170mm { 2 } else if pw >= 270mm { 4 } else { 3 })
  let margin = cd-margins(meta, top: 1.6cm, bottom: 1.7cm, left: 1.5cm, right: 1.5cm)
  let theme = (accent: accent, link: ink, ink: ink, heading: heading-ink, heading-font: display,
    justify: true, first-line-indent: 0.9em, par-spacing: 0.48em, leading: 0.48em, table-style: "booktabs",
    table-size: 0.86em, callout-style: "bar", caption-size: 0.85em, code-size: 0.82em, footnote-size: 0.85em,
    list-indent: 0.2em, hrule: "dots")
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: margin,
    background: cd-column-rules(0.4pt + rule, margin),
    header: context {
      if here().page() > 1 {
        set text(font: sans, size: 7pt, fill: muted)
        let n = str(counter(page).get().first())
        let middle = if meta.header-left != none { meta.header-left } else if meta.short-title != none { upper(meta.short-title) }
        let side = if meta.header-right != none { meta.header-right } else { meta.date }
        grid(columns: (auto, 1fr, auto), column-gutter: 1.2em,
          text(weight: "bold", fill: ink, n), text(tracking: 0.08em, middle), side)
        v(-0.4em)
        line(length: 100%, stroke: 0.6pt + ink)
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(hyphenate: true)
  show table: set text(font: sans, size: 7pt, hyphenate: false)
  set table(inset: (x: 3pt, y: 3pt))

  show heading.where(level: 1): it => block(above: 1.1em, below: 0.45em, sticky: true, {
    set par(justify: false, first-line-indent: 0pt, leading: 0.3em)
    text(font: display, size: 12pt, weight: "bold", fill: heading-ink, it.body)
  })
  show heading.where(level: 2): it => block(above: 0.95em, below: 0.4em, sticky: true,
    text(font: display, size: 10pt, weight: "bold", fill: heading-ink, it.body))
  show heading.where(level: 3): it => block(above: 0.85em, below: 0.35em, sticky: true,
    text(font: display, size: meta.font-size, weight: "bold", style: "italic", fill: heading-ink, it.body))
  show heading.where(level: 4): it => block(above: 0.8em, below: 0.3em, sticky: true,
    text(font: sans, size: meta.font-size * 0.9, weight: "bold", it.body))

  // Block quotes: pull quotes between rules, centred.
  show quote.where(block: true): it => block(width: 100%, above: 1em, below: 1em, inset: (y: 0.6em), breakable: false,
    stroke: (top: 1.4pt + ink, bottom: 0.4pt + ink), {
    set par(justify: false, first-line-indent: 0pt, leading: 0.38em)
    set align(center)
    text(font: display, size: 12.5pt, weight: "bold", style: "italic", fill: heading-ink, hyphenate: false, it.body)
    if it.attribution != none {
      v(0.3em)
      text(font: sans, size: 7pt, tracking: 0.08em, fill: muted, upper(it.attribution))
    }
  })
  show figure.caption: it => {
    set par(justify: false, first-line-indent: 0pt)
    set align(start)
    text(font: sans, size: 7pt, fill: muted, it)
  }
  show figure: set block(above: 0.9em, below: 0.9em)
  show footnote.entry: set par(first-line-indent: 0pt)

  // Opening: the flag rules with a dateline, headline, deck and byline.
  let names = cd-names(meta)
  let dateline = (meta.organization, meta.date).filter(x => x != none)
  let flag = {
    set block(spacing: 0pt)
    block(width: 100%, stroke: (top: 2.4pt + ink, bottom: 0.5pt + ink), inset: (top: 2.5pt, bottom: 0pt), {})
    if dateline.len() > 0 {
      block(width: 100%, inset: (y: 3.5pt), stroke: (bottom: 0.5pt + ink), {
        set text(font: sans, size: 7pt, tracking: 0.1em, fill: ink, top-edge: "cap-height", bottom-edge: "baseline")
        grid(columns: (1fr, auto), column-gutter: 1em,
          if meta.organization != none { text(weight: "bold", upper(meta.organization)) },
          if meta.date != none { upper(meta.date) })
      })
    }
  }
  let before = if cd-has-title(meta) {
    block(width: 100%, below: 1.3em, {
      set par(justify: false, first-line-indent: 0pt)
      flag
      v(0.9em)
      cd-balanced(align-to: start, text(font: display, size: 36pt * scale, weight: "bold", tracking: -0.025em,
        hyphenate: false, fill: heading-ink, par(leading: 0.18em, meta.title)))
      let deck = if meta.subtitle != none { meta.subtitle } else { meta.summary }
      if deck != none {
        v(0.65em)
        block(width: 88%, text(font: display, size: 13.5pt * scale, fill: cd-tint(meta, ink, 15%), par(leading: 0.4em, deck)))
      }
      if names.len() > 0 {
        v(0.8em)
        text(font: sans, size: 7.5pt, weight: "bold", tracking: 0.08em, fill: ink, upper(names.join(", ")))
      }
      v(0.5em)
      line(length: 100%, stroke: 0.5pt + ink)
    })
  } else if dateline.len() > 0 {
    block(width: 100%, below: 1.1em, flag)
  }
  let before = {
    before
    if meta.abstract != none { block(below: 1em, text(weight: "bold", meta.abstract)) }
    if meta.toc { block(below: 1em, cd-outlines(meta, toc-depth: 2)) }
  }
  cd-columns(cols, gutter: 0.5cm, before: before, rule: 0.4pt + rule, body)
}
