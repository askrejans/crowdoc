package html

import (
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

func TestFragment(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		// Whitespace and text
		{"collapse across inline boundaries", `<p>  a <b> b </b>  c </p>`, `P[a *[b ]c]`},
		{"nbsp kept", `<p>a&nbsp;&nbsp;b</p>`, "P[a  b]"},
		{"line breaks", "<p>line one <br>\n  line two</p>", `P[line one\nline two]`},
		{"bare text around blocks", `<div>intro text<p>para</p>tail <b>bold</b></div>`, "P[intro text]\nP[para]\nP[tail *[bold]]"},
		{"empty formatting dropped", `<p>x<b> </b><i class="icon"></i>y</p>`, `P[x y]`},
		{"inline formatting", `<p><strong>s</strong><em>e</em><cite>c</cite><u>u</u><del>d</del><sup>2</sup><sub>i</sub><mark>m</mark><small>sm</small><abbr title="x">AB</abbr></p>`, `P[*[s]_[e]_[c]u[u]~[d]^[2],[i]=[m]smAB]`},
		{"code and kbd", `<p><code>x  = 1</code> <kbd>Ctrl</kbd>+<kbd>C</kbd> <samp>out</samp></p>`, "P[`x = 1` span.kbd[Ctrl]+span.kbd[C] `out`]"},
		{"nested kbd", `<p><kbd><kbd>Alt</kbd>+<kbd>F4</kbd></kbd></p>`, `P[span.kbd[Alt]+span.kbd[F4]]`},
		{"quotes", `<p><q>outer</q></p>`, `P[“outer”]`},
		{"code link", `<p><code><a href="https://x.test/f">f()</a></code></p>`, "P[<https://x.test/f>[`f()`]]"},
		{"office css formatting", `<b style="font-weight:normal" id="docs-internal-guid-1"><p><span style="font-weight:700">Bold</span> <span style="font-style:italic">it</span> <span style="text-decoration:line-through">gone</span> <span style="vertical-align:super">sup</span> <span style="font-family:'Courier New'">mono</span></p></b>`, "P[*[Bold] _[it] ~[gone] ^[sup] `mono`]"},
		{"semantic span classes", `<p><span class="smallcaps">Sc</span> <span class="underline">U</span> <span class="mark">M</span></p>`, `P[sc[Sc] u[U] =[M]]`},
		{"ruby without rp", `<p><ruby>漢<rt>kan</rt></ruby></p>`, `P[漢(kan)]`},

		// Headings and anchors
		{"heading id sanitised", `<h2 id="Über Uns">Title</h2>`, `H2#uber-uns[Title]`},
		{"heading anchor child", `<h3><a id="sec"></a>Section</h3><p><a href="#sec">go</a></p>`, "H3#sec[Section]\nP[<#sec>[go]]"},
		{"section id moves to heading", `<section id="part"><h2>Part</h2><p>x</p></section><p><a href="#part">p</a></p>`, "H2#part[Part]\nP[x]\nP[<#part>[p]]"},
		{"paragraph anchor", `<p id="para1">Target</p><p><a href="#para1">l</a></p>`, "P[span#para1[]Target]\nP[<#para1>[l]]"},
		{"unreferenced ids dropped", `<p id="x">a</p><div id="y"><p>b</p></div>`, "P[a]\nP[b]"},
		{"permalink decoration dropped", `<h2 id="s">Setup<a class="anchor" href="#s">#</a></h2>`, `H2#s[Setup]`},
		{"empty heading dropped", `<h2> </h2><p>x</p>`, `P[x]`},

		// Links
		{"links", `<p><a href="https://e.test" title="T">ext</a> <a href="javascript:void(0)">js</a> <a href="#Top Part">in</a> <a href="mailto:a@b.test">mail</a> <a href="https://e.test"></a></p>`, "P[<https://e.test>[ext] js <#top-part>[in] <mailto:a@b.test>[mail]]"},
		{"linked image figure", `<p><a href="big.png"><img src="small.png" alt="S"></a></p>`, `Fig(!img(small.png alt=S))`},

		// Lists
		{"nested lists", `<ul><li>a<ul><li>b</li></ul></li><li>c</li></ul>`, `UL{Pl[a] UL{Pl[b]} | Pl[c]}`},
		{"stray nested list", `<ul><li>a</li><ul><li>b</li></ul><li>c</li></ul>`, `UL{Pl[a] UL{Pl[b]} | Pl[c]}`},
		{"ordered styles", `<ol type="I" start="4"><li>x</li></ol><ol style="list-style-type: lower-alpha"><li value="2">y</li></ol>`, "OL(4,4){Pl[x]}\nOL(2,1){Pl[y]}"},
		{"loose list", `<ol><li><p>one</p></li><li><p>two</p><p>more</p></li></ol>`, `OL(0,0)~{P[one] | P[two] P[more]}`},
		{"task list", `<ul class="contains-task-list"><li class="task-list-item"><input type="checkbox" disabled> open</li><li><p><input type="checkbox" checked> done</p></li></ul>`, `UL~{[ ] Pl[open] | [x] P[done]}`},
		{"definition list", `<dl><dt>Term</dt><dd>Def one</dd><dd><p>Def two</p></dd><div><dt>A</dt><dt>B</dt><dd>shared</dd></div></dl>`, `DL{Term: Pl[Def one] P[Def two] | A\nB: Pl[shared]}`},

		// Block structures
		{"blockquote with cite", `<blockquote><p>Quote.</p><cite>Someone</cite></blockquote>`, `BQ{P[Quote.] P[— _[Someone]]}`},
		{"blockquote with dashed footer", `<blockquote>Bare quote<footer>— A. Person</footer></blockquote>`, `BQ{P[Bare quote] P[— A. Person]}`},
		{"address", `<address>Street 1<br>City<br>Country</address>`, `LB{Street 1 / City / Country}`},
		{"center", `<center>mid</center>`, `Div.center{P[mid]}`},
		{"hr", `<p>a</p><hr><p>b</p>`, "P[a]\nHR\nP[b]"},
		{"details", `<details><summary>More <b>info</b></summary><p>Hidden body</p></details>`, `Div(More *[info]){P[Hidden body]}`},
		{"admonition", `<div class="admonition warning"><p class="admonition-title">Careful</p><p>Body</p></div>`, `Div.warning(Careful){P[Body]}`},
		{"markdown alert", `<div class="markdown-alert markdown-alert-important"><p class="markdown-alert-title"><svg></svg>Important</p><p>Read this.</p></div>`, `Div.important(Important){P[Read this.]}`},
		{"alert component", `<div class="alert alert-danger" role="alert">Stop</div>`, `Div.danger{P[Stop]}`},
		{"aside note", `<aside class="note"><p>Side note</p></aside>`, `Div.note{P[Side note]}`},
		{"callout with header", `<div class="callout callout-style-default callout-tip callout-titled"><div class="callout-header"><div class="callout-icon-container"><i class="callout-icon"></i></div><div class="callout-title-container flex-fill">Pro tip</div></div><div class="callout-body-container callout-body"><p>Use it.</p></div></div>`, `Div.tip(Pro tip){P[Use it.]}`},
		{"details callout", `<details class="note"><summary>Folded</summary><p>x</p></details>`, `Div.note(Folded){P[x]}`},
		{"note paragraph", `<p class="note">Remember.</p>`, `Div.note{P[Remember.]}`},
		{"non-callout class", `<div class="hatnote">Main article</div>`, `P[Main article]`},

		// Dropped content
		{"hidden content", `<p>a<span hidden>h</span><span aria-hidden="true">x</span><span style="display: none">n</span><span class="sr-only">sr</span>b</p>`, `P[ab]`},
		{"hidden until found kept", `<div hidden="until-found">found</div>`, `P[found]`},
		{"forms dropped", `<form><input name="q"><button>Go</button></form><p>after</p>`, `P[after]`},
		{"scripts dropped", `<script>var x=1</script><style>p{}</style><p>t</p>`, `P[t]`},

		// Code blocks
		{"pre code language", "<pre><code class=\"language-rust\">\nfn main() {}\n\n</code></pre>", `Code(rust)"fn main() {}"`},
		{"pre whitespace exact", "<pre>  a\n\tb  \n</pre>", `Code()"  a\n\tb  "`},
		{"brush class", `<pre class="brush: js; gutter: false">x</pre>`, `Code(js)"x"`},
		{"highlight-source wrapper", `<div class="highlight highlight-source-python"><pre>print(1)</pre></div>`, `Code(python)"print(1)"`},
		{"rouge wrapper", `<div class="language-ruby highlighter-rouge"><div class="highlight"><pre class="highlight"><code>puts 1</code></pre></div></div>`, `Code(ruby)"puts 1"`},
		{"line numbers stripped", `<pre><code><span class="lineno">1 </span>a
<span class="lineno">2 </span>b</code></pre>`, `Code()"a\nb"`},
		{"line gutter table", `<table class="highlighttable"><tr><td class="linenos"><div class="linenodiv"><pre>1
2</pre></div></td><td class="code"><div class="highlight"><pre><span></span>x = 1
y = 2</pre></div></td></tr></table>`, `Code()"x = 1\ny = 2"`},
		{"br inside pre", `<pre>a<br>b</pre>`, `Code()"a\nb"`},

		// Math
		{"mathml inline", `<p>So <math><msup><mi>x</mi><mn>2</mn></msup></math> holds.</p>`, `P[So $x^{2}$ holds.]`},
		{"mathml display", `<p>Then<math display="block"><mi>y</mi></math>done</p>`, "P[Then]\n$$y$$\nP[done]"},
		{"mathml alone", `<p><math><mi>z</mi></math></p>`, `$$z$$`},
		{"rendered math annotation", `<p>K <span class="katex"><span class="katex-mathml"><math><semantics><mrow><mi>a</mi></mrow><annotation encoding="application/x-tex">\alpha</annotation></semantics></math></span><span class="katex-html" aria-hidden="true"><span class="base">α</span></span></span> end</p>`, `P[K $\alpha$ end]`},
		{"rendered display math", `<p><span class="katex-display"><span class="katex"><span class="katex-mathml"><math display="block"><semantics><mi>b</mi><annotation encoding="application/x-tex">\beta^2</annotation></semantics></math></span></span></span></p>`, `$$\beta^2$$`},
		{"mjx container assistive mathml", `<p>M <mjx-container class="MathJax" jax="CHTML"><mjx-math aria-hidden="true"><mjx-mi>x</mjx-mi></mjx-math><mjx-assistive-mml><math><mi>x</mi><mo>+</mo><mn>1</mn></math></mjx-assistive-mml></mjx-container></p>`, `P[M $x+1$]`},
		{"mjx container data-latex", `<mjx-container display="true" data-latex="\sum_i i"></mjx-container>`, `$$\sum_i i$$`},
		{"math/tex scripts", `<p>V <span class="MathJax_Preview">x</span><span class="MathJax" id="MathJax-Element-1-Frame">x</span><script type="math/tex" id="MathJax-Element-1">x_1</script> and</p><script type="math/tex; mode=display">E=mc^2</script>`, "P[V $x_1$ and]\n$$E=mc^2$$"},
		{"unsafe tex kept as code", `<p>x <script type="math/tex">\input{/etc/passwd}</script></p>`, "P[x `\\input{/etc/passwd}`]"},
		{"monospace paragraphs become code", `<p><span style="font-family: 'Courier New'">a = 1</span></p><p><span style="font-family:monospace">&nbsp;&nbsp;b = 2</span></p><p>text</p>`, "Code()\"a = 1\\n  b = 2\"\nP[text]"},
		{"class styles", `<style>.c1{font-weight:700} span.c2 { vertical-align: super } .c3{position:relative;top:0.2em;vertical-align:baseline} @media print { .c1 { color: red } } .hide{display:none}</style><p><span class="c1">B</span>x<span class="c2">2</span>H<span class="c3">2</span><span class="hide">gone</span></p>`, "P[*[B]x^[2]H,[2]]"},

		// Media
		{"svg dropped", `<p>a<svg><text>t</text></svg>b</p>`, `P[ab]`},
		{"video link", `<p><video src="https://v.test/clip.mp4" controls>no support</video></p>`, `P[<https://v.test/clip.mp4>[clip.mp4]]`},
		{"image attributes", `<p>Icon <img src="i.png" alt=" an  icon " width="16" height="16px"> here</p>`, `P[Icon !img(i.png alt=an icon w=16px h=16px) here]`},
		{"tracking pixel", `<p>t<img src="https://t.test/p.gif" width="1" height="1"></p>`, `P[t]`},
		{"srcset largest width", `<p><img src="s.jpg" srcset="s.jpg 320w, m.jpg 800w, l.jpg 1600w" alt="A"></p>`, `Fig(!img(l.jpg alt=A))`},
		{"srcset density", `<p><img src="a.png" srcset="a@2x.png 2x, a@3x.png 3x"></p>`, `Fig(!img(a@3x.png))`},
		{"picture prefers jpeg", `<picture><source type="image/webp" srcset="p.webp 2000w"><source type="image/jpeg" srcset="p.jpg 1000w"><img src="p-small.jpg" alt="P"></picture>`, `Fig(!img(p.jpg alt=P))`},
		{"lazy image", `<p><img src="data:image/gif;base64,R0lGODlhAQABAAAAACw=" data-src="real.png" alt="R"></p>`, `Fig(!img(real.png alt=R))`},
		{"missing source keeps alt", `<p>x <img alt="diagram"> y</p>`, `P[x diagram y]`},
		{"figure with table", `<figure id="t1"><figcaption>Table 3. Scores</figcaption><table><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></table></figure>`, `T{cap:Scores; cols:0,0; head: (Pl[A]) (Pl[B]); body: (Pl[1]) (Pl[2])}`},
		{"figure with code", `<figure><pre><code class="language-sh">ls</code></pre><figcaption>Listing</figcaption></figure>`, `Code(sh)"ls"[Listing]`},
		{"figure with svg keeps caption", `<figure><svg></svg><figcaption>Chart</figcaption></figure>`, `P[Chart]`},
		{"figure several images", `<figure><img src="a.png"><img src="b.png"><figcaption>Both</figcaption></figure>`, `P[!img(a.png)!img(b.png)]` + "\n" + `P[Both]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, _ := Fragment(tt.in, ast.NewResources())
			if got := dump(blocks); got != tt.want {
				t.Errorf("\ngot:  %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestFragmentWarnings(t *testing.T) {
	_, warns := Fragment(`<p>a<svg></svg><iframe src="x"></iframe><canvas></canvas></p>`, nil)
	joined := strings.Join(warns, "\n")
	for _, want := range []string{"SVG", "iframe", "canvas"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing warning about %s in %q", want, joined)
		}
	}
}

func TestFragmentDataURI(t *testing.T) {
	res := ast.NewResources()
	in := `<p><img src="data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%2F%3E" alt="svg"></p>` +
		`<p><img src="data:image/png;base64,iVBO Rw0K
GgoAAAANSUhEUg==" alt="png"></p>` +
		`<p><img src="data:image/png;base64,!!!" alt="bad"></p>` +
		`<p><img src="data:text/html,<b>x</b>" alt="html"></p>`
	blocks, warns := Fragment(in, res)
	want := "Fig(!img(res:image-1.svg alt=svg))\nFig(!img(res:image-2.png alt=png))\nP[bad]\nP[html]"
	if got := dump(blocks); got != want {
		t.Errorf("\ngot:  %s\nwant: %s", got, want)
	}
	svg, ok := res.Get("res:image-1.svg")
	if !ok || svg.MediaType != "image/svg+xml" || string(svg.Data) != `<svg xmlns="http://www.w3.org/2000/svg"/>` {
		t.Errorf("svg resource = %+v", svg)
	}
	png, ok := res.Get("image-2.png")
	if !ok || !strings.HasPrefix(string(png.Data), "\x89PNG\r\n\x1a\n") {
		t.Errorf("png resource not decoded: %+v", png)
	}
	if len(warns) != 2 {
		t.Errorf("warnings = %q", warns)
	}
}

func TestFragmentTables(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"spans and sections",
			`<table><thead><tr><th rowspan="2">Name</th><th colspan="2">Score</th></tr><tr><th>A</th><th>B</th></tr></thead>
			<tbody><tr><td>x</td><td>1</td><td rowspan="0">z</td></tr><tr><td>y</td><td>2</td></tr></tbody>
			<tfoot><tr><td>Total</td><td colspan="2">3</td></tr></tfoot></table>`,
			`T{cols:0,0,0; head: (r2 Pl[Name]) (c2 Pl[Score]) / (Pl[A]) (Pl[B]); body: (Pl[x]) (Pl[1]) (r2 Pl[z]) / (Pl[y]) (Pl[2]); foot: (Pl[Total]) (c2 Pl[3])}`},
		{"role=table keeps a one-cell table",
			`<table role="table"><tr><td><p>a</p><p>b</p></td></tr></table>`,
			`T{cols:0; body: (P[a] P[b])}`},
		{"th-only first row is header",
			`<table><tr><th>H1</th><th>H2</th></tr><tr><td>a</td><td>b</td></tr></table>`,
			`T{cols:0,0; head: (Pl[H1]) (Pl[H2]); body: (Pl[a]) (Pl[b])}`},
		{"bold first row is header",
			`<table><tr><td><b>H1</b></td><td><strong>H2</strong></td></tr><tr><td>a</td><td>b</td></tr></table>`,
			`T{cols:0,0; head: (Pl[H1]) (Pl[H2]); body: (Pl[a]) (Pl[b])}`},
		{"row headers stay in body",
			`<table><tr><th>A</th><td>1</td></tr><tr><th>B</th><td>2</td></tr></table>`,
			`T{cols:0,0; body: (Pl[A]) (Pl[1]) / (Pl[B]) (Pl[2])}`},
		{"column alignment",
			`<table><tr><th>k</th><th>v</th></tr><tr><td>a</td><td style="text-align: right">1</td></tr><tr><td align="center">b</td><td align="right">2</td></tr></table>`,
			`T{cols:0,3; head: (Pl[k]) (Pl[v]); body: (Pl[a]) (Pl[1]) / (a2 Pl[b]) (Pl[2])}`},
		{"caption label stripped",
			`<table><caption><b>Table 1:</b> Data</caption><tr><td>a</td><td>b</td></tr></table>`,
			`T{cap:Data; cols:0,0; body: (Pl[a]) (Pl[b])}`},
		{"cell paragraphs",
			`<table><tr><td><p>one</p></td><td><p>a</p><p>b</p></td></tr><tr><td>x</td><td>y</td></tr></table>`,
			`T{cols:0,0; body: (Pl[one]) (P[a] P[b]) / (Pl[x]) (Pl[y])}`},
		{"single cell layout table",
			`<table><tr><td><h2>Title</h2><p>Body</p></td></tr></table>`,
			"H2[Title]\nP[Body]"},
		{"nested layout tables",
			`<table width="100%"><tr><td><p>Left</p></td><td><table><tr><td>in</td><td>ner</td></tr><tr><td>1</td><td>2</td></tr></table></td></tr></table>`,
			"P[Left]\nT{cols:0,0; body: (Pl[in]) (Pl[ner]) / (Pl[1]) (Pl[2])}"},
		{"presentation role",
			`<table role="presentation"><tr><td>a</td><td>b</td></tr></table>`,
			"P[a]\nP[b]"},
		{"dataframe table",
			`<div><style scoped>.dataframe tbody tr th {vertical-align: top;}</style>
<table border="1" class="dataframe">
  <thead>
    <tr style="text-align: right;"><th></th><th>name</th><th>value</th></tr>
  </thead>
  <tbody>
    <tr><th>0</th><td>alpha</td><td>1.5</td></tr>
    <tr><th>1</th><td>beta</td><td>NaN</td></tr>
  </tbody>
</table>
<p>2 rows × 2 columns</p></div>`,
			"T{cols:0,0,0; head: (a3 ) (a3 Pl[name]) (a3 Pl[value]); body: (Pl[0]) (Pl[alpha]) (Pl[1.5]) / (Pl[1]) (Pl[beta]) (Pl[NaN])}\nP[2 rows × 2 columns]"},
		{"empty table with caption",
			`<table><caption>Nothing</caption></table>`,
			`P[Nothing]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, _ := Fragment(tt.in, nil)
			if got := dump(blocks); got != tt.want {
				t.Errorf("\ngot:  %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestFragmentFootnotes(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"footnote-ref sup with section",
			`<p>Text<sup class="footnote-ref"><a href="#fn1" id="fnref1">[1]</a></sup>.</p>
<hr class="footnotes-sep"><section class="footnotes"><ol class="footnotes-list"><li id="fn1" class="footnote-item"><p>Note body <a href="#fnref1" class="footnote-backref">↩︎</a></p></li></ol></section>`,
			`P[Text^note{P[Note body]}.]`},
		{"colon footnote ids",
			`<p>Claim<sup id="fnref:1"><a href="#fn:1" class="footnote-ref" role="doc-noteref">1</a></sup></p>
<div class="footnotes" role="doc-endnotes"><hr><ol><li id="fn:1"><p>Source.&#160;<a href="#fnref:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a></p></li></ol></div>`,
			"P[Claim^note{P[Source. ]}]"},
		{"reversefootnote backlinks",
			`<p>Word<sup id="fnref:a" role="doc-noteref"><a href="#fn:a" class="footnote" rel="footnote">1</a></sup></p><div class="footnotes" role="doc-endnotes"><ol><li id="fn:a" role="doc-endnote"><p>Backlink note. <a href="#fnref:a" class="reversefootnote" role="doc-backlink">&#8617;</a></p></li></ol></div>`,
			`P[Word^note{P[Backlink note.]}]`},
		{"wiki references",
			`<p>Fact.<sup id="cite_ref-1" class="reference"><a href="#cite_note-1"><span class="cite-bracket">[</span>1<span class="cite-bracket">]</span></a></sup> More.<sup class="reference"><a href="#cite_note-1">[1]</a></sup></p>
<div class="reflist"><div class="mw-references-wrap"><ol class="references"><li id="cite_note-1"><span class="mw-cite-backlink"><b><a href="#cite_ref-1">^</a></b></span> <span class="reference-text">Author (2020). <i>Book</i>.</span></li><li id="cite_note-2"><span class="reference-text">Unused ref.</span></li></ol></div></div>`,
			"P[Fact.^note{P[Author (2020). _[Book].]} More.^note{P[Author (2020). _[Book].]}]\nOL(0,0){Pl[Unused ref.]}"},
		{"epub aside footnote",
			`<p>Body<a epub:type="noteref" href="#n1">1</a>.</p><aside epub:type="footnote" id="n1"><p>Aside note.</p></aside>`,
			`P[Body^note{P[Aside note.]}.]`},
		{"labelled aside footnotes",
			`<p>Ref <a class="footnote-reference brackets" href="#id3" id="id1" role="doc-noteref"><span class="fn-bracket">[</span>1<span class="fn-bracket">]</span></a></p>
<aside class="footnote-list brackets"><aside class="footnote brackets" id="id3" role="doc-footnote"><span class="label"><span class="fn-bracket">[</span><a role="doc-backlink" href="#id1">1</a><span class="fn-bracket">]</span></span><p>Aside list note.</p></aside></aside>`,
			`P[Ref ^note{P[Aside list note.]}]`},
		{"unreferenced note stays in place",
			`<p>No refs.</p><section class="footnotes"><ol><li id="fn9"><p>Orphan.</p></li></ol></section>`,
			"P[No refs.]\nOL(0,0)~{P[Orphan.]}"},
		{"self-referencing note does not loop",
			`<p>A<a href="#fn1" class="footnote-ref">1</a></p><section class="footnotes"><ol><li id="fn1"><p>See <a href="#fn1" class="footnote-ref">1</a></p></li></ol></section>`,
			`P[A^note{P[See]}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks, _ := Fragment(tt.in, nil)
			if got := dump(blocks); got != tt.want {
				t.Errorf("\ngot:  %s\nwant: %s", got, tt.want)
			}
		})
	}
}
