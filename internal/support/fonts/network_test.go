package fonts

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestInstallCoreNetwork downloads the core set for real and checks that
// every family is then found under the name the catalog (and Typst) uses.
// Run with CROWDOC_FONT_NETWORK=1.
func TestInstallCoreNetwork(t *testing.T) {
	if os.Getenv("CROWDOC_FONT_NETWORK") != "1" {
		t.Skip("set CROWDOC_FONT_NETWORK=1 to download fonts")
	}
	useScanCache(t)
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := Install(ctx, dir, []string{"core"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	present, _ := Installed(dir)
	names, err := Scan([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	var core int
	for _, f := range catalog {
		if f.Set != "core" {
			continue
		}
		core++
		if !contains(present, f.Name) {
			t.Errorf("%s not installed", f.Name)
		}
		if !names[f.Name] {
			t.Errorf("%s: scan found %v, not the catalog name", f.Name, names)
		}
	}
	if len(present) != core {
		t.Errorf("%d families present, want the %d core ones", len(present), core)
	}
}
