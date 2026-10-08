package html

import (
	"hash/fnv"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"
)

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

// classes returns the lower-cased class tokens of n.
func classes(n *html.Node) []string {
	return strings.Fields(strings.ToLower(attr(n, "class")))
}

func hasClass(n *html.Node, c string) bool {
	for _, x := range classes(n) {
		if x == c {
			return true
		}
	}
	return false
}

// hasClassPrefix reports whether any class token starts with prefix.
func hasClassPrefix(n *html.Node, prefix string) bool {
	for _, x := range classes(n) {
		if strings.HasPrefix(x, prefix) {
			return true
		}
	}
	return false
}

// tokens returns the lower-cased whitespace-separated tokens of an attribute
// such as role or epub:type.
func tokens(n *html.Node, key string) []string {
	return strings.Fields(strings.ToLower(attr(n, key)))
}

func hasToken(n *html.Node, key, tok string) bool {
	for _, x := range tokens(n, key) {
		if x == tok {
			return true
		}
	}
	return false
}

// epubType returns the epub:type tokens (the namespace prefix may vary).
func epubType(n *html.Node) []string {
	var out []string
	for _, a := range n.Attr {
		if strings.HasSuffix(a.Key, ":type") || a.Key == "type" && a.Namespace == "epub" {
			out = append(out, strings.Fields(strings.ToLower(a.Val))...)
		}
	}
	return out
}

func hasEpubType(n *html.Node, t string) bool {
	for _, x := range epubType(n) {
		if x == t {
			return true
		}
	}
	return false
}

func isElem(n *html.Node, tags ...string) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	for _, t := range tags {
		if n.Data == t {
			return true
		}
	}
	return false
}

// styleProp returns the value of a CSS property in the inline style.
func styleProp(n *html.Node, prop string) string {
	style := attr(n, "style")
	if style == "" {
		return ""
	}
	for _, decl := range strings.Split(style, ";") {
		k, v, ok := strings.Cut(decl, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), prop) {
			v = strings.TrimSpace(strings.ToLower(v))
			return strings.TrimSpace(strings.TrimSuffix(v, "!important"))
		}
	}
	return ""
}

// textContent returns all descendant text of n (scripts and styles skipped).
func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		switch x.Type {
		case html.TextNode:
			sb.WriteString(x.Data)
			return
		case html.ElementNode:
			if x.Data == "script" || x.Data == "style" || x.Data == "template" {
				return
			}
			if x.Data == "br" {
				sb.WriteByte('\n')
			}
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// textLen approximates the amount of visible text below n.
func textLen(n *html.Node) int {
	return len(strings.Join(strings.Fields(textContent(n)), " "))
}

// collapseWS folds runs of HTML whitespace into one space; NBSP is kept.
func collapseWS(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	space := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\f':
			if !space {
				sb.WriteByte(' ')
				space = true
			}
			continue
		}
		if r < 0x20 || r == 0x7f || r == 0xFEFF {
			continue
		}
		space = false
		sb.WriteRune(r)
	}
	return sb.String()
}

func normSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// SanitizeID turns an HTML id or URL fragment into an identifier made of
// lower-case ASCII letters, digits, '-', '_', ':' and '.'. Accents are
// stripped; other non-ASCII letters are dropped and a short hash keeps such
// ids distinct. Links and targets must both pass through SanitizeID.
func SanitizeID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if u, err := url.PathUnescape(id); err == nil {
		id = u
	}
	var sb strings.Builder
	dash, lossy := false, false
	for _, r := range norm.NFD.String(strings.ToLower(id)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == ':', r == '.':
			sb.WriteRune(r)
			dash = false
		case unicode.Is(unicode.Mn, r):
		default:
			if r >= 0x80 {
				lossy = true
			}
			if !dash && sb.Len() > 0 {
				sb.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.TrimRight(sb.String(), "-")
	if out == "" || lossy {
		h := fnv.New32a()
		h.Write([]byte(id))
		sum := strconv.FormatUint(uint64(h.Sum32()), 36)
		if out == "" {
			return "id-" + sum
		}
		return out + "-" + sum
	}
	return out
}

// parseLength parses an HTML dimension attribute ("300", "300px", "50%")
// into a CSS-like length, or "".
func parseLength(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" || v == "auto" {
		return ""
	}
	num := v
	unit := "px"
	switch {
	case strings.HasSuffix(v, "%"):
		num, unit = strings.TrimSuffix(v, "%"), "%"
	case strings.HasSuffix(v, "px"):
		num = strings.TrimSuffix(v, "px")
	case strings.HasSuffix(v, "pt"), strings.HasSuffix(v, "cm"), strings.HasSuffix(v, "mm"), strings.HasSuffix(v, "in"), strings.HasSuffix(v, "em"):
		num, unit = v[:len(v)-2], v[len(v)-2:]
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	if err != nil || f <= 0 || f > 100000 {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64) + unit
}
