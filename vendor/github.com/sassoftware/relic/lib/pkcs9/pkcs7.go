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

package pkcs9

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/sassoftware/relic/lib/pkcs7"
	"github.com/sassoftware/relic/lib/x509tools"
)

func TimestampAndMarshal(ctx context.Context, psd *pkcs7.ContentInfoSignedData, timestamper Timestamper, authenticode bool) (*TimestampedSignature, error) {
	if timestamper != nil {
		signerInfo := &psd.Content.SignerInfos[0]
		hash, err := x509tools.PkixDigestToHashE(signerInfo.DigestAlgorithm)
		if err != nil {
			return nil, err
		}
		token, err := timestamper.Timestamp(ctx, &Request{EncryptedDigest: signerInfo.EncryptedDigest, Hash: hash})
		if err != nil {
			return nil, err
		}
		if authenticode {
			err = AddStampToSignedAuthenticode(signerInfo, *token)
		} else {
			err = AddStampToSignedData(signerInfo, *token)
		}
		if err != nil {
			return nil, err
		}
	}
	verified, err := psd.Content.Verify(nil, false)
	if err != nil {
		return nil, fmt.Errorf("pkcs7: failed signature self-check: %w", err)
	}
	ts, err := VerifyOptionalTimestamp(verified)
	if err != nil {
		return nil, fmt.Errorf("pkcs7: failed signature self-check: %w", err)
	}
	blob, err := psd.Marshal()
	if err != nil {
		return nil, err
	}
	ts.Raw = blob
	return &ts, err
}

// Attach a RFC 3161 timestamp to a PKCS#7 SignerInfo
func AddStampToSignedData(signerInfo *pkcs7.SignerInfo, token pkcs7.ContentInfoSignedData) error {
	return signerInfo.UnauthenticatedAttributes.Add(OidAttributeTimeStampToken, token)
}

// Attach a RFC 3161 timestamp to a PKCS#7 SignerInfo using the OID for authenticode signatures
func AddStampToSignedAuthenticode(signerInfo *pkcs7.SignerInfo, token pkcs7.ContentInfoSignedData) error {
	return signerInfo.UnauthenticatedAttributes.Add(OidSpcTimeStampToken, token)
}

// Validated timestamp token
type CounterSignature struct {
	pkcs7.Signature
	Hash        crypto.Hash
	SigningTime time.Time
}

// Validated signature containing a optional timestamp token
type TimestampedSignature struct {
	pkcs7.Signature
	CounterSignature *CounterSignature
	Raw              []byte
}

// Look for a timestamp (counter-signature or timestamp token) in the
// UnauthenticatedAttributes of the given already-validated signature and check
// its integrity. The certificate chain is not checked; call VerifyChain() on
// the result to validate it fully. Returns nil if no timestamp is present.
func VerifyPkcs7(sig pkcs7.Signature) (*CounterSignature, error) {
	var tst pkcs7.ContentInfoSignedData
	// check several OIDs for timestamp tokens
	err := sig.SignerInfo.UnauthenticatedAttributes.GetOne(OidAttributeTimeStampToken, &tst)
	if _, ok := err.(pkcs7.ErrNoAttribute); ok {
		err = sig.SignerInfo.UnauthenticatedAttributes.GetOne(OidSpcTimeStampToken, &tst)
	}
	var imprintHash crypto.Hash
	if err == nil {
		// timestamptoken is a fully nested signedData containing a TSTInfo
		// that digests the parent signature blob
		return Verify(&tst, sig.SignerInfo.EncryptedDigest, sig.Intermediates)
	} else if _, ok := err.(pkcs7.ErrNoAttribute); ok {
		tsi := new(pkcs7.SignerInfo)
		if err := sig.SignerInfo.UnauthenticatedAttributes.GetOne(OidAttributeCounterSign, tsi); err != nil {
			if _, ok := err.(pkcs7.ErrNoAttribute); ok {
				return nil, nil
			}
			return nil, err
		}
		// counterSignature is simply a signerinfo. The certificate chain is
		// included in the parent structure, and the timestamp signs the
		// signature blob from the parent signerinfo
		imprintHash, _ = x509tools.PkixDigestToHash(sig.SignerInfo.DigestAlgorithm)
		return finishVerify(tsi, sig.SignerInfo.EncryptedDigest, sig.Intermediates, imprintHash, tsi, nil)
	}
	return nil, err
}

// Look for a timestamp token or counter-signature in the given signature and
// return a structure that can be used to validate the signature's certificate
// chain. If no timestamp is present, then the current time will be used when
// validating the chain.
func VerifyOptionalTimestamp(sig pkcs7.Signature) (TimestampedSignature, error) {
	tsig := TimestampedSignature{Signature: sig}
	ts, err := VerifyPkcs7(sig)
	if err != nil {
		return tsig, err
	}
	tsig.CounterSignature = ts
	return tsig, nil
}

// Verify that the timestamp token has a valid certificate chain
func (cs CounterSignature) VerifyChain(roots *x509.CertPool, extraCerts []*x509.Certificate) error {
	return cs.Signature.VerifyChain(roots, extraCerts, x509.ExtKeyUsageTimeStamping, cs.SigningTime)
}

// Verify the certificate chain of a PKCS#7 signature. If the signature has a
// valid timestamp token attached, then the timestamp is used for validating
// the primary signature's chain, making the signature valid after the
// certificates have expired.
func (sig TimestampedSignature) VerifyChain(roots *x509.CertPool, extraCerts []*x509.Certificate, usage x509.ExtKeyUsage) error {
	var signingTime time.Time
	if sig.CounterSignature != nil {
		if err := sig.CounterSignature.VerifyChain(roots, extraCerts); err != nil {
			return fmt.Errorf("validating timestamp: %w", err)
		}
		signingTime = sig.CounterSignature.SigningTime
	}
	return sig.Signature.VerifyChain(roots, extraCerts, usage, signingTime)
}

// Verify a non-RFC-3161 timestamp token against the given encrypted digest
// from the primary signature.
func VerifyMicrosoftToken(token *pkcs7.ContentInfoSignedData, encryptedDigest []byte) (*CounterSignature, error) {
	sig, err := token.Content.Verify(nil, false)
	if err != nil {
		return nil, err
	}
	content, err := token.Content.ContentInfo.Bytes()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(content, encryptedDigest) {
		return nil, errors.New("timestamp does not match the enclosing signature")
	}
	hash, _ := x509tools.PkixDigestToHash(sig.SignerInfo.DigestAlgorithm)
	signingTime, err := sig.SignerInfo.SigningTime()
	if err != nil {
		return nil, err
	}
	return &CounterSignature{
		Signature:   sig,
		Hash:        hash,
		SigningTime: signingTime,
	}, nil
}
