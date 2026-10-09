// crowdoc.typ — shared building blocks for crowdoc styles.
//
// Every style imports this file, calls `cd-base` inside its template and
// may restyle the helpers below through the theme dictionary.

#let cd-default-theme = (
  accent: rgb("#2f5d8a"),
  ink: rgb("#1b1b1f"),
  muted: rgb("#5f6368"),
  rule: rgb("#c9c9cf"),
  heading: rgb("#1b1b1f"),
  link: rgb("#2f5d8a"),
  code-bg: rgb("#f5f5f3"),
  code-border: none,
  code-size: 0.86em,
  quote-bar: rgb("#c9c9cf"),
  table-style: "booktabs", // booktabs | grid | striped | minimal
  table-head-fill: none,
  table-stripe: rgb("#f6f6f8"),
  table-size: 0.92em,
  radius: 4pt,
  callout-style: "bar", // bar | box | minimal
  callout: (
    note: rgb("#2f6db5"),
    info: rgb("#1f7a8c"),
    tip: rgb("#2e7d4f"),
    success: rgb("#2e7d4f"),
    important: rgb("#6a3fb5"),
    warning: rgb("#b26b00"),
    caution: rgb("#c2410c"),
    danger: rgb("#b42318"),
    example: rgb("#5b6470"),
    abstract: rgb("#3d5a80"),
  ),
  justify: true,
  first-line-indent: 0pt,
  par-spacing: 0.9em,
  leading: 0.62em,
  caption-size: 0.9em,
  footnote-size: 0.85em,
  heading-font: none, // none = main font
  heading-weight: "bold",
  link-underline: false,
  list-indent: 0.6em,
)

#let cd-theme = state("cd-theme", cd-default-theme)
#let cd-terms = state("cd-terms", (:))
#let cd-labels = state("cd-labels", (:))

// Merge dictionaries recursively (b wins).
#let cd-merge(a, b) = {
  let out = a
  for (k, v) in b {
    if type(v) == dictionary and type(out.at(k, default: none)) == dictionary {
      out.insert(k, cd-merge(out.at(k), v))
    } else {
      out.insert(k, v)
    }
  }
  out
}

#let cd-term(tm, key, fallback) = tm.at(key, default: fallback)

// ---------------------------------------------------------------------------
// Base typography
// ---------------------------------------------------------------------------

// A colour from the document's scheme, or the style default.
#let cd-color(meta, key, default) = meta.colors.at(key, default: default)

// A soft tint of c towards the page colour (works on dark pages too).
#let cd-tint(meta, c, amount) = {
  let page = meta.colors.at("page", default: white)
  color.mix((page, amount), (c, 100% - amount))
}

// Theme keys driven by colour-scheme roles.
#let cd-scheme-theme(c) = {
  let out = (:)
  let map = (accent: "accent", ink: "ink", heading: "heading", muted: "muted", link: "link", rule: "rule",
    code-bg: "code-bg", table-head: "table-head-fill", stripe: "table-stripe", quote: "quote-bar")
  for (role, key) in map {
    if role in c { out.insert(key, c.at(role)) }
  }
  if "accent" in c and "link" not in c { out.insert("link", c.accent) }
  if "secondary" in c { out.insert("callout", (tip: c.secondary, success: c.secondary, example: c.at("muted", default: c.secondary))) }
  if "accent" in c { out.insert("callout", out.at("callout", default: (:)) + (note: c.accent, abstract: c.accent, important: c.accent)) }
  out
}

