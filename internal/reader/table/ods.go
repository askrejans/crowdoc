package table

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// maxColumns bounds column repetition (the ODF and OOXML column limit).
const maxColumns = 16384

// ReadODS parses an OpenDocument spreadsheet (.ods). Each visible table
// becomes a section; embedded charts become data tables and pictures
// figures.
func ReadODS(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	if bytes.HasPrefix(data, oleMagic) {
		return nil, nil, errors.New("not an OpenDocument spreadsheet (legacy binary file)")
	}
	z, err := rd.OpenZip(data, o.Limits)
	if err != nil {
		return nil, nil, fmt.Errorf("not a valid ODS file: %w", err)
	}
	content, err := z.Read("content.xml")
	if err != nil {
		if errors.Is(err, rd.ErrLimit) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("not a valid ODS file: %w", err)
	}
	r := &odsReader{
		ctx:    ctx,
		z:      z,
		doc:    &ast.Document{Resources: ast.NewResources()},
		budget: o.Limits.Normalized().MaxTableCells,
		max:    o.Limits.Normalized().MaxTableCells,
	}
	readCoreProps(z, "meta.xml", &r.doc.Meta)
	sheets, err := r.parse(content, true)
	if err != nil {
		return nil, nil, err
	}
	visible := 0
	for _, s := range sheets {
		if !s.hidden {
			visible++
		}
	}
	level := 1
	if visible > 1 {
		level = 2
	}
	for _, s := range sheets {
		if s.hidden {
			r.warn.Addf("sheet %q is hidden and was skipped", s.name)
			continue
		}
		var blocks []ast.Block
		if g, truncated := buildGrid(s.cells, r.max); g != nil {
			if truncated {
				r.warn.Addf("sheet %q is too large and was truncated", s.name)
			}
			blocks = sheetBlocks(&s.sheet, g, level)
		}
		blocks = append(blocks, s.extras...)
		if len(blocks) == 0 {
			continue
		}
		if visible > 1 {
			r.doc.Blocks = append(r.doc.Blocks, &ast.Heading{Level: 1, Inlines: ast.Str(s.name)})
		}
		r.doc.Blocks = append(r.doc.Blocks, blocks...)
	}
	if len(r.doc.Blocks) == 0 {
		r.warn.Addf("the spreadsheet contains no data")
		r.doc.Blocks = []ast.Block{&ast.Para{}} // nothing to show; the warning explains
	}
	return r.doc, r.warn.List(), nil
}

type odsReader struct {
	ctx    context.Context
	z      *rd.Zip
	doc    *ast.Document
	warn   rd.Warnings
	budget int
	max    int
	images map[string]string
}

type odsSheet struct {
	sheet
	hidden bool
	title  string // chart objects: the chart title
}

// odsCell is a cell being read.
type odsCell struct {
	repeat, colSpan, rowSpan int
	covered                  bool
	valueType                string
	value, dateValue         string
	timeValue, boolValue     string
	stringValue              string
	text                     strings.Builder
	paras                    int
	link                     string
}

