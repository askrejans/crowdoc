package cite

import (
	"slices"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

const sampleRIS = "TY  - JOUR\r\n" +
	"AU  - Smith, John A.\r\n" +
	"AU  - Lee, Kim\r\n" +
	"TI  - Deep learning for crop\r\n" +
	"  yields in the Baltics\r\n" +
	"T2  - Journal of Field Crop Studies\r\n" +
	"JA  - J Agric Sci\r\n" +
	"PY  - 2020///\r\n" +
	"DA  - 2020/03/15/\r\n" +
	"VL  - 12\r\n" +
	"IS  - 3\r\n" +
	"SP  - 45\r\n" +
	"EP  - 67\r\n" +
	"DO  - 10.1000/xyz123\r\n" +
	"SN  - 1234-5678\r\n" +
	"UR  - https://example.org/a\r\n" +
	"ER  - \r\n" +
	"\r\n" +
	"TY  - CHAP\n" +
	"ID  - chap1\n" +
	"A1  - Bērziņš, Jānis\n" +
	"A2  - Ozola, Ilze\n" +
	"T1  - Grain storage\n" +
	"BT  - Handbook of agriculture\n" +
	"PB  - Dzīles\n" +
	"CY  - Rīga\n" +
	"SP  - 101-120\n" +
	"PY  - 2021\n" +
	"SN  - 978-9984-0-0000-0\n" +
	"ER  -\n" +
	"TY  - BOOK\n" +
	"AU  - Smith, John A.\n" +
	"TI  - Another 2020 work\n" +
	"PY  - 2020\n" +
	"TY  - RPRT\n" +
	"AU  - World Health Organization\n" +
	"TI  - Report\n" +
	"IS  - WHO-1\n" +
	"PY  - 2022\n" +
	"Y2  - 2023-01-02\n" +
	"ER  - \n" +
	"AU  - Stray, Tag\n"

func TestParseRIS(t *testing.T) {
	refs, warns := ParseRIS([]byte(sampleRIS))
	if len(refs) != 4 {
		t.Fatalf("got %d refs", len(refs))
	}
	var ids []string
	for _, r := range refs {
		ids = append(ids, r.ID)
	}
	if want := []string{"smith2020", "chap1", "smith2020b", "worldhealthorganization2022"}; !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	j := refs[0]
	check(t, "type", j.Type, "article-journal")
	check(t, "continued title", j.Title, "Deep learning for crop yields in the Baltics")
	check(t, "T2 wins over JA", j.ContainerTitle, "Journal of Field Crop Studies")
	check(t, "pages", j.Page, "45–67")
	check(t, "issn", j.ISSN, "1234-5678")
	check(t, "doi", j.DOI, "10.1000/xyz123")
	if j.Issued != (ast.Date{Year: 2020, Month: 3, Day: 15}) {
		t.Errorf("DA date = %+v", j.Issued)
	}
	if !slices.Equal(j.Author, []ast.Name{{Family: "Smith", Given: "John A."}, {Family: "Lee", Given: "Kim"}}) {
		t.Errorf("authors = %+v", j.Author)
	}
	c := refs[1]
	check(t, "chapter type", c.Type, "chapter")
	check(t, "BT container", c.ContainerTitle, "Handbook of agriculture")
	check(t, "isbn", c.ISBN, "978-9984-0-0000-0")
	check(t, "place", c.PublisherPlace, "Rīga")
	if len(c.Editor) != 1 || c.Editor[0].Family != "Ozola" {
		t.Errorf("editors = %+v", c.Editor)
	}
	r := refs[3]
	check(t, "report number", r.Number, "WHO-1")
	if len(r.Author) != 1 || r.Author[0].Literal != "World Health Organization" {
		t.Errorf("corporate author = %+v", r.Author)
	}
	if r.Accessed != (ast.Date{Year: 2023, Month: 1, Day: 2}) {
		t.Errorf("Y2 = %+v", r.Accessed)
	}
	joined := strings.Join(warns, "\n")
	if !strings.Contains(joined, "record without ER") || !strings.Contains(joined, "AU outside a record") {
		t.Errorf("warnings: %s", joined)
	}
}

const sampleNBIB = `PMID- 12345678
OWN - NLM
TI  - Effect of grain moisture on storage losses: a randomised
      trial.
LID - 10.1000/med.1 [doi]
AB  - Background text.
FAU - Smith, John Alan
AU  - Smith JA
FAU - van der Berg, Anna
AU  - van der Berg A
CN  - Grain Study Group
LA  - eng
PT  - Journal Article
DP  - 2019 Mar-Apr
TA  - J Grain Sci
JT  - Journal of grain science
VI  - 7
IP  - 2
PG  - 284-97
IS  - 1234-5678 (Electronic)

PMID- 23456789
TI  - Second record.
AU  - Ozola I
DP  - 2021 Jan 5
JT  - Latvian journal
PG  - e123
AID - S0000 [pii]
AID - 10.1000/med.2 [doi]
`

func TestParseNBIB(t *testing.T) {
	refs, warns := ParseNBIB([]byte(sampleNBIB))
	if len(refs) != 2 || len(warns) != 0 {
		t.Fatalf("refs=%d warns=%v", len(refs), warns)
	}
	a := refs[0]
	check(t, "id", a.ID, "smith2019")
	check(t, "joined title, trailing period removed", a.Title, "Effect of grain moisture on storage losses: a randomised trial")
	check(t, "journal", a.ContainerTitle, "Journal of grain science")
	check(t, "expanded pages", a.Page, "284–297")
	check(t, "doi", a.DOI, "10.1000/med.1")
	check(t, "issn", a.ISSN, "1234-5678")
	if a.Issued != (ast.Date{Year: 2019, Month: 3}) {
		t.Errorf("DP = %+v", a.Issued)
	}
	wantNames := []ast.Name{
		{Family: "Smith", Given: "John Alan"},
		{Family: "Berg", Particle: "van der", Given: "Anna"},
		{Literal: "Grain Study Group"},
	}
	if !slices.Equal(a.Author, wantNames) {
		t.Errorf("authors = %+v", a.Author)
	}
	b := refs[1]
	if b.Author[0] != (ast.Name{Family: "Ozola", Given: "I."}) {
		t.Errorf("AU-only name = %+v", b.Author[0])
	}
	check(t, "AID doi", b.DOI, "10.1000/med.2")
	if b.Issued != (ast.Date{Year: 2021, Month: 1, Day: 5}) {
		t.Errorf("DP day = %+v", b.Issued)
	}
	if got := formatOne("vancouver", "en", a); got != "Smith JA, van der Berg A, Grain Study Group. Effect of grain moisture on storage losses: a randomised trial. Journal of grain science. 2019 Mar;7(2):284-97. doi:[10.1000/med.1](https://doi.org/10.1000/med.1)" {
		t.Errorf("vancouver: %s", got)
	}
}

func TestParseDispatch(t *testing.T) {
	bib := []byte("@book{b, title={T}}")
	json := []byte(`[{"id": "j", "title": "T"}]`)
	ris := []byte("TY  - BOOK\nTI  - T\nER  - \n")
	nbib := []byte("PMID- 1\nTI  - T\n")
	yml := []byte("references:\n- id: y\n  title: T\n")
	tests := []struct {
		name string
		data []byte
		id   string
	}{
		{"refs.bib", bib, "b"}, {"refs.BibTeX", bib, "b"}, {"x.biblatex", bib, "b"},
		{"refs.json", json, "j"}, {"refs.yaml", yml, "y"}, {"refs.yml", yml, "y"},
		{"refs.ris", ris, "t"}, {"pubmed.nbib", nbib, "t"},
		{"unknown.txt", bib, "b"}, {"unknown", json, "j"}, {"export.dat", ris, "t"},
		{"medline.out", nbib, "t"}, {"noext", yml, "y"},
		{"bom.bib", append([]byte{0xEF, 0xBB, 0xBF}, bib...), "b"},
	}
	for _, tc := range tests {
		refs, _, err := Parse(tc.name, tc.data)
		if err != nil || len(refs) != 1 || refs[0].ID != tc.id {
			t.Errorf("Parse(%s) = %+v, %v", tc.name, refs, err)
		}
	}
	if _, _, err := Parse("x.txt", []byte("just some prose")); err == nil {
		t.Error("unrecognised content accepted")
	}
	if _, _, err := Parse("x.json", []byte("{broken")); err == nil {
		t.Error("broken JSON accepted")
	}
}

func TestParseDateString(t *testing.T) {
	tests := map[string]ast.Date{
		"2020":                {Year: 2020},
		"2020-05":             {Year: 2020, Month: 5},
		"2020-05-01":          {Year: 2020, Month: 5, Day: 1},
		"2020/05/01/":         {Year: 2020, Month: 5, Day: 1},
		"2020///":             {Year: 2020},
		"2020-05/2020-06":     {Year: 2020, Month: 5},
		"1998--1999":          {Year: 1998},
		"May 1, 2020":         {Year: 2020, Month: 5, Day: 1},
		"1 May 2020":          {Year: 2020, Month: 5, Day: 1},
		"2020 May 1":          {Year: 2020, Month: 5, Day: 1},
		"2020 Mar-Apr":        {Year: 2020, Month: 3},
		"Spring 2020":         {Year: 2020},
		"September 2019":      {Year: 2019, Month: 9},
		"Sept. 2019":          {Year: 2019, Month: 9},
		"05/01/2020":          {Year: 2020, Month: 5, Day: 1},
		"01.05.2020":          {Year: 2020, Month: 5, Day: 1},
		"2020. gada 1. maijs": {Year: 2020, Month: 5, Day: 1},
		"1 de mayo de 2020":   {Year: 2020, Month: 5, Day: 1},
		"c. 1850":             {Year: 1850, Circa: true},
		"1850~":               {Year: 1850, Circa: true},
		"[2020]":              {Year: 2020},
		"2020a":               {Year: 2020},
		"n.d.":                {},
		"":                    {},
		"forthcoming":         {Literal: "forthcoming"},
		"in press":            {Literal: "in press"},
		"2020-13-40":          {Year: 2020},
	}
	for in, want := range tests {
		if got := parseDateString(in); got != want {
			t.Errorf("parseDateString(%q) = %+v, want %+v", in, got, want)
		}
	}
}
