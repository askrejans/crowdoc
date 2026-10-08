// Manual — user manuals and handbooks: chapter openers, coloured thumb
// tabs on the outer edge, numbered steps and prominent callouts.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#e85d04"))
  let ink = cd-color(meta, "ink", rgb("#1d1d1f"))
  let muted = cd-color(meta, "muted", rgb("#6e6e73"))
  let theme = (accent: accent, link: accent.darken(10%), ink: ink, heading: ink, heading-font: meta.fonts.heading,
    justify: false, par-spacing: 0.9em, table-style: "striped", table-head-fill: cd-tint(meta, accent, 88%),
    callout-style: "box", radius: 6pt)
  let chapter-title() = {
    let hs = query(heading.where(level: 1).before(here()))
    if hs.len() > 0 { hs.last().body } else { meta.short-title }
  }
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: cd-margins(meta, top: 2.4cm, bottom: 2.4cm, left: 2.4cm, right: 2.8cm),
    header: context {
      if not cd-skip-first(meta) {
        set text(size: 8pt, fill: muted, font: meta.fonts.sans)
        grid(columns: (1fr, auto), meta.short-title, chapter-title())
      }
    },
    footer: context {
      if not cd-skip-first(meta) {
        set text(size: 8.5pt, fill: muted, font: meta.fonts.sans)
        align(right, text(weight: "bold", fill: accent, str(counter(page).get().first())))
      }
    },
    background: context {
      // Thumb tab: one step down the outer edge per chapter.
      let n = counter(heading).get().first()
      if n > 0 and not cd-skip-first(meta) {
        let step = calc.rem(n - 1, 8)
        place(top + right, dy: 3cm + step * 2.2cm, block(width: 0.5cm, height: 2cm, fill: accent, radius: (left: 3pt),
          align(center + horizon, text(fill: white, size: 9pt, weight: "bold", font: meta.fonts.sans, str(n)))))
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(font: meta.fonts.sans)
  set enum(numbering: (..n) => box(fill: accent, inset: (x: 4pt, y: 1.5pt), radius: 2pt, text(fill: white, size: 0.85em, weight: "bold", str(n.pos().last()))))
  show heading.where(level: 1): it => {
    pagebreak(weak: true)
    block(above: 0pt, below: 1.4em, {
      v(1.5cm)
      if it.numbering != none { text(font: meta.fonts.heading, size: 48pt, weight: "bold", fill: accent, counter(heading).display("1")) ; v(-0.4em) }
      text(font: meta.fonts.heading, size: 24pt, weight: "bold", it.body)
      v(0.4em)
      line(length: 100%, stroke: 0.8pt + accent)
    })
  }
  show heading.where(level: 2): it => block(above: 1.6em, below: 0.6em, sticky: true, {
    set text(font: meta.fonts.heading, size: 13.5pt, weight: "bold")
    if it.numbering != none { text(fill: accent, counter(heading).display(it.numbering)); h(0.5em) }
    it.body
  })
  show heading.where(level: 3): it => block(above: 1.2em, below: 0.5em, sticky: true, text(font: meta.fonts.heading, size: 11pt, weight: "bold", it.body))
  show: cd-chapter-numbering

  if meta.title-page {
    page(header: none, footer: none, background: none, {
      if meta.logo != none { image(meta.logo, height: 1.5cm) }
      v(1fr)
      block(width: 100%, inset: (left: 14pt), stroke: (left: 6pt + accent), {
        text(font: meta.fonts.heading, size: 36pt, weight: "bold", hyphenate: false, meta.title)
        if meta.subtitle != none { v(0.4em); text(size: 16pt, fill: muted, meta.subtitle) }
      })
      v(1.2fr)
      set text(size: 9pt, fill: muted)
      cd-join((meta.issuer, meta.version, meta.date), sep: [ · ])
    })
    counter(page).update(1)
  }
  if meta.toc { outline(depth: 2); pagebreak(weak: true) }
  body
}