#let cd-base(meta, theme: (:), body) = {
  let th = cd-merge(cd-merge(cd-default-theme, theme), cd-scheme-theme(meta.colors))
  th.insert("page", meta.colors.at("page", default: white))
  th.insert("mono", meta.fonts.mono)
  let tm = meta.terms
  set page(fill: meta.colors.at("page", default: auto))
  // Optional furniture: the style's own header and footer, removed, or a
  // quiet generic one added to a style that has none.
  let quiet = (size: 8pt, fill: th.at("muted", default: luma(110)))
  set page(header: none) if meta.at("header-mode", default: "style") == "off"
  set page(footer: none) if meta.at("footer-mode", default: "style") == "off"
  set page(header: context {
    if meta.show-title and here().page() == 1 { return }
    set text(..quiet)
    grid(columns: (1fr, auto), meta.short-title,
      if meta.at("footer-mode", default: "style") == "off" { str(counter(page).get().first()) })
  }) if meta.at("header-mode", default: "style") == "add"
  set page(footer: context {
    set text(..quiet)
    align(center, str(counter(page).get().first()))
  }) if meta.at("footer-mode", default: "style") == "add"

  set document(
    title: meta.title-text,
    author: meta.author-names,
    keywords: meta.keywords,
    description: meta.description,
    date: meta.pdf-date,
  )
  set text(
    font: meta.fonts.main,
    size: meta.font-size,
    lang: meta.lang,
    region: meta.region,
    fill: th.ink,
    hyphenate: auto,
    number-type: auto,
  )
  set par(
    justify: th.justify,
    leading: if meta.leading != none { meta.leading } else { th.leading },
    spacing: if meta.leading != none { meta.leading + 0.4em } else { th.par-spacing },
    first-line-indent: th.first-line-indent,
    justification-limits: (tracking: (min: -0.01em, max: 0.02em)),
  )
  show math.equation: set text(font: meta.fonts.math)
  set math.equation(supplement: cd-term(tm, "equation", "Equation"))

  // Code
  show raw: set text(font: meta.fonts.mono, size: th.code-size)
  show raw.where(block: true): it => block(
    width: 100%,
    fill: th.code-bg,
    stroke: th.code-border,
    inset: (x: 10pt, y: 8pt),
    radius: th.radius,
    breakable: true,
    above: 1em,
    below: 1em,
    {
      set par(justify: false, leading: 0.5em)
      set text(hyphenate: false)
      it
    },
  )
  show raw.where(block: false): it => box(
    fill: th.code-bg,
    inset: (x: 2.5pt),
    outset: (y: 2.5pt),
    radius: 2pt,
    text(hyphenate: false, it),
  )

  // Links
  show link: it => {
    if type(it.dest) == str {
      set text(fill: th.link)
      if th.link-underline { underline(offset: 2pt, stroke: 0.4pt + th.link, it) } else { it }
    } else { it }
  }

  // Headings
  set heading(
    numbering: if meta.number-sections { "1.1" } else { none },
    supplement: cd-term(tm, "section", "Section"),
  )
  show heading: set text(
    fill: th.heading,
    weight: th.heading-weight,
    font: if th.heading-font != none { th.heading-font } else { meta.fonts.main },
    hyphenate: false,
  )
  show heading: set par(justify: false)
  show heading: set block(sticky: true)

  // Figures and tables
  set figure(gap: 0.8em)
  show figure.where(kind: image): set figure(supplement: cd-term(tm, "figure", "Figure"))
  show figure.where(kind: table): set figure(supplement: cd-term(tm, "table", "Table"))
  show figure.where(kind: raw): set figure(supplement: cd-term(tm, "listing", "Listing"))
  show figure.where(kind: table): set figure.caption(position: top)
  show figure.where(kind: raw): set figure.caption(position: top)
  show figure.where(kind: table): set block(breakable: true)
  show figure.where(kind: raw): set block(breakable: true)
  let label-key(kind) = if kind == image { "figure" } else if kind == table { "table" } else if kind == raw { "listing" } else { none }
  show figure.caption: it => {
    set text(size: th.caption-size)
    set par(justify: false)
    if it.numbering == none or it.supplement == none {
      it.body
    } else {
      let key = label-key(it.kind)
      let num = context it.counter.display(it.numbering)
      let lbl = if key != none and key in meta.labels { (meta.labels.at(key))(num) } else [#it.supplement~#num]
      text(weight: "semibold")[#lbl#it.separator]
      it.body
    }
  }
  set figure.caption(separator: tm.at("caption-sep", default: ". "))

  set table(
    inset: (x: 6pt, y: 4.5pt),
    stroke: (x, y) => {
      if th.table-style == "grid" { 0.5pt + th.rule }
      else if th.table-style == "minimal" { none }
      else if y == 0 { (top: 0.9pt + th.ink, bottom: 0.5pt + th.ink) }
      else { none }
    },
    fill: (x, y) => {
      if y == 0 and th.table-head-fill != none { th.table-head-fill }
      else if th.table-style == "striped" and calc.even(y) and y > 0 { th.table-stripe }
      else { none }
    },
  )
  show table: set text(size: th.table-size)
  show table: set par(justify: false)
  show table.cell.where(y: 0): set text(weight: "semibold")

  // Quotes
  set quote(block: true)
  show quote.where(block: true): it => block(
    width: 100%,
    inset: (left: 12pt, y: 2pt),
    stroke: (left: 2pt + th.quote-bar),
    {
      set text(fill: th.ink.lighten(15%), style: "italic")
      it.body
      if it.attribution != none {
        align(end, text(size: 0.9em, style: "normal", fill: th.muted)[— #it.attribution])
      }
    },
  )

  // Lists
  set list(indent: th.list-indent, body-indent: 0.5em, marker: ([•], [‣], [–]))
  // Nested ordered lists: 1. → a) → i.
  set enum(indent: th.list-indent, body-indent: 0.5em, full: true, numbering: (..n) => {
    let ns = n.pos()
    numbering(("1.", "a)", "i.").at(calc.min(ns.len() - 1, 2)), ns.last())
  })
  set terms(separator: [ — ], hanging-indent: 1.5em)

  // Footnotes
  set footnote.entry(separator: line(length: 30%, stroke: 0.5pt + th.rule))
  show footnote.entry: set text(size: th.footnote-size)
  show footnote.entry: set par(justify: true)

  set outline(title: cd-term(tm, "contents", "Contents"), indent: auto)
  show outline.entry.where(level: 1): set block(above: 0.9em)

  cd-theme.update(th)
  cd-terms.update(tm)
  cd-labels.update(meta.labels)
  body
}

// ---------------------------------------------------------------------------
// Outlines
// ---------------------------------------------------------------------------

#let cd-outlines(meta, toc-depth: 3, pagebreak-after: false) = {
  let tm = meta.terms
  let any = false
  if meta.toc {
    outline(depth: toc-depth)
    any = true
  }
  if meta.lof {
    outline(title: cd-term(tm, "list-of-figures", "List of Figures"), target: figure.where(kind: image))
    any = true
  }
  if meta.lot {
    outline(title: cd-term(tm, "list-of-tables", "List of Tables"), target: figure.where(kind: table))
    any = true
  }
  if any and pagebreak-after { pagebreak(weak: true) }
}

// ---------------------------------------------------------------------------
// Body helpers used by the generated document
// ---------------------------------------------------------------------------

#let cd-theorem-kinds = ("theorem", "lemma", "corollary", "proposition", "definition", "remark", "example", "exercise", "solution")

#let callout(kind: "note", title: none, body) = context {
  let th = cd-theme.get()
  let tm = cd-terms.get()
  if kind == "proof" {
    block(width: 100%, breakable: true, {
      emph(if title != none { title } else { cd-term(tm, "proof", "Proof") })
      [. ]
      body
      h(1fr)
      box(width: 0.55em, height: 0.55em, stroke: 0.6pt + th.ink)
    })
    return
  }
  if kind in cd-theorem-kinds and kind != "example" {
    let c = counter("cd-" + kind)
    c.step()
    block(width: 100%, breakable: true, above: 1.1em, below: 1.1em, {
      text(weight: "bold")[#cd-term(tm, kind, upper(kind.first()) + kind.slice(1)) #context c.display()]
      if title != none [ (#title)]
      [. ]
      if kind in ("theorem", "lemma", "corollary", "proposition") { emph(body) } else { body }
    })
    return
  }
  let col = th.callout.at(kind, default: th.accent)
  let label = if title != none { title } else { cd-term(tm, kind, upper(kind.first()) + kind.slice(1)) }
  let style = th.callout-style
  block(
    width: 100%,
    breakable: true,
    above: 1em,
    below: 1em,
    fill: if style == "minimal" { none } else { color.mix((th.at("page", default: white), 91%), (col, 9%)) },
    stroke: if style == "box" { 0.6pt + col.lighten(40%) } else { (left: 2.5pt + col) },
    inset: (left: 12pt, right: 10pt, y: 9pt),
    radius: if style == "box" { th.radius } else { (right: th.radius) },
    {
      if label != none and label != [] {
        block(below: 0.55em, sticky: true, text(weight: "semibold", fill: col, size: 0.95em, label))
      }
      body
    },
  )
}

#let cd-checkbox(done) = box(
  width: 0.78em,
  height: 0.78em,
  baseline: 0.08em,
  stroke: 0.6pt + luma(90),
  radius: 1.5pt,
  if done {
    place(dx: 0.14em, dy: 0.12em, curve(
      stroke: (paint: luma(40), thickness: 0.9pt, cap: "round", join: "round"),
      curve.move((0em, 0.28em)),
      curve.line((0.18em, 0.48em)),
      curve.line((0.52em, 0.02em)),
    ))
  },
)

#let task-list(..items) = {
  let th = cd-theme
  block(above: 0.8em, below: 0.8em, stack(dir: ttb, spacing: 0.55em, ..items.pos().map(it => grid(
    columns: (1.3em, 1fr),
    column-gutter: 0.2em,
    cd-checkbox(it.at(0)),
    it.at(1),
  ))))
}

#let cd-hrule() = context {
  let th = cd-theme.get()
  let kind = th.at("hrule", default: "line")
  align(center, block(above: 1.2em, below: 1.2em,
    if kind == "ornament" { text(fill: th.accent, size: 1.1em, th.at("ornament", default: [⁂])) }
    else if kind == "dots" { text(fill: th.muted, tracking: 0.6em, [· · ·]) }
    else { line(length: 30%, stroke: 0.6pt + th.rule) }))
}

#let cd-lines(..lines) = block(above: 0.8em, below: 0.8em, {
  set par(justify: false)
  lines.pos().join(linebreak())
})

