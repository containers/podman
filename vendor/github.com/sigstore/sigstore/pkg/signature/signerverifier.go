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

package signature

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

type SignerVerifier interface {
	Signer
	Verifier
}

// LoadSignerVerifier returns a signature.SignerVerifier based on the algorithm of the private key
// provided.
//
// If privateKey is an RSA key, a RSAPKCS1v15SignerVerifier will be returned. If a
// RSAPSSSignerVerifier is desired instead, use the LoadRSAPSSSignerVerifier() method directly.
func LoadSignerVerifier(privateKey crypto.PrivateKey, hashFunc crypto.Hash) (SignerVerifier, error) {
	switch pk := privateKey.(type) {
	case *rsa.PrivateKey:
		return LoadRSAPKCS1v15SignerVerifier(pk, hashFunc)
	case *ecdsa.PrivateKey:
		return LoadECDSASignerVerifier(pk, hashFunc)
	case ed25519.PrivateKey:
		return LoadED25519SignerVerifier(pk)
	}
	return nil, errors.New("unsupported public key type")
}

// LoadSignerVerifierFromPEMFile returns a signature.SignerVerifier based on the algorithm of the private key
// in the file. The SignerVerifier will use the hash function specified when computing digests.
//
// If publicKey is an RSA key, a RSAPKCS1v15SignerVerifier will be returned. If a
// RSAPSSSignerVerifier is desired instead, use the LoadRSAPSSSignerVerifier() and
// cryptoutils.UnmarshalPEMToPrivateKey() methods directly.
func LoadSignerVerifierFromPEMFile(path string, hashFunc crypto.Hash, pf cryptoutils.PassFunc) (SignerVerifier, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	priv, err := cryptoutils.UnmarshalPEMToPrivateKey(fileBytes, pf)
	if err != nil {
		return nil, err
	}
	return LoadSignerVerifier(priv, hashFunc)
}
