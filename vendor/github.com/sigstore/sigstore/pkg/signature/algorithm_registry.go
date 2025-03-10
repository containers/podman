//
// Copyright 2024 The Sigstore Authors.
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

package signature

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"fmt"

	v1 "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
)

// PublicKeyType represents the public key algorithm for a given signature algorithm.
type PublicKeyType uint

const (
	// RSA public key
	RSA PublicKeyType = iota
	// ECDSA public key
	ECDSA
	// ED25519 public key
	ED25519
)

// RSAKeySize represents the size of an RSA public key in bits.
type RSAKeySize int

type algorithmDetails struct {
	// knownAlgorithm is the signature algorithm that the following details refer to.
	knownAlgorithm v1.PublicKeyDetails

	// keyType is the public key algorithm being used.
	keyType PublicKeyType

	// hashType is the hash algorithm being used.
	hashType crypto.Hash

	// extraKeyParams contains any extra parameters required to check a given public key against this entry.
	//
	// The underlying type of these parameters is dependent on the keyType.
	// For example, ECDSA algorithms will store an elliptic curve here whereas, RSA keys will store the key size.
	// Algorithms that don't require any extra parameters leave this set to nil.
	extraKeyParams interface{}

	// flagValue is a string representation of the signature algorithm that follows the naming conventions of CLI
	// arguments that are used for Sigstore services.
	flagValue string
}

func (a algorithmDetails) GetRSAKeySize() (RSAKeySize, error) {
	if a.keyType != RSA {
		return 0, fmt.Errorf("unable to retrieve RSA key size for key type: %T", a.keyType)
	}
	rsaKeySize, ok := a.extraKeyParams.(RSAKeySize)
	if !ok {
		// This should be unreachable.
		return 0, fmt.Errorf("unable to retrieve key size for RSA, malformed algorithm details?: %T", a.keyType)
	}
	return rsaKeySize, nil
}

func (a algorithmDetails) GetECDSACurve() (*elliptic.Curve, error) {
	if a.keyType != ECDSA {
		return nil, fmt.Errorf("unable to retrieve ECDSA curve for key type: %T", a.keyType)
	}
	ecdsaCurve, ok := a.extraKeyParams.(elliptic.Curve)
	if !ok {
		// This should be unreachable.
		return nil, fmt.Errorf("unable to retrieve curve for ECDSA, malformed algorithm details?: %T", a.keyType)
	}
	return &ecdsaCurve, nil
}

func (a algorithmDetails) checkKey(pubKey crypto.PublicKey) (bool, error) {
	switch a.keyType {
	case RSA:
		rsaKey, ok := pubKey.(*rsa.PublicKey)
		if !ok {
			return false, nil
		}
		keySize, err := a.GetRSAKeySize()
		if err != nil {
			return false, err
		}
		return rsaKey.Size()*8 == int(keySize), nil
	case ECDSA:
		ecdsaKey, ok := pubKey.(*ecdsa.PublicKey)
		if !ok {
			return false, nil
		}
		curve, err := a.GetECDSACurve()
		if err != nil {
			return false, err
		}
		return ecdsaKey.Curve == *curve, nil
	case ED25519:
		_, ok := pubKey.(ed25519.PublicKey)
		return ok, nil
	}
	return false, fmt.Errorf("unrecognized key type: %T", a.keyType)
}

func (a algorithmDetails) checkHash(hashType crypto.Hash) bool {
	return a.hashType == hashType
}

