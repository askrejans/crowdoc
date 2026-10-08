package layout_test

import (
	"context"
	"fmt"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/layout"
)

// Word boxes as an OCR engine reports them, converted from pixels at
// 300 dpi to points (× 72 / 300). There is no font information: sizes are
// estimated from the box heights.
func Example_ocr() {
	type box struct {
		text       string
		x, y, w, h int // pixels, top-left origin
	}
	words := []box{
		{"Harvest", 300, 300, 420, 100}, {"Report", 760, 300, 380, 100},
		{"The", 300, 520, 90, 50}, {"harvest", 410, 520, 180, 50}, {"started", 610, 520, 170, 50},
		{"early", 800, 520, 120, 62}, {"this", 940, 520, 90, 50}, {"year.", 1050, 520, 130, 62},
	}
	px := 72.0 / 300
	var page layout.Page
	page.Width, page.Height = 2480*px, 3508*px // A4 at 300 dpi
	for _, w := range words {
		page.Runs = append(page.Runs, layout.Run{
			Text: w.text,
			X:    float64(w.x) * px, Y: float64(w.y) * px,
			W: float64(w.w) * px, H: float64(w.h) * px,
		})
	}
	doc, _ := layout.Document(context.Background(), []layout.Page{page}, layout.Options{Lang: "en"})
	fmt.Println(doc.Meta.Title)
	fmt.Print(ast.Dump(doc.Blocks))
	// Output:
	// Harvest Report
	// Para The harvest started early this year.
}
