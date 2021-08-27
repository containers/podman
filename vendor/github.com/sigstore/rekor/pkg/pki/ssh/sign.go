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
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"io"

	"golang.org/x/crypto/ssh"
)

// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig#L81
type MessageWrapper struct {
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Hash          string
}

// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig#L34
type WrappedSig struct {
	MagicHeader   [6]byte
	Version       uint32
	PublicKey     string
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Signature     string
}

const (
	magicHeader          = "SSHSIG"
	defaultHashAlgorithm = "sha512"
)

var supportedHashAlgorithms = map[string]func() hash.Hash{
	"sha256": sha256.New,
	"sha512": sha512.New,
}

func sign(s ssh.AlgorithmSigner, m io.Reader) (*ssh.Signature, error) {
	hf := sha512.New()
	if _, err := io.Copy(hf, m); err != nil {
		return nil, err
	}
	mh := hf.Sum(nil)

	sp := MessageWrapper{
		Namespace:     "file",
		HashAlgorithm: defaultHashAlgorithm,
		Hash:          string(mh),
	}

	dataMessageWrapper := ssh.Marshal(sp)
	dataMessageWrapper = append([]byte(magicHeader), dataMessageWrapper...)

	// ssh-rsa is not supported for RSA keys:
	// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig#L71
	// We can use the default value of "" for other key types though.
	algo := ""
	if s.PublicKey().Type() == ssh.KeyAlgoRSA {
		algo = ssh.SigAlgoRSASHA2512
	}
	sig, err := s.SignWithAlgorithm(rand.Reader, dataMessageWrapper, algo)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

func Sign(sshPrivateKey string, data io.Reader) ([]byte, error) {
	s, err := ssh.ParsePrivateKey([]byte(sshPrivateKey))
	if err != nil {
		return nil, err
	}

	as, ok := s.(ssh.AlgorithmSigner)
	if !ok {
		return nil, err
	}

	sig, err := sign(as, data)
	if err != nil {
		return nil, err
	}

	armored := Armor(sig, s.PublicKey())
	return armored, nil
}
