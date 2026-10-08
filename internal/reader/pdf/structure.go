package pdf

import (
	"github.com/askrejans/crowdoc/v2/layout"
)

// Tagged PDFs carry a logical structure tree (paragraphs, headings, list
// items, table cells, notes …) whose leaves are marked-content sequences
// identified by page and MCID. The reader turns it into layout hints: each
// MCID gets the id of its logical block and the block's role, which the
// layout analysis then trusts over its geometric guesses.

// mcHint is the logical block of one marked-content sequence.
type mcHint struct {
	block int
	role  layout.Role
}

// standardTypes are the structure types defined by PDF 1.7 and 2.0.
var standardTypes = func() map[name]bool {
	m := map[name]bool{}
	for _, s := range []name{"Document", "DocumentFragment", "Part", "Art", "Sect", "Div", "Aside",
		"BlockQuote", "Caption", "TOC", "TOCI", "Index", "NonStruct", "Private", "P", "H", "H1", "H2",
		"H3", "H4", "H5", "H6", "Title", "L", "LI", "Lbl", "LBody", "Table", "TR", "TH", "TD", "THead",
		"TBody", "TFoot", "Span", "Quote", "Note", "FENote", "Reference", "BibEntry", "Code", "Link",
		"Annot", "Ruby", "RB", "RT", "RP", "Warichu", "WT", "WP", "Figure", "Formula", "Form", "Sub",
		"Em", "Strong", "Artifact"} {
		m[s] = true
	}
	return m
}()

// structLimits bound hostile structure trees.
const (
	maxStructDepth    = 96
	maxStructElements = 500_000
)

// structCtx is the context inherited down the structure tree.
type structCtx struct {
	page   ref
	block  int
	role   layout.Role
	owner  bool // the block owns all content below (cells, notes, code …)
	inList bool
	inHead bool // inside a table header section
	inPara bool
}

type structWalker struct {
	f       *file
	roleMap dict
	hints   map[ref]map[int]mcHint
	visited map[ref]bool
	nextID  int
	count   int
}

// structureHints reads the structure tree. It returns nil for untagged
// files.
func (f *file) structureHints() map[ref]map[int]mcHint {
	root, _ := f.resolve(f.trailer["Root"]).(dict)
	tree, _ := f.resolve(root["StructTreeRoot"]).(dict)
	if tree == nil {
		return nil
	}
	w := &structWalker{f: f, hints: map[ref]map[int]mcHint{}, visited: map[ref]bool{}}
	w.roleMap, _ = f.resolve(tree["RoleMap"]).(dict)
	w.kids(tree["K"], structCtx{}, 0)
	if len(w.hints) == 0 {
		return nil
	}
	return w.hints
}

// standardType maps a structure type through the role map to a standard
// type, keeping the original name for types that only mean something
// before mapping (word processors map their "Quote" and "Title" paragraph
// styles to P).
func (w *structWalker) standardType(s name) (std, raw name) {
	raw, std = s, s
	if standardTypes[s] {
		// Standard types keep their meaning even when a producer maps its
		// style names (which may collide with them) through the role map.
		return s, s
	}
	for i := 0; i < 8 && w.roleMap != nil; i++ {
		m, ok := w.f.resolve(w.roleMap[std]).(name)
		if !ok || m == std {
			break
		}
		std = m
	}
	return std, raw
}

func (w *structWalker) kids(v any, ctx structCtx, depth int) {
	if depth > maxStructDepth || w.count > maxStructElements {
		return
	}
	switch k := v.(type) {
	case ref:
		if w.visited[k] {
			return
		}
		w.visited[k] = true
		w.kids(w.f.resolve(k), ctx, depth+1)
	case array:
		for _, e := range k {
			w.kids(e, ctx, depth+1)
		}
	case int:
		w.mark(ctx.page, k, ctx)
	case dict:
		switch k["Type"] {
		case name("MCR"):
			pg := ctx.page
			if r, ok := k["Pg"].(ref); ok {
				pg = r
			}
			if id, ok := integer(w.f.resolve(k["MCID"])); ok {
				w.mark(pg, id, ctx)
			}
		case name("OBJR"):
		default:
			w.element(k, ctx, depth)
		}
	}
}

