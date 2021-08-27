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

package x509tools

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
)

var (
	// RFC 3279
	OidPublicKeyRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	OidPublicKeyDSA   = asn1.ObjectIdentifier{1, 2, 840, 10040, 4, 1}
	OidPublicKeyECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}

	oidSignatureMD2WithRSA      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 2}
	oidSignatureMD5WithRSA      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 4}
	oidSignatureSHA1WithRSA     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 5}
	oidSignatureSHA256WithRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
	oidSignatureSHA384WithRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12}
	oidSignatureSHA512WithRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13}
	oidSignatureDSAWithSHA1     = asn1.ObjectIdentifier{1, 2, 840, 10040, 4, 3}
	oidSignatureDSAWithSHA256   = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 3, 2}
	oidSignatureECDSAWithSHA1   = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 1}
	oidSignatureECDSAWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
	oidSignatureECDSAWithSHA384 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3}
	oidSignatureECDSAWithSHA512 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 4}
	oidISOSignatureSHA1WithRSA  = asn1.ObjectIdentifier{1, 3, 14, 3, 2, 29}

	// RFC 4055
	OidMGF1            = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	OidSignatureRSAPSS = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
)

const Asn1TagBMPString = asn1.TagBMPString

type sigAlgInfo struct {
	oid        asn1.ObjectIdentifier
	pubKeyAlgo x509.PublicKeyAlgorithm
	hash       crypto.Hash
}

var sigAlgInfos = []sigAlgInfo{
	// without a digest
	{OidPublicKeyRSA, x509.RSA, 0},
	{OidSignatureRSAPSS, x509.RSA, 0},
	{OidPublicKeyDSA, x509.DSA, 0},
	{OidPublicKeyECDSA, x509.ECDSA, 0},
	// with a digest
	{oidSignatureMD5WithRSA, x509.RSA, crypto.MD5},
	{oidSignatureSHA1WithRSA, x509.RSA, crypto.SHA1},
	{oidISOSignatureSHA1WithRSA, x509.RSA, crypto.SHA1},
	{oidSignatureSHA256WithRSA, x509.RSA, crypto.SHA256},
	{oidSignatureSHA384WithRSA, x509.RSA, crypto.SHA384},
	{oidSignatureSHA512WithRSA, x509.RSA, crypto.SHA512},
	{oidSignatureDSAWithSHA1, x509.DSA, crypto.SHA1},
	{oidSignatureDSAWithSHA256, x509.DSA, crypto.SHA256},
	{oidSignatureECDSAWithSHA1, x509.ECDSA, crypto.SHA1},
	{oidSignatureECDSAWithSHA256, x509.ECDSA, crypto.SHA256},
	{oidSignatureECDSAWithSHA384, x509.ECDSA, crypto.SHA384},
	{oidSignatureECDSAWithSHA512, x509.ECDSA, crypto.SHA512},
}

// Given a public key and signer options, return the appropriate X.509 digest and signature algorithms
func PkixAlgorithms(pub crypto.PublicKey, opts crypto.SignerOpts) (digestAlg, sigAlg pkix.AlgorithmIdentifier, err error) {
	digestAlg, ok := PkixDigestAlgorithm(opts.HashFunc())
	if !ok {
		err = errors.New("unsupported digest algorithm")
		return
	}
	if pss, ok := opts.(*rsa.PSSOptions); ok {
		rsapub, ok := pub.(*rsa.PublicKey)
		if !ok {
			err = errors.New("RSA-PSS is only valid for RSA keys")
			return
		}
		var params asn1.RawValue
		params, err = MarshalRSAPSSParameters(rsapub, pss)
		if err != nil {
			return
		}
		sigAlg = pkix.AlgorithmIdentifier{
			Algorithm:  OidSignatureRSAPSS,
			Parameters: params,
		}
		return
	}
	switch pub.(type) {
	case *rsa.PublicKey:
		sigAlg.Algorithm = OidPublicKeyRSA
	case *ecdsa.PublicKey:
		sigAlg.Algorithm = OidPublicKeyECDSA
	default:
		err = errors.New("unsupported public key algorithm")
		return
	}
	sigAlg.Parameters = asn1.NullRawValue
	return
}

// Convert a crypto.Hash to a X.509 AlgorithmIdentifier
func PkixDigestAlgorithm(hash crypto.Hash) (alg pkix.AlgorithmIdentifier, ok bool) {
	if oid, ok2 := HashOids[hash]; ok2 {
		alg.Algorithm = oid
		alg.Parameters = asn1.NullRawValue
		ok = true
	}
	return
}

// Convert a X.509 AlgorithmIdentifier to a crypto.Hash
func PkixDigestToHash(alg pkix.AlgorithmIdentifier) (hash crypto.Hash, ok bool) {
	for hash, oid := range HashOids {
		if alg.Algorithm.Equal(oid) {
			return hash, true
		}
	}
	return 0, false
}

// Convert a X.509 AlgorithmIdentifier to a crypto.Hash
func PkixDigestToHashE(alg pkix.AlgorithmIdentifier) (hash crypto.Hash, err error) {
	hash, ok := PkixDigestToHash(alg)
	if ok && hash.Available() {
		return hash, nil
	}
	return 0, UnknownDigestError{Algorithm: alg.Algorithm}
}

// Convert a crypto.PublicKey to a X.509 AlgorithmIdentifier
func PkixPublicKeyAlgorithm(pub crypto.PublicKey) (alg pkix.AlgorithmIdentifier, ok bool) {
	_, alg, err := PkixAlgorithms(pub, nil)
	return alg, err == nil
}

// Verify a signature using the algorithm specified by the given X.509 AlgorithmIdentifier
func PkixVerify(pub crypto.PublicKey, digestAlg, sigAlg pkix.AlgorithmIdentifier, digest, sig []byte) error {
	hash, err := PkixDigestToHashE(digestAlg)
	if err != nil {
		return err
	}
	var info sigAlgInfo
	for _, a := range sigAlgInfos {
		if sigAlg.Algorithm.Equal(a.oid) {
			info = a
		}
	}
	if info.hash != 0 && info.hash != hash {
		return errors.New("signature type does not match digest type")
	}
	switch info.pubKeyAlgo {
	case x509.RSA:
		key, ok := pub.(*rsa.PublicKey)
		if !ok {
			return errors.New("incorrect key type for signature")
		}
		if sigAlg.Algorithm.Equal(OidSignatureRSAPSS) {
			opts, err := UnmarshalRSAPSSParameters(hash, sigAlg.Parameters)
			if err != nil {
				return err
			}
			return rsa.VerifyPSS(key, hash, digest, sig, opts)
		}
		return rsa.VerifyPKCS1v15(key, hash, digest, sig)
	case x509.ECDSA:
		key, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("incorrect key type for signature")
		}
		esig, err := UnmarshalEcdsaSignature(sig)
		if err != nil {
			return err
		}
		if !ecdsa.Verify(key, digest, esig.R, esig.S) {
			return errors.New("ECDSA verification failed")
		}
		return nil
	default:
		return errors.New("unsupported public key algorithm")
	}
}

type UnknownDigestError struct {
	Algorithm asn1.ObjectIdentifier
}

func (e UnknownDigestError) Error() string {
	return fmt.Sprintf("unsupported hash algorithm %s", e.Algorithm)
}
