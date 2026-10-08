package pdf

import (
	"bytes"
	"compress/flate"
	"encoding/ascii85"
	"errors"
	"testing"

	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func testFile() *file {
	return &file{budget: 1 << 30, maxEntry: 1 << 28}
}

func TestFlate(t *testing.T) {
	want := bytes.Repeat([]byte("compressible text "), 50)
	got, err := inflate(deflate(want), 1<<20)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("zlib: %q, %v", got, err)
	}
	// Raw deflate data without the zlib header.
	var raw bytes.Buffer
	w, _ := flate.NewWriter(&raw, flate.BestCompression)
	w.Write(want)
	w.Close()
	if got, err := inflate(raw.Bytes(), 1<<20); err != nil || !bytes.Equal(got, want) {
		t.Errorf("raw deflate: %d bytes, %v", len(got), err)
	}
	// Truncated data yields what could be decoded.
	z := deflate(want)
	if got, _ := inflate(z[:len(z)/2], 1<<20); len(got) == 0 || !bytes.HasPrefix(want, got) {
		t.Errorf("truncated: %q", got)
	}
	// The output limit is enforced.
	if _, err := inflate(deflate(make([]byte, 1<<16)), 1000); !errors.Is(err, rd.ErrLimit) {
		t.Errorf("limit: %v", err)
	}
}

func TestPredictors(t *testing.T) {
	rows := [][]byte{{10, 20, 30, 40}, {11, 22, 33, 44}, {255, 0, 128, 7}}
	var flat []byte
	for _, r := range rows {
		flat = append(flat, r...)
	}
	// Encode with every PNG filter type and with the TIFF predictor.
	enc := func(ft byte) []byte {
		var out []byte
		prev := make([]byte, 4)
		for _, r := range rows {
			out = append(out, ft)
			for i := range r {
				var left, up, ul byte
				if i >= 1 {
					left, ul = r[i-1], prev[i-1]
				}
				up = prev[i]
				switch ft {
				case 0:
					out = append(out, r[i])
				case 1:
					out = append(out, r[i]-left)
				case 2:
					out = append(out, r[i]-up)
				case 3:
					out = append(out, r[i]-byte((int(left)+int(up))/2))
				case 4:
					out = append(out, r[i]-paeth(left, up, ul))
				}
			}
			prev = r
		}
		return out
	}
	parms := dict{"Predictor": 12, "Columns": 4}
	for ft := byte(0); ft <= 4; ft++ {
		got, err := unpredict(enc(ft), parms)
		if err != nil || !bytes.Equal(got, flat) {
			t.Errorf("PNG filter %d: %v, %v", ft, got, err)
		}
	}
	var tiff []byte
	for _, r := range rows {
		tiff = append(tiff, r[0])
		for i := 1; i < len(r); i++ {
			tiff = append(tiff, r[i]-r[i-1])
		}
	}
	if got, _ := unpredict(tiff, dict{"Predictor": 2, "Columns": 4}); !bytes.Equal(got, flat) {
		t.Errorf("TIFF: %v", got)
	}
	// Three components per pixel: the PNG Sub filter works per pixel.
	rgb := []byte{1, 10, 1, 2, 3, 4} // two pixels, Sub-filtered
	got, _ := unpredict(append([]byte{1}, rgb...), dict{"Predictor": 11, "Colors": 3, "Columns": 2})
	if want := []byte{1, 10, 1, 3, 13, 5}; !bytes.Equal(got, want) {
		t.Errorf("RGB Sub: %v, want %v", got, want)
	}
}

func TestLZW(t *testing.T) {
	// The example from the PDF reference ("-----A---B", early change).
	enc := []byte{0x80, 0x0B, 0x60, 0x50, 0x22, 0x0C, 0x0C, 0x85, 0x01}
	got, err := lzwDecode(enc, true, 1<<20)
	if err != nil || string(got) != "-----A---B" {
		t.Errorf("got %q, %v", got, err)
	}
	// Garbage stops decoding without a panic.
	lzwDecode([]byte{0xff, 0xff, 0xff, 0xff, 0xff}, true, 1<<20)
}

func TestASCIIFilters(t *testing.T) {
	want := []byte("Text with all bytes \x00\x01\xfe\xff and zeros \x00\x00\x00\x00 end")
	enc := make([]byte, ascii85.MaxEncodedLen(len(want)))
	n := ascii85.Encode(enc, want)
	a85 := append(append([]byte("<~"), enc[:n]...), "~>"...)
	if got := ascii85Decode(a85); !bytes.Equal(got, want) {
		t.Errorf("ASCII85: %q", got)
	}
	// Whitespace, the "z" shorthand and a missing terminator.
	if got := ascii85Decode([]byte(" z 87cURD]i,\"Ebo80")); string(got) != "\x00\x00\x00\x00Hello World!" {
		t.Errorf("ASCII85 z: %q", got)
	}
	f := testFile()
	if got, _ := f.applyFilter("ASCIIHexDecode", []byte("48 65 6c\n6C 6f>trailing"), nil); string(got) != "Hello" {
		t.Errorf("ASCIIHex: %q", got)
	}
	// RunLength: a literal run of 3, a repeat of 4, end of data.
	rl := []byte{2, 'a', 'b', 'c', 256 - 3, 'z', 128, 'x'}
	if got, _ := runLengthDecode(rl, 1<<20); string(got) != "abczzzz" {
		t.Errorf("RunLength: %q", got)
	}
}

func TestFilterChain(t *testing.T) {
	want := []byte("BT /F1 12 Tf (chained filters) Tj ET")
	z := deflate(want)
	enc := make([]byte, ascii85.MaxEncodedLen(len(z)))
	n := ascii85.Encode(enc, z)
	f := testFile()
	s := &stream{d: dict{"Filter": array{name("A85"), name("Fl")}}, raw: append(enc[:n], "~>"...)}
	got, _, _, err := f.decodeStream(s, false)
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("got %q, %v", got, err)
	}
	// Image codecs are handed over undecoded.
	s = &stream{d: dict{"Filter": array{name("FlateDecode"), name("DCTDecode")}}, raw: deflate([]byte("jpeg"))}
	got, codec, _, err := f.decodeStream(s, true)
	if err != nil || codec != "DCTDecode" || string(got) != "jpeg" {
		t.Errorf("image: %q %q %v", got, codec, err)
	}
	if _, _, _, err := f.decodeStream(&stream{d: dict{"Filter": name("JBIG2Decode")}}, false); err == nil {
		t.Error("image filter in a content stream accepted")
	}
	if _, _, _, err := f.decodeStream(&stream{d: dict{"Filter": name("Unknown")}}, false); err == nil {
		t.Error("unknown filter accepted")
	}
}

func TestDecompressionBudget(t *testing.T) {
	f := &file{budget: 5000, maxEntry: 1 << 20}
	s := &stream{d: dict{"Filter": name("FlateDecode")}, raw: deflate(make([]byte, 4000))}
	if _, _, _, err := f.decodeStream(s, false); err != nil {
		t.Fatal(err)
	}
	// The budget is shared by all streams of the file.
	if _, _, _, err := f.decodeStream(s, false); !errors.Is(err, rd.ErrLimit) {
		t.Errorf("second stream: %v", err)
	}
}
