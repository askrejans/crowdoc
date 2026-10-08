package pdf

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// The fixtures in testdata were produced by real PDF writers; the sources
// are in testdata/src:
//
//	typst-roundtrip.pdf  crowdoc itself (Typst, tagged, two columns, Latvian)
//	typst-notes.pdf      crowdoc (examples/general/minimal-notes.md)
//	latex-article.pdf    LuaLaTeX, untagged, two columns, hyphenation
//	lo-report.pdf        LibreOffice 26 (writer_pdf_Export, tagged)
//	quartz-text.pdf      macOS cupsfilter (plain text, Quartz PDFContext)
//	quartz-html.pdf      macOS AppKit printing of an HTML page (Quartz)
//	enc-*.pdf, password.pdf, plain-objstm.pdf, scan.pdf
//	                     PyMuPDF: RC4 40/128-bit, AES 128/256-bit with an
//	                     empty user password, a user password, object and
//	                     cross-reference streams, an image-only page.

func readFixture(t *testing.T, name string) *ast.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := Read(context.Background(), data, rd.Options{Name: name})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if doc.Resources == nil {
		t.Fatalf("%s: nil resources", name)
	}
	return doc
}

// wantInOrder checks that every fragment occurs in dump, in this order.
func wantInOrder(t *testing.T, dump string, fragments ...string) {
	t.Helper()
	pos := 0
	for _, f := range fragments {
		i := strings.Index(dump[pos:], f)
		if i < 0 {
			if strings.Contains(dump, f) {
				t.Errorf("%q is out of order", f)
			} else {
				t.Errorf("missing %q", f)
			}
			continue
		}
		pos += i + len(f)
	}
	if t.Failed() {
		t.Logf("dump:\n%s", dump)
	}
}

func authorNames(m ast.Meta) []string {
	var out []string
	for _, a := range m.Authors {
		out = append(out, a.Name)
	}
	return out
}

func TestTypstRoundTrip(t *testing.T) {
	for _, structure := range []bool{true, false} {
		name := "tagged"
		if !structure {
			name = "geometry"
		}
		t.Run(name, func(t *testing.T) {
			useStructure = structure
			defer func() { useStructure = true }()
			doc := readFixture(t, "typst-roundtrip.pdf")
			m := doc.Meta
			if m.Title != "Augsnes mitruma novērošana Zemgalē" || m.Subtitle != "Metodika un pirmie rezultāti" {
				t.Errorf("title %q, subtitle %q", m.Title, m.Subtitle)
			}
			if got := strings.Join(authorNames(m), "; "); got != "Ilze Kalniņa; Pēteris Ozoliņš" {
				t.Errorf("authors %q", got)
			}
			if len(m.Authors) == 2 && (strings.Join(m.Authors[0].Affiliations, "") != "Latvijas Lauksaimniecības universitāte" ||
				strings.Join(m.Authors[1].Affiliations, "") != "Zemkopības institūts") {
				t.Errorf("affiliations %+v", m.Authors)
			}
			if m.Date != "2026. gada 12. aprīlis" {
				t.Errorf("date %q", m.Date)
			}
			if strings.Join(m.Keywords, ",") != "augsne,mitrums,sensori" {
				t.Errorf("keywords %q", m.Keywords)
			}
			if !strings.HasPrefix(ast.BlocksText(m.Abstract), "Pētījumā salīdzināti augsnes mitruma mērījumi") {
				t.Errorf("abstract %q", ast.BlocksText(m.Abstract))
			}
			if m.Columns != 2 {
				t.Errorf("columns = %d", m.Columns)
			}
			if m.NumberSections == nil || *m.NumberSections {
				t.Errorf("manual section numbers not detected")
			}
			if !strings.HasPrefix(m.Lang, "lv") {
				t.Errorf("lang %q", m.Lang)
			}
			dump := ast.Dump(doc.Blocks)
			wantInOrder(t, dump,
				"H1 1 Ievads\n",
				"Para Augsnes mitrums ir galvenais faktors, kas nosaka sējumu dīgšanu un ražību.",
				"kalibrēšanu.^[Para Sensori kalibrēti laboratorijā pēc gravimetriskās metodes.] Ģimenes saimniecības, ķīmiskās analīzes un ļoti precīzi ņemti paraugi ir šī darba pamatā; ūdens režīms un žāvēšanas metodes aprakstītas tālāk.\n",
				"H1 2 Metodes\n",
				"Para Mērījumu secība bija šāda:\n",
				"Ordered(0,0)-tight\n  Item\n    Plain sensoru uzstādīšana 10 un 30 cm dziļumā;\n",
				"Plain kalibrēšana pēc katra lietus.\n",
				"Bullet-tight\n  Item\n    Plain kapacitatīvie sensori;\n",
				"Plain lauka planšete.\n",
				"Table cols=3 cap=Vidējais mitrums pa laukiem\n  head: |Lauks |Sensori |Mitrums (%)\n  body: |Ozolkalni |4 |24,1\n",
				"body: |Lejas |5 |26,3\n",
				"Quote\n  Para Mitrums ir jāmēra regulāri, nevis tikai pēc lietus, citādi secinājumi par augsnes ūdens režīmu būs nepilnīgi.\n",
				"H1 3 Rezultāti\n",
				"cap=Mitruma dinamika sezonas laikā\n",
				"Para Bezaršanas laukos mitrums saglabājās vidēji par trim dienām ilgāk nekā aramzemē.",
				"H2 3.1 Secinājumi\n",
				"Para Lēti sensori ir piemēroti saimniecību ikdienas vajadzībām, ja tos regulāri kalibrē un pārbauda.\n",
			)
			// The writer hyphenates Latvian with soft hyphens; none may
			// survive, and nothing of the title block may leak.
			for _, bad := range []string{"­", "Anotācija", "Atslēgvārdi", "Kalniņa", "1 attēls"} {
				if strings.Contains(dump, bad) {
					t.Errorf("dump contains %q", bad)
				}
			}
			if doc.Resources.Len() != 1 {
				t.Errorf("resources = %v", doc.Resources.Names())
			}
		})
	}
}

