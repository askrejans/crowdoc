package cite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/askrejans/crowdoc/v2/ast"
)

// ParseCSLJSON parses CSL-JSON: an array of items, or an object holding
// the array under "items" or "references".
func ParseCSLJSON(data []byte) ([]ast.Reference, []string, error) {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, utf8BOM)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, nil, fmt.Errorf("cite: CSL-JSON: %w", err)
	}
	switch v.(type) {
	case []any, map[string]any:
	default:
		return nil, nil, fmt.Errorf("cite: CSL-JSON: expected an array of items")
	}
	refs, warns := FromCSLValue(v)
	return refs, warns, nil
}

// ParseCSLYAML parses CSL-YAML: a list of items or a mapping with a
// "references" list (optionally wrapped in "---" document markers).
func ParseCSLYAML(data []byte) ([]ast.Reference, []string, error) {
	var v any
	if err := yaml.Unmarshal(bytes.TrimPrefix(data, utf8BOM), &v); err != nil {
		return nil, nil, fmt.Errorf("cite: CSL-YAML: %w", err)
	}
	if v == nil {
		return nil, nil, nil
	}
	refs, warns := FromCSLValue(v)
	return refs, warns, nil
}

// FromCSLValue converts an already decoded CSL-JSON/CSL-YAML value — a
// list of item maps, a single item, or a map with "references"/"items" —
// into references. Items without an id get a generated "<family><year>"
// key; duplicates keep the first item.
func FromCSLValue(v any) ([]ast.Reference, []string) {
	var warns []string
	items := cslItems(v)
	refs := make([]ast.Reference, 0, len(items))
	seen := map[string]bool{}
	gen := &keyGen{}
	for i, it := range items {
		m := asMap(it)
		if m == nil {
			warns = append(warns, fmt.Sprintf("csl: item %d is not an object", i+1))
			continue
		}
		r := cslRef(m)
		if r.ID == "" {
			r.ID = gen.next(&r)
			warns = append(warns, fmt.Sprintf("csl: item %d has no id; using %q", i+1, r.ID))
		}
		if seen[r.ID] {
			warns = append(warns, fmt.Sprintf("csl: duplicate id %q ignored", r.ID))
			continue
		}
		seen[r.ID] = true
		refs = append(refs, r)
	}
	return refs, warns
}

func cslItems(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case map[string]any, map[any]any:
		m := asMap(t)
		for _, k := range []string{"references", "items"} {
			if x, ok := m[k]; ok {
				return cslItems(x)
			}
		}
		for _, k := range []string{"id", "title", "type"} {
			if _, ok := m[k]; ok {
				return []any{m}
			}
		}
	}
	return nil
}

// asMap normalises the map types produced by JSON and YAML decoders.
func asMap(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		return t
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, x := range t {
			m[fmt.Sprint(k)] = x
		}
		return m
	}
	return nil
}

// cslString renders a scalar CSL value as text; arrays yield their first
// element (some exporters write "container-title" as a list).
func cslString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return stripCSLMarkup(strings.TrimSpace(t))
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case time.Time:
		return t.Format("2006-01-02")
	case []any:
		if len(t) > 0 {
			return cslString(t[0])
		}
	}
	return ""
}

