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
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// heavily borrowed from https://github.com/google/trillian-examples/blob/master/formats/log/checkpoint.go

type Checkpoint struct {
	// Ecosystem is the ecosystem/version string
	Ecosystem string
	// Size is the number of entries in the log at this checkpoint.
	Size uint64
	// Hash is the hash which commits to the contents of the entire log.
	Hash []byte
	// OtherContent is any additional data to be included in the signed payload; each element is assumed to be one line
	OtherContent []string
}

// String returns the String representation of the Checkpoint
func (c Checkpoint) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%d\n%s\n", c.Ecosystem, c.Size, base64.StdEncoding.EncodeToString(c.Hash))
	for _, line := range c.OtherContent {
		fmt.Fprintf(&b, "%s\n", line)
	}
	return b.String()
}

// MarshalText returns the common format representation of this Checkpoint.
func (c Checkpoint) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

// UnmarshalText parses the common formatted checkpoint data and stores the result
// in the Checkpoint.
//
// The supplied data is expected to begin with the following 3 lines of text,
// each followed by a newline:
// <ecosystem/version string>
// <decimal representation of log size>
// <base64 representation of root hash>
// <optional non-empty line of other content>...
// <optional non-empty line of other content>...
//
// This will discard any content found after the checkpoint (including signatures)
func (c *Checkpoint) UnmarshalText(data []byte) error {
	l := bytes.Split(data, []byte("\n"))
	if len(l) < 4 {
		return errors.New("invalid checkpoint - too few newlines")
	}
	eco := string(l[0])
	if len(eco) == 0 {
		return errors.New("invalid checkpoint - empty ecosystem")
	}
	size, err := strconv.ParseUint(string(l[1]), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid checkpoint - size invalid: %w", err)
	}
	h, err := base64.StdEncoding.DecodeString(string(l[2]))
	if err != nil {
		return fmt.Errorf("invalid checkpoint - invalid hash: %w", err)
	}
	*c = Checkpoint{
		Ecosystem: eco,
		Size:      size,
		Hash:      h,
	}
	if len(l) >= 5 {
		for _, line := range l[3:] {
			if len(line) == 0 {
				break
			}
			c.OtherContent = append(c.OtherContent, string(line))
		}
	}
	return nil
}

type SignedCheckpoint struct {
	Checkpoint
	SignedNote
}

func CreateSignedCheckpoint(c Checkpoint) (*SignedCheckpoint, error) {
	text, err := c.MarshalText()
	if err != nil {
		return nil, err
	}
	return &SignedCheckpoint{
		Checkpoint: c,
		SignedNote: SignedNote{Note: string(text)},
	}, nil
}

func SignedCheckpointValidator(strToValidate string) bool {
	s := SignedNote{}
	if err := s.UnmarshalText([]byte(strToValidate)); err != nil {
		return false
	}
	c := &Checkpoint{}
	return c.UnmarshalText([]byte(s.Note)) == nil
}

func CheckpointValidator(strToValidate string) bool {
	c := &Checkpoint{}
	return c.UnmarshalText([]byte(strToValidate)) == nil
}

func (r *SignedCheckpoint) UnmarshalText(data []byte) error {
	s := SignedNote{}
	if err := s.UnmarshalText([]byte(data)); err != nil {
		return errors.Wrap(err, "unmarshalling signed note")
	}
	c := Checkpoint{}
	if err := c.UnmarshalText([]byte(s.Note)); err != nil {
		return errors.Wrap(err, "unmarshalling checkpoint")
	}
	*r = SignedCheckpoint{Checkpoint: c, SignedNote: s}
	return nil
}

func (r *SignedCheckpoint) SetTimestamp(timestamp uint64) {
	var ts uint64
	for i, val := range r.OtherContent {
		if n, _ := fmt.Fscanf(strings.NewReader(val), "Timestamp: %d", &ts); n == 1 {
			r.OtherContent = append(r.OtherContent[:i], r.OtherContent[i+1:]...)
		}
	}
	r.OtherContent = append(r.OtherContent, fmt.Sprintf("Timestamp: %d", timestamp))
}

func (r *SignedCheckpoint) GetTimestamp() uint64 {
	var ts uint64
	for _, val := range r.OtherContent {
		if n, _ := fmt.Fscanf(strings.NewReader(val), "Timestamp: %d", &ts); n == 1 {
			break
		}
	}
	return ts
}
