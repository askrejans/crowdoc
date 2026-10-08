package docx

import (
	"bytes"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// drawing handles a w:drawing: pictures, charts, SmartArt and text boxes.
func (pb *paraBuilder) drawing(n *node) {
	for _, k := range n.kids {
		switch k.name {
		case "inline":
			pb.drawingObject(k, false)
		case "anchor":
			pb.drawingObject(k, true)
		}
	}
}

func (pb *paraBuilder) drawingObject(d *node, anchored bool) {
	if pb.suppressed() {
		return
	}
	docPr := d.child("docPr")
	if truthy(docPr.attr("hidden")) {
		return
	}
	alt := altText(docPr.attr("descr"))
	title := altText(docPr.attr("title"))
	if alt == "" {
		alt = title
	}
	var images []*ast.Image
	var boxes []*node
	var charts []string
	smartArt := false
	walk(d.child("graphic"), func(x *node) bool {
		switch {
		case x.name == "blip" && x.ns == "a":
			if img := pb.blipImage(x); img != nil {
				images = append(images, img)
			}
			return false
		case x.name == "chart" && x.ns == "c":
			charts = append(charts, x.attrNS("r", "id"))
			return false
		case x.name == "relIds" && x.ns == "dgm":
			smartArt = true
			return false
		case x.name == "txbxContent":
			boxes = append(boxes, x)
			return false
		}
		return true
	})
	if len(images) == 1 {
		ext := d.child("extent")
		images[0].Width = emuToPt(ext.attr("cx"))
		images[0].Height = emuToPt(ext.attr("cy"))
	}
	for _, img := range images {
		img.Alt, img.Title = alt, title
		pb.placeImage(img, anchored)
	}
	for _, rid := range charts {
		if t := pb.r.chartTable(pb.s.part, rid); t != nil {
			pb.after = append(pb.after, item{kind: kBlock, block: t, table: t, chart: true})
		}
	}
	if smartArt && len(images) == 0 {
		pb.r.warn.Addf("SmartArt graphic was dropped (no image fallback)")
	}
	for _, box := range boxes {
		pb.textBox(box)
	}
}

// placeImage emits an inline image, or a figure after the paragraph for
// floating images in the main flow.
func (pb *paraBuilder) placeImage(img *ast.Image, anchored bool) {
	if anchored && !pb.bc.inCell {
		fig := &ast.Figure{Image: img}
		pb.after = append(pb.after, item{kind: kBlock, block: fig, figure: fig})
		return
	}
	pb.emit(seg{node: img})
}

// textBox emits the content of a text box after the paragraph.
func (pb *paraBuilder) textBox(box *node) {
	s := pb.r.newStory(pb.s.part, pb.s.body)
	pb.after = append(pb.after, pb.r.blockItems(s, box.kids, pb.bc)...)
}

func (pb *paraBuilder) blipImage(blip *node) *ast.Image {
	if rid := blip.attrNS("r", "embed"); rid != "" {
		if img := pb.r.relImage(pb.s.part, rid); img != nil {
			return img
		}
	}
	if rid := blip.attrNS("r", "link"); rid != "" {
		return pb.r.relImage(pb.s.part, rid)
	}
	return nil
}

// pict handles legacy VML pictures, text boxes and horizontal rules.
func (pb *paraBuilder) pict(n *node) {
	if pb.suppressed() {
		return
	}
	pb.vml(n, "")
}

func (pb *paraBuilder) vml(n *node, style string) {
	for _, k := range n.kids {
		switch k.name {
		case "AlternateContent":
			if alt := chooseAlternate(k); alt != nil {
				pb.vml(alt, style)
			}
		case "imagedata":
			rid := k.attrNS("r", "id")
			if rid == "" {
				rid = k.attrNS("o", "relid")
			}
			if rid == "" {
				rid = k.attrNS("r", "href")
			}
			if rid == "" {
				continue
			}
			img := pb.r.relImage(pb.s.part, rid)
			if img == nil {
				continue
			}
			img.Alt = altText(k.attrNS("o", "title"))
			img.Width, img.Height = vmlSize(style)
			pb.emit(seg{node: img})
		case "txbxContent":
			pb.textBox(k)
		case "OLEObject":
		default:
			st := style
			if s := k.attr("style"); s != "" && k.ns == "v" {
				st = s
			}
			if k.name == "rect" && k.ns == "v" && strings.EqualFold(k.attrNS("o", "hr"), "t") {
				pb.after = append(pb.after, item{kind: kBlock, block: &ast.HorizontalRule{}})
				continue
			}
			pb.vml(k, st)
		}
	}
}

// object handles embedded OLE objects through their preview image.
func (pb *paraBuilder) object(n *node) {
	if pb.suppressed() {
		return
	}
	before := len(pb.root)
	for _, f := range pb.s.frames {
		before += len(f.segs)
	}
	pb.vml(n, "")
	after := len(pb.root)
	for _, f := range pb.s.frames {
		after += len(f.segs)
	}
	prog := n.find("OLEObject").attr("ProgID")
	switch {
	case after == before:
		if prog == "" {
			prog = "unknown type"
		}
		pb.r.warn.Addf("embedded object (%s) without a preview image was dropped", prog)
	case strings.HasPrefix(prog, "Equation."):
		pb.r.warn.Addf("legacy equation object (%s) kept as an image", prog)
	}
}

// altText cleans an image description. Older word processors stored the
// path of the inserted file there, which is not a description.
func altText(s string) string {
	s = strings.TrimSpace(rd.CleanText(s))
	l := strings.ToLower(s)
	switch {
	case len(s) > 2 && s[1] == ':' && (s[2] == '\\' || s[2] == '/'),
		strings.HasPrefix(s, `\\`), strings.HasPrefix(l, "file:"),
		strings.HasPrefix(s, "/") && rd.MediaTypeFromName(s) != "":
		return ""
	}
	return s
}

// relImage resolves an image relationship of part p.
func (r *reader) relImage(p *part, rid string) *ast.Image {
	if p == nil {
		return nil
	}
	rl, ok := p.rels[rid]
	if !ok {
		r.warn.Addf("image reference %s points to a missing relationship", rid)
		return nil
	}
	if rl.external {
		u := strings.TrimSpace(rl.target)
		lu := strings.ToLower(u)
		if strings.HasPrefix(lu, "http://") || strings.HasPrefix(lu, "https://") {
			return &ast.Image{Src: u}
		}
		// Linked files (file:// URLs or paths) are resolved by the caller,
		// which decides whether the document may read outside itself.
		if strings.HasPrefix(lu, "file:") || !strings.Contains(lu, ":") || (len(u) > 2 && u[1] == ':') {
			return &ast.Image{Src: u}
		}
		r.warn.Addf("linked image %q outside the document was dropped", u)
		return nil
	}
	src := r.mediaSource(rl.target)
	if src == "" {
		return nil
	}
	return &ast.Image{Src: src}
}

// mediaSource stores an archive image as a document resource.
func (r *reader) mediaSource(zipPath string) string {
	if s, ok := r.media[zipPath]; ok {
		return s
	}
	r.media[zipPath] = ""
	data, err := r.z.Read(zipPath)
	if err != nil {
		r.warn.Addf("image %s could not be read: %v", path.Base(zipPath), err)
		return ""
	}
	mt := rd.MediaTypeFromName(zipPath)
	if mt == "" {
		mt = sniffImage(data)
	}
	if mt == "" {
		r.warn.Addf("image %s has an unknown format and was dropped", path.Base(zipPath))
		return ""
	}
	name := strings.TrimPrefix(zipPath, "word/")
	src := r.doc.Resources.Add(name, mt, data)
	r.media[zipPath] = src
	return src
}

// sniffImage identifies common image formats by signature.
func sniffImage(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG")):
		return "image/png"
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(b, []byte("GIF8")):
		return "image/gif"
	case bytes.HasPrefix(b, []byte("BM")):
		return "image/bmp"
	case bytes.HasPrefix(b, []byte("II*\x00")), bytes.HasPrefix(b, []byte("MM\x00*")):
		return "image/tiff"
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return "image/webp"
	case bytes.HasPrefix(b, []byte("%PDF")):
		return "application/pdf"
	case len(b) >= 44 && bytes.Equal(b[:4], []byte{1, 0, 0, 0}) && bytes.Equal(b[40:44], []byte(" EMF")):
		return "image/emf"
	case bytes.HasPrefix(b, []byte{0xD7, 0xCD, 0xC6, 0x9A}), bytes.HasPrefix(b, []byte{1, 0, 9, 0}):
		return "image/wmf"
	}
	head := b
	if len(head) > 1024 {
		head = head[:1024]
	}
	if bytes.Contains(head, []byte("<svg")) {
		return "image/svg+xml"
	}
	return ""
}

