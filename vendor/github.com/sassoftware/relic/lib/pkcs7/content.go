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
	"encoding/asn1"
)

type contentInfo2 struct {
	ContentType asn1.ObjectIdentifier
	Value       asn1.RawValue
}

// Create a ContentInfo structure for the given bytes or structure. data can be
// nil for detached signatures.
func NewContentInfo(contentType asn1.ObjectIdentifier, data interface{}) (ci ContentInfo, err error) {
	if data == nil {
		return ContentInfo{ContentType: contentType}, nil
	}
	// There's no way to just encode the struct with the asn1.RawValue directly
	// while also supporting the ability to not emit the 2nd field for the nil
	// case, so instead this stupid dance of encoding it with the field then
	// stuffing it into Raw is necessary...
	encoded, err := asn1.Marshal(data)
	if err != nil {
		return ContentInfo{}, err
	}
	ci2 := contentInfo2{
		ContentType: contentType,
		Value: asn1.RawValue{
			Class:      asn1.ClassContextSpecific,
			Tag:        0,
			IsCompound: true,
			Bytes:      encoded,
		},
	}
	ciblob, err := asn1.Marshal(ci2)
	if err != nil {
		return ContentInfo{}, nil
	}
	return ContentInfo{Raw: ciblob, ContentType: contentType}, nil
}

// Unmarshal a structure from a ContentInfo.
func (ci ContentInfo) Unmarshal(dest interface{}) (err error) {
	// First re-decode the contentinfo but this time with the second field
	var ci2 contentInfo2
	_, err = asn1.Unmarshal(ci.Raw, &ci2)
	if err == nil {
		// Now decode the raw value in the second field
		_, err = asn1.Unmarshal(ci2.Value.Bytes, dest)
	}
	return
}

// Get raw content in DER encoding, or nil if it's not present
func (ci ContentInfo) Bytes() ([]byte, error) {
	var value asn1.RawValue
	if err := ci.Unmarshal(&value); err != nil {
		if _, ok := err.(asn1.SyntaxError); ok {
			// short sequence because the value was omitted
			return nil, nil
		}
		return nil, err
	}
	return value.Bytes, nil
}
