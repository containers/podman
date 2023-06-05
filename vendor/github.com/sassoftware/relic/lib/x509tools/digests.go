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
	"crypto/x509/pkix"
	"encoding/asn1"
	"strings"
	"sync"
)

var (
	// RFC 3279
	OidDigestMD5  = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 5}
	OidDigestSHA1 = asn1.ObjectIdentifier{1, 3, 14, 3, 2, 26}
	// RFC 5758
	OidDigestSHA224 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 4}
	OidDigestSHA256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	OidDigestSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	OidDigestSHA512 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}
)

var HashOids = map[crypto.Hash]asn1.ObjectIdentifier{
	crypto.MD5:    OidDigestMD5,
	crypto.SHA1:   OidDigestSHA1,
	crypto.SHA224: OidDigestSHA224,
	crypto.SHA256: OidDigestSHA256,
	crypto.SHA384: OidDigestSHA384,
	crypto.SHA512: OidDigestSHA512,
}

var HashNames = map[crypto.Hash]string{
	crypto.MD5:    "MD5",
	crypto.SHA1:   "SHA1",
	crypto.SHA224: "SHA-224",
	crypto.SHA256: "SHA-256",
	crypto.SHA384: "SHA-384",
	crypto.SHA512: "SHA-512",
}

var (
	hashesByName map[string]crypto.Hash
	once         sync.Once
)

func HashShortName(hash crypto.Hash) string {
	return normalName(HashNames[hash])
}

func normalName(name string) string {
	return strings.Replace(strings.ToLower(name), "-", "", 1)
}

func HashByName(name string) crypto.Hash {
	name = normalName(name)
	once.Do(func() {
		hashesByName = make(map[string]crypto.Hash, len(HashNames))
		for h, hn := range HashNames {
			hashesByName[normalName(hn)] = h
		}
	})
	return hashesByName[name]
}

type digestInfo struct {
	DigestAlgorithm pkix.AlgorithmIdentifier
	Digest          []byte
}

// Pack a digest along with an algorithm identifier. Mainly useful for
// PKCS#1v1.5 padding (RSA).
func MarshalDigest(hash crypto.Hash, digest []byte) (der []byte, ok bool) {
	alg, ok := PkixDigestAlgorithm(hash)
	if !ok {
		return nil, false
	}
	der, err := asn1.Marshal(digestInfo{alg, digest})
	if err != nil {
		return nil, false
	}
	return der, true
}
