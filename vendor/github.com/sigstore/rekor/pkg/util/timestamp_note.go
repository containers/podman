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
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/mod/sumdb/note"
)

// Signed note based timestamp responses

type TimestampNote struct {
	// Ecosystem is the ecosystem/version string
	Ecosystem string
	// MessageImprint is the hash of the message to timestamp, of the form sha256:<sha>
	MessageImprint []byte
	// Nonce is a short random  bytes to prove response freshness
	Nonce []byte
	// Time is the timestamp to imprint on the message
	Time time.Time
	// Radius is the time in microseconds used to indicate certainty
	Radius int64
	// CertChainRef is a reference URL to the valid timestamping cert chain used to sign the response
	CertChainRef *url.URL
	// OtherContent is any additional data to be included in the signed payload; each element is assumed to be one line
	OtherContent []string
}

// String returns the String representation of the TimestampNote
func (t TimestampNote) String() string {
	var b strings.Builder
	time, _ := t.Time.MarshalText()
	fmt.Fprintf(&b, "%s\n%s\n%d\n%s\n%d\n%s", t.Ecosystem, base64.StdEncoding.EncodeToString(t.MessageImprint),
		t.Nonce, time, t.Radius, t.CertChainRef)
	for _, line := range t.OtherContent {
		fmt.Fprintf(&b, "%s\n", line)
	}
	return b.String()
}

// MarshalText returns the common format representation of this TimestampNote.
func (t TimestampNote) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText parses the common formatted timestamp note data and stores the result
// in the TimestampNote.
//
// The supplied data is expected to begin with the following 6 lines of text,
// each followed by a newline:
// <ecosystem/version string>
// <base64 representation of message hash>
// <base64 representation of the nonce>
// <RFC 3339 representation of the time>
// <decimal representation of radius>
// <cert chain URI>
// <optional non-empty line of other content>...
// <optional non-empty line of other content>...
//
// This will discard any content found after the checkpoint (including signatures)
func (t *TimestampNote) UnmarshalText(data []byte) error {
	l := bytes.Split(data, []byte("\n"))
	if len(l) < 7 {
		return errors.New("invalid timestamp note - too few newlines")
	}
	eco := string(l[0])
	if len(eco) == 0 {
		return errors.New("invalid timestamp note - empty ecosystem")
	}
	h, err := base64.StdEncoding.DecodeString(string(l[1]))
	if err != nil {
		return fmt.Errorf("invalid timestamp note - invalid message hash: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(string(l[2]))
	if err != nil {
		return fmt.Errorf("invalid timestamp note - invalid nonce: %w", err)
	}
	var timestamp time.Time
	if err := timestamp.UnmarshalText(l[3]); err != nil {
		return fmt.Errorf("invalid timestamp note - invalid time: %w", err)
	}
	r, err := strconv.ParseInt(string(l[4]), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp note - invalid radius: %w", err)
	}
	u, err := url.Parse(string(l[5]))
	if err != nil {
		return fmt.Errorf("invalid timestamp note - invalid URI: %w", err)

	}
	*t = TimestampNote{
		Ecosystem:      eco,
		MessageImprint: h,
		Nonce:          nonce,
		Time:           timestamp,
		Radius:         r,
		CertChainRef:   u,
	}
	if len(l) >= 5 {
		for _, line := range l[3:] {
			if len(line) == 0 {
				break
			}
			t.OtherContent = append(t.OtherContent, string(line))
		}
	}
	return nil
}

func (t TimestampNote) Sign(identity string, signer crypto.Signer, opts crypto.SignerOpts) (*note.Signature, error) {
	hf := crypto.SHA256
	if opts != nil {
		hf = opts.HashFunc()
	}

	input, _ := t.MarshalText()
	var digest []byte
	if hf != crypto.Hash(0) {
		hasher := hf.New()
		_, err := hasher.Write(input)
		if err != nil {
			return nil, errors.Wrap(err, "hashing timestamp note before signing")
		}
		digest = hasher.Sum(nil)
	} else {
		digest, _ = t.MarshalText()
	}

	sig, err := signer.Sign(rand.Reader, digest, opts)
	if err != nil {
		return nil, errors.Wrap(err, "signing timestamp note")
	}
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return nil, errors.Wrap(err, "marshalling public key")
	}

	pkSha := sha256.Sum256(pubKeyBytes)

	signature := note.Signature{
		Name:   identity,
		Hash:   binary.BigEndian.Uint32(pkSha[:]),
		Base64: base64.StdEncoding.EncodeToString(sig),
	}

	return &signature, nil
}
