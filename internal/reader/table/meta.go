package table

import (
	"encoding/xml"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// readCoreProps fills metadata from OOXML docProps/core.xml or ODF
// meta.xml (both use Dublin Core names).
func readCoreProps(z zipReader, path string, m *ast.Meta) {
	data, err := z.Read(path)
	if err != nil {
		return
	}
	var creator, initial string
	var buf strings.Builder
	d := newDecoder(data)
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			buf.Reset()
		case xml.CharData:
			buf.Write(t)
		case xml.EndElement:
			v := strings.TrimSpace(buf.String())
			buf.Reset()
			if v == "" {
				continue
			}
			switch t.Name.Local {
			case "title":
				m.Title = v
			case "subject":
				m.Subject = v
			case "creator":
				creator = v
			case "initial-creator":
				initial = v
			case "description":
				m.Summary = v
			case "language":
				m.Lang = v
			case "keywords", "keyword":
				for _, k := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' }) {
					if k = strings.TrimSpace(k); k != "" {
						m.Keywords = append(m.Keywords, k)
					}
				}
			}
		}
	}
	// ODF's dc:creator is the last editor; the author is initial-creator.
	if initial != "" {
		creator = initial
	}
	if creator != "" {
		m.Authors = append(m.Authors, ast.Author{Name: creator})
	}
}
