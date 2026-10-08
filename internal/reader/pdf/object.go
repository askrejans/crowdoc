package pdf

import "math"

// PDF object model. Values are represented with plain Go types:
//
//	null        nil
//	boolean     bool
//	integer     int
//	real        float64
//	name        name
//	string      pdfString (raw bytes, already decrypted)
//	array       array
//	dictionary  dict
//	stream      *stream
//	reference   ref
//
// Content-stream operators are returned by the lexer as keyword values.
type (
	name      string
	pdfString string
	keyword   string
	array     []any
	dict      map[name]any
	ref       struct{ num, gen int }
)

// stream is a stream object. raw holds the bytes between "stream" and
// "endstream" exactly as stored in the file (still encrypted and encoded).
type stream struct {
	d   dict
	raw []byte
	ref ref
	// plain marks streams that must not be decrypted (cross-reference
	// streams, streams inside decrypted containers).
	plain bool
}

// num converts a numeric object to float64.
func num(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, false
		}
		return x, true
	}
	return 0, false
}

// integer converts a numeric object to int (reals are truncated).
func integer(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case float64:
		if math.IsNaN(x) || x > math.MaxInt32 || x < math.MinInt32 {
			return 0, false
		}
		return int(x), true
	}
	return 0, false
}

// matrix is an affine transformation [a b c d e f].
type matrix [6]float64

var identity = matrix{1, 0, 0, 1, 0, 0}

// mul returns m × n (apply m first, then n).
func (m matrix) mul(n matrix) matrix {
	return matrix{
		m[0]*n[0] + m[1]*n[2],
		m[0]*n[1] + m[1]*n[3],
		m[2]*n[0] + m[3]*n[2],
		m[2]*n[1] + m[3]*n[3],
		m[4]*n[0] + m[5]*n[2] + n[4],
		m[4]*n[1] + m[5]*n[3] + n[5],
	}
}

// apply transforms the point (x, y).
func (m matrix) apply(x, y float64) (float64, float64) {
	return x*m[0] + y*m[2] + m[4], x*m[1] + y*m[3] + m[5]
}

// matrixFrom reads a six-number array; ok is false when malformed.
func matrixFrom(v any) (matrix, bool) {
	a, ok := v.(array)
	if !ok || len(a) != 6 {
		return identity, false
	}
	var m matrix
	for i := range m {
		f, ok := num(a[i])
		if !ok {
			return identity, false
		}
		m[i] = f
	}
	return m, true
}

// rect is an axis-aligned rectangle in PDF user space.
type rect struct{ x0, y0, x1, y1 float64 }

func (r rect) w() float64 { return r.x1 - r.x0 }
func (r rect) h() float64 { return r.y1 - r.y0 }

// rectFrom reads a four-number array and normalises the corners.
func rectFrom(v any) (rect, bool) {
	a, ok := v.(array)
	if !ok || len(a) != 4 {
		return rect{}, false
	}
	var f [4]float64
	for i := range f {
		x, ok := num(a[i])
		if !ok {
			return rect{}, false
		}
		f[i] = x
	}
	r := rect{min(f[0], f[2]), min(f[1], f[3]), max(f[0], f[2]), max(f[1], f[3])}
	return r, r.w() > 0 && r.h() > 0
}
