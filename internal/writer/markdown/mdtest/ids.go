package mdtest

import "github.com/askrejans/crowdoc/v2/ast"

// ClearAutoIDs removes from back the heading identifiers whose
// counterparts in orig have none: Markdown headings always get an
// automatic identifier.
func ClearAutoIDs(orig, back []ast.Block) {
	var a, b []*ast.Heading
	collect := func(blocks []ast.Block, out *[]*ast.Heading) {
		ast.WalkBlocks(blocks, func(x ast.Block) bool {
			if h, ok := x.(*ast.Heading); ok {
				*out = append(*out, h)
			}
			return true
		})
	}
	collect(orig, &a)
	collect(back, &b)
	for i := range a {
		if i < len(b) && a[i].Attr.ID == "" {
			b[i].Attr.ID = ""
		}
	}
}
