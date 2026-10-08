package tex2typ

import (
	"math"
	"strconv"
	"strings"
)

// unitFactors converts TeX units to a Typst unit and multiplier.
var unitFactors = map[string]struct {
	unit   string
	factor float64
}{
	"pt": {"pt", 1}, "bp": {"pt", 1}, "mm": {"mm", 1}, "cm": {"cm", 1}, "in": {"in", 1},
	"em": {"em", 1}, "ex": {"em", 0.43}, "mu": {"em", 1.0 / 18}, "pc": {"pt", 12},
	"dd": {"pt", 1.07}, "cc": {"pt", 12.84}, "sp": {"pt", 1.0 / 65536}, "px": {"pt", 0.75},
}

// parseDimen reads a dimension argument, braced (\hspace{1em}) or inline
// (\kern 3pt, \mkern-2mu), and returns a Typst length or "" with a warning.
func (p *parser) parseDimen() string {
	p.skipSpace()
	var text string
	if p.peek().kind == tOpen {
		text = p.rawArg()
	} else {
		start := p.peek().pos
		end := scanDimen(p.src, start)
		for p.pos < len(p.toks) && p.toks[p.pos].pos < end {
			p.pos++
		}
		text = p.src[start:end]
	}
	d, ok := parseDimenText(text)
	if !ok {
		p.warn("unsupported length " + strconv.Quote(text) + " dropped")
		return ""
	}
	return d
}

// scanDimen returns the end of an inline TeX dimension starting at i.
func scanDimen(s string, i int) int {
	j := i
	for j < len(s) && (s[j] == '+' || s[j] == '-' || s[j] == ' ') {
		j++
	}
	digits := j
	for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.' || s[j] == ',') {
		j++
	}
	if j == digits {
		return i
	}
	for j < len(s) && s[j] == ' ' {
		j++
	}
	if j+2 <= len(s) {
		if _, ok := unitFactors[s[j:j+2]]; ok {
			return j + 2
		}
	}
	return digits
}

// parseDimenText converts "1.5em", "-3mu", "\fill" to a Typst length.
func parseDimenText(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == `\fill` || s == `\hfill` || s == `\stretch{1}` {
		return "1fr", true
	}
	neg := false
	for strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		neg = neg != (s[0] == '-')
		s = strings.TrimSpace(s[1:])
	}
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.' || s[i] == ',') {
		i++
	}
	num := strings.ReplaceAll(s[:i], ",", ".")
	unit := strings.TrimSpace(s[i:])
	if num == "" || num == "." {
		return "", false
	}
	v, err := strconv.ParseFloat(num, 64)
	if err != nil || math.IsInf(v, 0) || v > 1e4 {
		return "", false
	}
	f, ok := unitFactors[unit]
	if !ok {
		return "", false
	}
	v = math.Round(v*f.factor*10000) / 10000
	if neg && v != 0 {
		v = -v
	}
	return strconv.FormatFloat(v, 'f', -1, 64) + f.unit, true
}

// parseColor reads an xcolor specification ([model]{spec}) and returns a
// Typst colour expression, or "" (with a warning) when it is not understood.
func (p *parser) parseColor() string {
	model, _ := p.rawOptArg()
	spec := p.rawArg()
	hex, ok := colorHex(model, spec)
	if !ok {
		p.warn("unknown colour " + strconv.Quote(spec) + " ignored")
		return ""
	}
	return `rgb("#` + hex + `")`
}

func colored(col string, content *node) *node {
	if col == "" {
		return content
	}
	return call("text", content).with(code("fill", "#"+col))
}

// colorHex resolves an xcolor model/spec pair to RRGGBB.
func colorHex(model, spec string) (string, bool) {
	spec = strings.TrimSpace(spec)
	switch strings.ToLower(model) {
	case "html":
		s := strings.TrimPrefix(spec, "#")
		if len(s) == 6 && isHex(s) {
			return strings.ToUpper(s), true
		}
		return "", false
	case "rgb":
		v, ok := floats(spec, 3)
		if !ok {
			return "", false
		}
		return rgbHex(v[0]*255, v[1]*255, v[2]*255), true
	case "rgb255":
		v, ok := floats(spec, 3)
		if !ok {
			return "", false
		}
		return rgbHex(v[0], v[1], v[2]), true
	case "gray":
		v, ok := floats(spec, 1)
		if !ok {
			return "", false
		}
		return rgbHex(v[0]*255, v[0]*255, v[0]*255), true
	case "cmyk":
		v, ok := floats(spec, 4)
		if !ok {
			return "", false
		}
		k := 1 - v[3]
		return rgbHex(255*(1-v[0])*k, 255*(1-v[1])*k, 255*(1-v[2])*k), true
	case "":
		return mixColor(spec)
	}
	if model == "RGB" {
		v, ok := floats(spec, 3)
		if !ok {
			return "", false
		}
		return rgbHex(v[0], v[1], v[2]), true
	}
	return "", false
}

// mixColor resolves names and xcolor mixes such as "red!30!blue".
func mixColor(spec string) (string, bool) {
	if strings.HasPrefix(spec, "#") && len(spec) == 7 && isHex(spec[1:]) {
		return strings.ToUpper(spec[1:]), true
	}
	parts := strings.Split(spec, "!")
	if len(parts) > 16 {
		return "", false
	}
	cur, ok := lookupColor(parts[0])
	if !ok {
		return "", false
	}
	for i := 1; i < len(parts); i += 2 {
		pct, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if err != nil || pct < 0 || pct > 100 {
			return "", false
		}
		other := [3]float64{255, 255, 255}
		if i+1 < len(parts) {
			if other, ok = lookupColor(parts[i+1]); !ok {
				return "", false
			}
		}
		for c := range cur {
			cur[c] = cur[c]*pct/100 + other[c]*(100-pct)/100
		}
	}
	return rgbHex(cur[0], cur[1], cur[2]), true
}

func lookupColor(name string) ([3]float64, bool) {
	name = strings.TrimSpace(name)
	hex, ok := namedColors[name]
	if !ok {
		hex, ok = namedColors[strings.ToLower(name)]
	}
	if !ok {
		return [3]float64{}, false
	}
	var c [3]float64
	for i := range c {
		v, _ := strconv.ParseUint(hex[2*i:2*i+2], 16, 8)
		c[i] = float64(v)
	}
	return c, true
}

func floats(s string, n int) ([]float64, bool) {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
	if len(fields) != n {
		return nil, false
	}
	out := make([]float64, n)
	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil || v < 0 || math.IsInf(v, 0) {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

func rgbHex(r, g, b float64) string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 0, 6)
	for _, v := range [3]float64{r, g, b} {
		c := int(math.Round(math.Max(0, math.Min(255, v))))
		out = append(out, digits[c>>4], digits[c&15])
	}
	return string(out)
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i] | 0x20
		if !(s[i] >= '0' && s[i] <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
