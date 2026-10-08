package hyph

import (
	"strings"
	"sync"
	"testing"

	"golang.org/x/text/unicode/norm"
)

const shy = "\u00ad"

func TestInsertSoftHyphens(t *testing.T) {
	// In want, "·" marks an inserted soft hyphen.
	tests := []struct {
		name, tag, in, want string
	}{
		{"sentence", "lv",
			"Starptautiskā sadarbība, informācija un attīstība.",
			"Star·ptau·tis·kā sa·dar·bī·ba, in·for·mā·ci·ja un at·tīs·tī·ba."},
		{"capitalised and upper case", "lv", "Latvija LATVIJAS", "Lat·vi·ja LAT·VI·JAS"},
		{"short words untouched", "lv", "ar un uz ozols", "ar un uz ozols"},
		{"quotes and brackets", "lv", "„sadarbība“ (valoda) «grāmatvedība»",
			"„sa·dar·bī·ba“ (va·lo·da) «grā·mat·ve·dī·ba»"},
		{"dashes separate words", "lv", "sadarbība—valoda–attīstība",
			"sa·dar·bī·ba—va·lo·da–at·tīs·tī·ba"},
		{"slash separates words", "lv", "pārdevējs/pircējs", "pār·de·vējs/pir·cējs"},
		{"URL", "lv", "skatīt https://piemērs.lv/informācija/sadarbība?id=1 tālāk",
			"ska·tīt https://piemērs.lv/informācija/sadarbība?id=1 tā·lāk"},
		{"bare domain and www", "lv", "piemērs.lv www.dokumentācija.lv",
			"piemērs.lv www.dokumentācija.lv"},
		{"domain followed by full stop", "lv", "Apmeklējiet dokumentācija.lv.",
			"Ap·mek·lē·jiet dokumentācija.lv."},
		{"e-mail", "lv", "rakstiet: grāmatvedība@piemērs.lv",
			"rak·stiet: grāmatvedība@piemērs.lv"},
		{"file path", "lv", "/dokumenti/sadarbība.txt un C:\\dokumenti\\valoda",
			"/dokumenti/sadarbība.txt un C:\\dokumenti\\valoda"},
		{"digits", "lv", "sadarbība2026 2026. gada", "sadarbība2026 2026. ga·da"},
		{"underscore", "lv", "nodrošināšana_v2 sadarbība_valoda", "nodrošināšana_v2 sadarbība_valoda"},
		{"explicit hyphen", "lv", "Rīga-Jūrmala", "Rīga-Jūrmala"},
		{"existing soft hyphen", "lv", "sadar" + shy + "bība valoda", "sadar" + shy + "bība va" + shy + "lo" + shy + "da"},
		{"other script", "lv", "сотрудничество sadarbība", "сотрудничество sa·dar·bī·ba"},
		{"mixed-script word", "lv", "sadarbībaсотрудничество", "sadarbībaсотрудничество"},
		{"whitespace kinds preserved", "lv", "  sadarbība\t\nvaloda\u00a0attīstība \u2009x",
			"  sa·dar·bī·ba\t\nva·lo·da\u00a0at·tīs·tī·ba \u2009x"},
		{"Romanian", "ro", "Colaborarea internațională în învățământul românesc.",
			"Co·la·bo·ra·rea in·ter·națio·na·lă în în·vă·țămân·tul ro·mâ·nesc."},
		{"Romanian compound hyphen", "ro", "într-o dezvoltare", "într-o dezvol·ta·re"},
		{"Irish apostrophe", "ga", "d'idirnáisiúnta", "d'idir·náis·iúnta"},
		{"Irish exception", "ga", "bhrachtaí comhoibriú", "bhrachtaí comh·oib·riú"},
		{"Macedonian", "mk", "Меѓународна соработка", "Ме·ѓу·на·род·на со·ра·бот·ка"},
		{"Macedonian leaves Latin alone", "mk", "sadarbiba соработка", "sadarbiba со·ра·бот·ка"},
		{"Basque", "eu", "Nazioarteko lankidetza", "Na·zioar·te·ko lan·ki·de·tza"},
		{"Montenegrin", "cnr", "Međunarodna saradnja", "Me·đu·na·rod·na sa·rad·nja"},
		{"Serbian Latin", "sr-Latn", "međunarodna", "me·đu·na·rod·na"},
		{"Serbian Cyrillic left to Typst", "sr", "међународна", "међународна"},
		{"unsupported language", "en", "internationalisation", "internationalisation"},
		{"unknown tag", "xx-YY", "sadarbība", "sadarbība"},
		{"empty", "lv", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := strings.ReplaceAll(tt.want, "·", shy)
			if got := InsertSoftHyphens(tt.in, tt.tag); got != want {
				t.Errorf("InsertSoftHyphens(%q, %q)\n got %q\nwant %q", tt.in, tt.tag,
					strings.ReplaceAll(got, shy, "·"), tt.want)
			}
		})
	}
}

