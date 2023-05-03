// Copyright 2017 Google LLC. All Rights Reserved.
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

package client

import (
	"errors"
	"fmt"

	"github.com/google/trillian"
	"github.com/google/trillian/types"
	"github.com/transparency-dev/merkle"
	"github.com/transparency-dev/merkle/proof"
	"github.com/transparency-dev/merkle/rfc6962"
)

// LogVerifier allows verification of output from Trillian Logs, both regular
// and pre-ordered; it is safe for concurrent use (as its contents are fixed
// after construction).
type LogVerifier struct {
	// hasher is the hash strategy used to compute nodes in the Merkle tree.
	hasher merkle.LogHasher
}

// NewLogVerifier returns an object that can verify output from Trillian Logs.
func NewLogVerifier(hasher merkle.LogHasher) *LogVerifier {
	return &LogVerifier{hasher: hasher}
}

// NewLogVerifierFromTree creates a new LogVerifier using the algorithms
// specified by a Trillian Tree object.
func NewLogVerifierFromTree(config *trillian.Tree) (*LogVerifier, error) {
	if config == nil {
		return nil, errors.New("client: NewLogVerifierFromTree(): nil config")
	}
	log, pLog := trillian.TreeType_LOG, trillian.TreeType_PREORDERED_LOG
	if got := config.TreeType; got != log && got != pLog {
		return nil, fmt.Errorf("client: NewLogVerifierFromTree(): TreeType: %v, want %v or %v", got, log, pLog)
	}

	return NewLogVerifier(rfc6962.DefaultHasher), nil
}

// VerifyRoot verifies that newRoot is a valid append-only operation from
// trusted. If trusted.TreeSize is zero, a consistency proof is not needed.
func (c *LogVerifier) VerifyRoot(trusted *types.LogRootV1, newRoot *trillian.SignedLogRoot, consistency [][]byte) (*types.LogRootV1, error) {
	if trusted == nil {
		return nil, fmt.Errorf("VerifyRoot() error: trusted == nil")
	}
	if newRoot == nil {
		return nil, fmt.Errorf("VerifyRoot() error: newRoot == nil")
	}

	var r types.LogRootV1
	if err := r.UnmarshalBinary(newRoot.LogRoot); err != nil {
		return nil, err
	}

	// Implicitly trust the first root we get.
	if trusted.TreeSize != 0 {
		// Verify consistency proof.
		if err := proof.VerifyConsistency(c.hasher, trusted.TreeSize, r.TreeSize, consistency, trusted.RootHash, r.RootHash); err != nil {
			return nil, fmt.Errorf("failed to verify consistency proof from %d->%d %x->%x: %v", trusted.TreeSize, r.TreeSize, trusted.RootHash, r.RootHash, err)
		}
	}
	return &r, nil
}

// VerifyInclusionByHash verifies that the inclusion proof for the given Merkle leafHash
// matches the given trusted root.
func (c *LogVerifier) VerifyInclusionByHash(trusted *types.LogRootV1, leafHash []byte, pf *trillian.Proof) error {
	if trusted == nil {
		return fmt.Errorf("VerifyInclusionByHash() error: trusted == nil")
	}
	if pf == nil {
		return fmt.Errorf("VerifyInclusionByHash() error: proof == nil")
	}

	return proof.VerifyInclusion(c.hasher, uint64(pf.LeafIndex), trusted.TreeSize, leafHash, pf.Hashes, trusted.RootHash)
}
