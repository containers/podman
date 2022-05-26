/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spdx

import (
	"fmt"

	"github.com/pkg/errors"
)

type RelationshipType string

//nolint:revive,stylecheck
const (
	DESCRIBES              RelationshipType = "DESCRIBES"
	DESCRIBED_BY           RelationshipType = "DESCRIBED_BY"
	CONTAINS               RelationshipType = "CONTAINS"
	CONTAINED_BY           RelationshipType = "CONTAINED_BY"
	DEPENDS_ON             RelationshipType = "DEPENDS_ON"
	DEPENDENCY_OF          RelationshipType = "DEPENDENCY_OF"
	DEPENDENCY_MANIFEST_OF RelationshipType = "DEPENDENCY_MANIFEST_OF"
	BUILD_DEPENDENCY_OF    RelationshipType = "BUILD_DEPENDENCY_OF"
	DEV_DEPENDENCY_OF      RelationshipType = "DEV_DEPENDENCY_OF"
	OPTIONAL_DEPENDENCY_OF RelationshipType = "OPTIONAL_DEPENDENCY_OF"
	PROVIDED_DEPENDENCY_OF RelationshipType = "PROVIDED_DEPENDENCY_OF"
	TEST_DEPENDENCY_OF     RelationshipType = "TEST_DEPENDENCY_OF"
	RUNTIME_DEPENDENCY_OF  RelationshipType = "RUNTIME_DEPENDENCY_OF"
	EXAMPLE_OF             RelationshipType = "EXAMPLE_OF"
	GENERATES              RelationshipType = "GENERATES"
	GENERATED_FROM         RelationshipType = "GENERATED_FROM"
	ANCESTOR_OF            RelationshipType = "ANCESTOR_OF"
	DESCENDANT_OF          RelationshipType = "DESCENDANT_OF"
	VARIANT_OF             RelationshipType = "VARIANT_OF"
	DISTRIBUTION_ARTIFACT  RelationshipType = "DISTRIBUTION_ARTIFACT"
	PATCH_FOR              RelationshipType = "PATCH_FOR"
	PATCH_APPLIED          RelationshipType = "PATCH_APPLIED"
	COPY_OF                RelationshipType = "COPY_OF"
	FILE_ADDED             RelationshipType = "FILE_ADDED"
	FILE_DELETED           RelationshipType = "FILE_DELETED"
	FILE_MODIFIED          RelationshipType = "FILE_MODIFIED"
	EXPANDED_FROM_ARCHIVE  RelationshipType = "EXPANDED_FROM_ARCHIVE"
	DYNAMIC_LINK           RelationshipType = "DYNAMIC_LINK"
	STATIC_LINK            RelationshipType = "STATIC_LINK"
	DATA_FILE_OF           RelationshipType = "DATA_FILE_OF"
	TEST_CASE_OF           RelationshipType = "TEST_CASE_OF"
	BUILD_TOOL_OF          RelationshipType = "BUILD_TOOL_OF"
	DEV_TOOL_OF            RelationshipType = "DEV_TOOL_OF"
	TEST_OF                RelationshipType = "TEST_OF"
	TEST_TOOL_OF           RelationshipType = "TEST_TOOL_OF"
	DOCUMENTATION_OF       RelationshipType = "DOCUMENTATION_OF"
	OPTIONAL_COMPONENT_OF  RelationshipType = "OPTIONAL_COMPONENT_OF"
	METAFILE_OF            RelationshipType = "METAFILE_OF"
	PACKAGE_OF             RelationshipType = "PACKAGE_OF"
	AMENDS                 RelationshipType = "AMENDS"
	PREREQUISITE_FOR       RelationshipType = "PREREQUISITE_FOR"
	HAS_PREREQUISITE       RelationshipType = "HAS_PREREQUISITE"
	OTHER                  RelationshipType = "OTHER"
)

type Relationship struct {
	FullRender       bool             // Flag, then true the package will be rendered in the doc
	PeerReference    string           // SPDX Ref of the peer object. Will override the ID of provided package if set
	PeerExtReference string           // External doc reference if peer is a different doc
	Comment          string           // Relationship ship commnet
	Type             RelationshipType // Relationship of the specified package
	Peer             Object           // SPDX object that acts as peer
}

func (ro *Relationship) Render(hostObject Object) (string, error) {
	// We can render the relationship from an object or from a
	// predefined entity reference. But we have to have on of them
	if ro.Peer == nil && ro.PeerReference == "" {
		return "", errors.New(
			"unable to render reference no peer or peer reference defined",
		)
	}
	if ro.Peer != nil && ro.Peer.SPDXID() == "" {
		return "", errors.New("unable to render relationship, peer object has no SPDX ID")
	}

	if ro.FullRender && ro.Peer == nil {
		return "", errors.New("unable to render relationship. peer object has to be set")
	}

	if ro.Type == "" {
		return "", errors.New("unable to render relationship, type is not set")
	}

	// The host object must have an ID defined in all cases
	if hostObject.SPDXID() == "" {
		return "", errors.New("Unable to rennder relationship, hostObject has no ID")
	}

	docFragment := ""
	if ro.FullRender {
		objDoc, err := ro.Peer.Render()
		if err != nil {
			return "", errors.Wrapf(err, "rendering related object %s", hostObject.SPDXID())
		}
		docFragment += objDoc
	}
	peerExtRef := ""
	if ro.PeerExtReference != "" {
		peerExtRef = fmt.Sprintf("DocumentRef-%s:", ro.PeerExtReference)
	}
	if ro.Peer != nil {
		docFragment += fmt.Sprintf(
			"Relationship: %s %s %s%s\n", hostObject.SPDXID(), ro.Type, peerExtRef, ro.Peer.SPDXID(),
		)
	} else {
		docFragment += fmt.Sprintf(
			"Relationship: %s %s %s%s\n", hostObject.SPDXID(), ro.Type, peerExtRef, ro.PeerReference,
		)
	}
	return docFragment, nil
}
