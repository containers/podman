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
	"errors"
	"fmt"
	"time"
)

type ErrNoAttribute struct {
	ID asn1.ObjectIdentifier
}

func (e ErrNoAttribute) Error() string {
	return fmt.Sprintf("attribute not found: %s", e.ID)
}

// Bytes returns a SET OF form of the attribute list for digesting, per RFC 2315 9.3, 2nd paragraph
func (l *AttributeList) Bytes() ([]byte, error) {
	return marshalUnsortedSet(*l)
}

// Need to marshal authenticated attributes as a SET OF in order to digest them,
// but since go 1.15 sets get sorted which breaks the digest. Marshal as a
// sequence and then change the tag.
func marshalUnsortedSet(v interface{}) ([]byte, error) {
	encoded, err := asn1.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(encoded) > 0 {
		if encoded[0]&0x1f != asn1.TagSequence {
			return nil, fmt.Errorf("expected sequence, got %d", encoded[0]&0x1f)
		}
		// sequence 16 -> set 17
		encoded[0] |= 1
	}
	return encoded, nil
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

func (i SignerInfo) SigningTime() (time.Time, error) {
	var raw asn1.RawValue
	if err := i.AuthenticatedAttributes.GetOne(OidAttributeSigningTime, &raw); err != nil {
		return time.Time{}, err
	}
	return ParseTime(raw)
}

// AuthenticatedAttributesBytes returns a SET OF form of the attribute list for digesting, per RFC 2315 9.3, 2nd paragraph
func (i SignerInfo) AuthenticatedAttributesBytes() ([]byte, error) {
	if i.RawContent == nil {
		return i.AuthenticatedAttributes.Bytes()
	}
	// decode the SignerInfo as a sequence of raw values to extract how the
	// authenticated attributes were originally encoded extract the original
	var seq []asn1.RawValue
	if _, err := asn1.Unmarshal(i.RawContent, &seq); err != nil {
		return nil, err
	}
	if len(seq) < 4 {
		return nil, errors.New("short sequence in SignerInfo")
	}
	raw := seq[3]
	// tweak the attribute sequence to be a set
	return marshalUnsortedSet(asn1.RawValue{
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      raw.Bytes,
	})
}