func TestInsertSoftHyphensDecomposed(t *testing.T) {
	nfc := "Uzņēmējdarbības ģimene"
	nfd := norm.NFD.String(nfc)
	got := InsertSoftHyphens(nfd, "lv")
	if strings.ReplaceAll(got, shy, "") != nfd {
		t.Fatal("decomposed input was not preserved byte for byte")
	}
	if norm.NFC.String(got) != InsertSoftHyphens(nfc, "lv") {
		t.Errorf("NFD result %q does not compose to the NFC result %q",
			norm.NFC.String(got), InsertSoftHyphens(nfc, "lv"))
	}
	// No soft hyphen may separate a combining mark from its base letter.
	for i := strings.Index(got, shy); i >= 0; {
		next := got[i+len(shy):]
		r := []rune(next)[0]
		if isMark(r) {
			t.Fatalf("soft hyphen before combining mark in %q", got)
		}
		j := strings.Index(next, shy)
		if j < 0 {
			break
		}
		i += len(shy) + j
	}
}

func TestInsertSoftHyphensUnchangedStringIsShared(t *testing.T) {
	in := strings.Repeat("ar un uz ", 100)
	if got := InsertSoftHyphens(in, "lv"); got != in {
		t.Fatal("text without hyphenatable words changed")
	}
}

func TestConcurrentFirstUse(t *testing.T) {
	// Every language is compiled lazily; concurrent first use must be safe
	// (run with -race) and yield identical results.
	texts := map[string]string{
		"lv": "starptautiskā sadarbība", "ro": "colaborarea internațională",
		"mk": "меѓународна соработка", "ga": "comhoibriú idirnáisiúnta",
		"eu": "nazioarteko lankidetza", "cnr": "međunarodna saradnja",
	}
	var wg sync.WaitGroup
	results := make(chan [2]string, 8*len(texts))
	for range 8 {
		for tag, text := range texts {
			wg.Go(func() {
				results <- [2]string{tag, InsertSoftHyphens(text, tag)}
			})
		}
	}
	wg.Wait()
	close(results)
	seen := map[string]string{}
	for r := range results {
		if prev, ok := seen[r[0]]; ok && prev != r[1] {
			t.Errorf("%s: inconsistent results %q and %q", r[0], prev, r[1])
		}
		seen[r[0]] = r[1]
		if !strings.Contains(r[1], shy) {
			t.Errorf("%s: nothing hyphenated in %q", r[0], r[1])
		}
	}
}

func FuzzInsertSoftHyphens(f *testing.F) {
	for _, s := range []string{
		"Starptautiskā sadarbība", "https://piemērs.lv/a-b", "x@y.lv", "ā\u0301\u0301b",
		"\u0304sadarbība", "d'idirnáisiúnta", "\xff\xfe broken utf8 sadarbība", "a\u00ad\u00adb",
		strings.Repeat("ā", 300),
	} {
		f.Add(s, "lv")
		f.Add(s, "ga")
	}
	f.Fuzz(func(t *testing.T, text, tag string) {
		got := InsertSoftHyphens(text, tag)
		if strings.ReplaceAll(got, shy, "") != strings.ReplaceAll(text, shy, "") {
			t.Fatalf("InsertSoftHyphens changed text other than adding soft hyphens: %q → %q", text, got)
		}
		for _, w := range strings.Fields(text) {
			_ = Hyphenate(w, tag)
		}
	})
}

// benchText is ordinary running Latvian prose with punctuation, numbers and
// a URL, about 1 KiB.
const benchText = `Latvijas Republikas Saeima ir pieņēmusi un Valsts prezidents izsludina šādu likumu. ` +
	`Likuma mērķis ir nodrošināt pašvaldību informācijas sistēmu savietojamību, uzņēmējdarbības ` +
	`attīstību un starptautiskās sadarbības stiprināšanu. Grāmatvedības dokumentācija jāsagatavo ` +
	`saskaņā ar normatīvajiem aktiem, un apstiprināšanas procedūra ietver atbildīgo darbinieku ` +
	`parakstus. Ministru kabinets līdz 2026. gada 1. decembrim izdod noteikumus par kārtību, kādā ` +
	`tiek veikta datu apmaiņa (sk. https://piemērs.lv/likumi/123). Sabiedrības līdzdalība lēmumu ` +
	`pieņemšanā tiek nodrošināta, publicējot projektus un organizējot apspriešanu; priekšlikumus ` +
	`var iesniegt elektroniski vai rakstveidā. Ja persona nepiekrīt lēmumam, tā var to apstrīdēt ` +
	`viena mēneša laikā no tā spēkā stāšanās dienas. `

func BenchmarkInsertSoftHyphens(b *testing.B) {
	text := strings.Repeat(benchText, 64) // ~64 KiB
	InsertSoftHyphens(benchText, "lv")    // exclude lazy pattern compilation
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	for b.Loop() {
		InsertSoftHyphens(text, "lv")
	}
}

func BenchmarkCompileLatvian(b *testing.B) {
	src := embedded("hyph-lv.pat.txt")
	b.ReportAllocs()
	for b.Loop() {
		compile(src, "", 2, 2)
	}
}
