package table

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func checkDump(t *testing.T, got []ast.Block, want string) {
	t.Helper()
	if d := dump(got); d != want {
		t.Errorf("blocks mismatch\n got:\n%s\nwant:\n%s", d, want)
	}
}

func readCSV(t *testing.T, data []byte) (*ast.Document, []string) {
	t.Helper()
	doc, warns, err := ReadCSV(context.Background(), data, rd.Options{Name: "data.csv"})
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if doc.Resources == nil {
		t.Fatal("Resources not initialised")
	}
	if doc.Meta.Title != "" {
		t.Errorf("CSV must not set a title, got %q", doc.Meta.Title)
	}
	return doc, warns
}

func TestCSVDelimiters(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"comma", "a,b\n1,2\n", "Table[RR]{H[a|b][1|2]}"},
		{"semicolon with decimal commas", "Name;Price;Qty\nApple;1,50;3\nPear;2,25;4\n", "Table[-RR]{H[Name|Price|Qty][Apple|1,50|3][Pear|2,25|4]}"},
		{"tab", "x\ty\nfoo, bar\t1\n", "Table[-R]{H[x|y][foo, bar|1]}"},
		{"pipe", "a|b|c\n1|2|3\n", "Table[RRR]{H[a|b|c][1|2|3]}"},
		{"quoted delimiters and newlines", "name,comment\n\"Smith, J\",\"line one\nline two\"\n", "Table[--]{H[name|comment][Smith, J|line one↵line two]}"},
		{"space before quote", "a, \"b,c\"\n1, 2\n", "Table[RR]{H[a|b,c][1|2]}"},
		{"sep hint line", "sep=;\na;b\n1,5;2\n", "Table[RR]{H[a|b][1,5|2]}"},
		{"commas in text do not win", "id;text\n1;hello, world\n2;foo, bar\n3;baz\n", "Table[R-]{H[id|text][1|hello, world][2|foo, bar][3|baz]}"},
		{"single column", "names\nAnna\nJānis\n", "Table[-]{H[names][Anna][Jānis]}"},
		{"crlf and cr line endings", "a,b\r\n1,2\r\n", "Table[RR]{H[a|b][1|2]}"},
		{"classic mac line endings", "a,b\r1,2\r", "Table[RR]{H[a|b][1|2]}"},
		{"lazy quotes", "a,b\nsay \"hi\",2\n", "Table[-R]{H[a|b][say \"hi\"|2]}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := readCSV(t, []byte(tc.in))
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestCSVShape(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"ragged rows padded", "a,b,c\n1\n2,3\n", "Table[RR-]{H[a|b|c][1||][2|3|]}"},
		{"trailing empty columns and rows trimmed", "a,b,,\n1,2,,\n,,,\n\n", "Table[RR]{H[a|b][1|2]}"},
		{"header only", "a,b\n", "Table[--]{H[a|b]}"},
		{"cells trimmed", "  a , b \n 1 , 2 \n", "Table[RR]{H[a|b][1|2]}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := readCSV(t, []byte(tc.in))
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestCSVNumericColumns(t *testing.T) {
	in := "Item,Amount,Code,Mixed\n" +
		"a,\"1,234.50\",X1,1\n" +
		"b,(12.00),X2,2\n" +
		"c,€ 7,X3,three\n" +
		"d,12%,X4,4\n" +
		"e,-3,X5,\n"
	doc, _ := readCSV(t, []byte(in))
	tbl := doc.Blocks[0].(*ast.Table)
	got := []ast.Align{tbl.Cols[0].Align, tbl.Cols[1].Align, tbl.Cols[2].Align, tbl.Cols[3].Align}
	want := []ast.Align{ast.AlignDefault, ast.AlignRight, ast.AlignDefault, ast.AlignDefault}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d align = %v, want %v", i, got[i], want[i])
		}
	}
	// 3 of 4 non-empty values numeric is below 80%.
	doc, _ = readCSV(t, []byte("h\n1\n2\n3\n4\nx\n"))
	if a := doc.Blocks[0].(*ast.Table).Cols[0].Align; a != ast.AlignRight {
		t.Errorf("4 of 5 numeric should be right aligned, got %v", a)
	}
	doc, _ = readCSV(t, []byte("h\n1\n2\nx\ny\n"))
	if a := doc.Blocks[0].(*ast.Table).Cols[0].Align; a != ast.AlignDefault {
		t.Errorf("2 of 4 numeric should not be right aligned, got %v", a)
	}
}

