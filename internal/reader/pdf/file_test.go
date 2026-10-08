package pdf

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func readText(t *testing.T, data []byte) string {
	t.Helper()
	doc, _, err := read(context.Background(), data, rd.Options{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return strings.TrimSpace(ast.BlocksText(doc.Blocks))
}

func TestClassicXref(t *testing.T) {
	data := classicPDF("/Root 1 0 R", onePage("BT /F1 12 Tf 72 300 Td (Hello classic) Tj ET")...)
	if got := readText(t, data); got != "Hello classic" {
		t.Errorf("got %q", got)
	}
}

func TestXrefStreamAndObjectStream(t *testing.T) {
	objs := map[int]string{}
	for i, o := range onePage("BT /F1 12 Tf 72 300 Td (Packed objects) Tj ET") {
		objs[i+1] = o
	}
	// The catalog, page tree and page live in the object stream; streams
	// cannot, so the content stays a top-level object.
	data := objStmPDF(1, objs, []int{1, 2, 3})
	f, err := openFile(data, rd.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if e := f.xref[2]; e.kind != 2 || e.idx != 1 {
		t.Errorf("object 2 entry %+v", e)
	}
	if d, ok := f.getObj(3).(dict); !ok || d["Type"] != name("Page") {
		t.Errorf("object 3 = %#v", f.getObj(3))
	}
	if got := readText(t, data); got != "Packed objects" {
		t.Errorf("got %q", got)
	}
}

func TestIncrementalUpdate(t *testing.T) {
	base := classicPDF("/Root 1 0 R", onePage("BT /F1 12 Tf 72 300 Td (Old text) Tj ET")...)
	prev := bytes.LastIndex(base, []byte("startxref"))
	prevOff := 0
	fmt.Sscanf(string(base[prev+len("startxref"):]), "%d", &prevOff)
	var b bytes.Buffer
	b.Write(base)
	off := b.Len()
	fmt.Fprintf(&b, "4 0 obj\n%s\nendobj\n", streamObj("", []byte("BT /F1 12 Tf 72 300 Td (New text) Tj ET")))
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n4 1\n%010d 00000 n \ntrailer\n<< /Size 5 /Root 1 0 R /Prev %d >>\nstartxref\n%d\n%%%%EOF\n", off, prevOff, xref)
	if got := readText(t, b.Bytes()); got != "New text" {
		t.Errorf("got %q, want the updated object", got)
	}
}

func TestRepair(t *testing.T) {
	good := classicPDF("/Root 1 0 R", onePage("BT /F1 12 Tf 72 300 Td (Recovered) Tj ET")...)
	tests := map[string][]byte{
		// Every offset is shifted by the junk prefix.
		"prefix": append([]byte("some junk the mailer added\r\n"), good...),
		// The table points nowhere.
		"bad offsets": bytes.Replace(good, []byte("0000000"), []byte("0009999"), -1),
		// No cross-reference table or trailer at all.
		"no xref": good[:bytes.Index(good, []byte("xref"))],
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if got := readText(t, data); got != "Recovered" {
				t.Errorf("got %q", got)
			}
		})
	}
}

func TestPageTreeInheritance(t *testing.T) {
	// Resources and MediaBox are inherited from the page tree.
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 /MediaBox [0 0 300 500] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Times-Roman >> >> >> >>",
		"<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>",
		streamObj("", []byte("BT /F1 10 Tf 50 400 Td (Inherited) Tj ET")),
	}
	data := classicPDF("/Root 1 0 R", objs...)
	pages, err := extractPages(context.Background(), data, rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || pages[0].Width != 300 || pages[0].Height != 500 {
		t.Fatalf("pages %+v", pages)
	}
	r := pages[0].Runs[0]
	if r.Text != "Inherited" || r.FontSize != 10 || r.X != 50 || r.Y != 500-400-8 {
		t.Errorf("run %+v", r)
	}
}

func TestPageLimit(t *testing.T) {
	var kids []string
	objs := []string{"<< /Type /Catalog /Pages 2 0 R >>", ""}
	n := MaxPages + 3
	for i := 0; i < n; i++ {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+i))
		objs = append(objs, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] >>")
	}
	objs[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n)
	f, err := openFile(classicPDF("/Root 1 0 R", objs...), rd.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	pages, truncated := f.pages(MaxPages)
	if len(pages) != MaxPages || !truncated {
		t.Errorf("%d pages, truncated %v", len(pages), truncated)
	}
}

func TestCyclicStructures(t *testing.T) {
	// A page tree that contains itself and a reference chain that loops.
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [2 0 R 3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox 5 0 R /Contents 4 0 R /Resources << /Font << /F1 << /Subtype /Type1 /BaseFont /Courier >> >> >> >>",
		streamObj("", []byte("BT /F1 12 Tf 10 50 Td (Loop) Tj ET")),
		"6 0 R",
		"5 0 R",
	}
	if got := readText(t, classicPDF("/Root 1 0 R", objs...)); got != "Loop" {
		t.Errorf("got %q", got)
	}
}

func TestRotatedPage(t *testing.T) {
	// A landscape page stored as portrait: /Rotate 90 and text drawn with
	// a matching text matrix reads upright on screen.
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 500 300] /Rotate 90 /Contents 4 0 R /Resources << /Font << /F1 << /Subtype /Type1 /BaseFont /Helvetica >> >> >> >>",
		streamObj("", []byte("BT /F1 10 Tf 0 1 -1 0 100 50 Tm (Upright) Tj ET BT /F1 10 Tf 200 200 Td (Sideways) Tj ET")),
	}
	pages, err := extractPages(context.Background(), classicPDF("/Root 1 0 R", objs...), rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := pages[0]
	if p.Width != 300 || p.Height != 500 || len(p.Runs) != 1 {
		t.Fatalf("page %vx%v runs %+v", p.Width, p.Height, p.Runs)
	}
	if r := p.Runs[0]; r.Text != "Upright" || r.X != 50 || r.Y != 92 {
		t.Errorf("run %+v", r)
	}
}
