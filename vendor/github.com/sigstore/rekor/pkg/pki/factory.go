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

package pki

import (
	"fmt"
	"io"

	"github.com/sigstore/rekor/pkg/pki/minisign"
	"github.com/sigstore/rekor/pkg/pki/pgp"
	"github.com/sigstore/rekor/pkg/pki/pkcs7"
	"github.com/sigstore/rekor/pkg/pki/ssh"
	"github.com/sigstore/rekor/pkg/pki/x509"
)

type Format string

const (
	PGP      Format = "pgp"
	Minisign Format = "minisign"
	SSH      Format = "ssh"
	X509     Format = "x509"
	PKCS7    Format = "pkcs7"
)

type ArtifactFactory struct {
	impl pkiImpl
}

func NewArtifactFactory(format Format) (*ArtifactFactory, error) {
	if impl, ok := artifactFactoryMap[format]; ok {
		return &ArtifactFactory{impl: impl}, nil
	}
	return nil, fmt.Errorf("%v is not a supported PKI format", format)
}

type pkiImpl struct {
	newPubKey    func(io.Reader) (PublicKey, error)
	newSignature func(io.Reader) (Signature, error)
}

var artifactFactoryMap map[Format]pkiImpl

func init() {
	artifactFactoryMap = map[Format]pkiImpl{
		PGP: {
			newPubKey: func(r io.Reader) (PublicKey, error) {
				return pgp.NewPublicKey(r)
			},
			newSignature: func(r io.Reader) (Signature, error) {
				return pgp.NewSignature(r)
			},
		},
		Minisign: {
			newPubKey: func(r io.Reader) (PublicKey, error) {
				return minisign.NewPublicKey(r)
			},
			newSignature: func(r io.Reader) (Signature, error) {
				return minisign.NewSignature(r)
			},
		},
		SSH: {
			newPubKey: func(r io.Reader) (PublicKey, error) {
				return ssh.NewPublicKey(r)
			},
			newSignature: func(r io.Reader) (Signature, error) {
				return ssh.NewSignature(r)
			},
		},
		X509: {
			newPubKey: func(r io.Reader) (PublicKey, error) {
				return x509.NewPublicKey(r)
			},
			newSignature: func(r io.Reader) (Signature, error) {
				return x509.NewSignature(r)
			},
		},
		PKCS7: {
			newPubKey: func(r io.Reader) (PublicKey, error) {
				return pkcs7.NewPublicKey(r)
			},
			newSignature: func(r io.Reader) (Signature, error) {
				return pkcs7.NewSignature(r)
			},
		},
	}
}

func SupportedFormats() []string {
	var formats []string
	for f := range artifactFactoryMap {
		formats = append(formats, string(f))
	}
	return formats
}

func (a ArtifactFactory) NewPublicKey(r io.Reader) (PublicKey, error) {
	return a.impl.newPubKey(r)
}

func (a ArtifactFactory) NewSignature(r io.Reader) (Signature, error) {
	return a.impl.newSignature(r)
}
