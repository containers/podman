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

package ssh

import (
	"io"

	"golang.org/x/crypto/ssh"
)

func Verify(message io.Reader, armoredSignature []byte, publicKey []byte) error {
	decodedSignature, err := Decode(armoredSignature)
	if err != nil {
		return err
	}

	desiredPk, _, _, _, err := ssh.ParseAuthorizedKey(publicKey)
	if err != nil {
		return err
	}

	// Hash the message so we can verify it against the signature.
	h := supportedHashAlgorithms[decodedSignature.hashAlg]()
	if _, err := io.Copy(h, message); err != nil {
		return err
	}
	hm := h.Sum(nil)

	toVerify := MessageWrapper{
		Namespace:     "file",
		HashAlgorithm: decodedSignature.hashAlg,
		Hash:          string(hm),
	}
	signedMessage := ssh.Marshal(toVerify)
	signedMessage = append([]byte(magicHeader), signedMessage...)
	return desiredPk.Verify(signedMessage, decodedSignature.signature)
}
