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
	crand "crypto/rand"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

func isSupportedAlg(alg crypto.Hash, supportedAlgs []crypto.Hash) bool {
	if supportedAlgs == nil {
		return true
	}
	for _, supportedAlg := range supportedAlgs {
		if alg == supportedAlg {
			return true
		}
	}
	return false
}

func ComputeDigestForSigning(rawMessage io.Reader, defaultHashFunc crypto.Hash, supportedHashFuncs []crypto.Hash, opts ...SignOption) (digest []byte, hashedWith crypto.Hash, err error) {
	var cryptoSignerOpts crypto.SignerOpts = defaultHashFunc
	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&cryptoSignerOpts)
	}
	hashedWith = cryptoSignerOpts.HashFunc()
	if !isSupportedAlg(hashedWith, supportedHashFuncs) {
		return nil, crypto.Hash(0), fmt.Errorf("unsupported hash algorithm: %q not in %v", hashedWith.String(), supportedHashFuncs)
	}
	if len(digest) > 0 {
		if hashedWith != crypto.Hash(0) && len(digest) != hashedWith.Size() {
			err = errors.New("unexpected length of digest for hash function specified")
		}
		return
	}
	digest, err = hashMessage(rawMessage, hashedWith)
	return
}

func ComputeDigestForVerifying(rawMessage io.Reader, defaultHashFunc crypto.Hash, supportedHashFuncs []crypto.Hash, opts ...VerifyOption) (digest []byte, hashedWith crypto.Hash, err error) {
	var cryptoSignerOpts crypto.SignerOpts = defaultHashFunc
	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&cryptoSignerOpts)
	}
	hashedWith = cryptoSignerOpts.HashFunc()
	if !isSupportedAlg(hashedWith, supportedHashFuncs) {
		return nil, crypto.Hash(0), fmt.Errorf("unsupported hash algorithm: %q not in %v", hashedWith.String(), supportedHashFuncs)
	}
	if len(digest) > 0 {
		if hashedWith != crypto.Hash(0) && len(digest) != hashedWith.Size() {
			err = errors.New("unexpected length of digest for hash function specified")
		}
		return
	}
	digest, err = hashMessage(rawMessage, hashedWith)
	return
}

func hashMessage(rawMessage io.Reader, hashFunc crypto.Hash) ([]byte, error) {
	if rawMessage == nil {
		return nil, errors.New("message cannot be nil")
	}
	if hashFunc == crypto.Hash(0) {
		return io.ReadAll(rawMessage)
	}
	hasher := hashFunc.New()
	// avoids reading entire message into memory
	if _, err := io.Copy(hasher, rawMessage); err != nil {
		return nil, errors.Wrap(err, "hashing message")
	}
	return hasher.Sum(nil), nil
}

func selectRandFromOpts(opts ...SignOption) io.Reader {
	rand := crand.Reader
	for _, opt := range opts {
		opt.ApplyRand(&rand)
	}
	return rand
}
