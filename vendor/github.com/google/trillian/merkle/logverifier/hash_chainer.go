// Copyright 2018 Google LLC. All Rights Reserved.
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
	"github.com/google/trillian/merkle/hashers"
)

// hashChainer provides convenience methods for hashing subranges of Merkle
// Tree proofs to obtain (sub-)tree hashes. Depending on how the path to a tree
// node relates to the query and/or tree borders, different methods are there.
//
// TODO(pavelkalinnikov): Add a Merkle Trees doc with visual explanations.
type hashChainer struct {
	hasher hashers.LogHasher
}

// chainInner computes a subtree hash for a node on or below the tree's right
// border. Assumes |proof| hashes are ordered from lower levels to upper, and
// |seed| is the initial subtree/leaf hash on the path located at the specified
// |index| on its level.
func (c hashChainer) chainInner(seed []byte, proof [][]byte, index int64) []byte {
	for i, h := range proof {
		if (index>>uint(i))&1 == 0 {
			seed = c.hasher.HashChildren(seed, h)
		} else {
			seed = c.hasher.HashChildren(h, seed)
		}
	}
	return seed
}

// chainInnerRight computes a subtree hash like chainInner, but only takes
// hashes to the left from the path into consideration, which effectively means
// the result is a hash of the corresponding earlier version of this subtree.
func (c hashChainer) chainInnerRight(seed []byte, proof [][]byte, index int64) []byte {
	for i, h := range proof {
		if (index>>uint(i))&1 == 1 {
			seed = c.hasher.HashChildren(h, seed)
		}
	}
	return seed
}

// chainBorderRight chains proof hashes along tree borders. This differs from
// inner chaining because |proof| contains only left-side subtree hashes.
func (c hashChainer) chainBorderRight(seed []byte, proof [][]byte) []byte {
	for _, h := range proof {
		seed = c.hasher.HashChildren(h, seed)
	}
	return seed
}
