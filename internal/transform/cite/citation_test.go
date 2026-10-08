package cite

import (
	"slices"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

func citeRefs() []ast.Reference {
	return []ast.Reference{
		{ID: "smith2019", Type: "book", Author: []ast.Name{name("Smith", "John")}, Title: "Alpha", Issued: date(2019, 0, 0), Publisher: "P"},
		{ID: "smith2020x", Type: "article-journal", Author: []ast.Name{name("Smith", "John")}, Title: "Gamma", ContainerTitle: "J", Issued: date(2020, 0, 0)},
		{ID: "smith2020y", Type: "article-journal", Author: []ast.Name{name("Smith", "John")}, Title: "Beta", ContainerTitle: "J", Issued: date(2020, 0, 0)},
		{ID: "adams", Type: "book", Author: []ast.Name{name("Adams", "Ann"), name("Brown", "Bob"), name("Clark", "Carl")}, Title: "Three", Issued: date(2018, 0, 0)},
		{ID: "four", Type: "book", Author: []ast.Name{name("Doe", "Dan"), name("Eve", "Eva"), name("Fox", "Fay"), name("Gray", "Gus")}, Title: "Four", Issued: date(2017, 0, 0)},
		{ID: "two", Type: "book", Author: []ast.Name{name("Ozola", "Ilze"), name("Kalniņš", "Pēteris")}, Title: "Two", Issued: date(2016, 0, 0)},
		{ID: "anonart", Type: "article-journal", Title: "Crop rotation revisited: a review", ContainerTitle: "J", Issued: date(2019, 0, 0)},
		{ID: "anonbook", Type: "book", Title: "Farming atlas of Europe and beyond", Issued: date(2015, 0, 0)},
		{ID: "nd1", Type: "webpage", Author: []ast.Name{name("Ozols", "Pēteris")}, Title: "Zeta page"},
		{ID: "nd2", Type: "webpage", Author: []ast.Name{name("Ozols", "Pēteris")}, Title: "Alpha page"},
		{ID: "lee1", Type: "book", Author: []ast.Name{name("Lee", "Kim"), name("Park", "Min"), name("Choi", "Ana")}, Title: "L1", Issued: date(2021, 0, 0)},
		{ID: "lee2", Type: "book", Author: []ast.Name{name("Lee", "Kim"), name("Jung", "Bo"), name("Han", "Yu")}, Title: "L2", Issued: date(2021, 0, 0)},
		{ID: "asmith", Type: "book", Author: []ast.Name{name("Smith", "Alice")}, Title: "Other Smith", Issued: date(2010, 0, 0)},
	}
}

func cite(mode ast.CiteMode, items ...ast.CiteItem) *ast.Cite {
	return &ast.Cite{Items: items, Mode: mode}
}

func keys(ks ...string) []ast.CiteItem {
	out := make([]ast.CiteItem, len(ks))
	for i, k := range ks {
		out[i] = ast.CiteItem{Key: k}
	}
	return out
}

func para(ins ...ast.Inline) *ast.Para { return &ast.Para{Inlines: ins} }

// TestInTextCitations covers grouping of works by the same author, year
// suffixes, et al. thresholds, anonymous works, n.d. disambiguation,
// added names, initials for different authors sharing a family name,
// locators, suffixes and prefixes — parenthetical and narrative.
func TestInTextCitations(t *testing.T) {
	items := [][]ast.CiteItem{
		keys("smith2019", "smith2020x", "smith2020y"),
		keys("adams"),
		keys("four"),
		keys("two"),
		{{Key: "anonart", Locator: "4"}},
		keys("anonbook"),
		keys("nd1", "nd2"),
		keys("lee1", "lee2"),
		keys("asmith"),
		{{Key: "two", Locator: "12-14", Suffix: "emphasis added"}},
		{{Key: "two", Locator: "3", LocatorLabel: "chapter", Prefix: "see also"}},
	}
	want := map[string][][2]string{
		"apa": {
			{"(J. Smith, 2019, 2020a, 2020b)", "J. Smith (2019, 2020a, 2020b)"},
			{"(Adams et al., 2018)", "Adams et al. (2018)"},
			{"(Doe et al., 2017)", "Doe et al. (2017)"},
			{"(Ozola & Kalniņš, 2016)", "Ozola and Kalniņš (2016)"},
			{"(“Crop rotation revisited,” 2019, p. 4)", "“Crop rotation revisited” (2019, p. 4)"},
			{"(_Farming atlas of Europe_, 2015)", "_Farming atlas of Europe_ (2015)"},
			{"(Ozols, n.d.-a, n.d.-b)", "Ozols (n.d.-a, n.d.-b)"},
			{"(Lee, Jung, et al., 2021; Lee, Park, et al., 2021)", "Lee, Park, et al. (2021); Lee, Jung, et al. (2021)"},
			{"(A. Smith, 2010)", "A. Smith (2010)"},
			{"(Ozola & Kalniņš, 2016, pp. 12–14, emphasis added)", "Ozola and Kalniņš (2016, pp. 12–14, emphasis added)"},
			{"(see also Ozola & Kalniņš, 2016, Chapter 3)", "see also Ozola and Kalniņš (2016, Chapter 3)"},
		},
		"chicago": {
			{"(J. Smith 2019, 2020a, 2020b)", "J. Smith (2019, 2020a, 2020b)"},
			{"(Adams, Brown, and Clark 2018)", "Adams, Brown, and Clark (2018)"},
			{"(Doe et al. 2017)", "Doe et al. (2017)"},
			{"(Ozola and Kalniņš 2016)", "Ozola and Kalniņš (2016)"},
			{"(“Crop rotation revisited” 2019, 4)", "“Crop rotation revisited” (2019, 4)"},
			{"(_Farming atlas of Europe_ 2015)", "_Farming atlas of Europe_ (2015)"},
			{"(Ozols n.d.-a, n.d.-b)", "Ozols (n.d.-a, n.d.-b)"},
			{"(Lee, Park, and Choi 2021; Lee, Jung, and Han 2021)", "Lee, Park, and Choi (2021); Lee, Jung, and Han (2021)"},
			{"(A. Smith 2010)", "A. Smith (2010)"},
			{"(Ozola and Kalniņš 2016, 12–14, emphasis added)", "Ozola and Kalniņš (2016, 12–14, emphasis added)"},
			{"(see also Ozola and Kalniņš 2016, chap. 3)", "see also Ozola and Kalniņš (2016, chap. 3)"},
		},
		"harvard": {
			{"(J. Smith, 2019, 2020a, 2020b)", "J. Smith (2019, 2020a, 2020b)"},
			{"(Adams, Brown and Clark, 2018)", "Adams, Brown and Clark (2018)"},
			{"(Doe et al., 2017)", "Doe et al. (2017)"},
			{"(Ozola and Kalniņš, 2016)", "Ozola and Kalniņš (2016)"},
			{"(‘Crop rotation revisited’, 2019, p. 4)", "‘Crop rotation revisited’ (2019, p. 4)"},
			{"(_Farming atlas of Europe_, 2015)", "_Farming atlas of Europe_ (2015)"},
			{"(Ozols, no date a, no date b)", "Ozols (no date a, no date b)"},
			{"(Lee, Park and Choi, 2021; Lee, Jung and Han, 2021)", "Lee, Park and Choi (2021); Lee, Jung and Han (2021)"},
			{"(A. Smith, 2010)", "A. Smith (2010)"},
			{"(Ozola and Kalniņš, 2016, pp. 12–14, emphasis added)", "Ozola and Kalniņš (2016, pp. 12–14, emphasis added)"},
			{"(see also Ozola and Kalniņš, 2016, chap. 3)", "see also Ozola and Kalniņš (2016, chap. 3)"},
		},
		"ieee": {
			{"[1]–[3]", "Smith [1]; Smith [2]; Smith [3]"},
			{"[4]", "Adams et al. [4]"},
			{"[5]", "Doe et al. [5]"},
			{"[6]", "Ozola and Kalniņš [6]"},
			{"[7, p. 4]", "“Crop rotation revisited” [7, p. 4]"},
			{"[8]", "_Farming atlas of Europe_ [8]"},
			{"[9], [10]", "Ozols [9]; Ozols [10]"},
			{"[11], [12]", "Lee et al. [11]; Lee et al. [12]"},
			{"[13]", "Smith [13]"},
			{"[6, pp. 12–14], emphasis added", "Ozola and Kalniņš [6, pp. 12–14], emphasis added"},
			{"see also [6, ch. 3]", "see also Ozola and Kalniņš [6, ch. 3]"},
		},
		"vancouver": {
			{"[1–3]", "Smith [1]; Smith [2]; Smith [3]"},
			{"[4]", "Adams et al. [4]"},
			{"[5]", "Doe et al. [5]"},
			{"[6]", "Ozola and Kalniņš [6]"},
			{"[7, p. 4]", "“Crop rotation revisited” [7, p. 4]"},
			{"[8]", "_Farming atlas of Europe_ [8]"},
			{"[9, 10]", "Ozols [9]; Ozols [10]"},
			{"[11, 12]", "Lee et al. [11]; Lee et al. [12]"},
			{"[13]", "Smith [13]"},
			{"[6, p. 12-4, emphasis added]", "Ozola and Kalniņš [6, p. 12-4], emphasis added"},
			{"[see also 6, chap. 3]", "see also Ozola and Kalniņš [6, chap. 3]"},
		},
		"mla": {
			{"(J. Smith, _Alpha_; J. Smith, “Gamma”; J. Smith, “Beta”)", "J. Smith, _Alpha_; J. Smith, “Gamma”; J. Smith, “Beta”"},
			{"(Adams et al.)", "Adams et al."},
			{"(Doe et al.)", "Doe et al."},
			{"(Ozola and Kalniņš)", "Ozola and Kalniņš"},
			{"(“Crop rotation revisited” 4)", "“Crop rotation revisited” (4)"},
			{"(_Farming atlas of Europe_)", "_Farming atlas of Europe_"},
			{"(Ozols, _Zeta page_; Ozols, _Alpha page_)", "Ozols, _Zeta page_; Ozols, _Alpha page_"},
			{"(Lee et al., _L1_; Lee et al., _L2_)", "Lee et al., _L1_; Lee et al., _L2_"},
			{"(A. Smith)", "A. Smith"},
			{"(Ozola and Kalniņš 12–14, emphasis added)", "Ozola and Kalniņš (12–14, emphasis added)"},
			{"(see also Ozola and Kalniņš, ch. 3)", "see also Ozola and Kalniņš (ch. 3)"},
		},
	}
	for _, st := range styleOrder {
		t.Run(st, func(t *testing.T) {
			doc := &ast.Document{}
			var cs [][2]*ast.Cite
			for _, it := range items {
				p, n := cite(ast.CiteParenthetical, it...), cite(ast.CiteNarrative, it...)
				cs = append(cs, [2]*ast.Cite{p, n})
				doc.Blocks = append(doc.Blocks, para(p, &ast.Text{Value: " "}, n))
			}
			res, err := Process(doc, citeRefs(), Options{Style: st})
			if err != nil || len(res.Warnings) > 0 {
				t.Fatalf("err=%v warnings=%v", err, res.Warnings)
			}
			for i, pair := range cs {
				w := want[st][i]
				if got := mdText(pair[0].Rendered); got != w[0] {
					t.Errorf("case %d parenthetical:\n got: %s\nwant: %s", i, got, w[0])
				}
				if got := mdText(pair[1].Rendered); got != w[1] {
					t.Errorf("case %d narrative:\n got: %s\nwant: %s", i, got, w[1])
				}
			}
		})
	}
}

func TestYearSuffixesInBibliography(t *testing.T) {
	doc := &ast.Document{Blocks: []ast.Block{para(cite(0, keys("smith2020x", "smith2020y", "nd1", "nd2")...))}}
	res, _ := Process(doc, citeRefs(), Options{Style: "apa"})
	var got []string
	for _, e := range res.Entries {
		got = append(got, mdText(e.Inlines))
	}
	want := []string{
		"Ozols, P. (n.d.-a). _Alpha page_.",
		"Ozols, P. (n.d.-b). _Zeta page_.",
		"Smith, J. (2020a). Beta. _J_.",
		"Smith, J. (2020b). Gamma. _J_.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("entries:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if res.Numeric || res.Entries[0].Label != "" || res.Entries[2].ID != "ref-smith2020y" {
		t.Errorf("unexpected entry metadata: %+v", res.Entries[2])
	}
}

func TestMLARepeatedAuthors(t *testing.T) {
	doc := &ast.Document{Blocks: []ast.Block{para(cite(0, keys("smith2019", "smith2020x", "asmith")...))}}
	res, _ := Process(doc, citeRefs(), Options{Style: "mla"})
	var got []string
	for _, e := range res.Entries {
		got = append(got, mdText(e.Inlines))
	}
	want := []string{
		"Smith, Alice. _Other Smith_. 2010.",
		"Smith, John. _Alpha_. P, 2019.",
		"---. “Gamma.” _J_, 2020.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("entries:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestCitationLinks(t *testing.T) {
	refs := citeRefs()
	tests := []struct {
		style string
		c     *ast.Cite
		want  string
	}{
		{"apa", cite(0, ast.CiteItem{Key: "two", Locator: "33"}), "([Ozola & Kalniņš, 2016, p. 33](#ref-two))"},
		{"apa", cite(ast.CiteNarrative, keys("two")...), "[Ozola and Kalniņš (2016)](#ref-two)"},
		{"apa", cite(0, keys("smith2019", "smith2020x")...), "([Smith, 2019](#ref-smith2019), [2020](#ref-smith2020x))"},
		{"apa", cite(ast.CiteNarrative, keys("smith2019", "smith2020x")...), "[Smith](#ref-smith2019) ([2019](#ref-smith2019), [2020](#ref-smith2020x))"},
		{"apa", cite(0, ast.CiteItem{Key: "two", SuppressAuthor: true}), "([2016](#ref-two))"},
		{"mla", cite(0, ast.CiteItem{Key: "two", Locator: "5"}), "([Ozola and Kalniņš 5](#ref-two))"},
		{"mla", cite(0, ast.CiteItem{Key: "two", Locator: "5", SuppressAuthor: true}), "([5](#ref-two))"},
		{"ieee", cite(0, keys("smith2019", "smith2020x", "two")...), "[[1](#ref-smith2019)]–[[3](#ref-two)]"},
		{"vancouver", cite(0, keys("smith2019", "smith2020x", "two")...), "[[1](#ref-smith2019)–[3](#ref-two)]"},
		{"ieee", cite(ast.CiteNarrative, ast.CiteItem{Key: "adams", Locator: "2", LocatorLabel: "figure"}), "Adams et al. [[1](#ref-adams), Fig. 2]"},
	}
	for _, tc := range tests {
		doc := &ast.Document{Blocks: []ast.Block{para(tc.c)}}
		if _, err := Process(doc, refs, Options{Style: tc.style}); err != nil {
			t.Fatal(err)
		}
		if got := md(tc.c.Rendered); got != tc.want {
			t.Errorf("%s:\n got: %s\nwant: %s", tc.style, got, tc.want)
		}
	}
}

// TestNumericOrder checks that numbers follow the reading order, with
// footnote citations counted at their reference point, captions and
// table cells included, the abstract first and NoCite entries last.
func TestNumericOrder(t *testing.T) {
	refs := []ast.Reference{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "unused"} {
		refs = append(refs, ast.Reference{ID: k, Type: "book", Title: "Book " + k, Author: []ast.Name{name(strings.ToUpper(k), "X")}, Issued: date(2000, 0, 0)})
	}
	ca, cb, cc, cd := cite(0, keys("a")...), cite(0, keys("b")...), cite(0, keys("c")...), cite(0, keys("d")...)
	ce, cf, cg := cite(0, keys("e")...), cite(0, keys("f")...), cite(0, keys("g")...)
	doc := &ast.Document{
		Meta: ast.Meta{Abstract: []ast.Block{para(cg)}, NoCite: []string{"h"}},
		Blocks: []ast.Block{
			para(cb, &ast.Note{Blocks: []ast.Block{para(cc)}}, &ast.Text{Value: " and "}, ca),
			&ast.Table{Caption: []ast.Inline{ce}, Body: []ast.Row{{Cells: []ast.Cell{{Blocks: []ast.Block{&ast.Plain{Inlines: []ast.Inline{cd}}}}}}}},
			&ast.Figure{Caption: []ast.Inline{cf}},
			para(cite(0, keys("a", "b", "c")...)),
		},
	}
	res, err := Process(doc, refs, Options{Style: "ieee"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Numeric {
		t.Fatal("ieee should be numeric")
	}
	var order []string
	for _, e := range res.Entries {
		order = append(order, e.Label+":"+strings.TrimPrefix(e.ID, "ref-"))
	}
	want := []string{"1:g", "2:b", "3:c", "4:a", "5:e", "6:d", "7:f", "8:h"}
	if !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
	for c, n := range map[*ast.Cite]string{cg: "[1]", cb: "[2]", cc: "[3]", ca: "[4]", ce: "[5]", cd: "[6]", cf: "[7]"} {
		if got := mdText(c.Rendered); got != n {
			t.Errorf("cite %s rendered %q, want %q", c.Items[0].Key, got, n)
		}
	}
}

func TestNumericCompression(t *testing.T) {
	var refs []ast.Reference
	var first []ast.Inline
	for _, k := range []string{"r1", "r2", "r3", "r4", "r5", "r6"} {
		refs = append(refs, ast.Reference{ID: k, Type: "book", Title: k, Issued: date(2000, 0, 0)})
		first = append(first, cite(0, keys(k)...))
	}
	tests := []struct {
		items          []ast.CiteItem
		ieee, vancouve string
	}{
		{keys("r1", "r2", "r3"), "[1]–[3]", "[1–3]"},
		{keys("r3", "r1", "r2"), "[1]–[3]", "[1–3]"},
		{keys("r1", "r3"), "[1], [3]", "[1, 3]"},
		{keys("r1", "r2"), "[1], [2]", "[1, 2]"},
		{keys("r2", "r4", "r5", "r6"), "[2], [4]–[6]", "[2, 4–6]"},
		{keys("r1", "r1"), "[1]", "[1]"},
		{[]ast.CiteItem{{Key: "r1"}, {Key: "r2", Locator: "5"}, {Key: "r3"}}, "[1], [2, p. 5], [3]", "[1, 2 (p. 5), 3]"},
		{[]ast.CiteItem{{Key: "r4", Prefix: "see"}}, "see [4]", "[see 4]"},
	}
	for _, style := range []string{"ieee", "vancouver"} {
		for _, tc := range tests {
			c := cite(0, tc.items...)
			doc := &ast.Document{Blocks: []ast.Block{para(first...), para(c)}}
			if _, err := Process(doc, refs, Options{Style: style}); err != nil {
				t.Fatal(err)
			}
			want := tc.ieee
			if style == "vancouver" {
				want = tc.vancouve
			}
			if got := mdText(c.Rendered); got != want {
				t.Errorf("%s %v: got %q, want %q", style, tc.items, got, want)
			}
		}
	}
}

func TestUnknownKeys(t *testing.T) {
	refs := citeRefs()
	tests := []struct {
		style string
		items []ast.CiteItem
		want  string
	}{
		{"apa", keys("nope"), "*?nope*"},
		{"apa", keys("nope", "nada"), "*?nope*; *?nada*"},
		{"apa", keys("two", "nope"), "(Ozola & Kalniņš, 2016; *?nope*)"},
		{"ieee", keys("two", "nope"), "[1], *?nope*"},
		{"vancouver", keys("two", "nope"), "[1, *?nope*]"},
		{"ieee", keys("nope"), "*?nope*"},
	}
	for _, tc := range tests {
		c := cite(0, tc.items...)
		again := cite(0, keys("nope")...)
		doc := &ast.Document{Blocks: []ast.Block{para(c, again)}}
		res, err := Process(doc, refs, Options{Style: tc.style})
		if err != nil {
			t.Fatal(err)
		}
		if got := mdText(c.Rendered); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.style, got, tc.want)
		}
		n := 0
		for _, w := range res.Warnings {
			if w == "citation key not found: nope" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("want exactly one warning for nope, got %v", res.Warnings)
		}
		for _, e := range res.Entries {
			if strings.Contains(e.ID, "nope") {
				t.Errorf("unknown key in bibliography: %v", e.ID)
			}
		}
	}
}

func TestNoCite(t *testing.T) {
	doc := &ast.Document{Blocks: []ast.Block{para(cite(0, keys("two")...))}}
	res, _ := Process(doc, citeRefs(), Options{Style: "apa", NoCite: []string{"*"}})
	if len(res.Entries) != len(citeRefs()) {
		t.Errorf("nocite * gave %d entries", len(res.Entries))
	}

	doc = &ast.Document{
		Meta:   ast.Meta{NoCite: []string{"@adams"}},
		Blocks: []ast.Block{para(cite(0, keys("two")...))},
	}
	res, _ = Process(doc, citeRefs(), Options{Style: "ieee", NoCite: []string{"four", "missing"}})
	var ids []string
	for _, e := range res.Entries {
		ids = append(ids, e.Label+":"+e.ID)
	}
	if want := []string{"1:ref-two", "2:ref-four", "3:ref-adams"}; !slices.Equal(ids, want) {
		t.Errorf("entries %v, want %v", ids, want)
	}
	if !slices.Contains(res.Warnings, "nocite key not found: missing") {
		t.Errorf("missing nocite warning: %v", res.Warnings)
	}
}

func TestReferenceMerging(t *testing.T) {
	external := []ast.Reference{
		{ID: "Key1", Type: "book", Title: "External title", Issued: date(2001, 0, 0)},
		{ID: "Key1", Type: "book", Title: "Duplicate", Issued: date(2002, 0, 0)},
	}
	doc := &ast.Document{
		References: []ast.Reference{
			{ID: "Key1", Type: "book", Title: "Embedded title", Issued: date(2003, 0, 0)},
			{ID: "only-embedded", Type: "book", Title: "Gap filler", Issued: date(2004, 0, 0)},
		},
		Blocks: []ast.Block{para(cite(0, keys("key1", "only-embedded")...))},
	}
	res, _ := Process(doc, external, Options{Style: "apa"})
	if len(res.Entries) != 2 {
		t.Fatalf("got %d entries", len(res.Entries))
	}
	got := mdText(res.Entries[0].Inlines) + " | " + mdText(res.Entries[1].Inlines)
	if got != "_External title_. (2001). | _Gap filler_. (2004)." {
		t.Errorf("merged entries: %s", got)
	}
	if !slices.Contains(res.Warnings, "duplicate citation key: Key1 (first definition kept)") {
		t.Errorf("missing duplicate warning: %v", res.Warnings)
	}
}

func TestEntryID(t *testing.T) {
	for in, want := range map[string]string{
		"Smith2020":    "ref-smith2020",
		"doe:2020/x y": "ref-doe:2020-x-y",
		"a_b.c-d":      "ref-a_b.c-d",
		"Bērziņš":      "ref-b-rzi--",
		"":             "ref-",
		"ÜBER@2020#1":  "ref--ber-2020-1",
	} {
		if got := EntryID(in); got != want {
			t.Errorf("EntryID(%q) = %q, want %q", in, got, want)
		}
	}
	refs := []ast.Reference{
		{ID: "Ab", Type: "book", Title: "One", Issued: date(2001, 0, 0)},
		{ID: "ab", Type: "book", Title: "Two", Issued: date(2002, 0, 0)},
	}
	c1, c2 := cite(0, keys("Ab")...), cite(0, keys("ab")...)
	doc := &ast.Document{Blocks: []ast.Block{para(c1, c2)}}
	res, _ := Process(doc, refs, Options{Style: "ieee"})
	if res.Entries[0].ID != "ref-ab" || res.Entries[1].ID != "ref-ab-2" {
		t.Errorf("colliding anchors not separated: %s %s", res.Entries[0].ID, res.Entries[1].ID)
	}
	if md(c2.Rendered) != "[[2](#ref-ab-2)]" {
		t.Errorf("citation does not link to the deduplicated anchor: %s", md(c2.Rendered))
	}
}

func TestStyleAndLanguageSelection(t *testing.T) {
	two := citeRefs()[5]
	c := cite(0, ast.CiteItem{Key: "two", Locator: "33"})
	doc := &ast.Document{Blocks: []ast.Block{para(c)}}
	res, _ := Process(doc, []ast.Reference{two}, Options{Style: "no-such-style"})
	if got := mdText(c.Rendered); got != "(Ozola & Kalniņš, 2016, p. 33)" {
		t.Errorf("fallback style: %s", got)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "unknown citation style") {
		t.Errorf("warnings: %v", res.Warnings)
	}

	doc.Meta.CitationStyle = "chicago"
	doc.Meta.Lang = "lv-LV"
	if _, err := Process(doc, []ast.Reference{two}, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := mdText(c.Rendered); got != "(Ozola un Kalniņš 2016, 33)" {
		t.Errorf("document style and language: %s", got)
	}
	if _, err := Process(nil, nil, Options{}); err == nil {
		t.Error("nil document accepted")
	}
}

func TestLatvianCitations(t *testing.T) {
	refs := citeRefs()
	tests := []struct {
		c    *ast.Cite
		want string
	}{
		{cite(0, ast.CiteItem{Key: "two", Locator: "33"}), "(Ozola & Kalniņš, 2016, 33. lpp.)"},
		{cite(0, ast.CiteItem{Key: "two", Locator: "33-35"}), "(Ozola & Kalniņš, 2016, 33.–35. lpp.)"},
		{cite(ast.CiteNarrative, keys("two")...), "Ozola un Kalniņš (2016)"},
		{cite(0, keys("adams")...), "(Adams u. c., 2018)"},
		{cite(0, keys("nd1")...), "(Ozols, b. g.)"},
		{cite(0, ast.CiteItem{Key: "two", Locator: "2", LocatorLabel: "chapter"}), "(Ozola & Kalniņš, 2016, 2. nod.)"},
	}
	for _, tc := range tests {
		doc := &ast.Document{Blocks: []ast.Block{para(tc.c)}}
		if _, err := Process(doc, refs, Options{Style: "apa", Lang: "lv"}); err != nil {
			t.Fatal(err)
		}
		if got := mdText(tc.c.Rendered); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestBibliographyCollation(t *testing.T) {
	var refs []ast.Reference
	var items []ast.CiteItem
	for _, fam := range []string{"Dābols", "Čakste", "Cīrulis", "Gogh", "Hale", "Gauss"} {
		n := name(fam, "A")
		if fam == "Gogh" {
			n.Particle = "van"
		}
		refs = append(refs, ast.Reference{ID: fam, Type: "book", Title: "T", Author: []ast.Name{n}, Issued: date(2000, 0, 0)})
		items = append(items, ast.CiteItem{Key: fam})
	}
	order := func(lang string) []string {
		doc := &ast.Document{Blocks: []ast.Block{para(cite(0, items...))}}
		res, _ := Process(doc, refs, Options{Style: "apa", Lang: lang})
		var out []string
		for _, e := range res.Entries {
			out = append(out, strings.SplitN(mdText(e.Inlines), ",", 2)[0])
		}
		return out
	}
	if got, want := order("lv"), []string{"Cīrulis", "Čakste", "Dābols", "Gauss", "van Gogh", "Hale"}; !slices.Equal(got, want) {
		t.Errorf("lv order %v, want %v", got, want)
	}
	if got, want := order("en"), []string{"Čakste", "Cīrulis", "Dābols", "Gauss", "van Gogh", "Hale"}; !slices.Equal(got, want) {
		t.Errorf("en order %v, want %v", got, want)
	}
}

func TestAPAPrefixAndSorting(t *testing.T) {
	refs := citeRefs()
	tests := []struct {
		items []ast.CiteItem
		want  string
	}{
		{[]ast.CiteItem{{Key: "smith2019", Prefix: "see"}, {Key: "adams"}}, "(see Adams et al., 2018; Smith, 2019)"},
		{[]ast.CiteItem{{Key: "smith2019"}, {Key: "adams", Prefix: "see also"}}, "(Smith, 2019; see also Adams et al., 2018)"},
		{[]ast.CiteItem{{Key: "two", Locator: "chap. 4"}}, "(Ozola & Kalniņš, 2016, Chapter 4)"},
		{[]ast.CiteItem{{Key: "two", Locator: "§ 2"}}, "(Ozola & Kalniņš, 2016, Section 2)"},
		{[]ast.CiteItem{{Key: "two", Locator: "4", LocatorLabel: "para"}}, "(Ozola & Kalniņš, 2016, para. 4)"},
		{[]ast.CiteItem{{Key: "two", Suffix: ", for a review"}}, "(Ozola & Kalniņš, 2016, for a review)"},
	}
	for _, tc := range tests {
		c := cite(0, tc.items...)
		doc := &ast.Document{Blocks: []ast.Block{para(c)}}
		if _, err := Process(doc, refs, Options{Style: "apa"}); err != nil {
			t.Fatal(err)
		}
		if got := mdText(c.Rendered); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestCitationsEverywhere(t *testing.T) {
	cs := []*ast.Cite{cite(0, keys("two")...), cite(0, keys("two")...), cite(0, keys("two")...), cite(0, keys("two")...), cite(0, keys("two")...)}
	empty := &ast.Cite{}
	doc := &ast.Document{Blocks: []ast.Block{
		&ast.Div{Title: []ast.Inline{cs[0]}, Blocks: []ast.Block{para(cs[1])}},
		&ast.DefinitionList{Items: []ast.DefinitionItem{{Term: []ast.Inline{cs[2]}, Definitions: [][]ast.Block{{para(cs[3])}}}}},
		&ast.List{Items: []ast.ListItem{{Blocks: []ast.Block{&ast.Plain{Inlines: []ast.Inline{&ast.Emph{Inlines: []ast.Inline{cs[4]}}}}}}}},
		para(empty),
	}}
	if _, err := Process(doc, citeRefs(), Options{Style: "harvard"}); err != nil {
		t.Fatal(err)
	}
	for i, c := range cs {
		if got := mdText(c.Rendered); got != "(Ozola and Kalniņš, 2016)" {
			t.Errorf("cite %d rendered %q", i, got)
		}
	}
	if empty.Rendered != nil {
		t.Errorf("empty cite rendered: %v", empty.Rendered)
	}
}

func TestMissingFieldWarnings(t *testing.T) {
	refs := []ast.Reference{
		{ID: "notitle", Type: "book", Author: []ast.Name{name("A", "B")}, Issued: date(2000, 0, 0)},
		{ID: "nojournal", Type: "article-journal", Title: "T", Author: []ast.Name{name("C", "D")}},
	}
	doc := &ast.Document{Blocks: []ast.Block{para(cite(0, keys("notitle", "nojournal")...))}}
	res, _ := Process(doc, refs, Options{})
	for _, want := range []string{"reference notitle has no title", "reference nojournal has no journal title"} {
		if !slices.Contains(res.Warnings, want) {
			t.Errorf("missing warning %q in %v", want, res.Warnings)
		}
	}
}
