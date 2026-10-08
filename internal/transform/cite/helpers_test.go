package cite

import (
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// md renders inlines as compact markup for assertions: _emph_, *strong*,
// [text](url) for links whose text differs from the URL.
func md(ins []ast.Inline) string {
	var sb strings.Builder
	writeMD(&sb, ins, true)
	return sb.String()
}

// mdText is md without link targets.
func mdText(ins []ast.Inline) string {
	var sb strings.Builder
	writeMD(&sb, ins, false)
	return sb.String()
}

func writeMD(sb *strings.Builder, ins []ast.Inline, links bool) {
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			sb.WriteString(n.Value)
		case *ast.Emph:
			sb.WriteString("_")
			writeMD(sb, n.Inlines, links)
			sb.WriteString("_")
		case *ast.Strong:
			sb.WriteString("*")
			writeMD(sb, n.Inlines, links)
			sb.WriteString("*")
		case *ast.Link:
			var inner strings.Builder
			writeMD(&inner, n.Inlines, links)
			if !links || inner.String() == n.URL {
				sb.WriteString(inner.String())
				continue
			}
			sb.WriteString("[" + inner.String() + "](" + n.URL + ")")
		case *ast.SoftBreak, *ast.LineBreak:
			sb.WriteString(" ")
		case *ast.Code:
			sb.WriteString("`" + n.Text + "`")
		case *ast.Math:
			sb.WriteString("$" + n.TeX + "$")
		case *ast.Note:
			sb.WriteString("^[")
			for _, b := range n.Blocks {
				for _, ins := range ast.InlinesOf(b) {
					writeMD(sb, ins, links)
				}
			}
			sb.WriteString("]")
		default:
			writeMD(sb, ast.InlineChildren(in), links)
		}
	}
}

func name(family, given string) ast.Name { return ast.Name{Family: family, Given: given} }

func date(y, m, d int) ast.Date { return ast.Date{Year: y, Month: m, Day: d} }

// fixtures returns one reference of each main type.
func fixtures() map[string]ast.Reference {
	refs := []ast.Reference{
		{
			ID: "smith2020", Type: "article-journal",
			Author:         []ast.Name{name("Smith", "John A."), name("Lee", "Kim")},
			Title:          "Deep learning for crop yields",
			ContainerTitle: "Journal of Field Crop Studies",
			Volume:         "12", Issue: "3", Page: "45-67",
			Issued: date(2020, 3, 0), DOI: "10.1000/xyz123",
		},
		{
			ID: "kowalski2019", Type: "book",
			Author: []ast.Name{name("Kowalski", "Anna")},
			Title:  "Soil science basics", Edition: "2",
			Publisher: "Northfield Press", PublisherPlace: "London",
			Issued: date(2019, 0, 0),
		},
		{
			ID: "berzins2021", Type: "chapter",
			Author:         []ast.Name{name("Bērziņš", "Jānis")},
			Title:          "Grain storage",
			ContainerTitle: "Handbook of agriculture",
			Editor:         []ast.Name{name("Ozola", "Ilze"), name("Kalniņš", "Pēteris")},
			Page:           "101-120", Publisher: "Dzīles", PublisherPlace: "Rīga",
			Issued: date(2021, 0, 0),
		},
		{
			ID: "conf2020", Type: "paper-conference",
			Author:         []ast.Name{name("Smith", "John")},
			Title:          "Sensor networks in the field",
			ContainerTitle: "Proceedings of the International Conference on Agritech",
			Page:           "1-10", Publisher: "Agritech Society", EventPlace: "Riga",
			Issued: date(2020, 6, 0), DOI: "10.1000/conf.2020.1",
		},
		{
			ID: "liepa2018", Type: "thesis", Genre: "PhD thesis",
			Author:    []ast.Name{name("Liepa", "Marta")},
			Title:     "Soil carbon dynamics in Latvian forests",
			Publisher: "University of Latvia", PublisherPlace: "Riga",
			Issued: date(2018, 0, 0),
		},
		{
			ID: "who2022", Type: "report",
			Author: []ast.Name{{Literal: "World Health Organization"}},
			Title:  "Global nutrition report", Number: "WHO-123",
			Publisher: "World Health Organization", PublisherPlace: "Geneva",
			Issued: date(2022, 0, 0), URL: "https://example.org/report.pdf",
		},
		{
			ID: "page", Type: "webpage",
			Author:         []ast.Name{name("Ozols", "Pēteris")},
			Title:          "How to store grain",
			ContainerTitle: "Farming Today",
			URL:            "https://example.org/grain", Accessed: date(2021, 5, 1),
		},
	}
	out := map[string]ast.Reference{}
	for _, r := range refs {
		out[r.ID] = r
	}
	return out
}

