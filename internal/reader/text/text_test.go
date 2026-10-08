package text

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
	xunicode "golang.org/x/text/encoding/unicode"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func read(t *testing.T, s string) (*ast.Document, []string) {
	t.Helper()
	doc, warns, err := Read(context.Background(), []byte(s), rd.Options{Name: "notes.txt"})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc.Resources == nil {
		t.Fatal("Resources not initialised")
	}
	return doc, warns
}

func checkDump(t *testing.T, got []ast.Block, want string) {
	t.Helper()
	if d := dump(got); d != want {
		t.Errorf("blocks mismatch\n got:\n%s\nwant:\n%s", d, want)
	}
}

func TestTitleAndMetadata(t *testing.T) {
	tests := []struct {
		name, in, title, body string
	}{
		{"first line title", "Meeting Notes\n\nWe met today.\n", "Meeting Notes", "P We met today."},
		{"underlined title", "Project Plan\n============\n\nIntro text.\n", "Project Plan", "P Intro text."},
		{"markdown title", "# Release notes\n\nFixed bugs.\n", "Release notes", "P Fixed bugs."},
		{"only line is not a title", "Just one line\n", "", "P Just one line"},
		{"wrapped first paragraph is not a title", "This first line\ncontinues here.\n\nMore.\n", "", "P This first line⏎continues here.\nP More."},
		{"salutation is not a title", "Dear Anna,\n\nThanks for the note.\n", "", "P Dear Anna,\nP Thanks for the note."},
		{"list item is not a title", "- milk\n\n- eggs\n", "", "ULloose{P milk | P eggs}"},
		{"long line is not a title", strings.Repeat("word ", 25) + "\n\nNext.\n", "", "P " + strings.TrimSpace(strings.Repeat("word ", 25)) + "\nP Next."},
		{"leading blank lines", "\n\n  Title  \n\nBody\n", "Title", "P Body"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, tc.in)
			if doc.Meta.Title != tc.title {
				t.Errorf("title = %q, want %q", doc.Meta.Title, tc.title)
			}
			checkDump(t, doc.Blocks, tc.body)
		})
	}
}

func TestKeyValueHeader(t *testing.T) {
	in := "Title: Quarterly Review\nAuthor: Anna Bērziņa, Juris Kalniņš\nDate: 2024-03-01\nKeywords: finance; review, Q1\nVersion: 1.2\nStatus: DRAFT\n\nSUMMARY\n\nAll good.\n"
	doc, _ := read(t, in)
	m := doc.Meta
	if m.Title != "Quarterly Review" || m.Date != "2024-03-01" || m.Version != "1.2" || m.Status != "DRAFT" {
		t.Errorf("meta = %+v", m)
	}
	if names := m.AuthorNames(); !slices.Equal(names, []string{"Anna Bērziņa", "Juris Kalniņš"}) {
		t.Errorf("authors = %q", names)
	}
	if !slices.Equal(m.Keywords, []string{"finance", "review", "Q1"}) {
		t.Errorf("keywords = %q", m.Keywords)
	}
	checkDump(t, doc.Blocks, "H1 SUMMARY\nP All good.")

	// A single "Key: Value" line is content, and unknown keys stop the block.
	doc, _ = read(t, "Notes\n\nDate: today\nPlace: here\n\nHello there.\n")
	checkDump(t, doc.Blocks, "P Date: today⏎Place: here\nP Hello there.")
	doc, _ = read(t, "Author: Smith, John\nSubject: Notes\n\nText.\n")
	if names := doc.Meta.AuthorNames(); !slices.Equal(names, []string{"Smith, John"}) || doc.Meta.Subject != "Notes" {
		t.Errorf("meta = %+v", doc.Meta)
	}
}

