package cite

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/askrejans/crowdoc/v2/ast"
)

const sampleCSLJSON = `[
  {
    "id": "smith2020",
    "type": "article-journal",
    "title": "Deep learning for <i>crop</i> yields",
    "container-title": ["Journal of Field Crop Studies", "J Agric Sci"],
    "volume": 12,
    "issue": "3",
    "page": "45-67",
    "DOI": "https://doi.org/10.1000/XYZ123",
    "author": [
      {"family": "Smith", "given": "John A."},
      {"family": "Gogh", "given": "Vincent", "non-dropping-particle": "van"},
      {"literal": "World Health Organization"},
      "Lee, Kim"
    ],
    "issued": {"date-parts": [["2020", "3", 15]]},
    "accessed": {"raw": "2021-05-01"},
    "URL": "https://example.org/a"
  },
  {
    "id": 42,
    "type": "book",
    "title": "Numeric id",
    "editor": [{"family": "Ozola", "given": "Ilze"}],
    "issued": {"literal": "forthcoming"},
    "publisher-place": "Rīga",
    "edition": 2
  },
  {
    "type": "chapter",
    "title": "No id here",
    "author": [{"family": "Bērziņš", "given": "Jānis"}],
    "issued": {"date-parts": [[2019]], "circa": true},
    "event-title": "Grain Days",
    "collection-title": "Series",
    "collection-number": 7
  },
  {"id": "smith2020", "title": "Duplicate"},
  "not an object"
]`

