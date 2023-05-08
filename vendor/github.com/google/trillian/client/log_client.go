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

// Package client verifies responses from the Trillian log.
package client

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/trillian"
	"github.com/google/trillian/client/backoff"
	"github.com/google/trillian/types"
	"github.com/transparency-dev/merkle"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LogClient represents a client for a given Trillian log instance.
type LogClient struct {
	*LogVerifier
	LogID         int64
	MinMergeDelay time.Duration
	client        trillian.TrillianLogClient
	root          types.LogRootV1
	rootLock      sync.Mutex
	updateLock    sync.Mutex
}

// New returns a new LogClient.
func New(logID int64, client trillian.TrillianLogClient, verifier *LogVerifier, root types.LogRootV1) *LogClient {
	return &LogClient{
		LogVerifier: verifier,
		LogID:       logID,
		client:      client,
		root:        root,
	}
}

// NewFromTree creates a new LogClient given a tree config.
func NewFromTree(client trillian.TrillianLogClient, config *trillian.Tree, root types.LogRootV1) (*LogClient, error) {
	verifier, err := NewLogVerifierFromTree(config)
	if err != nil {
		return nil, err
	}

	return New(config.GetTreeId(), client, verifier, root), nil
}

// AddLeaf adds leaf to the append only log.
// Blocks and continuously updates the trusted root until a successful inclusion proof
// can be retrieved.
func (c *LogClient) AddLeaf(ctx context.Context, data []byte) error {
	if err := c.QueueLeaf(ctx, data); err != nil {
		return fmt.Errorf("QueueLeaf(): %v", err)
	}
	if err := c.WaitForInclusion(ctx, data); err != nil {
		return fmt.Errorf("WaitForInclusion(): %v", err)
	}
	return nil
}

// ListByIndex returns the requested leaves by index.
func (c *LogClient) ListByIndex(ctx context.Context, start, count int64) ([]*trillian.LogLeaf, error) {
	resp, err := c.client.GetLeavesByRange(ctx,
		&trillian.GetLeavesByRangeRequest{
			LogId:      c.LogID,
			StartIndex: start,
			Count:      count,
		})
	if err != nil {
		return nil, err
	}
	// Verify that we got back the requested leaves.
	if len(resp.Leaves) < int(count) {
		return nil, fmt.Errorf("len(Leaves)=%d, want %d", len(resp.Leaves), count)
	}
	for i, l := range resp.Leaves {
		if want := start + int64(i); l.LeafIndex != want {
			return nil, fmt.Errorf("Leaves[%d].LeafIndex=%d, want %d", i, l.LeafIndex, want)
		}
	}

	return resp.Leaves, nil
}

// WaitForRootUpdate repeatedly fetches the latest root until there is an
// update, which it then applies, or until ctx times out.
func (c *LogClient) WaitForRootUpdate(ctx context.Context) (*types.LogRootV1, error) {
	b := &backoff.Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	for {
		newTrusted, err := c.UpdateRoot(ctx)
		switch status.Code(err) {
		case codes.OK:
			if newTrusted != nil {
				return newTrusted, nil
			}
		case codes.Unavailable, codes.NotFound, codes.FailedPrecondition:
			// Retry.
		default:
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, status.Errorf(codes.DeadlineExceeded, "%v", ctx.Err())
		case <-time.After(b.Duration()):
		}
	}
}

// getAndVerifyLatestRoot fetches and verifies the latest root against a trusted root, seen in the past.
// Pass nil for trusted if this is the first time querying this log.
func (c *LogClient) getAndVerifyLatestRoot(ctx context.Context, trusted *types.LogRootV1) (*types.LogRootV1, error) {
	resp, err := c.client.GetLatestSignedLogRoot(ctx,
		&trillian.GetLatestSignedLogRootRequest{
			LogId:         c.LogID,
			FirstTreeSize: int64(trusted.TreeSize),
		})
	if err != nil {
		return nil, err
	}

	// TODO(gbelvin): Turn on root verification.
	/*
		logRoot, err := c.VerifyRoot(&types.LogRootV1{}, resp.GetSignedLogRoot(), nil)
		if err != nil {
			return nil, err
		}
	*/
	// TODO(gbelvin): Remove this hack when all implementations store digital signatures.
	var logRoot types.LogRootV1
	if err := logRoot.UnmarshalBinary(resp.GetSignedLogRoot().LogRoot); err != nil {
		return nil, err
	}

	if trusted.TreeSize > 0 &&
		logRoot.TreeSize == trusted.TreeSize &&
		bytes.Equal(logRoot.RootHash, trusted.RootHash) {
		// Tree has not been updated.
		return &logRoot, nil
	}

	// Verify root update if the tree / the latest signed log root isn't empty.
	if logRoot.TreeSize > 0 {
		if _, err := c.VerifyRoot(trusted, resp.GetSignedLogRoot(), resp.GetProof().GetHashes()); err != nil {
			return nil, err
		}
	}
	return &logRoot, nil
}

// GetRoot returns a copy of the latest trusted root.
func (c *LogClient) GetRoot() *types.LogRootV1 {
	c.rootLock.Lock()
	defer c.rootLock.Unlock()

	// Copy the internal trusted root in order to prevent clients from modifying it.
	ret := c.root
	return &ret
}

