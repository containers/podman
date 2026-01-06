//go:build windows

package hypervctl

import (
	"encoding/xml"
	"io"
	"strings"
)

type CimKvpItems struct {
	Instances []CimKvpItem `xml:"INSTANCE"`
}

type CimKvpItem struct {
	Properties []CimKvpItemProperty `xml:"PROPERTY"`
}

type CimKvpItemProperty struct {
	Name  string `xml:"NAME,attr"`
	Value string `xml:"VALUE"`
}

func parseKvpMapXml(kvpXml string) (map[string]string, error) {
	// Workaround XML decoder's inability to handle multiple root elements
	r := io.MultiReader(
		strings.NewReader("<root>"),
		strings.NewReader(kvpXml),
		strings.NewReader("</root>"),
	)

	var items CimKvpItems
	if err := xml.NewDecoder(r).Decode(&items); err != nil {
		return nil, err
	}

	ret := make(map[string]string)
	for _, item := range items.Instances {
		var key, value string
		for _, prop := range item.Properties {
			if strings.EqualFold(prop.Name, "Name") {
				key = prop.Value
			} else if strings.EqualFold(prop.Name, "Data") {
				value = prop.Value
			}
		}
		if len(key) > 0 {
			ret[key] = value
		}
	}

	return ret, nil
}
