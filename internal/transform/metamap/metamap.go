// Package metamap maps loosely typed metadata (YAML frontmatter, CLI
// --meta overrides, MCP arguments) onto ast.Meta.
package metamap

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/askrejans/crowdoc/v2/ast"
)

// SplitFrontmatter separates a leading YAML frontmatter block from src.
// The block starts with a line "---" and ends with "---" or "...". It
// returns the decoded mapping (nil when absent) and the remaining body.
func SplitFrontmatter(src string) (map[string]any, string, error) {
	s := strings.TrimPrefix(src, "\ufeff")
	first, rest, ok := strings.Cut(s, "\n")
	if !ok || strings.TrimRight(first, " \t\r") != "---" {
		return nil, src, nil
	}
	lines := strings.SplitAfter(rest, "\n")
	for i, line := range lines {
		t := strings.TrimRight(line, " \t\r\n")
		if t != "---" && t != "..." {
			continue
		}
		yamlSrc := strings.Join(lines[:i], "")
		body := strings.Join(lines[i+1:], "")
		if strings.TrimSpace(yamlSrc) == "" {
			return map[string]any{}, body, nil
		}
		var m map[string]any
		dec := yaml.NewDecoder(bytes.NewReader([]byte(yamlSrc)))
		if err := dec.Decode(&m); err != nil {
			// Not YAML after all (e.g. a document starting with a rule):
			// leave the text untouched so nothing is lost.
			return nil, src, fmt.Errorf("frontmatter is not valid YAML: %w", err)
		}
		return m, body, nil
	}
	return nil, src, nil
}

// ParseBlocks converts Markdown text to blocks (used for rich fields such as
// the abstract). It may be nil, in which case such fields become paragraphs.
type ParseBlocks func(markdown string) []ast.Block

