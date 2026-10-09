package markdown

import (
	"bytes"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/askrejans/crowdoc/v2/ast"
)

// frontmatter renders the metadata as YAML frontmatter with the keys the
// reader understands; empty fields are left out.
func (w *writer) frontmatter() string {
	m := &w.doc.Meta
	root := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key string, v any) {
		var n yaml.Node
		if err := n.Encode(v); err != nil {
			w.warn("metadata %q dropped: %v", key, err)
			return
		}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &n)
	}
	str := func(key, v string) {
		if v = strings.TrimSpace(v); v != "" {
			add(key, v)
		}
	}
	list := func(key string, v []string) {
		var out []string
		for _, s := range v {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			add(key, out)
		}
	}
	boolPtr := func(key string, v *bool) {
		if v != nil {
			add(key, *v)
		}
	}
	flag := func(key string, v bool) {
		if v {
			add(key, true)
		}
	}

	if !m.TitleFromName {
		str("title", m.Title)
	}
	str("subtitle", m.Subtitle)
	str("short-title", m.ShortTitle)
	if len(m.Authors) > 0 {
		add("author", authorsValue(m.Authors))
	}
	str("organization", m.Organization)
	str("date", m.Date)
	str("version", m.Version)
	str("status", m.Status)
	str("classification", m.Classification)
	str("type", m.DocType)
	str("style", m.Style)
	str("lang", m.Lang)
	str("subject", m.Subject)
	list("keywords", m.Keywords)
	str("summary", m.Summary)
	if len(m.Abstract) > 0 {
		if text := w.blocksWithNotes(m.Abstract); text != "" {
			var n yaml.Node
			if err := n.Encode(text); err == nil {
				n.Style = yaml.LiteralStyle
				if _, err := yaml.Marshal(&n); err != nil {
					n.Style = 0
				}
				root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "abstract"}, &n)
			}
		}
	}

	// Letters, memos, theses, jurisdictions, agreements.
	list("recipient", m.Recipient)
	list("sender", m.Sender)
	str("to", m.To)
	str("from", m.From)
	str("cc", m.CC)
	str("opening", m.Opening)
	str("closing", m.Closing)
	str("place", m.Place)
	str("institution", m.Institution)
	str("faculty", m.Faculty)
	str("department", m.Department)
	str("degree", m.Degree)
	str("supervisor", m.Supervisor)
	str("location", m.Location)
	str("jurisdiction", m.Jurisdiction)
	str("seller-country-code", m.SellerCountryCode)
	str("buyer-country-code", m.BuyerCountryCode)
	str("vat-breakdown", m.VATBreakdown)
	str("legal-notice", m.LegalNotice)
	list("parties", m.Parties)

	// Layout.
	boolPtr("toc", m.TOC)
	flag("lof", m.LOF)
	flag("lot", m.LOT)
	boolPtr("number-sections", m.NumberSections)
	if m.NoTitlePage {
		add("title-page", false)
	}
	boolPtr("show-title", m.ShowTitle)
	boolPtr("signatures", m.Signatures)
	if m.FontSize > 0 {
		add("font-size", m.FontSize)
	}
	str("paper", m.Paper)
	flag("landscape", m.Landscape)
	if m.Columns > 0 {
		add("columns", m.Columns)
	}
	str("line-spacing", m.LineSpacing)
	mg := m.Margins
	if mg.Top != "" && mg.Top == mg.Bottom && mg.Top == mg.Left && mg.Top == mg.Right {
		str("margin", mg.Top)
	} else {
		str("margin-top", mg.Top)
		str("margin-bottom", mg.Bottom)
		str("margin-left", mg.Left)
		str("margin-right", mg.Right)
	}
	str("header-left", m.HeaderLeft)
	str("header-right", m.HeaderRight)
	str("footer-left", m.FooterLeft)
	str("footer-right", m.FooterRight)
	if m.Logo != "" {
		str("logo", w.media.resolve(w, m.Logo))
	}
	boolPtr("links-as-notes", m.LinksAsNotes)
	flag("pdfa", m.PDFA)
	str("fonts", m.Fonts.Pairing)
	str("main-font", m.Fonts.Main)
	str("sans-font", m.Fonts.Sans)
	str("mono-font", m.Fonts.Mono)
	str("math-font", m.Fonts.Math)
	str("accent", m.Accent)
	if len(m.Colors) > 0 {
		colors := map[string]string{}
		for k, v := range m.Colors {
			colors[k] = v
		}
		if m.ColorScheme != "" {
			colors["scheme"] = m.ColorScheme
		}
		add("colors", colors)
	} else {
		str("color-scheme", m.ColorScheme)
	}

	// Citations.
	list("bibliography", m.Bibliography)
	str("csl", m.CitationStyle)
	str("reference-section-title", m.ReferenceSectionTitle)
	list("nocite", m.NoCite)
	if _, ok := m.Extra["references"]; !ok && len(w.doc.References) > 0 {
		add("references", cslReferences(w.doc.References))
	}

	// Everything else, in key order.
	keys := make([]string, 0, len(m.Extra))
	for k := range m.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if m.Extra[k] != nil {
			add(k, m.Extra[k])
		}
	}

	if len(root.Content) == 0 {
		return ""
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		w.warn("frontmatter dropped: %v", err)
		return ""
	}
	enc.Close()
	return "---\n" + buf.String() + "---\n"
}

