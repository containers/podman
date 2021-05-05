package manifests

import (
	"github.com/containers/image/v5/signature"
)

var (
	// storageAllowedPolicyScopes overrides the policy for local storage
	// to ensure that we can read images from it.
	storageAllowedPolicyScopes = signature.PolicyTransportScopes{
		"": []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
	}
)
