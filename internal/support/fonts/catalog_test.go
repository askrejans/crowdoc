package fonts

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestCatalogIntegrity(t *testing.T) {
	cat := Catalog()
	if len(cat) < 40 {
		t.Fatalf("catalog has %d families, want at least 40", len(cat))
	}
	categories := map[string]bool{"serif": true, "sans": true, "mono": true, "math": true, "display": true, "emoji": true}
	licenses := map[string]bool{"OFL-1.1": true, "Apache-2.0": true, "UFL-1.0": true}
	names := map[string]bool{}
	dirs := map[string]bool{}
	urls := map[string]bool{}
	archives := map[string]string{}

	for _, f := range cat {
		lname := strings.ToLower(f.Name)
		if names[lname] {
			t.Errorf("duplicate family %q", f.Name)
		}
		names[lname] = true
		if d := familyDir(f.Name); d == "" || dirs[d] {
			t.Errorf("%s: install directory %q empty or shared", f.Name, d)
		} else {
			dirs[d] = true
		}
		if !categories[f.Category] {
			t.Errorf("%s: unknown category %q", f.Name, f.Category)
		}
		if !licenses[f.License] {
			t.Errorf("%s: licence %q is not OFL/Apache/UFL", f.Name, f.License)
		}
		if !contains(setNames, f.Set) {
			t.Errorf("%s: unknown set %q", f.Name, f.Set)
		}
		if !strings.HasPrefix(f.Homepage, "https://") {
			t.Errorf("%s: homepage %q is not https", f.Name, f.Homepage)
		}
		if len(f.Scripts) == 0 || len(f.Files) == 0 {
			t.Errorf("%s: scripts and files must not be empty", f.Name)
		}
		variable := false
		for _, file := range f.Files {
			checkFile(t, f.Name, file)
			key := file.URL + "#" + file.Member
			if urls[key] {
				t.Errorf("%s: duplicate source %s", f.Name, key)
			}
			urls[key] = true
			if file.Member != "" {
				pin := fmt.Sprintf("%d/%s", file.ArchiveSize, file.ArchiveSHA256)
				if prev, ok := archives[file.URL]; ok && prev != pin {
					t.Errorf("%s: archive %s pinned inconsistently", f.Name, file.URL)
				}
				archives[file.URL] = pin
			}
			variable = variable || strings.Contains(file.Name, "[")
		}
		if variable != f.Variable {
			t.Errorf("%s: Variable=%v but file names say %v", f.Name, f.Variable, variable)
		}
	}
}

func checkFile(t *testing.T, family string, f File) {
	t.Helper()
	if f.Name == "" || f.Name != filepath.Base(f.Name) || strings.ContainsAny(f.Name, `/\`) || strings.HasPrefix(f.Name, ".") {
		t.Errorf("%s: unsafe file name %q", family, f.Name)
	}
	if ext := strings.ToLower(filepath.Ext(f.Name)); ext != ".ttf" && ext != ".otf" {
		t.Errorf("%s: %s is not a TTF/OTF file", family, f.Name)
	}
	u, err := url.Parse(f.URL)
	if err != nil || u.Scheme != "https" {
		t.Errorf("%s: %s: URL %q is not https", family, f.Name, f.URL)
	} else if u.Host != "raw.githubusercontent.com" && u.Host != "github.com" {
		t.Errorf("%s: %s: unexpected host %s", family, f.Name, u.Host)
	}
	if strings.Contains(f.URL, "/google/fonts/") && !strings.Contains(f.URL, "/"+googleFontsCommit+"/") {
		t.Errorf("%s: %s: google/fonts URL not pinned to %s", family, f.Name, googleFontsCommit)
	}
	if f.Member == "" {
		if !strings.HasSuffix(f.URL, "/"+urlEscaper.Replace(f.Name)) {
			t.Errorf("%s: URL %s does not end in file name %s", family, f.URL, f.Name)
		}
		if f.ArchiveSHA256 != "" || f.ArchiveSize != 0 {
			t.Errorf("%s: %s: archive pin without a member", family, f.Name)
		}
	} else {
		if path.Base(f.Member) != f.Name || !strings.HasSuffix(f.URL, ".zip") {
			t.Errorf("%s: member %s of %s does not match file %s", family, f.Member, f.URL, f.Name)
		}
		if !sha256Hex.MatchString(f.ArchiveSHA256) || f.ArchiveSize <= 0 || f.ArchiveSize > 64<<20 {
			t.Errorf("%s: %s: archive not pinned (%d bytes, %q)", family, f.Name, f.ArchiveSize, f.ArchiveSHA256)
		}
	}
	if !sha256Hex.MatchString(f.SHA256) {
		t.Errorf("%s: %s: SHA-256 %q is not 64 lowercase hex digits", family, f.Name, f.SHA256)
	}
	if f.Size <= 0 || f.Size > 64<<20 {
		t.Errorf("%s: %s: implausible size %d", family, f.Name, f.Size)
	}
}

func TestCatalogCoversRequiredFamilies(t *testing.T) {
	for _, name := range []string{
		"EB Garamond", "Source Serif 4", "STIX Two Text", "Charis SIL", "Inter",
		"Atkinson Hyperlegible Next", "JetBrains Mono", "STIX Two Math", "Libertinus Math",
		"Fira Math", "Garamond-Math", "Noto Naskh Arabic", "Noto Sans Hebrew",
		"Noto Sans JP", "Noto Serif SC", "Noto Color Emoji",
	} {
		if _, ok := lookupFamily(name); !ok {
			t.Errorf("catalog lacks %s", name)
		}
	}
	for _, f := range Catalog() {
		if contains(f.Scripts, "japanese") && f.Set != "cjk" {
			t.Errorf("%s: CJK family belongs in the optional cjk set, not %s", f.Name, f.Set)
		}
	}
}

func TestCatalogReturnsCopy(t *testing.T) {
	a := Catalog()
	a[0].Name = "changed"
	a[0].Files[0].SHA256 = "changed"
	a[0].Scripts[0] = "changed"
	b := Catalog()
	if b[0].Name == "changed" || b[0].Files[0].SHA256 == "changed" || b[0].Scripts[0] == "changed" {
		t.Fatal("Catalog exposes package state")
	}
}

func TestFamilyDir(t *testing.T) {
	for in, want := range map[string]string{
		"EB Garamond":     "eb-garamond",
		"DM Sans 9pt":     "dm-sans-9pt",
		"Garamond-Math":   "garamond-math",
		"  Odd  -- Name ": "odd-name",
	} {
		if got := familyDir(in); got != want {
			t.Errorf("familyDir(%q) = %q, want %q", in, got, want)
		}
	}
}