// emuToPt converts English Metric Units to a point length ("144pt").
func emuToPt(v string) string {
	n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || n <= 0 || math.IsInf(n, 0) || math.IsNaN(n) {
		return ""
	}
	return formatPt(n / 12700)
}

func formatPt(pt float64) string {
	pt = math.Round(pt*10) / 10
	if pt <= 0 || pt > 1e5 {
		return ""
	}
	return strconv.FormatFloat(pt, 'f', -1, 64) + "pt"
}

// vmlSize extracts width and height from a VML style attribute.
func vmlSize(style string) (w, h string) {
	for _, decl := range strings.Split(style, ";") {
		k, v, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "width":
			w = cssToPt(v)
		case "height":
			h = cssToPt(v)
		}
	}
	return w, h
}

func cssToPt(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	units := []struct {
		suffix string
		factor float64
	}{{"pt", 1}, {"in", 72}, {"cm", 72 / 2.54}, {"mm", 72 / 25.4}, {"px", 0.75}, {"pc", 12}, {"", 0.75}}
	for _, u := range units {
		if !strings.HasSuffix(v, u.suffix) {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(v, u.suffix)), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return ""
		}
		return formatPt(n * u.factor)
	}
	return ""
}

// ---------------------------------------------------------------------------
// Charts

