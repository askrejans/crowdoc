package mathml

import "testing"

func FuzzConvertXML(f *testing.F) {
	seeds := []string{
		`<math><munderover><mo>∑</mo><mi>i</mi><mi>n</mi></munderover></math>`,
		`<math><mrow><mo>(</mo><mtable><mtr><mtd><mn>1</mn></mtd></mtr></mtable><mo>)</mo></mrow></math>`,
		`<math><mmultiscripts><mi>X</mi><mprescripts/><mi>a</mi></mmultiscripts></math>`,
		`<math><semantics><mi>x</mi><annotation encoding="TeX">\frac{1}{2}</annotation></semantics></math>`,
		`<math><mfenced open="{" close=""><mtable columnalign="left"><mtr><mtd><mi>a</mi></mtd></mtr></mtable></mfenced></math>`,
	}
	for _, s := range seeds {
		for i := 0; i <= len(s); i += 7 {
			f.Add([]byte(s[:i]))
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		tex, _, err := ConvertXML(data)
		if err == nil && !SafeTeX(tex) && tex != "" {
			// Generated TeX only uses known commands; braces must balance.
			t.Fatalf("unsafe output %q for %q", tex, data)
		}
	})
}
