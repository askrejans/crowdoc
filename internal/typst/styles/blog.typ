// Blog — a well-designed long-form web article, printed: one sans column
// at a comfortable measure, a large lede, an author and date line, clear
// headings, accent links, rounded code blocks and images that break out
// of the text column.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#2563eb"))
  let ink = cd-color(meta, "ink", rgb("#1f2328"))
  let heading-ink = cd-color(meta, "heading", rgb("#0d1117"))
  let muted = cd-color(meta, "muted", rgb("#656d76"))
  let rule = cd-color(meta, "rule", rgb("#d8dee4"))
  let sans = meta.fonts.sans
  let head = meta.fonts.heading
  let theme = (accent: accent, link: accent, ink: ink, heading: heading-ink, heading-font: head,
    justify: false, first-line-indent: 0pt, par-spacing: 1.15em, leading: 0.78em, link-underline: true,
    table-style: "striped", table-head-fill: cd-tint(meta, accent, 92%), table-stripe: cd-tint(meta, ink, 96%),
    code-bg: cd-color(meta, "code-bg", cd-tint(meta, ink, 95%)), radius: 7pt, callout-style: "box",
    quote-bar: accent, caption-size: 0.82em, list-indent: 0.4em, hrule: "dots")
  // A reading measure of about 70 characters; images may use the margins.
  let side = 24%
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.4cm, bottom: 2.4cm, left: side, right: side),
    header: context {
      if here().page() > 1 {
        set text(font: sans, size: 7.5pt, fill: muted)
        let left-bit = if meta.header-left != none { meta.header-left } else { meta.short-title }
        grid(columns: (1fr, auto), column-gutter: 1em, left-bit, if meta.header-right != none { meta.header-right })
      }
    },
    footer: context {
      set text(font: sans, size: 7.5pt, fill: muted)
      grid(columns: (1fr, auto), column-gutter: 1em,
        if meta.footer-left != none { meta.footer-left } else { meta.organization },
        if meta.footer-right != none { meta.footer-right } else { cd-page-of(meta) })
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: sans)
  show raw: set text(font: meta.fonts.mono)
  show raw.where(block: true): set text(size: 0.8em)
  show raw.where(block: true): set block(inset: (x: 14pt, y: 12pt))

  show heading.where(level: 1): it => block(above: 2.1em, below: 0.75em, sticky: true, {
    set par(leading: 0.4em)
    text(font: head, size: 17pt, weight: "bold", tracking: -0.01em, fill: heading-ink, it.body)
  })
  show heading.where(level: 2): it => block(above: 1.7em, below: 0.6em, sticky: true,
    text(font: head, size: 13pt, weight: "bold", fill: heading-ink, it.body))
  show heading.where(level: 3): it => block(above: 1.4em, below: 0.5em, sticky: true,
    text(font: head, size: meta.font-size, weight: "bold", fill: accent, it.body))
  show heading.where(level: 4): it => block(above: 1.2em, below: 0.4em, sticky: true,
    text(font: head, size: meta.font-size, weight: "semibold", fill: muted, it.body))

  // Quotes: a heavy accent bar and slightly larger text.
  show quote.where(block: true): it => block(width: 100%, above: 1.5em, below: 1.5em, inset: (left: 16pt, y: 3pt),
    stroke: (left: 3pt + accent), {
    set text(size: 1.12em, fill: heading-ink, weight: "medium")
    set par(leading: 0.7em)
    it.body
    if it.attribution != none { v(0.3em); text(size: 0.8em, weight: "regular", fill: muted)[— #it.attribution] }
  })
  // Images with a caption break out of the text column a little.
  show figure.where(kind: image): it => pad(x: -2.2cm, it)
  show figure.caption: it => align(center, text(fill: muted, it))
  show figure: set block(above: 1.6em, below: 1.6em)
  show image: it => box(clip: true, radius: 6pt, it)

  // Header block: title, subtitle, author and date.
  let names = cd-names(meta)
  if cd-has-title(meta) {
    block(width: 100%, below: 2em, {
      set par(justify: false, leading: 0.32em)
      if meta.doc-type != "" {
        text(size: 8.5pt, weight: "semibold", fill: accent, meta.doc-type)
        v(0.8em)
      }
      cd-balanced(align-to: start, text(font: head, size: 29pt, weight: "bold", tracking: -0.025em,
        hyphenate: false, fill: heading-ink, meta.title))
      if meta.subtitle != none {
        v(0.7em)
        text(size: 14pt, fill: muted, par(leading: 0.5em, meta.subtitle))
      }
      if names.len() > 0 or meta.date != none {
        v(1.5em)
        let parts = meta.author-names.at(0, default: "").split(" ").filter(w => w != "")
        let initials = parts.slice(0, calc.min(2, parts.len())).map(w => w.clusters().first()).join(default: "")
        set text(size: 8.5pt)
        grid(columns: (auto, 1fr), column-gutter: 9pt, align: horizon,
          if initials != "" {
            pdf.artifact(box(width: 26pt, height: 26pt, radius: 50%, fill: accent,
              align(center + horizon, text(size: 9pt, weight: "bold", fill: cd-on-accent(meta), initials))))
          },
          {
            set par(leading: 0.5em)
            if names.len() > 0 { text(weight: "semibold", fill: heading-ink, names.join(", ")); linebreak() }
            text(fill: muted, cd-join((meta.date, meta.organization), sep: [ · ]))
          })
      }
      v(1.2em)
      line(length: 100%, stroke: 0.6pt + rule)
    })
  }
  if meta.summary != none and cd-has-title(meta) {
    block(below: 1.4em, text(size: 12.5pt, weight: "medium", fill: heading-ink, meta.summary))
  }
  if meta.abstract != none { callout(kind: "abstract", title: none, meta.abstract) }
  if meta.toc { block(below: 1.6em, cd-outlines(meta, toc-depth: 2)) }
  // The lede: the opening paragraph set larger.
  cd-with-lede(body, lede => par(text(size: 1.2em, fill: heading-ink, lede)))
}