func TestTypstNotes(t *testing.T) {
	doc := readFixture(t, "typst-notes.pdf")
	if doc.Meta.Title != "Project Meeting Notes" || doc.Meta.Date != "15 January 2026" {
		t.Errorf("meta %q %q", doc.Meta.Title, doc.Meta.Date)
	}
	wantInOrder(t, ast.Dump(doc.Blocks),
		"H1 Attendees\nBullet-tight\n  Item\n    Plain [Person A] – Engineering Lead\n",
		"H1 Agenda\nOrdered(0,0)-tight\n  Item\n    Plain Sprint review\n",
		"Para Completed 14 out of 16 planned story points. Two items carried over:\n",
		// Task lists: the check boxes are vector drawings.
		"Bullet-tight\n  Item[ ]\n    Plain API pagination refactor (blocked by schema migration)\n  Item[ ]\n",
		"  Item[x]\n    Plain User onboarding flow redesign\n",
		"Quote\n  Para We agreed to prioritize the mobile experience for Q2, shifting two desktop-only features to Q3.\n",
		"H2 Feature Prioritization\nTable cols=4\n  head: |Feature |Priority |Owner |ETA\n  body: |Mobile responsive |P0 |Engineering |April 15\n",
		"H1 Action Items\nOrdered(0,0)-tight\n  Item\n    Plain **[Person A]:** Complete API schema migration plan by Friday\n",
		"H1 Next Meeting\nPara Wednesday, January 22, 2026 at 10:00 AM.\n",
	)
}

