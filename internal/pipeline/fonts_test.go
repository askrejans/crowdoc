package pipeline

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/askrejans/crowdoc/v2/internal/typst"
)

func TestSelectedFontDirectoriesIncludePacksAndCustomFonts(t *testing.T) {
	root := t.TempDir()
	pack := filepath.Join(root, "packs", "source-sans")
	custom := filepath.Join(root, "custom", "serif")
	tex := filepath.Join(root, "tex", "math")
	idx := map[string][]string{
		"Source Sans 3": {filepath.Join(pack, "regular.ttf"), filepath.Join(pack, "bold.ttf")},
		"Custom Serif":  {filepath.Join(custom, "regular.otf")},
		"STIX Two Math": {filepath.Join(tex, "math.otf")},
		"Unused":        {filepath.Join(root, "packs", "unused", "regular.ttf")},
	}
	set := typst.FontSet{Main: []string{"custom serif"}, Sans: []string{"source sans 3"}, Math: []string{"stix two math"}}
	roots := []string{filepath.Join(root, "packs"), filepath.Join(root, "custom"), filepath.Join(root, "tex")}
	got := extraFontDirs(set, idx, roots)
	want := []string{pack, custom, tex}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected font directories = %v, want %v", got, want)
	}
}

func TestSelectedFontDirectoriesResolveSymlinkedRoots(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked-fonts")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	real, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	got := extraFontDirs(typst.FontSet{Main: []string{"Custom"}}, map[string][]string{
		"Custom": {filepath.Join(real, "regular.otf")},
	}, []string{link})
	if len(got) != 1 || got[0] != real {
		t.Fatalf("symlinked custom font directory lost: %v", got)
	}
}
