// Package media resolves document images into bundle assets: local files,
// embedded resources, data: URIs and (optionally) remote URLs. It reads
// intrinsic sizes and converts formats the target engine cannot read.
package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // register decoders for DecodeConfig
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/askrejans/crowdoc/v2/ast"
)

// Asset is an image prepared for a bundle.
type Asset struct {
	Path            string // bundle-relative path; empty if unavailable
	Format          string // png, jpg, gif, webp, svg, pdf
	WidthPx         int
	HeightPx        int
	NaturalWidthPt  float64
	NaturalHeightPt float64
	Missing         string // reason the image is unavailable
}

// Options configures a Resolver.
type Options struct {
	// BaseDir resolves relative paths. Empty disables local file access.
	BaseDir string
	// AllowLocalFiles permits reading files outside of embedded resources.
	AllowLocalFiles bool
	// RestrictToBase confines local files to BaseDir: absolute paths,
	// ".." and symbolic links leading outside it are refused. Use it for
	// untrusted documents whose assets were uploaded into BaseDir.
	RestrictToBase bool
	// AllowRemote permits fetching http(s) images.
	AllowRemote bool
	// HTTPClient is used for remote images (default: 20 s timeout).
	HTTPClient *http.Client
	// MaxBytes limits a single image (default 64 MiB).
	MaxBytes int64
	// Formats lists the formats the engine reads natively
	// (default: png, jpg, gif, webp, svg, pdf).
	Formats map[string]bool
}

// Resolver turns image sources into assets. It is safe for concurrent use.
type Resolver struct {
	o     Options
	res   *ast.Resources
	ctx   context.Context
	mu    sync.Mutex
	cache map[string]Asset
	files map[string][]byte
}

// NewResolver creates a resolver for one document.
func NewResolver(ctx context.Context, res *ast.Resources, o Options) *Resolver {
	if o.MaxBytes <= 0 {
		o.MaxBytes = 64 << 20
	}
	if o.Formats == nil {
		o.Formats = map[string]bool{"png": true, "jpg": true, "gif": true, "webp": true, "svg": true, "pdf": true}
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Resolver{o: o, res: res, ctx: ctx, cache: map[string]Asset{}, files: map[string][]byte{}}
}

// Files returns the asset files to place in the bundle.
func (r *Resolver) Files() map[string][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string][]byte, len(r.files))
	for k, v := range r.files {
		out[k] = v
	}
	return out
}

// Resolve prepares src. Results are cached per source string.
func (r *Resolver) Resolve(src string) Asset {
	src = strings.TrimSpace(src)
	r.mu.Lock()
	if a, ok := r.cache[src]; ok {
		r.mu.Unlock()
		return a
	}
	r.mu.Unlock()
	a := r.resolve(src)
	r.mu.Lock()
	r.cache[src] = a
	r.mu.Unlock()
	return a
}

// Load returns the raw content of an image source (a "res:" name, a data:
// URI, a local path or, when allowed, a remote URL) under the resolver's
// access rules, without converting it. name is a file name for it.
func (r *Resolver) Load(src string) (name string, data []byte, err error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", nil, errors.New("empty image source")
	}
	return r.load(src)
}

func (r *Resolver) resolve(src string) Asset {
	if src == "" {
		return Asset{Missing: "empty image source"}
	}
	name, data, err := r.load(src)
	if err != nil {
		return Asset{Missing: err.Error()}
	}
	return r.prepare(name, data)
}

func (r *Resolver) load(src string) (string, []byte, error) {
	lower := strings.ToLower(src)
	switch {
	case strings.HasPrefix(src, "res:"):
		res, ok := r.res.Get(src)
		if !ok {
			return "", nil, errors.New("embedded image not found")
		}
		return res.Name, res.Data, nil
	case strings.HasPrefix(lower, "data:"):
		return decodeDataURI(src)
	case strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://"):
		if !r.o.AllowRemote {
			return "", nil, errors.New("remote images are disabled (allow fetching remote images to include it)")
		}
		return r.fetch(src)
	case strings.HasPrefix(lower, "file://"):
		u, err := url.Parse(src)
		if err != nil {
			return "", nil, err
		}
		src = u.Path
	}
	if !r.o.AllowLocalFiles {
		return "", nil, errors.New("local files are not accessible for this document")
	}
	p := src
	if unescaped, err := url.PathUnescape(p); err == nil && !fileExists(p) {
		p = unescaped
	}
	if r.o.RestrictToBase {
		safe, err := r.confined(p)
		if err != nil {
			return "", nil, err
		}
		p = safe
	} else if !filepath.IsAbs(p) {
		p = filepath.Join(r.o.BaseDir, filepath.FromSlash(p))
	}
	f, err := os.Open(p)
	if err != nil {
		return "", nil, fmt.Errorf("cannot open %s", filepath.Base(p))
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, r.o.MaxBytes+1))
	if err != nil {
		return "", nil, err
	}
	if int64(len(data)) > r.o.MaxBytes {
		return "", nil, errors.New("image file is too large")
	}
	return filepath.Base(p), data, nil
}

