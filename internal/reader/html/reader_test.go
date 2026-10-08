package html

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func parseBody(t *testing.T, src string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return findElem(doc, "body", 0)
}

func read(t *testing.T, src string) (*ast.Document, []string) {
	t.Helper()
	doc, warns, err := Read(context.Background(), []byte(src), rd.Options{Name: "page.html"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc.Resources == nil {
		t.Fatal("Resources not initialised")
	}
	return doc, warns
}

const blogPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Understanding Widgets | Example Blog</title>
<meta name="author" content="Jane Doe">
<meta name="description" content="A deep dive into widgets.">
<meta name="keywords" content="widgets, gadgets; tools">
<link rel="stylesheet" href="s.css">
</head>
<body>
<a class="skip-link" href="#main">Skip to content</a>
<header class="site-header"><a href="/">Example Blog</a><nav><ul><li><a href="/a">A</a></li></ul></nav></header>
<div id="page">
<article>
<h1>Understanding Widgets</h1>
<p class="byline">By Jane</p>
<p>Widgets are <b>very</b> <em>useful</em>.  They have   many
 uses<sup class="footnote-ref"><a href="#fn1" id="fnref1">[1]</a></sup>.</p>
<h2 id="setup">Setup <a class="headerlink" href="#setup">¶</a></h2>
<ul>
<li>First item</li>
<li>Second item
  <ul><li>Nested <code>code</code></li></ul>
</li>
<li><input type="checkbox" checked disabled> Done task</li>
</ul>
<pre><code class="language-go">
func main() {
	fmt.Println("hi")
}
</code></pre>
<figure><img src="data:image/png;base64,iVBORw0KGgo=" alt="Logo" width="200" height="100"><figcaption>Figure 1: The logo</figcaption></figure>
<p>See <a href="#setup">setup</a>.</p>
<form class="comment-form"><textarea></textarea><button>Send</button></form>
<section class="footnotes"><hr><ol><li id="fn1"><p>The footnote text. <a href="#fnref1" class="footnote-back">↩\uFE0E</a></p></li></ol></section>
</article>
<aside class="sidebar"><h3>Popular posts</h3></aside>
</div>
<footer>Copyright</footer>
<script>track()</script>
</body>
</html>`

func TestReadBlogPage(t *testing.T) {
	doc, warns := read(t, blogPage)
	m := doc.Meta
	if m.Title != "Understanding Widgets" {
		t.Errorf("Title = %q (site suffix should be stripped)", m.Title)
	}
	if got := m.AuthorNames(); !reflect.DeepEqual(got, []string{"Jane Doe"}) {
		t.Errorf("Authors = %v", got)
	}
	if m.Summary != "A deep dive into widgets." || m.Lang != "en" {
		t.Errorf("Summary/Lang = %q/%q", m.Summary, m.Lang)
	}
	if !reflect.DeepEqual(m.Keywords, []string{"widgets", "gadgets", "tools"}) {
		t.Errorf("Keywords = %q", m.Keywords)
	}
	want := strings.Join([]string{
		`P[By Jane]`,
		`P[Widgets are *[very] _[useful]. They have many uses^note{P[The footnote text.]}.]`,
		`H2#setup[Setup]`,
		`UL{Pl[First item] | Pl[Second item] UL{Pl[Nested ` + "`code`" + `]} | [x] Pl[Done task]}`,
		`Code(go)"func main() {\n\tfmt.Println(\"hi\")\n}"`,
		`Fig(!img(res:image-1.png alt=Logo w=200px h=100px))[The logo]`,
		`P[See <#setup>[setup].]`,
	}, "\n")
	if got := dump(doc.Blocks); got != want {
		t.Errorf("blocks:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if doc.Resources.Len() != 1 {
		t.Errorf("resources = %v", doc.Resources.Names())
	}
	if len(warns) != 0 {
		t.Errorf("warnings = %q", warns)
	}
}

