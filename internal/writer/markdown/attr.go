package markdown

import (
	"crypto/sha256"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/askrejans/crowdoc/v2/ast"
)

// attrSyntax selects the attribute parser the output is read by.
type attrSyntax int

const (
	// attrGoldmark is goldmark's heading attribute parser.
	attrGoldmark attrSyntax = iota
	// attrPandoc is the reader's own parser (fenced divs, code blocks).
	attrPandoc
	// attrInline is the reader's parser applied to text after an image or
	// display math: the text is unescaped first.
	attrInline
)

// attrString renders {#id .class key="value"}; "" when empty. Parts the
// reader cannot read back are dropped with a warning.
func (w *writer) attrString(a ast.Attr, syn attrSyntax) string {
	if syn == attrInline {
		return w.inlineAttrString(a)
	}
	var parts []string
	if a.ID != "" {
		if safeID(a.ID) != "" || syn == attrPandoc && pandocClass(a.ID) {
			parts = append(parts, "#"+a.ID)
		} else {
			w.warn("identifier %q dropped", a.ID)
		}
	}
	for _, cl := range a.Classes {
		if safeClass(cl) || syn == attrPandoc && pandocClass(cl) {
			parts = append(parts, "."+cl)
		} else if cl != "" {
			w.warn("class %q dropped", cl)
		}
	}
	for _, k := range sortedKeys(a.KV) {
		v := a.KV[k]
		okName := validAttrName(k)
		if syn == attrPandoc {
			okName = pandocClass(k) && k == strings.ToLower(k)
		}
		if !okName {
			w.warn("attribute %q dropped", k)
			continue
		}
		q, ok := quoteAttr(v, syn)
		if !ok {
			w.warn("attribute %s=%q dropped", k, v)
			continue
		}
		parts = append(parts, k+"="+q)
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// inlineAttrString renders attributes read from a text run (after an
// image or display math): every part is escaped as text, values are
// always quoted and may hold any character but a line break.
func (w *writer) inlineAttrString(a ast.Attr) string {
	esc := func(s string) string {
		return w.escapeText(s, &seqState{prev: ' '}, ictx{braces: true, quotes: true}, ' ', textFlags{})
	}
	var parts []string
	if a.ID != "" {
		if pandocClass(a.ID) {
			parts = append(parts, "#"+esc(a.ID))
		} else {
			w.warn("identifier %q dropped", a.ID)
		}
	}
	for _, cl := range a.Classes {
		if pandocClass(cl) {
			parts = append(parts, "."+esc(cl))
		} else if cl != "" {
			w.warn("class %q dropped", cl)
		}
	}
	for _, k := range sortedKeys(a.KV) {
		v := a.KV[k]
		if !pandocClass(k) || k != strings.ToLower(k) || strings.ContainsAny(v, "\n\r") {
			w.warn("attribute %q dropped", k)
			continue
		}
		if v != "" && !strings.ContainsAny(v, " \t\"'={}") {
			parts = append(parts, esc(k)+"="+esc(v))
		} else {
			parts = append(parts, esc(k)+`="`+esc(v)+`"`)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// quoteAttr quotes an attribute value for the given parser.
func quoteAttr(v string, syn attrSyntax) (string, bool) {
	if strings.ContainsAny(v, "{}\n\r") {
		return "", false
	}
	if syn == attrGoldmark {
		// goldmark reads JSON-like strings; values are always quoted so
		// they stay strings.
		if strings.ContainsAny(v, `"\`) {
			return "", false
		}
		return `"` + v + `"`, true
	}
	// The reader trims quotes of both kinds from the ends of a value.
	if strings.HasPrefix(v, `"`) || strings.HasPrefix(v, "'") || strings.HasSuffix(v, `"`) || strings.HasSuffix(v, "'") {
		return "", false
	}
	if v != "" && !strings.ContainsAny(v, " \t\"'=") {
		return v, true
	}
	if !strings.Contains(v, `"`) {
		return `"` + v + `"`, true
	}
	if !strings.Contains(v, "'") {
		return "'" + v + "'", true
	}
	return "", false
}

// safeID reports id when both attribute parsers read it back unchanged.
func safeID(id string) string {
	for _, r := range id {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ':' || r == '.' || r >= utf8.RuneSelf && !unicode.IsSpace(r) && !unicode.IsPunct(r) && !unicode.IsSymbol(r)) {
			return ""
		}
	}
	return id
}

// pandocClass reports whether the reader's own attribute parser reads
// class c back.
func pandocClass(c string) bool {
	return c != "" && c != "-" && !strings.ContainsAny(c, " \t\n\r{}=") && !strings.HasPrefix(c, "#") && !strings.HasPrefix(c, ".")
}

func safeClass(c string) bool {
	return c != "" && c != "-" && safeID(c) == c
}

// safeWord keeps a code language or raw format usable in a fence info
// string.
func safeWord(s string) string {
	s = strings.TrimSpace(s)
	if strings.Trim(s, "{}.") != s {
		return "" // the reader trims these from the ends
	}
	for _, r := range s {
		if unicode.IsSpace(r) || r == '`' {
			return ""
		}
	}
	return s
}

func sortStrings(s []string) { sort.Strings(s) }

// ---------------------------------------------------------------------------
// Media
// ---------------------------------------------------------------------------

// mediaSet collects the images written next to the Markdown.
type mediaSet struct {
	dir   string
	load  func(string) (string, []byte, error)
	files map[string][]byte
	bySrc map[string]string
	bySum map[[32]byte]string
}

func newMediaSet(o Options) *mediaSet {
	return &mediaSet{dir: o.MediaDir, load: o.LoadImage, files: map[string][]byte{}, bySrc: map[string]string{}, bySum: map[[32]byte]string{}}
}

// resolve returns the source to write for an image: a path inside the
// media directory for embedded and local images, the original source for
// remote and missing images.
func (m *mediaSet) resolve(w *writer, src string) string {
	s := strings.TrimSpace(src)
	lower := strings.ToLower(s)
	if s == "" || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || m.load == nil {
		return src
	}
	if p, ok := m.bySrc[s]; ok {
		return p
	}
	name, data, err := m.load(s)
	if err != nil || len(data) == 0 {
		reason := "not found"
		if err != nil {
			reason = err.Error()
		}
		if strings.HasPrefix(lower, "data:") {
			w.warn("embedded image could not be decoded: %s", reason)
		} else {
			w.warn("image %s kept as a link: %s", shortSrc(s), reason)
		}
		m.bySrc[s] = src
		return src
	}
	sum := sha256.Sum256(data)
	if p, ok := m.bySum[sum]; ok {
		m.bySrc[s] = p
		return p
	}
	base := safeFileName(name, data)
	p := m.dir + "/" + base
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for k := 2; ; k++ {
		if _, taken := m.files[p]; !taken {
			break
		}
		p = m.dir + "/" + stem + "-" + itoa(k) + ext
	}
	m.files[p] = data
	m.byNum(sum, p)
	m.bySrc[s] = p
	return p
}

func (m *mediaSet) byNum(sum [32]byte, p string) { m.bySum[sum] = p }

func shortSrc(s string) string {
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}

// safeFileName makes a portable file name: ASCII letters, digits, ".", "-"
// and "_", no leading dot, with an extension matching the content.
func safeFileName(name string, data []byte) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	var sb strings.Builder
	dash := false
	for _, r := range name {
		ok := r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-')
		if ok {
			sb.WriteRune(r)
			dash = false
		} else if !dash {
			sb.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(sb.String(), ".-")
	for strings.Contains(s, "..") {
		s = strings.ReplaceAll(s, "..", ".")
	}
	ext := strings.ToLower(path.Ext(s))
	stem := strings.TrimSuffix(s, path.Ext(s))
	if stem == "" {
		stem = "image"
	}
	if len(stem) > 60 {
		stem = stem[:60]
	}
	if sniffed := sniffExt(data); sniffed != "" && ext != sniffed && !(ext == ".jpeg" && sniffed == ".jpg") && !(ext == ".tiff" && sniffed == ".tif") {
		ext = sniffed
	}
	if ext == "" || len(ext) > 6 {
		ext = ".bin"
	}
	return stem + ext
}

func sniffExt(b []byte) string {
	switch {
	case len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n":
		return ".png"
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return ".jpg"
	case len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a"):
		return ".gif"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return ".webp"
	case len(b) >= 2 && string(b[:2]) == "BM":
		return ".bmp"
	case len(b) >= 4 && (string(b[:4]) == "II*\x00" || string(b[:4]) == "MM\x00*"):
		return ".tif"
	case len(b) >= 5 && string(b[:5]) == "%PDF-":
		return ".pdf"
	}
	head := strings.ToLower(string(b[:min(len(b), 1024)]))
	if strings.Contains(head, "<svg") {
		return ".svg"
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
