package pdf

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"errors"
	"fmt"
	"io"

	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// imageFilters are passed through to the image decoder instead of being
// decoded here.
var imageFilters = map[name]bool{
	"DCTDecode": true, "DCT": true,
	"JPXDecode":      true,
	"CCITTFaxDecode": true, "CCF": true,
	"JBIG2Decode": true,
}

var filterAbbrev = map[name]name{
	"AHx": "ASCIIHexDecode", "A85": "ASCII85Decode", "LZW": "LZWDecode",
	"Fl": "FlateDecode", "RL": "RunLengthDecode", "CCF": "CCITTFaxDecode",
	"DCT": "DCTDecode",
}

func filterList(v any) []name {
	switch x := v.(type) {
	case name:
		return []name{x}
	case array:
		out := make([]name, 0, len(x))
		for _, e := range x {
			if n, ok := e.(name); ok {
				out = append(out, n)
			}
		}
		return out
	}
	return nil
}

func parmsAt(v any, i int) dict {
	switch x := v.(type) {
	case dict:
		return x
	case array:
		if i < len(x) {
			d, _ := x[i].(dict)
			return d
		}
	}
	return nil
}

// decodeStream decrypts and decodes s. When stopAtImage is set, decoding
// stops at the first image codec filter, which is returned with its
// parameters so the image decoder can handle it.
func (f *file) decodeStream(s *stream, stopAtImage bool) ([]byte, name, dict, error) {
	data := s.raw
	if f.sec != nil && !s.plain {
		data = f.sec.decrypt(data, s.ref, f.sec.streamMethod(s))
	}
	filters := filterList(s.d["Filter"])
	parmsV := s.d["DecodeParms"]
	if parmsV == nil {
		parmsV = s.d["DP"]
	}
	for i, fl := range filters {
		if full, ok := filterAbbrev[fl]; ok {
			fl = full
		}
		parms, _ := f.resolve(parmsAt(f.resolve(parmsV), i)).(dict)
		if imageFilters[fl] {
			if stopAtImage {
				return data, fl, parms, nil
			}
			return nil, fl, parms, fmt.Errorf("image filter %s in a non-image stream", fl)
		}
		var err error
		data, err = f.applyFilter(fl, data, parms)
		if err != nil {
			return nil, "", nil, err
		}
	}
	return data, "", nil, nil
}

func (f *file) limit() int64 { return min(f.budget, f.maxEntry) }

func (f *file) applyFilter(fl name, data []byte, parms dict) ([]byte, error) {
	var out []byte
	var err error
	lim := f.limit()
	switch fl {
	case "FlateDecode":
		out, err = inflate(data, lim)
	case "LZWDecode":
		early := 1
		if v, ok := integer(parms["EarlyChange"]); ok {
			early = v
		}
		out, err = lzwDecode(data, early != 0, lim)
	case "ASCII85Decode":
		out = ascii85Decode(data)
	case "ASCIIHexDecode":
		lx := lexer{b: data}
		out = []byte(lx.hexString())
	case "RunLengthDecode":
		out, err = runLengthDecode(data, lim)
	case "Crypt":
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported stream filter %s", fl)
	}
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > lim {
		return nil, fmt.Errorf("%w: decompressed stream is too large", rd.ErrLimit)
	}
	if fl == "FlateDecode" || fl == "LZWDecode" {
		out, err = unpredict(out, parms)
		if err != nil {
			return nil, err
		}
	}
	f.budget -= int64(len(out))
	return out, nil
}

// inflate decodes zlib data, tolerating a missing header, bad checksums and
// truncation (whatever was decoded is returned).
func inflate(data []byte, lim int64) ([]byte, error) {
	var r io.Reader
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err == nil {
		r = zr
	} else {
		r = flate.NewReader(bytes.NewReader(data))
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, io.LimitReader(r, lim+1))
	if err != nil && buf.Len() == 0 && len(data) > 2 {
		// Some writers emit raw deflate data after a bogus header.
		buf.Reset()
		_, err = io.Copy(&buf, io.LimitReader(flate.NewReader(bytes.NewReader(data[2:])), lim+1))
		if err != nil && buf.Len() == 0 {
			return nil, errors.New("corrupt compressed stream")
		}
	}
	if int64(buf.Len()) > lim {
		return nil, fmt.Errorf("%w: decompressed stream is too large", rd.ErrLimit)
	}
	return buf.Bytes(), nil
}

