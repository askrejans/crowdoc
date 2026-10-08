package fonts

import (
	"reflect"
	"strings"
	"testing"
)

// texLiveOnly are families pairings use when TeX Live provides them; they are
// not downloadable because their licences are not OFL/Apache/UFL or because
// a catalog family already covers the role.
var texLiveOnly = []string{
	"New Computer Modern Sans", "New Computer Modern Mono", "Latin Modern Sans", "Latin Modern Mono",
	"XCharter", "XCharter Math", "Source Serif Pro", "Source Sans Pro",
}

func TestPairingsIntegrity(t *testing.T) {
	ps := Pairings()
	if len(ps) < 16 {
		t.Fatalf("%d pairings, want at least 16", len(ps))
	}
	categories := map[string]bool{"book": true, "academic": true, "business": true, "modern": true, "accessible": true, "multilingual": true}
	keys := map[string]string{}
	claim := func(key, owner string) {
		if prev, ok := keys[key]; ok {
			t.Errorf("name %q used by both %s and %s", key, prev, owner)
		}
		keys[key] = owner
	}
	for _, p := range ps {
		claim(normalizeName(p.Name), p.Name)
		for _, a := range pairingAliases[p.Name] {
			if a != normalizeName(a) {
				t.Errorf("%s: alias %q is not normalised", p.Name, a)
			}
			claim(a, p.Name)
		}
		if p.Title == "" || p.Description == "" || len(p.Uses) == 0 {
			t.Errorf("%s: title, description and uses are required", p.Name)
		}
		if !categories[p.Category] {
			t.Errorf("%s: unknown category %q", p.Name, p.Category)
		}
		checkChain(t, p.Name, RoleMain, p.Main, embeddedSerif, embeddedCM)
		checkChain(t, p.Name, RoleSans, p.Sans, embeddedCM)
		checkChain(t, p.Name, RoleMono, p.Mono, embeddedMono)
		checkChain(t, p.Name, RoleMath, p.Math, embeddedMath)
		if len(p.Heading) > 0 {
			checkChain(t, p.Name, RoleHeading, p.Heading, embeddedSerif, embeddedCM)
		}
	}
	for name := range pairingAliases {
		if _, ok := LookupPairing(name); !ok {
			t.Errorf("aliases defined for unknown pairing %q", name)
		}
	}
}

func checkChain(t *testing.T, pairing string, role Role, chain []string, finals ...string) {
	t.Helper()
	if len(chain) == 0 {
		t.Errorf("%s/%s: empty chain", pairing, role)
		return
	}
	if last := chain[len(chain)-1]; !containsFold(finals, last) {
		t.Errorf("%s/%s: chain ends in %q, want one of %v so text never disappears", pairing, role, last, finals)
	}
	seen := map[string]bool{}
	for _, n := range chain {
		k := strings.ToLower(n)
		if seen[k] {
			t.Errorf("%s/%s: %q listed twice", pairing, role, n)
		}
		seen[k] = true
		_, inCatalog := lookupFamily(n)
		if !inCatalog && !isEmbedded(n) && !containsFold(systemSans, n) && !containsFold(texLiveOnly, n) {
			t.Errorf("%s/%s: %q is neither in the catalog nor an embedded, system or TeX Live family", pairing, role, n)
		}
	}
	head := chain[0]
	f, inCatalog := lookupFamily(head)
	switch {
	case role == RoleMath:
		if inCatalog && f.Category != "math" || !inCatalog && !strings.Contains(head, "Math") {
			t.Errorf("%s: math chain starts with non-math family %q", pairing, head)
		}
	case (role == RoleMain || role == RoleSans) && inCatalog:
		// Typst never slants a roman, so a body face without italics would
		// silently drop emphasis.
		if !hasItalic(f) {
			t.Errorf("%s/%s: %q has no italic", pairing, role, head)
		}
	}
}

func hasItalic(f Family) bool {
	for _, file := range f.Files {
		if strings.Contains(file.Name, "Italic") {
			return true
		}
	}
	return false
}

