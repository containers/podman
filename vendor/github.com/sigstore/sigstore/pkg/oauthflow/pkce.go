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

package oauthflow

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	PKCES256 = "S256"
)

type PKCE struct {
	Challenge string
	Method    string
	Value     string
}

func NewPKCE(provider *oidc.Provider) (*PKCE, error) {
	var providerClaims struct {
		CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
	}

	if err := provider.Claims(&providerClaims); err != nil {
		// will only error out if the JSON was malformed, which shouldn't happen at this point
		return nil, err
	}

	var chosenMethod string
	for _, method := range providerClaims.CodeChallengeMethodsSupported {
		// per RFC7636, any server that supports PKCE must support S256
		if method == PKCES256 && chosenMethod != PKCES256 {
			chosenMethod = PKCES256
			break
		} else if method != "plain" {
			fmt.Printf("Unsupported code challenge method in list: '%v'", method)
		}
	}
	if chosenMethod == "" {
		return nil, fmt.Errorf("PKCE is not supported by OIDC provider '%v'", provider.Endpoint().AuthURL)
	}

	// we use two 27 character strings to meet requirements of RFC 7636:
	// (minimum length of 43 characters and a maximum length of 128 characters)
	value := randStr() + randStr()

	h := sha256.New()
	_, _ = h.Write([]byte(value))
	challenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return &PKCE{
		Challenge: challenge,
		Method:    chosenMethod,
		Value:     value,
	}, nil
}

func (p *PKCE) AuthURLOpts() []oauth2.AuthCodeOption {
	return []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge_method", string(p.Method)),
		oauth2.SetAuthURLParam("code_challenge", p.Challenge),
	}
}

func (p *PKCE) TokenURLOpts() []oauth2.AuthCodeOption {
	return []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", p.Value),
	}
}
