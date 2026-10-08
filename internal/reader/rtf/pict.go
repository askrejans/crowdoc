package rtf

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strconv"

	"github.com/askrejans/crowdoc/v2/ast"
)

// pictRec is a \pict being parsed.
type pictRec struct {
	kind           string
	data           []byte
	nibble         byte
	half           bool
	picw, pich     int
	goalW, goalH   int
	scaleX, scaleY int
	alt            string
}

func (pc *pictRec) addHex(s string) {
	for i := 0; i < len(s); i++ {
		v, ok := hexVal(s[i])
		if !ok {
			continue
		}
		if pc.half {
			pc.data = append(pc.data, pc.nibble<<4|v)
			pc.half = false
		} else {
			pc.nibble, pc.half = v, true
		}
	}
}

func (p *parser) pictWord(t token, st *state) bool {
	pc := st.pict
	if pc == nil {
		return false
	}
	switch t.name {
	case "pngblip":
		pc.kind = "png"
	case "jpegblip":
		pc.kind = "jpeg"
	case "emfblip":
		pc.kind = "emf"
	case "wmetafile":
		pc.kind = "wmf"
	case "dibitmap":
		pc.kind = "dib"
	case "wbitmap":
		pc.kind = "ddb"
	case "macpict":
		pc.kind = "pict"
	case "pmmetafile":
		pc.kind = "os2"
	case "picw":
		pc.picw = t.param
	case "pich":
		pc.pich = t.param
	case "picwgoal":
		pc.goalW = t.param
	case "pichgoal":
		pc.goalH = t.param
	case "picscalex":
		pc.scaleX = t.param
	case "picscaley":
		pc.scaleY = t.param
	default:
		return false
	}
	return true
}

// endPict stores a finished picture as a resource and inserts it.
func (p *parser) endPict(pc *pictRec, sh *shapeRec) {
	if pc == nil || len(pc.data) == 0 {
		return
	}
	data := pc.data
	mt, ext := sniffImage(data)
	if mt == "" {
		switch pc.kind {
		case "png":
			mt, ext = "image/png", ".png"
		case "jpeg":
			mt, ext = "image/jpeg", ".jpg"
		case "emf":
			mt, ext = "image/emf", ".emf"
		case "wmf":
			data = placeableWMF(data, pc)
			mt, ext = "image/wmf", ".wmf"
		case "dib":
			if bmp := dibToBMP(data); bmp != nil {
				data, mt, ext = bmp, "image/bmp", ".bmp"
			}
		}
	}
	if mt == "" {
		p.warn.Addf("unsupported picture format (%s) was dropped", kindName(pc.kind))
		return
	}
	p.images++
	sum := sha256.Sum256(data)
	src, ok := p.imageSrcs[sum]
	if !ok {
		if p.imageSrcs == nil {
			p.imageSrcs = map[[32]byte]string{}
		}
		src = p.doc.Resources.Add("rtf/image"+strconv.Itoa(len(p.imageSrcs)+1)+ext, mt, data)
		p.imageSrcs[sum] = src
	}
	img := &ast.Image{Src: src, Alt: pc.alt}
	if w, h, unit := pc.size(mt); w > 0 && h > 0 {
		img.Width, img.Height = formatLen(w, unit), formatLen(h, unit)
	}
	p.paraInline(img)
	if sh != nil {
		sh.done = true
		sh.imgs = append(sh.imgs, img)
	}
}

func kindName(k string) string {
	if k == "" {
		return "unknown"
	}
	return k
}

// size returns the display size: the goal size in twips when present,
// otherwise the native size (pixels for bitmaps, 0.01 mm for metafiles).
func (pc *pictRec) size(mt string) (w, h float64, unit string) {
	sx, sy := float64(pc.scaleX)/100, float64(pc.scaleY)/100
	if sx <= 0 {
		sx = 1
	}
	if sy <= 0 {
		sy = 1
	}
	if pc.goalW > 0 && pc.goalH > 0 {
		return float64(pc.goalW) / 20 * sx, float64(pc.goalH) / 20 * sy, "pt"
	}
	if pc.picw <= 0 || pc.pich <= 0 {
		return 0, 0, ""
	}
	if mt == "image/emf" || mt == "image/wmf" {
		return float64(pc.picw) * 72 / 2540 * sx, float64(pc.pich) * 72 / 2540 * sy, "pt"
	}
	return float64(pc.picw) * sx, float64(pc.pich) * sy, "px"
}

