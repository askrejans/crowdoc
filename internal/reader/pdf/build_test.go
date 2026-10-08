package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
)

// Helpers that assemble small PDF files in memory for the parser tests.

// classicPDF numbers objs from 1 and writes them with a cross-reference
// table. Each obj is the text between "N 0 obj" and "endobj".
func classicPDF(trailer string, objs ...string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d %s >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, trailer, xref)
	return b.Bytes()
}

// streamObj returns a stream object with the given dictionary entries.
func streamObj(dict string, data []byte) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(data), data)
}

func deflate(data []byte) []byte {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(data)
	w.Close()
	return b.Bytes()
}

// pngUp encodes rows with the PNG "Up" predictor (filter type 2).
func pngUp(rows [][]byte) []byte {
	var out []byte
	prev := make([]byte, len(rows[0]))
	for _, r := range rows {
		out = append(out, 2)
		for i, c := range r {
			out = append(out, c-prev[i])
		}
		prev = r
	}
	return out
}

// objStmPDF stores the objects listed in packed inside an object stream
// and indexes everything with a compressed cross-reference stream using
// the PNG Up predictor, as PDF 1.5 writers do.
func objStmPDF(root int, objs map[int]string, packed []int) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.5\n")
	maxNum := 0
	for n := range objs {
		maxNum = max(maxNum, n)
	}
	inStm := map[int]int{}
	var header, body strings.Builder
	for i, n := range packed {
		inStm[n] = i
		fmt.Fprintf(&header, "%d %d ", n, body.Len())
		body.WriteString(objs[n] + "\n")
	}
	stmNum := maxNum + 1
	xrefNum := maxNum + 2
	type entry struct{ typ, f2, f3 int }
	entries := make([]entry, xrefNum+1)
	for n := 1; n <= maxNum; n++ {
		o, ok := objs[n]
		if !ok {
			continue
		}
		if idx, ok := inStm[n]; ok {
			entries[n] = entry{2, stmNum, idx}
			continue
		}
		entries[n] = entry{1, b.Len(), 0}
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n, o)
	}
	data := []byte(header.String() + body.String())
	entries[stmNum] = entry{1, b.Len(), 0}
	fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", stmNum, streamObj(fmt.Sprintf("/Type /ObjStm /N %d /First %d /Filter /FlateDecode", len(packed), header.Len()), deflate(data)))
	entries[xrefNum] = entry{1, b.Len(), 0}
	var rows [][]byte
	for _, e := range entries {
		rows = append(rows, []byte{byte(e.typ), byte(e.f2 >> 8), byte(e.f2), byte(e.f3)})
	}
	xs := deflate(pngUp(rows))
	fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", xrefNum, streamObj(fmt.Sprintf(
		"/Type /XRef /Size %d /W [1 2 1] /Root %d 0 R /Filter /FlateDecode /DecodeParms << /Predictor 12 /Columns 4 >>",
		xrefNum+1, root), xs))
	fmt.Fprintf(&b, "startxref\n%d\n%%%%EOF\n", entries[xrefNum].f2)
	return b.Bytes()
}

// onePage returns the objects of a one-page document whose page draws
// content with Helvetica as /F1; extra objects follow from number 5.
func onePage(content string, extra ...string) []string {
	return append([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 400 400] /Contents 4 0 R /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >> >> >> >>",
		streamObj("", []byte(content)),
	}, extra...)
}