// parse reads the tables of an ODF content part. Charts and pictures are
// collected only for top-level documents (not for chart objects).
func (r *odsReader) parse(content []byte, top bool) ([]*odsSheet, error) {
	d := newDecoder(content)
	hiddenStyles := map[string]bool{}
	var (
		sheets     []*odsSheet
		cur        *odsSheet
		depth      int // table nesting
		styleName  string
		hiddenCols []colRange
		colIdx     int
		rowIdx     int
		rowRepeat  int
		rowHidden  bool
		inHeader   bool
		headerRow  bool
		rowCells   []sheetCell
		cell       *odsCell
		inP        int
		frame      *odsFrame
		chartTitle strings.Builder
		inTitle    bool
		tokens     int
	)
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		if tokens++; tokens&8191 == 0 {
			if err := r.ctx.Err(); err != nil {
				return nil, err
			}
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "style":
				if attr(t, "family") == "table" {
					styleName = attr(t, "name")
				} else {
					styleName = ""
				}
			case "table-properties":
				if styleName != "" && attr(t, "display") == "false" {
					hiddenStyles[styleName] = true
				}
			case "table":
				depth++
				if depth > 1 {
					continue
				}
				cur = &odsSheet{sheet: sheet{name: attr(t, "name")}, hidden: hiddenStyles[attr(t, "style-name")]}
				sheets = append(sheets, cur)
				hiddenCols, colIdx, rowIdx = nil, 0, 0
			case "table-column":
				if depth != 1 {
					continue
				}
				n := repeatAttr(t, "number-columns-repeated")
				if v := attr(t, "visibility"); v == "collapse" || v == "filter" {
					hiddenCols = append(hiddenCols, colRange{colIdx, colIdx + n - 1})
				}
				colIdx = min(colIdx+n, maxColumns)
			case "table-header-rows":
				inHeader = depth == 1
			case "table-row":
				if depth != 1 {
					continue
				}
				rowRepeat = repeatAttr(t, "number-rows-repeated")
				v := attr(t, "visibility")
				rowHidden = v == "collapse" || v == "filter"
				headerRow = inHeader
				colIdx, rowCells = 0, rowCells[:0]
			case "table-cell", "covered-table-cell":
				if depth != 1 {
					continue
				}
				cell = &odsCell{
					repeat:      repeatAttr(t, "number-columns-repeated"),
					colSpan:     repeatAttr(t, "number-columns-spanned"),
					rowSpan:     repeatAttr(t, "number-rows-spanned"),
					covered:     t.Name.Local == "covered-table-cell",
					valueType:   attr(t, "value-type"),
					value:       attr(t, "value"),
					dateValue:   attr(t, "date-value"),
					timeValue:   attr(t, "time-value"),
					boolValue:   attr(t, "boolean-value"),
					stringValue: attr(t, "string-value"),
				}
			case "annotation", "note":
				skipElement(d)
			case "p", "h":
				if cell != nil {
					if cell.paras > 0 {
						cell.text.WriteByte('\n')
					}
					cell.paras++
					inP++
				} else if inTitle {
					inP++
				}
			case "span":
			case "s":
				if inP > 0 && cell != nil {
					cell.text.WriteString(strings.Repeat(" ", min(repeatAttr(t, "c"), 100)))
				}
			case "tab":
				if inP > 0 && cell != nil {
					cell.text.WriteByte('\t')
				}
			case "line-break":
				if inP > 0 && cell != nil {
					cell.text.WriteByte('\n')
				}
			case "a":
				if cell != nil && cell.link == "" {
					cell.link = attr(t, "href")
				}
			case "frame":
				if top && cur != nil {
					frame = &odsFrame{width: attr(t, "width"), height: attr(t, "height")}
				}
			case "image":
				if frame != nil && frame.image == "" {
					frame.image = attr(t, "href")
				}
			case "object":
				if frame != nil {
					frame.object = attr(t, "href")
				}
			case "title", "desc":
				if frame != nil {
					frame.textTarget = t.Name.Local
				}
				// A chart object's first title is the chart title; axis
				// titles follow it.
				if t.Name.Local == "title" && !top && cur == nil && chartTitle.Len() == 0 {
					inTitle = true
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "table":
				depth--
				if depth == 0 {
					cur = nil
				}
			case "table-header-rows":
				inHeader = false
			case "p", "h":
				if inP > 0 {
					inP--
				}
			case "title", "desc":
				if frame != nil {
					frame.textTarget = ""
				}
				if t.Name.Local == "title" {
					inTitle = false
				}
			case "table-cell", "covered-table-cell":
				if cell == nil || depth != 1 {
					continue
				}
				rowCells = r.addCell(cell, colIdx, rowCells, hiddenCols)
				colIdx = min(colIdx+cell.repeat, maxColumns)
				cell = nil
			case "table-row":
				if cur == nil || depth != 1 {
					continue
				}
				if !rowHidden && len(rowCells) > 0 {
					for k := range rowRepeat {
						if r.budget < len(rowCells) {
							r.warn.Addf("the spreadsheet has too many cells; the rest was dropped")
							break
						}
						r.budget -= len(rowCells)
						for _, c := range rowCells {
							c.row = rowIdx + k
							cur.cells = append(cur.cells, c)
						}
					}
					if headerRow && rowIdx == cur.headerRows {
						cur.headerRows += rowRepeat
					}
				}
				rowIdx += rowRepeat
			case "frame":
				if frame != nil && cur != nil {
					if b := r.frame(frame); b != nil {
						cur.extras = append(cur.extras, b)
					}
				}
				frame = nil
			}
		case xml.CharData:
			switch {
			case cell != nil && inP > 0:
				cell.text.Write(t)
			case frame != nil && frame.textTarget != "":
				frame.alt += string(t)
			case inTitle && inP > 0:
				chartTitle.Write(t)
			}
		}
	}
	if !top && len(sheets) > 0 {
		sheets[0].title = strings.TrimSpace(chartTitle.String())
	}
	return sheets, nil
}

