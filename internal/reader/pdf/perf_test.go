package pdf

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// paperPDF builds a two-column paper of n pages with base-14 fonts: a
// section heading at the top of each page and justified-looking body
// lines in both columns.
func paperPDF(n int) []byte {
	words := strings.Fields("consensus protocols heterogeneous wireless sensor networks energy budget radio range computational capability communication overhead convergence guarantees intermittent connectivity deployment theoretical predictions")
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"", // page tree, filled in below
		"<< /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Times-Roman >> /F2 << /Type /Font /Subtype /Type1 /BaseFont /Times-Bold >> >> >>",
	}
	var kids []string
	w := 0
	for p := 0; p < n; p++ {
		var c bytes.Buffer
		fmt.Fprintf(&c, "BT /F2 14 Tf 57 780 Td (%d Section number %d) Tj ET\n", p+1, p+1)
		for col := 0; col < 2; col++ {
			x := 57 + col*250
			for l := 0; l < 52; l++ {
				var line []string
				for len(strings.Join(line, " ")) < 40 {
					line = append(line, words[w%len(words)])
					w++
				}
				fmt.Fprintf(&c, "BT /F1 10 Tf %d %d Td (%s) Tj ET\n", x, 750-l*13, strings.Join(line, " "))
			}
		}
		objs = append(objs, streamObj("", c.Bytes()))
		content := len(objs)
		objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources 3 0 R /Contents %d 0 R >>", content))
		kids = append(kids, fmt.Sprintf("%d 0 R", len(objs)))
	}
	objs[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n)
	return classicPDF("/Root 1 0 R", objs...)
}

func TestThirtyPagePerformance(t *testing.T) {
	data := paperPDF(30)
	start := time.Now()
	doc, _, err := Read(context.Background(), data, rd.Options{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	headings := 0
	for _, b := range doc.Blocks {
		if _, ok := b.(*ast.Heading); ok {
			headings++
		}
	}
	if headings != 30 {
		t.Errorf("%d headings, want 30", headings)
	}
	// About 50 ms on a laptop; the bound leaves room for slow, shared and
	// race-instrumented test machines.
	if elapsed > 5*time.Second {
		t.Errorf("reading 30 pages took %v", elapsed)
	}
}

func BenchmarkRead(b *testing.B) {
	for _, name := range []string{"latex-article.pdf", "typst-roundtrip.pdf", "lo-report.pdf"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				if _, _, err := Read(context.Background(), data, rd.Options{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	data := paperPDF(30)
	b.Run("paper-30-pages", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, _, err := Read(context.Background(), data, rd.Options{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
