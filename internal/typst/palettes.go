package typst

import (
	"sort"
	"strings"
)

// Palette is a named colour scheme. Keys:
//
//	accent     signature colour: rules, numbers, highlights
//	secondary  supporting colour: secondary rules, tips, ornaments
//	ink        body text
//	heading    headings and titles
//	muted      headers, footers, captions, labels
//	link       hyperlinks
//	rule       hairlines and table rules
//	code-bg    code block background
//	table-head table header fill
//	stripe     zebra stripe fill
//	quote      block-quote bar
//	page       page background (only for screen-oriented schemes)
type Palette struct {
	Name        string
	Title       string
	Description string
	Colors      map[string]string
}

// ColorKeys lists the keys a colour scheme may set.
var ColorKeys = []string{"accent", "secondary", "ink", "heading", "muted", "link", "rule", "code-bg", "table-head", "stripe", "quote", "page"}

var palettes = map[string]*Palette{}

func init() {
	for _, p := range []*Palette{
		{"oxford", "Oxford", "Deep navy with antique gold — scholarly and assured.", map[string]string{
			"accent": "#1F3A5F", "secondary": "#B08D57", "ink": "#1B1D22", "heading": "#14213D", "muted": "#5E6470",
			"link": "#1F3A5F", "rule": "#C9CDD4", "code-bg": "#F4F5F7", "table-head": "#E8ECF2", "stripe": "#F6F7F9", "quote": "#B08D57"}},
		{"bordeaux", "Bordeaux", "Wine red and rose gold — warm, literary, celebratory.", map[string]string{
			"accent": "#6D1A36", "secondary": "#C08A6E", "ink": "#221A1C", "heading": "#4A1024", "muted": "#6F6266",
			"link": "#8A2446", "rule": "#D8C9CC", "code-bg": "#F8F3F4", "table-head": "#F1E4E8", "stripe": "#FAF6F7", "quote": "#C08A6E"}},
		{"emerald", "Emerald", "Forest emerald with brushed brass — confident and natural.", map[string]string{
			"accent": "#0B5D4B", "secondary": "#B8935A", "ink": "#17201D", "heading": "#0A3F33", "muted": "#5D6964",
			"link": "#0B6E58", "rule": "#C6D3CE", "code-bg": "#F2F6F4", "table-head": "#E2EEE9", "stripe": "#F5F9F7", "quote": "#B8935A"}},
		{"graphite", "Graphite", "Charcoal and steel — understated, architectural.", map[string]string{
			"accent": "#2E3440", "secondary": "#6B7B8C", "ink": "#1E2126", "heading": "#16191E", "muted": "#6B7280",
			"link": "#3B4A5C", "rule": "#CDD1D6", "code-bg": "#F3F4F6", "table-head": "#E7E9EC", "stripe": "#F6F7F8", "quote": "#9AA3AD"}},
		{"champagne", "Champagne", "Gilded bronze on warm ivory — luxurious and calm.", map[string]string{
			"accent": "#8C6A3F", "secondary": "#C6A46B", "ink": "#2A241C", "heading": "#3A2E20", "muted": "#7A6F60",
			"link": "#7A5A30", "rule": "#E2D6C2", "code-bg": "#F7F2E9", "table-head": "#F1E8D8", "stripe": "#FAF6EF", "quote": "#C6A46B"}},
		{"sage", "Sage", "Muted sage and clay — gentle, organic, contemporary.", map[string]string{
			"accent": "#4F6F52", "secondary": "#B5835A", "ink": "#1F2420", "heading": "#33483A", "muted": "#6B746C",
			"link": "#4F6F52", "rule": "#D3DACF", "code-bg": "#F4F6F2", "table-head": "#E5ECE2", "stripe": "#F7F9F6", "quote": "#B5835A"}},
		{"terracotta", "Terracotta", "Fired clay and sand — sunny, Mediterranean, inviting.", map[string]string{
			"accent": "#B5523B", "secondary": "#D9B38C", "ink": "#2A1F1A", "heading": "#7A3122", "muted": "#7D6A60",
			"link": "#A3472F", "rule": "#E5D3C6", "code-bg": "#FAF4F0", "table-head": "#F4E3DA", "stripe": "#FBF7F4", "quote": "#D9B38C"}},
		{"midnight", "Midnight", "Deep indigo with electric violet — modern and striking.", map[string]string{
			"accent": "#4C1D95", "secondary": "#7C3AED", "ink": "#16142B", "heading": "#1E1B4B", "muted": "#64618A",
			"link": "#5B21B6", "rule": "#D4D1E8", "code-bg": "#F4F3FA", "table-head": "#E7E4F7", "stripe": "#F7F6FC", "quote": "#7C3AED"}},
		{"nordic", "Nordic", "Fjord blue and frost — clean, cool, Scandinavian.", map[string]string{
			"accent": "#2F5D7C", "secondary": "#8FB3C9", "ink": "#1C232A", "heading": "#20435A", "muted": "#66737F",
			"link": "#2F5D7C", "rule": "#D1DCE4", "code-bg": "#F2F6F9", "table-head": "#E1EBF2", "stripe": "#F5F8FB", "quote": "#8FB3C9"}},
		{"royal", "Royal", "Royal purple and old gold — ceremonial and rich.", map[string]string{
			"accent": "#5B2A86", "secondary": "#C9A227", "ink": "#1E1826", "heading": "#3D1A5B", "muted": "#6D6478",
			"link": "#5B2A86", "rule": "#DCD2E6", "code-bg": "#F6F3F9", "table-head": "#ECE3F4", "stripe": "#F9F7FB", "quote": "#C9A227"}},
		{"swiss", "Swiss", "Black type with a single vermilion accent — International Typographic Style.", map[string]string{
			"accent": "#D94F30", "secondary": "#111111", "ink": "#111111", "heading": "#111111", "muted": "#666666",
			"link": "#C2412A", "rule": "#BDBDBD", "code-bg": "#F4F4F4", "table-head": "#EDEDED", "stripe": "#F7F7F7", "quote": "#D94F30"}},
		{"lagoon", "Lagoon", "Teal and coral — fresh, optimistic, friendly.", map[string]string{
			"accent": "#0F766E", "secondary": "#F97362", "ink": "#14201F", "heading": "#0B4F4A", "muted": "#5F6E6C",
			"link": "#0F766E", "rule": "#CFE0DD", "code-bg": "#F1F7F6", "table-head": "#DDEFEC", "stripe": "#F4FAF9", "quote": "#F97362"}},
		{"parchment", "Parchment", "Ink, bronze and teal on warm parchment — timeless and crafted.", map[string]string{
			"accent": "#273142", "secondary": "#B98A46", "ink": "#101522", "heading": "#101522", "muted": "#5E6574",
			"link": "#3F7C7A", "rule": "#D8DFE6", "code-bg": "#F6F1E6", "table-head": "#EFE6D3", "stripe": "#FAF6EE", "quote": "#B98A46"}},
		{"sepia", "Sepia", "Old-photograph browns on cream — vintage, nostalgic.", map[string]string{
			"accent": "#704214", "secondary": "#A67C52", "ink": "#2B2118", "heading": "#4A2C10", "muted": "#7A6A58",
			"link": "#704214", "rule": "#DDCDB8", "code-bg": "#F3EBDD", "table-head": "#EFE3CF", "stripe": "#F8F2E8", "quote": "#A67C52",
			"page": "#FBF6EC"}},
		{"mono", "Monochrome", "Pure black and greys — economical to print, maximally sober.", map[string]string{
			"accent": "#000000", "secondary": "#555555", "ink": "#000000", "heading": "#000000", "muted": "#555555",
			"link": "#000000", "rule": "#999999", "code-bg": "#F2F2F2", "table-head": "#E6E6E6", "stripe": "#F5F5F5", "quote": "#999999"}},
		{"night", "Night", "Light type on a dark page — for reading on screens at night.", map[string]string{
			"accent": "#8AB4F8", "secondary": "#F6C177", "ink": "#E4E6EB", "heading": "#FFFFFF", "muted": "#9AA0A6",
			"link": "#8AB4F8", "rule": "#3C4043", "code-bg": "#1F2228", "table-head": "#262A31", "stripe": "#1A1D22", "quote": "#F6C177",
			"page": "#121418"}},
	} {
		palettes[p.Name] = p
	}
}

// LookupPalette finds a palette by name (case-insensitive).
func LookupPalette(name string) (*Palette, bool) {
	p, ok := palettes[strings.ToLower(strings.TrimSpace(name))]
	return p, ok
}

// Palettes returns all palettes sorted by name.
func Palettes() []*Palette {
	out := make([]*Palette, 0, len(palettes))
	for _, p := range palettes {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
