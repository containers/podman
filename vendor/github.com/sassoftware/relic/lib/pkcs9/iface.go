// Copyright © SAS Institute Inc.
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

package pkcs9

import (
	"context"
	"crypto"

	"github.com/sassoftware/relic/lib/pkcs7"
)

// Timestamper is the common interface for the timestamp client and middleware
type Timestamper interface {
	Timestamp(ctx context.Context, req *Request) (*pkcs7.ContentInfoSignedData, error)
}

// Request holds parameters for a timestamp operation
type Request struct {
	// EncryptedDigest is the raw encrypted signature value
	EncryptedDigest []byte
	// Hash is the desired hash function for the timestamp. Ignored for legacy requests.
	Hash crypto.Hash
	// Legacy indicates a nonstandard microsoft timestamp request, otherwise RFC 3161 is used
	Legacy bool
}
