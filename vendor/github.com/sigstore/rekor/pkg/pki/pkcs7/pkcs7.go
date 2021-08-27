/*
Copyright © 2021 The Sigstore Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkcs7

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/sassoftware/relic/lib/pkcs7"
)

// EmailAddressOID defined by https://oidref.com/1.2.840.113549.1.9.1
var EmailAddressOID asn1.ObjectIdentifier = []int{1, 2, 840, 113549, 1, 9, 1}

type Signature struct {
	signedData pkcs7.SignedData
	detached   bool
	raw        *[]byte
}

// NewSignature creates and validates an PKCS7 signature object
func NewSignature(r io.Reader) (*Signature, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	// try PEM decoding first
	var pkcsBytes *[]byte
	block, _ := pem.Decode(b)
	if block != nil {
		if block.Type != "PKCS7" {
			return nil, fmt.Errorf("unknown PEM block type %s found during PKCS7 parsing", block.Type)
		}
		pkcsBytes = &block.Bytes
	} else {
		// PEM decoding failed, it might just be raw ASN.1 data
		pkcsBytes = &b
	}

	psd, err := pkcs7.Unmarshal(*pkcsBytes)
	if err != nil {
		return nil, err
	}

	// we store the detached signature as the raw, canonical format
	if _, err := psd.Detach(); err != nil {
		return nil, err
	}

	detached, err := psd.Marshal()
	if err != nil {
		return nil, err
	}

	cb, err := psd.Content.ContentInfo.Bytes()
	if err != nil {
		return nil, err
	}

	return &Signature{
		signedData: psd.Content,
		raw:        &detached,
		detached:   cb == nil,
	}, nil
}

// CanonicalValue implements the pki.Signature interface
func (s Signature) CanonicalValue() ([]byte, error) {
	if s.raw == nil {
		return nil, fmt.Errorf("PKCS7 signature has not been initialized")
	}

	p := pem.Block{
		Type:  "PKCS7",
		Bytes: *s.raw,
	}

	var buf bytes.Buffer
	if err := pem.Encode(&buf, &p); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Verify implements the pki.Signature interface
func (s Signature) Verify(r io.Reader, k interface{}) error {
	if len(*s.raw) == 0 {
		return fmt.Errorf("PKCS7 signature has not been initialized")
	}

	// if content was passed to this, verify signature as if it were detached
	bb := bytes.Buffer{}
	var extContent []byte
	if r != nil {
		n, err := io.Copy(&bb, r)
		if err != nil {
			return err
		}
		if n > 0 {
			extContent = bb.Bytes()
		} else if s.detached {
			return errors.New("PKCS7 signature is detached and there is no external content to verify against")
		}
	}

	if _, err := s.signedData.Verify(extContent, false); err != nil {
		return err
	}

	return nil
}

// PublicKey Public Key contained in cert inside PKCS7 bundle
type PublicKey struct {
	key     crypto.PublicKey
	certs   []*x509.Certificate
	rawCert []byte
}

// NewPublicKey implements the pki.PublicKey interface
func NewPublicKey(r io.Reader) (*PublicKey, error) {
	rawPub, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// try PEM decoding first
	var pkcsBytes *[]byte
	block, _ := pem.Decode(rawPub)
	if block != nil {
		if block.Type != "PKCS7" {
			return nil, fmt.Errorf("unknown PEM block type %s found during PKCS7 parsing", block.Type)
		}
		pkcsBytes = &block.Bytes
	} else {
		// PEM decoding failed, it might just be raw ASN.1 data
		pkcsBytes = &rawPub
	}
	pkcs7, err := pkcs7.Unmarshal(*pkcsBytes)
	if err != nil {
		return nil, err
	}
	certs, err := pkcs7.Content.Certificates.Parse()
	if err != nil {
		return nil, err
	}
	for _, cert := range certs {
		return &PublicKey{key: cert.PublicKey, certs: certs, rawCert: cert.Raw}, nil
	}
	return nil, errors.New("unable to extract public key from certificate inside PKCS7 bundle")
}

// CanonicalValue implements the pki.PublicKey interface
func (k PublicKey) CanonicalValue() ([]byte, error) {
	if k.rawCert == nil {
		return nil, fmt.Errorf("PKCS7 public key has not been initialized")
	}
	//TODO: should we export the entire cert chain, not just the first one?
	p := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: k.rawCert,
	}

	var buf bytes.Buffer
	if err := pem.Encode(&buf, &p); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EmailAddresses implements the pki.PublicKey interface
func (k PublicKey) EmailAddresses() []string {
	var names []string
	// Get email address from Subject name in raw cert.
	cert, err := x509.ParseCertificate(k.rawCert)
	if err != nil {
		// This should not happen from a valid PublicKey, but fail gracefully.
		return names
	}

	for _, name := range cert.Subject.Names {
		if name.Type.Equal(EmailAddressOID) {
			names = append(names, strings.ToLower(name.Value.(string)))
		}
	}

	return names
}