func TestLookupPairing(t *testing.T) {
	for in, want := range map[string]string{
		"classic":           "classic",
		"Computer Modern":   "computer-modern",
		"computer_modern":   "computer-modern",
		"CM":                "computer-modern",
		"LIBERTINUS":        "libertine",
		"Atkinson":          "accessible",
		" EB-Garamond ":     "classic",
		"multilingual":      "noto",
		"Libre Baskerville": "baskerville",
	} {
		p, ok := LookupPairing(in)
		if !ok || p.Name != want {
			t.Errorf("LookupPairing(%q) = %q, %v; want %q", in, p.Name, ok, want)
		}
	}
	for _, in := range []string{"", "nope", "comic"} {
		if p, ok := LookupPairing(in); ok {
			t.Errorf("LookupPairing(%q) unexpectedly found %q", in, p.Name)
		}
	}
	p, _ := LookupPairing("classic")
	p.Main[0] = "changed"
	if q, _ := LookupPairing("classic"); q.Main[0] == "changed" {
		t.Fatal("LookupPairing exposes package state")
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		chain     []string
		available map[string]bool
		want      []string
	}{
		{"nothing installed keeps embedded tail", []string{"EB Garamond", "Libertinus Serif"}, nil, []string{"Libertinus Serif"}},
		{"installed family first", []string{"EB Garamond", "Libertinus Serif"}, map[string]bool{"EB Garamond": true}, []string{"EB Garamond", "Libertinus Serif"}},
		{"match ignores case", []string{"Source Sans 3", "Arial", "New Computer Modern"}, map[string]bool{"source sans 3": true, "ARIAL": true}, []string{"Source Sans 3", "Arial", "New Computer Modern"}},
		{"false entries are absent", []string{"Inter", "New Computer Modern"}, map[string]bool{"Inter": false}, []string{"New Computer Modern"}},
		{"embedded kept mid-chain", []string{"New Computer Modern", "X", "Libertinus Serif"}, nil, []string{"New Computer Modern", "Libertinus Serif"}},
		{"final kept even if unknown", []string{"A", "B"}, nil, []string{"B"}},
		{"duplicates dropped", []string{"Inter", "inter", "DejaVu Sans Mono"}, map[string]bool{"Inter": true}, []string{"Inter", "DejaVu Sans Mono"}},
		{"empty", nil, nil, nil},
	}
	for _, tt := range tests {
		if got := Resolve(tt.chain, tt.available); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: Resolve = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFontsScriptFallbacks(t *testing.T) {
	classic, _ := LookupPairing("classic")
	modern, _ := LookupPairing("modern")
	avail := map[string]bool{
		"EB Garamond": true, "Inter": true, "Garamond-Math": true,
		"Noto Serif JP": true, "Noto Serif SC": true, "Noto Naskh Arabic": true,
		"Noto Sans JP": true, "Noto Sans Arabic": true, "Noto Color Emoji": true,
	}
	tests := []struct {
		name string
		p    Pairing
		role Role
		lang string
		want []string
	}{
		{"japanese serif body", classic, RoleMain, "ja-JP",
			[]string{"EB Garamond", "Noto Serif JP", "Libertinus Serif", "Noto Naskh Arabic", "Noto Serif SC", "Noto Color Emoji"}},
		{"chinese defaults to simplified", classic, RoleMain, "zh",
			[]string{"EB Garamond", "Noto Serif SC", "Libertinus Serif", "Noto Naskh Arabic", "Noto Serif JP", "Noto Color Emoji"}},
		{"english gets all scripts after the fallback", classic, RoleMain, "en",
			[]string{"EB Garamond", "Libertinus Serif", "Noto Naskh Arabic", "Noto Serif SC", "Noto Serif JP", "Noto Color Emoji"}},
		{"arabic sans body uses sans script faces", modern, RoleMain, "ar",
			[]string{"Inter", "Noto Sans Arabic", "Libertinus Serif", "Noto Sans JP", "Noto Color Emoji"}},
		{"mono uses sans script faces", classic, RoleMono, "ja",
			[]string{"Noto Sans JP", "DejaVu Sans Mono", "Noto Sans Arabic", "Noto Color Emoji"}},
		{"math gets no script faces", classic, RoleMath, "ja",
			[]string{"Garamond-Math", "New Computer Modern Math"}},
	}
	for _, tt := range tests {
		if got := tt.p.Fonts(tt.role, tt.lang, avail); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s:\n got %q\nwant %q", tt.name, got, tt.want)
		}
	}
}

func TestChainHeadingFallsBackToMain(t *testing.T) {
	classic, _ := LookupPairing("classic")
	if got := classic.Chain(RoleHeading); !reflect.DeepEqual(got, classic.Main) {
		t.Errorf("heading chain = %q, want main %q", got, classic.Main)
	}
	editorial, _ := LookupPairing("editorial")
	if got := editorial.Chain(RoleHeading); got[0] != "Playfair Display" {
		t.Errorf("editorial heading starts with %q", got[0])
	}
	if got := classic.Chain("bogus"); got != nil {
		t.Errorf("unknown role returned %q", got)
	}
}

func TestLangScript(t *testing.T) {
	for in, want := range map[string]string{
		"":           "",
		"en":         "",
		"lv-LV":      "",
		"ar":         "arabic",
		"fa_IR":      "arabic",
		"he":         "hebrew",
		"ja":         "ja",
		"ko-KR":      "ko",
		"zh":         "zh-hans",
		"zh-Hans-CN": "zh-hans",
		"zh-TW":      "zh-hant",
		"zh-Hant":    "zh-hant",
		"ZH-hk":      "zh-hant",
	} {
		if got := langScript(in); got != want {
			t.Errorf("langScript(%q) = %q, want %q", in, got, want)
		}
	}
}
