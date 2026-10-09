package layout

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/ast"
)

// ocrPage builds OCR-style input: one run per word, boxes as an engine
// reports them (ascenders and descenders change the box height), no font
// information.
type ocrPage struct {
	Page
	rng    *rand.Rand
	jitter float64 // random baseline offset per word (handwriting)
	slope  float64 // baseline drift per point of x
}

func newOCRPage() *ocrPage {
	return &ocrPage{Page: Page{Width: 612, Height: 792}, rng: rand.New(rand.NewSource(1))}
}

// line places text with its baseline at y, starting at x, using em size h.
// It returns the x position after the last word.
func (p *ocrPage) line(x, y, h float64, text string) float64 {
	for _, w := range strings.Fields(text) {
		width := 0.0
		for _, r := range w {
			switch {
			case strings.ContainsRune("ilj.,;:'!|", r):
				width += 0.28 * h
			case r == 'm' || r == 'w' || r == 'M' || r == 'W':
				width += 0.8 * h
			case r >= 'A' && r <= 'Z':
				width += 0.65 * h
			default:
				width += 0.5 * h
			}
		}
		base := y + p.slope*(x-72)
		if p.jitter > 0 {
			base += (p.rng.Float64()*2 - 1) * p.jitter
		}
		top, bottom := base-0.52*h, base
		if strings.ContainsAny(w, "ij") {
			top = base - 0.68*h
		}
		if strings.ContainsAny(w, "ABCDEFGHIJKLMNOPQRSTUVWXYZbdfhklt0123456789ĀČĒĢĪĶĻŅŠŪŽ") {
			top = base - 0.72*h
		}
		if strings.ContainsAny(w, "gjpqyģ,;") {
			bottom = base + 0.22*h
		}
		p.Runs = append(p.Runs, Run{Text: w, X: x, Y: top, W: width, H: bottom - top})
		x += width + 0.3*h
	}
	return x
}

// para sets text as a ragged paragraph between x0 and x1 starting with its
// first baseline at y, and returns the baseline below the last line.
func (p *ocrPage) para(x0, x1, y, h, pitch float64, text string) float64 {
	words := strings.Fields(text)
	var line []string
	width := 0.0
	flush := func() {
		if len(line) > 0 {
			p.line(x0, y, h, strings.Join(line, " "))
			y += pitch
		}
		line, width = nil, 0
	}
	for _, w := range words {
		ww := float64(len([]rune(w)))*0.5*h + 0.3*h
		if width+ww > x1-x0 && len(line) > 0 {
			flush()
		}
		line = append(line, w)
		width += ww
	}
	flush()
	return y
}

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

const (
	loremA = "Soil moisture determines how quickly seeds germinate and how well young plants establish themselves in spring."
	loremB = "Measurements were taken every hour with capacitive sensors buried at two depths in each of the twelve fields."
	loremC = "The sensors were calibrated against gravimetric samples after every heavy rain so that drift stayed small."
)

// Apple Vision and similar engines report tight word boxes: gaps of about a
// tenth of the text height. Each run is still a word of its own.
func TestOCRTightWordGapsKeepSpaces(t *testing.T) {
	var runs []Run
	x, y, h := 56.0, 70.0, 18.5
	for _, w := range strings.Fields("Measurements were taken every hour with capacitive sensors") {
		ww := float64(len([]rune(w))) * 0.45 * h
		runs = append(runs, Run{Text: w, X: x, Y: y, W: ww, H: h})
		x += ww + 0.08*h
	}
	doc, _ := Document(context.Background(), []Page{{Width: 595, Height: 842, Runs: runs}}, Options{})
	if got := ast.Dump(doc.Blocks); !strings.Contains(got, "Measurements were taken every hour with capacitive sensors") {
		t.Fatalf("words run together:\n%s", got)
	}
}

func TestOCRTwoColumns(t *testing.T) {
	p := newOCRPage()
	p.line(72, 80, 22, "Field Observations")
	p.line(72, 125, 15, "Methods")
	y := p.para(72, 290, 150, 11, 14, loremA+" "+loremB)
	y = p.para(72, 290, y+10, 11, 14, loremC)
	// The last paragraph of the left column continues at the top of the
	// right one.
	y = p.para(72, 290, y+10, 11, 14, "Results from the first season show that moisture stayed")
	_ = y
	p.para(320, 540, 150, 11, 14, "within the optimal band on most fields during the whole growing period.")
	p.line(320, 205, 15, "Discussion")
	p.para(320, 540, 230, 11, 14, loremB+" "+loremA)

	blocks, meta, _ := Reconstruct(context.Background(), []Page{p.Page}, nil, Options{})
	if meta.Title != "Field Observations" {
		t.Errorf("title %q", meta.Title)
	}
	wantInOrder(t, ast.Dump(blocks),
		"H1 Methods\n",
		"Para "+loremA+" "+loremB+"\n",
		"Para "+loremC+"\n",
		"Para Results from the first season show that moisture stayed within the optimal band on most fields during the whole growing period.\n",
		"H1 Discussion\n",
		"Para "+loremB+" "+loremA+"\n",
	)
}

