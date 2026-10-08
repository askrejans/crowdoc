package cite

import (
	"slices"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

const sampleBib = `
% A comment line with an e-mail me@example.org
@string{ jas = "Journal of Field Crop Studies" }
@STRING( lu = {University of Latvia} )
@preamble{ "\newcommand{\noopsort}[1]{}" }
@comment{ ignored @article{fake, title={x}} }

@Article{Smith2020,
  Author    = {Smith, John A. and Lee, Kim and others},
  title     = {{Deep} Learning for {CROP} Yields: a \emph{case} study},
  journal   = jas,
  year      = 2020,
  month     = mar,
  volume    = {12},
  number    = 3,
  pages     = {45--67},
  doi       = {https://doi.org/10.1000/xyz123},
}

@book{berzins,
  author = {B{\=e}rzi{\c{n}}{\v{s}}, J{\=a}nis and {\v S}ulca, {\c K}{\=a}rlis and {World Health Organization}},
  title = "Grauda glab{\=a}{\v{s}}ana -- " # "praktiskais ce{\c{l}}vedis",
  publisher = {Dz{\=\i}les}, address = {R{\=\i}ga},
  date = {2021-05/2021-06},
  edition = {2nd},
}

@inproceedings{vangogh,
  author = {van Gogh, Vincent and de la Fontaine, Jean and Ludwig van Beethoven and Smith, Jr., John and {\"O}zt{\"u}rk, Ayla},
  title = {Caf\'e {\'e}tudes na\"ive and Erd\H{o}s},
  crossref = {procs},
  pages = {1--10},
}

@proceedings{procs,
  title = {Proceedings of Things},
  year = {2019},
  editor = {Doe, Jane},
  publisher = {Example Press},
}

@misc{arxiv1,
  author = {Doe, Jane},
  title = {Preprint title},
  eprint = {2001.12345},
  archivePrefix = {arXiv},
  year = {2020},
}

@phdthesis{thesis1,
  author = {Liepa, Marta},
  title = {Soil},
  school = lu,
  year = {2018},
  url = {https://example.org/a\_b~c},
}

@article{broken,
  author = {Nobody},
  title = {Missing brace
}

@online{web1,
  title = {Math $\alpha$-helix and 50\% \& more \ldots},
  url = {\url{https://example.org/x_y}},
  urldate = {2021-05-01},
}

@techreport{rep1,
  author = {{Latvian Grain Institute}},
  title = {Annual report},
  institution = {Latvian Grain Institute},
  number = {LGI-7},
  type = {Technical Report},
  year = 2022,
}

@article{mag1, entrysubtype = {magazine}, title = {Mag}, journaltitle = {Weekly}, journalsubtitle = {News},
  author = {A, B}, date = {2020-04-03}}

@misc{web2, title = {Page}, howpublished = {\url{https://example.org/p}}, note = {Accessed 2020}}
`

func TestParseBibTeX(t *testing.T) {
	refs, warns := ParseBibTeX([]byte(sampleBib))
	byID := map[string]ast.Reference{}
	var ids []string
	for _, r := range refs {
		byID[r.ID] = r
		ids = append(ids, r.ID)
	}
	wantIDs := []string{"Smith2020", "berzins", "vangogh", "procs", "arxiv1", "thesis1", "web1", "rep1", "mag1", "web2"}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("ids = %v, want %v", ids, wantIDs)
	}
	if want := []string{"bibtex: line 58: malformed @article entry skipped: expected ',' or end of entry after field title"}; !slices.Equal(warns, want) {
		t.Errorf("warnings = %v", warns)
	}

	s := byID["Smith2020"]
	check(t, "smith type", s.Type, "article-journal")
	check(t, "smith title", s.Title, "Deep Learning for CROP Yields: a case study")
	check(t, "smith journal (macro)", s.ContainerTitle, "Journal of Field Crop Studies")
	check(t, "smith issue from number", s.Issue, "3")
	check(t, "smith pages", s.Page, "45–67")
	check(t, "smith doi", s.DOI, "10.1000/xyz123")
	if s.Issued != (ast.Date{Year: 2020, Month: 3}) {
		t.Errorf("smith issued = %+v", s.Issued)
	}
	if len(s.Author) != 3 || !isOthers(s.Author[2]) || s.Author[0] != (ast.Name{Family: "Smith", Given: "John A."}) {
		t.Errorf("smith authors = %+v", s.Author)
	}

	b := byID["berzins"]
	check(t, "latvian title with concatenation", b.Title, "Grauda glabāšana – praktiskais ceļvedis")
	check(t, "latvian publisher", b.Publisher, "Dzīles")
	check(t, "dotless i macron", b.PublisherPlace, "Rīga")
	wantNames := []ast.Name{{Family: "Bērziņš", Given: "Jānis"}, {Family: "Šulca", Given: "Ķārlis"}, {Literal: "World Health Organization"}}
	if !slices.Equal(b.Author, wantNames) {
		t.Errorf("latvian authors = %+v", b.Author)
	}
	if b.Issued != (ast.Date{Year: 2021, Month: 5}) {
		t.Errorf("date range start = %+v", b.Issued)
	}

	v := byID["vangogh"]
	check(t, "accents", v.Title, "Café études naïve and Erdős")
	check(t, "crossref booktitle", v.ContainerTitle, "Proceedings of Things")
	check(t, "crossref publisher", v.Publisher, "Example Press")
	if v.Issued.Year != 2019 || len(v.Editor) != 1 || v.Editor[0].Family != "Doe" {
		t.Errorf("crossref year/editor: %+v %+v", v.Issued, v.Editor)
	}
	wantNames = []ast.Name{
		{Family: "Gogh", Particle: "van", Given: "Vincent"},
		{Family: "Fontaine", Particle: "de la", Given: "Jean"},
		{Family: "Beethoven", Particle: "van", Given: "Ludwig"},
		{Family: "Smith", Given: "John", Suffix: "Jr."},
		{Family: "Öztürk", Given: "Ayla"},
	}
	if !slices.Equal(v.Author, wantNames) {
		t.Errorf("name grammar = %+v", v.Author)
	}

	a := byID["arxiv1"]
	check(t, "arxiv type", a.Type, "article")
	check(t, "arxiv url", a.URL, "https://arxiv.org/abs/2001.12345")
	check(t, "arxiv number", a.Number, "arXiv:2001.12345")
	check(t, "arxiv publisher", a.Publisher, "arXiv")

	th := byID["thesis1"]
	check(t, "thesis genre", th.Genre, "PhD thesis")
	check(t, "school macro as publisher", th.Publisher, "University of Latvia")
	check(t, "url escapes kept raw", th.URL, "https://example.org/a_b~c")

	w := byID["web1"]
	check(t, "online type", w.Type, "webpage")
	check(t, "math and escapes", w.Title, "Math α-helix and 50% & more …")
	check(t, "url wrapper", w.URL, "https://example.org/x_y")
	if w.Accessed != (ast.Date{Year: 2021, Month: 5, Day: 1}) {
		t.Errorf("urldate = %+v", w.Accessed)
	}

	r := byID["rep1"]
	check(t, "report number", r.Number, "LGI-7")
	check(t, "report genre", r.Genre, "Technical Report")
	check(t, "institution", r.Publisher, "Latvian Grain Institute")
	if len(r.Author) != 1 || r.Author[0].Literal != "Latvian Grain Institute" {
		t.Errorf("corporate author = %+v", r.Author)
	}

	m := byID["mag1"]
	check(t, "entrysubtype", m.Type, "article-magazine")
	check(t, "journal subtitle", m.ContainerTitle, "Weekly: News")
	if m.Issued != (ast.Date{Year: 2020, Month: 4, Day: 3}) {
		t.Errorf("iso date = %+v", m.Issued)
	}
	check(t, "howpublished url", byID["web2"].URL, "https://example.org/p")
	check(t, "note", byID["web2"].Note, "Accessed 2020")
}

