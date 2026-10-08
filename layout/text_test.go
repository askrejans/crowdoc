package layout

import (
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

func TestParseMarker(t *testing.T) {
	tests := []struct {
		in      string
		ok      bool
		ordered bool
		num     int
		style   ast.NumberStyle
		task    ast.TaskState
	}{
		{"•", true, false, 0, 0, 0},
		{"–", true, false, 0, 0, 0},
		{"", true, false, 0, 0, 0}, // symbol font bullet in the private use area
		{"1.", true, true, 1, ast.NumberDecimal, 0},
		{"12)", true, true, 12, ast.NumberDecimal, 0},
		{"(3)", true, true, 3, ast.NumberDecimal, 0},
		{"b)", true, true, 2, ast.NumberLowerAlpha, 0},
		{"C.", true, true, 3, ast.NumberUpperAlpha, 0},
		{"iv.", true, true, 4, ast.NumberLowerRoman, 0},
		{"XII.", true, true, 12, ast.NumberUpperRoman, 0},
		{"☐", true, false, 0, 0, ast.TaskOpen},
		{"☑", true, false, 0, 0, ast.TaskDone},
		{"1990", false, false, 0, 0, 0},
		{"word", false, false, 0, 0, 0},
		{"(a", false, false, 0, 0, 0},
		{"1234.", false, false, 0, 0, 0},
	}
	for _, tt := range tests {
		m, ok := parseMarker(tt.in)
		if ok != tt.ok || ok && (m.ordered != tt.ordered || m.num != tt.num || ast.NumberStyle(m.style) != tt.style || m.task != tt.task) {
			t.Errorf("parseMarker(%q) = %+v, %v", tt.in, m, ok)
		}
	}
}

func TestDehyphenate(t *testing.T) {
	d := &doc{freq: map[string]int{"data-driven": 2, "realtime": 1}}
	tests := []struct {
		prefix, suffix string
		join           bool
	}{
		{"infor-", "mation", true},
		{"ražī-", "bu", true},      // Latvian
		{"data-", "driven", false}, // the document spells it with a hyphen
		{"real-", "time", true},    // ...and this one without
		{"self-", "contained", false},
		{"COVID-", "19", false},
		{"state-of-", "the-art", false},
		{"x-", "axis", false},     // a single letter before the hyphen
		{"well-", "Known", false}, // a capital after the break
	}
	for _, tt := range tests {
		if got := d.dehyphenate(tt.prefix, tt.suffix); got != tt.join {
			t.Errorf("dehyphenate(%q, %q) = %v", tt.prefix, tt.suffix, got)
		}
	}
}

func TestIsDate(t *testing.T) {
	for s, want := range map[string]bool{
		"15 January 2026":        true,
		"January 15, 2026":       true,
		"Jan. 2026":              true,
		"2026-04-12":             true,
		"12.04.2026":             true,
		"2026. gada 12. aprīlis": true,
		"12. maijs 2025":         true,
		"3. März 2024":           true,
		"Monday morning":         false,
		"15 January 2026 at 10":  false,
		"Section 2026":           false,
	} {
		if got := isDate(s); got != want {
			t.Errorf("isDate(%q) = %v", s, got)
		}
	}
}

func TestIsPageNumber(t *testing.T) {
	for s, want := range map[string]bool{
		"7": true, "- 12 -": true, "Page 3 of 9": true, "iv": true, "3. lpp.": true, "Lappuse 4": true,
		"1234": false, "Chapter": false, "IV. Results": false, "2026 Annual Report": false,
	} {
		if got := isPageNumber(s); got != want {
			t.Errorf("isPageNumber(%q) = %v", s, got)
		}
	}
}

func TestContentsEntries(t *testing.T) {
	for s, want := range map[string]bool{
		"1 Introduction . . . . . . . . . . 3": true,
		"2.1 Methods .................. 12":    true,
		"Appendix A ______________ iv":         true,
		"The price is 3.50... per item":        false,
		"Results were good . . . we think so.": false,
	} {
		if got := tocEntryRe.MatchString(s); got != want {
			t.Errorf("toc entry %q = %v", s, got)
		}
	}
}

func TestMathText(t *testing.T) {
	for in, want := range map[string]struct {
		text         string
		italic, bold bool
	}{
		"𝐻":   {"H", true, false},
		"𝑝𝑖":  {"pi", true, false},
		"𝛽":   {"β", true, false},
		"𝐀𝐁":  {"AB", false, true},
		"𝟏𝟐":  {"12", false, true},
		"ℎ":   {"h", true, false},
		"x+y": {"x+y", false, false},
	} {
		text, it, bd := mathText(in)
		if text != want.text || it != want.italic || bd != want.bold {
			t.Errorf("mathText(%q) = %q, %v, %v", in, text, it, bd)
		}
	}
}

func TestInkRatio(t *testing.T) {
	for in, want := range map[string]float64{
		"or": 0.5, "the": 0.72, "gap": 0.72, "Typography": 0.94, "ķē": 0.87, "—": 0,
	} {
		if got := inkRatio(in); got < want-0.01 || got > want+0.01 {
			t.Errorf("inkRatio(%q) = %v, want %v", in, got, want)
		}
	}
}