func (w *structWalker) mark(pg ref, mcid int, ctx structCtx) {
	if mcid < 0 || ctx.block == 0 {
		return
	}
	m := w.hints[pg]
	if m == nil {
		m = map[int]mcHint{}
		w.hints[pg] = m
	}
	m[mcid] = mcHint{block: ctx.block, role: ctx.role}
}

func (w *structWalker) newBlock(ctx *structCtx, role layout.Role, owner bool) {
	w.nextID++
	ctx.block, ctx.role, ctx.owner = w.nextID, role, owner
}

func (w *structWalker) element(e dict, ctx structCtx, depth int) {
	w.count++
	if r, ok := e["Pg"].(ref); ok {
		ctx.page = r
	}
	s, _ := w.f.resolve(e["S"]).(name)
	std, raw := w.standardType(s)
	switch {
	case raw == "Title" || std == "Title":
		if !ctx.owner {
			w.newBlock(&ctx, layout.RoleTitle, true)
		}
	case std == "H" || len(std) == 2 && std[0] == 'H' && std[1] >= '1' && std[1] <= '6':
		if !ctx.owner {
			role := layout.RoleHeading1
			if std != "H" {
				role = layout.RoleHeading1 + layout.Role(std[1]-'1')
			}
			w.newBlock(&ctx, role, true)
		}
	case std == "Quote" && !ctx.inPara, std == "BlockQuote":
		if !ctx.owner {
			w.newBlock(&ctx, layout.RoleQuote, false)
		}
		ctx.inPara = true
	case std == "P":
		if !ctx.owner {
			role := layout.RoleParagraph
			if ctx.role == layout.RoleQuote || ctx.role == layout.RoleListBody || ctx.role == layout.RoleTOC {
				role = ctx.role
			}
			w.newBlock(&ctx, role, false)
		}
		ctx.inPara = true
	case std == "L":
		ctx.inList = true
	case std == "LI":
		ctx.inList = true
		ctx.inPara = false
	case std == "Lbl":
		switch {
		case ctx.owner:
		case ctx.inList && !ctx.inPara:
			w.newBlock(&ctx, layout.RoleListLabel, true)
		case ctx.inPara:
			ctx.role = layout.RoleNoteRef
		}
	case std == "LBody":
		if !ctx.owner {
			w.newBlock(&ctx, layout.RoleListBody, false)
		}
		ctx.inPara = true
	case std == "THead":
		ctx.inHead = true
	case std == "TH" || std == "TD":
		if !ctx.owner {
			role := layout.RoleTableCell
			if std == "TH" || ctx.inHead {
				role = layout.RoleTableHeader
			}
			w.newBlock(&ctx, role, true)
		}
		ctx.inPara = true
	case std == "Caption":
		if !ctx.owner {
			w.newBlock(&ctx, layout.RoleCaption, true)
		}
	case std == "Note" || std == "FENote":
		w.newBlock(&ctx, layout.RoleNote, true)
		ctx.inPara = false
	case std == "Code":
		if !ctx.owner && !ctx.inPara {
			w.newBlock(&ctx, layout.RoleCode, true)
		}
	case std == "Formula":
		if !ctx.owner && !ctx.inPara {
			w.newBlock(&ctx, layout.RoleFormula, true)
		}
	case std == "TOC" || std == "TOCI":
		if !ctx.owner {
			w.newBlock(&ctx, layout.RoleTOC, true)
		}
	case std == "BibEntry":
		if !ctx.owner {
			w.newBlock(&ctx, layout.RoleReference, true)
		}
	case std == "Artifact":
		return
	}
	w.kids(e["K"], ctx, depth+1)
}