func check(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", what, got, want)
	}
}

func TestBibTeXNames(t *testing.T) {
	tests := []struct {
		raw  string
		want []ast.Name
	}{
		{"Smith, John A.", []ast.Name{{Family: "Smith", Given: "John A."}}},
		{"John A. Smith", []ast.Name{{Family: "Smith", Given: "John A."}}},
		{"Jean-Paul Sartre", []ast.Name{{Family: "Sartre", Given: "Jean-Paul"}}},
		{"Vincent van Gogh", []ast.Name{{Family: "Gogh", Particle: "van", Given: "Vincent"}}},
		{"Jean de la Fontaine", []ast.Name{{Family: "Fontaine", Particle: "de la", Given: "Jean"}}},
		{"Van Gogh, Vincent", []ast.Name{{Family: "Van Gogh", Given: "Vincent"}}},
		{"{\\\"u}ber, Hans", []ast.Name{{Family: "über", Given: "Hans"}}},
		{"J{\\\"o}rg M{\\\"u}ller", []ast.Name{{Family: "Müller", Given: "Jörg"}}},
		{"Ivan {\\v{S}}ulc", []ast.Name{{Family: "Šulc", Given: "Ivan"}}},
		{"Aristotle", []ast.Name{{Family: "Aristotle"}}},
		{"{Smith and Sons, Ltd.}", []ast.Name{{Literal: "Smith and Sons, Ltd."}}},
		{"Smith, J. AND Lee, K.", []ast.Name{{Family: "Smith", Given: "J."}, {Family: "Lee", Given: "K."}}},
		{"Smith, J. and\n  others", []ast.Name{{Family: "Smith", Given: "J."}, {Literal: othersLiteral}}},
		{"Ch.~de~Gaulle", []ast.Name{{Family: "Gaulle", Particle: "de", Given: "Ch."}}},
		{"Candy, Jr., John and {European Commission}", []ast.Name{{Family: "Candy", Given: "John", Suffix: "Jr."}, {Literal: "European Commission"}}},
		{"", nil},
	}
	for _, tc := range tests {
		if got := parseBibNames(tc.raw); !slices.Equal(got, tc.want) {
			t.Errorf("parseBibNames(%q) = %+v, want %+v", tc.raw, got, tc.want)
		}
	}
}

