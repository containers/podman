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
		return nil, fmt.Errorf("pkcs7: %s", err)
	}
	psd.Content.ContentInfo, _ = NewContentInfo(psd.Content.ContentInfo.ContentType, nil)
	return content, nil
}

// dump raw certificates to structure
func marshalCertificates(certs []*x509.Certificate) RawCertificates {
	var buf bytes.Buffer
	for _, cert := range certs {
		buf.Write(cert.Raw)
	}
	val := asn1.RawValue{Bytes: buf.Bytes(), Class: 2, Tag: 0, IsCompound: true}
	b, _ := asn1.Marshal(val)
	return RawCertificates{Raw: b}
}

// parse raw certificates from structure
func (raw RawCertificates) Parse() ([]*x509.Certificate, error) {
	var val asn1.RawValue
	if len(raw.Raw) == 0 {
		return nil, nil
	}
	if _, err := asn1.Unmarshal(raw.Raw, &val); err != nil {
		return nil, err
	}
	return x509.ParseCertificates(val.Bytes)
}