// Apply copies recognised keys from values into m. Keys are matched
// case-insensitively with '_' and '-' treated alike. Unknown keys are kept
// in m.Extra. Problems are returned as warnings.
func Apply(m *ast.Meta, values map[string]any, parse ParseBlocks) []string {
	var warns []string
	warn := func(format string, args ...any) { warns = append(warns, fmt.Sprintf(format, args...)) }

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, rawKey := range keys {
		v := values[rawKey]
		key := normKey(rawKey)
		switch key {
		case "title":
			m.Title = Text(v)
		case "subtitle":
			m.Subtitle = Text(v)
		case "short-title", "shorttitle", "running-title", "runningtitle":
			m.ShortTitle = Text(v)
		case "author", "authors":
			m.Authors = Authors(v)
		case "organization", "organisation", "company", "publisher":
			m.Organization = Text(v)
		case "date":
			m.Date = Text(v)
		case "version":
			m.Version = Text(v)
		case "status":
			m.Status = strings.ToUpper(Text(v))
		case "classification":
			m.Classification = strings.ToUpper(Text(v))
		case "type", "doctype", "document-type":
			m.DocType = strings.ToLower(Text(v))
		case "style", "template-style":
			m.Style = strings.ToLower(Text(v))
		case "lang", "language":
			m.Lang = Text(v)
		case "subject":
			m.Subject = Text(v)
		case "keywords", "tags":
			m.Keywords = List(v)
		case "summary", "description":
			m.Summary = Text(v)
		case "abstract":
			text := Text(v)
			if parse != nil {
				m.Abstract = parse(text)
			} else if p := paragraph(text); p != nil {
				m.Abstract = []ast.Block{p}
			}
		case "toc", "table-of-contents":
			if b, ok := Bool(v); ok {
				m.TOC = &b
			} else {
				warn("frontmatter %q: expected true or false", rawKey)
			}
		case "lof", "list-of-figures":
			m.LOF, _ = Bool(v)
		case "lot", "list-of-tables":
			m.LOT, _ = Bool(v)
		case "number-sections", "numbersections", "numbered", "section-numbering":
			if b, ok := Bool(v); ok {
				m.NumberSections = &b
			}
		case "no-title-page", "notitlepage":
			m.NoTitlePage, _ = Bool(v)
		case "title-page", "titlepage":
			if b, ok := Bool(v); ok {
				m.NoTitlePage = !b
			}
		case "signatures":
			if b, ok := Bool(v); ok {
				m.Signatures = &b
			}
		case "font-size", "fontsize":
			if n, ok := FontSize(v); ok {
				m.FontSize = n
			} else {
				warn("frontmatter %q: font size must be between 8 and 14 pt", rawKey)
			}
		case "paper", "papersize", "paper-size":
			m.Paper = strings.ToLower(strings.TrimSpace(Text(v)))
		case "landscape":
			m.Landscape, _ = Bool(v)
		case "orientation":
			m.Landscape = strings.EqualFold(Text(v), "landscape")
		case "columns":
			if n, err := strconv.Atoi(Text(v)); err == nil && (n == 1 || n == 2) {
				m.Columns = n
			} else {
				warn("frontmatter %q: columns must be 1 or 2", rawKey)
			}
		case "linestretch", "line-spacing", "linespacing", "spacing":
			m.LineSpacing = Text(v)
		case "margin", "margins":
			applyMargins(m, v)
		case "geometry":
			for _, item := range List(v) {
				k, val, ok := strings.Cut(item, "=")
				if ok && strings.TrimSpace(k) == "margin" {
					applyMargins(m, strings.TrimSpace(val))
				} else if ok {
					setMargin(m, strings.TrimSpace(k), strings.TrimSpace(val))
				}
			}
		case "margin-top", "margin-bottom", "margin-left", "margin-right":
			setMargin(m, strings.TrimPrefix(key, "margin-"), Text(v))
		case "header-left":
			m.HeaderLeft = Text(v)
		case "header-right":
			m.HeaderRight = Text(v)
		case "footer-left":
			m.FooterLeft = Text(v)
		case "footer-right":
			m.FooterRight = Text(v)
		case "logo":
			m.Logo = Text(v)
		case "links-as-notes", "linksasnotes":
			if b, ok := Bool(v); ok {
				m.LinksAsNotes = &b
			}
		case "pdfa", "pdf-a", "pdf-standard", "archival":
			if b, ok := Bool(v); ok {
				m.PDFA = b
			} else {
				m.PDFA = strings.HasPrefix(strings.ToLower(Text(v)), "a")
			}
		case "font", "mainfont", "main-font", "serif-font":
			m.Fonts.Main = Text(v)
		case "sansfont", "sans-font", "heading-font":
			m.Fonts.Sans = Text(v)
		case "monofont", "mono-font", "code-font":
			m.Fonts.Mono = Text(v)
		case "mathfont", "math-font":
			m.Fonts.Math = Text(v)
		case "fonts", "typeface", "typefaces", "font-pairing", "pairing":
			if name, ok := v.(string); ok {
				m.Fonts.Pairing = strings.ToLower(strings.TrimSpace(name))
			}
			if mm, ok := v.(map[string]any); ok {
				for fk, fv := range mm {
					switch normKey(fk) {
					case "main", "body", "serif":
						m.Fonts.Main = Text(fv)
					case "sans", "heading", "headings":
						m.Fonts.Sans = Text(fv)
					case "mono", "code":
						m.Fonts.Mono = Text(fv)
					case "math":
						m.Fonts.Math = Text(fv)
					}
				}
			}
		case "accent", "accent-color", "accentcolor", "color", "theme-color":
			m.Accent = Text(v)
		case "colors", "colours", "color-scheme", "colour-scheme", "palette", "colorscheme":
			switch t := v.(type) {
			case map[string]any:
				if m.Colors == nil {
					m.Colors = map[string]string{}
				}
				for ck, cv := range t {
					key := normKey(ck)
					if key == "scheme" || key == "palette" || key == "name" {
						m.ColorScheme = strings.ToLower(Text(cv))
						continue
					}
					m.Colors[key] = Text(cv)
				}
			default:
				m.ColorScheme = strings.ToLower(Text(v))
			}
		case "bibliography", "bib", "bibliographies":
			m.Bibliography = append(m.Bibliography, List(v)...)
		case "csl", "citation-style", "citationstyle", "cite-style", "reference-style":
			m.CitationStyle = strings.ToLower(strings.TrimSuffix(Text(v), ".csl"))
		case "reference-section-title", "references-title", "bibliography-title":
			m.ReferenceSectionTitle = Text(v)
		case "nocite":
			for _, k := range List(v) {
				for _, part := range strings.FieldsFunc(k, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
					part = strings.TrimPrefix(strings.TrimSpace(part), "@")
					if part != "" {
						m.NoCite = append(m.NoCite, part)
					}
				}
			}
		case "recipient", "to-address", "recipient-address":
			m.Recipient = Lines(v)
		case "sender", "from-address", "sender-address", "return-address":
			m.Sender = Lines(v)
		case "to":
			m.To = Text(v)
		case "from":
			m.From = Text(v)
		case "cc":
			m.CC = Text(v)
		case "opening", "salutation", "greeting":
			m.Opening = Text(v)
		case "closing", "valediction":
			m.Closing = Text(v)
		case "place", "city":
			m.Place = Text(v)
		case "institution", "university", "school":
			m.Institution = Text(v)
		case "faculty":
			m.Faculty = Text(v)
		case "department":
			m.Department = Text(v)
		case "degree", "degree-name":
			m.Degree = Text(v)
		case "supervisor", "advisor", "adviser":
			m.Supervisor = Text(v)
		case "location":
			m.Location = Text(v)
		case "jurisdiction":
			m.Jurisdiction = Text(v)
		case "seller-country-code":
			m.SellerCountryCode = strings.ToUpper(Text(v))
		case "buyer-country-code":
			m.BuyerCountryCode = strings.ToUpper(Text(v))
		case "vat-breakdown":
			m.VATBreakdown = Text(v)
		case "legal-notice":
			m.LegalNotice = Text(v)
		case "parties":
			m.Parties = List(v)
		default:
			if m.Extra == nil {
				m.Extra = map[string]any{}
			}
			m.Extra[rawKey] = v
		}
	}
	return warns
}

