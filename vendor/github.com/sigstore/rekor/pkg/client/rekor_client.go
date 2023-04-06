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

package client

import (
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/hashicorp/go-cleanhttp"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/sigstore/rekor/pkg/generated/client"
	"github.com/sigstore/rekor/pkg/util"
)

func GetRekorClient(rekorServerURL string, opts ...Option) (*client.Rekor, error) {
	url, err := url.Parse(rekorServerURL)
	if err != nil {
		return nil, err
	}
	o := makeOptions(opts...)

	retryableClient := retryablehttp.NewClient()
	defaultTransport := cleanhttp.DefaultTransport()
	if o.InsecureTLS {
		/* #nosec G402 */
		defaultTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	retryableClient.HTTPClient = &http.Client{
		Transport: defaultTransport,
	}
	retryableClient.RetryMax = int(o.RetryCount)
	retryableClient.Logger = o.Logger

	httpClient := retryableClient.StandardClient()
	httpClient.Transport = createRoundTripper(httpClient.Transport, o)

	// sanitize path
	if url.Path == "" {
		url.Path = client.DefaultBasePath
	}

	rt := httptransport.NewWithClient(url.Host, url.Path, []string{url.Scheme}, httpClient)
	rt.Consumers["application/json"] = runtime.JSONConsumer()
	rt.Consumers["application/x-pem-file"] = runtime.TextConsumer()
	rt.Producers["application/json"] = runtime.JSONProducer()

	registry := strfmt.Default
	registry.Add("signedCheckpoint", &util.SignedNote{}, util.SignedCheckpointValidator)
	return client.New(rt, registry), nil
}
