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

package logverifier

import (
	"bytes"
	"errors"
	"fmt"
	"math/bits"

	"github.com/google/trillian/merkle/hashers"
)

// RootMismatchError occurs when an inclusion proof fails.
type RootMismatchError struct {
	ExpectedRoot   []byte
	CalculatedRoot []byte
}

func (e RootMismatchError) Error() string {
	return fmt.Sprintf("calculated root:\n%v\n does not match expected root:\n%v", e.CalculatedRoot, e.ExpectedRoot)
}

// LogVerifier verifies inclusion and consistency proofs for append only logs.
type LogVerifier struct {
	hasher hashers.LogHasher
}

// New returns a new LogVerifier for a tree.
func New(hasher hashers.LogHasher) LogVerifier {
	return LogVerifier{hasher}
}

// VerifyInclusionProof verifies the correctness of the proof given the passed
// in information about the tree and leaf.
func (v LogVerifier) VerifyInclusionProof(leafIndex, treeSize int64, proof [][]byte, root []byte, leafHash []byte) error {
	calcRoot, err := v.RootFromInclusionProof(leafIndex, treeSize, proof, leafHash)
	if err != nil {
		return err
	}
	if !bytes.Equal(calcRoot, root) {
		return RootMismatchError{
			CalculatedRoot: calcRoot,
			ExpectedRoot:   root,
		}
	}
	return nil
}

// RootFromInclusionProof calculates the expected tree root given the proof and leaf.
// leafIndex starts at 0.  treeSize is the number of nodes in the tree.
// proof is an array of neighbor nodes from the bottom to the root.
func (v LogVerifier) RootFromInclusionProof(leafIndex, treeSize int64, proof [][]byte, leafHash []byte) ([]byte, error) {
	switch {
	case leafIndex < 0:
		return nil, fmt.Errorf("leafIndex %d < 0", leafIndex)
	case treeSize < 0:
		return nil, fmt.Errorf("treeSize %d < 0", treeSize)
	case leafIndex >= treeSize:
		return nil, fmt.Errorf("leafIndex is beyond treeSize: %d >= %d", leafIndex, treeSize)
	}
	if got, want := len(leafHash), v.hasher.Size(); got != want {
		return nil, fmt.Errorf("leafHash has unexpected size %d, want %d", got, want)
	}

	inner, border := decompInclProof(leafIndex, treeSize)
	if got, want := len(proof), inner+border; got != want {
		return nil, fmt.Errorf("wrong proof size %d, want %d", got, want)
	}

	ch := hashChainer(v)
	res := ch.chainInner(leafHash, proof[:inner], leafIndex)
	res = ch.chainBorderRight(res, proof[inner:])
	return res, nil
}

// VerifyConsistencyProof checks that the passed in consistency proof is valid
// between the passed in tree snapshots. Snapshots are the respective tree
// sizes. Accepts shapshot2 >= snapshot1 >= 0.
func (v LogVerifier) VerifyConsistencyProof(snapshot1, snapshot2 int64, root1, root2 []byte, proof [][]byte) error {
	switch {
	case snapshot1 < 0:
		return fmt.Errorf("snapshot1 (%d) < 0 ", snapshot1)
	case snapshot2 < snapshot1:
		return fmt.Errorf("snapshot2 (%d) < snapshot1 (%d)", snapshot1, snapshot2)
	case snapshot1 == snapshot2:
		if !bytes.Equal(root1, root2) {
			return RootMismatchError{
				CalculatedRoot: root1,
				ExpectedRoot:   root2,
			}
		} else if len(proof) > 0 {
			return errors.New("root1 and root2 match, but proof is non-empty")
		}
		return nil // Proof OK.
	case snapshot1 == 0:
		// Any snapshot greater than 0 is consistent with snapshot 0.
		if len(proof) > 0 {
			return fmt.Errorf("expected empty proof, but got %d components", len(proof))
		}
		return nil // Proof OK.
	case len(proof) == 0:
		return errors.New("empty proof")
	}

	inner, border := decompInclProof(snapshot1-1, snapshot2)
	shift := bits.TrailingZeros64(uint64(snapshot1))
	inner -= shift // Note: shift < inner if snapshot1 < snapshot2.

	// The proof includes the root hash for the sub-tree of size 2^shift.
	seed, start := proof[0], 1
	if snapshot1 == 1<<uint(shift) { // Unless snapshot1 is that very 2^shift.
		seed, start = root1, 0
	}
	if got, want := len(proof), start+inner+border; got != want {
		return fmt.Errorf("wrong proof size %d, want %d", got, want)
	}
	proof = proof[start:]
	// Now len(proof) == inner+border, and proof is effectively a suffix of
	// inclusion proof for entry |snapshot1-1| in a tree of size |snapshot2|.

	// Verify the first root.
	ch := hashChainer(v)
	mask := (snapshot1 - 1) >> uint(shift) // Start chaining from level |shift|.
	hash1 := ch.chainInnerRight(seed, proof[:inner], mask)
	hash1 = ch.chainBorderRight(hash1, proof[inner:])
	if !bytes.Equal(hash1, root1) {
		return RootMismatchError{
			CalculatedRoot: hash1,
			ExpectedRoot:   root1,
		}
	}

	// Verify the second root.
	hash2 := ch.chainInner(seed, proof[:inner], mask)
	hash2 = ch.chainBorderRight(hash2, proof[inner:])
	if !bytes.Equal(hash2, root2) {
		return RootMismatchError{
			CalculatedRoot: hash2,
			ExpectedRoot:   root2,
		}
	}

	return nil // Proof OK.
}

// VerifiedPrefixHashFromInclusionProof calculates a root hash over leaves
// [0..subSize), based on the inclusion |proof| and |leafHash| for a leaf at
// index |subSize-1| in a tree of the specified |size| with the passed in
// |root| hash.
// Returns an error if the |proof| verification fails. The resulting smaller
// tree's root hash is trusted iff the bigger tree's |root| hash is trusted.
func (v LogVerifier) VerifiedPrefixHashFromInclusionProof(
	subSize, size int64,
	proof [][]byte, root []byte, leafHash []byte,
) ([]byte, error) {
	if subSize <= 0 {
		return nil, fmt.Errorf("subtree size is %d, want > 0", subSize)
	}
	leaf := subSize - 1
	if err := v.VerifyInclusionProof(leaf, size, proof, root, leafHash); err != nil {
		return nil, err
	}

	inner := innerProofSize(leaf, size)
	ch := hashChainer(v)
	res := ch.chainInnerRight(leafHash, proof[:inner], leaf)
	res = ch.chainBorderRight(res, proof[inner:])
	return res, nil
}

// decompInclProof breaks down inclusion proof for a leaf at the specified
// |index| in a tree of the specified |size| into 2 components. The splitting
// point between them is where paths to leaves |index| and |size-1| diverge.
// Returns lengths of the bottom and upper proof parts correspondingly. The sum
// of the two determines the correct length of the inclusion proof.
func decompInclProof(index, size int64) (int, int) {
	inner := innerProofSize(index, size)
	border := bits.OnesCount64(uint64(index) >> uint(inner))
	return inner, border
}

func innerProofSize(index, size int64) int {
	return bits.Len64(uint64(index ^ (size - 1)))
}
