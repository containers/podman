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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2"
)

const htmlPage = `<html>
<title>Sigstore Auth</title>
<body>
<h1>Sigstore Auth Successful</h1>
<p>You may now close this page.</p>
</body>
</html>
`

type TokenGetter interface {
	GetIDToken(provider *oidc.Provider, config oauth2.Config) (*OIDCIDToken, error)
}

type OIDCIDToken struct {
	RawString string
	Subject   string
}

// DefaultIDTokenGetter is the default implementation.
// The HTML page and message printed to the terminal can be customized.
var DefaultIDTokenGetter = &InteractiveIDTokenGetter{
	MessagePrinter: func(url string) { fmt.Fprintf(os.Stderr, "Your browser will now be opened to:\n%s\n", url) },
	HTMLPage:       htmlPage,
}

func OIDConnect(issuer string, id string, secret string, tg TokenGetter) (*OIDCIDToken, error) {
	provider, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		return nil, err
	}
	config := oauth2.Config{
		ClientID:     id,
		ClientSecret: secret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  "http://localhost:5556/auth/callback",
		Scopes:       []string{oidc.ScopeOpenID, "email"},
	}

	return tg.GetIDToken(provider, config)
}

type claims struct {
	Email    string `json:"email"`
	Verified bool   `json:"email_verified"`
	Subject  string `json:"sub"`
}

func SubjectFromToken(tok *oidc.IDToken) (string, error) {
	claims := claims{}
	if err := tok.Claims(&claims); err != nil {
		return "", err
	}
	return subjectFromClaims(claims)
}

func subjectFromClaims(c claims) (string, error) {
	if c.Email != "" {
		if !c.Verified {
			return "", errors.New("not verified by identity provider")
		}
		return c.Email, nil
	}

	if c.Subject == "" {
		return "", errors.New("no subject found in claims")
	}
	return c.Subject, nil
}

type StaticTokenGetter struct {
	RawToken string
}

func (stg *StaticTokenGetter) GetIDToken(_ *oidc.Provider, _ oauth2.Config) (*OIDCIDToken, error) {
	unsafeTok, err := jose.ParseSigned(stg.RawToken)
	if err != nil {
		return nil, err
	}
	// THIS LOGIC IS GENERALLY UNSAFE BUT OK HERE
	// We are only parsing the id-token passed directly to a command line tool by a user, so it is trusted locally.
	// We need to extract the email address to attach an additional signed proof to the server.
	// THE SERVER WILL DO REAL VERIFICATION HERE
	unsafePayload := unsafeTok.UnsafePayloadWithoutVerification()
	claims := claims{}
	if err := json.Unmarshal(unsafePayload, &claims); err != nil {
		return nil, err
	}

	subj, err := subjectFromClaims(claims)
	if err != nil {
		return nil, err
	}

	return &OIDCIDToken{
		RawString: stg.RawToken,
		Subject:   subj,
	}, nil
}