func TestLaTeXArticle(t *testing.T) {
	doc := readFixture(t, "latex-article.pdf")
	m := doc.Meta
	if m.Title != "Distributed Consensus in Heterogeneous Sensor Networks" || m.Date != "12 May 2025" {
		t.Errorf("meta %q %q", m.Title, m.Date)
	}
	if got := strings.Join(authorNames(m), "; "); got != "Anna Kalniņa; Roberts Bērziņš; Laura Smith" {
		t.Errorf("authors %q", got)
	}
	// "wire-less", "bud-get", "in-termittent" are hyphenated at line ends.
	if abs := ast.BlocksText(m.Abstract); !strings.Contains(abs, "heterogeneous wireless sensor networks whose nodes differ in energy budget, radio range and computational capability. Our protocol reduces communication overhead by forty percent while preserving convergence guarantees under intermittent connectivity.") {
		t.Errorf("abstract %q", abs)
	}
	if m.Columns != 2 {
		t.Errorf("columns = %d", m.Columns)
	}
	dump := ast.Dump(doc.Blocks)
	wantInOrder(t, dump,
		"H1 1 Introduction\nPara Wireless sensor networks are increasingly deployed",
		"In such deployments the individual nodes are rarely identical: manufacturers, battery chemistries and firmware revisions vary, and the resulting heterogeneity complicates the design of distributed algorithms.^[Para A survey of deployed networks is given by the working group on embedded sensing.] Classical consensus",
		"in large-scale simulations.^[Para Simulation code and measurement data are available from the authors on request.]\n",
		"H1 2 Related work\nPara Gossip-based averaging was introduced for peer-to-peer systems",
		"H2 2.1 Consensus under constraints\n",
		"Table cols=4 cap=Network configurations used in the evaluation.\n  head: |Configuration |Nodes |Range (m) |Energy (J)\n  body: |Small |50 |30 |120\n",
		"Bullet-tight\n  Item\n    Plain limited transmission power and therefore limited neighbourhoods;\n",
		"H1 3 Method\n",
		"Ordered(0,0)-tight\n  Item\n    Plain estimate the disagreement with the neighbourhood;\n",
		"H1 4 Evaluation\n",
		"cap=Messages transmitted per round for increasing network sizes.\n",
		"H1 6 Conclusion\n",
		"H1 References\nPara [1] A. Author and B. Writer.",
	)
	for _, bad := range []string{"indi- vidual", "dis- tributed", "con- straints", "simulta- neously"} {
		if strings.Contains(dump, bad) {
			t.Errorf("stray hyphenation %q", bad)
		}
	}
}

func TestLibreOfficeReport(t *testing.T) {
	doc := readFixture(t, "lo-report.pdf")
	if doc.Meta.Title != "Lauksaimniecības pārskats 2025" {
		t.Errorf("title %q", doc.Meta.Title)
	}
	if strings.Join(authorNames(doc.Meta), "") != "Ilze Kalniņa" || strings.Join(doc.Meta.Keywords, "") != "lauksaimniecība" {
		t.Errorf("meta %+v", doc.Meta)
	}
	dump := ast.Dump(doc.Blocks)
	wantInOrder(t, dump,
		"H1 Ievads\n",
		"ekonomisko dzīvotspēju.^[Para Dati iegūti no Centrālās statistikas pārvaldes datubāzes.] Pārskatā apkopoti",
		// Soft hyphens at line ends ("infra-structures") are removed.
		"these infrastructures accumulate heterogeneous observations whose interpretation requires considerable methodological sophistication",
		"H2 Galvenās tendences\nPara Svarīgākās tendences ir šādas:\nBullet-tight\n",
		"    Plain bioloģiskās lauksaimniecības īpatsvars sasniedza 15 %;\n    Bullet-tight\n      Item\n        Plain īpaši strauji – Vidzemē un Latgalē;\n",
		"Quote\n  Para Zeme nav mantota no mūsu senčiem",
		"Para Tabulā apkopoti vidējie ražas rādītāji pa reģioniem.^[Para Ražība norādīta tonnās no hektāra.]\n",
		"Table cols=4 cap=Vidējā raža pa reģioniem\n  head: |Reģions |Kvieši |Mieži |Rapsis\n  body: |Zemgale |6,1 |4,8 |3,9\n",
		"H1 Secinājumi\nOrdered(0,0)-tight\n",
		"Plain Jāatbalsta mazās saimniecības:\n    Ordered(0,1)-tight\n      Item\n        Plain ar konsultācijām;\n",
		"Figure src=\"res:page2-image1.png\" w=340pt cap=Ražas dinamika pēdējos astoņos gados\n",
		"Para Noslēgumā jāuzsver, ka **ilgtspējīga attīstība** ir iespējama tikai ar _visu iesaistīto pušu_ sadarbību.\n",
	)
	if strings.Contains(dump, "Lappuse") {
		t.Error("page footer was not removed")
	}
}