// chartTable renders the cached data of a chart part as a table, since a
// chart without an image fallback cannot be drawn.
func (r *reader) chartTable(p *part, rid string) *ast.Table {
	rl, ok := p.rels[rid]
	if !ok || rl.external {
		r.warn.Addf("chart reference %s is missing", rid)
		return nil
	}
	root := r.optionalPart(rl.target)
	chart := root.child("chart")
	if chart == nil {
		r.warn.Addf("chart %s was dropped (no chart data)", path.Base(rl.target))
		return nil
	}
	type series struct {
		name   string
		cats   map[int]string
		vals   map[int]string
		maxIdx int
	}
	var all []series
	for _, plot := range chart.child("plotArea").kids {
		if !strings.HasSuffix(plot.name, "Chart") {
			continue
		}
		for _, ser := range plot.childrenNamed("ser") {
			s := series{name: chartText(ser.child("tx")), maxIdx: -1}
			cat := ser.child("cat")
			if cat == nil {
				cat = ser.child("xVal")
			}
			val := ser.child("val")
			if val == nil {
				val = ser.child("yVal")
			}
			s.cats = chartPoints(cat)
			s.vals = chartPoints(val)
			for idx := range s.vals {
				if idx > s.maxIdx {
					s.maxIdx = idx
				}
			}
			for idx := range s.cats {
				if idx > s.maxIdx {
					s.maxIdx = idx
				}
			}
			all = append(all, s)
		}
	}
	rows := -1
	for _, s := range all {
		if s.maxIdx > rows {
			rows = s.maxIdx
		}
	}
	if len(all) == 0 || rows < 0 {
		r.warn.Addf("chart %s was dropped (no cached data)", path.Base(rl.target))
		return nil
	}
	if rows > 10000 {
		rows = 10000
	}
	cats := all[0].cats
	for _, s := range all {
		if len(s.cats) > 0 {
			cats = s.cats
			break
		}
	}
	t := &ast.Table{Cols: make([]ast.ColSpec, len(all)+1)}
	head := ast.Row{Cells: []ast.Cell{plainCell("")}}
	for i, s := range all {
		name := s.name
		if name == "" {
			name = "Series " + strconv.Itoa(i+1)
		}
		head.Cells = append(head.Cells, plainCell(name))
	}
	t.Head = []ast.Row{head}
	for idx := 0; idx <= rows; idx++ {
		row := ast.Row{Cells: []ast.Cell{plainCell(cats[idx])}}
		for _, s := range all {
			row.Cells = append(row.Cells, plainCell(formatNumber(s.vals[idx])))
		}
		t.Body = append(t.Body, row)
	}
	for i := 1; i < len(t.Cols); i++ {
		t.Cols[i].Align = ast.AlignRight
	}
	if title := chartText(chart.child("title")); title != "" {
		t.Caption = ast.Str(title)
	}
	r.warn.Addf("chart rendered as a table of its data")
	return t
}

func plainCell(s string) ast.Cell {
	return ast.Cell{Blocks: []ast.Block{&ast.Plain{Inlines: ast.Str(strings.TrimSpace(s))}}}
}

// chartText collects the text of a chart title or series name.
func chartText(n *node) string {
	if n == nil {
		return ""
	}
	var parts []string
	walk(n, func(x *node) bool {
		if (x.name == "t" && x.ns == "a") || (x.name == "v" && x.ns == "c") {
			parts = append(parts, x.text)
			return false
		}
		return true
	})
	return strings.TrimSpace(rd.CleanText(strings.Join(parts, "")))
}

// chartPoints reads the cached points of a category or value reference.
func chartPoints(n *node) map[int]string {
	out := map[int]string{}
	if n == nil {
		return out
	}
	walk(n, func(x *node) bool {
		if x.name == "lvl" && len(out) > 0 {
			return false
		}
		if x.name == "pt" {
			if idx, err := strconv.Atoi(x.attr("idx")); err == nil && idx >= 0 && idx <= 10000 {
				if _, dup := out[idx]; !dup {
					out[idx] = strings.TrimSpace(x.child("v").text)
				}
			}
			return false
		}
		return true
	})
	return out
}

// formatNumber shortens binary floating-point noise in cached values.
func formatNumber(s string) string {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) || math.Abs(f) >= 1e15 {
		return s
	}
	rounded := math.Round(f*1e10) / 1e10
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}
