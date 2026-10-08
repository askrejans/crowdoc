package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

// maxImagePixels bounds the size of a decoded raster image.
const maxImagePixels = 40_000_000

// colorSpace describes how image samples map to RGB.
type colorSpace struct {
	kind    string // gray, rgb, cmyk, indexed, separation
	n       int    // components per sample
	palette []color.NRGBA
}

// decodeImage converts an image XObject to PNG or JPEG bytes.
func (f *file) decodeImage(s *stream) ([]byte, string, error) {
	w, _ := integer(f.resolve(s.d["Width"]))
	h, _ := integer(f.resolve(s.d["Height"]))
	if w <= 0 || h <= 0 || w > 1<<16 || h > 1<<16 || w*h > maxImagePixels {
		return nil, "", errors.New("image has invalid or excessive dimensions")
	}
	data, codec, _, err := f.decodeStream(s, true)
	if err != nil {
		return nil, "", err
	}
	cs := f.colorSpace(s.d["ColorSpace"])
	switch codec {
	case "DCTDecode":
		return f.jpegImage(data, cs, s)
	case "":
	default:
		return nil, "", fmt.Errorf("unsupported image encoding %s", codec)
	}
	bpc, ok := integer(f.resolve(s.d["BitsPerComponent"]))
	if !ok {
		bpc = 8
	}
	switch bpc {
	case 1, 2, 4, 8, 16:
	default:
		return nil, "", fmt.Errorf("unsupported image bit depth %d", bpc)
	}
	if cs.kind == "" {
		return nil, "", errors.New("unsupported image color space")
	}
	decode := f.decodeArray(s.d["Decode"], cs, bpc)
	img := samplesToImage(data, w, h, bpc, cs, decode)
	if img == nil {
		return nil, "", errors.New("image data is truncated")
	}
	if sm, ok := f.resolve(s.d["SMask"]).(*stream); ok {
		f.applySMask(img, sm)
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, opaqueIfPossible(img)); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}

func (f *file) colorSpace(v any) colorSpace {
	switch c := f.resolve(v).(type) {
	case name:
		switch c {
		case "DeviceGray", "CalGray", "G":
			return colorSpace{kind: "gray", n: 1}
		case "DeviceRGB", "CalRGB", "RGB":
			return colorSpace{kind: "rgb", n: 3}
		case "DeviceCMYK", "CMYK":
			return colorSpace{kind: "cmyk", n: 4}
		}
	case array:
		if len(c) == 0 {
			break
		}
		fam, _ := f.resolve(c[0]).(name)
		switch fam {
		case "CalGray":
			return colorSpace{kind: "gray", n: 1}
		case "CalRGB", "Lab":
			return colorSpace{kind: "rgb", n: 3}
		case "ICCBased":
			if len(c) > 1 {
				if st, ok := f.resolve(c[1]).(*stream); ok {
					n, _ := integer(f.resolve(st.d["N"]))
					switch n {
					case 1:
						return colorSpace{kind: "gray", n: 1}
					case 3:
						return colorSpace{kind: "rgb", n: 3}
					case 4:
						return colorSpace{kind: "cmyk", n: 4}
					}
					if alt := st.d["Alternate"]; alt != nil {
						return f.colorSpace(alt)
					}
				}
			}
		case "Indexed", "I":
			if len(c) >= 4 {
				return f.indexedSpace(c)
			}
		case "Separation":
			return colorSpace{kind: "separation", n: 1}
		case "DeviceN":
			if len(c) > 1 {
				if names, ok := f.resolve(c[1]).(array); ok && len(names) > 0 {
					return colorSpace{kind: "separation", n: len(names)}
				}
			}
		}
		return f.colorSpace(c[0])
	}
	return colorSpace{}
}

func (f *file) indexedSpace(c array) colorSpace {
	base := f.colorSpace(c[1])
	hival, _ := integer(f.resolve(c[2]))
	var lookup []byte
	switch l := f.resolve(c[3]).(type) {
	case pdfString:
		lookup = []byte(l)
	case *stream:
		lookup, _, _, _ = f.decodeStream(l, false)
	}
	if base.kind == "" || base.kind == "indexed" || hival < 0 || hival > 255 {
		return colorSpace{}
	}
	pal := make([]color.NRGBA, hival+1)
	for i := range pal {
		off := i * base.n
		if off+base.n > len(lookup) {
			break
		}
		pal[i] = toRGB(base, lookup[off:off+base.n])
	}
	return colorSpace{kind: "indexed", n: 1, palette: pal}
}

