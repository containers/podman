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

package rekord

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/types"
)

const (
	KIND = "rekord"
)

type BaseRekordType struct {
	types.RekorType
}

func init() {
	types.TypeMap.Store(KIND, New)
}

func New() types.TypeImpl {
	brt := BaseRekordType{}
	brt.Kind = KIND
	brt.VersionMap = VersionMap
	return &brt
}

var VersionMap = types.NewSemVerEntryFactoryMap()

func (rt BaseRekordType) UnmarshalEntry(pe models.ProposedEntry) (types.EntryImpl, error) {
	if pe == nil {
		return nil, errors.New("proposed entry cannot be nil")
	}

	rekord, ok := pe.(*models.Rekord)
	if !ok {
		return nil, errors.New("cannot unmarshal non-Rekord types")
	}

	return rt.VersionedUnmarshal(rekord, *rekord.APIVersion)
}

func (rt *BaseRekordType) CreateProposedEntry(ctx context.Context, version string, props types.ArtifactProperties) (models.ProposedEntry, error) {
	if version == "" {
		version = rt.DefaultVersion()
	}
	ei, err := rt.VersionedUnmarshal(nil, version)
	if err != nil {
		return nil, errors.Wrap(err, "fetching Rekord version implementation")
	}
	return ei.CreateFromArtifactProperties(ctx, props)
}

func (rt BaseRekordType) DefaultVersion() string {
	return "0.0.1"
}
