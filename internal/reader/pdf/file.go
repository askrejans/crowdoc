package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// xrefEntry locates one object: kind 1 = at a file offset, kind 2 = inside
// an object stream.
type xrefEntry struct {
	kind uint8
	off  int // file offset (kind 1) or object-stream number (kind 2)
	idx  int // index inside the object stream (kind 2)
	gen  int
}

// file is an opened PDF: cross-reference table, trailer and object cache.
// It is not safe for concurrent use.
type file struct {
	data    []byte
	hdrOff  int // position of "%PDF-"; offsets of files with a junk prefix are relative to it
	xref    map[int]xrefEntry
	trailer dict
	cache   map[int]any
	loading map[int]bool
	objstms map[int]*objStream
	sec     *security
	fonts   map[ref]*pdfFont

	repaired bool
	budget   int64 // remaining decompression budget
	maxEntry int64 // largest single decoded stream
}

type objStream struct {
	data    []byte
	offs    []int // offset of each object relative to data
	objNums []int // object number of each entry
}

var errNotPDF = errors.New("not a PDF file")

// maxObjects bounds the number of cross-reference entries we accept.
const maxObjects = 8_000_000

func openFile(data []byte, limits rd.Limits) (*file, error) {
	limits = limits.Normalized()
	f := &file{
		data:     data,
		xref:     map[int]xrefEntry{},
		cache:    map[int]any{},
		loading:  map[int]bool{},
		objstms:  map[int]*objStream{},
		budget:   limits.MaxUnpacked,
		maxEntry: limits.MaxEntry,
	}
	head := data[:min(len(data), 1024)]
	f.hdrOff = bytes.Index(head, []byte("%PDF-"))
	if f.hdrOff < 0 {
		if !bytes.Contains(data[:min(len(data), 1<<20)], []byte(" obj")) {
			return nil, errNotPDF
		}
		f.hdrOff = 0
	}
	if err := f.loadXref(); err != nil || !f.rootOK() {
		if rerr := f.repair(); rerr != nil {
			if err != nil {
				return nil, err
			}
			return nil, rerr
		}
	}
	if !f.rootOK() {
		return nil, errors.New("malformed PDF: document catalog not found")
	}
	if enc, ok := f.trailer["Encrypt"]; ok && enc != nil {
		encRef, _ := enc.(ref)
		ed, _ := f.resolve(enc).(dict)
		if ed == nil {
			return nil, errors.New("malformed PDF: unreadable encryption dictionary")
		}
		sec, err := newSecurity(ed, f.fileID(), encRef)
		if err != nil {
			return nil, err
		}
		f.sec = sec
		// Objects read before the key was known (the catalog check above)
		// were not decrypted.
		f.cache = map[int]any{}
		f.objstms = map[int]*objStream{}
	}
	return f, nil
}

func (f *file) fileID() []byte {
	a, _ := f.resolve(f.trailer["ID"]).(array)
	if len(a) > 0 {
		if s, ok := f.resolve(a[0]).(pdfString); ok {
			return []byte(s)
		}
	}
	return nil
}

func (f *file) rootOK() bool {
	if f.trailer == nil {
		return false
	}
	d, ok := f.resolve(f.trailer["Root"]).(dict)
	return ok && d != nil && d["Pages"] != nil
}

// loadXref reads the cross-reference chain starting at startxref.
func (f *file) loadXref() error {
	tail := max(0, len(f.data)-4096)
	i := bytes.LastIndex(f.data[tail:], []byte("startxref"))
	if i < 0 {
		return errors.New("malformed PDF: missing startxref")
	}
	lx := newLexer(f.data)
	lx.pos = tail + i + len("startxref")
	tok, _ := lx.next()
	off, ok := tok.(int)
	if !ok {
		return errors.New("malformed PDF: bad startxref")
	}
	seen := map[int]bool{}
	for n := 0; n < 512; n++ {
		if seen[off] {
			break
		}
		seen[off] = true
		trailer, err := f.readXrefSection(off)
		if err != nil {
			return err
		}
		f.mergeTrailer(trailer)
		if xs, ok := integer(trailer["XRefStm"]); ok && !seen[xs] {
			seen[xs] = true
			if t2, err := f.readXrefSection(xs); err == nil {
				f.mergeTrailer(t2)
			}
		}
		prev, ok := integer(trailer["Prev"])
		if !ok || prev <= 0 {
			break
		}
		off = prev
	}
	if len(f.xref) == 0 {
		return errors.New("malformed PDF: empty cross-reference table")
	}
	return nil
}

