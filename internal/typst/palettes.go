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
		{"ink", "Ink", "Blue-black fountain-pen ink and royal blue — personal, handwritten feel.", map[string]string{
			"accent": "#1E3A8A", "secondary": "#3B5BA9", "ink": "#141B2D", "heading": "#0F1E4A", "muted": "#5B6478",
			"link": "#1E3A8A", "rule": "#CBD2E1", "code-bg": "#F2F4F9", "table-head": "#E3E8F3", "stripe": "#F6F7FB", "quote": "#3B5BA9"}},
		{"ocean", "Ocean", "Deep sea blue with aqua highlights — calm, clear, trustworthy.", map[string]string{
			"accent": "#0B4F71", "secondary": "#22A39F", "ink": "#122028", "heading": "#083A54", "muted": "#5A6B75",
			"link": "#0B6A94", "rule": "#C8D9E1", "code-bg": "#F0F6F8", "table-head": "#DCEBF1", "stripe": "#F4F9FA", "quote": "#22A39F"}},
		{"forest", "Forest", "Pine green and moss — grounded, outdoorsy, sustainable.", map[string]string{
			"accent": "#2D5A27", "secondary": "#8A9A3B", "ink": "#1A2117", "heading": "#1F3F1B", "muted": "#626B5E",
			"link": "#2D6A27", "rule": "#CFD8C8", "code-bg": "#F3F6F0", "table-head": "#E2EADB", "stripe": "#F6F8F3", "quote": "#8A9A3B"}},
		{"rose", "Rose", "Dusty rose and mauve — soft, personal, elegant.", map[string]string{
			"accent": "#A44A63", "secondary": "#C98B9C", "ink": "#2A1E22", "heading": "#7A2F45", "muted": "#7A676D",
			"link": "#A44A63", "rule": "#E8D3D9", "code-bg": "#FAF3F5", "table-head": "#F4E2E7", "stripe": "#FCF7F8", "quote": "#C98B9C"}},
		{"plum", "Plum", "Ripe plum with soft lilac — rich, creative, refined.", map[string]string{
			"accent": "#5E2750", "secondary": "#A67FA0", "ink": "#22171F", "heading": "#451A3A", "muted": "#6E6070",
			"link": "#6E2D5E", "rule": "#DDD0DB", "code-bg": "#F7F2F6", "table-head": "#EDE1EB", "stripe": "#FAF6F9", "quote": "#A67FA0"}},
		{"slate", "Slate", "Blue-grey slate with a warm amber accent — modern and composed.", map[string]string{
			"accent": "#3E5468", "secondary": "#D08C2A", "ink": "#1C232B", "heading": "#26333F", "muted": "#66717C",
			"link": "#3E5E7E", "rule": "#CDD3D9", "code-bg": "#F2F4F6", "table-head": "#E3E7EB", "stripe": "#F6F7F9", "quote": "#D08C2A"}},
		{"amber", "Amber", "Honey amber and dark walnut — warm, golden, inviting.", map[string]string{
			"accent": "#A86A12", "secondary": "#5C3D1E", "ink": "#2A2016", "heading": "#5C3D1E", "muted": "#7A6A55",
			"link": "#946010", "rule": "#E6D6BC", "code-bg": "#FBF5EA", "table-head": "#F5E7CC", "stripe": "#FCF8F0", "quote": "#D9A441"}},
		{"olive", "Olive", "Olive green and mustard — earthy, Mediterranean, relaxed.", map[string]string{
			"accent": "#5B6327", "secondary": "#C9A227", "ink": "#22231A", "heading": "#41471C", "muted": "#6E6F5E",
			"link": "#5B6327", "rule": "#D9DAC6", "code-bg": "#F6F6EF", "table-head": "#E9EAD6", "stripe": "#F9F9F3", "quote": "#C9A227"}},
		{"cobalt", "Cobalt", "Vivid cobalt blue with orange — bold, energetic, contemporary.", map[string]string{
			"accent": "#0047AB", "secondary": "#F28C28", "ink": "#121826", "heading": "#002E70", "muted": "#5C6577",
			"link": "#0047AB", "rule": "#C9D5EA", "code-bg": "#F1F5FC", "table-head": "#DDE7F7", "stripe": "#F5F8FD", "quote": "#F28C28"}},
		{"crimson", "Crimson", "Strong crimson red with charcoal — decisive and urgent.", map[string]string{
			"accent": "#B0122D", "secondary": "#3A3A3A", "ink": "#1A1617", "heading": "#7E0C20", "muted": "#6A6264",
			"link": "#B0122D", "rule": "#E2CDD0", "code-bg": "#F8F3F4", "table-head": "#F2E0E3", "stripe": "#FBF7F8", "quote": "#B0122D"}},
		{"mint", "Mint", "Fresh mint and deep navy — light, clean, optimistic.", map[string]string{
			"accent": "#1F7A5C", "secondary": "#7FD1B9", "ink": "#16202A", "heading": "#1B2A4A", "muted": "#5E6B72",
			"link": "#1F7A5C", "rule": "#CFE5DD", "code-bg": "#F0F8F5", "table-head": "#DDF1EA", "stripe": "#F5FBF9", "quote": "#7FD1B9"}},
		{"lavender", "Lavender", "Lavender and soft grey — gentle, calm, quietly modern.", map[string]string{
			"accent": "#6B5B95", "secondary": "#A99BD0", "ink": "#211F2A", "heading": "#4A3F6E", "muted": "#6C6878",
			"link": "#6B5B95", "rule": "#DCD8E8", "code-bg": "#F6F5FA", "table-head": "#E9E6F3", "stripe": "#F9F8FC", "quote": "#A99BD0"}},
		{"sunset", "Sunset", "Coral orange and magenta — vibrant, festive, eye-catching.", map[string]string{
			"accent": "#E2572B", "secondary": "#B8327A", "ink": "#2A1A17", "heading": "#9C2E14", "muted": "#7A6460",
			"link": "#C2481F", "rule": "#F0D5CB", "code-bg": "#FDF5F1", "table-head": "#FAE3DA", "stripe": "#FEF8F5", "quote": "#B8327A"}},
		{"newsprint", "Newsprint", "Black ink on off-white newsprint — the look of a printed paper.", map[string]string{
			"accent": "#1A1A1A", "secondary": "#6E6E6E", "ink": "#1C1C1C", "heading": "#0D0D0D", "muted": "#5C5A55",
			"link": "#1A1A1A", "rule": "#B9B5AA", "code-bg": "#ECE8DE", "table-head": "#E4DFD2", "stripe": "#EFEBE2", "quote": "#8C877C",
			"page": "#F4F1EA"}},
		{"ivory", "Ivory", "Black and antique gold on an ivory page — formal and timeless.", map[string]string{
			"accent": "#8A6D2F", "secondary": "#2B2B2B", "ink": "#1F1C17", "heading": "#151310", "muted": "#6E675B",
			"link": "#7A5F26", "rule": "#DCD3BF", "code-bg": "#F6F0E1", "table-head": "#F0E7D2", "stripe": "#FAF6EC", "quote": "#B8995A",
			"page": "#FFFDF5"}},
		{"kraft", "Kraft", "Dark brown on kraft-paper brown — handmade, rustic, craft-fair.", map[string]string{
			"accent": "#5A3A1A", "secondary": "#2F4F4F", "ink": "#2B1F12", "heading": "#3D2810", "muted": "#5E4C38",
			"link": "#5A3A1A", "rule": "#B89B76", "code-bg": "#E2CFAF", "table-head": "#DCC6A2", "stripe": "#E6D5B8", "quote": "#7A5A34",
			"page": "#EADBC0"}},
		{"contrast", "High contrast", "Pure black on white with bold blue links — maximum legibility.", map[string]string{
			"accent": "#000000", "secondary": "#0033CC", "ink": "#000000", "heading": "#000000", "muted": "#333333",
			"link": "#0033CC", "rule": "#000000", "code-bg": "#F0F0F0", "table-head": "#DDDDDD", "stripe": "#F2F2F2", "quote": "#000000"}},
		{"dusk", "Dusk", "Warm cream type on deep plum-grey — a softer dark page for screens.", map[string]string{
			"accent": "#F2A65A", "secondary": "#C5A3D9", "ink": "#EDE3D6", "heading": "#FFF6EA", "muted": "#A89B91",
			"link": "#F2B872", "rule": "#4A4050", "code-bg": "#2C2533", "table-head": "#352D3D", "stripe": "#282130", "quote": "#C5A3D9",
			"page": "#1E1A24"}},
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
