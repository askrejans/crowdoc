package html

import (
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// PruneLinks replaces internal links ("#id") whose target id does not exist
// in blocks by their text, so writers never see dangling references
// (links to removed navigation, to "#top", or to anchors in dropped
// content). Several block lists may be passed when they share anchors.
func PruneLinks(lists ...[]ast.Block) {
	ids := map[string]bool{}
	for _, blocks := range lists {
		forEachInlines(blocks, func(ins []ast.Inline) []ast.Inline {
			collectIDs(ins, ids)
			return ins
		})
		ast.WalkBlocks(blocks, func(b ast.Block) bool {
			switch n := b.(type) {
			case *ast.Heading:
				ids[n.Attr.ID] = true
			case *ast.Table:
				ids[n.Attr.ID] = true
			case *ast.Figure:
				ids[n.Attr.ID] = true
				if n.Image != nil {
					ids[n.Image.Attr.ID] = true
				}
			case *ast.CodeBlock:
				ids[n.Attr.ID] = true
			case *ast.Div:
				ids[n.Attr.ID] = true
			case *ast.MathBlock:
				ids[n.Label] = true
			case *ast.ReferenceList:
				for _, e := range n.Entries {
					ids[e.ID] = true
				}
			}
			return true
		})
	}
	delete(ids, "")
	for _, blocks := range lists {
		forEachInlines(blocks, func(ins []ast.Inline) []ast.Inline {
			return pruneInlines(ins, ids)
		})
	}
}

func collectIDs(ins []ast.Inline, ids map[string]bool) {
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Span:
			ids[n.Attr.ID] = true
		case *ast.Image:
			ids[n.Attr.ID] = true
		}
		collectIDs(ast.InlineChildren(in), ids)
	}
}

func pruneInlines(ins []ast.Inline, ids map[string]bool) []ast.Inline {
	var out []ast.Inline
	changed := false
	for i, in := range ins {
		if l, ok := in.(*ast.Link); ok && strings.HasPrefix(l.URL, "#") && !ids[l.URL[1:]] {
			if !changed {
				out = append(out, ins[:i]...)
				changed = true
			}
			out = append(out, pruneInlines(l.Inlines, ids)...)
			continue
		}
		if kids := ast.InlineChildren(in); len(kids) > 0 {
			if _, isCite := in.(*ast.Cite); !isCite {
				setChildren(in, pruneInlines(kids, ids))
			}
		}
		if changed {
			out = append(out, in)
		}
	}
	if !changed {
		return ins
	}
	return out
}

// forEachInlines applies fn to every inline sequence held by blocks,
// including captions, table cells, definition terms and footnotes.
func forEachInlines(blocks []ast.Block, fn func([]ast.Inline) []ast.Inline) {
	var visitIns func([]ast.Inline) []ast.Inline
	visitIns = func(ins []ast.Inline) []ast.Inline {
		ins = fn(ins)
		for _, in := range ins {
			switch n := in.(type) {
			case *ast.Note:
				forEachInlines(n.Blocks, fn)
			case *ast.Span:
				n.Inlines = visitIns(n.Inlines)
			default:
				if kids := ast.InlineChildren(in); len(kids) > 0 && setChildren(in, nil) {
					setChildren(in, visitIns(kids))
				}
			}
		}
		return ins
	}
	for _, b := range blocks {
		switch n := b.(type) {
		case *ast.Para:
			n.Inlines = visitIns(n.Inlines)
		case *ast.Plain:
			n.Inlines = visitIns(n.Inlines)
		case *ast.Heading:
			n.Inlines = visitIns(n.Inlines)
		case *ast.CodeBlock:
			n.Caption = visitIns(n.Caption)
		case *ast.BlockQuote:
			forEachInlines(n.Blocks, fn)
		case *ast.Div:
			n.Title = visitIns(n.Title)
			forEachInlines(n.Blocks, fn)
		case *ast.List:
			for i := range n.Items {
				forEachInlines(n.Items[i].Blocks, fn)
			}
		case *ast.DefinitionList:
			for i := range n.Items {
				n.Items[i].Term = visitIns(n.Items[i].Term)
				for _, d := range n.Items[i].Definitions {
					forEachInlines(d, fn)
				}
			}
		case *ast.Table:
			n.Caption = visitIns(n.Caption)
			for _, rows := range [][]ast.Row{n.Head, n.Body, n.Foot} {
				for _, r := range rows {
					for _, c := range r.Cells {
						forEachInlines(c.Blocks, fn)
					}
				}
			}
		case *ast.Figure:
			n.Caption = visitIns(n.Caption)
		case *ast.LineBlock:
			for i := range n.Lines {
				n.Lines[i] = visitIns(n.Lines[i])
			}
		case *ast.ReferenceList:
			for i := range n.Entries {
				n.Entries[i].Inlines = visitIns(n.Entries[i].Inlines)
			}
		}
	}
}