func TestLatexToUnicode(t *testing.T) {
	tests := map[string]string{
		`{\"o}`:                                  "ö",
		`\"{o}`:                                  "ö",
		`\"o`:                                    "ö",
		`\'{\i}`:                                 "í",
		`\'\i`:                                   "í",
		`\={a}\=e{\=\i}\={u}`:                    "āēīū",
		`\v{s}\v{c}\v{z}\v S`:                    "ščžŠ",
		`\c{k}\c{g}\c{l}\c{n}\c K`:               "ķģļņĶ",
		`\c{c}\k{a}\r{u}\H{o}\u{a}`:              "çąůőă",
		`\d{s}\b{t}\.{z}\~n\^e`:                  "ṣṯżñê",
		`\aa\AA\ae\AE\oe\OE\o\O`:                 "åÅæÆœŒøØ",
		`\l\L\ss{}`:                              "łŁß",
		`\& \% \$ \# \_ \{ \}`:                   "& % $ # _ { }",
		`a~b`:                                    "a\u00a0b",
		`1--2---3`:                               "1–2—3",
		"``quoted''":                             "“quoted”",
		`\textit{x} \emph{y} \textbf{z} {\em w}`: "x y z w",
		`$E=mc^2$ and $\alpha \leq \beta$`:       "E=mc^2 and α ≤ β",
		`{{Nested} {Braces}}`:                    "Nested Braces",
		`\href{https://x.org}{link text}`:        "link text",
		`\enquote{q}`:                            "“q”",
		`multi   line
		 text`: "multi line text",
		`\LaTeX\ and \TeX`:  "LaTeX and TeX",
		`\unknown{kept}`:    "kept",
		`\noopsort{a}Zebra`: "Zebra",
		`plain`:             "plain",
		`trailing \`:        "trailing",
	}
	for in, want := range tests {
		if got := latexToUnicode(in); got != want {
			t.Errorf("latexToUnicode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBibTeXTypes(t *testing.T) {
	tests := map[string]string{
		"article": "article-journal", "book": "book", "booklet": "pamphlet", "inbook": "chapter",
		"incollection": "chapter", "inproceedings": "paper-conference", "conference": "paper-conference",
		"manual": "book", "mastersthesis": "thesis", "phdthesis": "thesis", "thesis": "thesis",
		"misc": "document", "online": "webpage", "electronic": "webpage", "www": "webpage",
		"techreport": "report", "report": "report", "unpublished": "manuscript", "proceedings": "book",
		"collection": "book", "patent": "patent", "dataset": "dataset", "software": "software",
		"standard": "standard", "artwork": "document",
	}
	for typ, want := range tests {
		refs, _ := ParseBibTeX([]byte("@" + typ + "{k, title={T}}"))
		if len(refs) != 1 || refs[0].Type != want {
			t.Errorf("@%s → %+v, want type %s", typ, refs, want)
		}
	}
	refs, _ := ParseBibTeX([]byte(`@mastersthesis{m, title={T}, school={U}}
@thesis{t, title={T}, type={mathesis}}
@thesis{u, title={T}, type={Habilitationsschrift}}`))
	for i, want := range []string{"Master's thesis", "Master's thesis", "Habilitationsschrift"} {
		if refs[i].Genre != want {
			t.Errorf("thesis %d genre = %q, want %q", i, refs[i].Genre, want)
		}
	}
}

func TestBibTeXMalformed(t *testing.T) {
	src := `@article{ , title = {No key}}
@article{nokey title = {x}}
@book{good1, title = {Fine}, year = 2001}
@article{noeq, title {x}}
@article{badmacro, title = undefinedmacro # { tail}, year = 2002}
@book{good1, title = {Duplicate}}
@book{good2, title = {Unterminated`
	refs, warns := ParseBibTeX([]byte(src))
	var ids []string
	for _, r := range refs {
		ids = append(ids, r.ID)
	}
	if want := []string{"good1", "badmacro"}; !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	joined := strings.Join(warns, "\n")
	for _, want := range []string{"missing citation key", "expected '=' after field title", "undefined @string macro \"undefinedmacro\"", "duplicate key good1", "line 7"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings lack %q:\n%s", want, joined)
		}
	}
	if refs[1].Title != "tail" {
		t.Errorf("value after undefined macro = %q", refs[1].Title)
	}
	if r, w := ParseBibTeX(nil); len(r) != 0 || len(w) != 0 {
		t.Errorf("empty input: %v %v", r, w)
	}
	// Garbage never panics.
	for _, junk := range []string{"@", "@{", "@article", "@article{", "@article{k,", "@article{k, t = {", "@string{x = }", "}}}{{{@@@"} {
		ParseBibTeX([]byte(junk))
	}
}

func TestBibTeXDates(t *testing.T) {
	src := `@book{a, title={T}, year={2019}, month={7}}
@book{b, title={T}, year={forthcoming}}
@book{c, title={T}, date={1850~}}
@book{d, title={T}, year={1998--1999}}
@book{e, title={T}, year={2020}, month={September}, day={9}}
@book{f, title={T}, date={2020-02-30}}`
	refs, _ := ParseBibTeX([]byte(src))
	want := []ast.Date{
		{Year: 2019, Month: 7},
		{Literal: "forthcoming"},
		{Year: 1850, Circa: true},
		{Year: 1998},
		{Year: 2020, Month: 9, Day: 9},
		{Year: 2020, Month: 2, Day: 30},
	}
	for i, r := range refs {
		if r.Issued != want[i] {
			t.Errorf("%s issued = %+v, want %+v", r.ID, r.Issued, want[i])
		}
	}
}

// TestBibTeXToEntry runs a parsed entry through formatting: accents and
// "and others" must come out right in a finished reference.
func TestBibTeXToEntry(t *testing.T) {
	refs, _ := ParseBibTeX([]byte(`@article{k,
  author = {B{\=e}rzi{\c{n}}{\v{s}}, J{\=a}nis and others},
  title = {Gra{\v{s}}u {\c{k}}{\=\i}mija},
  journal = {Latvijas Zin{\=a}tņu Vēstis}, year = 2020, volume = 3, pages = {1--9}}`))
	if len(refs) != 1 {
		t.Fatal(refs)
	}
	if got := formatOne("apa", "lv", refs[0]); got != "Bērziņš, J., u. c. (2020). Grašu ķīmija. _Latvijas Zinātņu Vēstis_, _3_, 1–9." {
		t.Errorf("got %s", got)
	}
}
