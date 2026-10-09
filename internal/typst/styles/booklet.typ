// Booklet — a pamphlet or zine on small pages (A5 from A4, half letter
// from letter): a coloured cover when the document has a title, then the
// text with chapter-style headings, ornamental breaks and page numbers at
// the outer edge. The cover counts as page 1, so left-hand pages are even.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#3f5f4a"))
  let ink = cd-color(meta, "ink", rgb("#22211f"))
  let heading-ink = cd-color(meta, "heading", ink)
  let muted = cd-color(meta, "muted", rgb("#73706a"))
  let on-accent = cd-on-accent(meta)
  let display = cd-display-font(meta)
  let paper = if meta.paper == "a4" { "a5" } else if meta.paper == "us-letter" { "us-statement" } else { meta.paper }
  // A cover only for a real title: one taken from the file name names the
  // PDF but is not printed on a cover or in running heads.
  let named = cd-has-title(meta) and not meta.at("title-from-name", default: false)
  let cover = meta.title-page and named
  let short-title = if named { meta.short-title }
  let theme = (accent: accent, link: accent, ink: ink, heading: heading-ink, heading-font: display,
    justify: true, first-line-indent: 1.1em, par-spacing: 0.64em, leading: 0.64em, table-style: "booktabs",
    table-size: 0.88em, callout-style: "minimal", caption-size: 0.86em, code-size: 0.8em, quote-bar: accent,
    hrule: "ornament", ornament: cd-lozenges(accent))
  // Outer edge: right on odd (recto) pages, left on even (verso) pages.
  let outer(p, body) = align(if calc.odd(p) { right } else { left }, body)
  set page(
    paper: paper,
    flipped: meta.landscape,
    margin: cd-twosided(meta, inside: 1.8cm, outside: 1.6cm, top: 1.8cm, bottom: 2cm),
    header: context {
      let p = here().page()
      if p > 1 and not query(heading.where(level: 1)).any(h => h.location().page() == p) {
        set text(size: 7.5pt, fill: muted, tracking: 0.08em)
        let running = if calc.even(p) {
          if meta.header-left != none { meta.header-left } else if short-title != none { smallcaps(lower(short-title)) }
        } else {
          if meta.header-right != none { meta.header-right } else {
            let hs = query(heading.where(level: 1).before(here()))
            if hs.len() > 0 { emph(hs.last().body) }
          }
        }
        outer(p, running)
      }
    },
    footer: context {
      let p = here().page()
      if not (cover and p == 1) {
        set text(size: 8.5pt, fill: muted)
        outer(p, grid(columns: 2, column-gutter: 0.6em, align: horizon,
          ..if calc.odd(p) { (pdf.artifact(cd-lozenges(accent, size: 2.4pt, gap: 0.3em)), str(counter(page).get().first())) }
          else { (str(counter(page).get().first()), pdf.artifact(cd-lozenges(accent, size: 2.4pt, gap: 0.3em))) }))
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(number-type: "old-style")
  show table: set text(number-type: "lining")

  // Chapter-style top-level headings: room above, an ornament, centred.
  show heading.where(level: 1): it => block(width: 100%, above: 2.6em, below: 1.5em, sticky: true, align(center, {
    set par(justify: false, first-line-indent: 0pt, leading: 0.35em)
    pdf.artifact(cd-lozenges(accent))
    v(0.7em)
    if it.numbering != none {
      text(size: 8.5pt, tracking: 0.18em, fill: accent, upper(meta.terms.at("chapter", default: "Chapter")) + [ ] + counter(heading).display("1"))
      v(0.35em)
    }
    text(font: display, size: 16pt, weight: "regular", fill: heading-ink, hyphenate: false, it.body)
  }))
  show heading.where(level: 2): it => block(above: 1.6em, below: 0.7em, sticky: true,
    text(font: display, size: 11pt, weight: "bold", fill: heading-ink, it.body))
  show heading.where(level: 3): it => block(above: 1.3em, below: 0.5em, sticky: true,
    text(size: meta.font-size, style: "italic", fill: heading-ink, it.body))
  show heading.where(level: 4): it => block(above: 1.1em, below: 0.45em, sticky: true,
    text(size: meta.font-size, tracking: 0.06em, smallcaps(lower(it.body))))

  show quote.where(block: true): it => pad(x: 1.2em, block(above: 1em, below: 1em, {
    set text(style: "italic", fill: cd-tint(meta, ink, 10%))
    set par(first-line-indent: 0pt)
    it.body
    if it.attribution != none { align(end, text(size: 0.9em, style: "normal", fill: muted)[— #it.attribution]) }
  }))
  show figure.caption: it => text(style: "italic", fill: muted, it)
  show footnote.entry: set par(first-line-indent: 0pt)

  let names = cd-names(meta)
  if cover {
    page(fill: accent, header: none, footer: none, margin: (x: 1.6cm, top: 2cm, bottom: 2cm), {
      set text(fill: on-accent)
      set par(justify: false, first-line-indent: 0pt)
      if meta.organization != none { text(size: 8pt, tracking: 0.18em, upper(meta.organization)) }
      v(1fr)
      if meta.logo != none { block(below: 1.2em, image(meta.logo, height: 1.4cm)) }
      cd-balanced(align-to: start, text(font: display, size: 30pt, weight: "regular", tracking: -0.01em,
        hyphenate: false, par(leading: 0.25em, meta.title)))
      if meta.subtitle != none {
        v(0.9em)
        text(size: 12.5pt, style: "italic", par(leading: 0.5em, meta.subtitle))
      }
      v(1.4em)
      cd-lozenges(on-accent)
      v(2fr)
      if names.len() > 0 { text(size: 10pt, tracking: 0.1em, smallcaps(lower(names.join(", ")))); linebreak() }
      if meta.date != none { text(size: 9pt, meta.date) }
    })
  } else if named {
    align(center, block(below: 2em, {
      set par(justify: false, first-line-indent: 0pt)
      v(1.2em)
      cd-balanced(text(font: display, size: 21pt, fill: heading-ink, hyphenate: false, par(leading: 0.3em, meta.title)))
      if meta.subtitle != none { v(0.5em); text(size: 11pt, style: "italic", fill: muted, meta.subtitle) }
      if names.len() > 0 { v(0.8em); text(size: 9.5pt, tracking: 0.1em, smallcaps(lower(names.join(", ")))) }
      v(1em)
      cd-lozenges(accent)
    }))
  }
  if meta.abstract != none { pad(x: 1em, bottom: 1em, text(style: "italic", meta.abstract)) }
  if meta.toc { cd-outlines(meta, toc-depth: 2, pagebreak-after: true) }
  body
}
