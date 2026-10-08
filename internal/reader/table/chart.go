package table

import (
	"encoding/xml"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

const maxChartPoints = 100000

type chartSeries struct {
	name                    string
	cats, vals              map[int]string
	n                       int // number of points
	nameRef, catRef, valRef string
}

type chartInfo struct {
	title, catTitle string
	autoDeleted     bool
	series          []*chartSeries
}

// parseChart extracts the title and the cached series data of a
// DrawingML chart part.
func parseChart(data []byte) *chartInfo {
	ci := &chartInfo{}
	d := newDecoder(data)
	var (
		stack      []string
		ser        *chartSeries
		text       strings.Builder
		pt         = -1
		titleOwner string
		titleParts []string
		para       strings.Builder
		valFormat  string
		catFormat  string
		lvl        int
	)
	in := func(name string) bool { return slices.Contains(stack, name) }
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, name)
			switch name {
			case "ser":
				ser = &chartSeries{cats: map[int]string{}, vals: map[int]string{}}
				ci.series = append(ci.series, ser)
			case "title":
				titleOwner, titleParts = parent, nil
			case "p":
				para.Reset()
			case "pt":
				pt, _ = strconv.Atoi(attr(t, "idx"))
			case "cat", "xVal":
				lvl, catFormat = 0, ""
			case "lvl":
				lvl++
			case "val", "yVal":
				valFormat = ""
			case "autoTitleDeleted":
				v := attr(t, "val")
				ci.autoDeleted = v == "1" || v == "true" || v == ""
			case "v", "t", "formatCode", "f":
				text.Reset()
			}
		case xml.EndElement:
			name := t.Name.Local
			switch name {
			case "t":
				if titleOwner != "" {
					para.WriteString(text.String())
				}
			case "p":
				if titleOwner != "" && para.Len() > 0 {
					titleParts = append(titleParts, para.String())
				}
			case "formatCode":
				if in("cat") || in("xVal") {
					catFormat = text.String()
				} else {
					valFormat = text.String()
				}
			case "v":
				v := strings.TrimSpace(text.String())
				switch {
				case titleOwner != "" && !in("ser"):
					titleParts = append(titleParts, v)
				case ser == nil || !in("ser"):
				case in("tx"):
					ser.name = v
				case pt < 0 || pt >= maxChartPoints:
				case in("cat") || in("xVal"):
					if lvl <= 1 {
						ser.cats[pt] = chartValue(v, catFormat)
						ser.n = max(ser.n, pt+1)
					}
				case in("val") || in("yVal"):
					ser.vals[pt] = chartValue(v, valFormat)
					ser.n = max(ser.n, pt+1)
				}
			case "f":
				if ser != nil && in("ser") {
					ref := strings.TrimSpace(text.String())
					switch {
					case in("tx"):
						ser.nameRef = ref
					case in("cat") || in("xVal"):
						ser.catRef = ref
					case in("val") || in("yVal"):
						ser.valRef = ref
					}
				}
			case "pt":
				pt = -1
			case "title":
				title := strings.Join(titleParts, " ")
				switch titleOwner {
				case "chart":
					ci.title = title
				case "catAx", "dateAx":
					ci.catTitle = title
				}
				titleOwner = ""
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			text.Write(t)
		}
	}
	return ci
}

// resolveSeries fills series data from the referenced cells when the
// chart part has no cached values (some generators never write them).
func resolveSeries(s *chartSeries, lookup func(ref string) []string) {
	if s.name == "" && s.nameRef != "" {
		s.name = strings.Join(slices.DeleteFunc(lookup(s.nameRef), func(v string) bool { return v == "" }), " ")
	}
	fill := func(m map[int]string, ref string) {
		if len(m) > 0 || ref == "" {
			return
		}
		for i, v := range lookup(ref) {
			if v != "" {
				m[i] = v
			}
			s.n = max(s.n, i+1)
		}
	}
	fill(s.cats, s.catRef)
	fill(s.vals, s.valRef)
}

func chartValue(v, format string) string {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	if format == "" {
		format = "General"
	}
	s, _ := formatNumber(f, format, false)
	return s
}

// chartTable renders chart data as a table: one row per category, one
// column per series, captioned with the chart title.
func chartTable(ci *chartInfo, lookup func(ref string) []string) *ast.Table {
	if lookup != nil {
		for _, s := range ci.series {
			resolveSeries(s, lookup)
		}
	}
	n := 0
	var cats map[int]string
	for _, s := range ci.series {
		n = max(n, s.n)
		if cats == nil && len(s.cats) > 0 {
			cats = s.cats
		}
	}
	if n == 0 {
		return nil
	}
	header := []string{ci.catTitle}
	for _, s := range ci.series {
		header = append(header, s.name)
	}
	records := [][]string{header}
	for i := range n {
		row := []string{cats[i]}
		if cats == nil {
			row[0] = strconv.Itoa(i + 1)
		}
		empty := cats[i] == ""
		for _, s := range ci.series {
			row = append(row, s.vals[i])
			empty = empty && s.vals[i] == ""
		}
		if empty {
			continue // hidden or blank source rows are not plotted
		}
		records = append(records, row)
	}
	if len(records) < 2 {
		return nil
	}
	tbl := denseTable(records)
	switch {
	case ci.title != "":
		tbl.Caption = ast.Str(ci.title)
	case len(ci.series) == 1 && !ci.autoDeleted && ci.series[0].name != "":
		// Spreadsheet applications show a lone series' name as the title.
		tbl.Caption = ast.Str(ci.series[0].name)
	}
	return tbl
}

