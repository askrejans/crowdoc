package table

import "testing"

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		v        float64
		code     string
		date1904 bool
		want     string
		kind     cellKind
	}{
		{0.1 + 0.2, "General", false, "0.3", kNumber},
		{1234567.891, "General", false, "1234567.891", kNumber},
		{-42, "General", false, "-42", kNumber},
		{1e20, "General", false, "1E+20", kNumber},
		{2.675, "0.00", false, "2.68", kNumber},
		{1234.5, "#,##0.00", false, "1,234.50", kNumber},
		{-1234.5, "#,##0.00", false, "-1,234.50", kNumber},
		{1234.5, "#,##0", false, "1,235", kNumber},
		{0.125, "0.0%", false, "12.5%", kNumber},
		{0.125, "0%", false, "13%", kNumber},
		{1300.25, `"€"#,##0.00`, false, "€1,300.25", kNumber},
		{1300.25, `[$€-426]\ #,##0.00`, false, "€ 1,300.25", kNumber},
		{1234.56, `_-* #,##0.00\ "€"_-;\-* #,##0.00\ "€"_-;_-* "-"??\ "€"_-;_-@_-`, false, "1,234.56 €", kNumber},
		{-1234.56, `_-* #,##0.00\ "€"_-;\-* #,##0.00\ "€"_-;_-* "-"??\ "€"_-;_-@_-`, false, "-1,234.56 €", kNumber},
		{-5, "#,##0.00;(#,##0.00)", false, "(5.00)", kNumber},
		{-5, `0.00_);[Red]\(0.00\)`, false, "(5.00)", kNumber},
		{0, `0;-0;"zero"`, false, "zero", kNumber},
		{12345, "0.00E+00", false, "1.23E+04", kNumber},
		{0.5, "#.##", false, ".5", kNumber},
		{7, "000", false, "007", kNumber},
		{1500000, "#,##0,,\"M\"", false, "2M", kNumber},
		{1.5, "# ?/?", false, "1.5", kNumber},
		{45306, "yyyy-mm-dd", false, "2024-01-15", kDate},
		{45306, "dd/mm/yyyy", false, "2024-01-15", kDate},
		{45306, "[$-F800]dddd, mmmm dd, yyyy", false, "2024-01-15", kDate},
		{45306.604166666664, "m/d/yy h:mm", false, "2024-01-15 14:30", kDate},
		{45306.5, "yyyy-mm-dd", false, "2024-01-15 12:00", kDate},
		{0.604166666664, "h:mm", false, "14:30", kDate},
		{0.6041782407407408, "h:mm:ss", false, "14:30:01", kDate},
		{1.5, "[h]:mm:ss", false, "36:00:00", kDate},
		{0.0208333, "mm:ss", false, "30:00", kDate},
		{1, "yyyy-mm-dd", false, "1900-01-01", kDate},
		{0, "yyyy-mm-dd", true, "1904-01-01", kDate},
		{45306, `0 "days"`, false, "45306 days", kNumber},
		{45306, `#,##0 "m²"`, false, "45,306 m²", kNumber},
	}
	for _, tc := range tests {
		got, kind := formatNumber(tc.v, tc.code, tc.date1904)
		if got != tc.want || kind != tc.kind {
			t.Errorf("formatNumber(%v, %q) = %q (%d), want %q (%d)", tc.v, tc.code, got, kind, tc.want, tc.kind)
		}
	}
}

func TestClassifyFormat(t *testing.T) {
	tests := []struct {
		code string
		want dateParts
	}{
		{"General", dateParts{}},
		{"0.00E+00", dateParts{}},
		{`#,##0 "days"`, dateParts{}},
		{"mmm-yy", dateParts{date: true}},
		{"h:mm", dateParts{time: true, hours: true}},
		{"mm:ss", dateParts{time: true, seconds: true}},
		{"[h]:mm", dateParts{time: true, hours: true, duration: true}},
		{"yyyy-mm-dd hh:mm:ss", dateParts{date: true, time: true, hours: true, seconds: true}},
		{"[Red]0.00", dateParts{}},
		{`\d0`, dateParts{}},
	}
	for _, tc := range tests {
		if got := classifyFormat(tc.code); got != tc.want {
			t.Errorf("classifyFormat(%q) = %+v, want %+v", tc.code, got, tc.want)
		}
	}
}

func TestISODates(t *testing.T) {
	tests := map[string]string{
		"2024-03-05":           "2024-03-05",
		"2024-03-05T00:00:00":  "2024-03-05",
		"2024-03-05T10:30:00":  "2024-03-05 10:30",
		"2024-03-05T10:30:00Z": "2024-03-05 10:30",
		"garbage":              "garbage",
	}
	for in, want := range tests {
		if got := isoDateTime(in); got != want {
			t.Errorf("isoDateTime(%q) = %q, want %q", in, got, want)
		}
	}
	if got := isoDuration("PT10H05M30S"); got != "10:05:30" {
		t.Errorf("isoDuration = %q", got)
	}
	if got := isoDuration("PT08H00M00S"); got != "8:00" {
		t.Errorf("isoDuration = %q", got)
	}
}
