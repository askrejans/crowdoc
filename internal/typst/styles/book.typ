// Book — two-sided book layout with chapter openers, running heads,
// old-style figures and generous outer margins.
#import "crowdoc.typ": *

#let template(meta, body) = {
  let accent = cd-color(meta, "accent", rgb("#8b2c1c"))
  let ink = cd-color(meta, "ink", rgb("#1d1b18"))
  let muted = cd-color(meta, "muted", rgb("#6f6a61"))
  let theme = (accent: accent, link: ink, ink: ink, heading: ink, first-line-indent: 1.3em,
    par-spacing: 0.5em, leading: 0.68em, table-style: "booktabs", callout-style: "minimal",
    hrule: "ornament", ornament: [⁂])
  set page(
    paper: if meta.paper == "a4" { "iso-b5" } else { meta.paper },
    margin: cd-twosided(meta, inside: 2.2cm, outside: 2.6cm, top: 2.5cm, bottom: 2.6cm),
    header: context {
      let p = here().page()
      // No running head on chapter openers.
      let opener = query(heading.where(level: 1)).any(h => h.location().page() == p)
      if p > 2 and not opener {
        set text(size: 8.5pt, fill: muted, tracking: 0.06em)
        let chapter = {
          let hs = query(heading.where(level: 1).before(here()))
          if hs.len() > 0 { hs.last().body } else { meta.short-title }
        }
        if calc.even(p) { grid(columns: (auto, 1fr), str(counter(page).get().first()), align(right, smallcaps(lower(meta.short-title)))) }
        else { grid(columns: (1fr, auto), text(style: "italic", chapter), str(counter(page).get().first())) }
      }
    },
    footer: context {
      let p = here().page()
      if query(heading.where(level: 1)).any(h => h.location().page() == p) {
        set text(size: 8.5pt, fill: muted)
        align(center, str(counter(page).get().first()))
      }
    },
  )
  show: cd-base.with(meta, theme: theme)
  set text(number-type: "old-style")
  show table: set text(number-type: "lining")
  show heading.where(level: 1): it => {
    pagebreak(weak: true, to: "odd")
    block(above: 0pt, below: 2.2em, {
      v(3.2cm)
      align(center, {
        if it.numbering != none { text(size: 11pt, fill: accent, tracking: 0.2em, upper(meta.terms.at("chapter", default: "Chapter")) + [ ] + counter(heading).display("1")); v(0.6em) }
        text(size: 21pt, weight: "regular", it.body)
        v(0.6em)
        text(fill: accent, [⁂])
      })
    })
  }
  show heading.where(level: 2): it => block(above: 1.8em, below: 0.8em, sticky: true, align(center, text(size: meta.font-size, tracking: 0.08em, smallcaps(lower(it.body)))))
  show heading.where(level: 3): it => block(above: 1.3em, below: 0.6em, sticky: true, text(size: meta.font-size, style: "italic", it.body))

  // Title page and its verso (or, without a title page, the title opens
  // the first page).
  if meta.title-page { page(header: none, footer: none, {
    v(25%)
    align(center, {
      cd-balanced(text(size: 28pt, hyphenate: false, meta.title))
      if meta.subtitle != none { v(0.8em); text(size: 13pt, style: "italic", fill: muted, meta.subtitle) }
      v(2.4em)
      text(size: 13pt, tracking: 0.12em, smallcaps(lower(cd-names(meta).join(", "))))
    })
    v(1fr)
    align(center, text(size: 10pt, fill: muted, cd-join((meta.organization, meta.place, meta.date), sep: [ · ])))
  })
  page(header: none, footer: none, {
    v(1fr)
    set text(size: 8pt, fill: muted)
    set par(first-line-indent: 0pt)
    [© #cd-join((meta.date, meta.issuer), sep: [ ])]
    if meta.version != none [ \ #meta.version]
    if meta.keywords.len() > 0 [ \ #meta.keywords.join(" · ")]
  })
  counter(page).update(1) } else if meta.title != none and meta.title != "" {
    align(center, {
      v(2em)
      cd-balanced(text(size: 22pt, hyphenate: false, meta.title))
      if meta.subtitle != none { v(0.5em); text(size: 12pt, style: "italic", fill: muted, meta.subtitle) }
      let names = cd-names(meta)
      if names.len() > 0 { v(0.8em); text(size: 11pt, tracking: 0.12em, smallcaps(lower(names.join(", ")))) }
      v(2.4em)
    })
  }
  if meta.abstract != none { page(header: none, { v(30%); pad(x: 1.5cm, text(style: "italic", meta.abstract)) }) }
  if meta.toc { outline(depth: 2); pagebreak(weak: true) }
  body
}