func formatLen(v float64, unit string) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64) + unit
}

// sniffImage identifies common image formats by their magic bytes.
func sniffImage(b []byte) (mt, ext string) {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", ".png"
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", ".jpg"
	case bytes.HasPrefix(b, []byte("GIF8")):
		return "image/gif", ".gif"
	case bytes.HasPrefix(b, []byte("BM")) && len(b) > 14:
		return "image/bmp", ".bmp"
	case bytes.HasPrefix(b, []byte("II*\x00")), bytes.HasPrefix(b, []byte("MM\x00*")):
		return "image/tiff", ".tif"
	case len(b) > 44 && binary.LittleEndian.Uint32(b) == 1 && string(b[40:44]) == " EMF":
		return "image/emf", ".emf"
	case bytes.HasPrefix(b, []byte{0xD7, 0xCD, 0xC6, 0x9A}):
		return "image/wmf", ".wmf"
	}
	return "", ""
}

// placeableWMF prefixes a raw Windows metafile (as stored by \wmetafile)
// with the "placeable" header standalone .wmf files carry.
func placeableWMF(data []byte, pc *pictRec) []byte {
	w, h := pc.goalW, pc.goalH // twips
	if w <= 0 || h <= 0 {
		w, h = pc.picw*1440/2540, pc.pich*1440/2540
	}
	if w <= 0 || h <= 0 || w > 0x7FFF || h > 0x7FFF {
		w, h = 1440, 1440
	}
	hdr := make([]byte, 22)
	binary.LittleEndian.PutUint32(hdr[0:], 0x9AC6CDD7)
	binary.LittleEndian.PutUint16(hdr[10:], uint16(w))
	binary.LittleEndian.PutUint16(hdr[12:], uint16(h))
	binary.LittleEndian.PutUint16(hdr[14:], 1440)
	var sum uint16
	for i := 0; i < 20; i += 2 {
		sum ^= binary.LittleEndian.Uint16(hdr[i:])
	}
	binary.LittleEndian.PutUint16(hdr[20:], sum)
	return append(hdr, data...)
}

// dibToBMP turns a device-independent bitmap (BITMAPINFO + bits) into a
// .bmp file by adding the 14-byte file header.
func dibToBMP(dib []byte) []byte {
	if len(dib) < 12 {
		return nil
	}
	hdrSize := int(binary.LittleEndian.Uint32(dib))
	if hdrSize < 12 || hdrSize > len(dib) {
		return nil
	}
	var bitCount, colors, entry, masks int
	if hdrSize == 12 {
		bitCount = int(binary.LittleEndian.Uint16(dib[10:]))
		entry = 3
	} else {
		if hdrSize < 40 {
			return nil
		}
		bitCount = int(binary.LittleEndian.Uint16(dib[14:]))
		colors = int(binary.LittleEndian.Uint32(dib[32:]))
		entry = 4
		if compression := binary.LittleEndian.Uint32(dib[16:]); hdrSize == 40 && (compression == 3 || compression == 6) {
			masks = 12
			if compression == 6 {
				masks = 16
			}
		}
	}
	if colors == 0 && bitCount > 0 && bitCount <= 8 {
		colors = 1 << bitCount
	}
	offset := 14 + hdrSize + colors*entry + masks
	if offset-14 > len(dib) || colors > 1<<16 {
		return nil
	}
	out := make([]byte, 14, 14+len(dib))
	out[0], out[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(out[2:], uint32(14+len(dib)))
	binary.LittleEndian.PutUint32(out[10:], uint32(offset))
	return append(out, dib...)
}