func TestHeadings(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"setext levels", "Intro\n\nOne\n===\n\nTwo\n---\n\nThree\n~~~\n\ntext\n", "H1 One\nH2 Two\nH3 Three\nP text"},
		{"setext after paragraph lines", "Intro text\nmore text\nSection\n-------\nbody\n", "P Intro text⏎more text\nH2 Section\nP body"},
		{"all caps", "x\n\nINTRODUCTION\nThis explains.\n\nRESULTS AND DISCUSSION\n\nDone.\n", "H1 INTRODUCTION\nP This explains.\nH1 RESULTS AND DISCUSSION\nP Done."},
		{"all caps with digits", "x\n\nPART 2: ĒKAS\n\ny\n", "H1 PART 2: ĒKAS\nP y"},
		{"all caps sentence is text", "x\n\nTHIS IS SHOUTING.\n\ny\n", "P THIS IS SHOUTING.\nP y"},
		{"all caps inside paragraph", "x\n\nfirst line\nNASA LAUNCH\n", "P first line⏎NASA LAUNCH"},
		{"numbered outline", "x\n\n1. Introduction\n\nText one.\n\n2.1 Methods\n\nText two.\n\nIV. Results\n\nText three.\n", "H1 Introduction\nP Text one.\nH2 Methods\nP Text two.\nH1 Results\nP Text three."},
		{"numbered items are a list", "x\n\n1. Buy milk\n\n2. Buy eggs\n", "OL(1,0)loose{P Buy milk | P Buy eggs}"},
		{"markdown headings", "x\n\n## Setup ##\n\n### Deep\n", "H2 Setup\nH3 Deep"},
		{"rule", "a\n\n-----\n\nb\n", "HR\nP b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, tc.in)
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestLists(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"bullets", "x\n\n- one\n- two\n* three\n", "UL{one | two}\nUL{three}"},
		{"unicode bullets", "x\n\n• one\n• two\n", "UL{one | two}"},
		{"nested by indentation", "x\n\n- a\n  - a1\n  - a2\n- b\n", "UL{a; UL{a1 | a2} | b}"},
		{"wrapped item lines", "x\n\n- first item that\n  wraps here\n- second\nwrapped lazily\n", "UL{first item that⏎wraps here | second⏎wrapped lazily}"},
		{"item paragraphs", "x\n\n1. One\n\n   More about one.\n\n2. Two\n", "OL(1,0)loose{P One; P More about one. | P Two}"},
		{"paren numbers", "x\n\n1) a\n2) b\n", "OL(1,0){a | b}"},
		{"start number", "x\n\n3. c\n4. d\n", "OL(3,0){c | d}"},
		{"letters", "x\n\na. alpha\nb. beta\n", "OL(1,1){alpha | beta}"},
		{"parenthesised letters", "x\n\n(a) alpha\n(b) beta\n", "OL(1,1){alpha | beta}"},
		{"roman", "x\n\ni. one\nii. two\niii. three\niv. four\nv. five\n", "OL(1,3){one | two | three | four | five}"},
		{"upper roman", "x\n\nI) one\nII) two\n", "OL(1,4){one | two}"},
		{"initial is not a list", "x\n\nA. Smith wrote this book.\n", "P A. Smith wrote this book."},
		{"ordinal date is not a list", "x\n\n15. martā notika sapulce.\n", "P 15. martā notika sapulce."},
		{"year is not a list", "x\n\n2024. gadā viss bija labi.\n2025. gadā arī.\n", "P 2024. gadā viss bija labi.⏎2025. gadā arī."},
		{"list after paragraph", "Shopping:\n- milk\n- eggs\n", "P Shopping:\nUL{milk | eggs}"},
		{"different markers split", "x\n\n- a\n1. b\n", "UL{a}\nOL(1,0){b}"},
		{"nested ordered in bullets", "x\n\n- fruit\n    1. apple\n    2. pear\n- veg\n", "UL{fruit; OL(1,0){apple | pear} | veg}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, tc.in)
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestIndentedBlocks(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"code", "Run:\n\n    go build ./...\n\n    ./app --flag=1\n\nDone.\n", "P Run:\nCode() \"go build ./...\\n\\n./app --flag=1\"\nP Done."},
		{"tab indented code with nesting", "x\n\n\tif x {\n\t\ty()\n\t}\n", "Code() \"if x {\\n\\ty()\\n}\""},
		{"first-line indent is a paragraph", "x\n\n    It was a dark and stormy night;\nthe rain fell in torrents.\n", "P It was a dark and stormy night;⏎the rain fell in torrents."},
		{"indented prose is a quote", "x\n\n    The only thing we have to fear is fear itself.\n", "Quote{P The only thing we have to fear is fear itself.}"},
		{"aligned columns are preformatted", "x\n\n    Name      Value\n    alpha     1\n", "Code() \"Name      Value\\nalpha     1\""},
		{"uniformly indented file", "    First line here, then\n    more text.\n\n    Second paragraph.\n", "P First line here, then⏎more text.\nP Second paragraph."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, tc.in)
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestLineBlocksAndParagraphs(t *testing.T) {
	doc, _ := read(t, "x\n\nSIA Example\nBrīvības iela 1\nRīga, LV-1010\nLatvia\n\nA normal paragraph that is long enough to be a paragraph and\nwraps onto a second line and a third one as well, which is\nhow plain text is usually written.\n")
	checkDump(t, doc.Blocks, ""+
		"Lines{SIA Example / Brīvības iela 1 / Rīga, LV-1010 / Latvia}\n"+
		"P A normal paragraph that is long enough to be a paragraph and⏎wraps onto a second line and a third one as well, which is⏎how plain text is usually written.")
	doc, _ = read(t, "x\n\nspaces    inside    collapse\n")
	checkDump(t, doc.Blocks, "P spaces inside collapse")
}

func TestAutolinks(t *testing.T) {
	tests := []struct{ in, want string }{
		{"See https://example.com/a?b=1.", "P See [https://example.com/a?b=1](https://example.com/a?b=1)."},
		{"Visit www.example.org, then", "P Visit [www.example.org](https://www.example.org), then"},
		{"(see http://en.wikipedia.org/wiki/Go_(language))", "P (see [http://en.wikipedia.org/wiki/Go_(language)](http://en.wikipedia.org/wiki/Go_(language)))"},
		{"(see https://x.example/y)", "P (see [https://x.example/y](https://x.example/y))"},
		{"Mail anna.berzina+test@example.co.uk!", "P Mail [anna.berzina+test@example.co.uk](mailto:anna.berzina+test@example.co.uk)!"},
		{"two: a@b.lv and c@d.com.", "P two: [a@b.lv](mailto:a@b.lv) and [c@d.com](mailto:c@d.com)."},
		{"not mail: user@localhost or @handle", "P not mail: user@localhost or @handle"},
		{"no www.x link", "P no www.x link"},
		{"mixed https://a.example and b@c.example", "P mixed [https://a.example](https://a.example) and [b@c.example](mailto:b@c.example)"},
	}
	for _, tc := range tests {
		doc, _ := read(t, "x\n\n"+tc.in+"\n")
		checkDump(t, doc.Blocks, tc.want)
	}
}

func TestEncodingsAndPages(t *testing.T) {
	u16, _ := xunicode.UTF16(xunicode.LittleEndian, xunicode.UseBOM).NewEncoder().Bytes([]byte("Virsraksts\r\n\r\nTeksts ar āčē.\r\n"))
	cp1257, _ := charmap.Windows1257.NewEncoder().Bytes([]byte("Virsraksts\n\nŠī ir ļoti īsa piezīme ar garumzīmēm.\n"))
	tests := []struct {
		name  string
		in    []byte
		title string
		want  string
	}{
		{"utf-16 with crlf", u16, "Virsraksts", "P Teksts ar āčē."},
		{"windows-1257", cp1257, "Virsraksts", "P Šī ir ļoti īsa piezīme ar garumzīmēm."},
		{"classic mac line endings", []byte("T\r\rbody\r"), "T", "P body"},
		{"form feed pages", []byte("Head\n\npage one\n\fpage two\n\f\fpage three"), "Head", "P page one\nPB\nP page two\nPB\nP page three"},
		{"control characters dropped", []byte("Head\n\nbe\x07ll\x00\n"), "Head", "P bell"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _, err := Read(context.Background(), tc.in, rd.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if doc.Meta.Title != tc.title {
				t.Errorf("title = %q", doc.Meta.Title)
			}
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestEmptyAndCancelled(t *testing.T) {
	for _, in := range []string{"", "  \n\n\t\n", "\xEF\xBB\xBF"} {
		doc, warns := read(t, in)
		if len(doc.Blocks) != 0 || len(warns) != 1 {
			t.Errorf("%q: blocks = %s, warnings = %q", in, dump(doc.Blocks), warns)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := Read(ctx, []byte(strings.Repeat("line\n", 5000)), rd.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func FuzzRead(f *testing.F) {
	f.Add([]byte("Title\n=====\n\n- a\n  - b\n\n    code\n\n1. x\n\nmail a@b.cd https://x.y/(z)\f"))
	f.Add([]byte("Title: a\nAuthor: b\n\nIV. X\n\n(a) q\n(b) r\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, _, err := Read(context.Background(), data, rd.Options{})
		if err != nil || doc == nil {
			t.Fatalf("doc = %v, err = %v", doc, err)
		}
	})
}
