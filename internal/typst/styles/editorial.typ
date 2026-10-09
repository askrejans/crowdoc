// Editorial — the opinion page of a quality paper: kicker, large serif
// headline, deck, an author line under a fine rule, then one narrow
// justified column opening with a drop cap.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#b4232a"))
  let ink = cd-color(meta, "ink", rgb("#1a1a1a"))
  let heading-ink = cd-color(meta, "heading", ink)
  let muted = cd-color(meta, "muted", rgb("#676767"))
  let rule = cd-color(meta, "rule", rgb("#c8c8c8"))
  let display = cd-display-font(meta)
  let sans = meta.fonts.sans
  let theme = (accent: accent, link: accent, ink: ink, heading: heading-ink, heading-font: display,
    justify: true, first-line-indent: 1.2em, par-spacing: 0.72em, leading: 0.72em, table-style: "booktabs",
    callout-style: "minimal", caption-size: 0.85em, quote-bar: rule,
    hrule: "ornament", ornament: cd-lozenges(muted))
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.8cm, bottom: 2.8cm, left: 22%, right: 22%),
    header: context {
      if here().page() > 1 {
        set text(size: 8pt, fill: muted)
        let left-bit = if meta.header-left != none { meta.header-left } else if meta.short-title != none { emph(meta.short-title) }
        let right-bit = if meta.header-right != none { meta.header-right } else {
          let names = cd-names(meta)
          if names.len() > 0 { text(font: sans, size: 7pt, tracking: 0.1em, upper(names.join(", "))) }
        }
        grid(columns: (1fr, auto), column-gutter: 1em, left-bit, right-bit)
      }
    },
    footer: context {
      set text(size: 8.5pt, fill: muted)
      grid(columns: (1fr, auto, 1fr),
        align(left, if meta.footer-left != none { meta.footer-left }),
        str(counter(page).get().first()),
        align(right, if meta.footer-right != none { meta.footer-right }))
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(number-type: "old-style")
  show table: set text(number-type: "lining")

  show heading.where(level: 1): it => block(above: 1.9em, below: 0.7em, sticky: true, {
    set par(justify: false, first-line-indent: 0pt)
    text(font: display, size: 14pt, weight: "bold", fill: heading-ink, it.body)
  })
  show heading.where(level: 2): it => block(above: 1.5em, below: 0.55em, sticky: true,
    text(font: display, size: 12pt, weight: "regular", style: "italic", fill: heading-ink, it.body))
  show heading.where(level: 3): it => block(above: 1.2em, below: 0.45em, sticky: true,
    text(font: sans, size: 8.5pt, weight: "semibold", tracking: 0.08em, fill: heading-ink, upper(it.body)))
  show heading.where(level: 4): it => block(above: 1em, below: 0.4em, sticky: true,
    text(size: meta.font-size, style: "italic", it.body))

  // Quotations: indented and set a little smaller, no rule.
  show quote.where(block: true): it => pad(left: 1.6em, right: 1.6em, block(above: 1.1em, below: 1.1em, {
    set text(size: 0.94em, fill: cd-tint(meta, ink, 12%))
    set par(first-line-indent: 0pt)
    it.body
    if it.attribution != none { align(end, text(size: 0.92em, fill: muted)[— #it.attribution]) }
  }))
  show figure.caption: it => {
    set par(first-line-indent: 0pt)
    text(size: 0.85em, fill: muted, it)
  }

  // Opening: kicker, headline, deck and the author line.
  let kicker = if meta.doc-type != "" { meta.doc-type } else { meta.organization }
  let deck = if meta.subtitle != none { meta.subtitle } else { meta.summary }
  let names = cd-names(meta)
  if cd-has-title(meta) {
    block(width: 100%, below: 2.2em, {
      set par(justify: false, first-line-indent: 0pt)
      pdf.artifact(line(length: 100%, stroke: 1.6pt + heading-ink))
      v(0.75em)
      if kicker != none {
        text(font: sans, size: 8pt, weight: "bold", tracking: 0.16em, fill: accent, upper(kicker))
        v(0.8em)
      }
      cd-balanced(align-to: start, text(font: display, size: 31pt, weight: "bold", tracking: -0.01em,
        hyphenate: false, fill: heading-ink, par(leading: 0.28em, meta.title)))
      if deck != none {
        v(0.85em)
        text(size: 13.5pt, style: "italic", fill: muted, par(leading: 0.5em, deck))
      }
      if meta.subtitle != none and meta.summary != none {
        v(0.6em)
        text(size: 10.5pt, fill: muted, meta.summary)
      }
      if names.len() > 0 or meta.date != none {
        v(1.2em)
        line(length: 100%, stroke: 0.5pt + rule)
        v(0.25em)
        set text(font: sans, size: 7.5pt)
        grid(columns: (1fr, auto), column-gutter: 1em,
          if names.len() > 0 { text(weight: "semibold", tracking: 0.1em, fill: heading-ink, upper(names.join(", "))) },
          if meta.date != none { text(fill: muted, tracking: 0.04em, meta.date) })
      }
    })
  } else if kicker != none {
    // No title, but a declared section: the kicker alone heads the page.
    block(below: 1.6em, text(font: sans, size: 8pt, weight: "bold", tracking: 0.16em, fill: accent, upper(kicker)))
  }
  if meta.abstract != none { block(below: 1.6em, text(style: "italic", meta.abstract)) }
  if meta.toc { block(below: 1.6em, cd-outlines(meta, toc-depth: 2)) }
  cd-with-dropcap(meta, cd-with-end-mark(body, rotate(45deg, square(size: 0.42em, fill: accent))),
    indent: 1.2em, lines: 3, font: display, fill: heading-ink, weight: "bold")
}
