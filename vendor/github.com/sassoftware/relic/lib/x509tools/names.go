//
// Copyright (c) SAS Institute Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package x509tools

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

type NameStyle int

const (
	NameStyleOpenSsl NameStyle = iota
	NameStyleLdap
	NameStyleMsOsco
)

type attrName struct {
	Type asn1.ObjectIdentifier
	Name string
}

var nameStyleLdap = []attrName{
	{asn1.ObjectIdentifier{2, 5, 4, 3}, "CN"},
	{asn1.ObjectIdentifier{2, 5, 4, 4}, "surname"},
	{asn1.ObjectIdentifier{2, 5, 4, 5}, "serialNumber"},
	{asn1.ObjectIdentifier{2, 5, 4, 6}, "C"},
	{asn1.ObjectIdentifier{2, 5, 4, 7}, "L"},
	{asn1.ObjectIdentifier{2, 5, 4, 8}, "ST"},
	{asn1.ObjectIdentifier{2, 5, 4, 9}, "street"},
	{asn1.ObjectIdentifier{2, 5, 4, 10}, "O"},
	{asn1.ObjectIdentifier{2, 5, 4, 11}, "OU"},
	{asn1.ObjectIdentifier{2, 5, 4, 12}, "title"},
	{asn1.ObjectIdentifier{2, 5, 4, 13}, "description"},
	{asn1.ObjectIdentifier{2, 5, 4, 17}, "postalCode"},
	{asn1.ObjectIdentifier{2, 5, 4, 18}, "postOfficeBox"},
	{asn1.ObjectIdentifier{2, 5, 4, 20}, "telephoneNumber"},
	{asn1.ObjectIdentifier{2, 5, 4, 42}, "givenName"},
	{asn1.ObjectIdentifier{2, 5, 4, 43}, "initials"},
	{asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25}, "dc"},
	{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1}, "emailAddress"},
}

// Per [MS-OSCO]
// https://msdn.microsoft.com/en-us/library/dd947276(v=office.12).aspx
var nameStyleMsOsco = []attrName{
	{asn1.ObjectIdentifier{2, 5, 4, 3}, "CN"},
	{asn1.ObjectIdentifier{2, 5, 4, 7}, "L"},
	{asn1.ObjectIdentifier{2, 5, 4, 10}, "O"},
	{asn1.ObjectIdentifier{2, 5, 4, 11}, "OU"},
	{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1}, "E"},
	{asn1.ObjectIdentifier{2, 5, 4, 6}, "C"},
	{asn1.ObjectIdentifier{2, 5, 4, 8}, "S"},
	{asn1.ObjectIdentifier{2, 5, 4, 9}, "STREET"},
	{asn1.ObjectIdentifier{2, 5, 4, 12}, "T"},
	{asn1.ObjectIdentifier{2, 5, 4, 42}, "G"},
	{asn1.ObjectIdentifier{2, 5, 4, 43}, "I"},
	{asn1.ObjectIdentifier{2, 5, 4, 4}, "SN"},
	{asn1.ObjectIdentifier{2, 5, 4, 5}, "SERIALNUMBER"},
	{asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25}, "DC"},
	{asn1.ObjectIdentifier{2, 5, 4, 13}, "Description"},
	{asn1.ObjectIdentifier{2, 5, 4, 17}, "PostalCode"},
	{asn1.ObjectIdentifier{2, 5, 4, 18}, "POBox"},
	{asn1.ObjectIdentifier{2, 5, 4, 20}, "Phone"},
}

// returned by the Format* functions in case there's something cripplingly
// wrong with it
const InvalidName = "<invalid>"

// Format the name (RDN sequence) from its raw DER to a readable style.
func FormatPkixName(der []byte, style NameStyle) string {
	var seq pkix.RDNSequence
	if _, err := asn1.Unmarshal(der, &seq); err != nil {
		return InvalidName
	}
	var b strings.Builder
	if style == NameStyleOpenSsl {
		for _, rdnSet := range seq {
			for _, attr := range rdnSet {
				b.WriteByte('/')
				b.WriteString(attName(attr.Type, style))
				b.WriteByte('=')
				b.WriteString(attValue(attr.Value, style))
			}
		}
	} else {
		// Per RFC 2253 2.1, reverse the order
		for i := len(seq) - 1; i >= 0; i-- {
			if i < len(seq)-1 {
				b.WriteString(", ")
			}
			rdnSet := seq[i]
			for j, attr := range rdnSet {
				if j > 0 {
					b.WriteString(" + ")
				}
				b.WriteString(attName(attr.Type, style))
				b.WriteByte('=')
				b.WriteString(attValue(attr.Value, style))
			}
		}
	}
	return b.String()
}

func attName(t asn1.ObjectIdentifier, style NameStyle) string {
	var names []attrName
	var defaultPrefix string
	switch style {
	case NameStyleLdap, NameStyleOpenSsl:
		names = nameStyleLdap
	case NameStyleMsOsco:
		names = nameStyleMsOsco
		defaultPrefix = "OID."
	default:
		panic("invalid style argument")
	}
	for _, name := range names {
		if name.Type.Equal(t) {
			return name.Name
		}
	}
	return defaultPrefix + t.String()
}

func attValue(raw interface{}, style NameStyle) string {
	var value string
	switch v := raw.(type) {
	case string:
		value = v
	case fmt.Stringer:
		value = v.String()
	case int64:
		value = fmt.Sprint(v)
	default:
		return InvalidName
	}
	switch style {
	case NameStyleOpenSsl:
		value = strings.Replace(value, "/", "\\/", -1)
	case NameStyleLdap, NameStyleMsOsco:
		quote := false
		if len(value) == 0 {
			quote = true
		}
		if strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
			quote = true
		}
		if i := strings.IndexAny(value, ",+=\n<>#;'\""); i >= 0 {
			quote = true
		}
		value = strings.Replace(value, "\"", "\"\"", -1)
		if quote {
			value = "\"" + value + "\""
		}
	}
	return value
}

// DEPRECATED - Use asn1.Unmarshal
func ParseBMPString(raw asn1.RawValue) (s string) {
	asn1.Unmarshal(raw.FullBytes, &s)
	return
}

func ToBMPString(value string) asn1.RawValue {
	runes := utf16.Encode([]rune(value))
	raw := make([]byte, 2*len(runes))
	for i, r := range runes {
		binary.BigEndian.PutUint16(raw[i*2:], r)
	}
	return asn1.RawValue{Tag: asn1.TagBMPString, Bytes: raw}
}

// Format the certificate subject name in LDAP style
func FormatSubject(cert *x509.Certificate) string {
	return FormatPkixName(cert.RawSubject, NameStyleLdap)
}

// Format the certificate issuer name in LDAP style
func FormatIssuer(cert *x509.Certificate) string {
	return FormatPkixName(cert.RawIssuer, NameStyleLdap)
}
