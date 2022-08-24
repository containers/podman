package trust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/image/v5/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPolicyEntries(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "policy.json")

	minimalPolicy := &signature.Policy{
		Default: []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
	}
	minimalPolicyJSON, err := json.Marshal(minimalPolicy)
	require.NoError(t, err)
	err = os.WriteFile(policyPath, minimalPolicyJSON, 0600)
	require.NoError(t, err)

	err = AddPolicyEntries(policyPath, AddPolicyEntriesInput{
		Scope: "default",
		Type:  "reject",
	})
	assert.NoError(t, err)
	err = AddPolicyEntries(policyPath, AddPolicyEntriesInput{
		Scope: "quay.io/accepted",
		Type:  "accept",
	})
	assert.NoError(t, err)
	err = AddPolicyEntries(policyPath, AddPolicyEntriesInput{
		Scope:       "quay.io/multi-signed",
		Type:        "signedBy",
		PubKeyFiles: []string{"/1.pub", "/2.pub"},
	})
	assert.NoError(t, err)

	// Test that the outcome is consumable, and compare it with the expected values.
	parsedPolicy, err := signature.NewPolicyFromFile(policyPath)
	require.NoError(t, err)
	assert.Equal(t, &signature.Policy{
		Default: signature.PolicyRequirements{
			signature.NewPRReject(),
		},
		Transports: map[string]signature.PolicyTransportScopes{
			"docker": {
				"quay.io/accepted": {
					signature.NewPRInsecureAcceptAnything(),
				},
				"quay.io/multi-signed": {
					xNewPRSignedByKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
					xNewPRSignedByKeyPath(t, "/2.pub", signature.NewPRMMatchRepoDigestOrExact()),
				},
			},
		},
	}, parsedPolicy)
}

// xNewPRSignedByKeyPath is a wrapper for NewPRSignedByKeyPath which must not fail.
func xNewPRSignedByKeyPath(t *testing.T, keyPath string, signedIdentity signature.PolicyReferenceMatch) signature.PolicyRequirement {
	pr, err := signature.NewPRSignedByKeyPath(signature.SBKeyTypeGPGKeys, keyPath, signedIdentity)
	require.NoError(t, err)
	return pr
}
