package pdf

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// FuzzRead feeds arbitrary bytes to the reader. It calls read, not Read,
// so that a panic is reported instead of being turned into an error.
func FuzzRead(f *testing.F) {
	seeds, _ := filepath.Glob("testdata/*.pdf")
	for _, s := range seeds {
		data, err := os.ReadFile(s)
		if err != nil || len(data) > 64<<10 {
			continue
		}
		f.Add(data)
	}
	f.Add([]byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj 3 0 obj<</Type/Page/Contents 4 0 R/Resources<</Font<</F1<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>>>>>>>endobj 4 0 obj<</Length 30>>stream\nBT /F1 12 Tf 72 700 Td (Hi) Tj ET\nendstream endobj trailer<</Root 1 0 R>>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		doc, _, err := read(ctx, data, rd.Options{Limits: rd.Limits{MaxUnpacked: 64 << 20, MaxEntry: 16 << 20}})
		if err == nil && (doc == nil || doc.Resources == nil) {
			t.Fatal("nil document without an error")
		}
	})
}
