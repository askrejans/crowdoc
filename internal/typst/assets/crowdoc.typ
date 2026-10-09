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
  // A display equation wider than its column is scaled down to fit.
  show math.equation.where(block: true): it => layout(size => {
    let w = measure(it).width
    if w > size.width and size.width > 0pt {
      let f = size.width / w * 100%
      scale(x: f, y: f, reflow: true, it)
    } else { it }
  })
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
  // Narrow columns (multi-column styles, large type): long words in cells
  // must be able to break instead of running into the next column.
  show table: set text(hyphenate: true)
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
  // A style may draw callouts itself: theme key callout-show is a function
  // (kind, title, colour, body) => content, returning auto for the default
  // look (title is none unless the document gave one).
  let custom = th.at("callout-show", default: none)
  if custom != none {
    let drawn = custom(kind, title, col, body)
    if drawn != auto { return drawn }
  }
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

// ---------------------------------------------------------------------------
// --- text and letter styles ---
// ---------------------------------------------------------------------------

// Whether the document language is written right to left.
#let cd-rtl(meta) = meta.lang in ("ar", "he", "fa", "ur", "yi", "ps", "ug", "sd", "dv", "ckb")

// Width and height of the page for meta.paper and meta.landscape.
#let cd-page-dims(meta) = {
  let sizes = (
    "a3": (297mm, 420mm), "a4": (210mm, 297mm), "a5": (148mm, 210mm), "a6": (105mm, 148mm),
    "iso-b5": (176mm, 250mm), "us-letter": (215.9mm, 279.4mm), "us-legal": (215.9mm, 355.6mm),
    "us-executive": (184.15mm, 266.7mm),
  )
  let s = sizes.at(meta.paper, default: (210mm, 297mm))
  if meta.landscape { (s.at(1), s.at(0)) } else { s }
}

// Margins that keep lines to a comfortable measure: the text block is
// `measure` wide per column and the side margins never fall below
// `min-side`. `start-share` is the part of the spare width that goes to the
// margin where lines begin (0.5 centres the block). Document margins win.
#let cd-measure-margins(meta, measure, min-side: 2cm, top: 2.5cm, bottom: 2.5cm, start-share: 0.5) = {
  let (w, h) = cd-page-dims(meta)
  let cols = calc.max(meta.columns, 1)
  let text-w = measure * cols + 0.04 * w * (cols - 1)
  let spare = calc.max(2 * min-side, w - text-w)
  let lead = calc.max(min-side, spare * start-share)
  let trail = calc.max(min-side, spare - lead)
  let (l, r) = if cd-rtl(meta) { (trail, lead) } else { (lead, trail) }
  cd-margins(meta, top: top, bottom: bottom, left: l, right: r)
}

// A quiet running line for headers and footers: the document's own
// header/footer texts at the ends, the style's content in the middle.
#let cd-running(first, middle, last) = grid(
  columns: (1fr, auto, 1fr),
  align(start, first), middle, align(end, last),
)

// ---------------------------------------------------------------------------
// --- article and leaflet styles ---
// ---------------------------------------------------------------------------

// Whether a printable title exists (untitled documents have none).
#let cd-has-title(meta) = meta.title != none and meta.title != [] and meta.title != ""

// Display face for headlines: the pairing's heading face when it has one,
// otherwise the main (text) face rather than the sans fallback.
#let cd-display-font(meta) = if meta.fonts.heading == meta.fonts.sans { meta.fonts.main } else { meta.fonts.heading }

// Colour for type set on an accent fill: the page colour (white on paper,
// dark on night schemes).
#let cd-on-accent(meta) = meta.colors.at("page", default: white)

// Paper size (width, height) of the common Typst paper names, with the
// page turned for landscape. Unknown names fall back to A4.
#let cd-paper-dims(paper, flipped: false) = {
  let mm = (
    "a3": (297, 420), "a4": (210, 297), "a5": (148, 210), "a6": (105, 148),
    "iso-b5": (176, 250), "iso-b4": (250, 353), "us-letter": (215.9, 279.4),
    "us-legal": (215.9, 355.6), "us-executive": (184.15, 266.7), "us-statement": (139.7, 215.9),
  ).at(paper, default: (210, 297))
  let (w, h) = (mm.at(0) * 1mm, mm.at(1) * 1mm)
  if flipped { (h, w) } else { (w, h) }
}

