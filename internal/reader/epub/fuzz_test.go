package epub

import (
	"context"
	"testing"

	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func FuzzRead(f *testing.F) {
	book := epub3(f)
	f.Add(book)
	for i := 0; i < len(book); i += len(book) / 16 {
		corrupt := append([]byte(nil), book...)
		corrupt[i] ^= 0xff
		f.Add(corrupt)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = Read(context.Background(), data, rd.Options{})
	})
}
