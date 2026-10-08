package odt

import (
	"context"
	"testing"

	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func FuzzReadFlat(f *testing.F) {
	seed := flatODT(contentBodyForFuzz, "")
	for i := 0; i < len(seed); i += 61 {
		f.Add([]byte(seed[:i]))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = Read(context.Background(), data, rd.Options{Limits: rd.Limits{MaxTableCells: 500}})
	})
}

const contentBodyForFuzz = `<text:h text:outline-level="2">H</text:h><text:p>a<text:span text:style-name="T1">b</text:span><text:note><text:note-body><text:p>n</text:p></text:note-body></text:note></text:p>
<text:list text:style-name="LH"><text:list-item><text:list><text:list-item><text:p>x</text:p></text:list-item></text:list></text:list-item></text:list>
<table:table><table:table-row table:number-rows-repeated="5"><table:table-cell table:number-columns-spanned="3" table:number-rows-spanned="9"><text:p>c</text:p></table:table-cell></table:table-row></table:table>
<text:p><draw:frame><draw:text-box><text:p><draw:frame><draw:image/></draw:frame><text:sequence>1</text:sequence></text:p></draw:text-box></draw:frame></text:p>`
