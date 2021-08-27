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
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"io"
	"math/big"
)

// Make a random 12 byte big.Int
func MakeSerial() *big.Int {
	blob := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, blob); err != nil {
		return nil
	}
	return new(big.Int).SetBytes(blob)
}

// Choose a X509 signature algorithm suitable for the specified public key
func X509SignatureAlgorithm(pub crypto.PublicKey) x509.SignatureAlgorithm {
	switch pub.(type) {
	case *rsa.PublicKey:
		if ArgRSAPSS {
			return x509.SHA256WithRSAPSS
		}
		return x509.SHA256WithRSA
	case *ecdsa.PublicKey:
		return x509.ECDSAWithSHA256
	default:
		return x509.UnknownSignatureAlgorithm
	}
}

type pkixPublicKey struct {
	Algo      pkix.AlgorithmIdentifier
	BitString asn1.BitString
}

// Calculcate subject key identifier from a public key per RFC 3280
func SubjectKeyID(pub crypto.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	// extract the raw "bit string" part of the public key bytes
	var pki pkixPublicKey
	if rest, err := asn1.Unmarshal(der, &pki); err != nil {
		return nil, err
	} else if len(rest) != 0 {
		return nil, errors.New("trailing garbage on public key")
	}
	digest := sha1.Sum(pki.BitString.RightAlign())
	return digest[:], nil
}

// Test whether two public or private keys have the same public key
func SameKey(pub1, pub2 interface{}) bool {
	if privkey, ok := pub1.(crypto.Signer); ok {
		pub1 = privkey.Public()
	}
	if privkey, ok := pub2.(crypto.Signer); ok {
		pub2 = privkey.Public()
	}
	switch key1 := pub1.(type) {
	case *rsa.PublicKey:
		key2, ok := pub2.(*rsa.PublicKey)
		return ok && key1.E == key2.E && key1.N.Cmp(key2.N) == 0
	case *ecdsa.PublicKey:
		key2, ok := pub2.(*ecdsa.PublicKey)
		return ok && key1.X.Cmp(key2.X) == 0 && key1.Y.Cmp(key2.Y) == 0
	default:
		return false
	}
}

// Verify an RSA or ECDSA signature
func Verify(pub interface{}, hash crypto.Hash, hashed []byte, sig []byte) error {
	switch pubk := pub.(type) {
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(pubk, hash, hashed, sig)
	case *ecdsa.PublicKey:
		esig, err := UnmarshalEcdsaSignature(sig)
		if err != nil {
			return err
		}
		if !ecdsa.Verify(pubk, hashed, esig.R, esig.S) {
			return errors.New("ECDSA verification failed")
		}
		return nil
	}
	return errors.New("unsupported public key algorithm")
}

// Determine the type of a public or private key
func GetPublicKeyAlgorithm(key interface{}) x509.PublicKeyAlgorithm {
	if privkey, ok := key.(crypto.Signer); ok {
		key = privkey.Public()
	}
	switch key.(type) {
	case *rsa.PublicKey:
		return x509.RSA
	case *ecdsa.PublicKey:
		return x509.ECDSA
	default:
		return x509.UnknownPublicKeyAlgorithm
	}
}
