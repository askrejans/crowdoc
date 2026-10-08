package table

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"golang.org/x/text/encoding/ianaindex"
)

// newDecoder returns a lenient decoder; reading uses RawToken and local
// names only, so namespace prefixes (transitional or strict OOXML, ODF)
// do not matter.
func newDecoder(b []byte) *xml.Decoder {
	d := xml.NewDecoder(bytes.NewReader(b))
	d.Strict = false
	d.CharsetReader = func(label string, r io.Reader) (io.Reader, error) {
		enc, err := ianaindex.IANA.Encoding(label)
		if err != nil || enc == nil {
			return r, nil
		}
		return enc.NewDecoder().Reader(r), nil
	}
	return d
}

// attr returns the value of the attribute with the given local name.
func attr(se xml.StartElement, local string) string {
	for _, a := range se.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// relationship is one entry of an OPC .rels part.
type relationship struct {
	typ, target string
	external    bool
}

// readRels parses the relationships of part (e.g. "xl/workbook.xml" ->
// "xl/_rels/workbook.xml.rels"); targets are resolved to archive paths.
func readRels(z zipReader, part string) map[string]relationship {
	dir, file := "", part
	if i := strings.LastIndexByte(part, '/'); i >= 0 {
		dir, file = part[:i+1], part[i+1:]
	}
	data, err := z.Read(dir + "_rels/" + file + ".rels")
	rels := map[string]relationship{}
	if err != nil {
		return rels
	}
	d := newDecoder(data)
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Relationship" {
			continue
		}
		rel := relationship{typ: attr(se, "Type"), target: attr(se, "Target"), external: strings.EqualFold(attr(se, "TargetMode"), "External")}
		if !rel.external {
			rel.target = resolvePath(part, rel.target)
		}
		rels[attr(se, "Id")] = rel
	}
	return rels
}

func relType(r relationship, suffix string) bool { return strings.HasSuffix(r.typ, "/"+suffix) }

// zipReader is the part of *rd.Zip the readers use.
type zipReader interface {
	Read(name string) ([]byte, error)
}