func (f *file) mergeTrailer(t dict) {
	if f.trailer == nil {
		f.trailer = dict{}
	}
	for k, v := range t {
		if _, ok := f.trailer[k]; !ok {
			f.trailer[k] = v
		}
	}
}

// readXrefSection parses a classic table or a cross-reference stream at off.
func (f *file) readXrefSection(off int) (dict, error) {
	for _, o := range []int{off, off + f.hdrOff} {
		if o < 0 || o >= len(f.data) {
			continue
		}
		lx := newLexer(f.data)
		lx.pos = o
		lx.skipSpace()
		if hasPrefixAt(f.data, lx.pos, "xref") {
			lx.pos += 4
			return f.readXrefTable(lx)
		}
		if t, ok := f.readXrefStream(lx); ok {
			return t, nil
		}
		if o == off && f.hdrOff == 0 {
			break
		}
	}
	return nil, fmt.Errorf("malformed PDF: no cross-reference section at offset %d", off)
}

func (f *file) readXrefTable(lx *lexer) (dict, error) {
	for {
		tok, ok := lx.next()
		if !ok {
			return nil, errors.New("malformed PDF: truncated cross-reference table")
		}
		if tok == keyword("trailer") {
			t, _ := lx.object(0)
			d, _ := t.(dict)
			if d == nil {
				return nil, errors.New("malformed PDF: bad trailer")
			}
			return d, nil
		}
		start, ok := tok.(int)
		if !ok {
			return nil, errors.New("malformed PDF: bad cross-reference subsection")
		}
		tok, _ = lx.next()
		count, ok := tok.(int)
		if !ok || count < 0 || start < 0 || start+count > maxObjects {
			return nil, errors.New("malformed PDF: bad cross-reference subsection")
		}
		for k := 0; k < count; k++ {
			t1, _ := lx.next()
			t2, _ := lx.next()
			t3, _ := lx.next()
			off, ok1 := t1.(int)
			gen, ok2 := t2.(int)
			kw, ok3 := t3.(keyword)
			if !ok1 || !ok2 || !ok3 {
				return nil, errors.New("malformed PDF: bad cross-reference entry")
			}
			num := start + k
			if _, exists := f.xref[num]; exists || kw != "n" || off <= 0 {
				continue
			}
			f.xref[num] = xrefEntry{kind: 1, off: off, gen: gen}
		}
	}
}

func (f *file) readXrefStream(lx *lexer) (dict, bool) {
	obj, _, ok := f.parseIndirectAt(lx.pos)
	if !ok {
		return nil, false
	}
	s, ok := obj.(*stream)
	if !ok || s.d["Type"] != name("XRef") {
		return nil, false
	}
	s.plain = true
	data, _, _, err := f.decodeStream(s, false)
	if err != nil {
		return nil, false
	}
	w, _ := s.d["W"].(array)
	if len(w) < 3 {
		return nil, false
	}
	var widths [3]int
	rowLen := 0
	for i := 0; i < 3; i++ {
		n, ok := integer(w[i])
		if !ok || n < 0 || n > 8 {
			return nil, false
		}
		widths[i] = n
		rowLen += n
	}
	if rowLen == 0 {
		return nil, false
	}
	size, _ := integer(s.d["Size"])
	index := []int{0, size}
	if ia, ok := s.d["Index"].(array); ok && len(ia) >= 2 {
		index = index[:0]
		for _, v := range ia {
			n, _ := integer(v)
			index = append(index, n)
		}
	}
	pos := 0
	for i := 0; i+1 < len(index); i += 2 {
		start, count := index[i], index[i+1]
		if start < 0 || count < 0 || start+count > maxObjects {
			break
		}
		for k := 0; k < count && pos+rowLen <= len(data); k++ {
			row := data[pos : pos+rowLen]
			pos += rowLen
			typ := 1
			if widths[0] > 0 {
				typ = int(beInt(row[:widths[0]]))
			}
			f2 := beInt(row[widths[0] : widths[0]+widths[1]])
			f3 := beInt(row[widths[0]+widths[1]:])
			num := start + k
			if _, exists := f.xref[num]; exists {
				continue
			}
			switch typ {
			case 1:
				if f2 > 0 && f2 < int64(len(f.data)) {
					f.xref[num] = xrefEntry{kind: 1, off: int(f2), gen: int(f3)}
				}
			case 2:
				f.xref[num] = xrefEntry{kind: 2, off: int(f2), idx: int(f3)}
			}
		}
	}
	return s.d, true
}