// UpdateRoot retrieves the current SignedLogRoot, verifying it against roots this client has
// seen in the past, and updating the currently trusted root if the new root verifies, and is
// newer than the currently trusted root.
func (c *LogClient) UpdateRoot(ctx context.Context) (*types.LogRootV1, error) {
	// Only one root update should be running at any point in time, because
	// the update involves a consistency proof from the old value, and if the
	// old value could change along the way (in another goroutine) then the
	// result could be inconsistent.
	//
	// For example, if the current root is A and two root updates A->B and A->C
	// happen in parallel, then we might end up with the transitions A->B->C:
	//     cur := A            cur := A
	//    getRoot() => B      getRoot() => C
	//    proof(A->B) ok      proof(A->C) ok
	//    c.root = B
	//                        c.root = C
	// and the last step (B->C) has no proof and so could hide a forked tree.
	c.updateLock.Lock()
	defer c.updateLock.Unlock()

	currentlyTrusted := c.GetRoot()
	newTrusted, err := c.getAndVerifyLatestRoot(ctx, currentlyTrusted)
	if err != nil {
		return nil, err
	}

	// Lock "rootLock" for the "root" update.
	c.rootLock.Lock()
	defer c.rootLock.Unlock()

	if newTrusted.TimestampNanos > currentlyTrusted.TimestampNanos &&
		newTrusted.TreeSize >= currentlyTrusted.TreeSize {

		// Take a copy of the new trusted root in order to prevent clients from modifying it.
		c.root = *newTrusted

		return newTrusted, nil
	}

	return nil, nil
}

// WaitForInclusion blocks until the requested data has been verified with an
// inclusion proof.
//
// It will continuously update the root to the latest one available until the
// data is found, or an error is returned.
//
// It is best to call this method with a context that will timeout to avoid
// waiting forever.
func (c *LogClient) WaitForInclusion(ctx context.Context, data []byte) error {
	leaf := prepareLeaf(c.hasher, data)

	// If a minimum merge delay has been configured, wait at least that long before
	// starting to poll
	if c.MinMergeDelay > 0 {
		select {
		case <-ctx.Done():
			return status.Errorf(codes.DeadlineExceeded, "%v", ctx.Err())
		case <-time.After(c.MinMergeDelay):
		}
	}

	var root *types.LogRootV1
	for {
		root = c.GetRoot()

		// It is illegal to ask for an inclusion proof with TreeSize = 0.
		if root.TreeSize >= 1 {
			ok, err := c.getAndVerifyInclusionProof(ctx, leaf.MerkleLeafHash, root)
			if err != nil && status.Code(err) != codes.NotFound {
				return err
			} else if ok {
				return nil
			}
		}

		// If not found or tree is empty, wait for a root update before retrying again.
		if _, err := c.WaitForRootUpdate(ctx); err != nil {
			return err
		}

		// Retry
	}
}

func (c *LogClient) getAndVerifyInclusionProof(ctx context.Context, leafHash []byte, sth *types.LogRootV1) (bool, error) {
	resp, err := c.client.GetInclusionProofByHash(ctx,
		&trillian.GetInclusionProofByHashRequest{
			LogId:    c.LogID,
			LeafHash: leafHash,
			TreeSize: int64(sth.TreeSize),
		})
	if err != nil {
		return false, err
	}
	if len(resp.Proof) < 1 {
		return false, nil
	}
	for _, proof := range resp.Proof {
		if err := c.VerifyInclusionByHash(sth, leafHash, proof); err != nil {
			return false, fmt.Errorf("VerifyInclusionByHash(): %v", err)
		}
	}
	return true, nil
}

// AddSequencedLeaves adds any number of pre-sequenced leaves to the log.
// Indexes must be contiguous.
func (c *LogClient) AddSequencedLeaves(ctx context.Context, dataByIndex map[int64][]byte) error {
	if len(dataByIndex) == 0 {
		return nil
	}
	leaves := make([]*trillian.LogLeaf, 0, len(dataByIndex))
	indexes := make([]int64, 0, len(dataByIndex))
	for index := range dataByIndex {
		indexes = append(indexes, index)
	}
	sort.Slice(indexes, func(a, b int) bool { return indexes[a] < indexes[b] })

	for i, index := range indexes {
		// Check index continuity.
		if want := indexes[0] + int64(i); index != want {
			return fmt.Errorf("missing index in contiugous index range. got: %v, want: %v", index, want)
		}
		leaf := prepareLeaf(c.hasher, dataByIndex[index])
		leaf.LeafIndex = index
		leaves = append(leaves, leaf)
	}
	resp, err := c.client.AddSequencedLeaves(ctx, &trillian.AddSequencedLeavesRequest{
		LogId:  c.LogID,
		Leaves: leaves,
	})
	for _, leaf := range resp.GetResults() {
		if s := status.FromProto(leaf.GetStatus()); s.Code() != codes.OK && s.Code() != codes.AlreadyExists {
			return status.Errorf(s.Code(), "unexpected fail status in AddSequencedLeaves: %+v, err: %v", leaf, s.Message())
		}
	}
	return err
}

// QueueLeaf adds a leaf to a Trillian log without blocking.
// AlreadyExists is considered a success case by this function.
func (c *LogClient) QueueLeaf(ctx context.Context, data []byte) error {
	leaf := prepareLeaf(c.hasher, data)
	_, err := c.client.QueueLeaf(ctx, &trillian.QueueLeafRequest{
		LogId: c.LogID,
		Leaf:  leaf,
	})
	return err
}

// prepareLeaf returns a trillian.LogLeaf prepopulated with leaf data and hash.
func prepareLeaf(hasher merkle.LogHasher, data []byte) *trillian.LogLeaf {
	leafHash := hasher.HashLeaf(data)
	return &trillian.LogLeaf{
		LeafValue:      data,
		MerkleLeafHash: leafHash,
	}
}
