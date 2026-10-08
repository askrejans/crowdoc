package fonts

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font/sfnt"
)

// maxScanFileSize skips files no real font reaches; it bounds memory when a
// font directory contains something unexpected.
const maxScanFileSize = 256 << 20

// scanCacheVersion invalidates cached names when the naming rules change.
const scanCacheVersion = 1

// scanCachePath locates the scan cache; tests point it elsewhere.
var scanCachePath = func() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "crowdoc", "font-scan.json")
}

type scanEntry struct {
	Size     int64    `json:"size"`
	ModTime  int64    `json:"mtime"`
	Families []string `json:"families,omitempty"`
}

type scanCache struct {
	Version int                  `json:"version"`
	Files   map[string]scanEntry `json:"files"`
}

// Scan walks dirs recursively and returns the family names (as Typst names
// them) of every OTF/TTF/TTC/OTC font found. Unreadable or malformed fonts
// are skipped. Names are cached on disk keyed by path, size and mtime, so
// repeated scans of large trees such as TeX Live are cheap. The map is
// usable even when an error is returned.
func Scan(dirs []string) (map[string]bool, error) {
	idx, err := ScanIndex(dirs)
	found := make(map[string]bool, len(idx))
	for f := range idx {
		found[f] = true
	}
	return found, err
}

// ScanIndex is like Scan but maps every family name to the font files that
// provide it, so callers can hand an engine only the directories it needs.
func ScanIndex(dirs []string) (map[string][]string, error) {
	cachePath := scanCachePath()
	old := loadScanCache(cachePath)
	next := scanCache{Version: scanCacheVersion, Files: map[string]scanEntry{}}
	found := map[string][]string{}
	var roots []string
	var firstErr error
	changed := false

	for _, dir := range dirs {
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) && firstErr == nil {
				firstErr = err
			}
			continue
		}
		roots = append(roots, root)
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if path == root {
					return err
				}
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() || !isFontFile(path) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			e, ok := old.Files[path]
			if !ok || e.Size != info.Size() || e.ModTime != info.ModTime().UnixNano() {
				e = scanEntry{Size: info.Size(), ModTime: info.ModTime().UnixNano(), Families: fontFamilies(path, info.Size())}
				changed = true
			}
			next.Files[path] = e
			for _, f := range e.Families {
				found[f] = append(found[f], path)
			}
			return nil
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Keep cached entries of trees this scan did not visit; entries of
	// visited trees that were not seen again belong to deleted files.
	for path, e := range old.Files {
		if !underAny(path, roots) {
			next.Files[path] = e
		} else if _, ok := next.Files[path]; !ok {
			changed = true
		}
	}
	if changed {
		saveScanCache(cachePath, next)
	}
	return found, firstErr
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	}
	return false
}

// fontFamilies parses a font file (or collection) and returns the Typst
// family names it contains.
func fontFamilies(path string, size int64) []string {
	if size > maxScanFileSize {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// The ReaderAt variant of the parser fails on fonts whose table
	// directory points past the file end; the byte-slice variant does not.
	c, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil
	}
	var buf sfnt.Buffer
	var out []string
	for i := 0; i < c.NumFonts(); i++ {
		f, err := c.Font(i)
		if err != nil {
			continue
		}
		ps, _ := f.Name(&buf, sfnt.NameIDPostScript)
		fam := firstName(f, &buf, sfnt.NameIDFamily, sfnt.NameIDTypographicFamily, sfnt.NameIDFull)
		if name := typstFamily(ps, fam); name != "" && !contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

func firstName(f *sfnt.Font, buf *sfnt.Buffer, ids ...sfnt.NameID) string {
	for _, id := range ids {
		if s, err := f.Name(buf, id); err == nil && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func underAny(path string, roots []string) bool {
	for _, r := range roots {
		if path == r || strings.HasPrefix(path, r+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func loadScanCache(path string) scanCache {
	var c scanCache
	if path == "" {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &c) != nil || c.Version != scanCacheVersion {
		return scanCache{}
	}
	return c
}

// saveScanCache is best effort: a read-only cache directory only costs
// speed, never correctness.
func saveScanCache(path string, c scanCache) {
	if path == "" {
		return
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".font-scan-*.json")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
	}
}
