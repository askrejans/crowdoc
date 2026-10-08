package crowdoc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/support/fonts"
	"github.com/askrejans/crowdoc/v2/internal/support/locale"
	"github.com/askrejans/crowdoc/v2/internal/transform/cite"
	"github.com/askrejans/crowdoc/v2/internal/typst"
)

// StyleInfo describes a built-in style.
type StyleInfo struct {
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	Uses          []string `json:"uses"`
	Pairing       string   `json:"default_fonts"`
	CitationStyle string   `json:"default_citation_style"`
	Aliases       []string `json:"aliases,omitempty"`
}

// Styles lists the built-in styles.
func Styles() []StyleInfo {
	var out []StyleInfo
	for _, s := range typst.Styles() {
		out = append(out, StyleInfo{
			Name: s.Name, Title: s.Title, Description: s.Description, Category: s.Category,
			Uses: s.Uses, Pairing: s.Pairing, CitationStyle: s.CitationStyle, Aliases: s.Aliases,
		})
	}
	return out
}

// ExportStyle writes the Typst source of a style (as template.typ) and the
// shared library (crowdoc.typ) into dir so it can be customised and used
// with Options.TemplatePath.
func ExportStyle(name, dir string) ([]string, error) {
	s, ok := typst.LookupStyle(name)
	if !ok {
		return nil, fmt.Errorf("unknown style %q", name)
	}
	src, err := typst.StyleSource(s)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	tmpl := filepath.Join(dir, s.Name+".typ")
	lib := filepath.Join(dir, "crowdoc.typ")
	if err := os.WriteFile(tmpl, []byte(src), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(lib, []byte(typst.LibrarySource()), 0o644); err != nil {
		return nil, err
	}
	return []string{tmpl, lib}, nil
}

// WriteBundle writes a rendered Typst project to dir.
func WriteBundle(b *Bundle, dir string) error {
	names := make([]string, 0, len(b.Files))
	for n := range b.Files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		clean := filepath.Clean(filepath.FromSlash(n))
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("invalid bundle path %q", n)
		}
		p := filepath.Join(dir, clean)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, b.Files[n], 0o644); err != nil {
			return err
		}
	}
	return nil
}