// unpredict reverses TIFF and PNG predictors.
func unpredict(data []byte, parms dict) ([]byte, error) {
	pred, _ := integer(parms["Predictor"])
	if pred <= 1 {
		return data, nil
	}
	colors, bpc, columns := 1, 8, 1
	if v, ok := integer(parms["Colors"]); ok && v > 0 && v <= 32 {
		colors = v
	}
	if v, ok := integer(parms["BitsPerComponent"]); ok && v > 0 && v <= 16 {
		bpc = v
	}
	if v, ok := integer(parms["Columns"]); ok && v > 0 && v <= 1<<20 {
		columns = v
	}
	bpp := max(1, (colors*bpc+7)/8)
	rowLen := (colors*bpc*columns + 7) / 8
	if rowLen <= 0 {
		return data, nil
	}
	if pred == 2 {
		if bpc != 8 {
			return data, nil
		}
		for r := 0; r+rowLen <= len(data); r += rowLen {
			row := data[r : r+rowLen]
			for i := bpp; i < len(row); i++ {
				row[i] += row[i-bpp]
			}
		}
		return data, nil
	}
	if pred < 10 {
		return data, nil
	}
	out := make([]byte, 0, len(data))
	prev := make([]byte, rowLen)
	for r := 0; r < len(data); r += rowLen + 1 {
		ft := data[r]
		end := min(len(data), r+1+rowLen)
		row := make([]byte, rowLen)
		copy(row, data[r+1:end])
		switch ft {
		case 1:
			for i := bpp; i < rowLen; i++ {
				row[i] += row[i-bpp]
			}
		case 2:
			for i := 0; i < rowLen; i++ {
				row[i] += prev[i]
			}
		case 3:
			for i := 0; i < rowLen; i++ {
				var left byte
				if i >= bpp {
					left = row[i-bpp]
				}
				row[i] += byte((int(left) + int(prev[i])) / 2)
			}
		case 4:
			for i := 0; i < rowLen; i++ {
				var a, c byte
				if i >= bpp {
					a, c = row[i-bpp], prev[i-bpp]
				}
				row[i] += paeth(a, prev[i], c)
			}
		}
		out = append(out, row[:end-r-1]...)
		prev = row
	}
	return out, nil
}

func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa, pb, pc := abs(p-int(a)), abs(p-int(b)), abs(p-int(c))
	switch {
	case pa <= pb && pa <= pc:
		return a
	case pb <= pc:
		return b
	}
	return c
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// lzwDecode implements the PDF flavour of LZW (MSB-first codes, optional
// early code-width change).
func lzwDecode(data []byte, early bool, lim int64) ([]byte, error) {
	var out []byte
	type entry struct {
		prefix int
		c      byte
		n      int
	}
	table := make([]entry, 4096)
	for i := 0; i < 256; i++ {
		table[i] = entry{-1, byte(i), 1}
	}
	next := 258
	width := 9
	prev := -1
	var bitBuf uint32
	bits := 0
	pos := 0
	tmp := make([]byte, 0, 4096)
	emit := func(code int) byte {
		tmp = tmp[:0]
		for c := code; c >= 0 && len(tmp) < 4096; c = table[c].prefix {
			tmp = append(tmp, table[c].c)
		}
		for i := len(tmp) - 1; i >= 0; i-- {
			out = append(out, tmp[i])
		}
		return tmp[len(tmp)-1]
	}
	for {
		for bits < width && pos < len(data) {
			bitBuf = bitBuf<<8 | uint32(data[pos])
			pos++
			bits += 8
		}
		if bits < width {
			break
		}
		code := int(bitBuf>>(bits-width)) & (1<<width - 1)
		bits -= width
		switch {
		case code == 256:
			next, width, prev = 258, 9, -1
			continue
		case code == 257:
			return out, nil
		}
		var first byte
		if code < next && (code < 256 || table[code].n > 0) {
			first = emit(code)
		} else if code == next && prev >= 0 {
			pf := emit(prev)
			out = append(out, pf)
			first = pf
		} else {
			return out, nil // corrupt code; keep what we have
		}
		if prev >= 0 && next < 4096 {
			table[next] = entry{prev, first, table[prev].n + 1}
			next++
		}
		prev = code
		e := 0
		if early {
			e = 1
		}
		if next+e >= 1<<width && width < 12 {
			width++
		}
		if int64(len(out)) > lim {
			return nil, fmt.Errorf("%w: decompressed stream is too large", rd.ErrLimit)
		}
	}
	return out, nil
}

// ascii85Decode decodes base-85 data, tolerating whitespace, the "<~"
// prefix, a missing "~>" terminator and a truncated final group.
func ascii85Decode(data []byte) []byte {
	data = bytes.TrimPrefix(bytes.TrimSpace(data), []byte("<~"))
	out := make([]byte, 0, len(data)*4/5)
	var group [5]byte
	n := 0
	for _, c := range data {
		if c == '~' {
			break
		}
		if isSpace(c) {
			continue
		}
		if c == 'z' && n == 0 {
			out = append(out, 0, 0, 0, 0)
			continue
		}
		if c < '!' || c > 'u' {
			continue
		}
		group[n] = c - '!'
		n++
		if n == 5 {
			v := uint32(0)
			for _, g := range group {
				v = v*85 + uint32(g)
			}
			out = append(out, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
			n = 0
		}
	}
	if n > 1 {
		for i := n; i < 5; i++ {
			group[i] = 84
		}
		v := uint32(0)
		for _, g := range group {
			v = v*85 + uint32(g)
		}
		b := []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
		out = append(out, b[:n-1]...)
	}
	return out
}

func runLengthDecode(data []byte, lim int64) ([]byte, error) {
	out := make([]byte, 0, len(data)*2)
	for i := 0; i < len(data); {
		l := int(data[i])
		i++
		switch {
		case l == 128:
			return out, nil
		case l < 128:
			end := min(len(data), i+l+1)
			out = append(out, data[i:end]...)
			i = end
		default:
			if i >= len(data) {
				return out, nil
			}
			for k := 0; k < 257-l; k++ {
				out = append(out, data[i])
			}
			i++
		}
		if int64(len(out)) > lim {
			return nil, fmt.Errorf("%w: decompressed stream is too large", rd.ErrLimit)
		}
	}
	return out, nil
}
