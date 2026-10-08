package table

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

var oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// ReadXLSX parses an Office Open XML workbook (.xlsx, .xlsm). Each visible
// sheet becomes a section; charts become data tables and pictures figures.
func ReadXLSX(ctx context.Context, data []byte, o rd.Options) (*ast.Document, []string, error) {
	if bytes.HasPrefix(data, oleMagic) {
		return nil, nil, errors.New("legacy .xls workbooks are not supported; save as .xlsx")
	}
	z, err := rd.OpenZip(data, o.Limits)
	if err != nil {
		return nil, nil, fmt.Errorf("not a valid XLSX workbook: %w", err)
	}
	if z.Has("xl/workbook.bin") {
		return nil, nil, errors.New("binary .xlsb workbooks are not supported; save as .xlsx")
	}
	x := &xlsxReader{
		ctx:      ctx,
		z:        z,
		doc:      &ast.Document{Resources: ast.NewResources()},
		budget:   o.Limits.Normalized().MaxTableCells,
		maxCells: o.Limits.Normalized().MaxTableCells,
	}
	if err := x.read(); err != nil {
		return nil, nil, err
	}
	return x.doc, x.warn.List(), nil
}

type xlsxReader struct {
	ctx      context.Context
	z        *rd.Zip
	doc      *ast.Document
	warn     rd.Warnings
	budget   int // cells left before truncation
	maxCells int
	date1904 bool
	sst      []string
	numFmts  map[int]string
	xfs      []xfStyle
	images   map[string]string // media path -> resource
	sheets   []wbSheet
	rels     map[string]relationship
	parsed   map[string]*parsedSheet
}

type parsedSheet struct {
	sheet
	drawings []string
}

type xfStyle struct {
	numFmt int
	align  ast.Align
}

type wbSheet struct {
	name, rid, state string
}

func (x *xlsxReader) read() error {
	wbPath := "xl/workbook.xml"
	for _, rel := range readRels(x.z, "") {
		if relType(rel, "officeDocument") && !rel.external {
			wbPath = rel.target
		}
	}
	wb, err := x.z.Read(wbPath)
	if err != nil {
		return fmt.Errorf("not a valid XLSX workbook: %w", err)
	}
	x.sheets = x.workbook(wb)
	x.rels = readRels(x.z, wbPath)
	sstPath, stylesPath := "xl/sharedStrings.xml", "xl/styles.xml"
	for _, rel := range x.rels {
		switch {
		case relType(rel, "sharedStrings"):
			sstPath = rel.target
		case relType(rel, "styles"):
			stylesPath = rel.target
		}
	}
	x.sharedStrings(sstPath)
	x.styles(stylesPath)
	readCoreProps(x.z, "docProps/core.xml", &x.doc.Meta)

	visible := 0
	for _, s := range x.sheets {
		if s.state == "hidden" || s.state == "veryHidden" {
			x.warn.Addf("sheet %q is hidden and was skipped", s.name)
			continue
		}
		visible++
	}
	level := 1
	if visible > 1 {
		level = 2
	}
	// Read every visible sheet first: charts may refer to cells of any
	// sheet when they carry no cached values.
	for _, s := range x.sheets {
		if err := x.ctx.Err(); err != nil {
			return err
		}
		if s.state != "hidden" && s.state != "veryHidden" {
			if _, err := x.sheetNamed(s.name); err != nil {
				return err
			}
		}
	}
	for _, s := range x.sheets {
		if err := x.ctx.Err(); err != nil {
			return err
		}
		if s.state == "hidden" || s.state == "veryHidden" {
			continue
		}
		rel := x.rels[s.rid]
		var blocks []ast.Block
		switch {
		case relType(rel, "worksheet"):
			if ps := x.parsed[s.name]; ps != nil {
				for _, dr := range ps.drawings {
					ps.extras = append(ps.extras, x.drawing(dr)...)
				}
				blocks = x.layout(&ps.sheet, level)
			}
		case relType(rel, "chartsheet"):
			blocks = x.chartsheet(rel.target)
		}
		if len(blocks) == 0 {
			continue
		}
		if visible > 1 {
			x.doc.Blocks = append(x.doc.Blocks, &ast.Heading{Level: 1, Inlines: ast.Str(s.name)})
		}
		x.doc.Blocks = append(x.doc.Blocks, blocks...)
	}
	if len(x.doc.Blocks) == 0 {
		x.warn.Addf("the workbook contains no data")
		x.doc.Blocks = []ast.Block{&ast.Para{}} // nothing to show; the warning explains
	}
	return nil
}