#let cd-rtl-langs = ("ar", "he", "fa", "ur", "yi", "ps", "sd", "ckb", "dv", "ug")
// Scripts without letter case or with vertical/ideographic traditions get
// no drop cap.
#let cd-dropcap-langs-excluded = cd-rtl-langs + ("zh", "ja", "ko", "th", "lo", "km", "my", "bo")

#let cd-seq = [].func()
#let cd-space = [ ].func()

// Inline content that may belong to a paragraph.
#let cd-is-inline(c) = {
  let f = c.func()
  if f == raw or f == math.equation { return not c.at("block", default: false) }
  f in (text, cd-space, smartquote, strong, emph, link, footnote, box, highlight, underline,
    overline, strike, smallcaps, sub, super, linebreak, h, ref, cite, metadata, pdf.artifact)
}

// The top-level children of content, with nested sequences flattened.
#let cd-children(body) = if body == none { () } else if body.func() == cd-seq {
  body.children.map(c => if c.func() == cd-seq { cd-children(c) } else { (c,) }).flatten()
} else { (body,) }

// Splits the opening paragraph off a body: (lead, rest). lead is the array
// of inline children of the first paragraph, or none when the body opens
// with something else (a heading, list, table, figure …).
#let cd-split-lead(body) = {
  let kids = cd-children(body)
  let i = 0
  while i < kids.len() and kids.at(i).func() in (cd-space, parbreak) { i += 1 }
  let start = i
  while i < kids.len() and cd-is-inline(kids.at(i)) { i += 1 }
  if i == start { return (none, body) }
  (kids.slice(start, i), kids.slice(i).join())
}

// The initial of a paragraph for a drop cap: leading quotation marks and
// punctuation with the first letter (or a number of at most two digits).
// Returns (initial, rest, opening smart quotes) or none when the paragraph
// does not start with plain text.
#let cd-take-initial(kids) = {
  let prefix = ()
  let k = 0
  let alnum = regex("^[\p{L}\p{N}]")
  while k < kids.len() {
    let c = kids.at(k)
    let f = c.func()
    if f == smartquote {
      prefix.push(c)
    } else if f == text {
      let cl = c.text.clusters()
      let j = 0
      while j < cl.len() and cl.at(j).match(alnum) == none {
        if cl.at(j).trim() == "" { return none }
        j += 1
      }
      if j == cl.len() {
        if cl.len() > 3 { return none }
        prefix.push(c)
      } else {
        let n = 1
        if cl.at(j).match(regex("^\p{N}")) != none {
          while j + n < cl.len() and cl.at(j + n).match(regex("^\p{N}")) != none { n += 1 }
          if n > 2 { return none }
        }
        let tail = cl.slice(j + n).join(default: "")
        let rest = if tail != "" { (text(tail),) } else { () }
        return ((..prefix, text(cl.slice(0, j + n).join())).join(), rest + kids.slice(k + 1), prefix.filter(p => p.func() == smartquote))
      }
    } else if f in (strong, emph) {
      let inner = cd-take-initial(cd-children(c.body))
      if inner == none { return none }
      let (ini, rest, quotes) = inner
      let wrapped = if rest.len() > 0 { (f(rest.join()),) } else { () }
      return ((..prefix, f(ini)).join(), wrapped + kids.slice(k + 1), prefix.filter(p => p.func() == smartquote) + quotes)
    } else {
      return none
    }
    k += 1
  }
  none
}

// Number of double smart quotes in content.
#let cd-count-dquotes(c) = {
  let f = c.func()
  if f == smartquote { if c.at("double", default: true) { 1 } else { 0 } }
  else if f == cd-seq { c.children.map(cd-count-dquotes).sum(default: 0) }
  else if c.has("body") and type(c.body) == content { cd-count-dquotes(c.body) }
  else { 0 }
}

// Inline children as words (break opportunities at spaces only).
#let cd-words(kids) = {
  let words = ()
  let cur = ()
  for c in kids {
    let f = c.func()
    if f == cd-space {
      if cur.len() > 0 { words.push(cur.join()); cur = () }
    } else if f == text and c.text.contains(" ") {
      for (i, part) in c.text.split(" ").enumerate() {
        if i > 0 and cur.len() > 0 { words.push(cur.join()); cur = () }
        if part != "" { cur.push(text(part)) }
      }
    } else {
      cur.push(c)
    }
  }
  if cur.len() > 0 { words.push(cur.join()) }
  words
}

