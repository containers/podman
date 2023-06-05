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

package minisign

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	minisign "github.com/jedisct1/go-minisign"
	sigsig "github.com/sigstore/sigstore/pkg/signature"
	"golang.org/x/crypto/blake2b"
)

// Signature Signature that follows the minisign standard; supports both minisign and signify generated signatures
type Signature struct {
	signature *minisign.Signature
}

// NewSignature creates and validates a minisign signature object
func NewSignature(r io.Reader) (*Signature, error) {
	var s Signature
	var inputBuffer bytes.Buffer

	if _, err := io.Copy(&inputBuffer, r); err != nil {
		return nil, fmt.Errorf("unable to read minisign signature: %w", err)
	}

	inputString := inputBuffer.String()
	signature, err := minisign.DecodeSignature(inputString)
	if err != nil {
		// try to parse as signify
		lines := strings.Split(strings.TrimRight(inputString, "\n"), "\n")
		if len(lines) != 2 {
			return nil, fmt.Errorf("invalid signature provided: %v lines detected", len(lines))
		}
		sigBytes, b64Err := base64.StdEncoding.DecodeString(lines[1])
		if b64Err != nil {
			return nil, fmt.Errorf("invalid signature provided: base64 decoding failed")
		}
		if len(sigBytes) != len(signature.SignatureAlgorithm)+len(signature.KeyId)+len(signature.Signature) {
			return nil, fmt.Errorf("invalid signature provided: incorrect size %v detected", len(sigBytes))
		}
		copy(signature.SignatureAlgorithm[:], sigBytes[0:2])
		copy(signature.KeyId[:], sigBytes[2:10])
		copy(signature.Signature[:], sigBytes[10:])
	}

	s.signature = &signature
	return &s, nil
}

// CanonicalValue implements the pki.Signature interface
func (s Signature) CanonicalValue() ([]byte, error) {
	if s.signature == nil {
		return nil, fmt.Errorf("minisign signature has not been initialized")
	}

	buf := bytes.NewBuffer([]byte("untrusted comment:\n"))
	b64Buf := bytes.NewBuffer(s.signature.SignatureAlgorithm[:])
	if _, err := b64Buf.Write(s.signature.KeyId[:]); err != nil {
		return nil, fmt.Errorf("error canonicalizing minisign signature: %w", err)
	}
	if _, err := b64Buf.Write(s.signature.Signature[:]); err != nil {
		return nil, fmt.Errorf("error canonicalizing minisign signature: %w", err)
	}
	if _, err := buf.WriteString(base64.StdEncoding.EncodeToString(b64Buf.Bytes())); err != nil {
		return nil, fmt.Errorf("error canonicalizing minisign signature: %w", err)
	}
	return buf.Bytes(), nil
}

// Verify implements the pki.Signature interface
func (s Signature) Verify(r io.Reader, k interface{}, opts ...sigsig.VerifyOption) error {
	if s.signature == nil {
		return fmt.Errorf("minisign signature has not been initialized")
	}

	key, ok := k.(*PublicKey)
	if !ok {
		return fmt.Errorf("cannot use Verify with a non-minisign key")
	}
	if key.key == nil {
		return fmt.Errorf("minisign public key has not been initialized")
	}

	verifier, err := sigsig.LoadED25519Verifier(key.key.PublicKey[:])
	if err != nil {
		return err
	}

	prehashed := s.signature.SignatureAlgorithm[1] == 0x44
	if prehashed {
		h, _ := blake2b.New512(nil)
		_, err := io.Copy(h, r)
		if err != nil {
			return fmt.Errorf("reading minisign data")
		}
		r = bytes.NewReader(h.Sum(nil))
	}

	return verifier.VerifySignature(bytes.NewReader(s.signature.Signature[:]), r, opts...)
}

// PublicKey Public Key that follows the minisign standard; supports signify and minisign public keys
type PublicKey struct {
	key *minisign.PublicKey
}

// NewPublicKey implements the pki.PublicKey interface
func NewPublicKey(r io.Reader) (*PublicKey, error) {
	var k PublicKey
	var inputBuffer bytes.Buffer

	if _, err := io.Copy(&inputBuffer, r); err != nil {
		return nil, fmt.Errorf("unable to read minisign public key: %w", err)
	}

	inputString := inputBuffer.String()

	// There are three ways a minisign key can be stored.
	// 1. The entire text key
	// 2. A base64 encoded string
	// 3. A legacy format we stored of just the key material (no key ID or Algorithm) due to bug fixed in https://github.com/sigstore/rekor/pull/562
	key, err := minisign.DecodePublicKey(inputString)
	if err == nil {
		k.key = &key
		return &k, nil
	}
	key, err = minisign.NewPublicKey(inputString)
	if err == nil {
		k.key = &key
		return &k, nil
	}

	if len(inputString) == 32 {
		k.key = &minisign.PublicKey{
			SignatureAlgorithm: [2]byte{'E', 'd'},
			KeyId:              [8]byte{},
		}
		copy(k.key.PublicKey[:], inputBuffer.Bytes())
		return &k, nil
	}
	return nil, fmt.Errorf("unable to read minisign public key: %w", err)
}

// CanonicalValue implements the pki.PublicKey interface
func (k PublicKey) CanonicalValue() ([]byte, error) {
	if k.key == nil {
		return nil, fmt.Errorf("minisign public key has not been initialized")
	}

	bin := []byte{}
	bin = append(bin, k.key.SignatureAlgorithm[:]...)
	bin = append(bin, k.key.KeyId[:]...)
	bin = append(bin, k.key.PublicKey[:]...)
	b64Key := base64.StdEncoding.EncodeToString(bin)
	return []byte(b64Key), nil
}

// EmailAddresses implements the pki.PublicKey interface
func (k PublicKey) EmailAddresses() []string {
	return nil
}

// Subjects implements the pki.PublicKey interface
func (k PublicKey) Subjects() []string {
	return nil
}

// Identities implements the pki.PublicKey interface
func (k PublicKey) Identities() ([]string, error) {
	// returns base64-encoded key (sig alg, key ID, and public key)
	key, err := k.CanonicalValue()
	if err != nil {
		return nil, err
	}
	return []string{string(key)}, nil
}
