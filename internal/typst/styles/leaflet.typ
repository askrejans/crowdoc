// Leaflet — a landscape sheet folded into three panels. The panels are
// fold-aligned: each has the same inner margins, so the gap between two
// columns straddles a fold. Text flows from panel to panel; headings are
// coloured panel heads. Faint fold marks sit outside the text area.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#0e6e6a"))
  let ink = cd-color(meta, "ink", rgb("#1d2422"))
  let heading-ink = cd-color(meta, "heading", accent)
  let muted = cd-color(meta, "muted", rgb("#5d6966"))
  let rule = cd-color(meta, "rule", rgb("#c3cfcc"))
  let on-accent = cd-on-accent(meta)
  let sans = meta.fonts.sans
  let head = meta.fonts.heading
  let panels = calc.max(meta.columns, 2)
  let margin = cd-margins(meta, top: 1.3cm, bottom: 1.3cm, left: 1.1cm, right: 1.1cm)
  let theme = (accent: accent, link: accent, ink: ink, heading: heading-ink, heading-font: head,
    justify: false, first-line-indent: 0pt, par-spacing: 0.75em, leading: 0.6em, table-style: "striped",
    table-head-fill: cd-tint(meta, accent, 86%), table-size: 0.9em, callout-style: "box", radius: 3pt,
    caption-size: 0.85em, code-size: 0.82em, quote-bar: accent, list-indent: 0.2em, hrule: "dots")
  // The gutter is the sum of the panel margins so the folds fall exactly
  // between two columns, whatever the margins.
  let gutter = margin.left + margin.right
  set page(
    paper: meta.paper,
    flipped: true,
    margin: margin,
    background: context {
      let mark(x, y0, y1) = place(top + left, dx: x, dy: y0, line(angle: 90deg, length: y1 - y0, stroke: 0.4pt + rule))
      let mt = if type(margin.top) == length { margin.top.to-absolute() } else { 1cm }
      let mb = if type(margin.bottom) == length { margin.bottom.to-absolute() } else { 1cm }
      for k in range(1, panels) {
        let x = page.width * k / panels
        mark(x, 0pt, mt * 0.45)
        mark(x, page.height - mb * 0.45, page.height)
      }
    },
  )
  set columns(gutter: gutter)
  show: cd-base.with(meta, theme: theme)
  set text(font: sans)

  // Panel heads: a coloured bar with the heading reversed out.
  show heading.where(level: 1): it => block(width: 100%, above: 1.3em, below: 0.7em, sticky: true,
    fill: accent, inset: (x: 7pt, y: 6pt), radius: 2pt, {
    set par(justify: false, leading: 0.35em)
    text(font: head, size: 11.5pt, weight: "bold", fill: on-accent, hyphenate: false, it.body)
  })
  show heading.where(level: 2): it => block(above: 1.1em, below: 0.45em, sticky: true, {
    text(font: head, size: 10.5pt, weight: "bold", fill: heading-ink, it.body)
    v(-0.5em)
    line(length: 100%, stroke: 0.8pt + accent)
  })
  show heading.where(level: 3): it => block(above: 0.9em, below: 0.35em, sticky: true,
    text(font: head, size: meta.font-size, weight: "bold", fill: heading-ink, it.body))
  show heading.where(level: 4): it => block(above: 0.8em, below: 0.3em, sticky: true,
    text(font: head, size: meta.font-size, weight: "semibold", style: "italic", it.body))

  set list(marker: box(baseline: -0.22em, square(size: 0.42em, fill: accent)), body-indent: 0.6em)
  show quote.where(block: true): it => block(width: 100%, above: 1em, below: 1em, inset: 9pt, radius: 3pt,
    fill: cd-tint(meta, accent, 90%), {
    set text(size: 1.08em, style: "italic", fill: heading-ink)
    it.body
    if it.attribution != none { v(0.2em); align(end, text(size: 0.85em, style: "normal", fill: muted)[— #it.attribution]) }
  })
  show figure.caption: it => text(fill: muted, it)

  // Title block at the top of the first panel.
  let opening = if cd-has-title(meta) {
    block(width: 100%, below: 1.2em, {
      set par(justify: false)
      if meta.logo != none { block(below: 0.9em, image(meta.logo, height: 1.1cm)) }
      block(width: 100%, fill: accent, inset: (x: 9pt, top: 12pt, bottom: 11pt), radius: 3pt, {
        set text(fill: on-accent)
        text(font: head, size: 21pt, weight: "bold", tracking: -0.01em, hyphenate: false, par(leading: 0.28em, meta.title))
        if meta.subtitle != none {
          v(0.55em)
          text(size: 10.5pt, par(leading: 0.45em, meta.subtitle))
        }
      })
      let line-bits = (meta.organization, meta.date).filter(x => x != none)
      if line-bits.len() > 0 {
        v(0.6em)
        text(size: 8pt, weight: "semibold", fill: muted, line-bits.join[ · ])
      }
    })
  }
  let content = {
  opening
  if meta.summary != none { block(below: 1em, text(size: 10.5pt, weight: "medium", fill: heading-ink, meta.summary)) }
  if meta.abstract != none { block(below: 1em, meta.abstract) }
  if meta.toc { block(below: 1em, cd-outlines(meta, toc-depth: 2)) }
  body
  }
  // A short text is spread evenly over the panels; a longer one fills them
  // in turn (with notes at the foot of each panel).
  context {
    let (pw, ph) = (page.width, page.height)
    let len(v, d) = if type(v) == length { v.to-absolute() } else { d }
    let colw = pw / panels - len(margin.left, 1cm) - len(margin.right, 1cm)
    let avail = ph - len(margin.top, 1cm) - len(margin.bottom, 1cm)
    let total = measure(block(width: colw, content)).height
    if total < avail * panels * 0.85 {
      cd-columns(panels, gutter: gutter, content)
    } else {
      set page(columns: panels)
      content
    }
  }
}
