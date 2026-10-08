// Package fonts curates open-licensed typefaces and typeface pairings for the
// typesetting engines, downloads them on demand into a cache directory and
// finds the families that are already available (cache, TeX Live).
//
// Family names follow Typst's naming (name ID 1 with style words trimmed),
// because that is what the engine matches against. Every pairing chain ends
// in a family Typst embeds, so output never depends on a download.
package fonts

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Role is the job a typeface does in a document.
type Role string

// Roles a pairing assigns typefaces to.
const (
	RoleMain    Role = "main"
	RoleSans    Role = "sans"
	RoleMono    Role = "mono"
	RoleMath    Role = "math"
	RoleHeading Role = "heading"
)

// File is one font file pinned by size and SHA-256. URL serves the font
// itself, or, when Member is set, a zip release archive (pinned by
// ArchiveSize and ArchiveSHA256) that contains it at path Member.
type File struct {
	Name   string
	URL    string
	SHA256 string
	Size   int64

	Member        string
	ArchiveSHA256 string
	ArchiveSize   int64
}

// Family is a catalog entry. Name is the family name as Typst reports it.
// Category is one of serif, sans, mono, math, display, emoji. Set groups
// families for installation: core, extended, cjk, emoji.
type Family struct {
	Name     string
	Category string
	License  string
	Homepage string
	Scripts  []string
	Set      string
	Files    []File
	Variable bool
}

// Size is the total size of the family's font files in bytes.
func (f Family) Size() int64 {
	var n int64
	for _, file := range f.Files {
		n += file.Size
	}
	return n
}

// setNames are the installation sets in the order they are offered.
var setNames = []string{"core", "extended", "cjk", "emoji"}

// EnvDir overrides the font cache directory.
const EnvDir = "CROWDOC_FONTS"

// DefaultDir is where Install puts fonts: $CROWDOC_FONTS, or the user cache
// directory's crowdoc/fonts.
func DefaultDir() string {
	if d := os.Getenv(EnvDir); d != "" {
		return d
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "crowdoc", "fonts")
}

// TeXLiveDirs returns TeX Live's OpenType and TrueType font trees, if
// TeX Live is installed.
func TeXLiveDirs() []string { return existing(texLiveDirs()) }

// SystemDirs returns the operating system's font directories.
func SystemDirs() []string {
	home, _ := os.UserHomeDir()
	var c []string
	switch runtime.GOOS {
	case "darwin":
		c = []string{"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library", "Fonts")}
	case "windows":
		c = []string{filepath.Join(os.Getenv("WINDIR"), "Fonts"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows", "Fonts")}
	default:
		c = []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".fonts"), filepath.Join(home, ".local", "share", "fonts")}
	}
	return existing(c)
}

// SearchDirs lists the existing font directories to hand to an engine in
// priority order: extra directories, then the installed font cache.
// TeX Live trees are large, so callers pass only the subdirectories they
// need (see ScanIndex and TeXLiveDirs).
func SearchDirs(extra ...string) []string {
	cands := append([]string{}, extra...)
	cands = append(cands, DefaultDir())
	return existing(cands)
}

func existing(cands []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, d := range cands {
		if d == "" {
			continue
		}
		d = filepath.Clean(d)
		if seen[d] {
			continue
		}
		seen[d] = true
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			out = append(out, d)
		}
	}
	return out
}

var (
	texLiveOnce sync.Once
	texLiveRoot string
)

// texLiveDirs returns TeX Live's font trees. The location is asked from
// kpsewhich; GUI processes often lack TeX in PATH, so well-known install
// locations are tried as well.
func texLiveDirs() []string {
	texLiveOnce.Do(func() { texLiveRoot = findTexmfDist() })
	if texLiveRoot == "" {
		return nil
	}
	fonts := filepath.Join(texLiveRoot, "fonts")
	return []string{filepath.Join(fonts, "opentype"), filepath.Join(fonts, "truetype")}
}

func findTexmfDist() string {
	// Well-known locations first: starting kpsewhich costs a noticeable
	// fraction of a second.
	for _, d := range []string{
		newestGlob("/usr/local/texlive/20*/texmf-dist"),
		"/Library/TeX/Root/texmf-dist",
		newestGlob("/opt/texlive/20*/texmf-dist"),
		newestGlob(`C:\texlive\20*\texmf-dist`),
		"/usr/share/texlive/texmf-dist",
		"/usr/share/texmf-dist",
	} {
		if fi, err := os.Stat(filepath.Join(d, "fonts")); d != "" && err == nil && fi.IsDir() {
			return d
		}
	}
	kpse := []string{"kpsewhich"}
	kpse = append(kpse, newestGlob("/usr/local/texlive/20*/bin/*/kpsewhich"))
	kpse = append(kpse, "/Library/TeX/texbin/kpsewhich")
	for _, k := range kpse {
		if k == "" {
			continue
		}
		if p, err := exec.LookPath(k); err == nil {
			if d := kpsewhichDist(p); d != "" {
				return d
			}
		}
	}
	for _, d := range []string{
		newestGlob("/usr/local/texlive/20*/texmf-dist"),
		newestGlob("/opt/texlive/20*/texmf-dist"),
		newestGlob(`C:\texlive\20*\texmf-dist`),
		"/Library/TeX/Root/texmf-dist",
		"/usr/share/texlive/texmf-dist",
		"/usr/share/texmf-dist",
	} {
		if fi, err := os.Stat(filepath.Join(d, "fonts")); d != "" && err == nil && fi.IsDir() {
			return d
		}
	}
	return ""
}

func kpsewhichDist(kpsewhich string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, kpsewhich, "-var-value=TEXMFDIST").Output()
	if err != nil {
		return ""
	}
	d := strings.TrimSpace(string(out))
	if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
		return ""
	}
	return d
}

// newestGlob returns the lexically last match, i.e. the newest yearly
// TeX Live release.
func newestGlob(pattern string) string {
	m, _ := filepath.Glob(pattern)
	if len(m) == 0 {
		return ""
	}
	sort.Strings(m)
	return m[len(m)-1]
}
