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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	/* #nosec */
	SigstoreDeviceURL = "https://oauth2.sigstore.dev/auth/device/code"
	/* #nosec */
	SigstoreTokenURL = "https://oauth2.sigstore.dev/auth/device/token"
)

type deviceResp struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
}

type tokenResp struct {
	IDToken string `json:"id_token"`
	Error   string `json:"error"`
}

type DeviceFlowTokenGetter struct {
	MessagePrinter func(string)
	Sleeper        func(time.Duration)
	Issuer         string
	CodeURL        string
	TokenURL       string
}

func NewDeviceFlowTokenGetter(issuer, codeURL, tokenURL string) *DeviceFlowTokenGetter {
	return &DeviceFlowTokenGetter{
		MessagePrinter: func(s string) { fmt.Println(s) },
		Sleeper:        time.Sleep,
		Issuer:         issuer,
		CodeURL:        codeURL,
		TokenURL:       tokenURL,
	}
}

func (d *DeviceFlowTokenGetter) deviceFlow(clientID string) (string, error) {
	data := url.Values{
		"client_id": []string{clientID},
		"scope":     []string{"openid email"},
	}

	/* #nosec */
	resp, err := http.PostForm(d.CodeURL, data)
	if err != nil {
		return "", err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	parsed := deviceResp{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", err
	}
	uri := parsed.VerificationURIComplete
	if uri == "" {
		uri = parsed.VerificationURI
	}
	d.MessagePrinter(fmt.Sprintf("Enter the verification code %s in your browser at: %s", parsed.UserCode, uri))
	d.MessagePrinter(fmt.Sprintf("Code will be valid for %d seconds", parsed.ExpiresIn))

	for {
		// Some providers use a secret here, we don't need for sigstore oauth one so leave it off.
		data := url.Values{
			"grant_type":  []string{"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": []string{parsed.DeviceCode},
			"scope":       []string{"openid", "email"},
		}

		/* #nosec */
		resp, err := http.PostForm(d.TokenURL, data)
		if err != nil {
			return "", err
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		tr := tokenResp{}
		if err := json.Unmarshal(b, &tr); err != nil {
			return "", err
		}

		if tr.IDToken != "" {
			d.MessagePrinter("Token received!")
			return tr.IDToken, nil
		}
		switch tr.Error {
		case "access_denied", "expired_token":
			return "", fmt.Errorf("error obtaining token: %s", tr.Error)
		case "authorization_pending":
			d.Sleeper(time.Duration(parsed.Interval) * time.Second)
		case "slow_down":
			// Add ten seconds if we got told to slow down
			d.Sleeper(time.Duration(parsed.Interval)*time.Second + 10*time.Second)
		default:
			return "", fmt.Errorf("unexpected error in device flow: %s", tr.Error)
		}
	}
}

func (d *DeviceFlowTokenGetter) GetIDToken(p *oidc.Provider, cfg oauth2.Config) (*OIDCIDToken, error) {
	idToken, err := d.deviceFlow(cfg.ClientID)
	if err != nil {
		return nil, err
	}
	verifier := p.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	parsedIDToken, err := verifier.Verify(context.Background(), idToken)
	if err != nil {
		return nil, err
	}

	subj, err := SubjectFromToken(parsedIDToken)
	if err != nil {
		return nil, err
	}

	return &OIDCIDToken{
		RawString: idToken,
		Subject:   subj,
	}, nil
}
