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

// PKCS#7 is a specification for signing or encrypting data using ASN.1
// structures. It is also known as CMS (cryptographic message syntax) and is
// discussed in RFC 2315, RFC 3369, RFC 3852, and RFC 5652.
//
// This package implements signature operations needed for creating and
// validating signature technologies based on PKCS#7 including Java and
// Microsoft Authenticode
package pkcs7

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
)

var (
	OidData                   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	OidSignedData             = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	OidAttributeContentType   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	OidAttributeMessageDigest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	OidAttributeSigningTime   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
)

const MimeType = "application/pkcs7-mime"

type ContentInfo struct {
	Raw         asn1.RawContent
	ContentType asn1.ObjectIdentifier
}

type ContentInfoSignedData struct {
	ContentType asn1.ObjectIdentifier
	Content     SignedData `asn1:"explicit,optional,tag:0"`
}

type SignedData struct {
	Version                    int                        `asn1:"default:1"`
	DigestAlgorithmIdentifiers []pkix.AlgorithmIdentifier `asn1:"set"`
	ContentInfo                ContentInfo                ``
	Certificates               RawCertificates            `asn1:"optional,tag:0"`
	CRLs                       []pkix.CertificateList     `asn1:"optional,tag:1"`
	SignerInfos                []SignerInfo               `asn1:"set"`
}

type RawCertificates []asn1.RawValue

type Attribute struct {
	Type   asn1.ObjectIdentifier
	Values asn1.RawValue
}

type AttributeList []Attribute

type SignerInfo struct {
	RawContent asn1.RawContent

	Version                   int                      `asn1:"default:1"`
	IssuerAndSerialNumber     IssuerAndSerial          ``
	DigestAlgorithm           pkix.AlgorithmIdentifier ``
	AuthenticatedAttributes   AttributeList            `asn1:"optional,tag:0"`
	DigestEncryptionAlgorithm pkix.AlgorithmIdentifier ``
	EncryptedDigest           []byte                   ``
	UnauthenticatedAttributes AttributeList            `asn1:"optional,tag:1"`
}

type IssuerAndSerial struct {
	IssuerName   asn1.RawValue
	SerialNumber *big.Int
}