// confined resolves a relative path inside BaseDir and refuses anything that
// escapes it, including through symbolic links.
func (r *Resolver) confined(p string) (string, error) {
	denied := errors.New("image path is outside the document's files")
	if r.o.BaseDir == "" || filepath.IsAbs(p) || strings.HasPrefix(filepath.ToSlash(p), "/") {
		return "", denied
	}
	base, err := filepath.EvalSymlinks(r.o.BaseDir)
	if err != nil {
		return "", denied
	}
	full := filepath.Join(base, filepath.FromSlash(p))
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", fmt.Errorf("cannot open %s", filepath.Base(p))
	}
	rel, err := filepath.Rel(base, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", denied
	}
	return real, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (r *Resolver) fetch(u string) (string, []byte, error) {
	ctx, cancel := context.WithTimeout(r.ctx, 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "image/*")
	resp, err := r.o.HTTPClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, r.o.MaxBytes+1))
	if err != nil {
		return "", nil, err
	}
	if int64(len(data)) > r.o.MaxBytes {
		return "", nil, errors.New("remote image is too large")
	}
	pu, _ := url.Parse(u)
	name := "remote"
	if pu != nil {
		name = path.Base(pu.Path)
	}
	return name, data, nil
}

// prepare sniffs the format, converts when needed and stores the file.
func (r *Resolver) prepare(name string, data []byte) Asset {
	format := sniff(data)
	if format == "" && (isWMF(data) || isEMF(data)) {
		if img, f, ok := metafileBitmap(data); ok {
			data, format = img, f
		} else {
			return Asset{Missing: "vector Windows metafile pictures are not supported" + extHint("wmf")}
		}
	}
	if format == "" {
		ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
		return Asset{Missing: "unsupported or unrecognised image format" + extHint(ext)}
	}
	var a Asset
	switch {
	case r.o.Formats[format]:
		a.Format = format
	case format == "bmp" || format == "tiff" || (format == "webp" && !r.o.Formats["webp"]) || (format == "gif" && !r.o.Formats["gif"]):
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return Asset{Missing: "could not decode " + strings.ToUpper(format) + " image"}
		}
		var buf bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
			return Asset{Missing: "could not convert image"}
		}
		data, format, a.Format = buf.Bytes(), "png", "png"
	case format == "heic" || format == "avif":
		conv, err := convertWithSips(r.ctx, data, format)
		if err != nil {
			return Asset{Missing: strings.ToUpper(format) + " images need conversion to JPEG or PNG first"}
		}
		data, format, a.Format = conv, "jpg", "jpg"
	default:
		return Asset{Missing: strings.ToUpper(format) + " images are not supported by the selected engine; convert to PNG or PDF"}
	}

	w, h, dpi := dimensions(data, format)
	a.WidthPx, a.HeightPx = w, h
	if w > 0 && h > 0 {
		if dpi < 72 || dpi > 1200 {
			dpi = 96
		}
		a.NaturalWidthPt = float64(w) * 72 / dpi
		a.NaturalHeightPt = float64(h) * 72 / dpi
	}
	sum := sha256.Sum256(data)
	a.Path = "assets/" + hex.EncodeToString(sum[:8]) + "." + format
	r.mu.Lock()
	r.files[a.Path] = data
	r.mu.Unlock()
	return a
}

func extHint(ext string) string {
	switch ext {
	case "emf", "wmf":
		return " (Windows metafile — save the picture as PNG or SVG)"
	case "eps", "ps":
		return " (PostScript — convert to PDF)"
	}
	if ext != "" {
		return " (." + ext + ")"
	}
	return ""
}

