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

package pgp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-playground/validator"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

// Signature Signature that follows the PGP standard; supports both armored & binary detached signatures
type Signature struct {
	isArmored bool
	signature []byte
}

// NewSignature creates and validates a PGP signature object
func NewSignature(r io.Reader) (*Signature, error) {
	var s Signature
	var inputBuffer bytes.Buffer

	if _, err := io.Copy(&inputBuffer, r); err != nil {
		return nil, fmt.Errorf("unable to read PGP signature: %w", err)
	}

	sigByteReader := bytes.NewReader(inputBuffer.Bytes())

	var sigReader io.Reader
	sigBlock, err := armor.Decode(sigByteReader)
	if err == nil {
		s.isArmored = true
		if sigBlock.Type != openpgp.SignatureType {
			return nil, fmt.Errorf("invalid PGP signature provided")
		}
		sigReader = sigBlock.Body
	} else {
		s.isArmored = false
		if _, err := sigByteReader.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("unable to read binary PGP signature: %w", err)
		}
		sigReader = sigByteReader
	}

	sigPktReader := packet.NewReader(sigReader)
	sigPkt, err := sigPktReader.Next()
	if err != nil {
		return nil, fmt.Errorf("invalid PGP signature: %w", err)
	}

	if _, ok := sigPkt.(*packet.Signature); !ok {
		if _, ok := sigPkt.(*packet.SignatureV3); !ok {
			return nil, fmt.Errorf("valid PGP signature was not detected")
		}
	}

	s.signature = inputBuffer.Bytes()
	return &s, nil
}

// FetchSignature implements pki.Signature interface
func FetchSignature(ctx context.Context, url string) (*Signature, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error initializing fetch for PGP signature: %w", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching PGP signature: %w", err)
	}
	defer resp.Body.Close()

	sig, err := NewSignature(resp.Body)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

// CanonicalValue implements the pki.Signature interface
func (s Signature) CanonicalValue() ([]byte, error) {
	if len(s.signature) == 0 {
		return nil, fmt.Errorf("PGP signature has not been initialized")
	}

	if s.isArmored {
		return s.signature, nil
	}

	var canonicalBuffer bytes.Buffer
	// Use an inner function so we can defer the Close()
	if err := func() error {
		ew, err := armor.Encode(&canonicalBuffer, openpgp.SignatureType, nil)
		if err != nil {
			return fmt.Errorf("error encoding canonical value of PGP signature: %w", err)
		}
		defer ew.Close()

		if _, err := io.Copy(ew, bytes.NewReader(s.signature)); err != nil {
			return fmt.Errorf("error generating canonical value of PGP signature: %w", err)
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	return canonicalBuffer.Bytes(), nil
}

// Verify implements the pki.Signature interface
func (s Signature) Verify(r io.Reader, k interface{}) error {
	if len(s.signature) == 0 {
		return fmt.Errorf("PGP signature has not been initialized")
	}

	key, ok := k.(*PublicKey)
	if !ok {
		return fmt.Errorf("cannot use Verify with a non-PGP signature")
	}
	if len(key.key) == 0 {
		return fmt.Errorf("PGP public key has not been initialized")
	}

	verifyFn := openpgp.CheckDetachedSignature
	if s.isArmored {
		verifyFn = openpgp.CheckArmoredDetachedSignature
	}

	if _, err := verifyFn(key.key, r, bytes.NewReader(s.signature)); err != nil {
		return err
	}

	return nil
}

// PublicKey Public Key that follows the PGP standard; supports both armored & binary detached signatures
type PublicKey struct {
	key openpgp.EntityList
}

// NewPublicKey implements the pki.PublicKey interface
func NewPublicKey(r io.Reader) (*PublicKey, error) {
	var k PublicKey
	var inputBuffer bytes.Buffer

	startToken := []byte(`-----BEGIN PGP`)
	endToken := []byte(`-----END PGP`)

	bufferedReader := bufio.NewReader(r)
	armorCheck, err := bufferedReader.Peek(len(startToken))
	if err != nil {
		return nil, fmt.Errorf("unable to read PGP public key: %w", err)
	}
	if bytes.Equal(startToken, armorCheck) {
		// looks like we have armored input
		scan := bufio.NewScanner(bufferedReader)
		scan.Split(bufio.ScanLines)

		for scan.Scan() {
			line := scan.Bytes()
			inputBuffer.Write(line)
			fmt.Fprintf(&inputBuffer, "\n")

			if bytes.HasPrefix(line, endToken) {
				// we have a complete armored message; process it
				keyBlock, err := armor.Decode(&inputBuffer)
				if err == nil {
					if keyBlock.Type != openpgp.PublicKeyType && keyBlock.Type != openpgp.PrivateKeyType {
						return nil, fmt.Errorf("invalid PGP type detected")
					}
					keys, err := openpgp.ReadKeyRing(keyBlock.Body)
					if err != nil {
						return nil, fmt.Errorf("error reading PGP public key: %w", err)
					}
					if k.key == nil {
						k.key = keys
					} else {
						k.key = append(k.key, keys...)
					}
					inputBuffer.Reset()
				} else {
					return nil, fmt.Errorf("invalid PGP public key provided: %w", err)
				}
			}
		}
	} else {
		// process as binary
		k.key, err = openpgp.ReadKeyRing(bufferedReader)
		if err != nil {
			return nil, fmt.Errorf("error reading binary PGP public key: %w", err)
		}
	}

	if len(k.key) == len(k.key.DecryptionKeys()) {
		return nil, fmt.Errorf("no PGP public keys could be read")
	}

	return &k, nil
}

// FetchPublicKey implements pki.PublicKey interface
func FetchPublicKey(ctx context.Context, url string) (*PublicKey, error) {
	//TODO: detect if url is hkp and adjust accordingly
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching PGP public key: %w", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching PGP public key: %w", err)
	}
	defer resp.Body.Close()

	key, err := NewPublicKey(resp.Body)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// CanonicalValue implements the pki.PublicKey interface
func (k PublicKey) CanonicalValue() ([]byte, error) {
	if k.key == nil {
		return nil, fmt.Errorf("PGP public key has not been initialized")
	}

	var canonicalBuffer bytes.Buffer

	// Use an inner function so we can defer the close()
	if err := func() error {
		armoredWriter, err := armor.Encode(&canonicalBuffer, openpgp.PublicKeyType, nil)
		if err != nil {
			return fmt.Errorf("error generating canonical value of PGP public key: %w", err)
		}
		defer armoredWriter.Close()

		for _, entity := range k.key {
			if err := entity.Serialize(armoredWriter); err != nil {
				return fmt.Errorf("error generating canonical value of PGP public key: %w", err)
			}
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	return canonicalBuffer.Bytes(), nil
}

func (k PublicKey) KeyRing() (openpgp.KeyRing, error) {
	if k.key == nil {
		return nil, errors.New("PGP public key has not been initialized")
	}

	return k.key, nil
}

// EmailAddresses implements the pki.PublicKey interface
func (k PublicKey) EmailAddresses() []string {
	var names []string
	// Extract from cert
	for _, entity := range k.key {
		for _, identity := range entity.Identities {
			validate := validator.New()
			errs := validate.Var(identity.UserId.Email, "required,email")
			if errs == nil {
				names = append(names, identity.UserId.Email)
			}
		}
	}
	return names
}
