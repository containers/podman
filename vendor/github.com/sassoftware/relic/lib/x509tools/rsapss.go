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
	"crypto/rsa"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
)

// pssParameters reflects the parameters in an AlgorithmIdentifier that
// specifies RSA PSS. See https://tools.ietf.org/html/rfc3447#appendix-A.2.3
type pssParameters struct {
	// The following three fields are not marked as
	// optional because the default values specify SHA-1,
	// which is no longer suitable for use in signatures.
	Hash         pkix.AlgorithmIdentifier `asn1:"explicit,tag:0"`
	MGF          pkix.AlgorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength   int                      `asn1:"explicit,tag:2"`
	TrailerField int                      `asn1:"optional,explicit,tag:3,default:1"`
}

func MarshalRSAPSSParameters(pub *rsa.PublicKey, opts *rsa.PSSOptions) (asn1.RawValue, error) {
	hashAlg, ok := PkixDigestAlgorithm(opts.Hash)
	if !ok {
		return asn1.RawValue{}, errors.New("unsupported digest algorithm")
	}
	hashRaw, err := asn1.Marshal(hashAlg)
	if err != nil {
		return asn1.RawValue{}, err
	}
	saltLength := opts.SaltLength
	switch saltLength {
	case rsa.PSSSaltLengthAuto:
		saltLength = (pub.N.BitLen()+7)/8 - 2 - opts.Hash.Size()
	case rsa.PSSSaltLengthEqualsHash:
		saltLength = opts.Hash.Size()
	}
	params := pssParameters{
		Hash: hashAlg,
		MGF: pkix.AlgorithmIdentifier{
			Algorithm:  OidMGF1,
			Parameters: asn1.RawValue{FullBytes: hashRaw},
		},
		SaltLength:   saltLength,
		TrailerField: 1,
	}
	serialized, err := asn1.Marshal(params)
	if err != nil {
		return asn1.RawValue{}, err
	}
	return asn1.RawValue{FullBytes: serialized}, nil
}

func UnmarshalRSAPSSParameters(hash crypto.Hash, raw asn1.RawValue) (*rsa.PSSOptions, error) {
	hashOid, ok := HashOids[hash]
	if !ok {
		return nil, errors.New("unsupported digest algorithm")
	}
	if raw.Tag == asn1.TagNull {
		// defaults
		if hash != crypto.SHA1 {
			return nil, errors.New("RSA-PSS parameters not provided but digest type is not SHA-1")
		}
		return &rsa.PSSOptions{Hash: crypto.SHA1, SaltLength: 20}, nil
	}
	var params pssParameters
	if rest, err := asn1.Unmarshal(raw.FullBytes, &params); err != nil || len(rest) > 0 {
		return nil, errors.New("invalid RSA-PSS parameters")
	}
	// validate that all hash OIDs match
	if !params.Hash.Algorithm.Equal(hashOid) {
		return nil, errors.New("digest type mismatch in RSA-PSS parameters")
	}
	if !params.MGF.Algorithm.Equal(OidMGF1) {
		return nil, errors.New("unsupported MGF in RSA-PSS parameters")
	}
	var mgfDigest pkix.AlgorithmIdentifier
	if rest, err := asn1.Unmarshal(params.MGF.Parameters.FullBytes, &mgfDigest); err != nil || len(rest) > 0 {
		return nil, errors.New("invalid RSA-PSS parameters")
	}
	if !mgfDigest.Algorithm.Equal(hashOid) {
		return nil, errors.New("digest type mismatch in RSA-PSS parameters")
	}
	if params.TrailerField != 0 && params.TrailerField != 1 {
		return nil, errors.New("invalid RSA-PSS parameters")
	}
	if params.SaltLength == 0 {
		params.SaltLength = hash.Size()
	}
	return &rsa.PSSOptions{Hash: hash, SaltLength: params.SaltLength}, nil
}