func toRGB(cs colorSpace, v []byte) color.NRGBA {
	switch cs.kind {
	case "gray":
		return color.NRGBA{v[0], v[0], v[0], 255}
	case "rgb":
		return color.NRGBA{v[0], v[1], v[2], 255}
	case "cmyk":
		k := 255 - int(v[3])
		return color.NRGBA{
			uint8((255 - int(v[0])) * k / 255),
			uint8((255 - int(v[1])) * k / 255),
			uint8((255 - int(v[2])) * k / 255), 255}
	case "separation":
		g := 255 - v[0]
		return color.NRGBA{g, g, g, 255}
	}
	return color.NRGBA{0, 0, 0, 255}
}

// decodeArray returns per-component [min, max] pairs scaled to 0–255
// output, or nil for the default mapping.
func (f *file) decodeArray(v any, cs colorSpace, bpc int) []float64 {
	a, ok := f.resolve(v).(array)
	if !ok || len(a) < 2*cs.n || cs.kind == "indexed" {
		return nil
	}
	out := make([]float64, 2*cs.n)
	def := true
	for i := range out {
		x, _ := num(f.resolve(a[i]))
		out[i] = x
		if (i%2 == 0 && x != 0) || (i%2 == 1 && x != 1) {
			def = false
		}
	}
	if def {
		return nil
	}
	return out
}

func samplesToImage(data []byte, w, h, bpc int, cs colorSpace, decode []float64) *image.NRGBA {
	n := cs.n
	rowBits := w * n * bpc
	rowBytes := (rowBits + 7) / 8
	if len(data) < rowBytes*h {
		if len(data) < rowBytes*(h/2) {
			return nil
		}
		// Tolerate a short final strip: pad with zero samples.
		data = append(data, make([]byte, rowBytes*h-len(data))...)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	maxV := float64(int(1)<<bpc - 1)
	comp := make([]byte, n)
	for y := 0; y < h; y++ {
		row := data[y*rowBytes : (y+1)*rowBytes]
		for x := 0; x < w; x++ {
			for c := 0; c < n; c++ {
				idx := x*n + c
				var frac float64
				switch bpc {
				case 8:
					frac = float64(row[idx]) / 255
				case 16:
					frac = float64(row[2*idx]) / 255 // high byte is enough
				default:
					bit := idx * bpc
					frac = float64(int(row[bit/8]>>(8-bpc-bit%8))&(1<<bpc-1)) / maxV
				}
				if cs.kind == "indexed" {
					comp[c] = byte(frac*maxV + 0.5)
					continue
				}
				if decode != nil {
					frac = decode[2*c] + frac*(decode[2*c+1]-decode[2*c])
				}
				comp[c] = byte(clamp01(frac)*255 + 0.5)
			}
			var px color.NRGBA
			if cs.kind == "indexed" {
				if int(comp[0]) < len(cs.palette) {
					px = cs.palette[comp[0]]
				} else {
					px = color.NRGBA{0, 0, 0, 255}
				}
			} else {
				px = toRGB(cs, comp)
			}
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = px.R, px.G, px.B, 255
		}
	}
	return img
}

func clamp01(x float64) float64 {
	return max(0, min(1, x))
}

func (f *file) applySMask(img *image.NRGBA, sm *stream) {
	w, _ := integer(f.resolve(sm.d["Width"]))
	h, _ := integer(f.resolve(sm.d["Height"]))
	b := img.Bounds()
	if w != b.Dx() || h != b.Dy() {
		return
	}
	data, codec, _, err := f.decodeStream(sm, true)
	if err != nil || codec != "" {
		return
	}
	bpc, ok := integer(f.resolve(sm.d["BitsPerComponent"]))
	if !ok || (bpc != 1 && bpc != 2 && bpc != 4 && bpc != 8 && bpc != 16) {
		return
	}
	mask := samplesToImage(data, w, h, bpc, colorSpace{kind: "gray", n: 1}, nil)
	if mask == nil {
		return
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = mask.Pix[i-3]
	}
}

// opaqueIfPossible drops the alpha channel when every pixel is opaque, which
// keeps PNG output small.
func opaqueIfPossible(img *image.NRGBA) image.Image {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 255 {
			return img
		}
	}
	rgba := &image.RGBA{Pix: img.Pix, Stride: img.Stride, Rect: img.Rect}
	return rgba
}

// jpegImage passes JPEG data through unless it is CMYK (or inverted),
// which many consumers render wrongly; those are converted to PNG.
func (f *file) jpegImage(data []byte, cs colorSpace, s *stream) ([]byte, string, error) {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("corrupt JPEG image: %w", err)
	}
	if cfg.ColorModel != color.CMYKModel && cs.kind != "cmyk" && f.decodeArray(s.d["Decode"], cs, 8) == nil {
		return data, "image/jpeg", nil
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("corrupt JPEG image: %w", err)
	}
	b := img.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			px := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			o := out.PixOffset(x, y)
			out.Pix[o], out.Pix[o+1], out.Pix[o+2], out.Pix[o+3] = px.R, px.G, px.B, 255
		}
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, opaqueIfPossible(out)); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}
