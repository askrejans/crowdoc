package fonts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestTypographicFamily(t *testing.T) {
	for in, want := range map[string]string{
		// Cases from Typst's own test suite.
		"Atma Light":                    "Atma",
		"eras bold":                     "eras",
		"footlight mt light":            "footlight mt",
		"times new roman":               "times new roman",
		"noto sans mono cond sembd":     "noto sans mono",
		"noto serif SEMCOND sembd":      "noto serif",
		"crimson text":                  "crimson text",
		"footlight light":               "footlight",
		"Noto Sans":                     "Noto Sans",
		"Noto Sans Light":               "Noto Sans",
		"Noto Sans Semicondensed Heavy": "Noto Sans",
		"Familx":                        "Familx",
		"Font Ultra":                    "Font Ultra",
		"Font Ultra Bold":               "Font",
		// Name ID 1 values of catalog fonts, checked against `typst fonts`.
		"Source Sans 3 ExtraLight":   "Source Sans 3",
		"Montserrat Thin":            "Montserrat",
		"IBM Plex Mono SemiBold":     "IBM Plex Mono",
		"Libertinus Serif SemiBold":  "Libertinus Serif",
		"DM Sans 9pt":                "DM Sans 9pt",
		"Newsreader 16pt":            "Newsreader 16pt",
		"Garamond-Math":              "Garamond-Math",
		"  .SF Compact Display Bold": "SF Compact Display",
		"Ünïcode Bold":               "Ünïcode",
	} {
		if got := typographicFamily(in); got != want {
			t.Errorf("typographicFamily(%q) = %q, want %q", in, got, want)
		}
	}
	if got := typstFamily("NewCMSans10-Regular", "NewComputerModernSans10"); got != "New Computer Modern Sans" {
		t.Errorf("exception not applied: %q", got)
	}
}

// texLiveFont finds a font file shipped by TeX Live, skipping the test when
// TeX Live is not installed.
func texLiveFont(t *testing.T, rel string) string {
	t.Helper()
	for _, d := range texLiveDirs() {
		p := filepath.Join(d, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skipf("TeX Live font %s not found", rel)
	return ""
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func useScanCache(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cache", "scan.json")
	old := scanCachePath
	scanCachePath = func() string { return path }
	t.Cleanup(func() { scanCachePath = old })
	return path
}

func TestScan(t *testing.T) {
	serif := texLiveFont(t, "public/libertinus-fonts/LibertinusSerif-Semibold.otf")
	cmsans := texLiveFont(t, "public/newcomputermodern/NewCMSans10-Regular.otf")
	cachePath := useScanCache(t)

	// Scan reports resolved paths (macOS temp dirs live behind a symlink).
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	semibold := filepath.Join(dir, "nested", "deeper", "LibertinusSerif-Semibold.otf")
	copyFile(t, serif, semibold)
	copyFile(t, cmsans, filepath.Join(dir, "NewCMSans10-Regular.OTF"))
	// Junk that must be ignored without failing the scan.
	if err := os.WriteFile(filepath.Join(dir, "broken.ttf"), []byte("not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Scan([]string{dir, filepath.Join(dir, "does-not-exist")})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"Libertinus Serif": true, "New Computer Modern Sans": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %v, want %v", got, want)
	}

	// The cache must now answer for unchanged files: plant a sentinel name
	// and expect it back.
	c := loadScanCache(cachePath)
	e, ok := c.Files[semibold]
	if !ok {
		t.Fatalf("cache lacks %s: %v", semibold, c.Files)
	}
	if _, ok := c.Files[filepath.Join(dir, "broken.ttf")]; !ok {
		t.Error("unparseable fonts should be cached too, so they are not re-read every time")
	}
	e.Families = []string{"Sentinel"}
	c.Files[semibold] = e
	writeCache(t, cachePath, c)
	got, _ = Scan([]string{dir})
	if !got["Sentinel"] || got["Libertinus Serif"] {
		t.Fatalf("cached entry not used: %v", got)
	}

	// A changed mtime invalidates the entry.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(semibold, later, later); err != nil {
		t.Fatal(err)
	}
	got, _ = Scan([]string{dir})
	if got["Sentinel"] || !got["Libertinus Serif"] {
		t.Fatalf("stale cache entry used: %v", got)
	}

	// Deleted files leave the cache; entries of other trees stay.
	c = loadScanCache(cachePath)
	c.Files["/elsewhere/Other.otf"] = scanEntry{Size: 1, Families: []string{"Other"}}
	writeCache(t, cachePath, c)
	if err := os.Remove(semibold); err != nil {
		t.Fatal(err)
	}
	got, _ = Scan([]string{dir})
	if got["Libertinus Serif"] || got["Other"] {
		t.Fatalf("Scan = %v after delete", got)
	}
	c = loadScanCache(cachePath)
	if _, ok := c.Files[semibold]; ok {
		t.Error("deleted file still cached")
	}
	if _, ok := c.Files["/elsewhere/Other.otf"]; !ok {
		t.Error("cache entry of an unscanned tree was dropped")
	}
}

func TestScanWithoutCache(t *testing.T) {
	src := texLiveFont(t, "public/libertinus-fonts/LibertinusMath-Regular.otf")
	old := scanCachePath
	scanCachePath = func() string { return "" }
	t.Cleanup(func() { scanCachePath = old })
	dir := t.TempDir()
	copyFile(t, src, filepath.Join(dir, "m.otf"))
	got, err := Scan([]string{dir})
	if err != nil || !got["Libertinus Math"] {
		t.Fatalf("Scan = %v, %v", got, err)
	}
}

func writeCache(t *testing.T, path string, c scanCache) {
	t.Helper()
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