#let kbd(body) = box(
  inset: (x: 3pt, y: 0pt),
  outset: (y: 2.5pt),
  radius: 2.5pt,
  stroke: 0.5pt + luma(170),
  fill: luma(248),
  text(size: 0.85em, body),
)

// Bibliography produced by crowdoc's citation processor.
// entries: array of (id: str, label: str or none, body: content)
#let cd-bibliography(title: none, numeric: false, entries) = context {
  let th = cd-theme.get()
  let tm = cd-terms.get()
  let ttl = if title != none { title } else { cd-term(tm, "references", "References") }
  heading(numbering: none, ttl)
  set par(justify: false)
  set text(size: 0.94em)
  if numeric {
    let width = calc.max(..entries.map(e => measure([#e.label]).width))
    grid(
      columns: (width + 0.6em, 1fr),
      row-gutter: 0.75em,
      ..entries.map(e => ([#metadata(none)#label(e.id)#e.label], e.body)).flatten(),
    )
  } else {
    for e in entries {
      block(above: 0.7em, below: 0.7em, par(hanging-indent: 1.6em, first-line-indent: 0pt)[#metadata(none)#label(e.id)#e.body])
    }
  }
}

// Signature lines for agreements and letters.
#let cd-signatures(parties: (), name-label: "Name", sign-label: "Signature", date-label: "Date") = context {
  let th = cd-theme.get()
  let ps = if parties.len() == 0 { ([], []) } else { parties }
  v(2.2em)
  block(breakable: false, grid(
    columns: (1fr,) * calc.min(ps.len(), 2),
    column-gutter: 2.5em,
    row-gutter: 2.6em,
    ..ps.map(p => {
      if p != [] { text(weight: "semibold", p); v(2.6em) } else { v(3.2em) }
      line(length: 100%, stroke: 0.5pt + th.ink)
      v(0.25em)
      set text(size: 0.85em, fill: th.muted)
      grid(columns: (1fr, auto), sign-label, date-label)
    }),
  ))
}

// Placeholder shown when an image could not be loaded.
#let cd-missing-image(alt, width: 60%) = block(
  width: width,
  height: 3.2cm,
  stroke: (paint: luma(170), thickness: 0.6pt, dash: "dashed"),
  radius: 3pt,
  inset: 8pt,
  align(center + horizon, text(size: 0.85em, fill: luma(110), alt)),
)

// Appendix: restart top-level numbering with letters.
#let cd-appendix(body) = {
  counter(heading).update(0)
  set heading(numbering: "A.1")
  body
}

// Running title helpers
#let cd-page-of(meta) = context {
  let n = counter(page).get().first()
  let t = counter(page).final().first()
  (meta.page-of)(n, t)
}

// ---------------------------------------------------------------------------
// Layout building blocks for styles
// ---------------------------------------------------------------------------

// Page margins: document overrides win over style defaults.
#let cd-margins(meta, top: 2.5cm, bottom: 2.5cm, left: 2.5cm, right: 2.5cm, inside: none, outside: none) = {
  let pick(v, d) = if v != auto { v } else { d }
  if inside != none {
    (
      top: pick(meta.margins.top, top),
      bottom: pick(meta.margins.bottom, bottom),
      inside: pick(meta.margins.left, inside),
      outside: pick(meta.margins.right, outside),
    )
  } else {
    (
      top: pick(meta.margins.top, top),
      bottom: pick(meta.margins.bottom, bottom),
      left: pick(meta.margins.left, left),
      right: pick(meta.margins.right, right),
    )
  }
}

#let cd-names(meta) = meta.authors.map(a => a.name)

#let cd-join(items, sep: [, ], last: none) = {
  let items = items.filter(x => x != none)
  if items.len() == 0 { return none }
  if items.len() == 1 { return items.first() }
  if last == none { return items.join(sep) }
  items.slice(0, -1).join(sep) + last + items.last()
}

// Authors with numbered affiliation marks, as in journals.
#let cd-author-block(meta, size: 1em, align-to: center, emails: true) = {
  if meta.authors.len() == 0 { return }
  let affs = ()
  for a in meta.authors {
    for af in a.affiliations {
      if af not in affs { affs.push(af) }
    }
  }
  let marks(a) = a.affiliations.map(af => str(affs.position(x => x == af) + 1)).join(",")
  align(align-to, {
    set text(size: size)
    meta.authors.map(a => box[#a.name#if affs.len() > 1 and a.affiliations.len() > 0 { super(marks(a)) }#if a.corresponding { super[\*] }]).join([, #h(0.2em)])
    if affs.len() > 0 {
      v(0.35em)
      set text(size: 0.86em, style: "italic")
      if affs.len() == 1 { affs.first() } else {
        affs.enumerate().map(((i, af)) => [#super(str(i + 1))#af]).join(linebreak())
      }
    }
    let mails = meta.authors.filter(a => a.email != none).map(a => raw(a.email))
    if emails and mails.len() > 0 {
      v(0.25em)
      set text(size: 0.8em)
      mails.join([ · ])
    }
  })
}

#let cd-keywords-line(meta, label: auto) = {
  if meta.keywords.len() == 0 { return }
  let lbl = if label == auto { meta.terms.at("keywords", default: "Keywords") } else { label }
  par(hanging-indent: 1.2em, first-line-indent: 0pt)[#text(weight: "semibold")[#lbl:] #meta.keywords.join(", ")]
}

// A quiet two-column grid of label/value pairs (cover pages, memos).
#let cd-fields(rows, label-style: (:), gutter: 1.4em, row-gutter: 0.6em, compact: false) = {
  let cells = ()
  for (label, value) in rows {
    if value != none and value != [] and value != "" {
      cells.push(text(..label-style, label))
      cells.push(value)
    }
  }
  if cells.len() == 0 { return }
  grid(columns: if compact { (auto, auto) } else { (auto, 1fr) }, column-gutter: gutter, row-gutter: row-gutter, ..cells)
}

// Code blocks with line numbers.
#let cd-numbered(r) = {
  show raw.line: it => {
    box(width: 2.2em, align(right, text(fill: luma(150), size: 0.85em, str(it.number))))
    h(0.8em)
    it.body
  }
  r
}

// Running header/footer helpers (show nothing on the first physical page
// when the style has a title page).
#let cd-skip-first(meta) = meta.title-page and here().page() == 1

#let cd-date-place(meta) = cd-join((meta.place, meta.date), sep: [, ])

// Balanced multi-line text (titles): lines of similar length instead of a
// full first line and a lone last word.
#let cd-balanced(body, align-to: center) = layout(size => {
  let natural = measure(body).width
  if natural <= size.width { return align(align-to, body) }
  let lines = calc.ceil(natural / size.width)
  let target = calc.min(size.width, natural / lines * 1.12 + measure([M]).width)
  align(align-to, box(width: target, {
    set par(justify: false)
    set align(align-to)
    body
  }))
})

// Chapter-based numbering: figures, tables, listings and equations are
// numbered within top-level headings ("2.3").
#let cd-chapter-numbering(body) = {
  show heading.where(level: 1): it => {
    counter(figure.where(kind: image)).update(0)
    counter(figure.where(kind: table)).update(0)
    counter(figure.where(kind: raw)).update(0)
    counter(math.equation).update(0)
    it
  }
  let ch(n) = {
    let h = counter(heading).get().first()
    if h > 0 { numbering("1.1", h, n) } else { numbering("1", n) }
  }
  set figure(numbering: n => context ch(n))
  set math.equation(numbering: n => context [(#ch(n))])
  body
}

// Two-sided margins that put the larger margin on the outside.
#let cd-twosided(meta, inside: 2.4cm, outside: 2.8cm, top: 2.6cm, bottom: 2.8cm) = cd-margins(meta, top: top, bottom: bottom, inside: inside, outside: outside)

// Cross-reference that degrades gracefully: unnumbered headings link with
// their title; numbered elements use the normal "Figure 2" form.
#let cd-ref(target, bare: false) = context {
  let found = query(target)
  if found.len() == 0 { return text(weight: "bold")[??] }
  let el = found.first()
  let f = el.func()
  if f == heading and el.numbering == none {
    link(target, emph(el.body))
  } else if f == figure and el.numbering != none {
    let key = if el.kind == image { "figure" } else if el.kind == table { "table" } else if el.kind == raw { "listing" } else { none }
    let num = numbering(el.numbering, ..el.counter.at(el.location()))
    let labels = cd-labels.get()
    if bare { link(target, num) }
    else if key != none and key in labels { link(target, (labels.at(key))(num)) }
    else { ref(target) }
  } else if f == heading or f == figure or f == math.equation {
    if bare { ref(target, supplement: none) } else { ref(target) }
  } else {
    link(target, sym.arrow.r)
  }
}

// An image at its natural size, never wider than its container.
#let cd-image(path, natural: none, alt: none) = layout(size => {
  let w = if natural == none { size.width } else { calc.min(natural, size.width) }
  image(path, width: w, alt: alt)
})

// Program output (notebook cells): quiet monospace lines behind a rule;
// errors in red.
#let cd-output(text-content, error: false) = context {
  let th = cd-theme.get()
  let c = if error { th.callout.danger } else { th.rule }
  block(width: 100%, breakable: true, inset: (left: 10pt, y: 3pt), stroke: (left: 1.5pt + c),
    fill: if error { color.mix((th.at("page", default: white), 94%), (c, 6%)) } else { none }, {
    set text(font: th.mono, size: th.code-size * 0.8, fill: if error { c } else { th.muted }, hyphenate: false)
    set par(justify: false, leading: 0.5em)
    text-content.split("\n").map(l => l.replace(" ", "\u{a0}")).join(linebreak())
  })
}