// Note that deprecated options in PublicKeyDetails are not included in this
// list, including PKCS1v1.5 encoded RSA. Refer to the v1.PublicKeyDetails enum
// for more details.
var supportedAlgorithms = []algorithmDetails{
	{v1.PublicKeyDetails_PKIX_RSA_PKCS1V15_2048_SHA256, RSA, crypto.SHA256, RSAKeySize(2048), "rsa-sign-pkcs1-2048-sha256"},
	{v1.PublicKeyDetails_PKIX_RSA_PKCS1V15_3072_SHA256, RSA, crypto.SHA256, RSAKeySize(3072), "rsa-sign-pkcs1-3072-sha256"},
	{v1.PublicKeyDetails_PKIX_RSA_PKCS1V15_4096_SHA256, RSA, crypto.SHA256, RSAKeySize(4096), "rsa-sign-pkcs1-4096-sha256"},
	{v1.PublicKeyDetails_PKIX_RSA_PSS_2048_SHA256, RSA, crypto.SHA256, RSAKeySize(2048), "rsa-sign-pss-2048-sha256"},
	{v1.PublicKeyDetails_PKIX_RSA_PSS_3072_SHA256, RSA, crypto.SHA256, RSAKeySize(3072), "rsa-sign-pss-3072-sha256"},
	{v1.PublicKeyDetails_PKIX_RSA_PSS_4096_SHA256, RSA, crypto.SHA256, RSAKeySize(4096), "rsa-sign-pss-4092-sha256"},
	{v1.PublicKeyDetails_PKIX_ECDSA_P256_SHA_256, ECDSA, crypto.SHA256, elliptic.P256(), "ecdsa-sha2-256-nistp256"},
	{v1.PublicKeyDetails_PKIX_ECDSA_P384_SHA_384, ECDSA, crypto.SHA384, elliptic.P384(), "ecdsa-sha2-384-nistp384"},
	{v1.PublicKeyDetails_PKIX_ECDSA_P521_SHA_512, ECDSA, crypto.SHA512, elliptic.P521(), "ecdsa-sha2-512-nistp521"},
	{v1.PublicKeyDetails_PKIX_ED25519, ED25519, crypto.Hash(0), nil, "ed25519"},
	{v1.PublicKeyDetails_PKIX_ED25519_PH, ED25519, crypto.SHA512, nil, "ed25519-ph"},
}

// AlgorithmRegistryConfig represents a set of permitted algorithms for a given Sigstore service or component.
//
// Individual services may wish to restrict what algorithms are allowed to a subset of what is covered in the algorithm
// registry (represented by v1.PublicKeyDetails).
type AlgorithmRegistryConfig struct {
	permittedAlgorithms []algorithmDetails
}

// getAlgorithmDetails retrieves a set of details for a given v1.PublicKeyDetails flag that allows users to
// introspect the public key algorithm, hash algorithm and more.
func getAlgorithmDetails(knownSignatureAlgorithm v1.PublicKeyDetails) (*algorithmDetails, error) {
	for _, detail := range supportedAlgorithms {
		if detail.knownAlgorithm == knownSignatureAlgorithm {
			return &detail, nil
		}
	}
	return nil, fmt.Errorf("could not find algorithm details for known signature algorithm: %s", knownSignatureAlgorithm)
}

// NewAlgorithmRegistryConfig creates a new AlgorithmRegistryConfig for a set of permitted signature algorithms.
func NewAlgorithmRegistryConfig(algorithmConfig []v1.PublicKeyDetails) (*AlgorithmRegistryConfig, error) {
	permittedAlgorithms := make([]algorithmDetails, 0, len(supportedAlgorithms))
	for _, algorithm := range algorithmConfig {
		a, err := getAlgorithmDetails(algorithm)
		if err != nil {
			return nil, err
		}
		permittedAlgorithms = append(permittedAlgorithms, *a)
	}
	return &AlgorithmRegistryConfig{permittedAlgorithms: permittedAlgorithms}, nil
}

// IsAlgorithmPermitted checks whether a given public key/hash algorithm combination is permitted by a registry config.
func (registryConfig AlgorithmRegistryConfig) IsAlgorithmPermitted(key crypto.PublicKey, hash crypto.Hash) (bool, error) {
	for _, algorithm := range registryConfig.permittedAlgorithms {
		keyMatch, err := algorithm.checkKey(key)
		if err != nil {
			return false, err
		}
		if keyMatch && algorithm.checkHash(hash) {
			return true, nil
		}
	}
	return false, nil
}

// FormatSignatureAlgorithmFlag formats a v1.PublicKeyDetails to a string that conforms to the naming conventions
// of CLI arguments that are used for Sigstore services.
func FormatSignatureAlgorithmFlag(algorithm v1.PublicKeyDetails) (string, error) {
	for _, a := range supportedAlgorithms {
		if a.knownAlgorithm == algorithm {
			return a.flagValue, nil
		}
	}
	return "", fmt.Errorf("could not find matching flag for signature algorithm: %s", algorithm)
}

// ParseSignatureAlgorithmFlag parses a string produced by FormatSignatureAlgorithmFlag and returns the corresponding
// v1.PublicKeyDetails value.
func ParseSignatureAlgorithmFlag(flag string) (v1.PublicKeyDetails, error) {
	for _, a := range supportedAlgorithms {
		if a.flagValue == flag {
			return a.knownAlgorithm, nil
		}
	}
	return v1.PublicKeyDetails_PUBLIC_KEY_DETAILS_UNSPECIFIED, fmt.Errorf("could not find matching signature algorithm for flag: %s", flag)
}
