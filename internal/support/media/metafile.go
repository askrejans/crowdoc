package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"

	_ "golang.org/x/image/bmp" // DIB decoding
)

// Windows metafiles (WMF/EMF) are how RTF and older Word documents store
// most pictures; usually they just wrap one bitmap. Vector drawings cannot
// be rendered without a metafile renderer and are reported as unsupported.

func isWMF(b []byte) bool {
	if len(b) >= 4 && binary.LittleEndian.Uint32(b) == 0x9AC6CDD7 {
		return true // placeable header
	}
	return len(b) >= 18 && (b[0] == 1 || b[0] == 2) && b[1] == 0 && b[2] == 9 && b[3] == 0
}

func isEMF(b []byte) bool {
	return len(b) >= 44 && binary.LittleEndian.Uint32(b) == 1 && string(b[40:44]) == " EMF"
}

// metafileBitmap extracts the largest embedded bitmap of a WMF or EMF file
// and returns it as PNG (or the embedded JPEG/PNG stream as is).
func metafileBitmap(b []byte) ([]byte, string, bool) {
	var dibs [][]byte
	switch {
	case isEMF(b):
		dibs = emfDIBs(b)
	case isWMF(b):
		dibs = wmfDIBs(b)
	default:
		return nil, "", false
	}
	var best []byte
	for _, d := range dibs {
		if len(d) > len(best) {
			best = d
		}
	}
	if best == nil {
		return nil, "", false
	}
	return dibToImage(best)
}

func wmfDIBs(b []byte) [][]byte {
	off := 0
	if binary.LittleEndian.Uint32(b) == 0x9AC6CDD7 {
		off = 22
	}
	if off+18 > len(b) {
		return nil
	}
	off += int(binary.LittleEndian.Uint16(b[off+2:])) * 2 // header size in words
	var out [][]byte
	for off+6 <= len(b) {
		size := int(binary.LittleEndian.Uint32(b[off:])) * 2
		fn := binary.LittleEndian.Uint16(b[off+4:])
		if size < 6 || off+size > len(b) {
			break
		}
		rec := b[off+6 : off+size]
		var skip int
		switch fn {
		case 0x0F43: // META_STRETCHDIB
			skip = 4 + 2 + 2*8
		case 0x0B41: // META_DIBSTRETCHBLT
			skip = 4 + 2*8
		case 0x0940: // META_DIBBITBLT
			skip = 4 + 2*6
		case 0x0D33: // META_SETDIBTODEV
			skip = 2 * 9
		case 0x0000: // META_EOF
			return out
		default:
			skip = -1
		}
		if skip >= 0 && skip+40 <= len(rec) {
			out = append(out, rec[skip:])
		}
		off += size
	}
	return out
}

func emfDIBs(b []byte) [][]byte {
	var out [][]byte
	off := 0
	for off+8 <= len(b) {
		typ := binary.LittleEndian.Uint32(b[off:])
		size := int(binary.LittleEndian.Uint32(b[off+4:]))
		if size < 8 || off+size > len(b) {
			break
		}
		rec := b[off : off+size]
		var bmiAt int
		switch typ {
		case 0x51: // EMR_STRETCHDIBITS
			bmiAt = 8 + 16 + 4*6
		case 0x50: // EMR_SETDIBITSTODEVICE
			bmiAt = 8 + 16 + 4*6
		case 0x4C, 0x4D: // EMR_BITBLT, EMR_STRETCHBLT
			bmiAt = 8 + 16 + 4*7 + 24 + 4*2
		case 0x0E: // EMR_EOF
			return out
		}
		if bmiAt > 0 && bmiAt+16 <= len(rec) {
			offBmi := int(binary.LittleEndian.Uint32(rec[bmiAt:]))
			cbBmi := int(binary.LittleEndian.Uint32(rec[bmiAt+4:]))
			offBits := int(binary.LittleEndian.Uint32(rec[bmiAt+8:]))
			cbBits := int(binary.LittleEndian.Uint32(rec[bmiAt+12:]))
			if offBmi > 0 && cbBmi >= 40 && offBmi+cbBmi <= len(rec) && offBits >= offBmi+cbBmi && offBits+cbBits <= len(rec) {
				dib := append(append([]byte{}, rec[offBmi:offBmi+cbBmi]...), rec[offBits:offBits+cbBits]...)
				out = append(out, dib)
			}
		}
		off += size
	}
	return out
}

// dibToImage converts a packed DIB (BITMAPINFOHEADER + colours + bits).
func dibToImage(dib []byte) ([]byte, string, bool) {
	if len(dib) < 40 {
		return nil, "", false
	}
	hdr := int(binary.LittleEndian.Uint32(dib))
	if hdr < 40 || hdr > len(dib) {
		return nil, "", false
	}
	bitCount := int(binary.LittleEndian.Uint16(dib[14:]))
	compression := binary.LittleEndian.Uint32(dib[16:])
	clrUsed := int(binary.LittleEndian.Uint32(dib[32:]))
	switch compression {
	case 4, 5: // BI_JPEG, BI_PNG: the bits are a complete image file
		data := dib[hdr:]
		if f := sniff(data); f == "jpg" || f == "png" {
			return data, f, true
		}
		return nil, "", false
	}
	colours := clrUsed
	if colours == 0 && bitCount <= 8 {
		colours = 1 << bitCount
	}
	pixelOffset := 14 + hdr + colours*4
	if compression == 3 && hdr == 40 { // BI_BITFIELDS masks follow the header
		pixelOffset += 12
	}
	file := make([]byte, 14, 14+len(dib))
	file[0], file[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(file[2:], uint32(14+len(dib)))
	binary.LittleEndian.PutUint32(file[10:], uint32(pixelOffset))
	file = append(file, dib...)
	img, _, err := image.Decode(bytes.NewReader(file))
	if err != nil {
		return nil, "", false
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		return nil, "", false
	}
	return buf.Bytes(), "png", true
}