func normKey(k string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(k)), "_", "-")
}

func applyMargins(m *ast.Meta, v any) {
	if mm, ok := v.(map[string]any); ok {
		for k, val := range mm {
			setMargin(m, normKey(k), Text(val))
		}
		return
	}
	all := Text(v)
	m.Margins = ast.Margins{Top: all, Bottom: all, Left: all, Right: all}
}

func setMargin(m *ast.Meta, side, val string) {
	switch side {
	case "top":
		m.Margins.Top = val
	case "bottom":
		m.Margins.Bottom = val
	case "left", "inner":
		m.Margins.Left = val
	case "right", "outer":
		m.Margins.Right = val
	case "x":
		m.Margins.Left, m.Margins.Right = val, val
	case "y":
		m.Margins.Top, m.Margins.Bottom = val, val
	}
}

// Text converts a scalar (or a list, joined with ", ") to a trimmed string.
func Text(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case time.Time:
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
			return t.Format("2006-01-02")
		}
		return t.Format("2006-01-02 15:04")
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(t))
		for _, x := range t {
			if s := Text(x); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		for _, k := range []string{"name", "text", "value", "literal"} {
			if s, ok := t[k]; ok {
				return Text(s)
			}
		}
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// List converts a list or a comma/semicolon separated string to strings.
func List(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		var out []string
		for _, x := range t {
			if s := Text(x); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	default:
		s := Text(v)
		sep := ","
		if strings.Contains(s, ";") {
			sep = ";"
		}
		var out []string
		for _, part := range strings.Split(s, sep) {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out
	}
}

// Lines converts a list or a multi-line string to lines.
func Lines(v any) []string {
	if l, ok := v.([]any); ok {
		return List(l)
	}
	var out []string
	for _, line := range strings.Split(Text(v), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// Bool parses YAML-ish booleans.
func Bool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case int:
		return t != 0, true
	}
	switch strings.ToLower(Text(v)) {
	case "true", "yes", "on", "y", "1":
		return true, true
	case "false", "no", "off", "n", "0":
		return false, true
	}
	return false, false
}

// FontSize parses "11", "11pt", 11 into a size in points (8–14).
func FontSize(v any) (int, bool) {
	s := strings.TrimSuffix(strings.ToLower(Text(v)), "pt")
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f < 8 || f > 14 {
		return 0, false
	}
	return int(f + 0.5), true
}

// Authors parses an author value: "A, B", ["A", "B"] or a list of maps
// with name/affiliation(s)/email/orcid/corresponding.
func Authors(v any) []ast.Author {
	var out []ast.Author
	add := func(x any) {
		switch t := x.(type) {
		case map[string]any:
			a := ast.Author{}
			for k, val := range t {
				switch normKey(k) {
				case "name", "full-name", "fullname":
					a.Name = Text(val)
				case "affiliation", "affiliations", "institute", "institution", "organization":
					// A single string is one affiliation (they contain commas);
					// several need a YAML list.
					if l, ok := val.([]any); ok {
						a.Affiliations = append(a.Affiliations, List(l)...)
					} else if s := Text(val); s != "" {
						a.Affiliations = append(a.Affiliations, s)
					}
				case "email", "e-mail", "mail":
					a.Email = Text(val)
				case "orcid":
					a.ORCID = Text(val)
				case "corresponding":
					a.Corresponding, _ = Bool(val)
				}
			}
			if a.Name == "" {
				given, family := Text(t["given"]), Text(t["family"])
				a.Name = strings.TrimSpace(given + " " + family)
			}
			if a.Name != "" {
				out = append(out, a)
			}
		default:
			s := Text(t)
			if s == "" {
				return
			}
			// "A and B" / "A; B" lists in a single string. Commas are
			// ambiguous ("Smith, John") so they only split when every
			// part looks like a full name.
			var parts []string
			switch {
			case strings.Contains(s, ";"):
				parts = strings.Split(s, ";")
			case strings.Contains(s, " and "):
				parts = strings.Split(s, " and ")
			default:
				parts = []string{s}
			}
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, ast.Author{Name: p})
				}
			}
		}
	}
	if list, ok := v.([]any); ok {
		for _, x := range list {
			add(x)
		}
	} else {
		add(v)
	}
	return out
}

func paragraph(text string) ast.Block {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	return &ast.Para{Inlines: ast.Str(text)}
}

// ParseOverride parses a "key=value" command-line override. Values are
// decoded as YAML scalars or flow collections ("[a, b]", "true", "11").
func ParseOverride(s string) (string, any, error) {
	k, v, ok := strings.Cut(s, "=")
	if !ok || strings.TrimSpace(k) == "" {
		return "", nil, fmt.Errorf("metadata override %q must be key=value", s)
	}
	var val any
	if err := yaml.Unmarshal([]byte(v), &val); err != nil || val == nil {
		val = v
	}
	return strings.TrimSpace(k), val, nil
}
