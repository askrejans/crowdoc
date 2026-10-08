package pdf

import "sort"

// pageInfo is one page with its inherited attributes resolved.
type pageInfo struct {
	d         dict
	ref       ref
	resources dict
	box       rect // crop box (visible area) in user space
	rotate    int  // 0, 90, 180 or 270
}

// pages walks the page tree in document order. It stops after maxPages
// pages and reports whether pages were dropped.
func (f *file) pages(maxPages int) ([]pageInfo, bool) {
	root, _ := f.resolve(f.trailer["Root"]).(dict)
	var out []pageInfo
	truncated := false
	visited := map[ref]bool{}
	type inherited struct {
		res      dict
		media    rect
		crop     rect
		hasCrop  bool
		rotate   int
		hasMedia bool
	}
	var walk func(v any, inh inherited, depth int)
	walk = func(v any, inh inherited, depth int) {
		if depth > 64 || truncated {
			return
		}
		r, isRef := v.(ref)
		if isRef {
			if visited[r] {
				return
			}
			visited[r] = true
		}
		d, _ := f.resolve(v).(dict)
		if d == nil {
			return
		}
		if res, ok := f.resolve(d["Resources"]).(dict); ok {
			inh.res = res
		}
		if mb, ok := rectFrom(f.resolveArray(d["MediaBox"])); ok {
			inh.media, inh.hasMedia = mb, true
		}
		if cb, ok := rectFrom(f.resolveArray(d["CropBox"])); ok {
			inh.crop, inh.hasCrop = cb, true
		}
		if rot, ok := integer(f.resolve(d["Rotate"])); ok {
			inh.rotate = ((rot % 360) + 360) % 360 / 90 * 90
		}
		kids, hasKids := f.resolve(d["Kids"]).(array)
		if d["Type"] == name("Pages") || (hasKids && d["Type"] != name("Page")) {
			for _, k := range kids {
				walk(k, inh, depth+1)
			}
			return
		}
		if len(out) >= maxPages {
			truncated = true
			return
		}
		box := rect{0, 0, 612, 792}
		if inh.hasMedia {
			box = inh.media
		}
		if inh.hasCrop {
			c := rect{max(box.x0, inh.crop.x0), max(box.y0, inh.crop.y0), min(box.x1, inh.crop.x1), min(box.y1, inh.crop.y1)}
			if c.w() > 1 && c.h() > 1 {
				box = c
			}
		}
		out = append(out, pageInfo{d: d, ref: r, resources: inh.res, box: box, rotate: inh.rotate})
	}
	if root != nil {
		walk(root["Pages"], inherited{}, 0)
	}
	if len(out) == 0 && !truncated {
		out, truncated = f.scanPages(maxPages)
	}
	return out, truncated
}

// scanPages is the fallback for broken page trees: every object of type
// /Page in object-number order.
func (f *file) scanPages(maxPages int) ([]pageInfo, bool) {
	nums := make([]int, 0, len(f.xref))
	for n := range f.xref {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var out []pageInfo
	for _, n := range nums {
		d, ok := f.getObj(n).(dict)
		if !ok || d["Type"] != name("Page") {
			continue
		}
		if len(out) >= maxPages {
			return out, true
		}
		box, ok := rectFrom(f.resolveArray(d["MediaBox"]))
		if !ok {
			box = rect{0, 0, 612, 792}
		}
		res, _ := f.resolve(d["Resources"]).(dict)
		out = append(out, pageInfo{d: d, ref: ref{n, 0}, resources: res, box: box})
	}
	return out, false
}

// resolveArray resolves an array and its numeric elements.
func (f *file) resolveArray(v any) any {
	a, ok := f.resolve(v).(array)
	if !ok {
		return nil
	}
	out := make(array, len(a))
	for i, e := range a {
		out[i] = f.resolve(e)
	}
	return out
}
