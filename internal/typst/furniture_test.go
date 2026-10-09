package typst

import (
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

func TestOptionalTitleHeaderFooter(t *testing.T) {
	on, off := true, false
	minimal, _ := LookupStyle("minimal")
	report, _ := LookupStyle("report")
	if minimal.HasHeader() || !minimal.HasFooter() || !report.HasHeader() || !report.HasFooter() {
		t.Fatalf("furniture detection: minimal %v/%v report %v/%v", minimal.HasHeader(), minimal.HasFooter(), report.HasHeader(), report.HasFooter())
	}
	cases := []struct {
		name           string
		st             *Style
		meta           ast.Meta
		title          bool
		header, footer string
	}{
		{"real title shown", minimal, ast.Meta{Title: "Notes"}, true, "style", "style"},
		{"file-name title hidden", minimal, ast.Meta{Title: "Scan", TitleFromName: true}, false, "style", "style"},
		{"file-name title on a title page", report, ast.Meta{Title: "Scan", TitleFromName: true}, true, "style", "style"},
		{"forced title", minimal, ast.Meta{Title: "Scan", TitleFromName: true, ShowTitle: &on}, true, "style", "style"},
		{"hidden real title", minimal, ast.Meta{Title: "Notes", ShowTitle: &off}, false, "style", "style"},
		{"header added, footer removed", minimal, ast.Meta{Title: "Notes", ShowHeader: &on, ShowFooter: &off}, true, "add", "off"},
		{"designed header kept when forced", report, ast.Meta{Title: "R", ShowHeader: &on}, true, "style", "style"},
		{"designed header removed", report, ast.Meta{Title: "R", ShowHeader: &off}, true, "off", "style"},
	}
	for _, c := range cases {
		s := resolveSettings(&ast.Document{Meta: c.meta}, c.st)
		if s.ShowTitle != c.title || s.HeaderMode != c.header || s.FooterMode != c.footer {
			t.Errorf("%s: got title=%v header=%s footer=%s", c.name, s.ShowTitle, s.HeaderMode, s.FooterMode)
		}
	}
}
