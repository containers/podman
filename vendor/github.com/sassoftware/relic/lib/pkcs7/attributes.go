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
	"fmt"
)

type ErrNoAttribute struct {
	ID asn1.ObjectIdentifier
}

func (e ErrNoAttribute) Error() string {
	return fmt.Sprintf("attribute not found: %s", e.ID)
}

// marshal authenticated attributes for digesting
func (l *AttributeList) Bytes() ([]byte, error) {
	// needs an explicit SET OF tag but not the class-specific tag from the
	// original struct. see RFC 2315 9.3, 2nd paragraph
	encoded, err := asn1.Marshal(struct {
		A []Attribute `asn1:"set"`
	}{A: *l})
	if err != nil {
		return nil, err
	}
	var raw asn1.RawValue
	if _, err := asn1.Unmarshal(encoded, &raw); err != nil {
		return nil, err
	}
	return raw.Bytes, nil
}

// unmarshal a single attribute, if it exists
func (l *AttributeList) GetOne(oid asn1.ObjectIdentifier, dest interface{}) error {
	for _, raw := range *l {
		if !raw.Type.Equal(oid) {
			continue
		}
		rest, err := asn1.Unmarshal(raw.Values.Bytes, dest)
		if err != nil {
			return err
		} else if len(rest) != 0 {
			return fmt.Errorf("attribute %s: expected one, found multiple", oid)
		} else {
			return nil
		}
	}
	return ErrNoAttribute{oid}
}

// create or append to an attribute
func (l *AttributeList) Add(oid asn1.ObjectIdentifier, obj interface{}) error {
	value, err := asn1.Marshal(obj)
	if err != nil {
		return err
	}
	for _, attr := range *l {
		if attr.Type.Equal(oid) {
			attr.Values.Bytes = append(attr.Values.Bytes, value...)
			return nil
		}
	}
	*l = append(*l, Attribute{
		Type: oid,
		Values: asn1.RawValue{
			Class:      asn1.ClassUniversal,
			Tag:        asn1.TagSet,
			IsCompound: true,
			Bytes:      value,
		}})
	return nil
}
