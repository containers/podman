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
	"net/url"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	genclient "github.com/sigstore/fulcio/pkg/generated/client"
)

// SigstorePublicServerURL is the URL of Sigstore's public Fulcio service.
const SigstorePublicServerURL = "https://fulcio.sigstore.dev"

// New returns a new client to interact with the given fulcio server.
func New(server *url.URL) *genclient.Fulcio {
	rt := httptransport.New(server.Host, genclient.DefaultBasePath, []string{server.Scheme})
	rt.Consumers["application/pem-certificate-chain"] = runtime.TextConsumer()
	return genclient.New(rt, strfmt.Default)
}