func TestCSVEncodings(t *testing.T) {
	enc := func(cm *charmap.Charmap, s string) []byte {
		b, err := cm.NewEncoder().Bytes([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	utf16 := func(e unicode.Endianness, bom unicode.BOMPolicy, s string) []byte {
		b, err := unicode.UTF16(e, bom).NewEncoder().Bytes([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"utf-8 bom", append([]byte{0xEF, 0xBB, 0xBF}, "vārds,skaits\nŠķūnis,2\n"...), "Table[-R]{H[vārds|skaits][Šķūnis|2]}"},
		{"utf-16le bom tab separated", utf16(unicode.LittleEndian, unicode.UseBOM, "Name\tCount\nĀbele\t3\n"), "Table[-R]{H[Name|Count][Ābele|3]}"},
		{"utf-16be bom", utf16(unicode.BigEndian, unicode.UseBOM, "a;b\nž;1\n"), "Table[-R]{H[a|b][ž|1]}"},
		{"utf-16le without bom", utf16(unicode.LittleEndian, unicode.IgnoreBOM, "col1,col2\nabc,def\n"), "Table[--]{H[col1|col2][abc|def]}"},
		{"windows-1257 latvian", enc(charmap.Windows1257, "Pilsēta;Iedzīvotāji\nRīga;600000\nJūrmala;50000\nŠķēde;10\n"), "Table[-R]{H[Pilsēta|Iedzīvotāji][Rīga|600000][Jūrmala|50000][Šķēde|10]}"},
		{"windows-1251 russian", enc(charmap.Windows1251, "Город,Население\nМосква,13000000\n"), "Table[-R]{H[Город|Население][Москва|13000000]}"},
		{"windows-1252 french", enc(charmap.Windows1252, "ville,café\nNîmes,été\n"), "Table[--]{H[ville|café][Nîmes|été]}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := readCSV(t, tc.in)
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestCSVEmpty(t *testing.T) {
	for _, in := range []string{"", "   \n\n", ",,,\n,,\n", "\xEF\xBB\xBF"} {
		doc, warns := readCSV(t, []byte(in))
		if len(warns) != 1 || warns[0] != "the file contains no data" {
			t.Errorf("%q: warnings = %q", in, warns)
		}
		checkDump(t, doc.Blocks, "P ")
	}
}

func TestCSVLimits(t *testing.T) {
	var b strings.Builder
	b.WriteString("a,b\n")
	for range 100 {
		b.WriteString("1,2\n")
	}
	doc, warns, err := ReadCSV(context.Background(), []byte(b.String()), rd.Options{Limits: rd.Limits{MaxTableCells: 21}})
	if err != nil {
		t.Fatal(err)
	}
	tbl := doc.Blocks[0].(*ast.Table)
	if len(tbl.Body) != 9 || len(warns) != 1 || !strings.Contains(warns[0], "truncated") {
		t.Errorf("body rows = %d, warnings = %q", len(tbl.Body), warns)
	}
}

func TestCSVContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := ReadCSV(ctx, []byte("a,b\n1,2\n"), rd.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestTSV(t *testing.T) {
	doc, _, err := ReadTSV(context.Background(), []byte("a\tb\tc\nx, y, z\t\t3\n"), rd.Options{})
	if err != nil {
		t.Fatal(err)
	}
	checkDump(t, doc.Blocks, "Table[--R]{H[a|b|c][x, y, z||3]}")
}

func TestIsNumeric(t *testing.T) {
	yes := []string{"0", "-1", "+2.5", "1,234.56", "1.234,56", "1 234,56", "1 234", "(1,234)", "12%", "€12", "12 €", "$-5", "1e10", "2.5E-3", ".5", "−3", "USD 10"}
	no := []string{"", "abc", "1-2", "2024-01-05", "10:30", "1..2", "1,,2", "e5", "12a", "--", "€", "1e", "1,", ",5x"}
	for _, s := range yes {
		if !isNumeric(s) {
			t.Errorf("isNumeric(%q) = false", s)
		}
	}
	for _, s := range no {
		if isNumeric(s) {
			t.Errorf("isNumeric(%q) = true", s)
		}
	}
}

func TestDetectLegacyCodePage(t *testing.T) {
	enc := func(cm *charmap.Charmap, s string) []byte {
		b, _ := cm.NewEncoder().Bytes([]byte(s))
		return b
	}
	tests := []struct {
		in   []byte
		want int
	}{
		{enc(charmap.Windows1257, "Šis ir ļoti garš teksts ar daudzām garumzīmēm, žagariem un šķiedrām."), 1257},
		{enc(charmap.Windows1251, "Это русский текст с кириллицей."), 1251},
		{enc(charmap.Windows1252, "Ça coûte très cher à Nîmes, déjà."), 1252},
		{[]byte("plain ascii"), 1252},
	}
	for _, tc := range tests {
		if got := legacyCodePage(tc.in); got != tc.want {
			t.Errorf("legacyCodePage(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func FuzzReadCSV(f *testing.F) {
	f.Add([]byte("a,b\n\"x\ny\",2\n"))
	f.Add([]byte("sep=|\na|b\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, _, err := ReadCSV(context.Background(), data, rd.Options{})
		if err != nil || doc == nil || len(doc.Blocks) != 1 {
			t.Fatalf("doc = %v, err = %v", doc, err)
		}
	})
}