func authorsValue(authors []ast.Author) []any {
	var out []any
	for _, a := range authors {
		if a.Name == "" {
			continue
		}
		simple := len(a.Affiliations) == 0 && a.Email == "" && a.ORCID == "" && !a.Corresponding &&
			!strings.Contains(a.Name, ";") && !strings.Contains(a.Name, " and ")
		if simple {
			out = append(out, a.Name)
			continue
		}
		var m yaml.Node
		m.Kind = yaml.MappingNode
		kv := func(k string, v any) {
			var n yaml.Node
			if err := n.Encode(v); err == nil {
				m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, &n)
			}
		}
		kv("name", a.Name)
		switch len(a.Affiliations) {
		case 0:
		case 1:
			kv("affiliation", a.Affiliations[0])
		default:
			kv("affiliation", a.Affiliations)
		}
		if a.Email != "" {
			kv("email", a.Email)
		}
		if a.ORCID != "" {
			kv("orcid", a.ORCID)
		}
		if a.Corresponding {
			kv("corresponding", true)
		}
		out = append(out, &m)
	}
	return out
}

// cslReferences converts references to CSL-YAML items.
func cslReferences(refs []ast.Reference) []any {
	var out []any
	for _, r := range refs {
		n := &yaml.Node{Kind: yaml.MappingNode}
		kv := func(k string, v any) {
			var x yaml.Node
			if err := x.Encode(v); err == nil {
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, &x)
			}
		}
		s := func(k, v string) {
			if v != "" {
				kv(k, v)
			}
		}
		s("id", r.ID)
		s("type", r.Type)
		s("title", r.Title)
		s("title-short", r.ShortTitle)
		names := func(k string, ns []ast.Name) {
			if len(ns) == 0 {
				return
			}
			var list []map[string]string
			for _, nm := range ns {
				m := map[string]string{}
				if nm.Literal != "" {
					m["literal"] = nm.Literal
				} else {
					if nm.Family != "" {
						m["family"] = nm.Family
					}
					if nm.Given != "" {
						m["given"] = nm.Given
					}
					if nm.Particle != "" {
						m["non-dropping-particle"] = nm.Particle
					}
					if nm.Suffix != "" {
						m["suffix"] = nm.Suffix
					}
				}
				list = append(list, m)
			}
			kv(k, list)
		}
		names("author", r.Author)
		names("editor", r.Editor)
		names("translator", r.Translator)
		date := func(k string, d ast.Date) {
			if d.IsZero() {
				return
			}
			m := map[string]any{}
			if d.Year != 0 {
				parts := []int{d.Year}
				if d.Month != 0 {
					parts = append(parts, d.Month)
					if d.Day != 0 {
						parts = append(parts, d.Day)
					}
				}
				m["date-parts"] = [][]int{parts}
			} else {
				m["literal"] = d.Literal
			}
			if d.Circa {
				m["circa"] = true
			}
			kv(k, m)
		}
		date("issued", r.Issued)
		date("accessed", r.Accessed)
		s("container-title", r.ContainerTitle)
		s("collection-title", r.CollectionTitle)
		s("publisher", r.Publisher)
		s("publisher-place", r.PublisherPlace)
		s("edition", r.Edition)
		s("volume", r.Volume)
		s("issue", r.Issue)
		s("page", r.Page)
		s("number", r.Number)
		s("genre", r.Genre)
		s("event-title", r.Event)
		s("event-place", r.EventPlace)
		s("medium", r.Medium)
		s("version", r.Version)
		s("language", r.Language)
		s("URL", r.URL)
		s("DOI", r.DOI)
		s("ISBN", r.ISBN)
		s("ISSN", r.ISSN)
		s("note", r.Note)
		s("abstract", r.Abstract)
		out = append(out, n)
	}
	return out
}