// formatOne formats a single reference in a style and language through
// the whole pipeline (as an uncited NoCite entry).
func formatOne(style, lang string, r ast.Reference) string {
	res, err := Process(&ast.Document{}, []ast.Reference{r}, Options{Style: style, Lang: lang, NoCite: []string{r.ID}})
	if err != nil || len(res.Entries) != 1 {
		return "<error>"
	}
	return md(res.Entries[0].Inlines)
}

// moreFixtures covers the less common reference types.
func moreFixtures() []ast.Reference {
	return []ast.Reference{
		{
			ID: "mag", Type: "article-magazine", Author: []ast.Name{name("Brown", "Mary")},
			Title: "The future of farming", ContainerTitle: "Fieldwork Weekly", Volume: "8", Issue: "2",
			Page: "20-24", Issued: date(2021, 4, 0),
		},
		{
			ID: "news", Type: "article-newspaper", Author: []ast.Name{name("Kalniņa", "Ieva")},
			Title: "Harvest breaks records", ContainerTitle: "Riverside Courier", Page: "A3",
			Issued: date(2022, 9, 14), URL: "https://example.org/news/harvest",
		},
		{
			ID: "blog", Type: "post-weblog", Author: []ast.Name{name("Green", "Tom")},
			Title: "Why soil matters?", ContainerTitle: "Soil Notes", Issued: date(2023, 1, 5),
			URL: "https://example.org/blog/soil",
		},
		{
			ID: "data", Type: "dataset", Author: []ast.Name{{Literal: "Latvian Grain Institute"}},
			Title: "Grain moisture measurements 2010–2020", Version: "1.2", Publisher: "Open Data Archive",
			Issued: date(2021, 0, 0), DOI: "10.5555/data.42",
		},
		{
			ID: "soft", Type: "software", Author: []ast.Name{name("Ozola", "Ilze")},
			Title: "fieldcalc", Version: "3.0.1", Publisher: "Example Foundation",
			Issued: date(2024, 0, 0), URL: "https://example.org/fieldcalc",
		},
		{
			ID: "pat", Type: "patent", Author: []ast.Name{name("Krūmiņš", "Andris")},
			Title: "Grain dryer with heat recovery", Number: "LV 15123", Issued: date(2016, 7, 20),
		},
		{
			ID: "std", Type: "standard", Author: []ast.Name{{Literal: "International Organization for Standardization"}},
			Title: "Cereals and pulses — Determination of moisture content", Number: "ISO 712:2009",
			Publisher: "International Organization for Standardization", PublisherPlace: "Geneva",
			Issued: date(2009, 0, 0),
		},
		{
			ID: "law", Type: "legislation", Title: "Agricultural Land Act", Number: "No. 45",
			Issued: date(2010, 0, 0), URL: "https://example.org/law/45",
		},
		{
			ID: "ms", Type: "manuscript", Author: []ast.Name{name("Vītols", "Juris")},
			Title: "Notes on crop rotation", Publisher: "Riga Technical University", Issued: date(2015, 0, 0),
		},
		{
			ID: "edbook", Type: "book", Editor: []ast.Name{name("Ozola", "Ilze"), name("Kalniņš", "Pēteris")},
			Title: "Handbook of agriculture", Publisher: "Dzīles", PublisherPlace: "Rīga", Issued: date(2021, 0, 0),
		},
		{
			ID: "anon", Type: "article-journal", Title: "Crop rotation revisited: a review",
			ContainerTitle: "Agronomy Letters", Volume: "4", Page: "1-9", Issued: date(2019, 0, 0),
		},
		{
			ID: "arxiv", Type: "article", Author: []ast.Name{name("Doe", "Jane")}, Title: "Learning to farm",
			Number: "arXiv:2001.12345", Publisher: "arXiv", Issued: date(2020, 0, 0),
			URL: "https://arxiv.org/abs/2001.12345",
		},
	}
}