// sheetNamed returns a parsed worksheet, reading it on first use.
func (x *xlsxReader) sheetNamed(name string) (*parsedSheet, error) {
	if ps, ok := x.parsed[name]; ok {
		return ps, nil
	}
	if x.parsed == nil {
		x.parsed = map[string]*parsedSheet{}
	}
	x.parsed[name] = nil // guards against recursion
	for _, s := range x.sheets {
		if s.name != name {
			continue
		}
		rel, ok := x.rels[s.rid]
		if !ok || !relType(rel, "worksheet") {
			return nil, nil
		}
		ps, err := x.worksheet(rel.target, s.name)
		if err != nil || ps == nil {
			return nil, err
		}
		x.parsed[name] = ps
		return ps, nil
	}
	return nil, nil
}

// lookup returns the display texts of a cell range reference such as
// 'Sheet 1'!$B$2:$B$9, in row-major order.
func (x *xlsxReader) lookup(ref string) []string {
	ref = strings.Trim(strings.TrimSpace(ref), "()")
	if i := strings.IndexByte(ref, ','); i >= 0 {
		ref = ref[:i] // a union range: the first area
	}
	i := strings.LastIndexByte(ref, '!')
	if i <= 0 {
		return nil
	}
	name := ref[:i]
	if strings.HasPrefix(name, "'") && strings.HasSuffix(name, "'") && len(name) >= 2 {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	c1, r1, c2, r2, ok := rangeRef(ref[i+1:])
	if !ok || (c2-c1+1)*(r2-r1+1) > maxChartPoints {
		return nil
	}
	ps, err := x.sheetNamed(name)
	if err != nil || ps == nil {
		return nil
	}
	w := c2 - c1 + 1
	out := make([]string, w*(r2-r1+1))
	for _, c := range ps.cells {
		if c.row >= r1 && c.row <= r2 && c.col >= c1 && c.col <= c2 {
			out[(c.row-r1)*w+c.col-c1] = c.text
		}
	}
	return out
}

func (x *xlsxReader) layout(s *sheet, level int) []ast.Block {
	var blocks []ast.Block
	if g, truncated := buildGrid(s.cells, x.maxCells); g != nil {
		if truncated {
			x.warn.Addf("sheet %q is too large and was truncated", s.name)
		}
		blocks = sheetBlocks(s, g, level)
	}
	return append(blocks, s.extras...)
}

func (x *xlsxReader) workbook(data []byte) []wbSheet {
	var sheets []wbSheet
	d := newDecoder(data)
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "sheet":
			sheets = append(sheets, wbSheet{name: attr(se, "name"), rid: attr(se, "id"), state: attr(se, "state")})
		case "workbookPr":
			v := attr(se, "date1904")
			x.date1904 = v == "1" || v == "true"
		}
	}
	return sheets
}

// sharedStrings reads the shared string table; rich text runs are joined
// and phonetic guides (rPh) ignored.
func (x *xlsxReader) sharedStrings(path string) {
	data, err := x.z.Read(path)
	if err != nil {
		return
	}
	d := newDecoder(data)
	var buf strings.Builder
	inT, phonetic := false, 0
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				buf.Reset()
			case "t":
				inT = phonetic == 0
			case "rPh":
				phonetic++
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "si":
				x.sst = append(x.sst, decodeXString(buf.String()))
			case "t":
				inT = false
			case "rPh":
				phonetic--
			}
		case xml.CharData:
			if inT {
				buf.Write(t)
			}
		}
	}
}

// decodeXString resolves the _xHHHH_ escapes OOXML uses for control
// characters.
func decodeXString(s string) string {
	if !strings.Contains(s, "_x") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i+6 < len(s) && s[i] == '_' && s[i+1] == 'x' && s[i+6] == '_' {
			if n, err := strconv.ParseUint(s[i+2:i+6], 16, 16); err == nil {
				if n == '\r' {
					// _x000D_ precedes a real newline in cell text
				} else if n >= 0x20 || n == '\n' || n == '\t' {
					b.WriteRune(rune(n))
				}
				i += 6
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// styles reads number formats and cell alignment from styles.xml.
func (x *xlsxReader) styles(path string) {
	x.numFmts = map[int]string{}
	data, err := x.z.Read(path)
	if err != nil {
		return
	}
	d := newDecoder(data)
	inXfs := false
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "numFmt":
				if id, err := strconv.Atoi(attr(t, "numFmtId")); err == nil {
					x.numFmts[id] = attr(t, "formatCode")
				}
			case "cellXfs":
				inXfs = true
			case "xf":
				if inXfs {
					id, _ := strconv.Atoi(attr(t, "numFmtId"))
					x.xfs = append(x.xfs, xfStyle{numFmt: id})
				}
			case "alignment":
				if inXfs && len(x.xfs) > 0 {
					x.xfs[len(x.xfs)-1].align = alignOf(attr(t, "horizontal"))
				}
			}
		case xml.EndElement:
			if t.Name.Local == "cellXfs" {
				inXfs = false
			}
		}
	}
}

