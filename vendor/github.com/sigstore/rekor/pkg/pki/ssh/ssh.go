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

package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/asaskevich/govalidator"
	sigsig "github.com/sigstore/sigstore/pkg/signature"
	"golang.org/x/crypto/ssh"
)

type Signature struct {
	signature *ssh.Signature
	pk        ssh.PublicKey
	hashAlg   string
}

// NewSignature creates and Validates an ssh signature object
func NewSignature(r io.Reader) (*Signature, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	sig, err := Decode(b)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

// CanonicalValue implements the pki.Signature interface
func (s Signature) CanonicalValue() ([]byte, error) {
	return []byte(Armor(s.signature, s.pk)), nil
}

// Verify implements the pki.Signature interface
func (s Signature) Verify(r io.Reader, k interface{}, _ ...sigsig.VerifyOption) error {
	if s.signature == nil {
		return fmt.Errorf("ssh signature has not been initialized")
	}

	key, ok := k.(*PublicKey)
	if !ok {
		return fmt.Errorf("invalid public key type for: %v", k)
	}

	ck, err := key.CanonicalValue()
	if err != nil {
		return err
	}
	cs, err := s.CanonicalValue()
	if err != nil {
		return err
	}
	return Verify(r, cs, ck)
}

// PublicKey contains an ssh PublicKey
type PublicKey struct {
	key     ssh.PublicKey
	comment string
}

// NewPublicKey implements the pki.PublicKey interface
func NewPublicKey(r io.Reader) (*PublicKey, error) {
	// 64K seems generous as a limit for valid SSH keys
	// we use http.MaxBytesReader and pass nil for ResponseWriter to reuse stdlib
	// and not reimplement this; There is a proposal for this to be fixed in 1.20
	// https://github.com/golang/go/issues/51115
	// TODO: switch this to stdlib once golang 1.20 comes out
	rawPub, err := io.ReadAll(http.MaxBytesReader(nil, io.NopCloser(r), 65536))
	if err != nil {
		return nil, err
	}

	key, comment, _, _, err := ssh.ParseAuthorizedKey(rawPub)
	if err != nil {
		return nil, err
	}

	return &PublicKey{key: key, comment: comment}, nil
}

// CanonicalValue implements the pki.PublicKey interface
func (k PublicKey) CanonicalValue() ([]byte, error) {
	if k.key == nil {
		return nil, fmt.Errorf("ssh public key has not been initialized")
	}
	return ssh.MarshalAuthorizedKey(k.key), nil
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
	var identities []string

	// an authorized key format
	authorizedKey := string(bytes.TrimSpace(ssh.MarshalAuthorizedKey(k.key)))
	identities = append(identities, authorizedKey)

	if govalidator.IsEmail(k.comment) {
		identities = append(identities, k.comment)
	}

	return identities, nil
}