// ColorScheme is a named colour palette.
type ColorScheme struct {
	Name        string            `json:"name"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Colors      map[string]string `json:"colors"`
}

// ColorSchemes lists the built-in colour schemes.
func ColorSchemes() []ColorScheme {
	var out []ColorScheme
	for _, p := range typst.Palettes() {
		colors := make(map[string]string, len(p.Colors))
		for k, v := range p.Colors {
			colors[k] = v
		}
		out = append(out, ColorScheme{Name: p.Name, Title: p.Title, Description: p.Description, Colors: colors})
	}
	return out
}

// ColorRoles lists the colour roles a scheme or override may set.
func ColorRoles() []string { return append([]string(nil), typst.ColorKeys...) }

// StyleSource returns the Typst source of a built-in style.
func StyleSource(name string) (string, error) {
	s, ok := typst.LookupStyle(name)
	if !ok {
		return "", fmt.Errorf("unknown style %q", name)
	}
	return typst.StyleSource(s)
}

// FontPairing describes a typeface pairing.
type FontPairing struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Uses        []string `json:"uses,omitempty"`
	Main        string   `json:"main"`
	Sans        string   `json:"sans"`
	Mono        string   `json:"mono"`
	Math        string   `json:"math"`
	Heading     string   `json:"heading,omitempty"`
}

// FontPairings lists the typeface pairings.
func FontPairings() []FontPairing {
	var out []FontPairing
	first := func(c []string) string {
		if len(c) == 0 {
			return ""
		}
		return c[0]
	}
	for _, p := range fonts.Pairings() {
		out = append(out, FontPairing{
			Name: p.Name, Title: p.Title, Description: p.Description, Category: p.Category, Uses: p.Uses,
			Main: first(p.Main), Sans: first(p.Sans), Mono: first(p.Mono), Math: first(p.Math), Heading: first(p.Heading),
		})
	}
	return out
}

// FontFamily describes a downloadable font family.
type FontFamily struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Set      string   `json:"set"`
	License  string   `json:"license"`
	Scripts  []string `json:"scripts,omitempty"`
	Bytes    int64    `json:"bytes"`
}

// FontFamilies lists the curated, openly licensed font families that
// InstallFonts can download.
func FontFamilies() []FontFamily {
	var out []FontFamily
	for _, f := range fonts.Catalog() {
		out = append(out, FontFamily{Name: f.Name, Category: f.Category, Set: f.Set, License: f.License, Scripts: f.Scripts, Bytes: f.Size()})
	}
	return out
}

// FontDirs returns the font directories crowdoc uses: extra, the installed
// font cache, then TeX Live font directories when present.
func FontDirs(extra []string) []string { return fonts.SearchDirs(extra...) }

// InstallFonts downloads font sets ("core", "extended", "cjk", "emoji",
// "all") or individual families into the font cache, verifying checksums.
func InstallFonts(ctx context.Context, sets []string, progress func(family string, done, total int64)) error {
	var setNames, families []string
	for _, s := range sets {
		switch strings.ToLower(s) {
		case "core", "extended", "cjk", "emoji", "all":
			setNames = append(setNames, strings.ToLower(s))
		default:
			families = append(families, s)
		}
	}
	return fonts.Install(ctx, fonts.DefaultDir(), setNames, families, progress)
}

// Language describes a supported document language.
type Language struct {
	Tag        string `json:"tag"`
	Name       string `json:"name"`
	NativeName string `json:"native_name"`
	RTL        bool   `json:"rtl,omitempty"`
}

// Languages lists the document languages crowdoc localises (captions,
// dates, quotation marks and hyphenation).
func Languages() []Language {
	var out []Language
	for _, t := range locale.Supported() {
		out = append(out, Language{Tag: t.Tag, Name: t.EnglishName, NativeName: t.NativeName, RTL: t.RTL})
	}
	return out
}

// CitationStyleInfo describes a citation style.
type CitationStyleInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

// CitationStyles lists the citation styles.
func CitationStyles() []CitationStyleInfo {
	var out []CitationStyleInfo
	for _, s := range cite.Styles() {
		out = append(out, CitationStyleInfo{Name: s.Name, Title: s.Title, Kind: s.Kind, Description: s.Description})
	}
	return out
}

// FormattedReferences is a formatted reference list.
type FormattedReferences struct {
	Style    string   `json:"style"`
	Text     []string `json:"text"`
	Markdown []string `json:"markdown"`
	Warnings []string `json:"warnings"`
}

// FormatReferences formats bibliography files (name → content) in a
// citation style. keys selects entries; empty formats all of them.
func FormatReferences(files map[string][]byte, keys []string, style, lang string) (*FormattedReferences, error) {
	var refs []ast.Reference
	out := &FormattedReferences{Style: style}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		r, w, err := cite.Parse(n, files[n])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		refs = append(refs, r...)
		out.Warnings = append(out.Warnings, w...)
	}
	doc := &ast.Document{}
	nocite := keys
	if len(nocite) == 0 {
		nocite = []string{"*"}
	}
	res, err := cite.Process(doc, refs, cite.Options{Style: style, Lang: lang, NoCite: nocite})
	if err != nil {
		return nil, err
	}
	out.Warnings = append(out.Warnings, res.Warnings...)
	for _, e := range res.Entries {
		prefix := ""
		if e.Label != "" {
			prefix = "[" + e.Label + "] "
		}
		out.Text = append(out.Text, prefix+ast.PlainText(e.Inlines))
		out.Markdown = append(out.Markdown, prefix+inlineMarkdown(e.Inlines))
	}
	return out, nil
}

func inlineMarkdown(ins []ast.Inline) string {
	var sb strings.Builder
	for _, in := range ins {
		switch n := in.(type) {
		case *ast.Text:
			sb.WriteString(n.Value)
		case *ast.Emph:
			sb.WriteString("*" + inlineMarkdown(n.Inlines) + "*")
		case *ast.Strong:
			sb.WriteString("**" + inlineMarkdown(n.Inlines) + "**")
		case *ast.Link:
			text := inlineMarkdown(n.Inlines)
			if text == n.URL || strings.HasPrefix(n.URL, "#") {
				sb.WriteString(text)
			} else {
				sb.WriteString("[" + text + "](" + n.URL + ")")
			}
		case *ast.SmallCaps:
			sb.WriteString(inlineMarkdown(n.Inlines))
		default:
			sb.WriteString(ast.PlainText([]ast.Inline{in}))
		}
	}
	return sb.String()
}