func beInt(b []byte) int64 {
	var v int64
	for _, c := range b {
		v = v<<8 | int64(c)
	}
	return v
}

// parseIndirectAt parses "num gen obj <object> [stream ...]" at pos.
func (f *file) parseIndirectAt(pos int) (any, ref, bool) {
	lx := newLexer(f.data)
	lx.pos = pos
	t1, _ := lx.next()
	t2, _ := lx.next()
	t3, _ := lx.next()
	n, ok1 := t1.(int)
	g, ok2 := t2.(int)
	if !ok1 || !ok2 || t3 != keyword("obj") {
		return nil, ref{}, false
	}
	r := ref{n, g}
	obj, _ := lx.object(0)
	if k, ok := obj.(keyword); ok && k == "endobj" {
		return nil, r, true
	}
	d, isDict := obj.(dict)
	if !isDict {
		return obj, r, true
	}
	save := lx.pos
	tok, _ := lx.next()
	if tok != keyword("stream") {
		lx.pos = save
		return d, r, true
	}
	return &stream{d: d, raw: f.streamData(d, lx.pos), ref: r}, r, true
}

// streamData locates the bytes of a stream whose "stream" keyword ends at
// pos, validating /Length against the "endstream" marker.
func (f *file) streamData(d dict, pos int) []byte {
	b := f.data
	if pos < len(b) && b[pos] == '\r' {
		pos++
	}
	if pos < len(b) && b[pos] == '\n' {
		pos++
	}
	if pos > len(b) {
		return nil
	}
	if n, ok := integer(f.resolve(d["Length"])); ok && n >= 0 && pos+n <= len(b) {
		end := pos + n
		k := end
		for k < len(b) && k < end+4 && isSpace(b[k]) {
			k++
		}
		if hasPrefixAt(b, k, "endstream") {
			return b[pos:end:end]
		}
	}
	end := indexFrom(b, pos, "endstream")
	if end < 0 {
		end = len(b)
	}
	e := end
	if e > pos && b[e-1] == '\n' {
		e--
	}
	if e > pos && b[e-1] == '\r' {
		e--
	}
	return b[pos:e:e]
}

