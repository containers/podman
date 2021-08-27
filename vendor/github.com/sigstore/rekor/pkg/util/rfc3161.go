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

package util

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"math/big"
	"time"

	"github.com/sassoftware/relic/lib/pkcs7"
	"github.com/sassoftware/relic/lib/pkcs9"
	"github.com/sassoftware/relic/lib/x509tools"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

type GeneralName struct {
	Name asn1.RawValue `asn1:"optional,tag:4"`
}

type IssuerNameAndSerial struct {
	IssuerName   GeneralName
	SerialNumber *big.Int
}

type EssCertIDv2 struct {
	HashAlgorithm       pkix.AlgorithmIdentifier `asn1:"optional"` // SHA256
	CertHash            []byte
	IssuerNameAndSerial IssuerNameAndSerial `asn1:"optional"`
}

type SigningCertificateV2 struct {
	Certs []EssCertIDv2
}

func createSigningCertificate(certificate *x509.Certificate) ([]byte, error) {
	h := sha256.Sum256(certificate.Raw) // TODO: Get from certificate, defaults to 256
	signingCert := SigningCertificateV2{
		Certs: []EssCertIDv2{{
			CertHash: h[:],
			IssuerNameAndSerial: IssuerNameAndSerial{
				IssuerName:   GeneralName{Name: asn1.RawValue{Tag: 4, Class: 2, IsCompound: true, Bytes: certificate.RawIssuer}},
				SerialNumber: certificate.SerialNumber,
			},
		}},
	}
	signingCertBytes, err := asn1.Marshal(signingCert)
	if err != nil {
		return nil, err
	}
	return signingCertBytes, nil
}

func marshalCertificates(certs []*x509.Certificate) pkcs7.RawCertificates {
	c := make(pkcs7.RawCertificates, len(certs))
	for i, cert := range certs {
		c[i] = asn1.RawValue{FullBytes: cert.Raw}
	}
	return c
}

func getPKIXPublicKeyAlgorithm(cert x509.Certificate) (*pkix.AlgorithmIdentifier, error) {
	identifier := pkix.AlgorithmIdentifier{
		Parameters: asn1.NullRawValue,
	}
	switch alg := cert.PublicKeyAlgorithm; alg {
	case x509.RSA:
		identifier.Algorithm = x509tools.OidPublicKeyRSA
	case x509.ECDSA:
		identifier.Algorithm = x509tools.OidPublicKeyECDSA
	case x509.Ed25519:
		identifier.Algorithm = asn1.ObjectIdentifier{1, 3, 101, 112}
	default:
		return nil, fmt.Errorf("unknown public key algorithm")
	}

	return &identifier, nil
}

type TimestampRequestOptions struct {
	// The policy that the client expects the TSA to use for creating the timestamp token.
	// If no policy is specified the TSA uses its default policy.
	TSAPolicyOid asn1.ObjectIdentifier

	// The nonce to specify in the request.
	Nonce *big.Int

	// Hash function to use when constructing the timestamp request. Defaults to SHA-256.
	Hash crypto.Hash
}

func TimestampRequestFromDigest(digest []byte, opts TimestampRequestOptions) (*pkcs9.TimeStampReq, error) {
	alg, _ := x509tools.PkixDigestAlgorithm(opts.Hash)
	msg := pkcs9.TimeStampReq{
		Version: 1,
		MessageImprint: pkcs9.MessageImprint{
			HashAlgorithm: alg,
			HashedMessage: digest,
		},
		CertReq: true,
	}
	if opts.Nonce != nil {
		msg.Nonce = opts.Nonce
	}
	if opts.TSAPolicyOid != nil {
		msg.ReqPolicy = opts.TSAPolicyOid
	}

	return &msg, nil
}

func ParseTimestampRequest(data []byte) (*pkcs9.TimeStampReq, error) {
	msg := new(pkcs9.TimeStampReq)
	if rest, err := asn1.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("error umarshalling request")
	} else if len(rest) != 0 {
		return nil, fmt.Errorf("error umarshalling request, trailing bytes")
	}
	return msg, nil
}