// A paragraph opening with a drop cap `lines` lines deep. The text beside
// the initial is fitted by measuring; a paragraph too short to wrap the
// initial, or one that does not start with a letter, is set normally.
#let cd-dropcap(kids, lines: 3, font: auto, fill: auto, weight: "regular", gap: 0.35em) = layout(size => {
  let plain = par(first-line-indent: 0pt, kids.join())
  let r = cd-take-initial(kids)
  if r == none { return plain }
  let (initial, rest-kids, quotes) = r
  let words = cd-words(rest-kids)
  // The text beside the initial keeps the opening quotation marks, invisibly,
  // so its closing marks still close.
  let carry = quotes.map(q => text(size: 0pt, q)).join()
  if words.len() < 4 { return plain }
  let probe(n) = measure(block(width: size.width, par(first-line-indent: 0pt, justify: false, range(n).map(_ => [X]).join(linebreak())))).height
  let h1 = probe(1)
  let hn = probe(lines)
  let cap-args = (top-edge: "cap-height", bottom-edge: "baseline", weight: weight, hyphenate: false)
  if font != auto { cap-args.insert("font", font) }
  if fill != auto { cap-args.insert("fill", fill) }
  let ratio = measure(text(..cap-args, size: 100pt, "H")).height / 100pt
  let cap = text(..cap-args, size: hn / ratio, initial)
  let cw = measure(cap).width
  let gap = gap.to-absolute()
  let avail = size.width - cw - gap
  if avail < size.width * 0.5 { return plain }
  // An initial with a descender (Q, J) wraps one more line.
  let deep = measure(text(..cap-args, bottom-edge: "bounds", size: hn / ratio, initial)).height > hn + 0.12 * h1 * lines
  let hside = if deep { probe(lines + 1) } else { hn }
  let side(n) = par(first-line-indent: 0pt, [#carry#words.slice(0, n).join([ ])#if n < words.len() { linebreak(justify: true) }])
  let fits(n) = measure(block(width: avail, side(n))).height <= hside + 0.1pt
  // Shorter than the initial: no drop cap.
  if measure(block(width: avail, par(first-line-indent: 0pt, words.join([ ])))).height <= probe(lines - 1) + 0.1pt { return plain }
  let (lo, hi) = (0, words.len())
  if fits(hi) { lo = hi }
  while hi - lo > 1 {
    let mid = calc.quo(lo + hi, 2)
    if fits(mid) { lo = mid } else { hi = mid }
  }
  if lo == 0 { return plain }
  block(above: 0pt, below: par.leading, breakable: false, pad(left: cw + gap, par(first-line-indent: 0pt,
    box(width: 0pt, height: h1, place(top + left, dx: -(cw + gap), cap)) + side(lo).body)))
  if lo < words.len() {
    let open = quotes.filter(q => q.at("double", default: true)).len() + cd-count-dquotes(words.slice(0, lo).join([ ]))
    par(first-line-indent: 0pt, [#if calc.odd(open) { text(size: 0pt, smartquote()) }#words.slice(lo).join([ ])])
  }
})

// Sets the opening paragraph of body with a drop cap (when the body opens
// with a paragraph and the language has letter case). With a first-line
// indent, the paragraph after the opening one keeps its indent.
#let cd-with-dropcap(meta, body, indent: 0pt, ..args) = {
  if meta.lang in cd-dropcap-langs-excluded { return body }
  let (lead, rest) = cd-split-lead(body)
  if lead == none { return body }
  cd-dropcap(lead, ..args)
  let (next, after) = cd-split-lead(rest)
  if indent != 0pt and next != none and cd-children(rest).at(0, default: none) == parbreak() {
    par(first-line-indent: (amount: indent, all: true), next.join())
    after
  } else {
    rest
  }
}

// Restyles the opening paragraph of body (a lede): fn receives the
// paragraph content.
#let cd-with-lede(body, fn) = {
  let (lead, rest) = cd-split-lead(body)
  if lead == none { return body }
  fn(lead.join())
  parbreak()
  rest
}