func TestQuartz(t *testing.T) {
	doc := readFixture(t, "quartz-html.pdf")
	if doc.Meta.Title != "Field Guide to Soil Sampling" {
		t.Errorf("title %q", doc.Meta.Title)
	}
	wantInOrder(t, ast.Dump(doc.Blocks),
		"H1 Equipment\nBullet-tight\n  Item\n    Plain Stainless steel probe or auger\n",
		// The list renderer draws every marker twice.
		"H1 Procedure\nOrdered(0,0)-tight\n  Item\n    Plain Divide the field into uniform zones.\n",
		"Para Label each bag with the field name, the zone and the date. **Ship samples within two days** and keep them _cool and dry_ in the meantime.\n",
		"ģimenes saimniecības, ķīmiskās analīzes, ļoti svarīgi, ņemot vērā augsnes īpašības.",
	)

	// A plain-text printout: one monospaced size, structure from blank
	// lines, capitals and section numbers.
	doc = readFixture(t, "quartz-text.pdf")
	if doc.Meta.Title != "QUARTERLY OPERATIONS SUMMARY" {
		t.Errorf("title %q", doc.Meta.Title)
	}
	wantInOrder(t, ast.Dump(doc.Blocks),
		"H1 1. Overview\nPara The operations team completed the migration of the warehouse inventory system during the third quarter.",
		"H1 2. Key figures\nBullet-tight\n  Item\n    Plain Orders processed: 18 240\n",
		"H1 3. Next steps\n",
		"Para Latviešu valodā: Ražošanas apjomi ir pieauguši, un šīs pārmaiņas ietekmē gan ražību, gan vides ilgtspēju. Ģimenes saimniecības – ķīmija, ļoti, ņemot.\n",
	)
}

func TestSecurityHandlers(t *testing.T) {
	for _, name := range []string{"enc-rc4-40.pdf", "enc-rc4-128.pdf", "enc-aes-128.pdf", "enc-aes-256.pdf", "plain-objstm.pdf"} {
		t.Run(name, func(t *testing.T) {
			doc := readFixture(t, name)
			if doc.Meta.Title != "Encrypted Report" || strings.Join(authorNames(doc.Meta), "") != "Jane Roe" {
				t.Errorf("meta %q %v", doc.Meta.Title, authorNames(doc.Meta))
			}
			want := "Para The quick brown fox jumps over the lazy dog. This paragraph checks that encrypted streams decode correctly in every security handler revision.\nH1 Section Two\nPara Plain text after the heading.\n"
			if got := ast.Dump(doc.Blocks); got != want {
				t.Errorf("got\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestReadErrors(t *testing.T) {
	read := func(name string) error {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = Read(context.Background(), data, rd.Options{})
		return err
	}
	if err := read("password.pdf"); !errors.Is(err, ErrPassword) {
		t.Errorf("password.pdf: %v", err)
	}
	if err := read("scan.pdf"); !errors.Is(err, ErrNoText) || err.Error() != "this PDF has no text layer (it is a scan); run OCR first" {
		t.Errorf("scan.pdf: %v", err)
	}
	for _, data := range [][]byte{nil, []byte("%PDF-1.7\n"), []byte("not a pdf at all"), []byte("%PDF-1.4\n1 0 obj << >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF")} {
		if _, _, err := Read(context.Background(), data, rd.Options{}); err == nil {
			t.Errorf("%q: no error", data)
		}
	}
}

// TestMalformed truncates and corrupts real files: the reader must return
// a document or an error, never panic or hang.
func TestMalformed(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.pdf")
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var variants [][]byte
		for _, frac := range []float64{0.1, 0.35, 0.5, 0.75, 0.9, 0.98} {
			variants = append(variants, data[:int(float64(len(data))*frac)])
		}
		for k := 1; k < 8; k++ {
			c := append([]byte(nil), data...)
			for i := len(c) * k / 8; i < len(c)*k/8+64 && i < len(c); i++ {
				c[i] ^= 0x5a
			}
			variants = append(variants, c)
		}
		// A file whose cross-reference offsets are all wrong.
		variants = append(variants, append([]byte("garbage before the header\n"), data...))
		for i, v := range variants {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			doc, _, err := read(ctx, v, rd.Options{})
			cancel()
			if err == nil && (doc == nil || doc.Resources == nil) {
				t.Errorf("%s variant %d: nil document without error", f, i)
			}
		}
	}
}

func TestCancel(t *testing.T) {
	data, err := os.ReadFile("testdata/latex-article.pdf")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Read(ctx, data, rd.Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestLimits(t *testing.T) {
	data, err := os.ReadFile("testdata/lo-report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	// A decompression budget far below the content size must be honoured.
	_, _, err = Read(context.Background(), data, rd.Options{Limits: rd.Limits{MaxUnpacked: 2000, MaxEntry: 1000}})
	if err == nil {
		t.Fatal("expected an error with a tiny decompression budget")
	}
}
