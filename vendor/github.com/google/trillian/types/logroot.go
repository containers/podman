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

// Package types defines serialization and parsing functions for SignedLogRoot
// fields.
package types

import (
	"encoding/binary"
	"fmt"

	"github.com/google/trillian/types/internal/tls"

	"github.com/google/trillian"
)

// LogRootV1 holds the TLS-deserialization of the following structure
// (described in RFC5246 section 4 notation):
//
//	struct {
//	  uint64 tree_size;
//	  opaque root_hash<0..128>;
//	  uint64 timestamp_nanos;
//	  uint64 revision;
//	  opaque metadata<0..65535>;
//	} LogRootV1;
type LogRootV1 struct {
	// TreeSize is the number of leaves in the log Merkle tree.
	TreeSize uint64
	// RootHash is the hash of the root node of the tree.
	RootHash []byte `tls:"minlen:0,maxlen:128"`
	// TimestampNanos is the time in nanoseconds for when this root was created,
	// counting from the UNIX epoch.
	TimestampNanos uint64

	// Revision is the Merkle tree revision associated with this root.
	//
	// Deprecated: Revision is a concept internal to the storage layer.
	Revision uint64

	// Metadata holds additional data associated with this root.
	Metadata []byte `tls:"minlen:0,maxlen:65535"`
}

// LogRoot holds the TLS-deserialization of the following structure
// (described in RFC5246 section 4 notation):
// enum { v1(1), (65535)} Version;
//
//	struct {
//	  Version version;
//	  select(version) {
//	    case v1: LogRootV1;
//	  }
//	} LogRoot;
type LogRoot struct {
	Version tls.Enum   `tls:"size:2"`
	V1      *LogRootV1 `tls:"selector:Version,val:1"`
}

// UnmarshalBinary verifies that logRootBytes is a TLS serialized LogRoot, has
// the LOG_ROOT_FORMAT_V1 tag, and populates the caller with the deserialized
// *LogRootV1.
func (l *LogRootV1) UnmarshalBinary(logRootBytes []byte) error {
	if len(logRootBytes) < 3 {
		return fmt.Errorf("logRootBytes too short")
	}
	if l == nil {
		return fmt.Errorf("nil log root")
	}
	version := binary.BigEndian.Uint16(logRootBytes)
	if version != uint16(trillian.LogRootFormat_LOG_ROOT_FORMAT_V1) {
		return fmt.Errorf("invalid LogRoot.Version: %v, want %v",
			version, trillian.LogRootFormat_LOG_ROOT_FORMAT_V1)
	}

	var logRoot LogRoot
	if _, err := tls.Unmarshal(logRootBytes, &logRoot); err != nil {
		return err
	}

	*l = *logRoot.V1
	return nil
}

// MarshalBinary returns a canonical TLS serialization of LogRoot.
func (l *LogRootV1) MarshalBinary() ([]byte, error) {
	return tls.Marshal(LogRoot{
		Version: tls.Enum(trillian.LogRootFormat_LOG_ROOT_FORMAT_V1),
		V1:      l,
	})
}
