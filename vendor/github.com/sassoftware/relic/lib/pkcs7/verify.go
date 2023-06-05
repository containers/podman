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
	"bytes"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sassoftware/relic/lib/x509tools"
)

type Signature struct {
	SignerInfo    *SignerInfo
	Certificate   *x509.Certificate
	Intermediates []*x509.Certificate
}

// Verify the content in a SignedData structure. External content may be
// provided if it is a detached signature. Information about the signature is
// returned, however X509 chains are not validated. Call VerifyChain() to
// complete the verification process.
//
// If skipDigests is true, then the main content section is not checked, but
// the SignerInfos are still checked for a valid signature.
func (sd *SignedData) Verify(externalContent []byte, skipDigests bool) (Signature, error) {
	var content []byte
	if !skipDigests {
		var err error
		content, err = sd.ContentInfo.Bytes()
		if err != nil {
			return Signature{}, err
		} else if content == nil {
			if externalContent == nil {
				return Signature{}, errors.New("pkcs7: missing content")
			}
			content = externalContent
		} else if externalContent != nil {
			if !bytes.Equal(externalContent, content) {
				return Signature{}, errors.New("pkcs7: internal and external content were both provided but are not equal")
			}
		}
	}
	certs, err := sd.Certificates.Parse()
	if err != nil {
		return Signature{}, fmt.Errorf("pkcs7: %s", err)
	} else if len(certs) == 0 {
		return Signature{}, errors.New("pkcs7: certificate missing from signedData")
	}
	var cert *x509.Certificate
	var sig Signature
	for _, si := range sd.SignerInfos {
		cert, err = si.Verify(content, skipDigests, certs)
		if err != nil {
			return Signature{}, err
		}
		sig = Signature{&si, cert, certs}
	}
	return sig, nil
}

// Find the certificate that signed this SignerInfo from the bucket of certs
func (si *SignerInfo) FindCertificate(certs []*x509.Certificate) (*x509.Certificate, error) {
	is := si.IssuerAndSerialNumber
	for _, cert := range certs {
		if bytes.Equal(cert.RawIssuer, is.IssuerName.FullBytes) && cert.SerialNumber.Cmp(is.SerialNumber) == 0 {
			return cert, nil
		}
	}
	return nil, errors.New("pkcs7: certificate missing from signedData")
}

// Verify the signature contained in this SignerInfo and return the leaf
// certificate. X509 chains are not validated.
func (si *SignerInfo) Verify(content []byte, skipDigests bool, certs []*x509.Certificate) (*x509.Certificate, error) {
	hash, err := x509tools.PkixDigestToHashE(si.DigestAlgorithm)
	if err != nil {
		return nil, errors.Wrap(err, "pkcs7")
	}
	var digest []byte
	if !skipDigests {
		w := hash.New()
		w.Write(content)
		digest = w.Sum(nil)
	}
	if len(si.AuthenticatedAttributes) != 0 {
		// check the content digest against the messageDigest attribute
		var md []byte
		if err := si.AuthenticatedAttributes.GetOne(OidAttributeMessageDigest, &md); err != nil {
			return nil, err
		} else if digest != nil && !hmac.Equal(md, digest) {
			return nil, errors.New("pkcs7: content digest does not match")
		}
		// now pivot to verifying the hash over the authenticated attributes
		w := hash.New()
		attrbytes, err := si.AuthenticatedAttributes.Bytes()
		if err != nil {
			return nil, err
		}
		w.Write(attrbytes)
		digest = w.Sum(nil)
	} // otherwise the content hash is verified directly
	cert, err := si.FindCertificate(certs)
	if err != nil {
		return nil, err
	}
	// If skipDigests is set and AuthenticatedAttributes is not present then
	// there's no digest to check the signature against. For RSA at least it's
	// possible to decrypt the signature and check the padding but there's not
	// much point to it.
	if digest != nil {
		err = x509tools.PkixVerify(cert.PublicKey, si.DigestAlgorithm, si.DigestEncryptionAlgorithm, digest, si.EncryptedDigest)
		if err == rsa.ErrVerification {
			// "Symantec Time Stamping Services Signer" seems to be emitting
			// signatures without the AlgorithmIdentifier strucuture, so try
			// without it.
			err = x509tools.Verify(cert.PublicKey, 0, digest, si.EncryptedDigest)
		}
	}
	return cert, err
}

// Verify the X509 chain from a signature against the given roots. extraCerts
// will be added to the intermediates if provided. usage gives the certificate
// usage required for the leaf certificate, or ExtKeyUsageAny otherwise. If a
// PKCS#9 trusted timestamp was found, pass that timestamp in currentTime to
// validate the chain as of the time of the signature.
func (info Signature) VerifyChain(roots *x509.CertPool, extraCerts []*x509.Certificate, usage x509.ExtKeyUsage, currentTime time.Time) error {
	pool := x509.NewCertPool()
	for _, cert := range extraCerts {
		pool.AddCert(cert)
	}
	for _, cert := range info.Intermediates {
		pool.AddCert(cert)
	}
	opts := x509.VerifyOptions{
		Intermediates: pool,
		Roots:         roots,
		CurrentTime:   currentTime,
		KeyUsages:     []x509.ExtKeyUsage{usage},
	}
	_, err := info.Certificate.Verify(opts)
	return err
}
