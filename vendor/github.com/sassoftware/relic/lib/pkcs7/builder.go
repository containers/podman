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
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"

	"github.com/sassoftware/relic/lib/x509tools"
)

type SignatureBuilder struct {
	contentInfo ContentInfo
	hash        crypto.Hash
	digest      []byte
	certs       []*x509.Certificate
	privateKey  crypto.Signer
	signerOpts  crypto.SignerOpts
	authAttrs   AttributeList
}

// Build a PKCS#7 signature procedurally. Returns a structure that can have
// content and attributes attached to it.
func NewBuilder(privKey crypto.Signer, certs []*x509.Certificate, opts crypto.SignerOpts) *SignatureBuilder {
	return &SignatureBuilder{
		privateKey: privKey,
		signerOpts: opts,
		certs:      certs,
	}
}

// Embed bytes or a structure into the PKCS#7 content
func (sb *SignatureBuilder) SetContent(ctype asn1.ObjectIdentifier, data interface{}) error {
	cinfo, err := NewContentInfo(ctype, data)
	if err != nil {
		return err
	}
	return sb.SetContentInfo(cinfo)
}

// Set content to a generic "data" blob
func (sb *SignatureBuilder) SetContentData(data []byte) error {
	return sb.SetContent(OidData, data)
}

// Set a ContentInfo structure as the PKCS#7 content
func (sb *SignatureBuilder) SetContentInfo(cinfo ContentInfo) error {
	blob, err := cinfo.Bytes()
	if err != nil {
		return err
	}
	d := sb.signerOpts.HashFunc().New()
	d.Write(blob)
	sb.contentInfo = cinfo
	sb.digest = d.Sum(nil)
	return nil
}

// Set a "detached" content type, with digest
func (sb *SignatureBuilder) SetDetachedContent(ctype asn1.ObjectIdentifier, digest []byte) error {
	if len(digest) != sb.signerOpts.HashFunc().Size() {
		return errors.New("digest size mismatch")
	}
	cinfo, _ := NewContentInfo(ctype, nil)
	sb.contentInfo = cinfo
	sb.digest = digest
	return nil
}

// Add an authenticated attribute to SignerInfo
func (sb *SignatureBuilder) AddAuthenticatedAttribute(oid asn1.ObjectIdentifier, data interface{}) error {
	return sb.authAttrs.Add(oid, data)
}

// Complete the signature and return the full PKCS#7 structure
func (sb *SignatureBuilder) Sign() (*ContentInfoSignedData, error) {
	if sb.digest == nil {
		return nil, errors.New("SetContent was not called")
	}
	pubKey := sb.privateKey.Public()
	digestAlg, pkeyAlg, err := x509tools.PkixAlgorithms(pubKey, sb.signerOpts)
	if err != nil {
		return nil, fmt.Errorf("pkcs7: %w", err)
	}
	if len(sb.certs) < 1 || !x509tools.SameKey(pubKey, sb.certs[0].PublicKey) {
		return nil, errors.New("pkcs7: first certificate must match private key")
	}
	digest := sb.digest
	if sb.authAttrs != nil {
		// When authenticated attributes are present, then these are required.
		if err := sb.authAttrs.Add(OidAttributeContentType, sb.contentInfo.ContentType); err != nil {
			return nil, err
		}
		if err := sb.authAttrs.Add(OidAttributeMessageDigest, sb.digest); err != nil {
			return nil, err
		}
		// Now the signature is over the authenticated attributes instead of
		// the content directly.
		attrbytes, err := sb.authAttrs.Bytes()
		if err != nil {
			return nil, err
		}
		w := sb.signerOpts.HashFunc().New()
		w.Write(attrbytes)
		digest = w.Sum(nil)
	}
	sig, err := sb.privateKey.Sign(rand.Reader, digest, sb.signerOpts)
	if err != nil {
		return nil, err
	}
	return &ContentInfoSignedData{
		ContentType: OidSignedData,
		Content: SignedData{
			Version:                    1,
			DigestAlgorithmIdentifiers: []pkix.AlgorithmIdentifier{digestAlg},
			ContentInfo:                sb.contentInfo,
			Certificates:               marshalCertificates(sb.certs),
			CRLs:                       nil,
			SignerInfos: []SignerInfo{SignerInfo{
				Version: 1,
				IssuerAndSerialNumber: IssuerAndSerial{
					IssuerName:   asn1.RawValue{FullBytes: sb.certs[0].RawIssuer},
					SerialNumber: sb.certs[0].SerialNumber,
				},
				DigestAlgorithm:           digestAlg,
				DigestEncryptionAlgorithm: pkeyAlg,
				AuthenticatedAttributes:   sb.authAttrs,
				EncryptedDigest:           sig,
			}},
		},
	}, nil
}