// Appends an end mark (a small square, a dingbat) to the last paragraph of
// body when the text ends with one.
#let cd-with-end-mark(body, mark) = {
  let kids = cd-children(body)
  let n = kids.len()
  while n > 0 and kids.at(n - 1).func() in (cd-space, parbreak) { n -= 1 }
  if n == 0 or not cd-is-inline(kids.at(n - 1)) or kids.at(n - 1).func() == linebreak { return body }
  // A word joiner keeps the mark on the line of the last word.
  (..kids.slice(0, n), text("\u{2060}"), pdf.artifact(box(pad(left: 0.4em, mark))), ..kids.slice(n)).join()
}

// An ornament of three small lozenges (font independent).
#let cd-lozenges(fill, size: 3.2pt, gap: 0.55em) = pdf.artifact(box(baseline: -0.1em, stack(dir: ltr, spacing: gap,
  ..range(3).map(_ => rotate(45deg, reflow: true, square(size: size, fill: fill))))))

// Columns that balance when the text is short: a body that fits on the
// current page is split into columns of equal height instead of filling
// the first column and leaving the others empty. `before` is set above the
// columns at full width (an opening); `used` is space already taken on the
// page above this element; `rule` draws column rules.
#let cd-columns(n, body, gutter: 1em, before: none, used: 0pt, rule: none) = layout(size => {
  let gutter = gutter.to-absolute()
  let used = used
  if before != none {
    before
    used += measure(block(width: size.width, before)).height
  }
  if n <= 1 { return body }
  let colw = (size.width - gutter * (n - 1)) / n
  let total = measure(block(width: colw, body)).height
  let pitch = measure(block(width: colw, [X \ X])).height - measure(block(width: colw, [X])).height
  // Blocks that cannot break (quotes, boxes, figures) may leave a column
  // short by up to their height; allow for the tallest so nothing overflows.
  let breakable = ("space", "parbreak", "heading", "list", "enum", "terms", "par", "layout", "columns")
  let tallest = cd-children(body).filter(c => not cd-is-inline(c) and repr(c.func()) not in breakable)
    .map(c => measure(block(width: colw, c)).height).fold(0pt, calc.max)
  let height = total / n + pitch * 1.5 + tallest
  let rules(h) = if rule != none {
    for k in range(1, n) {
      place(top + left, dx: k * (colw + gutter) - gutter / 2, pdf.artifact(line(angle: 90deg, length: h, stroke: rule)))
    }
  }
  if height < (size.height - used) * 0.9 {
    block(height: height, breakable: false, width: 100%, { rules(height); columns(n, gutter: gutter, body) })
  } else if rule != none {
    // Flowing over pages: cd-column-rules draws the rules from the page
    // background using these markers.
    [#metadata((n: n, gutter: gutter, colw: colw))<cd-cols-start>]
    columns(n, gutter: gutter, [#body#metadata(none)<cd-cols-end>])
  } else {
    columns(n, gutter: gutter, body)
  }
})

// Column rules for cd-columns text that flows over pages; use as the page
// background. margin: the page margins (top and bottom are read).
#let cd-column-rules(rule, margin) = context {
  let starts = query(<cd-cols-start>)
  let ends = query(<cd-cols-end>)
  if starts.len() == 0 or ends.len() == 0 { return }
  let (s, e) = (starts.first(), ends.first())
  let p = here().page()
  let (sp, ep) = (s.location().page(), e.location().page())
  if p < sp or p > ep { return }
  let info = s.value
  let abs(v) = if type(v) == length { v.to-absolute() } else if type(v) == relative { v.length.to-absolute() + v.ratio * page.height } else { v * page.height }
  let x0 = s.location().position().x
  let y0 = if p == sp { s.location().position().y } else { abs(margin.top) }
  let y1 = page.height - abs(margin.bottom)
  let last = if p == ep { calc.floor((e.location().position().x - x0) / (info.colw + info.gutter) + 0.001) } else { info.n - 1 }
  for k in range(1, calc.min(last, info.n - 1) + 1) {
    place(top + left, dx: x0 + k * (info.colw + info.gutter) - info.gutter / 2, dy: y0,
      line(angle: 90deg, length: y1 - y0, stroke: rule))
  }
}
