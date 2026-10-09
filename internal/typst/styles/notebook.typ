// Notebook — text written on squared paper: a faint grid with a coloured
// margin line, every line of text sitting in a row of squares. Prints only
// what is in the document.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let ink = cd-color(meta, "ink", rgb("#1f2328"))
  let muted = cd-color(meta, "muted", rgb("#66707a"))
  let accent = cd-color(meta, "accent", rgb("#d0413a"))
  let size = meta.font-size
  // The square: one row per line of text.
  let sq = calc.max(5mm, size * 1.3)
  let rows = if meta.leading != none and meta.leading >= 1.4em { 2 } else { 1 }
  let pitch = sq * rows
  let drop = pitch * 0.27
  // Faint on paper; on a dark page the rule colour is already quiet.
  let dark = luma(meta.colors.at("page", default: white)).components().first() < 40%
  let grid-ink = cd-tint(meta, cd-color(meta, "rule", rgb("#9fbad6")), if dark { 10% } else { 52% })
  let margin-ink = cd-tint(meta, accent, if dark { 45% } else { 35% })
  let theme = (accent: accent, ink: ink, heading: cd-color(meta, "heading", ink), muted: muted, link: cd-color(meta, "link", accent),
    heading-font: meta.fonts.sans, justify: false, first-line-indent: 0pt, leading: 0pt, par-spacing: pitch,
    table-style: "minimal", callout-style: "bar", quote-bar: accent, code-bg: none, list-indent: 0.3em, hrule: "line")

  let (pw, ph) = cd-page-dims(meta)
  let snap(x) = calc.round(x / sq) * sq
  let ml = snap(calc.max(pw * 0.13, 3 * sq))
  let text-w = snap(size * 36) * calc.max(meta.columns, 1)
  let mr = calc.max(2 * sq, pw - ml - text-w)
  let rtl = cd-rtl(meta)
  let margins = cd-margins(meta, top: snap(ph * 0.08), bottom: snap(ph * 0.08), left: if rtl { mr } else { ml }, right: if rtl { ml } else { mr })
  let squares = tiling(size: (sq, sq), relative: "parent", {
    place(line(start: (0pt, 0pt), end: (sq, 0pt), stroke: 0.35pt + grid-ink))
    place(line(start: (0pt, 0pt), end: (0pt, sq), stroke: 0.35pt + grid-ink))
  })
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    columns: meta.columns,
    margin: margins,
    background: {
      rect(width: 100%, height: 100%, stroke: none, fill: squares)
      let x = if rtl { pw - margins.right + sq } else { margins.left - sq }
      place(top + left, dx: x - 0.4pt, rect(width: 0.8pt, height: 100%, stroke: none, fill: margin-ink))
    },
    // No running header of its own: only the document's header texts.
    ..(header: if meta.header-left != none or meta.header-right != none {
      set text(font: meta.fonts.sans, size: size * 0.8, fill: muted)
      cd-running(meta.header-left, none, meta.header-right)
    }),
    footer: context {
      set text(font: meta.fonts.sans, size: size * 0.8, fill: muted, number-type: "lining")
      let folio = if counter(page).final().first() > 1 { str(counter(page).get().first()) }
      cd-running(meta.footer-left, none, cd-join((meta.footer-right, folio), sep: h(1.2em)))
    },
  )
  show: cd-base.with(meta, theme: theme)
  // Every line box is exactly one row (or two): the baseline sits a little
  // above a grid line, so text keeps to the squares from page to page.
  set text(font: meta.fonts.sans, top-edge: pitch - drop, bottom-edge: -drop, costs: (hyphenation: 300%))
  set par(leading: 0pt, spacing: pitch, linebreaks: "optimized")
  set list(spacing: 0pt, marker: ([–], [·], [–]))
  set enum(spacing: 0pt)

  let num(it) = if it.numbering != none { counter(heading).display(it.numbering); h(0.6em) }
  show heading: set text(font: meta.fonts.sans, hyphenate: false)
  show heading: set par(leading: 0pt)
  show heading.where(level: 1): it => block(above: 2 * pitch, below: pitch, sticky: true, {
    set text(size: size * 1.45, weight: "semibold", top-edge: 2 * sq - drop, bottom-edge: -drop)
    underline(stroke: 0.9pt + accent, offset: drop * 0.55, evade: false, { num(it); it.body })
  })
  show heading.where(level: 2): it => block(above: 2 * pitch, below: pitch, sticky: true,
    text(size: size * 1.12, weight: "semibold", { num(it); it.body }))
  show heading: it => if it.level > 2 {
    block(above: pitch, below: pitch, sticky: true, text(size: size, weight: "semibold", fill: accent, { num(it); it.body }))
  } else { it }

  // Blocks keep to the grid: no vertical insets, rows exactly one square.
  set table(inset: (x: 0.45em, y: 0pt), stroke: (x, y) => if y == 0 { (bottom: 0.8pt + ink) } else { none })
  show table: set text(size: size * 0.94)
  show raw.where(block: true): it => block(width: 100%, inset: (x: 0.8em), stroke: (left: 1.2pt + cd-tint(meta, muted, 40%)), {
    set par(leading: 0pt)
    it
  })
  show quote.where(block: true): it => block(above: pitch, below: pitch, inset: (left: 1em), stroke: (left: 1.2pt + accent), {
    set text(fill: cd-tint(meta, ink, 18%))
    it.body
    if it.attribution != none { block(above: pitch)[— #it.attribution] }
  })
  show figure: set block(above: pitch, below: pitch)
  show figure.where(kind: image): it => layout(area => {
    let tall = measure(block(width: area.width, it)).height
    block(width: 100%, height: calc.ceil(tall / sq) * sq, it)
  })
  set footnote.entry(separator: line(length: 2 * sq, stroke: 0.6pt + muted), gap: 0pt)
  show footnote.entry: set text(size: size * 0.85)

  // Title: written on the first rows; only when the document has one.
  let byline = (cd-names(meta).join(", "), meta.date).filter(x => x != none)
  if meta.title != none {
    block(below: 2 * pitch, {
      set par(leading: 0pt, spacing: 0pt)
      text(size: size * 1.75, weight: "semibold", hyphenate: false, top-edge: 2 * sq - drop, bottom-edge: -drop, meta.title)
      if meta.subtitle != none { parbreak(); text(size: size * 1.1, fill: muted, meta.subtitle) }
      if byline.len() > 0 { parbreak(); text(fill: muted, byline.join(h(0.8em) + sym.dot.c + h(0.8em))) }
    })
  } else if byline.len() > 0 {
    block(below: 2 * pitch, text(fill: muted, byline.join(h(0.8em) + sym.dot.c + h(0.8em))))
  }
  if meta.abstract != none { block(below: pitch, text(style: "italic", meta.abstract)) }
  if meta.toc { outline(depth: 2); v(pitch) }
  body
}
