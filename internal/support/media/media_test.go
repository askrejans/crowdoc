package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/bmp"

	"github.com/askrejans/crowdoc/v2/ast"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withPHYs inserts a pHYs chunk declaring dpi.
func withPHYs(b []byte, dpi float64) []byte {
	ppm := uint32(dpi / 0.0254)
	chunk := make([]byte, 21)
	binary.BigEndian.PutUint32(chunk[0:], 9)
	copy(chunk[4:], "pHYs")
	binary.BigEndian.PutUint32(chunk[8:], ppm)
	binary.BigEndian.PutUint32(chunk[12:], ppm)
	chunk[16] = 1
	// CRC is not checked by the reader under test.
	return append(append(append([]byte{}, b[:33]...), chunk...), b[33:]...)
}

// jpegWithOrientation builds a JPEG carrying an EXIF orientation tag.
func jpegWithOrientation(t *testing.T, w, h int, orient uint16) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, byte(orient >> 8), byte(orient), 0, 0, 0, 0, 0, 0}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	app1 = append(app1, payload...)
	return append(append(append([]byte{}, b[:2]...), app1...), b[2:]...)
}

func TestNaturalSizes(t *testing.T) {
	res := ast.NewResources()
	plain := res.Add("a.png", "image/png", pngBytes(t, 960, 480))
	retina := res.Add("b.png", "image/png", withPHYs(pngBytes(t, 960, 480), 192))
	photo := res.Add("c.jpg", "image/jpeg", jpegWithOrientation(t, 400, 200, 6))
	r := NewResolver(context.Background(), res, Options{})

	if a := r.Resolve(plain); a.NaturalWidthPt != 720 || a.Format != "png" {
		t.Fatalf("96 dpi default: %+v", a)
	}
	if a := r.Resolve(retina); math.Abs(a.NaturalWidthPt-360) > 0.1 {
		t.Fatalf("192 dpi: %+v", a)
	}
	if a := r.Resolve(photo); a.WidthPx != 200 || a.HeightPx != 400 {
		t.Fatalf("EXIF rotation not applied to size: %+v", a)
	}
	if a, b := r.Resolve(plain), r.Resolve(plain); a.Path != b.Path || !strings.HasPrefix(a.Path, "assets/") {
		t.Fatalf("unstable asset path %q %q", a.Path, b.Path)
	}
	if len(r.Files()) != 3 {
		t.Fatalf("files: %d", len(r.Files()))
	}
}

func TestConversionAndRejection(t *testing.T) {
	var bm bytes.Buffer
	if err := bmp.Encode(&bm, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	res := ast.NewResources()
	b := res.Add("x.bmp", "", bm.Bytes())
	emf := res.Add("x.emf", "", []byte{1, 0, 0, 0, 0x6c, 0, 0, 0})
	r := NewResolver(context.Background(), res, Options{})
	if a := r.Resolve(b); a.Format != "png" || a.Missing != "" {
		t.Fatalf("BMP not converted: %+v", a)
	}
	if a := r.Resolve(emf); a.Missing == "" || a.Path != "" {
		t.Fatalf("EMF should be reported missing: %+v", a)
	}
	if a := r.Resolve("https://example.com/x.png"); !strings.Contains(a.Missing, "remote") {
		t.Fatalf("remote images must be opt-in: %+v", a)
	}
	if a := r.Resolve("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="); a.Format != "png" {
		t.Fatalf("data URI: %+v", a)
	}
}

func TestConfinedPaths(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "in.png"), pngBytes(t, 2, 2), 0o600); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.png")
	if err := os.WriteFile(secret, pngBytes(t, 2, 2), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(base, "link.png")); err != nil {
		t.Fatal(err)
	}
	r := NewResolver(context.Background(), ast.NewResources(), Options{BaseDir: base, AllowLocalFiles: true, RestrictToBase: true})
	if a := r.Resolve("in.png"); a.Path == "" {
		t.Fatalf("file inside base refused: %+v", a)
	}
	for _, bad := range []string{secret, "../" + filepath.Base(outside) + "/secret.png", "link.png", "file://" + secret} {
		if a := r.Resolve(bad); a.Path != "" {
			t.Fatalf("%s escaped the base directory", bad)
		}
	}
	trusted := NewResolver(context.Background(), ast.NewResources(), Options{BaseDir: base, AllowLocalFiles: true})
	if a := trusted.Resolve(secret); a.Path == "" {
		t.Fatalf("trusted documents may use absolute paths: %+v", a)
	}
}

func dibFrom(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.RGBA{0, 0, 255, 255})
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()[14:] // drop BITMAPFILEHEADER
}

func TestMetafileBitmaps(t *testing.T) {
	dib := dibFrom(t)
	le := binary.LittleEndian
	// WMF: standard header + META_STRETCHDIB + META_EOF.
	params := make([]byte, 4+2+2*8)
	rec := append(params, dib...)
	if len(rec)%2 == 1 {
		rec = append(rec, 0)
	}
	wmf := []byte{1, 0, 9, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	r := make([]byte, 6)
	le.PutUint32(r, uint32((6+len(rec))/2))
	le.PutUint16(r[4:], 0x0F43)
	wmf = append(append(wmf, r...), rec...)
	wmf = append(wmf, 3, 0, 0, 0, 0, 0)
	if data, f, ok := metafileBitmap(wmf); !ok || f != "png" || sniff(data) != "png" {
		t.Fatalf("WMF bitmap not extracted: ok=%v f=%s", ok, f)
	}
	// EMF: header record + EMR_STRETCHDIBITS.
	hdr := make([]byte, 88)
	le.PutUint32(hdr, 1)
	le.PutUint32(hdr[4:], 88)
	copy(hdr[40:], " EMF")
	const fixed = 80
	bmiSize := int(le.Uint32(dib))
	sd := make([]byte, fixed)
	le.PutUint32(sd, 0x51)
	le.PutUint32(sd[4:], uint32(fixed+len(dib)))
	at := 8 + 16 + 4*6
	le.PutUint32(sd[at:], fixed)
	le.PutUint32(sd[at+4:], uint32(bmiSize))
	le.PutUint32(sd[at+8:], uint32(fixed+bmiSize))
	le.PutUint32(sd[at+12:], uint32(len(dib)-bmiSize))
	emf := append(append(hdr, sd...), dib...)
	if data, f, ok := metafileBitmap(emf); !ok || sniff(data) != f {
		t.Fatalf("EMF bitmap not extracted: ok=%v", ok)
	}
	// A vector-only metafile is not a bitmap.
	if _, _, ok := metafileBitmap([]byte{1, 0, 9, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0, 0}); ok {
		t.Fatal("empty metafile reported a bitmap")
	}
}