func TestOCRLists(t *testing.T) {
	p := newOCRPage()
	p.line(72, 100, 11, "The survey had three steps:")
	p.line(90, 116, 11, "1.")
	p.line(108, 116, 11, "install the sensors in every field;")
	p.line(90, 132, 11, "2.")
	p.line(108, 132, 11, "read the data every hour;")
	p.line(90, 148, 11, "3.")
	p.line(108, 148, 11, "calibrate after rain.")
	p.line(72, 172, 11, "Equipment used:")
	for i, item := range []string{"capacitive sensors", "a solar data logger", "a field tablet"} {
		p.line(90, 188+float64(i)*16, 11, "•")
		p.line(104, 188+float64(i)*16, 11, item)
	}
	blocks, _, _ := Reconstruct(context.Background(), []Page{p.Page}, nil, Options{})
	wantInOrder(t, ast.Dump(blocks),
		"Para The survey had three steps:\n",
		"Ordered(0,0)-tight\n  Item\n    Plain install the sensors in every field;\n  Item\n    Plain read the data every hour;\n  Item\n    Plain calibrate after rain.\n",
		"Para Equipment used:\n",
		"Bullet-tight\n  Item\n    Plain capacitive sensors\n  Item\n    Plain a solar data logger\n  Item\n    Plain a field tablet\n",
	)
}

func TestOCRTable(t *testing.T) {
	p := newOCRPage()
	p.para(72, 540, 100, 11, 14, "The table lists the average values per farm.")
	rows := [][]string{
		{"Farm", "Fields", "Moisture"},
		{"Ozolkalni", "18", "24.1"},
		{"Bērzciems", "14", "21.7"},
		{"Lejas", "10", "26.3"},
	}
	for i, r := range rows {
		y := 140 + float64(i)*18
		p.line(72, y, 11, r[0])
		p.line(220, y, 11, r[1])
		p.line(320, y, 11, r[2])
	}
	p.para(72, 540, 230, 11, 14, "All farms stayed within the optimal band.")
	blocks, _, _ := Reconstruct(context.Background(), []Page{p.Page}, nil, Options{})
	wantInOrder(t, ast.Dump(blocks),
		"Para The table lists the average values per farm.\n",
		"Table cols=3\n  head: |Farm |Fields |Moisture\n  body: |Ozolkalni |18 |24.1\n  body: |Bērzciems |14 |21.7\n  body: |Lejas |10 |26.3\n",
		"Para All farms stayed within the optimal band.\n",
	)
}

func TestOCRHandwrittenNote(t *testing.T) {
	// Handwriting: every word sits on its own baseline, lines drift
	// upwards and letter sizes vary.
	p := newOCRPage()
	p.jitter = 1.8
	p.slope = -0.02
	lines := []string{
		"Dear Anna, thank you for",
		"the soil samples. The",
		"results look very good",
		"and we can start sowing",
		"next week if it stays dry.",
	}
	for i, l := range lines {
		p.line(90+float64(i%2)*4, 120+float64(i)*24, 14+float64(i%3), l)
	}
	p.line(260, 280, 14, "Pēteris")
	blocks, _, _ := Reconstruct(context.Background(), []Page{p.Page}, nil, Options{})
	text := ast.BlocksText(blocks)
	want := strings.Join(lines, " ")
	if !strings.Contains(strings.Join(strings.Fields(text), " "), want) {
		t.Errorf("text %q\nwant %q", text, want)
	}
	if !strings.Contains(text, "Pēteris") {
		t.Errorf("signature lost: %q", text)
	}
	for _, b := range blocks {
		if _, ok := b.(*ast.Table); ok {
			t.Errorf("handwriting taken for a table:\n%s", ast.Dump(blocks))
		}
	}
}