func TestParseCSLJSON(t *testing.T) {
	refs, warns, err := ParseCSLJSON([]byte(sampleCSLJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 3 {
		t.Fatalf("got %d refs", len(refs))
	}
	s := refs[0]
	check(t, "rich text stripped", s.Title, "Deep learning for crop yields")
	check(t, "container from list", s.ContainerTitle, "Journal of Field Crop Studies")
	check(t, "numeric volume", s.Volume, "12")
	check(t, "page range", s.Page, "45–67")
	check(t, "doi prefix stripped", s.DOI, "10.1000/XYZ123")
	check(t, "upper-case URL key", s.URL, "https://example.org/a")
	wantNames := []ast.Name{
		{Family: "Smith", Given: "John A."},
		{Family: "Gogh", Given: "Vincent", Particle: "van"},
		{Literal: "World Health Organization"},
		{Family: "Lee", Given: "Kim"},
	}
	if !slices.Equal(s.Author, wantNames) {
		t.Errorf("names = %+v", s.Author)
	}
	if s.Issued != (ast.Date{Year: 2020, Month: 3, Day: 15}) || s.Accessed != (ast.Date{Year: 2021, Month: 5, Day: 1}) {
		t.Errorf("dates = %+v %+v", s.Issued, s.Accessed)
	}

	b := refs[1]
	check(t, "numeric id", b.ID, "42")
	check(t, "numeric edition", b.Edition, "2")
	if b.Issued != (ast.Date{Literal: "forthcoming"}) || len(b.Editor) != 1 {
		t.Errorf("book = %+v", b)
	}

	c := refs[2]
	check(t, "generated id", c.ID, "berzins2019")
	check(t, "event", c.Event, "Grain Days")
	check(t, "collection number", c.CollectionTitle, "Series 7")
	if c.Issued != (ast.Date{Year: 2019, Circa: true}) {
		t.Errorf("circa = %+v", c.Issued)
	}
	joined := strings.Join(warns, "\n")
	for _, want := range []string{`item 3 has no id; using "berzins2019"`, `duplicate id "smith2020"`, "item 5 is not an object"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings lack %q:\n%s", want, joined)
		}
	}
}

func TestParseCSLJSONContainers(t *testing.T) {
	for _, src := range []string{
		`{"items": [{"id": "a", "title": "A"}]}`,
		`{"references": [{"id": "a", "title": "A"}]}`,
		`{"id": "a", "title": "A"}`,
	} {
		refs, _, err := ParseCSLJSON([]byte(src))
		if err != nil || len(refs) != 1 || refs[0].ID != "a" {
			t.Errorf("%s → %+v, %v", src, refs, err)
		}
	}
	for _, bad := range []string{`[{"id": "a"`, `"text"`, `42`, ``} {
		if _, _, err := ParseCSLJSON([]byte(bad)); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestParseCSLYAML(t *testing.T) {
	src := `---
references:
- id: ozols2021
  type: webpage
  title: Graudu glabāšana
  author:
    - family: Ozols
      given: Pēteris
  issued: 2021-05-01
  accessed:
    date-parts:
      - [2022, 1, 2]
  URL: https://example.org/x
- id: anon
  type: report
  title: Report
  issued: 2019
  number: R-1
...
`
	refs, warns, err := ParseCSLYAML([]byte(src))
	if err != nil || len(warns) != 0 {
		t.Fatalf("err=%v warns=%v", err, warns)
	}
	if len(refs) != 2 {
		t.Fatalf("got %d refs", len(refs))
	}
	if refs[0].Issued != (ast.Date{Year: 2021, Month: 5, Day: 1}) {
		t.Errorf("YAML timestamp = %+v", refs[0].Issued)
	}
	if refs[0].Accessed != (ast.Date{Year: 2022, Month: 1, Day: 2}) {
		t.Errorf("accessed = %+v", refs[0].Accessed)
	}
	check(t, "yaml title", refs[0].Title, "Graudu glabāšana")
	if refs[1].Issued != (ast.Date{Year: 2019}) || refs[1].Number != "R-1" {
		t.Errorf("second = %+v", refs[1])
	}

	list, _, err := ParseCSLYAML([]byte("- id: x\n  title: X\n"))
	if err != nil || len(list) != 1 {
		t.Errorf("bare list: %+v %v", list, err)
	}
	if _, _, err := ParseCSLYAML([]byte("references: [unclosed")); err == nil {
		t.Error("invalid YAML accepted")
	}
}

// TestFromCSLValue accepts frontmatter values decoded by either decoder.
func TestFromCSLValue(t *testing.T) {
	var fromYAML any
	if err := yaml.Unmarshal([]byte("- id: a\n  title: A\n  author: Smith, John; Lee, Kim\n  issued: {raw: May 2020}\n"), &fromYAML); err != nil {
		t.Fatal(err)
	}
	var fromJSON any
	if err := json.Unmarshal([]byte(`[{"id": "b", "title": "B", "issued": 2018}]`), &fromJSON); err != nil {
		t.Fatal(err)
	}
	legacy := []any{map[any]any{"id": "c", "title": "C", "author": []any{map[any]any{"family": "Doe"}}}}
	for _, tc := range []struct {
		v    any
		id   string
		date ast.Date
	}{
		{fromYAML, "a", ast.Date{Year: 2020, Month: 5}},
		{fromJSON, "b", ast.Date{Year: 2018}},
		{legacy, "c", ast.Date{}},
	} {
		refs, warns := FromCSLValue(tc.v)
		if len(refs) != 1 || refs[0].ID != tc.id || refs[0].Issued != tc.date || len(warns) != 0 {
			t.Errorf("%s: %+v %v", tc.id, refs, warns)
		}
	}
	refs, _ := FromCSLValue(fromYAML)
	if want := []ast.Name{{Family: "Smith", Given: "John"}, {Family: "Lee", Given: "Kim"}}; !slices.Equal(refs[0].Author, want) {
		t.Errorf("string names = %+v", refs[0].Author)
	}
	if refs, warns := FromCSLValue(nil); refs == nil && warns != nil {
		t.Errorf("nil value: %v %v", refs, warns)
	}
}

func TestNormType(t *testing.T) {
	for in, want := range map[string]string{
		"article-journal": "article-journal", "journal-article": "article-journal", "Journal Article": "article-journal",
		"book-chapter": "chapter", "proceedings-article": "paper-conference", "dissertation": "thesis",
		"legal_case": "legal_case", "legal-case": "legal_case", "motion picture": "motion_picture",
		"posted-content": "article", "": "document", "weird": "document", "WEBPAGE": "webpage",
	} {
		if got := normType(in); got != want {
			t.Errorf("normType(%q) = %q, want %q", in, got, want)
		}
	}
}