// repeatAttr reads a positive count attribute (default 1).
func repeatAttr(se xml.StartElement, local string) int {
	n, err := strconv.Atoi(attr(se, local))
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// addCell materialises a (possibly repeated) cell of the current row.
func (r *odsReader) addCell(c *odsCell, col int, row []sheetCell, hidden []colRange) []sheetCell {
	if c.covered {
		return row
	}
	text, kind := c.display()
	if text == "" {
		return row
	}
	link := ""
	if l := strings.TrimSpace(c.link); strings.Contains(l, "://") || strings.HasPrefix(l, "mailto:") {
		link = l
	}
	for k := 0; k < min(c.repeat, maxColumns-col); k++ {
		if inRanges(hidden, col+k) {
			continue
		}
		sc := sheetCell{col: col + k, text: text, kind: kind, link: link}
		if c.colSpan > 1 {
			sc.colSpan = c.colSpan
		}
		if c.rowSpan > 1 {
			sc.rowSpan = c.rowSpan
		}
		row = append(row, sc)
	}
	return row
}

// display returns a cell's text: the formatted text the application
// stored, or a rendering of the typed value when there is none.
func (c *odsCell) display() (string, cellKind) {
	kind := kText
	switch c.valueType {
	case "float", "percentage", "currency":
		kind = kNumber
	case "date", "time":
		kind = kDate
	case "boolean":
		kind = kBool
	}
	if text := strings.TrimSpace(c.text.String()); text != "" {
		return text, kind
	}
	switch c.valueType {
	case "float", "currency":
		if f, err := strconv.ParseFloat(c.value, 64); err == nil {
			return generalNumber(f), kind
		}
	case "percentage":
		if f, err := strconv.ParseFloat(c.value, 64); err == nil {
			return generalNumber(f*100) + "%", kind
		}
	case "date":
		return isoDateTime(c.dateValue), kind
	case "time":
		return isoDuration(c.timeValue), kind
	case "boolean":
		if c.boolValue == "true" {
			return "TRUE", kind
		}
		return "FALSE", kind
	case "string":
		return strings.TrimSpace(c.stringValue), kind
	}
	return "", kind
}

// isoDuration renders an ODF time value ("PT10H30M00S") as "10:30".
func isoDuration(s string) string {
	rest, ok := strings.CutPrefix(strings.ToUpper(s), "PT")
	if !ok {
		return s
	}
	var h, m int
	var sec float64
	for _, unit := range []byte{'H', 'M', 'S'} {
		v, after, found := strings.Cut(rest, string(unit))
		if !found {
			continue
		}
		switch unit {
		case 'H':
			h, _ = strconv.Atoi(v)
		case 'M':
			m, _ = strconv.Atoi(v)
		case 'S':
			sec, _ = strconv.ParseFloat(v, 64)
		}
		rest = after
	}
	out := strconv.Itoa(h) + ":" + pad2(int64(m))
	if s := int64(sec + 0.5); s > 0 {
		out += ":" + pad2(s)
	}
	return out
}

type odsFrame struct {
	width, height string
	image, object string
	alt           string
	textTarget    string
}

// frame renders a draw:frame holding a picture or a chart object.
func (r *odsReader) frame(f *odsFrame) ast.Block {
	if f.object != "" {
		if b := r.chartObject(f.object); b != nil {
			return b
		}
	}
	target := strings.TrimPrefix(f.image, "./")
	if target == "" || strings.Contains(target, "://") {
		return nil
	}
	src, ok := r.images[target]
	if !ok {
		data, err := r.z.Read(target)
		if err != nil {
			return nil
		}
		mt := rd.MediaTypeFromName(target)
		if mt == "" {
			r.warn.Addf("picture %q has an unknown format and was dropped", path.Base(target))
			return nil
		}
		src = r.doc.Resources.Add("ods/"+path.Base(target), mt, data)
		if r.images == nil {
			r.images = map[string]string{}
		}
		r.images[target] = src
	}
	img := &ast.Image{Src: src, Alt: strings.TrimSpace(f.alt), Width: lengthPt(f.width), Height: lengthPt(f.height)}
	if img.Width == "" || img.Height == "" {
		img.Width, img.Height = "", ""
	}
	return &ast.Figure{Image: img}
}

// chartObject renders an embedded chart from the data table cached in the
// chart object.
func (r *odsReader) chartObject(href string) ast.Block {
	dir := strings.TrimSuffix(strings.TrimPrefix(href, "./"), "/")
	content, err := r.z.Read(dir + "/content.xml")
	if err != nil {
		return nil
	}
	sub := &odsReader{ctx: r.ctx, z: r.z, doc: r.doc, budget: min(r.budget, maxChartPoints), max: maxChartPoints}
	sheets, err := sub.parse(content, false)
	if err != nil || len(sheets) == 0 || !bytes.Contains(content, []byte("chart:chart")) {
		return nil
	}
	s := sheets[0]
	g, _ := buildGrid(s.cells, maxChartPoints)
	if g == nil {
		r.warn.Addf("chart %q has no cached data and was dropped", dir)
		return nil
	}
	tbl := g.table(0, len(g.cells), 1)
	if s.title != "" {
		tbl.Caption = ast.Str(s.title)
	}
	return tbl
}

// lengthPt converts an ODF length ("4.5cm", "2in", "120pt") to points.
func lengthPt(s string) string {
	s = strings.TrimSpace(s)
	units := []struct {
		suffix string
		factor float64
	}{{"cm", 72 / 2.54}, {"mm", 72 / 25.4}, {"in", 72}, {"pt", 1}, {"pc", 12}, {"px", 0.75}}
	for _, u := range units {
		if v, ok := strings.CutSuffix(s, u.suffix); ok {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil || f <= 0 {
				return ""
			}
			return strconv.FormatFloat(f*u.factor, 'f', 1, 64) + "pt"
		}
	}
	return ""
}
