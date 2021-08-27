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

package types

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"reflect"

	"github.com/mitchellh/mapstructure"
	"github.com/sigstore/rekor/pkg/generated/models"
)

// EntryImpl specifies the behavior of a versioned type
type EntryImpl interface {
	APIVersion() string                               // the supported versions for this implementation
	IndexKeys() []string                              // the keys that should be added to the external index for this entry
	Canonicalize(ctx context.Context) ([]byte, error) // marshal the canonical entry to be put into the tlog
	Unmarshal(e models.ProposedEntry) error           // unmarshal the abstract entry into the specific struct for this versioned type
	Attestation() (string, []byte)
	CreateFromArtifactProperties(context.Context, ArtifactProperties) (models.ProposedEntry, error)
}

// EntryFactory describes a factory function that can generate structs for a specific versioned type
type EntryFactory func() EntryImpl

func NewProposedEntry(ctx context.Context, kind, version string, props ArtifactProperties) (models.ProposedEntry, error) {
	if tf, found := TypeMap.Load(kind); found {
		t := tf.(func() TypeImpl)()
		if t == nil {
			return nil, fmt.Errorf("error generating object for kind '%v'", kind)
		}
		return t.CreateProposedEntry(ctx, version, props)
	}
	return nil, fmt.Errorf("could not create entry for kind '%v'", kind)
}

// NewEntry returns the specific instance for the type and version specified in the doc
func NewEntry(pe models.ProposedEntry) (EntryImpl, error) {
	if pe == nil {
		return nil, errors.New("proposed entry cannot be nil")
	}

	kind := pe.Kind()
	if tf, found := TypeMap.Load(kind); found {
		t := tf.(func() TypeImpl)()
		if t == nil {
			return nil, fmt.Errorf("error generating object for kind '%v'", kind)
		}
		return t.UnmarshalEntry(pe)
	}
	return nil, fmt.Errorf("could not create entry for kind '%v'", kind)
}

// DecodeEntry maps the (abstract) input structure into the specific entry implementation class;
// while doing so, it detects the case where we need to convert from string to []byte and does
// the base64 decoding required to make that happen
func DecodeEntry(input, output interface{}) error {
	cfg := mapstructure.DecoderConfig{
		DecodeHook: func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
			if f.Kind() != reflect.String || t.Kind() != reflect.Slice {
				return data, nil
			}

			if data == nil {
				return nil, errors.New("attempted to decode nil data")
			}

			bytes, err := base64.StdEncoding.DecodeString(data.(string))
			if err != nil {
				return []byte{}, fmt.Errorf("failed parsing base64 data: %v", err)
			}
			return bytes, nil
		},
		Result: output,
	}

	dec, err := mapstructure.NewDecoder(&cfg)
	if err != nil {
		return fmt.Errorf("error initializing decoder: %w", err)
	}

	return dec.Decode(input)
}

// ArtifactProperties provide a consistent struct for passing values from
// CLI flags to the type+version specific CreateProposeEntry() methods
type ArtifactProperties struct {
	ArtifactPath   *url.URL
	ArtifactHash   string
	ArtifactBytes  []byte
	SignaturePath  *url.URL
	SignatureBytes []byte
	PublicKeyPath  *url.URL
	PublicKeyBytes []byte
	PKIFormat      string
}