func alignOf(h string) ast.Align {
	switch h {
	case "center", "centerContinuous":
		return ast.AlignCenter
	case "right":
		return ast.AlignRight
	case "left":
		return ast.AlignLeft
	}
	return ast.AlignDefault
}

func (x *xlsxReader) formatCode(style int) (string, ast.Align) {
	if style < 0 || style >= len(x.xfs) {
		return "General", ast.AlignDefault
	}
	xf := x.xfs[style]
	if code, ok := x.numFmts[xf.numFmt]; ok {
		return code, xf.align
	}
	if code, ok := builtinFormats[xf.numFmt]; ok {
		return code, xf.align
	}
	return "General", xf.align
}

// cellRef parses "AB12" into 0-based column and row.
func cellRef(ref string) (col, row int, ok bool) {
	i := 0
	for i < len(ref) && i < 3 {
		c := ref[i] | 0x20
		if c < 'a' || c > 'z' {
			break
		}
		col = col*26 + int(c-'a') + 1
		i++
	}
	if i == 0 || i == len(ref) {
		return 0, 0, false
	}
	r, err := strconv.Atoi(strings.TrimPrefix(ref[i:], "$"))
	if err != nil || r < 1 {
		return 0, 0, false
	}
	return col - 1, r - 1, true
}

// rangeRef parses "A1:C3" (or a single cell).
func rangeRef(ref string) (c1, r1, c2, r2 int, ok bool) {
	a, b, found := strings.Cut(strings.ReplaceAll(ref, "$", ""), ":")
	if c1, r1, ok = cellRef(a); !ok {
		return
	}
	if !found {
		return c1, r1, c1, r1, true
	}
	c2, r2, ok = cellRef(b)
	return min(c1, c2), min(r1, r2), max(c1, c2), max(r1, r2), ok
}

type colRange struct{ lo, hi int }

// worksheet reads one sheet. The cell budget is shared by the workbook.
func (x *xlsxReader) worksheet(path, name string) (*parsedSheet, error) {
	data, err := x.z.Read(path)
	if err != nil {
		if errors.Is(err, rd.ErrLimit) {
			return nil, err
		}
		x.warn.Addf("sheet %q could not be read", name)
		return nil, nil
	}
	rels := readRels(x.z, path)
	ps := &parsedSheet{sheet: sheet{name: name}}
	s := &ps.sheet
	var (
		hiddenCols []colRange
		hiddenRows = map[int]bool{}
		merges     [][4]int
		links      []linkRange
		drawings   []string
		row, col   = -1, -1
		inCell     bool
		ctype      string
		cstyle     int
		inV, inT   bool
		phonetic   int
		v, is      strings.Builder
		truncated  bool
	)
	d := newDecoder(data)
	for {
		tok, err := d.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			x.warn.Addf("sheet %q is damaged; content after the error was dropped", name)
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "col":
				if attr(t, "hidden") == "1" || attr(t, "hidden") == "true" {
					lo, _ := strconv.Atoi(attr(t, "min"))
					hi, _ := strconv.Atoi(attr(t, "max"))
					hiddenCols = append(hiddenCols, colRange{lo - 1, hi - 1})
				}
			case "row":
				if r, err := strconv.Atoi(attr(t, "r")); err == nil && r > 0 {
					row = r - 1
				} else {
					row++
				}
				col = -1
				if h := attr(t, "hidden"); h == "1" || h == "true" {
					hiddenRows[row] = true
				}
			case "c":
				if c, r, ok := cellRef(attr(t, "r")); ok {
					col, row = c, r
				} else {
					col++
				}
				inCell = true
				ctype = attr(t, "t")
				cstyle, _ = strconv.Atoi(attr(t, "s"))
				v.Reset()
				is.Reset()
			case "v":
				inV = inCell
			case "t":
				inT = inCell && phonetic == 0
			case "rPh":
				phonetic++
			case "mergeCell":
				if c1, r1, c2, r2, ok := rangeRef(attr(t, "ref")); ok {
					merges = append(merges, [4]int{c1, r1, c2, r2})
				}
			case "hyperlink":
				if rel, ok := rels[attr(t, "id")]; ok && rel.external {
					if c1, r1, c2, r2, ok := rangeRef(attr(t, "ref")); ok {
						links = append(links, linkRange{c1, r1, c2, r2, rel.target})
					}
				}
			case "drawing":
				if rel, ok := rels[attr(t, "id")]; ok {
					drawings = append(drawings, rel.target)
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v":
				inV = false
			case "t":
				inT = false
			case "rPh":
				phonetic--
			case "c":
				inCell = false
				if truncated || hiddenRows[row] || inRanges(hiddenCols, col) {
					continue
				}
				text, kind, align := x.cellValue(ctype, cstyle, v.String(), is.String())
				if text == "" {
					continue
				}
				if x.budget <= 0 {
					truncated = true
					x.warn.Addf("the workbook has too many cells; the rest was dropped")
					continue
				}
				x.budget--
				s.cells = append(s.cells, sheetCell{row: row, col: col, text: text, kind: kind, align: align})
			}
		case xml.CharData:
			switch {
			case inV:
				v.Write(t)
			case inT:
				is.Write(t)
			}
		}
	}
	applyMerges(s, merges, hiddenRows, hiddenCols)
	applyLinks(s, links)
	ps.drawings = drawings
	return ps, nil
}

