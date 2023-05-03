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
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/sigstore/rekor/pkg/log"
	"github.com/transparency-dev/merkle/proof"
	"github.com/transparency-dev/merkle/rfc6962"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/google/trillian"
	"github.com/google/trillian/client"
	"github.com/google/trillian/types"
)

// TrillianClient provides a wrapper around the Trillian client
type TrillianClient struct {
	client  trillian.TrillianLogClient
	logID   int64
	context context.Context
}

// NewTrillianClient creates a TrillianClient with the given Trillian client and log/tree ID.
func NewTrillianClient(ctx context.Context, logClient trillian.TrillianLogClient, logID int64) TrillianClient {
	return TrillianClient{
		client:  logClient,
		logID:   logID,
		context: ctx,
	}
}

// Response includes a status code, an optional error message, and one of the results based on the API call
type Response struct {
	// Status is the status code of the response
	Status codes.Code
	// Error contains an error on request or client failure
	Err error
	// GetAddResult contains the response from queueing a leaf in Trillian
	GetAddResult *trillian.QueueLeafResponse
	// GetLeafAndProofResult contains the response for fetching an inclusion proof and leaf
	GetLeafAndProofResult *trillian.GetEntryAndProofResponse
	// GetLatestResult contains the response for the latest checkpoint
	GetLatestResult *trillian.GetLatestSignedLogRootResponse
	// GetConsistencyProofResult contains the response for a consistency proof between two log sizes
	GetConsistencyProofResult *trillian.GetConsistencyProofResponse
	// getProofResult contains the response for an inclusion proof fetched by leaf hash
	getProofResult *trillian.GetInclusionProofByHashResponse
}

func unmarshalLogRoot(logRoot []byte) (types.LogRootV1, error) {
	var root types.LogRootV1
	if err := root.UnmarshalBinary(logRoot); err != nil {
		return types.LogRootV1{}, err
	}
	return root, nil
}

func (t *TrillianClient) root() (types.LogRootV1, error) {
	rqst := &trillian.GetLatestSignedLogRootRequest{
		LogId: t.logID,
	}
	resp, err := t.client.GetLatestSignedLogRoot(t.context, rqst)
	if err != nil {
		return types.LogRootV1{}, err
	}
	return unmarshalLogRoot(resp.SignedLogRoot.LogRoot)
}

func (t *TrillianClient) AddLeaf(byteValue []byte) *Response {
	leaf := &trillian.LogLeaf{
		LeafValue: byteValue,
	}
	rqst := &trillian.QueueLeafRequest{
		LogId: t.logID,
		Leaf:  leaf,
	}
	resp, err := t.client.QueueLeaf(t.context, rqst)

	// check for error
	if err != nil || (resp.QueuedLeaf.Status != nil && resp.QueuedLeaf.Status.Code != int32(codes.OK)) {
		return &Response{
			Status:       status.Code(err),
			Err:          err,
			GetAddResult: resp,
		}
	}

	root, err := t.root()
	if err != nil {
		return &Response{
			Status:       status.Code(err),
			Err:          err,
			GetAddResult: resp,
		}
	}
	v := client.NewLogVerifier(rfc6962.DefaultHasher)
	logClient := client.New(t.logID, t.client, v, root)

	waitForInclusion := func(ctx context.Context, leafHash []byte) *Response {
		if logClient.MinMergeDelay > 0 {
			select {
			case <-ctx.Done():
				return &Response{
					Status: codes.DeadlineExceeded,
					Err:    ctx.Err(),
				}
			case <-time.After(logClient.MinMergeDelay):
			}
		}
		for {
			root = *logClient.GetRoot()
			if root.TreeSize >= 1 {
				proofResp := t.getProofByHash(resp.QueuedLeaf.Leaf.MerkleLeafHash)
				// if this call succeeds or returns an error other than "not found", return
				if proofResp.Err == nil || (proofResp.Err != nil && status.Code(proofResp.Err) != codes.NotFound) {
					return proofResp
				}
				// otherwise wait for a root update before trying again
			}

			if _, err := logClient.WaitForRootUpdate(ctx); err != nil {
				return &Response{
					Status: codes.Unknown,
					Err:    err,
				}
			}
		}
	}

	proofResp := waitForInclusion(t.context, resp.QueuedLeaf.Leaf.MerkleLeafHash)
	if proofResp.Err != nil {
		return &Response{
			Status:       status.Code(proofResp.Err),
			Err:          proofResp.Err,
			GetAddResult: resp,
		}
	}

	proofs := proofResp.getProofResult.Proof
	if len(proofs) != 1 {
		err := fmt.Errorf("expected 1 proof from getProofByHash for %v, found %v", hex.EncodeToString(resp.QueuedLeaf.Leaf.MerkleLeafHash), len(proofs))
		return &Response{
			Status:       status.Code(err),
			Err:          err,
			GetAddResult: resp,
		}
	}

	leafIndex := proofs[0].LeafIndex
	leafResp := t.GetLeafAndProofByIndex(leafIndex)
	if leafResp.Err != nil {
		return &Response{
			Status:       status.Code(leafResp.Err),
			Err:          leafResp.Err,
			GetAddResult: resp,
		}
	}

	// overwrite queued leaf that doesn't have index set
	resp.QueuedLeaf.Leaf = leafResp.GetLeafAndProofResult.Leaf

	return &Response{
		Status:       status.Code(err),
		Err:          err,
		GetAddResult: resp,
		// include getLeafAndProofResult for inclusion proof
		GetLeafAndProofResult: leafResp.GetLeafAndProofResult,
	}
}