func TestProducerHints(t *testing.T) {
	// Runs as a tagged PDF provides them: blocks and roles.
	runs := []Run{
		{Text: "Page 2 of 9", X: 450, Y: 30, W: 80, H: 9, FontSize: 9, Role: RoleArtifact},
		{Text: "Overview", X: 72, Y: 80, W: 80, H: 11, FontSize: 11, Bold: true, Block: 1, Role: RoleHeading2},
		{Text: "The first paragraph ends here", X: 72, Y: 100, W: 200, H: 11, FontSize: 11, Block: 2, Role: RoleParagraph},
		// A new paragraph without any spacing or indentation: only the
		// block hint separates it.
		{Text: "and this line is a new paragraph", X: 72, Y: 114, W: 220, H: 11, FontSize: 11, Block: 3, Role: RoleParagraph},
		{Text: "1", X: 293, Y: 112, W: 4, H: 7, FontSize: 7, Block: 3, Role: RoleNoteRef},
		{Text: "1", X: 72, Y: 700, W: 4, H: 7, FontSize: 7, Block: 4, Role: RoleNote},
		{Text: "A note at the foot of the page.", X: 78, Y: 702, W: 160, H: 9, FontSize: 9, Block: 4, Role: RoleNote},
	}
	blocks, _, _ := Reconstruct(context.Background(), []Page{{Width: 612, Height: 792, Runs: runs}}, nil, Options{})
	got := ast.Dump(blocks)
	want := "H2 Overview\nPara The first paragraph ends here\nPara and this line is a new paragraph^[Para A note at the foot of the page.]\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestDocumentWithFigure(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	img.Set(3, 3, color.RGBA{200, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p := newOCRPage()
	p.line(72, 80, 22, "Annual Report")
	p.para(72, 540, 130, 11, 14, loremA)
	p.Images = []Image{{X: 150, Y: 160, W: 300, H: 200, Data: buf.Bytes(), MediaType: "image/png"}}
	p.line(150, 380, 10, "Figure 1: Moisture during the season")
	p.para(72, 540, 410, 11, 14, loremB)

	doc, _ := Document(context.Background(), []Page{p.Page}, Options{Lang: "en"})
	if doc.Meta.Title != "Annual Report" || doc.Resources == nil || doc.Resources.Len() != 1 {
		t.Fatalf("meta %+v, resources %v", doc.Meta.Title, doc.Resources.Names())
	}
	wantInOrder(t, ast.Dump(doc.Blocks),
		"Para "+loremA+"\n",
		`Figure src="res:page1-image1.png" w=300pt cap=Moisture during the season`+"\n",
		"Para "+loremB+"\n",
	)
}

func TestRobustInput(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	pages := []Page{
		{},
		{Width: 612, Height: 792, Runs: []Run{
			{Text: "nan", X: nan, Y: 10, W: 10, H: 10},
			{Text: "inf", X: 10, Y: inf, W: 10, H: 10},
			{Text: "negative", X: 10, Y: 10, W: -5, H: 10},
			{Text: "zero height", X: 10, Y: 10, W: 5, H: 0},
			{Text: "   ", X: 10, Y: 10, W: 5, H: 10},
			{Text: "outside", X: 5000, Y: 5000, W: 5, H: 10},
			{Text: "\x00\x01control", X: 10, Y: 30, W: 50, H: 10, FontSize: -3},
		}, Images: []Image{{X: nan, W: 100, H: 100, Data: []byte("x")}, {X: 1, Y: 1, W: 1e9, H: 1e9}}},
		{Width: -1, Height: 0, Runs: []Run{{Text: "only text", X: 1, Y: 1, W: 40, H: 10}}},
	}
	// Many words on one line, overprinted.
	var crowd []Run
	for i := 0; i < 3000; i++ {
		crowd = append(crowd, Run{Text: "w", X: float64(i % 50), Y: 100, W: 5, H: 10, FontSize: 10})
	}
	pages = append(pages, Page{Width: 612, Height: 792, Runs: crowd})
	blocks, _, _ := Reconstruct(context.Background(), pages, nil, Options{})
	if text := ast.BlocksText(blocks); !strings.Contains(text, "control") || !strings.Contains(text, "only text") || strings.Contains(text, "nan") {
		t.Errorf("text %q", text)
	}
	if blocks, _, _ := Reconstruct(nil, nil, nil, Options{}); len(blocks) != 0 { //nolint:staticcheck // a nil context is tolerated
		t.Errorf("no pages: %v", blocks)
	}
}

func TestCanceled(t *testing.T) {
	p := newOCRPage()
	p.para(72, 540, 100, 11, 14, loremA+loremB+loremC)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if blocks, _, _ := Reconstruct(ctx, []Page{p.Page, p.Page}, nil, Options{}); blocks != nil {
		t.Errorf("canceled reconstruction returned %d blocks", len(blocks))
	}
}