// drawItem is a chart or picture anchored on a sheet.
type drawItem struct {
	row, col int
	block    ast.Block
}

// drawing renders the charts and pictures of a drawing part in anchor
// order.
func (x *xlsxReader) drawing(part string) []ast.Block {
	data, err := x.z.Read(part)
	if err != nil {
		return nil
	}
	rels := readRels(x.z, part)
	d := newDecoder(data)
	var (
		items      []drawItem
		row, col   int
		inFrom     bool
		field      string
		num        strings.Builder
		inPic      bool
		alt, embed string
		cx, cy     int64
		altStart   = -1
	)
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "twoCellAnchor", "oneCellAnchor", "absoluteAnchor":
				row, col = 0, 0
			case "from":
				inFrom = true
			case "row", "col":
				if inFrom {
					field = t.Name.Local
					num.Reset()
				}
			case "pic":
				inPic, alt, embed, cx, cy = true, "", "", 0, 0
			case "cNvPr":
				if inPic {
					alt = attr(t, "descr")
					if alt == "" {
						alt = attr(t, "title")
					}
				}
			case "blip":
				if inPic {
					embed = attr(t, "embed")
				}
			case "ext":
				if w, err := strconv.ParseInt(attr(t, "cx"), 10, 64); err == nil && inPic {
					cx = w
					cy, _ = strconv.ParseInt(attr(t, "cy"), 10, 64)
				}
			case "chart":
				if rel, ok := rels[attr(t, "id")]; ok {
					if b := x.chart(rel); b != nil {
						items = append(items, drawItem{row, col, b})
					}
				}
			case "AlternateContent":
				altStart = len(items)
			case "Fallback":
				if altStart >= 0 && len(items) > altStart {
					skipElement(d) // the Choice branch already produced content
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "from":
				inFrom = false
			case "row", "col":
				if field == t.Name.Local {
					n, _ := strconv.Atoi(strings.TrimSpace(num.String()))
					if field == "row" {
						row = n
					} else {
						col = n
					}
					field = ""
				}
			case "pic":
				inPic = false
				if rel, ok := rels[embed]; ok && !rel.external {
					if b := x.picture(rel.target, alt, cx, cy); b != nil {
						items = append(items, drawItem{row, col, b})
					}
				}
			case "AlternateContent":
				altStart = -1
			}
		case xml.CharData:
			if field != "" {
				num.Write(t)
			}
		}
	}
	slices.SortStableFunc(items, func(a, b drawItem) int {
		if a.row != b.row {
			return a.row - b.row
		}
		return a.col - b.col
	})
	out := make([]ast.Block, 0, len(items))
	for _, it := range items {
		out = append(out, it.block)
	}
	return out
}

func (x *xlsxReader) chart(rel relationship) ast.Block {
	if rel.external {
		return nil
	}
	if !relType(rel, "chart") {
		x.warn.Addf("a chart of an unsupported type was dropped")
		return nil
	}
	data, err := x.z.Read(rel.target)
	if err != nil {
		return nil
	}
	ci := parseChart(data)
	tbl := chartTable(ci, x.lookup)
	if tbl == nil {
		name := ci.title
		if name == "" {
			name = path.Base(rel.target)
		}
		x.warn.Addf("chart %q has no cached data and was dropped", name)
		return nil
	}
	return tbl
}

// picture stores an embedded image and returns a figure for it.
func (x *xlsxReader) picture(target, alt string, cx, cy int64) ast.Block {
	src, ok := x.images[target]
	if !ok {
		data, err := x.z.Read(target)
		if err != nil {
			return nil
		}
		mt := rd.MediaTypeFromName(target)
		if mt == "" {
			x.warn.Addf("picture %q has an unknown format and was dropped", path.Base(target))
			return nil
		}
		src = x.doc.Resources.Add("xlsx/"+path.Base(target), mt, data)
		if x.images == nil {
			x.images = map[string]string{}
		}
		x.images[target] = src
	}
	img := &ast.Image{Src: src, Alt: strings.TrimSpace(alt)}
	if cx > 0 && cy > 0 {
		img.Width, img.Height = emuToPt(cx), emuToPt(cy)
	}
	return &ast.Figure{Image: img}
}

func emuToPt(v int64) string {
	return strconv.FormatFloat(float64(v)/12700, 'f', 1, 64) + "pt"
}

// skipElement consumes tokens up to the end of the current element.
func skipElement(d *xml.Decoder) {
	depth := 1
	for depth > 0 {
		tok, err := d.RawToken()
		if err != nil {
			return
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
}