func (t *TrillianClient) GetLeafAndProofByHash(hash []byte) *Response {
	// get inclusion proof for hash, extract index, then fetch leaf using index
	proofResp := t.getProofByHash(hash)
	if proofResp.Err != nil {
		return &Response{
			Status: status.Code(proofResp.Err),
			Err:    proofResp.Err,
		}
	}

	proofs := proofResp.getProofResult.Proof
	if len(proofs) != 1 {
		err := fmt.Errorf("expected 1 proof from getProofByHash for %v, found %v", hex.EncodeToString(hash), len(proofs))
		return &Response{
			Status: status.Code(err),
			Err:    err,
		}
	}

	return t.GetLeafAndProofByIndex(proofs[0].LeafIndex)
}

func (t *TrillianClient) GetLeafAndProofByIndex(index int64) *Response {
	ctx, cancel := context.WithTimeout(t.context, 20*time.Second)
	defer cancel()

	rootResp := t.GetLatest(0)
	if rootResp.Err != nil {
		return &Response{
			Status: status.Code(rootResp.Err),
			Err:    rootResp.Err,
		}
	}

	root, err := unmarshalLogRoot(rootResp.GetLatestResult.SignedLogRoot.LogRoot)
	if err != nil {
		return &Response{
			Status: status.Code(rootResp.Err),
			Err:    rootResp.Err,
		}
	}

	resp, err := t.client.GetEntryAndProof(ctx,
		&trillian.GetEntryAndProofRequest{
			LogId:     t.logID,
			LeafIndex: index,
			TreeSize:  int64(root.TreeSize),
		})

	if resp != nil && resp.Proof != nil {
		if err := proof.VerifyInclusion(rfc6962.DefaultHasher, uint64(index), root.TreeSize, resp.GetLeaf().MerkleLeafHash, resp.Proof.Hashes, root.RootHash); err != nil {
			return &Response{
				Status: status.Code(err),
				Err:    err,
			}
		}
		return &Response{
			Status: status.Code(err),
			Err:    err,
			GetLeafAndProofResult: &trillian.GetEntryAndProofResponse{
				Proof:         resp.Proof,
				Leaf:          resp.Leaf,
				SignedLogRoot: rootResp.GetLatestResult.SignedLogRoot,
			},
		}
	}

	return &Response{
		Status: status.Code(err),
		Err:    err,
	}
}

func (t *TrillianClient) GetLatest(leafSizeInt int64) *Response {

	ctx, cancel := context.WithTimeout(t.context, 20*time.Second)
	defer cancel()

	resp, err := t.client.GetLatestSignedLogRoot(ctx,
		&trillian.GetLatestSignedLogRootRequest{
			LogId:         t.logID,
			FirstTreeSize: leafSizeInt,
		})

	return &Response{
		Status:          status.Code(err),
		Err:             err,
		GetLatestResult: resp,
	}
}

func (t *TrillianClient) GetConsistencyProof(firstSize, lastSize int64) *Response {

	ctx, cancel := context.WithTimeout(t.context, 20*time.Second)
	defer cancel()

	resp, err := t.client.GetConsistencyProof(ctx,
		&trillian.GetConsistencyProofRequest{
			LogId:          t.logID,
			FirstTreeSize:  firstSize,
			SecondTreeSize: lastSize,
		})

	return &Response{
		Status:                    status.Code(err),
		Err:                       err,
		GetConsistencyProofResult: resp,
	}
}

func (t *TrillianClient) getProofByHash(hashValue []byte) *Response {
	ctx, cancel := context.WithTimeout(t.context, 20*time.Second)
	defer cancel()

	rootResp := t.GetLatest(0)
	if rootResp.Err != nil {
		return &Response{
			Status: status.Code(rootResp.Err),
			Err:    rootResp.Err,
		}
	}
	root, err := unmarshalLogRoot(rootResp.GetLatestResult.SignedLogRoot.LogRoot)
	if err != nil {
		return &Response{
			Status: status.Code(rootResp.Err),
			Err:    rootResp.Err,
		}
	}

	// issue 1308: if the tree is empty, there's no way we can return a proof
	if root.TreeSize == 0 {
		return &Response{
			Status: codes.NotFound,
			Err:    status.Error(codes.NotFound, "tree is empty"),
		}
	}

	resp, err := t.client.GetInclusionProofByHash(ctx,
		&trillian.GetInclusionProofByHashRequest{
			LogId:    t.logID,
			LeafHash: hashValue,
			TreeSize: int64(root.TreeSize),
		})

	if resp != nil {
		v := client.NewLogVerifier(rfc6962.DefaultHasher)
		for _, proof := range resp.Proof {
			if err := v.VerifyInclusionByHash(&root, hashValue, proof); err != nil {
				return &Response{
					Status: status.Code(err),
					Err:    err,
				}
			}
		}
		// Return an inclusion proof response with the requested
		return &Response{
			Status: status.Code(err),
			Err:    err,
			getProofResult: &trillian.GetInclusionProofByHashResponse{
				Proof:         resp.Proof,
				SignedLogRoot: rootResp.GetLatestResult.SignedLogRoot,
			},
		}
	}

	return &Response{
		Status: status.Code(err),
		Err:    err,
	}
}

func CreateAndInitTree(ctx context.Context, adminClient trillian.TrillianAdminClient, logClient trillian.TrillianLogClient) (*trillian.Tree, error) {
	t, err := adminClient.CreateTree(ctx, &trillian.CreateTreeRequest{
		Tree: &trillian.Tree{
			TreeType:        trillian.TreeType_LOG,
			TreeState:       trillian.TreeState_ACTIVE,
			MaxRootDuration: durationpb.New(time.Hour),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create tree: %w", err)
	}

	if err := client.InitLog(ctx, t, logClient); err != nil {
		return nil, fmt.Errorf("init log: %w", err)
	}
	log.Logger.Infof("Created new tree with ID: %v", t.TreeId)
	return t, nil
}
