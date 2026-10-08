package html

import (
	"context"
	"testing"

	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// FuzzRead checks that arbitrary input never panics; the seeds include
// every prefix-truncation shape of a realistic page.
func FuzzRead(f *testing.F) {
	for _, seed := range []string{blogPage, converterPage, wikiPage} {
		for i := 0; i < len(seed); i += 97 {
			f.Add([]byte(seed[:i]))
		}
	}
	f.Add([]byte("<math><mtable><mtr><mtd><mfrac><mi>"))
	f.Add([]byte("<table><tr><td rowspan=0 colspan=99999>x"))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, _, err := Read(context.Background(), data, rd.Options{Limits: rd.Limits{MaxTableCells: 1000}})
		if err == nil && doc.Resources == nil {
			t.Fatal("nil resources")
		}
	})
}
