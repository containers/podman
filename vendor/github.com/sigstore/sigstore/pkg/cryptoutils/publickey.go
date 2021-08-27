//
// Copyright 2021 The Sigstore Authors.
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

package cryptoutils

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

const (
	PublicKeyPEMType PEMType = "PUBLIC KEY"
)

// UnmarshalPEMToPublicKey converts a PEM-encoded byte slice into a crypto.PublicKey
func UnmarshalPEMToPublicKey(pemBytes []byte) (crypto.PublicKey, error) {
	derBytes, _ := pem.Decode(pemBytes)
	if derBytes == nil {
		return nil, errors.New("PEM decoding failed")
	}
	return x509.ParsePKIXPublicKey(derBytes.Bytes)
}

// MarshalPublicKeyToDER converts a crypto.PublicKey into a PKIX, ASN.1 DER byte slice
func MarshalPublicKeyToDER(pub crypto.PublicKey) ([]byte, error) {
	if pub == nil {
		return nil, errors.New("empty key")
	}
	return x509.MarshalPKIXPublicKey(pub)
}

// MarshalPublicKeyToPEM converts a crypto.PublicKey into a PEM-encoded byte slice
func MarshalPublicKeyToPEM(pub crypto.PublicKey) ([]byte, error) {
	derBytes, err := MarshalPublicKeyToDER(pub)
	if err != nil {
		return nil, err
	}
	return PEMEncode(PublicKeyPEMType, derBytes), nil
}
