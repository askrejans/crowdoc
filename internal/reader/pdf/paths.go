package pdf

import (
	"math"

	"github.com/askrejans/crowdoc/v2/layout"
)

// Vector graphics are not extracted, with one exception: task-list check
// boxes are often drawn as small stroked squares (with a check mark path
// inside for completed items) rather than as glyphs. They are turned into
// "☐" and "☑" text so that the layout analysis sees the list markers.

// pathBox tracks the current path's extent in display coordinates.
type pathBox struct {
	active             bool
	x0, y0, x1, y1     float64
	startX, startY     float64 // user-space start of the current subpath
	curX, curY         float64
	closed             bool
	segments, subpaths int
}

// vecMark is a small stroked shape: a check box or a check mark.
type vecMark struct {
	x0, y0, x1, y1 float64
	box            bool // closed square; otherwise an open stroke
}

func (p *pathBox) add(in *interp, x, y float64) {
	dx, dy := in.toDisplay(in.gs.ctm.apply(x, y))
	if !p.active {
		p.active = true
		p.x0, p.y0, p.x1, p.y1 = dx, dy, dx, dy
		return
	}
	p.x0, p.y0 = min(p.x0, dx), min(p.y0, dy)
	p.x1, p.y1 = max(p.x1, dx), max(p.y1, dy)
}

// pathOp handles path construction and painting operators. It reports
// whether op was one of them.
func (in *interp) pathOp(op keyword, ops []any) bool {
	p := &in.path
	switch op {
	case "m":
		if v, ok := nums(ops, 2); ok {
			p.add(in, v[0], v[1])
			p.startX, p.startY, p.curX, p.curY = v[0], v[1], v[0], v[1]
			p.subpaths++
		}
	case "l":
		if v, ok := nums(ops, 2); ok {
			p.add(in, v[0], v[1])
			p.curX, p.curY = v[0], v[1]
			p.segments++
		}
	case "c":
		if v, ok := nums(ops, 6); ok {
			p.add(in, v[0], v[1])
			p.add(in, v[2], v[3])
			p.add(in, v[4], v[5])
			p.curX, p.curY = v[4], v[5]
			p.segments++
		}
	case "v", "y":
		if v, ok := nums(ops, 4); ok {
			p.add(in, v[0], v[1])
			p.add(in, v[2], v[3])
			p.curX, p.curY = v[2], v[3]
			p.segments++
		}
	case "re":
		if v, ok := nums(ops, 4); ok {
			p.add(in, v[0], v[1])
			p.add(in, v[0]+v[2], v[1]+v[3])
			p.closed = true
			p.segments += 4
			p.subpaths++
		}
	case "h":
		p.closed = true
	case "S", "s", "B", "B*", "b", "b*":
		in.strokedPath(op == "s" || op == "b" || op == "b*")
		*p = pathBox{}
	case "f", "F", "f*", "n", "W", "W*":
		if op != "W" && op != "W*" {
			*p = pathBox{}
		}
	default:
		return false
	}
	return true
}

// strokedPath classifies a stroked path as a check box or check mark.
func (in *interp) strokedPath(closes bool) {
	p := &in.path
	if !p.active || p.subpaths != 1 || len(in.out.marks) >= 2000 {
		return
	}
	w, h := p.x1-p.x0, p.y1-p.y0
	closed := p.closed || closes || math.Abs(p.curX-p.startX) < 0.5 && math.Abs(p.curY-p.startY) < 0.5
	switch {
	case closed && p.segments >= 4 && w >= 4 && w <= 16 && h >= 4 && h <= 16 && w/h > 0.75 && w/h < 1.33:
		in.out.marks = append(in.out.marks, vecMark{p.x0, p.y0, p.x1, p.y1, true})
	case !closed && p.segments >= 2 && p.segments <= 3 && w <= 14 && h <= 14:
		in.out.marks = append(in.out.marks, vecMark{p.x0, p.y0, p.x1, p.y1, false})
	}
}

// checkboxRuns turns check boxes that start a line of text into marker
// runs ("☐", or "☑" when a check mark is drawn inside).
func checkboxRuns(marks []vecMark, runs []layout.Run) []layout.Run {
	var out []layout.Run
	for _, b := range marks {
		if !b.box {
			continue
		}
		w, h := b.x1-b.x0, b.y1-b.y0
		var size float64
		best := math.Inf(1)
		inside := false
		for _, r := range runs {
			if r.Role == layout.RoleArtifact {
				continue
			}
			// A frame around text (a keyboard key) is no check box.
			if r.X < b.x1-0.2*w && r.X+r.W > b.x0+0.2*w && r.Y < b.y1-0.2*h && r.Y+r.H > b.y0+0.2*h {
				inside = true
				break
			}
			// The item text starts just right of the box, on its line.
			cy := r.Y + 0.55*r.H
			if d := r.X - b.x1; d >= 0 && d < 2.5*w && d < best && math.Abs(cy-(b.y0+b.y1)/2) < 0.6*h {
				size, best = r.FontSize, d
			}
		}
		if size == 0 || inside {
			continue
		}
		text := "☐"
		for _, m := range marks {
			cx, cy := (m.x0+m.x1)/2, (m.y0+m.y1)/2
			if !m.box && cx > b.x0-2 && cx < b.x1+2 && cy > b.y0-2 && cy < b.y1+2 {
				text = "☑"
				break
			}
		}
		out = append(out, layout.Run{Text: text, X: b.x0, Y: b.y1 - 0.8*size, W: w, H: size, FontSize: size, Role: layout.RoleListLabel})
	}
	return out
}
