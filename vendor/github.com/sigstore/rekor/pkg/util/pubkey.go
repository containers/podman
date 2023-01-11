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
	"crypto/ecdsa"
	"errors"

	"github.com/sigstore/rekor/pkg/generated/client"
	"github.com/sigstore/rekor/pkg/generated/client/pubkey"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

func PublicKey(ctx context.Context, c *client.Rekor) (*ecdsa.PublicKey, error) {
	resp, err := c.Pubkey.GetPublicKey(&pubkey.GetPublicKeyParams{Context: ctx})
	if err != nil {
		return nil, err
	}

	// marshal the pubkey
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(resp.GetPayload()))
	if err != nil {
		return nil, err
	}
	ed, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key retrieved from Rekor is not an ECDSA key")
	}
	return ed, nil
}
