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
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Signed note based timestamp responses

type TimestampNote struct {
	// Origin is the unique identifier/version string
	Origin string
	// MessageImprint is the hash of the message to timestamp, of the form sha256:<sha>
	MessageImprint string
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
	fmt.Fprintf(&b, "%s\n%s\n%s\n%s\n%d\n%s\n", t.Origin, t.MessageImprint, base64.StdEncoding.EncodeToString(t.Nonce),
		time, t.Radius, t.CertChainRef)
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
// <message hash of the format sha256:$SHA>
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
	origin := string(l[0])
	if len(origin) == 0 {
		return errors.New("invalid timestamp note - empty ecosystem")
	}
	h := string(l[1])
	if err := ValidateSHA256Value(h); err != nil {
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
		Origin:         origin,
		MessageImprint: h,
		Nonce:          nonce,
		Time:           timestamp,
		Radius:         r,
		CertChainRef:   u,
	}
	if len(l) >= 8 {
		for _, line := range l[6:] {
			if len(line) == 0 {
				break
			}
			t.OtherContent = append(t.OtherContent, string(line))
		}
	}
	return nil
}

type SignedTimestampNote struct {
	TimestampNote
	SignedNote
}

func CreateSignedTimestampNote(t TimestampNote) (*SignedTimestampNote, error) {
	text, err := t.MarshalText()
	if err != nil {
		return nil, err
	}
	return &SignedTimestampNote{
		TimestampNote: t,
		SignedNote:    SignedNote{Note: string(text)},
	}, nil
}

func SignedTimestampNoteValidator(strToValidate string) bool {
	s := SignedNote{}
	if err := s.UnmarshalText([]byte(strToValidate)); err != nil {
		return false
	}
	c := &TimestampNote{}
	return c.UnmarshalText([]byte(s.Note)) == nil
}

func TimestampNoteValidator(strToValidate string) bool {
	c := &TimestampNote{}
	return c.UnmarshalText([]byte(strToValidate)) == nil
}

func (r *SignedTimestampNote) UnmarshalText(data []byte) error {
	s := SignedNote{}
	if err := s.UnmarshalText([]byte(data)); err != nil {
		return fmt.Errorf("unmarshalling signed note: %w", err)
	}
	t := TimestampNote{}
	if err := t.UnmarshalText([]byte(s.Note)); err != nil {
		return fmt.Errorf("unmarshalling timestamp note: %w", err)
	}
	*r = SignedTimestampNote{TimestampNote: t, SignedNote: s}
	return nil
}