// sniff identifies an image format from its magic bytes.
func sniff(b []byte) string {
	switch {
	case len(b) >= 8 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "jpg"
	case len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a"):
		return "gif"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp"
	case len(b) >= 2 && string(b[:2]) == "BM":
		return "bmp"
	case len(b) >= 4 && (string(b[:4]) == "II*\x00" || string(b[:4]) == "MM\x00*"):
		return "tiff"
	case len(b) >= 5 && string(b[:5]) == "%PDF-":
		return "pdf"
	case len(b) >= 12 && string(b[4:8]) == "ftyp":
		brand := string(b[8:12])
		switch brand {
		case "heic", "heix", "hevc", "hevx", "mif1", "msf1":
			return "heic"
		case "avif", "avis":
			return "avif"
		}
	}
	head := b
	if len(head) > 1024 {
		head = head[:1024]
	}
	t := strings.ToLower(string(head))
	if strings.Contains(t, "<svg") || (strings.HasPrefix(strings.TrimSpace(t), "<?xml") && bytes.Contains(bytes.ToLower(b[:min(len(b), 4096)]), []byte("<svg"))) {
		return "svg"
	}
	return ""
}

// dimensions returns pixel size (after EXIF rotation) and DPI (0 = unknown).
func dimensions(data []byte, format string) (int, int, float64) {
	switch format {
	case "svg", "pdf":
		return 0, 0, 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, 0
	}
	w, h := cfg.Width, cfg.Height
	dpi := 0.0
	switch format {
	case "png":
		dpi = pngDPI(data)
	case "jpg":
		var orient int
		dpi, orient = jpegInfo(data)
		if orient >= 5 && orient <= 8 {
			w, h = h, w
		}
	}
	return w, h, dpi
}

func pngDPI(b []byte) float64 {
	i := 8
	for i+12 <= len(b) {
		n := int(binary.BigEndian.Uint32(b[i:]))
		typ := string(b[i+4 : i+8])
		if typ == "pHYs" && n == 9 && i+8+9 <= len(b) {
			ppu := binary.BigEndian.Uint32(b[i+8:])
			if b[i+16] == 1 { // metres
				return float64(ppu) * 0.0254
			}
			return 0
		}
		if typ == "IDAT" || n < 0 || n > len(b) {
			return 0
		}
		i += 12 + n
	}
	return 0
}

// jpegInfo reads JFIF density and the EXIF orientation tag.
func jpegInfo(b []byte) (float64, int) {
	dpi, orient := 0.0, 1
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			break
		}
		marker := b[i+1]
		if marker == 0xDA || marker == 0xD9 {
			break
		}
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		if n < 2 || i+2+n > len(b) {
			break
		}
		seg := b[i+4 : i+2+n]
		switch {
		case marker == 0xE0 && len(seg) >= 12 && string(seg[:5]) == "JFIF\x00":
			units := seg[7]
			x := float64(binary.BigEndian.Uint16(seg[8:]))
			switch units {
			case 1:
				dpi = x
			case 2:
				dpi = x * 2.54
			}
		case marker == 0xE1 && len(seg) >= 14 && string(seg[:6]) == "Exif\x00\x00":
			if o := exifOrientation(seg[6:]); o > 0 {
				orient = o
			}
		}
		i += 2 + n
	}
	return dpi, orient
}

func exifOrientation(t []byte) int {
	if len(t) < 8 {
		return 0
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	off := int(bo.Uint32(t[4:]))
	if off+2 > len(t) {
		return 0
	}
	n := int(bo.Uint16(t[off:]))
	for k := 0; k < n; k++ {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 0
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			return int(bo.Uint16(t[e+8:]))
		}
	}
	return 0
}

func decodeDataURI(s string) (string, []byte, error) {
	meta, payload, ok := strings.Cut(s[len("data:"):], ",")
	if !ok {
		return "", nil, errors.New("malformed data: URI")
	}
	var data []byte
	var err error
	if strings.HasSuffix(strings.ToLower(meta), ";base64") {
		clean := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, payload)
		data, err = decodeBase64(clean)
	} else {
		var u string
		u, err = url.PathUnescape(payload)
		data = []byte(u)
	}
	if err != nil {
		return "", nil, errors.New("malformed data: URI")
	}
	return "embedded", data, nil
}

func decodeBase64(s string) ([]byte, error) {
	for _, enc := range encodings {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("invalid base64")
}

// convertWithSips converts HEIC/AVIF on macOS with the built-in sips tool.
func convertWithSips(ctx context.Context, data []byte, format string) ([]byte, error) {
	if runtime.GOOS != "darwin" {
		return nil, errors.New("no converter")
	}
	sips, err := exec.LookPath("sips")
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "crowdoc-img-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "in."+format)
	out := filepath.Join(dir, "out.jpg")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, sips, "-s", "format", "jpeg", "-s", "formatOptions", "90", in, "--out", out)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}
