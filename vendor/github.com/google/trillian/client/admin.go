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

package client

import (
	"context"
	"fmt"
	"time"

	"github.com/google/trillian"
	"github.com/google/trillian/client/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// CreateAndInitTree uses the adminClient and logClient to create the tree
// described by req.
// If req describes a LOG tree, then this function will also call the InitLog
// function using logClient.
// Internally, the function will continue to retry failed requests until either
// the tree is created (and if necessary, initialised) successfully, or ctx is
// cancelled.
func CreateAndInitTree(
	ctx context.Context,
	req *trillian.CreateTreeRequest,
	adminClient trillian.TrillianAdminClient,
	logClient trillian.TrillianLogClient) (*trillian.Tree, error) {
	b := &backoff.Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	var tree *trillian.Tree
	err := b.Retry(ctx, func() error {
		klog.Info("CreateTree...")
		var err error
		tree, err = adminClient.CreateTree(ctx, req)
		switch code := status.Code(err); code {
		case codes.Unavailable:
			klog.Errorf("Admin server unavailable: %v", err)
			return err
		case codes.OK:
			return nil
		default:
			klog.Errorf("failed to CreateTree(%+v): %T %v", req, err, err)
			return err
		}
	})
	if err != nil {
		return nil, err
	}

	switch tree.TreeType {
	case trillian.TreeType_LOG, trillian.TreeType_PREORDERED_LOG:
		if err := InitLog(ctx, tree, logClient); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("don't know how or whether to initialise tree type %v", tree.TreeType)
	}

	return tree, nil
}

// InitLog initialises a freshly created Log tree.
func InitLog(ctx context.Context, tree *trillian.Tree, logClient trillian.TrillianLogClient) error {
	if tree.TreeType != trillian.TreeType_LOG &&
		tree.TreeType != trillian.TreeType_PREORDERED_LOG {
		return fmt.Errorf("InitLog called with tree of type %v", tree.TreeType)
	}

	b := &backoff.Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	err := b.Retry(ctx, func() error {
		klog.Infof("Initialising Log %v...", tree.TreeId)
		req := &trillian.InitLogRequest{LogId: tree.TreeId}
		resp, err := logClient.InitLog(ctx, req)
		switch code := status.Code(err); code {
		case codes.Unavailable:
			klog.Errorf("Log server unavailable: %v", err)
			return err
		case codes.AlreadyExists:
			klog.Warningf("Bizarrely, the just-created Log (%v) is already initialised!: %v", tree.TreeId, err)
			return err
		case codes.OK:
			klog.Infof("Initialised Log (%v) with new SignedTreeHead:\n%+v",
				tree.TreeId, resp.Created)
			return nil
		default:
			klog.Errorf("failed to InitLog(%+v): %T %v", req, err, err)
			return err
		}
	})
	if err != nil {
		return err
	}

	// Wait for log root to become available.
	return b.Retry(ctx, func() error {
		_, err := logClient.GetLatestSignedLogRoot(ctx,
			&trillian.GetLatestSignedLogRootRequest{LogId: tree.TreeId})
		return err
	}, codes.FailedPrecondition)
}
