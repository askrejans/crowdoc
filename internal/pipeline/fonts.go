package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/engine"
	"github.com/askrejans/crowdoc/v2/internal/support/fonts"
	"github.com/askrejans/crowdoc/v2/internal/typst"
)

func fontDirs(opts Options) []string {
	dirs := opts.FontDirs
	if typstEngine, ok := opts.Engine.(*engine.Typst); ok {
		dirs = append(append([]string(nil), dirs...), typstEngine.FontPaths...)
	}
	return fonts.SearchDirs(dirs...)
}

// Default Typst engines are shared per configuration so the font list is
// computed once per process.
var (
	engineMu    sync.Mutex
	engineCache = map[string]*engine.Typst{}
)

func sharedTypst(opts Options) (*engine.Typst, error) {
	path, err := engine.FindTypst()
	if err != nil {
		return nil, err
	}
	dirs := fontDirs(opts)
	key := fmt.Sprint(path, dirs, opts.DeterministicFonts)
	engineMu.Lock()
	defer engineMu.Unlock()
	if e, ok := engineCache[key]; ok {
		return e, nil
	}
	e := &engine.Typst{Path: path, FontPaths: dirs, IgnoreSystemFonts: opts.DeterministicFonts}
	engineCache[key] = e
	return e, nil
}

// fontIndex maps available family names to font files. A custom engine
// reports its own families (no files); otherwise font directories are
// scanned (cached on disk), including TeX Live and system fonts.
func fontIndex(ctx context.Context, opts Options) map[string][]string {
	if opts.Engine != nil {
		if _, isTypst := opts.Engine.(*engine.Typst); !isTypst {
			idx := map[string][]string{}
			if m, err := opts.Engine.Fonts(ctx); err == nil {
				for f := range m {
					idx[f] = nil
				}
			}
			return idx
		}
	}
	dirs := fontDirs(opts)
	dirs = append(dirs, fonts.TeXLiveDirs()...)
	deterministic := opts.DeterministicFonts
	if typstEngine, ok := opts.Engine.(*engine.Typst); ok {
		deterministic = deterministic || typstEngine.IgnoreSystemFonts
	}
	if !deterministic {
		dirs = append(dirs, fonts.SystemDirs()...)
	}
	idx, _ := fonts.ScanIndex(dirs)
	return idx
}

// resolveFonts picks the pairing (document, then style default), applies
// per-role overrides and filters every chain to installed families.
func resolveFonts(ctx context.Context, doc *ast.Document, st *typst.Style, opts Options) (typst.FontSet, []string, []string) {
	var warns []string
	idx := fontIndex(ctx, opts)
	avail := make(map[string]bool, len(idx))
	for f := range idx {
		avail[strings.ToLower(f)] = true
	}
	name := st.Pairing
	if doc.Meta.Fonts.Pairing != "" {
		if _, ok := fonts.LookupPairing(doc.Meta.Fonts.Pairing); ok {
			name = doc.Meta.Fonts.Pairing
		} else {
			warns = append(warns, fmt.Sprintf("unknown typeface pairing %q (see --list-fonts)", doc.Meta.Fonts.Pairing))
		}
	}
	p, ok := fonts.LookupPairing(name)
	if !ok {
		p, _ = fonts.LookupPairing("libertine")
	}
	lang := doc.Meta.Lang
	role := func(r fonts.Role, override string) []string {
		chain := p.Fonts(r, lang, avail)
		if override == "" {
			return chain
		}
		// A pairing name selects that pairing's face for the role.
		if op, ok := fonts.LookupPairing(override); ok {
			return op.Fonts(r, lang, avail)
		}
		if !avail[strings.ToLower(override)] {
			warns = append(warns, fmt.Sprintf("font %q is not installed; using %s", override, firstOf(chain)))
			return chain
		}
		return append([]string{override}, chain...)
	}
	set := typst.FontSet{
		Main:    role(fonts.RoleMain, doc.Meta.Fonts.Main),
		Sans:    role(fonts.RoleSans, doc.Meta.Fonts.Sans),
		Mono:    role(fonts.RoleMono, doc.Meta.Fonts.Mono),
		Math:    role(fonts.RoleMath, doc.Meta.Fonts.Math),
		Heading: role(fonts.RoleHeading, doc.Meta.Fonts.Sans),
	}
	if doc.Meta.Fonts.Sans == "" && len(p.Heading) == 0 {
		// Without a dedicated heading face, styles choose between the main
		// and sans chains themselves.
		set.Heading = set.Sans
	}
	roots := append(fontDirs(opts), fonts.TeXLiveDirs()...)
	return set, warns, extraFontDirs(set, idx, roots)
}

// extraFontDirs passes the directories of selected non-system families to
// the engine. This includes installed font packs and caller font directories,
// even when a caller-provided Typst engine has no base font paths of its own.
// Passing only selected subdirectories keeps large font trees cheap to use.
func extraFontDirs(set typst.FontSet, idx map[string][]string, roots []string) []string {
	if len(roots) == 0 {
		return nil
	}
	// ScanIndex resolves symlinked roots; compare in the same namespace.
	resolved := make([]string, len(roots))
	for i, root := range roots {
		resolved[i] = root
		if real, err := filepath.EvalSymlinks(root); err == nil {
			resolved[i] = real
		}
	}
	lower := make(map[string][]string, len(idx))
	for f, paths := range idx {
		lower[strings.ToLower(f)] = append(lower[strings.ToLower(f)], paths...)
	}
	seen := map[string]bool{}
	var out []string
	for _, chain := range [][]string{set.Main, set.Sans, set.Mono, set.Math, set.Heading} {
		for _, fam := range chain {
			for _, p := range lower[strings.ToLower(fam)] {
				for _, r := range resolved {
					if strings.HasPrefix(p, r+string(filepath.Separator)) {
						d := filepath.Dir(p)
						if !seen[d] {
							seen[d] = true
							out = append(out, d)
						}
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func firstOf(c []string) string {
	if len(c) == 0 {
		return "the default"
	}
	return c[0]
}
