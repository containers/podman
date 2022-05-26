// Copyright 2014 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"context"
	"io"
	"net/url"
	"strings"

	"cloud.google.com/go/internal"
	gax "github.com/googleapis/gax-go/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// runWithRetry calls the function until it returns nil or a non-retryable error, or
// the context is done.
func runWithRetry(ctx context.Context, call func() error) error {
	return internal.Retry(ctx, gax.Backoff{}, func() (stop bool, err error) {
		err = call()
		if err == nil {
			return true, nil
		}
		if shouldRetry(err) {
			return false, err
		}
		return true, err
	})
}

func shouldRetry(err error) bool {
	if err == io.ErrUnexpectedEOF {
		return true
	}
	switch e := err.(type) {
	case *googleapi.Error:
		// Retry on 429 and 5xx, according to
		// https://cloud.google.com/storage/docs/exponential-backoff.
		return e.Code == 429 || (e.Code >= 500 && e.Code < 600)
	case *url.Error:
		// Retry socket-level errors ECONNREFUSED and ENETUNREACH (from syscall).
		// Unfortunately the error type is unexported, so we resort to string
		// matching.
		retriable := []string{"connection refused", "connection reset"}
		for _, s := range retriable {
			if strings.Contains(e.Error(), s) {
				return true
			}
		}
	case interface{ Temporary() bool }:
		if e.Temporary() {
			return true
		}
	}
	// HTTP 429, 502, 503, and 504 all map to gRPC UNAVAILABLE per
	// https://grpc.github.io/grpc/core/md_doc_http-grpc-status-mapping.html.
	//
	// This is only necessary for the experimental gRPC-based media operations.
	if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
		return true
	}
	// Unwrap is only supported in go1.13.x+
	if e, ok := err.(interface{ Unwrap() error }); ok {
		return shouldRetry(e.Unwrap())
	}
	return false
}
