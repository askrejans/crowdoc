package cite

import (
	"fmt"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

func allFixtures() map[string]ast.Reference {
	fx := fixtures()
	for _, r := range moreFixtures() {
		fx[r.ID] = r
	}
	return fx
}

// TestBibliographyEntries checks every style against every main reference
// type (journal, book, chapter, proceedings, thesis, report, web page,
// magazine, newspaper, blog post, dataset, software, patent, standard,
// legislation, manuscript, edited book, anonymous work, preprint).
func TestBibliographyEntries(t *testing.T) {
	fx := allFixtures()
	tests := []struct{ style, id, want string }{
		{"apa", "smith2020", "Smith, J. A., & Lee, K. (2020). Deep learning for crop yields. _Journal of Field Crop Studies_, _12_(3), 45–67. https://doi.org/10.1000/xyz123"},
		{"apa", "kowalski2019", "Kowalski, A. (2019). _Soil science basics_ (2nd ed.). Northfield Press."},
		{"apa", "berzins2021", "Bērziņš, J. (2021). Grain storage. In I. Ozola & P. Kalniņš (Eds.), _Handbook of agriculture_ (pp. 101–120). Dzīles."},
		{"apa", "conf2020", "Smith, J. (2020). Sensor networks in the field. In _Proceedings of the International Conference on Agritech_ (pp. 1–10). Agritech Society. https://doi.org/10.1000/conf.2020.1"},
		{"apa", "liepa2018", "Liepa, M. (2018). _Soil carbon dynamics in Latvian forests_ [Doctoral dissertation, University of Latvia]."},
		{"apa", "who2022", "World Health Organization. (2022). _Global nutrition report_ (Report No. WHO-123). https://example.org/report.pdf"},
		{"apa", "page", "Ozols, P. (n.d.). _How to store grain_. Farming Today. Retrieved May 1, 2021, from https://example.org/grain"},
		{"apa", "mag", "Brown, M. (2021, April). The future of farming. _Fieldwork Weekly_, _8_(2), 20–24."},
		{"apa", "news", "Kalniņa, I. (2022, September 14). Harvest breaks records. _Riverside Courier_, A3. https://example.org/news/harvest"},
		{"apa", "blog", "Green, T. (2023, January 5). Why soil matters? _Soil Notes_. https://example.org/blog/soil"},
		{"apa", "data", "Latvian Grain Institute. (2021). _Grain moisture measurements 2010–2020_ (Version 1.2) [Data set]. Open Data Archive. https://doi.org/10.5555/data.42"},
		{"apa", "soft", "Ozola, I. (2024). _fieldcalc_ (Version 3.0.1) [Computer software]. Example Foundation. https://example.org/fieldcalc"},
		{"apa", "pat", "Krūmiņš, A. (2016). _Grain dryer with heat recovery_ (Patent No. LV 15123)."},
		{"apa", "std", "International Organization for Standardization. (2009). _Cereals and pulses — Determination of moisture content_ (ISO 712:2009)."},
		{"apa", "law", "Agricultural Land Act, No. 45 (2010). https://example.org/law/45"},
		{"apa", "ms", "Vītols, J. (2015). _Notes on crop rotation_ [Unpublished manuscript]. Riga Technical University."},
		{"apa", "edbook", "Ozola, I., & Kalniņš, P. (Eds.). (2021). _Handbook of agriculture_. Dzīles."},
		{"apa", "anon", "Crop rotation revisited: a review. (2019). _Agronomy Letters_, _4_, 1–9."},
		{"apa", "arxiv", "Doe, J. (2020). _Learning to farm_ (arXiv:2001.12345) [Preprint]. arXiv. https://arxiv.org/abs/2001.12345"},
		{"chicago", "smith2020", "Smith, John A., and Kim Lee. 2020. “Deep learning for crop yields.” _Journal of Field Crop Studies_ 12 (3): 45–67. https://doi.org/10.1000/xyz123."},
		{"chicago", "kowalski2019", "Kowalski, Anna. 2019. _Soil science basics_. 2nd ed. London: Northfield Press."},
		{"chicago", "berzins2021", "Bērziņš, Jānis. 2021. “Grain storage.” In _Handbook of agriculture_, edited by Ilze Ozola and Pēteris Kalniņš, 101–20. Rīga: Dzīles."},
		{"chicago", "conf2020", "Smith, John. 2020. “Sensor networks in the field.” In _Proceedings of the International Conference on Agritech_, 1–10. Agritech Society. https://doi.org/10.1000/conf.2020.1."},
		{"chicago", "liepa2018", "Liepa, Marta. 2018. “Soil carbon dynamics in Latvian forests.” PhD diss., University of Latvia."},
		{"chicago", "who2022", "World Health Organization. 2022. _Global nutrition report_. Report WHO-123. Geneva: World Health Organization. https://example.org/report.pdf."},
		{"chicago", "page", "Ozols, Pēteris. n.d. “How to store grain.” Farming Today. Accessed May 1, 2021. https://example.org/grain."},
		{"chicago", "mag", "Brown, Mary. 2021. “The future of farming.” _Fieldwork Weekly_, April 2021, 20–24."},
		{"chicago", "news", "Kalniņa, Ieva. 2022. “Harvest breaks records.” _Riverside Courier_, September 14, 2022, A3. https://example.org/news/harvest."},
		{"chicago", "blog", "Green, Tom. 2023. “Why soil matters?” _Soil Notes_, January 5, 2023. https://example.org/blog/soil."},
		{"chicago", "data", "Latvian Grain Institute. 2021. _Grain moisture measurements 2010–2020_. Version 1.2. Open Data Archive. https://doi.org/10.5555/data.42."},
		{"chicago", "soft", "Ozola, Ilze. 2024. _fieldcalc_. Version 3.0.1. Example Foundation. https://example.org/fieldcalc."},
		{"chicago", "pat", "Krūmiņš, Andris. 2016. Grain dryer with heat recovery. Patent LV 15123."},
		{"chicago", "std", "International Organization for Standardization. 2009. _Cereals and pulses — Determination of moisture content_. ISO 712:2009. Geneva: International Organization for Standardization."},
		{"chicago", "law", "Agricultural Land Act. 2010. No. 45. https://example.org/law/45."},
		{"chicago", "ms", "Vītols, Juris. 2015. “Notes on crop rotation.” Unpublished manuscript, Riga Technical University."},
		{"chicago", "edbook", "Ozola, Ilze, and Pēteris Kalniņš, eds. 2021. _Handbook of agriculture_. Rīga: Dzīles."},
		{"chicago", "anon", "“Crop rotation revisited: a review.” 2019. _Agronomy Letters_ 4: 1–9."},
		{"chicago", "arxiv", "Doe, Jane. 2020. _Learning to farm_. arXiv:2001.12345. arXiv. https://arxiv.org/abs/2001.12345."},
		{"harvard", "smith2020", "Smith, J.A. and Lee, K. (2020) ‘Deep learning for crop yields’, _Journal of Field Crop Studies_, 12(3), pp. 45–67. Available at: https://doi.org/10.1000/xyz123."},
		{"harvard", "kowalski2019", "Kowalski, A. (2019) _Soil science basics_. 2nd edn. London: Northfield Press."},
		{"harvard", "berzins2021", "Bērziņš, J. (2021) ‘Grain storage’, in Ozola, I. and Kalniņš, P. (eds) _Handbook of agriculture_. Rīga: Dzīles, pp. 101–120."},
		{"harvard", "conf2020", "Smith, J. (2020) ‘Sensor networks in the field’, in _Proceedings of the International Conference on Agritech_. Agritech Society, pp. 1–10. Available at: https://doi.org/10.1000/conf.2020.1."},
		{"harvard", "liepa2018", "Liepa, M. (2018) _Soil carbon dynamics in Latvian forests_. PhD thesis. University of Latvia."},
		{"harvard", "who2022", "World Health Organization (2022) _Global nutrition report_. Report WHO-123. Geneva: World Health Organization. Available at: https://example.org/report.pdf."},
		{"harvard", "page", "Ozols, P. (no date) _How to store grain_. Available at: https://example.org/grain (Accessed: 1 May 2021)."},
		{"harvard", "mag", "Brown, M. (2021) ‘The future of farming’, _Fieldwork Weekly_, April, pp. 20–24."},
		{"harvard", "news", "Kalniņa, I. (2022) ‘Harvest breaks records’, _Riverside Courier_, 14 September, p. A3. Available at: https://example.org/news/harvest."},
		{"harvard", "blog", "Green, T. (2023) ‘Why soil matters?’, _Soil Notes_, 5 January. Available at: https://example.org/blog/soil."},
		{"harvard", "data", "Latvian Grain Institute (2021) _Grain moisture measurements 2010–2020_ (Version 1.2) [Dataset]. Open Data Archive. Available at: https://doi.org/10.5555/data.42."},
		{"harvard", "soft", "Ozola, I. (2024) _fieldcalc_ (Version 3.0.1) [Computer program]. Example Foundation. Available at: https://example.org/fieldcalc."},
		{"harvard", "pat", "Krūmiņš, A. (2016) _Grain dryer with heat recovery_. Patent LV 15123."},
		{"harvard", "std", "International Organization for Standardization (2009) _Cereals and pulses — Determination of moisture content_. ISO 712:2009. Geneva: International Organization for Standardization."},
		{"harvard", "law", "Agricultural Land Act (2010). No. 45. Available at: https://example.org/law/45."},
		{"harvard", "ms", "Vītols, J. (2015) _Notes on crop rotation_. Unpublished manuscript. Riga Technical University."},
		{"harvard", "edbook", "Ozola, I. and Kalniņš, P. (eds) (2021) _Handbook of agriculture_. Rīga: Dzīles."},
		{"harvard", "anon", "‘Crop rotation revisited: a review’ (2019) _Agronomy Letters_, 4, pp. 1–9."},
		{"harvard", "arxiv", "Doe, J. (2020) _Learning to farm_. arXiv:2001.12345. arXiv. Available at: https://arxiv.org/abs/2001.12345."},
		{"ieee", "smith2020", "J. A. Smith and K. Lee, “Deep learning for crop yields,” _Journal of Field Crop Studies_, vol. 12, no. 3, pp. 45–67, Mar. 2020, doi: [10.1000/xyz123](https://doi.org/10.1000/xyz123)."},
		{"ieee", "kowalski2019", "A. Kowalski, _Soil science basics_, 2nd ed. London: Northfield Press, 2019."},
		{"ieee", "berzins2021", "J. Bērziņš, “Grain storage,” in _Handbook of agriculture_, I. Ozola and P. Kalniņš, Eds. Rīga: Dzīles, 2021, pp. 101–120."},
		{"ieee", "conf2020", "J. Smith, “Sensor networks in the field,” in _Proceedings of the International Conference on Agritech_, Riga, 2020, pp. 1–10, doi: [10.1000/conf.2020.1](https://doi.org/10.1000/conf.2020.1)."},
		{"ieee", "liepa2018", "M. Liepa, “Soil carbon dynamics in Latvian forests,” Ph.D. dissertation, University of Latvia, Riga, 2018."},
		{"ieee", "who2022", "World Health Organization, “Global nutrition report,” World Health Organization, Geneva, Rep. WHO-123, 2022. [Online]. Available: https://example.org/report.pdf"},
		{"ieee", "page", "P. Ozols, “How to store grain,” Farming Today. Accessed: May 1, 2021. [Online]. Available: https://example.org/grain"},
		{"ieee", "mag", "M. Brown, “The future of farming,” _Fieldwork Weekly_, vol. 8, no. 2, pp. 20–24, Apr. 2021."},
		{"ieee", "news", "I. Kalniņa, “Harvest breaks records,” _Riverside Courier_, p. A3, Sep. 14, 2022. [Online]. Available: https://example.org/news/harvest"},
		{"ieee", "blog", "T. Green, “Why soil matters?” _Soil Notes_, Jan. 5, 2023. [Online]. Available: https://example.org/blog/soil"},
		{"ieee", "data", "Latvian Grain Institute, “Grain moisture measurements 2010–2020,” Open Data Archive, 2021, doi: [10.5555/data.42](https://doi.org/10.5555/data.42)."},
		{"ieee", "soft", "I. Ozola, _fieldcalc_, ver. 3.0.1. Example Foundation, 2024. [Online]. Available: https://example.org/fieldcalc"},
		{"ieee", "pat", "A. Krūmiņš, “Grain dryer with heat recovery,” Patent LV 15123, Jul. 20, 2016."},
		{"ieee", "std", "_Cereals and pulses — Determination of moisture content_, ISO 712:2009, 2009."},
		{"ieee", "law", "_Agricultural Land Act_. 2010. [Online]. Available: https://example.org/law/45"},
		{"ieee", "ms", "J. Vītols, “Notes on crop rotation,” unpublished."},
		{"ieee", "edbook", "I. Ozola and P. Kalniņš, Eds., _Handbook of agriculture_. Rīga: Dzīles, 2021."},
		{"ieee", "anon", "“Crop rotation revisited: a review,” _Agronomy Letters_, vol. 4, pp. 1–9, 2019."},
		{"ieee", "arxiv", "J. Doe, “Learning to farm,” arXiv, arXiv:2001.12345, 2020. [Online]. Available: https://arxiv.org/abs/2001.12345"},
		{"vancouver", "smith2020", "Smith JA, Lee K. Deep learning for crop yields. Journal of Field Crop Studies. 2020 Mar;12(3):45-67. doi:[10.1000/xyz123](https://doi.org/10.1000/xyz123)"},
		{"vancouver", "kowalski2019", "Kowalski A. Soil science basics. 2nd ed. London: Northfield Press; 2019."},
		{"vancouver", "berzins2021", "Bērziņš J. Grain storage. In: Ozola I, Kalniņš P, editors. Handbook of agriculture. Rīga: Dzīles; 2021. p. 101-20."},
		{"vancouver", "conf2020", "Smith J. Sensor networks in the field. In: Proceedings of the International Conference on Agritech. Agritech Society; 2020. p. 1-10. doi:[10.1000/conf.2020.1](https://doi.org/10.1000/conf.2020.1)"},
		{"vancouver", "liepa2018", "Liepa M. Soil carbon dynamics in Latvian forests [dissertation]. Riga: University of Latvia; 2018."},
		{"vancouver", "who2022", "World Health Organization. Global nutrition report. Geneva: World Health Organization; 2022. Report No.: WHO-123. Available from: https://example.org/report.pdf"},
		{"vancouver", "page", "Ozols P. How to store grain [Internet]. Farming Today [cited 2021 May 1]. Available from: https://example.org/grain"},
		{"vancouver", "mag", "Brown M. The future of farming. Fieldwork Weekly. 2021 Apr;8(2):20-4."},
		{"vancouver", "news", "Kalniņa I. Harvest breaks records. Riverside Courier. 2022 Sep 14:A3. Available from: https://example.org/news/harvest"},
		{"vancouver", "blog", "Green T. Why soil matters? Soil Notes. 2023 Jan 5. Available from: https://example.org/blog/soil"},
		{"vancouver", "data", "Latvian Grain Institute. Grain moisture measurements 2010–2020 [dataset]. Open Data Archive; 2021. doi:[10.5555/data.42](https://doi.org/10.5555/data.42)"},
		{"vancouver", "soft", "Ozola I. fieldcalc [computer program]. Example Foundation; 2024. Available from: https://example.org/fieldcalc"},
		{"vancouver", "pat", "Krūmiņš A. Grain dryer with heat recovery. 2016. Patent LV 15123."},
		{"vancouver", "std", "International Organization for Standardization. Cereals and pulses — Determination of moisture content. Geneva: International Organization for Standardization; 2009. ISO 712:2009."},
		{"vancouver", "law", "Agricultural Land Act. 2010. Available from: https://example.org/law/45"},
		{"vancouver", "ms", "Vītols J. Notes on crop rotation [unpublished manuscript]. Riga Technical University; 2015."},
		{"vancouver", "edbook", "Ozola I, Kalniņš P, editors. Handbook of agriculture. Rīga: Dzīles; 2021."},
		{"vancouver", "anon", "Crop rotation revisited: a review. Agronomy Letters. 2019;4:1-9."},
		{"vancouver", "arxiv", "Doe J. Learning to farm. arXiv; 2020. arXiv:2001.12345. Available from: https://arxiv.org/abs/2001.12345"},
		{"mla", "smith2020", "Smith, John A., and Kim Lee. “Deep learning for crop yields.” _Journal of Field Crop Studies_, vol. 12, no. 3, Mar. 2020, pp. 45–67, https://doi.org/10.1000/xyz123."},
		{"mla", "kowalski2019", "Kowalski, Anna. _Soil science basics_. 2nd ed., Northfield Press, 2019."},
		{"mla", "berzins2021", "Bērziņš, Jānis. “Grain storage.” _Handbook of agriculture_, edited by Ilze Ozola and Pēteris Kalniņš, Dzīles, 2021, pp. 101–20."},
		{"mla", "conf2020", "Smith, John. “Sensor networks in the field.” _Proceedings of the International Conference on Agritech_, Agritech Society, June 2020, pp. 1–10, https://doi.org/10.1000/conf.2020.1."},
		{"mla", "liepa2018", "Liepa, Marta. _Soil carbon dynamics in Latvian forests_. 2018. University of Latvia, PhD dissertation."},
		{"mla", "who2022", "_Global nutrition report_. WHO-123, World Health Organization, 2022, https://example.org/report.pdf."},
		{"mla", "page", "Ozols, Pēteris. “How to store grain.” _Farming Today_, https://example.org/grain. Accessed 1 May 2021."},
		{"mla", "mag", "Brown, Mary. “The future of farming.” _Fieldwork Weekly_, vol. 8, no. 2, Apr. 2021, pp. 20–24."},
		{"mla", "news", "Kalniņa, Ieva. “Harvest breaks records.” _Riverside Courier_, 14 Sept. 2022, p. A3, https://example.org/news/harvest."},
		{"mla", "blog", "Green, Tom. “Why soil matters?” _Soil Notes_, 5 Jan. 2023, https://example.org/blog/soil."},
		{"mla", "data", "Latvian Grain Institute. _Grain moisture measurements 2010–2020_. Version 1.2, Open Data Archive, 2021, https://doi.org/10.5555/data.42."},
		{"mla", "soft", "Ozola, Ilze. _fieldcalc_. Version 3.0.1, Example Foundation, 2024, https://example.org/fieldcalc."},
		{"mla", "pat", "Krūmiņš, Andris. _Grain dryer with heat recovery_. Patent LV 15123, 20 July 2016."},
		{"mla", "std", "_Cereals and pulses — Determination of moisture content_. ISO 712:2009, International Organization for Standardization, 2009."},
		{"mla", "law", "Agricultural Land Act. 2010, https://example.org/law/45."},
		{"mla", "ms", "Vītols, Juris. _Notes on crop rotation_. Riga Technical University, 2015."},
		{"mla", "edbook", "Ozola, Ilze, and Pēteris Kalniņš, editors. _Handbook of agriculture_. Dzīles, 2021."},
		{"mla", "anon", "“Crop rotation revisited: a review.” _Agronomy Letters_, vol. 4, 2019, pp. 1–9."},
		{"mla", "arxiv", "Doe, Jane. _Learning to farm_. arXiv:2001.12345, arXiv, 2020, https://arxiv.org/abs/2001.12345."},
	}
	for _, tc := range tests {
		t.Run(tc.style+"/"+tc.id, func(t *testing.T) {
			if got := formatOne(tc.style, "en", fx[tc.id]); got != tc.want {
				t.Errorf("\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

func TestLocalizedEntries(t *testing.T) {
	fx := allFixtures()
	tests := []struct{ style, lang, id, want string }{
		{"apa", "lv", "berzins2021", "Bērziņš, J. (2021). Grain storage. No: I. Ozola & P. Kalniņš (red.), _Handbook of agriculture_ (101.–120. lpp.). Dzīles."},
		{"apa", "lv-LV", "page", "Ozols, P. (b. g.). _How to store grain_. Farming Today. Skatīts 2021. gada 1. maijā, no https://example.org/grain"},
		{"apa", "lv", "kowalski2019", "Kowalski, A. (2019). _Soil science basics_ (2. izd.). Northfield Press."},
		{"apa", "lv", "liepa2018", "Liepa, M. (2018). _Soil carbon dynamics in Latvian forests_ [Promocijas darbs, University of Latvia]."},
		{"harvard", "lv", "smith2020", "Smith, J.A. un Lee, K. (2020) „Deep learning for crop yields“, _Journal of Field Crop Studies_, 12(3), 45.–67. lpp. Pieejams: https://doi.org/10.1000/xyz123."},
		{"harvard", "lv", "page", "Ozols, P. (b. g.) _How to store grain_. Pieejams: https://example.org/grain (Skatīts: 2021. gada 1. maijā)."},
		{"ieee", "lv", "smith2020", "J. A. Smith un K. Lee, „Deep learning for crop yields“, _Journal of Field Crop Studies_, 12. sēj., nr. 3, 45.–67. lpp., 2020. gada marts, doi: [10.1000/xyz123](https://doi.org/10.1000/xyz123)."},
		{"ieee", "lv", "page", "P. Ozols, „How to store grain“, Farming Today. Skatīts: 2021. gada 1. maijs. [Tiešsaiste]. Pieejams: https://example.org/grain"},
		{"vancouver", "lv", "berzins2021", "Bērziņš J. Grain storage. No: Ozola I, Kalniņš P, red. Handbook of agriculture. Rīga: Dzīles; 2021. 101.-20. lpp."},
		{"chicago", "lv", "smith2020", "Smith, John A., un Kim Lee. 2020. „Deep learning for crop yields“. _Journal of Field Crop Studies_ 12 (3): 45–67. https://doi.org/10.1000/xyz123."},
		{"mla", "lv", "smith2020", "Smith, John A., un Kim Lee. „Deep learning for crop yields“. _Journal of Field Crop Studies_, 12. sēj., nr. 3, 2020. gada marts, 45.–67. lpp., https://doi.org/10.1000/xyz123."},
		{"chicago", "de", "smith2020", "Smith, John A., und Kim Lee. 2020. „Deep learning for crop yields“. _Journal of Field Crop Studies_ 12 (3): 45–67. https://doi.org/10.1000/xyz123."},
		{"chicago", "de", "liepa2018", "Liepa, Marta. 2018. „Soil carbon dynamics in Latvian forests“. Dissertation, University of Latvia."},
		{"apa", "de", "kowalski2019", "Kowalski, A. (2019). _Soil science basics_ (2. Aufl.). Northfield Press."},
		{"mla", "fr", "smith2020", "Smith, John A., et Kim Lee. «\u00a0Deep learning for crop yields\u00a0». _Journal of Field Crop Studies_, vol. 12, no 3, mars 2020, p. 45–67, https://doi.org/10.1000/xyz123."},
		{"chicago", "fr", "berzins2021", "Bērziņš, Jānis. 2021. «\u00a0Grain storage\u00a0». In _Handbook of agriculture_, édité par Ilze Ozola et Pēteris Kalniņš, 101–20. Rīga: Dzīles."},
		{"apa", "es", "berzins2021", "Bērziņš, J. (2021). Grain storage. En I. Ozola & P. Kalniņš (eds.), _Handbook of agriculture_ (pp. 101–120). Dzīles."},
		{"ieee", "pl", "kowalski2019", "A. Kowalski, _Soil science basics_, wyd. 2. London: Northfield Press, 2019."},
		{"harvard", "lt", "kowalski2019", "Kowalski, A. (2019) _Soil science basics_. 2-asis leid. London: Northfield Press."},
		{"chicago", "sv", "kowalski2019", "Kowalski, Anna. 2019. _Soil science basics_. 2:a uppl. London: Northfield Press."},
		{"apa", "fi", "news", "Kalniņa, I. (2022, 14. syyskuuta). Harvest breaks records. _Riverside Courier_, A3. https://example.org/news/harvest"},
		{"apa", "xx", "kowalski2019", "Kowalski, A. (2019). _Soil science basics_ (2nd ed.). Northfield Press."},
	}
	for _, tc := range tests {
		t.Run(tc.style+"/"+tc.lang+"/"+tc.id, func(t *testing.T) {
			if got := formatOne(tc.style, tc.lang, fx[tc.id]); got != tc.want {
				t.Errorf("\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

func TestAuthorListTruncation(t *testing.T) {
	var many []ast.Name
	for i := range 21 {
		many = append(many, name(fmt.Sprintf("Author%02d", i+1), "Anna"))
	}
	ref := func(authors []ast.Name) ast.Reference {
		return ast.Reference{ID: "x", Type: "article-journal", Title: "Big team science?", ContainerTitle: "J",
			Issued: date(2020, 0, 0), Author: authors}
	}
	others := append(append([]ast.Name{}, many[:2]...), ast.Name{Literal: "others"})
	tests := []struct {
		style   string
		authors []ast.Name
		want    string
	}{
		{"apa", many, "Author01, A., Author02, A., Author03, A., Author04, A., Author05, A., Author06, A., Author07, A., Author08, A., Author09, A., Author10, A., Author11, A., Author12, A., Author13, A., Author14, A., Author15, A., Author16, A., Author17, A., Author18, A., Author19, A., … Author21, A. (2020). Big team science? _J_."},
		{"apa", many[:7], "Author01, A., Author02, A., Author03, A., Author04, A., Author05, A., Author06, A., & Author07, A. (2020). Big team science? _J_."},
		{"chicago", many[:11], "Author01, Anna, Anna Author02, Anna Author03, Anna Author04, Anna Author05, Anna Author06, Anna Author07, et al. 2020. “Big team science?” _J_."},
		{"chicago", many[:7], "Author01, Anna, Anna Author02, Anna Author03, Anna Author04, Anna Author05, Anna Author06, and Anna Author07. 2020. “Big team science?” _J_."},
		{"harvard", many[:3], "Author01, A., Author02, A. and Author03, A. (2020) ‘Big team science?’, _J_."},
		{"ieee", many[:6], "A. Author01, A. Author02, A. Author03, A. Author04, A. Author05, and A. Author06, “Big team science?” _J_, 2020."},
		{"ieee", many[:7], "A. Author01 et al., “Big team science?” _J_, 2020."},
		{"vancouver", many[:6], "Author01 A, Author02 A, Author03 A, Author04 A, Author05 A, Author06 A. Big team science? J. 2020."},
		{"vancouver", many[:7], "Author01 A, Author02 A, Author03 A, Author04 A, Author05 A, Author06 A, et al. Big team science? J. 2020."},
		{"mla", many[:2], "Author01, Anna, and Anna Author02. “Big team science?” _J_, 2020."},
		{"mla", many[:3], "Author01, Anna, et al. “Big team science?” _J_, 2020."},
		{"apa", others, "Author01, A., Author02, A., et al. (2020). Big team science? _J_."},
		{"ieee", others, "A. Author01 et al., “Big team science?” _J_, 2020."},
		{"vancouver", others, "Author01 A, Author02 A, et al. Big team science? J. 2020."},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("%s/%d", tc.style, i), func(t *testing.T) {
			if got := formatOne(tc.style, "en", ref(tc.authors)); got != tc.want {
				t.Errorf("\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

func TestNameForms(t *testing.T) {
	r := ast.Reference{ID: "x", Type: "article-journal", Title: "T", ContainerTitle: "J", Issued: date(2020, 0, 0),
		Author: []ast.Name{
			{Family: "Gogh", Particle: "van", Given: "Jean-Paul"},
			{Family: "King", Given: "Martin Luther", Suffix: "Jr."},
			{Family: "Alembert", Particle: "d'", Given: "Jean le Rond"},
		}}
	tests := map[string]string{
		"apa":       "van Gogh, J.-P., King, M. L., Jr., & d'Alembert, J. R. (2020). T. _J_.",
		"chicago":   "van Gogh, Jean-Paul, Martin Luther King Jr., and Jean le Rond d'Alembert. 2020. “T.” _J_.",
		"harvard":   "van Gogh, J.-P., King, M.L., Jr. and d'Alembert, J.R. (2020) ‘T’, _J_.",
		"ieee":      "J.-P. van Gogh, M. L. King, Jr., and J. R. d'Alembert, “T,” _J_, 2020.",
		"vancouver": "van Gogh JP, King ML Jr, d'Alembert JR. T. J. 2020.",
		"mla":       "van Gogh, Jean-Paul, et al. “T.” _J_, 2020.",
	}
	for style, want := range tests {
		if got := formatOne(style, "en", r); got != want {
			t.Errorf("%s:\n got: %s\nwant: %s", style, got, want)
		}
	}
}

func TestInitials(t *testing.T) {
	tests := []struct {
		given         string
		period, space bool
		want          string
	}{
		{"John Alan", true, true, "J. A."},
		{"Jean-Paul", true, true, "J.-P."},
		{"Jean-Paul", false, false, "JP"},
		{"J.A.", true, false, "J.A."},
		{"JA", true, true, "J. A."},
		{"Ēriks Ūdris", true, true, "Ē. Ū."},
		{"Maria de la Paz", true, true, "M. P."},
		{"", true, true, ""},
	}
	for _, tc := range tests {
		if got := initials(tc.given, tc.period, tc.space); got != tc.want {
			t.Errorf("initials(%q) = %q, want %q", tc.given, got, tc.want)
		}
	}
}

func TestParseNameString(t *testing.T) {
	tests := []struct {
		in   string
		want ast.Name
	}{
		{"Smith, John A.", ast.Name{Family: "Smith", Given: "John A."}},
		{"Smith, John, Jr.", ast.Name{Family: "Smith", Given: "John", Suffix: "Jr."}},
		{"Smith, Jr., John", ast.Name{Family: "Smith", Given: "John", Suffix: "Jr."}},
		{"van der Berg, Anna", ast.Name{Family: "Berg", Particle: "van der", Given: "Anna"}},
		{"Ludwig van Beethoven", ast.Name{Family: "Beethoven", Particle: "van", Given: "Ludwig"}},
		{"John Smith", ast.Name{Family: "Smith", Given: "John"}},
		{"Aristotle", ast.Name{Family: "Aristotle"}},
		{"World Health Organization", ast.Name{Literal: "World Health Organization"}},
		{"Latvijas Universitāte", ast.Name{Literal: "Latvijas Universitāte"}},
	}
	for _, tc := range tests {
		if got := parseNameString(tc.in); got != tc.want {
			t.Errorf("parseNameString(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestPageRanges(t *testing.T) {
	tests := []struct{ format, in, want string }{
		{"expanded", "45-67", "45–67"},
		{"expanded", "45--67", "45–67"},
		{"expanded", "45 – 67", "45–67"},
		{"expanded", "e1234", "e1234"},
		{"expanded", "S12-S19", "S12–S19"},
		{"expanded", "45-67, 70-72", "45–67, 70–72"},
		{"chicago", "3-10", "3–10"},
		{"chicago", "96-117", "96–117"},
		{"chicago", "100-104", "100–104"},
		{"chicago", "1100-1123", "1100–1123"},
		{"chicago", "101-108", "101–8"},
		{"chicago", "808-833", "808–33"},
		{"chicago", "321-328", "321–28"},
		{"chicago", "498-532", "498–532"},
		{"chicago", "1087-1089", "1087–89"},
		{"chicago", "1496-1500", "1496–500"},
		{"chicago", "11564-11615", "11564–615"},
		{"minimal", "284-287", "284–7"},
		{"minimal", "42-45", "42–5"},
		{"minimal-two", "42-45", "42–45"},
		{"minimal-two", "103-104", "103–04"},
		{"minimal-two", "2787-2816", "2787–816"},
		{"minimal", "98-102", "98–102"},
	}
	for _, tc := range tests {
		f := &fmtr{st: &style{pageRange: tc.format, pageDash: "–"}, loc: getLocale("en"), en: true}
		if got := f.pageRange(tc.in); got != tc.want {
			t.Errorf("%s %q = %q, want %q", tc.format, tc.in, got, tc.want)
		}
	}
}

func TestEditionAndThesisTerms(t *testing.T) {
	f := newFmtr(styles["apa"], "en")
	for in, want := range map[string]string{
		"2": "2nd ed.", "2nd": "2nd ed.", "Second": "2nd ed.", "3rd edition": "3rd ed.", "11": "11th ed.",
		"22": "22nd ed.", "1": "", "Revised": "Revised ed.", "Rev. ed.": "Rev. ed.",
	} {
		if got := f.edition(in); got != want {
			t.Errorf("edition(%q) = %q, want %q", in, got, want)
		}
	}
	for genre, want := range map[string]string{
		"PhD thesis": "Doctoral dissertation", "Ph.D. dissertation": "Doctoral dissertation",
		"Master's thesis": "Master's thesis", "MSc dissertation": "Master's thesis",
		"Bachelor thesis": "Bachelor's thesis", "Habilitation": "Habilitation", "": "Thesis",
	} {
		if got := f.thesisLabel(genre); got != want {
			t.Errorf("thesisLabel(%q) = %q, want %q", genre, got, want)
		}
	}
	lv := newFmtr(styles["apa"], "lv")
	if got := lv.thesisLabel("mastersthesis"); got != "Maģistra darbs" {
		t.Errorf("lv master thesis = %q", got)
	}
}

func TestStylesAndAliases(t *testing.T) {
	infos := Styles()
	if len(infos) != 6 {
		t.Fatalf("Styles() returned %d styles", len(infos))
	}
	kinds := map[string]string{}
	for _, s := range infos {
		if s.Title == "" || s.Description == "" {
			t.Errorf("style %s lacks title or description", s.Name)
		}
		kinds[s.Name] = s.Kind
	}
	if kinds["ieee"] != "numeric" || kinds["apa"] != "author-date" || kinds["mla"] != "author-page" {
		t.Errorf("unexpected kinds %v", kinds)
	}
	for in, want := range map[string]string{
		"apa": "apa", "APA7": "apa", "apa-7th-edition": "apa", "chicago-author-date": "chicago",
		"Chicago Author Date": "chicago", "harvard-cite-them-right": "harvard", "harvard1": "harvard",
		"ieee.csl": "ieee", "vancouver": "vancouver", "nature": "vancouver", "mla9": "mla",
		"modern-language-association": "mla", "numeric": "ieee", "authoryear": "chicago",
	} {
		got, ok := NormalizeStyle(in)
		if !ok || got != want {
			t.Errorf("NormalizeStyle(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "  ", "nonexistent-style"} {
		if _, ok := NormalizeStyle(bad); ok {
			t.Errorf("NormalizeStyle(%q) unexpectedly succeeded", bad)
		}
	}
}

func TestTitlePunctuation(t *testing.T) {
	r := ast.Reference{ID: "q", Type: "article-journal", Author: []ast.Name{name("Lee", "Kim")},
		Title: "Is soil alive?", ContainerTitle: "Soil", Volume: "2", Issued: date(2020, 0, 0)}
	tests := map[string]string{
		"apa":       "Lee, K. (2020). Is soil alive? _Soil_, _2_.",
		"chicago":   "Lee, Kim. 2020. “Is soil alive?” _Soil_ 2.",
		"harvard":   "Lee, K. (2020) ‘Is soil alive?’, _Soil_, 2.",
		"ieee":      "K. Lee, “Is soil alive?” _Soil_, vol. 2, 2020.",
		"vancouver": "Lee K. Is soil alive? Soil. 2020;2.",
		"mla":       "Lee, Kim. “Is soil alive?” _Soil_, vol. 2, 2020.",
	}
	for style, want := range tests {
		if got := formatOne(style, "en", r); got != want {
			t.Errorf("%s:\n got: %s\nwant: %s", style, got, want)
		}
	}
	r.Title = "Soil Inc."
	if got := formatOne("apa", "en", r); got != "Lee, K. (2020). Soil Inc. _Soil_, _2_." {
		t.Errorf("abbreviation period doubled: %s", got)
	}
}
