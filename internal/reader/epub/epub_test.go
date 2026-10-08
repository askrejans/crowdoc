package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/internal/reader/html/asttest"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

type file struct{ name, body string }

func buildZip(t testing.TB, files []file) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		method := zip.Deflate
		if f.name == "mimetype" {
			method = zip.Store
		}
		fw, err := w.CreateHeader(&zip.FileHeader{Name: f.name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const containerXML = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

const pngBytes = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

func xhtmlDoc(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Chapter</title><link rel="stylesheet" type="text/css" href="../css/style.css"/></head>
<body>` + body + `</body></html>`
}

func epub3(t testing.TB) []byte {
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
 <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:identifier id="uid">urn:uuid:1</dc:identifier>
  <dc:title id="t1">The Main Title</dc:title>
  <meta refines="#t1" property="title-type">main</meta>
  <dc:title id="t2">A Subtitle</dc:title>
  <meta refines="#t2" property="title-type">subtitle</meta>
  <dc:creator id="c1">Anna Bērziņa</dc:creator>
  <meta refines="#c1" property="role" scheme="marc:relators">aut</meta>
  <dc:creator id="c2">Ed Editor</dc:creator>
  <meta refines="#c2" property="role" scheme="marc:relators">edt</meta>
  <dc:creator>Plain Author</dc:creator>
  <dc:language>lv</dc:language>
  <dc:date>2023-05-01T00:00:00Z</dc:date>
  <dc:description>&lt;p&gt;A &lt;b&gt;short&lt;/b&gt; description.&lt;/p&gt;</dc:description>
  <dc:subject>Fiction</dc:subject>
  <dc:subject>History; Latvia</dc:subject>
  <dc:publisher>Example Press</dc:publisher>
  <meta property="dcterms:modified">2024-01-01T00:00:00Z</meta>
 </metadata>
 <manifest>
  <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
  <item id="cover" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
  <item id="ch1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
  <item id="ch2" href="text/ch%202.xhtml" media-type="application/xhtml+xml"/>
  <item id="notes" href="text/notes.xhtml" media-type="application/xhtml+xml"/>
  <item id="img-cover" href="images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
  <item id="img-fig" href="images/fig.png" media-type="image/png"/>
  <item id="css" href="css/style.css" media-type="text/css"/>
 </manifest>
 <spine>
  <itemref idref="cover"/>
  <itemref idref="nav" linear="no"/>
  <itemref idref="ch1"/>
  <itemref idref="ch2"/>
  <itemref idref="notes" linear="no"/>
 </spine>
</package>`
	nav := xhtmlDoc(`<nav epub:type="toc"><ol><li><a href="text/ch1.xhtml">One</a></li></ol></nav>`)
	cover := xhtmlDoc(`<div><svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" viewBox="0 0 600 800"><image width="600" height="800" xlink:href="../images/cover.jpg"/></svg></div>`)
	ch1 := xhtmlDoc(`<section epub:type="chapter">
<h1 id="intro">Intro</h1>
<p>Text<a epub:type="noteref" href="notes.xhtml#n1">1</a> and local<a epub:type="noteref" href="#fl">2</a>. See <a href="ch%202.xhtml#sec">the section</a> and <a href="ch 2.xhtml">chapter two</a>, or <a href="https://example.test/">the web</a>.</p>
<p><img src="../images/fig.png" alt="A figure"/></p>
<a id="dup"/>
<p>After a self-closing anchor<span epub:type="pagebreak" id="page5" title="5"/> on page five.</p>
<aside epub:type="footnote" id="fl"><p>Local footnote.</p></aside>
</section>`)
	ch2 := xhtmlDoc(`<section>
<h1>Chapter Two</h1>
<h2 id="sec">Section</h2>
<p id="dup">Duplicate id target.</p>
<p><a href="#dup">back to dup</a> <a href="../css/style.css">stylesheet</a></p>
<p>Missing <img src="../images/missing.png" alt="gone"/> image.</p>
<math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><mfrac><mi>a</mi><mi>b</mi></mfrac></math>
</section>`)
	notes := xhtmlDoc(`<section epub:type="endnotes"><h2>Notes</h2><ol><li epub:type="endnote" id="n1"><p>Endnote text. <a href="ch1.xhtml" epub:type="backlink">↩</a></p></li></ol></section>`)
	return buildZip(t, []file{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", opf},
		{"OEBPS/nav.xhtml", nav},
		{"OEBPS/text/cover.xhtml", cover},
		{"OEBPS/text/ch1.xhtml", ch1},
		{"OEBPS/text/ch 2.xhtml", ch2},
		{"OEBPS/text/notes.xhtml", notes},
		{"OEBPS/images/cover.jpg", "\xff\xd8\xff\xe0JFIF"},
		{"OEBPS/images/fig.png", pngBytes},
		{"OEBPS/css/style.css", "p{}"},
	})
}

func TestReadEPUB3(t *testing.T) {
	doc, warns, err := Read(context.Background(), epub3(t), rd.Options{Name: "book.epub"})
	if err != nil {
		t.Fatal(err)
	}
	m := doc.Meta
	if m.Title != "The Main Title" || m.Subtitle != "A Subtitle" {
		t.Errorf("Title/Subtitle = %q/%q", m.Title, m.Subtitle)
	}
	if got := m.AuthorNames(); !reflect.DeepEqual(got, []string{"Anna Bērziņa", "Plain Author"}) {
		t.Errorf("Authors = %q", got)
	}
	if m.Lang != "lv" || m.Date != "2023-05-01" || m.Summary != "A short description." || m.Organization != "Example Press" {
		t.Errorf("meta = lang %q date %q summary %q org %q", m.Lang, m.Date, m.Summary, m.Organization)
	}
	if !reflect.DeepEqual(m.Keywords, []string{"Fiction", "History", "Latvia"}) {
		t.Errorf("Keywords = %q", m.Keywords)
	}
	want := strings.Join([]string{
		`H1#intro[Intro]`,
		`P[Text^note{P[Endnote text.]} and local^note{P[Local footnote.]}. See <#sec>[the section] and <#ch-2>[chapter two], or <https://example.test/>[the web].]`,
		`Fig(!img(res:OEBPS/images/fig.png alt=A figure))`,
		`P[After a self-closing anchor on page five.]`,
		`H1#ch-2[Chapter Two]`,
		`H2#sec[Section]`,
		`P[span#ch-2-dup[]Duplicate id target.]`,
		`P[<#ch-2-dup>[back to dup] stylesheet]`,
		`P[Missing gone image.]`,
		`$$\frac{a}{b}$$`,
	}, "\n")
	if got := asttest.Dump(doc.Blocks); got != want {
		t.Errorf("blocks:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if names := doc.Resources.Names(); !reflect.DeepEqual(names, []string{"OEBPS/images/fig.png"}) {
		t.Errorf("resources = %q", names)
	}
	res, _ := doc.Resources.Get("OEBPS/images/fig.png")
	if res.MediaType != "image/png" || string(res.Data) != pngBytes {
		t.Errorf("resource = %+v", res)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "missing.png") {
		t.Errorf("warnings = %q", warns)
	}
}

func TestReadEPUB2(t *testing.T) {
	opf := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="BookId" version="2.0">
 <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
  <dc:title>Old Book</dc:title>
  <dc:creator opf:role="aut" opf:file-as="Doe, John">John Doe</dc:creator>
  <dc:creator opf:role="trl">Tom Translator</dc:creator>
  <dc:date opf:event="modification">2011-01-01</dc:date>
  <dc:date opf:event="publication">1999</dc:date>
  <dc:language>en</dc:language>
  <dc:subject>Classics</dc:subject>
  <meta name="cover" content="cover-image"/>
 </metadata>
 <manifest>
  <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
  <item id="p1" href="part1.html" media-type="application/xhtml+xml"/>
  <item id="p2" href="part2.html" media-type="application/xhtml+xml"/>
  <item id="cover-image" href="cover.png" media-type="image/png"/>
  <item id="titlepage" href="titlepage.xhtml" media-type="application/xhtml+xml"/>
 </manifest>
 <spine toc="ncx">
  <itemref idref="titlepage"/>
  <itemref idref="p1"/>
  <itemref idref="p2"/>
 </spine>
 <guide><reference type="cover" href="titlepage.xhtml" title="Cover"/></guide>
</package>`
	// part1 is windows-1252 encoded and declares it.
	part1 := "<?xml version=\"1.0\" encoding=\"windows-1252\"?>\n<html xmlns=\"http://www.w3.org/1999/xhtml\"><head><title/></head><body>" +
		"<h2 class=\"chapter\" id=\"c1\">Chapter \x93One\x94</h2><p class=\"body1\">Caf\xe9 society.</p><div class=\"body2\"/><p>Next <a href=\"part2.html#c1\">part</a>.</p></body></html>"
	part2 := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>p2</title></head><body>
<h2 id="c1">Chapter Two</h2><p>Second &amp; last.</p><p><img src="cover.png" alt=""/>Cover shown inline with text.</p></body></html>`
	titlepage := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Cover</title></head><body><div><img src="cover.png" alt="Cover"/></div></body></html>`
	data := buildZip(t, []file{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", strings.Replace(containerXML, "OEBPS/content.opf", "content.opf", 1)},
		{"content.opf", opf},
		{"toc.ncx", `<ncx><navMap><navPoint><navLabel><text>Should not appear</text></navLabel></navPoint></navMap></ncx>`},
		{"part1.html", part1},
		{"part2.html", part2},
		{"titlepage.xhtml", titlepage},
		{"cover.png", pngBytes},
	})
	doc, _, err := Read(context.Background(), data, rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	m := doc.Meta
	if m.Title != "Old Book" || m.Date != "1999" || m.Lang != "en" {
		t.Errorf("meta = %q %q %q", m.Title, m.Date, m.Lang)
	}
	if got := m.AuthorNames(); !reflect.DeepEqual(got, []string{"John Doe"}) {
		t.Errorf("Authors = %q", got)
	}
	want := strings.Join([]string{
		`H2#c1[Chapter “One”]`,
		`P[Café society.]`,
		`P[Next <#part2-c1>[part].]`,
		`H2#part2-c1[Chapter Two]`,
		`P[Second & last.]`,
		`P[!img(res:cover.png)Cover shown inline with text.]`,
	}, "\n")
	if got := asttest.Dump(doc.Blocks); got != want {
		t.Errorf("blocks:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestReadEPUBErrors(t *testing.T) {
	ctx := context.Background()
	if _, _, err := Read(ctx, []byte("not a zip"), rd.Options{}); err == nil {
		t.Error("expected error for non-zip input")
	}
	noOPF := buildZip(t, []file{{"mimetype", "application/epub+zip"}, {"a.txt", "x"}})
	if _, _, err := Read(ctx, noOPF, rd.Options{}); err == nil || !strings.Contains(err.Error(), "OPF") {
		t.Errorf("missing OPF: err = %v", err)
	}
	emptySpine := buildZip(t, []file{
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", `<package version="3.0"><metadata/><manifest/><spine/></package>`},
	})
	if _, _, err := Read(ctx, emptySpine, rd.Options{}); err == nil {
		t.Error("expected error for an empty spine")
	}
	_, _, err := Read(ctx, epub3(t), rd.Options{Limits: rd.Limits{MaxEntry: 64}})
	if !errors.Is(err, rd.ErrLimit) {
		t.Errorf("limits: err = %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := Read(cancelled, epub3(t), rd.Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: err = %v", err)
	}
}

func TestReadEPUBFallbacks(t *testing.T) {
	// No container.xml, OPF found by extension; only non-linear content.
	data := buildZip(t, []file{
		{"book.opf", `<package version="2.0"><metadata><title>T</title></metadata>
<manifest><item id="a" href="a.xhtml" media-type="application/xhtml+xml"/><item id="i" href="i.png" media-type="image/png"/></manifest>
<spine><itemref idref="a" linear="no"/><itemref idref="i"/><itemref idref="ghost"/></spine></package>`},
		{"a.xhtml", `<html><body><p>Only non-linear text.</p></body></html>`},
		{"i.png", pngBytes},
	})
	doc, warns, err := Read(context.Background(), data, rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := asttest.Dump(doc.Blocks); got != "P[Only non-linear text.]" {
		t.Errorf("blocks = %s", got)
	}
	if doc.Meta.Title != "T" {
		t.Errorf("Title = %q", doc.Meta.Title)
	}
	if len(warns) != 2 {
		t.Errorf("warnings = %q", warns)
	}
}
