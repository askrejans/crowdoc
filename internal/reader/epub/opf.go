package epub

import (
	"bytes"
	"encoding/xml"
	"strings"

	"golang.org/x/net/html/charset"
)

type container struct {
	Rootfiles []struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

type opfPackage struct {
	Version  string      `xml:"version,attr"`
	Metadata opfMetadata `xml:"metadata"`
	Manifest []opfItem   `xml:"manifest>item"`
	Spine    opfSpine    `xml:"spine"`
}

type opfMetadata struct {
	Titles       []opfElem `xml:"title"`
	Creators     []opfElem `xml:"creator"`
	Languages    []string  `xml:"language"`
	Dates        []opfElem `xml:"date"`
	Descriptions []string  `xml:"description"`
	Subjects     []string  `xml:"subject"`
	Publishers   []string  `xml:"publisher"`
	Metas        []opfMeta `xml:"meta"`
}

// opfElem is a Dublin Core element with the attributes EPUB 2 (opf:role,
// opf:event) and EPUB 3 (id, refined by <meta>) attach to it.
type opfElem struct {
	ID    string `xml:"id,attr"`
	Role  string `xml:"role,attr"`
	Event string `xml:"event,attr"`
	Value string `xml:",chardata"`
}

type opfMeta struct {
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Property string `xml:"property,attr"`
	Refines  string `xml:"refines,attr"`
	Value    string `xml:",chardata"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type opfSpine struct {
	Toc      string       `xml:"toc,attr"`
	Itemrefs []opfItemref `xml:"itemref"`
}

type opfItemref struct {
	IDRef  string `xml:"idref,attr"`
	Linear string `xml:"linear,attr"`
}

func unmarshalXML(data []byte, v any) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charset.NewReaderLabel
	return d.Decode(v)
}

// refinement returns the value of an EPUB 3 <meta refines="#id"> property.
func (m *opfMetadata) refinement(id, property string) string {
	if id == "" {
		return ""
	}
	for _, mt := range m.Metas {
		if mt.Refines == "#"+id && mt.Property == property {
			return strings.TrimSpace(mt.Value)
		}
	}
	return ""
}
