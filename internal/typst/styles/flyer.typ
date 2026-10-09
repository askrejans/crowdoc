// Flyer — a one-page announcement with a poster's directness: a huge
// headline reversed out of a solid accent band, large text, bold
// checklists and a call-to-action box. Without a title the accent band
// stays as a strip and the text starts at once.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#e4572e"))
  let ink = cd-color(meta, "ink", rgb("#17181c"))
  let heading-ink = cd-color(meta, "heading", ink)
  let muted = cd-color(meta, "muted", rgb("#5d6068"))
  let on-accent = cd-on-accent(meta)
  let head = meta.fonts.heading
  let (pw, _) = cd-paper-dims(meta.paper, flipped: meta.landscape)
  let scale = calc.min(1.15, calc.max(0.7, pw / 210mm))
  let margin = cd-margins(meta, top: 1.9cm, bottom: 2cm, left: 1.9cm, right: 1.9cm)
  let strip = 0.55cm
  let cta(kind, title, col, body) = {
    if kind not in ("tip", "important", "note", "success", "info") { return auto }
    block(width: 100%, above: 1.2em, below: 1.2em, breakable: false, fill: accent, radius: 8pt,
      inset: (x: 18pt, y: 15pt), {
      set text(fill: on-accent, size: 1.1em, weight: "medium")
      set par(leading: 0.55em)
      show strong: set text(weight: "black")
      show link: set text(fill: on-accent)
      if title != none { block(below: 0.5em, text(weight: "black", size: 1.15em, title)) }
      body
    })
  }
  let theme = (accent: accent, link: accent, ink: ink, heading: heading-ink, heading-font: head,
    justify: false, first-line-indent: 0pt, par-spacing: 0.85em, leading: 0.55em, table-style: "striped",
    table-head-fill: cd-tint(meta, accent, 85%), radius: 8pt, callout-style: "box", callout-show: cta,
    caption-size: 0.75em, code-size: 0.8em, list-indent: 0pt, hrule: "line")
  set page(
    paper: meta.paper,
    flipped: meta.landscape,
    margin: margin,
    background: context {
      // Accent strips: at the foot of every page, and at the head of the
      // first when there is no title band.
      place(bottom + left, rect(width: page.width, height: strip, fill: accent))
      if here().page() == 1 and not cd-has-title(meta) {
        place(top + left, rect(width: page.width, height: strip, fill: accent))
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  // Like a poster, all type scales with the sheet (A4 is the reference).
  set text(font: meta.fonts.sans, size: meta.font-size * scale)

  show heading.where(level: 1): it => block(above: 1.2em, below: 0.55em, sticky: true, {
    set par(leading: 0.25em)
    text(font: head, size: 27pt * scale, weight: "black", tracking: -0.02em, fill: heading-ink, hyphenate: false, it.body)
  })
  show heading.where(level: 2): it => block(above: 1.1em, below: 0.45em, sticky: true,
    text(font: head, size: 18pt * scale, weight: "bold", fill: accent, hyphenate: false, it.body))
  show heading.where(level: 3): it => block(above: 0.9em, below: 0.35em, sticky: true,
    text(font: head, size: meta.font-size * scale, weight: "bold", fill: heading-ink, it.body))
  show heading.where(level: 4): it => block(above: 0.8em, below: 0.3em, sticky: true,
    text(font: head, size: meta.font-size * scale, weight: "semibold", fill: muted, it.body))

  // Lists as bold checklists.
  let check = pdf.artifact(box(baseline: 0.17em, circle(radius: 0.5em, fill: accent, stroke: none,
    place(center + horizon, dx: 0.02em, curve(stroke: (paint: on-accent, thickness: 0.13em, cap: "round", join: "round"),
      curve.move((-0.24em, 0.02em)), curve.line((-0.07em, 0.2em)), curve.line((0.25em, -0.18em)))))))
  set list(marker: check, body-indent: 0.6em, spacing: 0.7em)
  show list: set text(weight: "semibold")
  set enum(numbering: (..n) => text(weight: "black", fill: accent, numbering(("1.", "a.", "i.").at(calc.min(n.pos().len() - 1, 2)), n.pos().last())), body-indent: 0.6em, spacing: 0.7em)
  show enum: set text(weight: "semibold")

  // Quotes speak loudly; the last one becomes a highlighted panel.
  let quotes = counter("cd-flyer-quote")
  show quote.where(block: true): it => {
    quotes.step()
    context {
      let last = quotes.get() == quotes.final()
      let attribution = if it.attribution != none {
        v(0.35em)
        text(size: 0.7em, weight: "medium", fill: muted)[— #it.attribution]
      }
      if last {
        block(width: 100%, above: 1.3em, below: 1.3em, breakable: false, fill: cd-tint(meta, accent, 86%), radius: 8pt,
          inset: (x: 18pt, y: 16pt), {
          set text(size: 1.25em, weight: "bold", fill: heading-ink)
          set par(leading: 0.45em)
          it.body
          attribution
        })
      } else {
        block(width: 100%, above: 1.2em, below: 1.2em, inset: (left: 14pt), stroke: (left: 4pt + accent), {
          set text(size: 1.12em, weight: "medium", fill: heading-ink)
          it.body
          attribution
        })
      }
    }
  }
  show figure.caption: it => text(fill: muted, it)

  // The headline band, bleeding off the top and sides of the page.
  let band = if cd-has-title(meta) {
    block(fill: accent, inset: (bottom: 1cm), {
      set text(fill: on-accent)
      set par(justify: false)
      if meta.organization != none {
        text(size: 10pt, weight: "bold", tracking: 0.14em, upper(meta.organization))
        v(0.9em)
      }
      cd-balanced(align-to: start, text(font: head, size: 54pt * scale, weight: "black", tracking: -0.03em,
        hyphenate: false, par(leading: 0.16em, meta.title)))
      if meta.subtitle != none {
        v(0.75em)
        block(width: 92%, text(size: 17pt * scale, weight: "medium", par(leading: 0.45em, meta.subtitle)))
      }
    })
  }
  let when = (meta.date, meta.place, meta.location).filter(x => x != none)
  let before = {
    if when.len() > 0 {
      block(below: 1.1em, text(size: 16pt * scale, weight: "black", fill: accent, when.join[ · ]))
    }
    if meta.summary != none { block(below: 1em, text(size: 1.15em, weight: "medium", fill: heading-ink, meta.summary)) }
    if meta.abstract != none { block(below: 1em, meta.abstract) }
  }
  // The opening paragraph is set larger, like a poster's first line.
  let body = cd-with-lede(body, lede => par(text(size: 1.18em, weight: "medium", fill: heading-ink, lede)))
  layout(size => context {
    let used = 0pt
    if band != none {
      let (l, t, r) = (margin.left, margin.top, margin.right)
      let full = block(width: page.width, inset: (left: l, right: r, top: t + 0.4cm), fill: accent, band)
      let h = measure(full).height
      place(top + left, dx: -l, dy: -t, full)
      v(h - t.to-absolute() + 1.2em)
      used = h - t.to-absolute() + 1.2em.to-absolute()
    }
    before
    used += measure(block(width: size.width, before)).height
    // Text that does not fit in one column goes into two.
    let total = measure(block(width: size.width, body)).height
    if meta.columns > 1 or total > size.height - used {
      cd-columns(calc.max(meta.columns, 2), gutter: 1.1cm, used: used, body)
    } else {
      body
    }
  })
}
