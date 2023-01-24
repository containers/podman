// Copyright 2022 The Sigstore Authors.
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

package certificate

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
)

var (
	OIDIssuer                   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
	OIDGitHubWorkflowTrigger    = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 2}
	OIDGitHubWorkflowSHA        = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 3}
	OIDGitHubWorkflowName       = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 4}
	OIDGitHubWorkflowRepository = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 5}
	OIDGitHubWorkflowRef        = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 6}
	OIDOtherName                = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 7}
)

// Extensions contains all custom x509 extensions defined by Fulcio
type Extensions struct {
	// NB: New extensions must be added here and documented
	// at docs/oidc-info.md

	// The OIDC issuer. Should match `iss` claim of ID token or, in the case of
	// a federated login like Dex it should match the issuer URL of the
	// upstream issuer. The issuer is not set the extensions are invalid and
	// will fail to render.
	Issuer string // OID 1.3.6.1.4.1.57264.1.1

	// Triggering event of the Github Workflow. Matches the `event_name` claim of ID
	// tokens from Github Actions
	GithubWorkflowTrigger string // OID 1.3.6.1.4.1.57264.1.2

	// SHA of git commit being built in Github Actions. Matches the `sha` claim of ID
	// tokens from Github Actions
	GithubWorkflowSHA string // OID 1.3.6.1.4.1.57264.1.3

	// Name of Github Actions Workflow. Matches the `workflow` claim of the ID
	// tokens from Github Actions
	GithubWorkflowName string // OID 1.3.6.1.4.1.57264.1.4

	// Repository of the Github Actions Workflow. Matches the `repository` claim of the ID
	// tokens from Github Actions
	GithubWorkflowRepository string // OID 1.3.6.1.4.1.57264.1.5

	// Git Ref of the Github Actions Workflow. Matches the `ref` claim of the ID tokens
	// from Github Actions
	GithubWorkflowRef string // 1.3.6.1.4.1.57264.1.6
}

func (e Extensions) Render() ([]pkix.Extension, error) {
	var exts []pkix.Extension

	if e.Issuer != "" {
		exts = append(exts, pkix.Extension{
			Id:    OIDIssuer,
			Value: []byte(e.Issuer),
		})
	} else {
		return nil, errors.New("extensions must have a non-empty issuer url")
	}
	if e.GithubWorkflowTrigger != "" {
		exts = append(exts, pkix.Extension{
			Id:    OIDGitHubWorkflowTrigger,
			Value: []byte(e.GithubWorkflowTrigger),
		})
	}
	if e.GithubWorkflowSHA != "" {
		exts = append(exts, pkix.Extension{
			Id:    OIDGitHubWorkflowSHA,
			Value: []byte(e.GithubWorkflowSHA),
		})
	}
	if e.GithubWorkflowName != "" {
		exts = append(exts, pkix.Extension{
			Id:    OIDGitHubWorkflowName,
			Value: []byte(e.GithubWorkflowName),
		})
	}
	if e.GithubWorkflowRepository != "" {
		exts = append(exts, pkix.Extension{
			Id:    OIDGitHubWorkflowRepository,
			Value: []byte(e.GithubWorkflowRepository),
		})
	}
	if e.GithubWorkflowRef != "" {
		exts = append(exts, pkix.Extension{
			Id:    OIDGitHubWorkflowRef,
			Value: []byte(e.GithubWorkflowRef),
		})
	}
	return exts, nil
}

func ParseExtensions(ext []pkix.Extension) (Extensions, error) {
	out := Extensions{}

	for _, e := range ext {
		switch {
		case e.Id.Equal(OIDIssuer):
			out.Issuer = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowTrigger):
			out.GithubWorkflowTrigger = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowSHA):
			out.GithubWorkflowSHA = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowName):
			out.GithubWorkflowName = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowRepository):
			out.GithubWorkflowRepository = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowRef):
			out.GithubWorkflowRef = string(e.Value)
		}
	}

	// We only ever return nil, but leaving error in place so that we can add
	// more complex parsing of fields in a backwards compatible way if needed.
	return out, nil
}