const converterPage = `<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" lang="de" xml:lang="de">
<head>
<meta charset="utf-8" />
<meta name="author" content="Ann Author" />
<meta name="dcterms.date" content="2024-02-01" />
<title>Converted Paper</title>
</head>
<body>
<header id="title-block-header">
<h1 class="title">Converted Paper</h1>
<p class="subtitle">With a subtitle</p>
<p class="author">Ann Author</p>
<p class="date">1 Feb 2024</p>
<div class="abstract"><div class="abstract-title">Abstract</div><p>We study things.</p></div>
</header>
<nav id="TOC" role="doc-toc"><ul><li><a href="#intro" id="toc-intro">Intro</a></li></ul></nav>
<section id="intro" class="level1">
<h1>Introduction</h1>
<p>Inline <span class="math inline">\(a+b\)</span>:</p>
<p><span class="math display">\[\int_0^1 f\,dx\]</span></p>
<p>Ref<a href="#fn1" class="footnote-ref" id="fnref1" role="doc-noteref"><sup>1</sup></a>.</p>
<div class="sourceCode" id="cb1"><pre class="sourceCode python"><code class="sourceCode python"><span id="cb1-1"><a href="#cb1-1" aria-hidden="true" tabindex="-1"></a><span class="bu">print</span>(<span class="st">&quot;x&quot;</span>)</span>
<span id="cb1-2"><a href="#cb1-2" aria-hidden="true" tabindex="-1"></a>x <span class="op">=</span> <span class="dv">1</span></span></code></pre></div>
</section>
<section id="footnotes" class="footnotes footnotes-end-of-document" role="doc-endnotes">
<hr />
<ol>
<li id="fn1"><p>First note.<a href="#fnref1" class="footnote-back" role="doc-backlink">↩\uFE0E</a></p></li>
</ol>
</section>
</body>
</html>`