func GetSigningTime(psd *pkcs7.ContentInfoSignedData) (time.Time, error) {
	// See sassoftware pkcs9 package for this code extracting TSTInfo
	infobytes, err := psd.Content.ContentInfo.Bytes()
	if err != nil {
		return time.Time{}, fmt.Errorf("unpack TSTInfo: %w", err)
	} else if infobytes[0] == 0x04 {
		// unwrap dummy OCTET STRING
		_, err = asn1.Unmarshal(infobytes, &infobytes)
		if err != nil {
			return time.Time{}, fmt.Errorf("unpack TSTInfo: %w", err)
		}
	}
	info := new(pkcs9.TSTInfo)
	if _, err := asn1.Unmarshal(infobytes, info); err != nil {
		return time.Time{}, fmt.Errorf("unpack TSTInfo: %w", err)
	}

	return pkcs7.ParseTime(info.GenTime)
}

func CreateRfc3161Response(ctx context.Context, req pkcs9.TimeStampReq, certChain []*x509.Certificate, signer signature.Signer) (*pkcs9.TimeStampResp, error) {
	// Populate TSTInfo.
	genTimeBytes, err := asn1.MarshalWithParams(time.Now(), "generalized")
	if err != nil {
		return nil, err
	}
	policy := asn1.ObjectIdentifier{1, 2, 3, 4, 1}
	if req.ReqPolicy.String() != "" {
		policy = req.ReqPolicy
	}

	info := pkcs9.TSTInfo{
		Version:        req.Version,
		MessageImprint: req.MessageImprint,
		// directoryName is tag 4 https://datatracker.ietf.org/doc/html/rfc3280#section-4.2.1.7
		TSA: pkcs9.GeneralName{Value: asn1.RawValue{Tag: 4, Class: 2, IsCompound: true, Bytes: certChain[0].RawSubject}},
		// TODO: Ensure that every (SerialNumber, TSA name) identifies a unique token.
		SerialNumber: x509tools.MakeSerial(),
		GenTime:      asn1.RawValue{FullBytes: genTimeBytes},
		Nonce:        req.Nonce,
		Policy:       policy,
		Extensions:   req.Extensions,
	}

	encoded, err := asn1.Marshal(info)
	if err != nil {
		return nil, err
	}
	contentInfo, err := pkcs7.NewContentInfo(pkcs9.OidTSTInfo, encoded)
	if err != nil {
		return nil, err
	}

	// TODO: Does this need to match the hash algorithm in the request?
	alg, _ := x509tools.PkixDigestAlgorithm(crypto.SHA256)
	contentInfoBytes, _ := contentInfo.Bytes()
	h := sha256.Sum256(contentInfoBytes)

	// Create SignerInfo and signature.
	signingCert, err := createSigningCertificate(certChain[0])
	if err != nil {
		return nil, err
	}
	attributes := new(pkcs7.AttributeList)
	if err := attributes.Add(pkcs7.OidAttributeContentType, contentInfo.ContentType); err != nil {
		return nil, err
	}
	if err := attributes.Add(pkcs7.OidAttributeMessageDigest, h[:]); err != nil {
		return nil, err
	}
	if err := attributes.Add(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}, signingCert); err != nil {
		return nil, err
	}

	// The signature is over the entire authenticated attributes, not just the TstInfo.
	attrBytes, err := attributes.Bytes()
	if err != nil {
		return nil, err
	}
	// Get signature.
	signature, err := signer.SignMessage(bytes.NewReader(attrBytes), options.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	sigAlg, err := getPKIXPublicKeyAlgorithm(*certChain[0])
	if err != nil {
		return nil, err
	}

	response := pkcs9.TimeStampResp{
		Status: pkcs9.PKIStatusInfo{
			Status: 0,
		},
		TimeStampToken: pkcs7.ContentInfoSignedData{
			ContentType: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}, // id-signedData
			Content: pkcs7.SignedData{
				Version:                    1,
				DigestAlgorithmIdentifiers: []pkix.AlgorithmIdentifier{alg},
				ContentInfo:                contentInfo,
				Certificates:               marshalCertificates(certChain),
				CRLs:                       nil,
				SignerInfos: []pkcs7.SignerInfo{{
					Version: 1,
					IssuerAndSerialNumber: pkcs7.IssuerAndSerial{
						IssuerName:   asn1.RawValue{FullBytes: certChain[0].RawIssuer},
						SerialNumber: certChain[0].SerialNumber,
					},
					DigestAlgorithm:           alg,
					DigestEncryptionAlgorithm: *sigAlg,
					AuthenticatedAttributes:   *attributes,
					EncryptedDigest:           signature,
				}},
			},
		},
	}
	return &response, nil
}