func inRanges(rs []colRange, c int) bool {
	for _, r := range rs {
		if c >= r.lo && c <= r.hi {
			return true
		}
	}
	return false
}

// cellValue converts a raw cell into display text.
func (x *xlsxReader) cellValue(typ string, style int, v, inline string) (string, cellKind, ast.Align) {
	code, align := x.formatCode(style)
	switch typ {
	case "s":
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || i < 0 || i >= len(x.sst) {
			return strings.TrimSpace(v), kText, align
		}
		return strings.TrimSpace(x.sst[i]), kText, align
	case "inlineStr":
		return strings.TrimSpace(decodeXString(inline)), kText, align
	case "str":
		return strings.TrimSpace(decodeXString(v)), kText, align
	case "b":
		switch strings.TrimSpace(v) {
		case "1", "true":
			return "TRUE", kBool, align
		case "0", "false":
			return "FALSE", kBool, align
		}
		return "", kBool, align
	case "e":
		return strings.TrimSpace(v), kError, align
	case "d":
		return isoDateTime(v), kDate, align
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", kText, align
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v, kText, align
	}
	text, kind := formatNumber(f, code, x.date1904)
	return text, kind, align
}

func applyMerges(s *sheet, merges [][4]int, hiddenRows map[int]bool, hiddenCols []colRange) {
	if len(merges) == 0 {
		return
	}
	at := make(map[[2]int]int, len(s.cells))
	for i, c := range s.cells {
		at[[2]int{c.row, c.col}] = i
	}
	for _, m := range merges {
		c1, r1, c2, r2 := m[0], m[1], m[2], m[3]
		if hiddenRows[r1] || inRanges(hiddenCols, c1) {
			continue
		}
		if i, ok := at[[2]int{r1, c1}]; ok {
			s.cells[i].colSpan = c2 - c1 + 1
			s.cells[i].rowSpan = r2 - r1 + 1
		}
	}
}

type linkRange struct {
	c1, r1, c2, r2 int
	url            string
}

func applyLinks(s *sheet, links []linkRange) {
	for _, l := range links {
		u, err := url.Parse(l.url)
		if err != nil || u.Scheme == "" || strings.EqualFold(u.Scheme, "file") {
			continue
		}
		for i := range s.cells {
			c := &s.cells[i]
			if c.row >= l.r1 && c.row <= l.r2 && c.col >= l.c1 && c.col <= l.c2 {
				c.link = l.url
			}
		}
	}
}

// chartsheet renders the charts of a sheet that holds only a chart.
func (x *xlsxReader) chartsheet(path string) []ast.Block {
	data, err := x.z.Read(path)
	if err != nil {
		return nil
	}
	rels := readRels(x.z, path)
	var out []ast.Block
	d := newDecoder(data)
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "drawing" {
			if rel, ok := rels[attr(se, "id")]; ok {
				out = append(out, x.drawing(rel.target)...)
			}
		}
	}
	return out
}

// resolvePath resolves a relationship target against the part that owns
// the relationship.
func resolvePath(part, target string) string {
	if t, err := url.PathUnescape(target); err == nil {
		target = t
	}
	return rd.ResolveZipPath(part, target)
}
