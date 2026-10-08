package crowdoc_test

import (
	"context"
	"fmt"

	"github.com/askrejans/crowdoc/v2"
)

// echoEngine stands in for an embedded Typst: it reports the job it got.
type echoEngine struct{}

func (echoEngine) Name() string { return "echo" }

func (echoEngine) Compile(_ context.Context, job *crowdoc.EngineJob) (*crowdoc.EngineResult, error) {
	return &crowdoc.EngineResult{PDF: []byte("%PDF-1.7 " + job.Main)}, nil
}

func (echoEngine) Fonts(context.Context) (map[string]bool, error) {
	return map[string]bool{"libertinus serif": true}, nil
}

// A custom engine receives the rendered project and returns the PDF.
func ExampleEngine() {
	res, err := crowdoc.Convert(context.Background(),
		crowdoc.Source{Name: "note.md", Data: []byte("# Hello\n\nWorld.")},
		crowdoc.Options{Engine: echoEngine{}})
	if err != nil {
		panic(err)
	}
	fmt.Println(string(res.PDF))
	// Output: %PDF-1.7 main.typ
}
