// Package reference defines the elements of a <reference> block.
package reference

import "encoding/xml"

// Author is the reference author.
type Author struct {
	Fullname     string        `xml:"fullname,attr,omitempty"`
	Initials     string        `xml:"initials,attr,omitempty"`
	Surname      string        `xml:"surname,attr,omitempty"`
	Role         string        `xml:"role,attr,omitempty"`
	Organization *Organization `xml:"organization,omitempty"`
	Address      *Address      `xml:"address,omitempty"`
}

type Organization struct {
	Abbrev string `xml:"abbrev,attr,omitempty"`
	Value  string `xml:",chardata"`
}

// this is copied from ../title.go; it might make sense to unify them, both especially, it we
// want to allow reference to be given in TOML as well. See #55.
// Author denotes an RFC author.

// Address denotes the address of an RFC author.
type Address struct {
	Phone  string        `xml:"phone,omitempty"`
	Email  string        `xml:"email,omitempty"`
	URI    string        `xml:"uri,omitempty"`
	Postal AddressPostal `xml:"postal,omitempty"`
}

// AddressPostal denotes the postal address of an RFC author.
type AddressPostal struct {
	PostalLine []string `xml:"postalline,omitempty"`

	Streets   []string `xml:"street,omitempty"`
	Cities    []string `xml:"city,omitempty"`
	Codes     []string `xml:"code,omitempty"`
	Countries []string `xml:"country,omitempty"`
	Regions   []string `xml:"region,omitempty"`
}

// Date is the reference date.
type Date struct {
	Year  string `xml:"year,attr,omitempty"`
	Month string `xml:"month,attr,omitempty"`
	Day   string `xml:"day,attr,omitempty"`
}

// Front the reference <front>.
type Front struct {
	Title   string   `xml:"title"`
	Authors []Author `xml:"author,omitempty"`
	Date    Date     `xml:"date"`
}

// Format is the reference <format>. This is deprecated in RFC 7991, see Section 3.3.
type Format struct {
	Type   string `xml:"type,attr,omitempty"`
	Target string `xml:"target,attr"`
}

// Reference is the entire <reference> structure.
type Reference struct {
	XMLName xml.Name `xml:"reference"`
	Anchor  string   `xml:"anchor,attr"`
	Front   Front    `xml:"front"`
	Format  *Format  `xml:"format,omitempty"`
	Target  string   `xml:"target,attr"`
}