// repair rebuilds the cross-reference table by scanning the whole file for
// "num gen obj" headers and trailer dictionaries.
func (f *file) repair() error {
	if f.repaired {
		return nil
	}
	f.repaired = true
	f.xref = map[int]xrefEntry{}
	f.cache = map[int]any{}
	f.objstms = map[int]*objStream{}
	var trailers []dict
	b := f.data
	for i := 0; i < len(b); {
		j := indexFrom(b, i, "obj")
		if j < 0 {
			break
		}
		i = j + 3
		if j >= 3 && hasPrefixAt(b, j-3, "end") {
			continue
		}
		if i < len(b) && isRegular(b[i]) {
			continue
		}
		start, num, gen, ok := objHeaderBefore(b, j)
		if !ok || num > maxObjects {
			continue
		}
		f.xref[num] = xrefEntry{kind: 1, off: start, gen: gen}
	}
	for i := 0; i < len(b); {
		j := indexFrom(b, i, "trailer")
		if j < 0 {
			break
		}
		i = j + 7
		lx := newLexer(b)
		lx.pos = i
		if t, _ := lx.object(0); t != nil {
			if d, ok := t.(dict); ok {
				trailers = append(trailers, d)
			}
		}
	}
	nums := make([]int, 0, len(f.xref))
	for n := range f.xref {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var objstmNums []int
	for _, n := range nums {
		obj, _, ok := f.parseIndirectAt(f.xref[n].off)
		if !ok {
			delete(f.xref, n)
			continue
		}
		s, isStream := obj.(*stream)
		if !isStream {
			continue
		}
		switch s.d["Type"] {
		case name("XRef"):
			trailers = append(trailers, s.d)
		case name("ObjStm"):
			objstmNums = append(objstmNums, n)
		}
	}
	f.trailer = dict{}
	for i := len(trailers) - 1; i >= 0; i-- {
		f.mergeTrailer(trailers[i])
	}
	for _, sn := range objstmNums {
		os := f.objStream(sn)
		if os == nil {
			continue
		}
		for idx := range os.offs {
			num := os.nums(idx)
			if _, exists := f.xref[num]; !exists && num > 0 {
				f.xref[num] = xrefEntry{kind: 2, off: sn, idx: idx}
			}
		}
	}
	if !f.rootOK() {
		for _, n := range nums {
			if d, ok := f.getObj(n).(dict); ok && d["Type"] == name("Catalog") {
				f.trailer["Root"] = ref{n, f.xref[n].gen}
				break
			}
		}
	}
	if len(f.xref) == 0 {
		return errNotPDF
	}
	return nil
}

// objHeaderBefore checks that "num gen" precede the "obj" keyword at j and
// returns the offset of num.
func objHeaderBefore(b []byte, j int) (start, num, gen int, ok bool) {
	k := j - 1
	if k < 0 || !isSpace(b[k]) {
		return 0, 0, 0, false
	}
	for k >= 0 && isSpace(b[k]) {
		k--
	}
	e := k + 1
	for k >= 0 && b[k] >= '0' && b[k] <= '9' {
		k--
	}
	if k+1 == e || e-k > 6 {
		return 0, 0, 0, false
	}
	gen = atoi(b[k+1 : e])
	if k < 0 || !isSpace(b[k]) {
		return 0, 0, 0, false
	}
	for k >= 0 && isSpace(b[k]) {
		k--
	}
	e = k + 1
	for k >= 0 && b[k] >= '0' && b[k] <= '9' {
		k--
	}
	if k+1 == e || e-k > 11 {
		return 0, 0, 0, false
	}
	if k >= 0 && isRegular(b[k]) {
		return 0, 0, 0, false
	}
	return k + 1, atoi(b[k+1 : e]), gen, true
}

func atoi(b []byte) int {
	n := 0
	for _, c := range b {
		n = n*10 + int(c-'0')
		if n > maxObjects*10 {
			return maxObjects + 1
		}
	}
	return n
}

// resolve follows references (bounded) and returns the direct object.
func (f *file) resolve(v any) any {
	for i := 0; i < 32; i++ {
		r, ok := v.(ref)
		if !ok {
			return v
		}
		v = f.getObj(r.num)
	}
	return nil
}

// getObj returns object num, loading and decrypting it on first use.
func (f *file) getObj(num int) any {
	if v, ok := f.cache[num]; ok {
		return v
	}
	if f.loading[num] {
		return nil
	}
	f.loading[num] = true
	defer delete(f.loading, num)
	v, ok := f.loadObj(num)
	if !ok && !f.repaired {
		if f.repair() == nil {
			v, _ = f.loadObj(num)
		}
	}
	f.cache[num] = v
	return v
}

func (f *file) loadObj(num int) (any, bool) {
	e, ok := f.xref[num]
	if !ok {
		return nil, true // missing objects are null by definition
	}
	switch e.kind {
	case 1:
		for _, off := range []int{e.off, e.off + f.hdrOff} {
			if off < 0 || off >= len(f.data) {
				continue
			}
			obj, r, ok := f.parseIndirectAt(off)
			if !ok || r.num != num {
				if f.hdrOff == 0 {
					break
				}
				continue
			}
			if f.sec != nil && !f.sec.isEncryptDict(r) {
				obj = f.sec.decryptObject(obj, r)
			}
			return obj, true
		}
		return nil, false
	case 2:
		os := f.objStream(e.off)
		if os == nil || e.idx < 0 || e.idx >= len(os.offs) {
			return nil, false
		}
		lx := newLexer(os.data)
		lx.pos = os.offs[e.idx]
		obj, _ := lx.object(0)
		if _, isKw := obj.(keyword); isKw {
			obj = nil
		}
		return obj, true
	}
	return nil, true
}

// objStream loads and indexes object stream num.
func (f *file) objStream(num int) *objStream {
	if os, ok := f.objstms[num]; ok {
		return os
	}
	f.objstms[num] = nil // guards against self-reference
	s, _ := f.getObj(num).(*stream)
	if s == nil {
		return nil
	}
	data, _, _, err := f.decodeStream(s, false)
	if err != nil {
		return nil
	}
	n, _ := integer(s.d["N"])
	first, _ := integer(s.d["First"])
	if n <= 0 || n > 1_000_000 || first < 0 || first > len(data) {
		return nil
	}
	os := &objStream{data: data}
	lx := newLexer(data[:first])
	var nums []int
	for i := 0; i < n; i++ {
		t1, ok1 := lx.next()
		t2, ok2 := lx.next()
		on, a := t1.(int)
		oo, b := t2.(int)
		if !ok1 || !ok2 || !a || !b || first+oo > len(data) || oo < 0 {
			break
		}
		nums = append(nums, on)
		os.offs = append(os.offs, first+oo)
	}
	os.objNums = nums
	f.objstms[num] = os
	return os
}

func (os *objStream) nums(i int) int {
	if i < len(os.objNums) {
		return os.objNums[i]
	}
	return -1
}