// stripCSLMarkup removes the HTML-like rich-text tags allowed in CSL-JSON
// strings (<i>, <b>, <sup>, <sub>, <span …>).
func stripCSLMarkup(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '<' {
			if end := strings.IndexByte(s[i:], '>'); end > 0 {
				tag := strings.ToLower(strings.TrimPrefix(s[i+1:i+end], "/"))
				name, _, _ := strings.Cut(tag, " ")
				switch name {
				case "i", "b", "sup", "sub", "span", "em", "strong", "sc":
					i += end + 1
					continue
				}
			}
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}

func cslRef(m map[string]any) ast.Reference {
	lm := make(map[string]any, len(m))
	for k, v := range m {
		lm[strings.ToLower(strings.ReplaceAll(k, "_", "-"))] = v
	}
	get := func(keys ...string) string {
		for _, k := range keys {
			if s := cslString(lm[k]); s != "" {
				return s
			}
		}
		return ""
	}
	r := ast.Reference{
		ID:              get("id", "citation-key", "key"),
		Type:            normType(get("type")),
		Title:           get("title"),
		ShortTitle:      get("title-short", "shorttitle", "short-title"),
		ContainerTitle:  get("container-title", "journal", "journal-title", "book-title"),
		CollectionTitle: get("collection-title"),
		Publisher:       get("publisher"),
		PublisherPlace:  get("publisher-place"),
		Edition:         get("edition"),
		Volume:          get("volume"),
		Issue:           get("issue"),
		Page:            normalizeRange(get("page", "pages")),
		Number:          get("number"),
		Genre:           get("genre"),
		Event:           get("event-title", "event"),
		EventPlace:      get("event-place"),
		Medium:          get("medium"),
		Version:         get("version"),
		Language:        get("language"),
		URL:             get("url"),
		DOI:             cleanDOI(get("doi")),
		ISBN:            get("isbn"),
		ISSN:            get("issn"),
		Note:            get("note"),
		Abstract:        get("abstract"),
		Author:          cslNames(lm["author"]),
		Editor:          cslNames(lm["editor"]),
		Translator:      cslNames(lm["translator"]),
		Issued:          cslDate(lm["issued"]),
		Accessed:        cslDate(lm["accessed"]),
	}
	if r.CollectionTitle != "" {
		if n := get("collection-number"); n != "" {
			r.CollectionTitle += " " + n
		}
	}
	return r
}

func cslNames(v any) []ast.Name {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		var out []ast.Name
		for _, part := range strings.Split(t, ";") {
			if n := parseNameString(part); n != (ast.Name{}) {
				out = append(out, n)
			}
		}
		return out
	case []any:
		var out []ast.Name
		for _, x := range t {
			if s, ok := x.(string); ok {
				if n := parseNameString(s); n != (ast.Name{}) {
					out = append(out, n)
				}
				continue
			}
			if n, ok := cslName(asMap(x)); ok {
				out = append(out, n)
			}
		}
		return out
	}
	if n, ok := cslName(asMap(v)); ok {
		return []ast.Name{n}
	}
	return nil
}

func cslName(m map[string]any) (ast.Name, bool) {
	if m == nil {
		return ast.Name{}, false
	}
	lm := make(map[string]any, len(m))
	for k, v := range m {
		lm[strings.ToLower(k)] = v
	}
	n := ast.Name{
		Family:   cslString(lm["family"]),
		Given:    cslString(lm["given"]),
		Literal:  cslString(lm["literal"]),
		Suffix:   cslString(lm["suffix"]),
		Particle: joinNonEmpty(" ", cslString(lm["dropping-particle"]), cslString(lm["non-dropping-particle"])),
	}
	if n.Literal == "" && n.Family == "" && n.Given == "" {
		if s := cslString(lm["name"]); s != "" {
			n.Literal = s
		}
	}
	if n.Literal != "" {
		n = ast.Name{Literal: n.Literal}
	}
	return n, n != (ast.Name{})
}

func cslDate(v any) ast.Date {
	switch t := v.(type) {
	case nil:
		return ast.Date{}
	case string:
		return parseDateString(t)
	case json.Number, float64, int, int64, uint64:
		return parseDateString(cslString(t))
	case time.Time:
		return ast.Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}
	case []any:
		return datePartsDate(t)
	}
	m := asMap(v)
	if m == nil {
		return ast.Date{}
	}
	var d ast.Date
	if dp, ok := m["date-parts"].([]any); ok {
		d = datePartsDate(dp)
	}
	if d.Year == 0 {
		for _, k := range []string{"raw", "literal"} {
			if s := cslString(m[k]); s != "" {
				d = parseDateString(s)
				if d.Year == 0 && k == "literal" {
					d = ast.Date{Literal: s}
				}
				break
			}
		}
	}
	switch c := m["circa"].(type) {
	case bool:
		d.Circa = d.Circa || c
	case string, json.Number, int, float64:
		d.Circa = d.Circa || (cslString(c) != "" && cslString(c) != "0" && cslString(c) != "false")
	}
	return d
}

// datePartsDate reads [[year, month, day], …] (only the start of a range).
func datePartsDate(dp []any) ast.Date {
	if len(dp) == 0 {
		return ast.Date{}
	}
	parts, ok := dp[0].([]any)
	if !ok {
		parts = dp // a flat [year, month, day]
	}
	var nums [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(strings.TrimSpace(cslString(parts[i])))
		if err != nil {
			break
		}
		nums[i] = n
	}
	d := ast.Date{Year: nums[0], Month: nums[1], Day: nums[2]}
	if d.Month < 1 || d.Month > 12 {
		d.Month, d.Day = 0, 0
	}
	if d.Day < 0 || d.Day > 31 {
		d.Day = 0
	}
	return d
}
