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

package pkcs7

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"time"
)

// Parse a signature from bytes
func Unmarshal(blob []byte) (*ContentInfoSignedData, error) {
	psd := new(ContentInfoSignedData)
	if rest, err := asn1.Unmarshal(blob, psd); err != nil {
		return nil, err
	} else if len(bytes.TrimRight(rest, "\x00")) != 0 {
		return nil, errors.New("pkcs7: trailing garbage after PKCS#7 structure")
	}
	return psd, nil
}

// Marshal the signature to bytes
func (psd *ContentInfoSignedData) Marshal() ([]byte, error) {
	return asn1.Marshal(*psd)
}

// Remove and return inlined content from the document, leaving a detached signature
func (psd *ContentInfoSignedData) Detach() ([]byte, error) {
	content, err := psd.Content.ContentInfo.Bytes()
	if err != nil {
		return nil, fmt.Errorf("pkcs7: %w", err)
	}
	psd.Content.ContentInfo, _ = NewContentInfo(psd.Content.ContentInfo.ContentType, nil)
	return content, nil
}

// dump raw certificates to structure
func marshalCertificates(certs []*x509.Certificate) RawCertificates {
	c := make(RawCertificates, len(certs))
	for i, cert := range certs {
		c[i] = asn1.RawValue{FullBytes: cert.Raw}
	}
	return c
}

// Parse raw certificates from structure. If any cert is invalid, the remaining valid certs are returned along with a CertificateError.
func (raw RawCertificates) Parse() ([]*x509.Certificate, error) {
	var invalid CertificateError
	var certs []*x509.Certificate
	for _, rawCert := range raw {
		cert, err := x509.ParseCertificate(rawCert.FullBytes)
		if err != nil {
			invalid.Invalid = append(invalid.Invalid, rawCert.FullBytes)
			invalid.Err = err
		} else {
			certs = append(certs, cert)
		}
	}
	if invalid.Err != nil {
		return certs, invalid
	}
	return certs, nil
}

type CertificateError struct {
	Invalid [][]byte
	Err     error
}

func (c CertificateError) Error() string {
	return c.Err.Error()
}

func (c CertificateError) Unwrap() error {
	return c.Err
}

// ParseTime parses a GeneralizedTime or UTCTime value that potentially has a fractional seconds part
func ParseTime(raw asn1.RawValue) (ret time.Time, err error) {
	// as of go 1.12 fractional timestamps fail to parse with a "did not serialize back to the original value" error, so this implementation without the serialize check is needed
	s := string(raw.Bytes)
	switch raw.Tag {
	case asn1.TagGeneralizedTime:
		formatStr := "20060102150405Z0700"
		return time.Parse(formatStr, s)
	case asn1.TagUTCTime:
		formatStr := "0601021504Z0700"
		ret, err = time.Parse(formatStr, s)
		if err != nil {
			formatStr = "060102150405Z0700"
			ret, err = time.Parse(formatStr, s)
		}
		return
	default:
		err = fmt.Errorf("unknown tag %d in timestamp field", raw.Tag)
		return
	}
}
