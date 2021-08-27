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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/segmentio/ksuid"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
)

const (
	oobRedirectURI = "urn:ietf:wg:oauth:2.0:oob"
)

// InteractiveIDTokenGetter is a type to get ID tokens for oauth flows
type InteractiveIDTokenGetter struct {
	MessagePrinter func(url string)
	HTMLPage       string
}

func (i *InteractiveIDTokenGetter) GetIDToken(p *oidc.Provider, cfg oauth2.Config) (*OIDCIDToken, error) {
	redirectURL, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return nil, err
	}

	// generate random fields and save them for comparison after OAuth2 dance
	stateToken := randStr()
	nonce := randStr()

	// require that OIDC provider support PKCE to provide sufficient security for the CLI
	pkce, err := NewPKCE(p)
	if err != nil {
		return nil, err
	}

	opts := append(pkce.AuthURLOpts(), oauth2.AccessTypeOnline, oidc.Nonce(nonce))
	authCodeURL := cfg.AuthCodeURL(stateToken, opts...)
	var code string
	if err := open.Run(authCodeURL); err != nil {
		// Swap to the out of band flow if we can't open the browser
		fmt.Fprintf(os.Stderr, "error opening browser: %v\n", err)
		code = doOobFlow(cfg, stateToken, opts)
	} else {
		fmt.Fprintf(os.Stderr, "Your browser will now be opened to:\n%s\n", authCodeURL)
		code, err = getCodeFromLocalServer(stateToken, redirectURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error getting code from local server: %v\n", err)
			code = doOobFlow(cfg, stateToken, opts)
		}
	}
	token, err := cfg.Exchange(context.Background(), code, append(pkce.TokenURLOpts(), oidc.Nonce(nonce))...)
	if err != nil {
		return nil, err
	}

	// requesting 'openid' scope should ensure an id_token is given when exchanging the code for an access token
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("id_token not present")
	}

	// verify nonce, client ID, access token hash before using it
	verifier := p.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	parsedIDToken, err := verifier.Verify(context.Background(), idToken)
	if err != nil {
		return nil, err
	}
	if parsedIDToken.Nonce != nonce {
		return nil, errors.New("nonce does not match value sent")
	}
	if parsedIDToken.AccessTokenHash != "" {
		if err := parsedIDToken.VerifyAccessToken(token.AccessToken); err != nil {
			return nil, err
		}
	}

	email, err := SubjectFromToken(parsedIDToken)
	if err != nil {
		return nil, err
	}

	returnToken := OIDCIDToken{
		RawString: idToken,
		Subject:   email,
	}
	return &returnToken, nil
}

func doOobFlow(cfg oauth2.Config, stateToken string, opts []oauth2.AuthCodeOption) string {
	cfg.RedirectURL = oobRedirectURI
	authURL := cfg.AuthCodeURL(stateToken, opts...)
	fmt.Fprintln(os.Stderr, "Go to the following link in a browser:\n\n\t", authURL)
	fmt.Fprintf(os.Stderr, "Enter verification code: ")
	var code string
	fmt.Scanln(&code)
	return code
}

func getCodeFromLocalServer(state string, redirectURL *url.URL) (string, error) {
	doneCh := make(chan string)
	errCh := make(chan error)
	m := http.NewServeMux()
	s := http.Server{
		Addr:    redirectURL.Host,
		Handler: m,
	}
	defer func() {
		go func() {
			_ = s.Shutdown(context.Background())
		}()
	}()

	go func() {
		m.HandleFunc(redirectURL.Path, func(w http.ResponseWriter, r *http.Request) {
			// even though these are fetched from the FormValue method,
			// these are supplied as query parameters
			if r.FormValue("state") != state {
				errCh <- errors.New("invalid state token")
				return
			}
			fmt.Fprint(w, htmlPage)
			doneCh <- r.FormValue("code")
		})
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	timeoutCh := time.NewTimer(120 * time.Second)
	select {
	case code := <-doneCh:
		return code, nil
	case err := <-errCh:
		return "", err
	case <-timeoutCh.C:
		return "", errors.New("timeout")
	}
}

func randStr() string {
	// we use ksuid here to ensure we get globally unique values to mitigate
	// risk of replay attacks

	// output is a 27 character base62 string which is by default URL-safe
	return ksuid.New().String()
}