func TestReadConverterStandalone(t *testing.T) {
	doc, _ := read(t, converterPage)
	m := doc.Meta
	if m.Title != "Converted Paper" || m.Subtitle != "With a subtitle" || m.Date != "2024-02-01" || m.Lang != "de" {
		t.Errorf("meta = title %q subtitle %q date %q lang %q", m.Title, m.Subtitle, m.Date, m.Lang)
	}
	if got := dump(m.Abstract); got != "P[We study things.]" {
		t.Errorf("Abstract = %s", got)
	}
	want := strings.Join([]string{
		`H1#intro[Introduction]`,
		`P[Inline $a+b$:]`,
		`$$\int_0^1 f\,dx$$`,
		`P[Ref^note{P[First note.]}.]`,
		`Code(python)"print(\"x\")\nx = 1"`,
	}, "\n")
	if got := dump(doc.Blocks); got != want {
		t.Errorf("blocks:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestReadScholarMeta(t *testing.T) {
	doc, _ := read(t, `<html><head><title>Journal site - Article</title>
<meta name="citation_title" content="On the Theory of Things">
<meta name="citation_author" content="Smith, John">
<meta name="citation_author_institution" content="University of Somewhere">
<meta name="citation_author_orcid" content="https://orcid.org/0000-0002-1825-0097">
<meta name="citation_author" content="Bērziņa, Anna">
<meta name="citation_publication_date" content="2021/03/04">
<meta name="citation_journal_title" content="Journal of Things">
<meta name="citation_doi" content="10.1000/xyz">
<meta name="citation_firstpage" content="10"><meta name="citation_lastpage" content="20">
<meta name="DC.subject" content="theory; things">
<meta name="DC.publisher" content="Example Press">
</head><body><p>x</p></body></html>`)
	m := doc.Meta
	if m.Title != "On the Theory of Things" {
		t.Errorf("Title = %q", m.Title)
	}
	wantAuthors := []ast.Author{
		{Name: "John Smith", Affiliations: []string{"University of Somewhere"}, ORCID: "0000-0002-1825-0097"},
		{Name: "Anna Bērziņa"},
	}
	if !reflect.DeepEqual(m.Authors, wantAuthors) {
		t.Errorf("Authors = %+v", m.Authors)
	}
	if m.Date != "2021-03-04" || m.Organization != "Example Press" {
		t.Errorf("Date/Org = %q/%q", m.Date, m.Organization)
	}
	if !reflect.DeepEqual(m.Keywords, []string{"theory", "things"}) {
		t.Errorf("Keywords = %q", m.Keywords)
	}
	if m.Extra["journal"] != "Journal of Things" || m.Extra["doi"] != "10.1000/xyz" || m.Extra["pages"] != "10–20" {
		t.Errorf("Extra = %v", m.Extra)
	}
}

func TestReadTitleHandling(t *testing.T) {
	tests := []struct {
		name, head, body, title, first string
	}{
		{"suffix kept when h1 differs", `<title>Home | Site</title>`, `<h1>Welcome</h1><p>x</p>`, "Home | Site", "H1[Welcome]"},
		{"dash suffix stripped", `<title>A - B — Site</title>`, `<main><h1>A - B</h1><p>x</p></main>`, "A - B", "P[x]"},
		{"og fallback", `<meta property="og:title" content="OG Title">`, `<p>x</p>`, "OG Title", "P[x]"},
		{"dublin core wins", `<title>T</title><meta name="DC.title" content="DC Title"><meta name="DC.creator" content="Doe, J.">`, `<p>x</p>`, "DC Title", "P[x]"},
		{"lone h1 promoted", ``, `<h1>Only Heading</h1><h2>Sub</h2>`, "Only Heading", "H2[Sub]"},
		{"several h1 kept", ``, `<h1>One</h1><h1>Two</h1>`, "", "H1[One]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _ := read(t, "<html><head>"+tt.head+"</head><body>"+tt.body+"</body></html>")
			if doc.Meta.Title != tt.title {
				t.Errorf("Title = %q, want %q", doc.Meta.Title, tt.title)
			}
			if len(doc.Blocks) == 0 {
				t.Fatal("no blocks")
			}
			if got := dump(doc.Blocks[:1]); got != tt.first {
				t.Errorf("first block = %s, want %s", got, tt.first)
			}
		})
	}
}

func TestReadContentRoot(t *testing.T) {
	t.Run("main element", func(t *testing.T) {
		doc, _ := read(t, `<body><div class="menu">Menu text</div><main><p>Main text</p><nav>In-page nav</nav></main><div>Sidebar text</div></body>`)
		if got := dump(doc.Blocks); got != "P[Main text]" {
			t.Errorf("got %s", got)
		}
	})
	t.Run("dominant article", func(t *testing.T) {
		long := strings.Repeat("Long article text. ", 40)
		doc, _ := read(t, `<body><div>Related links</div><article><p>`+long+`</p><footer><p>Tags: x</p></footer></article><article><p>Teaser</p></article></body>`)
		got := dump(doc.Blocks)
		if strings.Contains(got, "Related") || strings.Contains(got, "Teaser") || !strings.Contains(got, "Tags: x") {
			t.Errorf("got %s", got)
		}
	})
	t.Run("listing page keeps all articles", func(t *testing.T) {
		doc, _ := read(t, `<body><article><p>First post summary</p></article><article><p>Second post summary</p></article></body>`)
		if got := dump(doc.Blocks); got != "P[First post summary]\nP[Second post summary]" {
			t.Errorf("got %s", got)
		}
	})
	t.Run("body chrome removed", func(t *testing.T) {
		doc, _ := read(t, `<body><header><p>Site name</p></header><div role="navigation">Nav</div><p>Content</p><aside role="complementary">Ads</aside><footer><p>Footer</p></footer></body>`)
		if got := dump(doc.Blocks); got != "P[Content]" {
			t.Errorf("got %s", got)
		}
	})
}

func TestReadCharsets(t *testing.T) {
	// "Café “quoted”" in windows-1252.
	cp1252 := []byte("Caf\xe9 \x93quoted\x94")
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"meta charset", append(append([]byte(`<html><head><meta charset="windows-1252"></head><body><p>`), cp1252...), []byte(`</p></body></html>`)...), "P[Café “quoted”]"},
		{"http-equiv latin1", append(append([]byte(`<html><head><meta http-equiv="Content-Type" content="text/html; charset=iso-8859-1"></head><body><p>`), cp1252...), []byte(`</p></body></html>`)...), "P[Café “quoted”]"},
		{"no declaration falls back to windows-1252", append(append([]byte(`<p>`), cp1252...), []byte(`</p>`)...), "P[Café “quoted”]"},
		{"utf-8 with bom ignores wrong meta", []byte("\xef\xbb\xbf<meta charset=\"iso-8859-1\"><p>Rīga ✓</p>"), "P[Rīga ✓]"},
		{"utf-16 bom", utf16le("<p>Hi ā</p>"), "P[Hi ā]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _, err := Read(context.Background(), tt.data, rd.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if got := dump(doc.Blocks); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func utf16le(s string) []byte {
	out := []byte{0xff, 0xfe}
	for _, r := range s {
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}

func TestReadRobustness(t *testing.T) {
	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := Read(ctx, []byte("<p>x</p>"), rd.Options{}); !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("deep nesting", func(t *testing.T) {
		src := strings.Repeat("<div><span>", 3000) + "deep" + strings.Repeat("</span></div>", 3000)
		if _, _, err := Read(context.Background(), []byte(src), rd.Options{}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("table cell limit", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("<table>")
		for i := 0; i < 50; i++ {
			sb.WriteString("<tr><td>a</td><td>b</td></tr>")
		}
		sb.WriteString("</table>")
		doc, warns, err := Read(context.Background(), []byte(sb.String()), rd.Options{Limits: rd.Limits{MaxTableCells: 20}})
		if err != nil {
			t.Fatal(err)
		}
		tbl, ok := doc.Blocks[0].(*ast.Table)
		if !ok || len(tbl.Body) != 10 {
			t.Fatalf("expected a truncated 10-row table, got %s", dump(doc.Blocks))
		}
		if len(warns) == 0 || !strings.Contains(warns[0], "truncated") {
			t.Errorf("warnings = %q", warns)
		}
	})
	t.Run("garbage input", func(t *testing.T) {
		for _, src := range []string{"", "\x00\x01\x02", "<<<>>>", "<table><td>x", "<math><mfrac>", "<ul><li><ol><li>", strings.Repeat("<b>", 1000)} {
			if _, _, err := Read(context.Background(), []byte(src), rd.Options{}); err != nil {
				t.Errorf("%q: %v", src, err)
			}
		}
	})
}

func TestReadNodeHooks(t *testing.T) {
	src := `<p><img src="images/a.png" alt="A"> <img src="missing.png" alt="gone"> <a href="other.xhtml#sec">x</a> <a href="#local">y</a></p><h2 id="local">L</h2>`
	root := parseBody(t, src)
	opts := ConvertOptions{
		ResolveImage: func(s string) string {
			if s == "images/a.png" {
				return "res:OEBPS/images/a.png"
			}
			return ""
		},
		RewriteLink: func(h string) string {
			if strings.HasPrefix(h, "other.xhtml#") {
				return "#other-" + h[len("other.xhtml#"):]
			}
			if strings.HasPrefix(h, "#") {
				return "#p-" + h[1:]
			}
			return h
		},
		RewriteID: func(id string) string { return "p-" + id },
	}
	got, _ := ReadNode(context.Background(), root, opts)
	want := "P[!img(res:OEBPS/images/a.png alt=A) gone <#other-sec>[x] <#p-local>[y]]\nH2#p-local[L]"
	if d := dump(got); d != want {
		t.Errorf("\ngot:  %s\nwant: %s", d, want)
	}
}

const wikiPage = `<html><head><title>Riga - Encyclopedia</title></head><body>
<div id="mw-page-base"></div>
<div id="content" class="mw-body" role="main">
<h1 id="firstHeading" class="firstHeading"><span class="mw-page-title-main">Riga</span></h1>
<div id="bodyContent">
<div id="siteSub" class="noprint">From the encyclopedia</div>
<div class="hatnote navigation-not-searchable">For other uses, see <a href="/wiki/Riga_(disambiguation)">Riga (disambiguation)</a>.</div>
<table class="infobox ib-settlement vcard"><tbody><tr><th colspan="2" class="infobox-above">Riga</th></tr>
<tr><th scope="row" class="infobox-label">Country</th><td class="infobox-data"><a href="/wiki/Latvia">Latvia</a></td></tr>
<tr><th scope="row">Population</th><td>605,273<sup id="cite_ref-pop_1-0" class="reference"><a href="#cite_note-pop-1">[1]</a></sup></td></tr></tbody></table>
<p><b>Riga</b> (<span class="rt-commentedText"><span class="IPA">/ˈriːɡə/</span></span>; <a href="/wiki/Latvian">Latvian</a>: <i lang="lv">Rīga</i>) is the capital<sup id="cite_ref-pop_1-1" class="reference"><a href="#cite_note-pop-1">[1]</a></sup> of Latvia.</p>
<div id="toc" class="toc"><div class="toctitle"><h2 id="mw-toc-heading">Contents</h2></div><ul><li><a href="#History">1 History</a></li></ul></div>
<h2><span class="mw-headline" id="History">History</span><span class="mw-editsection"><span class="mw-editsection-bracket">[</span><a href="/w/index.php?title=Riga&amp;action=edit&amp;section=1">edit</a><span class="mw-editsection-bracket">]</span></span></h2>
<p>Area <span class="mwe-math-element"><span class="mwe-math-mathml-inline mwe-math-mathml-a11y" style="display: none;"><math xmlns="http://www.w3.org/1998/Math/MathML" alttext="{\displaystyle A=\pi r^{2}}"><semantics><mrow><mi>A</mi></mrow><annotation encoding="application/x-tex">{\displaystyle A=\pi r^{2}}</annotation></semantics></math></span><img src="https://example.org/api/rest_v1/media/math/render/svg/abc" class="mwe-math-fallback-image-inline" aria-hidden="true" alt="{\displaystyle A=\pi r^{2}}"></span>.</p>
<div class="thumb tright"><div class="thumbinner"><a href="/wiki/File:Riga.jpg" class="image"><img alt="" src="//img.example.org/riga.jpg" width="220" height="147" srcset="//img.example.org/330px-riga.jpg 1.5x, //img.example.org/440px-riga.jpg 2x"></a><div class="thumbcaption">Old town</div></div></div>
<a class="card" href="/x"><div class="card-title">Card title</div><p>Card desc</p></a>
<ul><li>Before<p>Block in li</p>After text</li></ul>
<h2><span class="mw-headline" id="References">References</span></h2>
<div class="reflist"><div class="mw-references-wrap"><ol class="references">
<li id="cite_note-pop-1"><span class="mw-cite-backlink">^ <a href="#cite_ref-pop_1-0"><sup><i><b>a</b></i></sup></a> <a href="#cite_ref-pop_1-1"><sup><i><b>b</b></i></sup></a></span> <span class="reference-text"><a rel="nofollow" class="external text" href="https://stat.gov.lv">"Population"</a>. CSB.</span></li>
</ol></div></div>
<div class="printfooter">Retrieved from x</div>
<div id="catlinks" class="catlinks"><a href="/wiki/Special:Categories">Categories</a></div>
</div></div>
<div id="mw-navigation"><h2>Navigation menu</h2></div>
<div id="footer" role="contentinfo">Footer text</div>
</body></html>
`

func TestReadWikiStylePage(t *testing.T) {
	doc, warns := read(t, wikiPage)
	if doc.Meta.Title != "Riga" {
		t.Errorf("Title = %q", doc.Meta.Title)
	}
	note := "^note{P[<https://stat.gov.lv>[\"Population\"]. CSB.]}"
	want := strings.Join([]string{
		"P[For other uses, see </wiki/Riga_(disambiguation)>[Riga (disambiguation)].]",
		"T{cols:0,0; head: (c2 Pl[Riga]); body: (Pl[Country]) (Pl[</wiki/Latvia>[Latvia]]) / (Pl[Population]) (Pl[605,273" + note + "])}",
		"P[*[Riga] (/ˈriːɡə/; </wiki/Latvian>[Latvian]: _[Rīga]) is the capital" + note + " of Latvia.]",
		"H2#history[History]",
		"P[Area ${\\displaystyle A=\\pi r^{2}}$.]",
		"Fig(!img(https://img.example.org/440px-riga.jpg w=220px h=147px))[Old town]",
		"P[Card title]",
		"P[Card desc]",
		"UL~{Pl[Before] P[Block in li] Pl[After text]}",
		"H2[References]",
	}, "\n")
	if got := dump(doc.Blocks); got != want {
		t.Errorf("blocks:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if len(warns) != 0 {
		t.Errorf("warnings = %q", warns)
	}
}

func TestReadPrunesDanglingLinks(t *testing.T) {
	doc, _ := read(t, `<body><nav id="menu"><a href="#x">nav</a></nav><p><a href="#top">Top</a>, <a href="#menu">menu</a>, <b><a href="#s">S</a></b> and <a href="#p">para</a>.</p><h2 id="s">S</h2><p id="p">x</p></body>`)
	want := "P[Top, menu, *[<#s>[S]] and <#p>[para].]\nH2#s[S]\nP[span#p[]x]"
	if got := dump(doc.Blocks); got != want {
		t.Errorf("\ngot:  %s\nwant: %s", got, want)
	}
}
