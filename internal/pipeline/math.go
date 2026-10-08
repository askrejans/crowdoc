package pipeline

import (
	"github.com/askrejans/crowdoc/v2/internal/mathconv/tex2typ"
	"github.com/askrejans/crowdoc/v2/internal/typst"
)

// mathHook converts LaTeX math notation to Typst math.
func mathHook(tex string) typst.MathResult {
	r := tex2typ.Convert(tex)
	return typst.MathResult{Typst: r.Typst, Label: r.Label, Warnings: r.Warnings}
}
