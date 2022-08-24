package trust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containers/image/v5/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyDescription(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "policy.json")

	// Override getGPGIdFromKeyPath because we don't want to bother with (and spend the unit-test time on) generating valid GPG keys, and running the real GPG binary.
	// Instead of reading the files at all, just expect file names like /id1,id2,...,idN.pub
	idReader := func(keyPath string) []string {
		require.True(t, strings.HasPrefix(keyPath, "/"))
		require.True(t, strings.HasSuffix(keyPath, ".pub"))
		return strings.Split(keyPath[1:len(keyPath)-4], ",")
	}

	for _, c := range []struct {
		policy   *signature.Policy
		expected []*Policy
	}{
		{
			&signature.Policy{
				Default: signature.PolicyRequirements{
					signature.NewPRReject(),
				},
				Transports: map[string]signature.PolicyTransportScopes{
					"docker": {
						"quay.io/accepted": {
							signature.NewPRInsecureAcceptAnything(),
						},
						"registry.redhat.io": {
							xNewPRSignedByKeyPath(t, "/redhat.pub", signature.NewPRMMatchRepoDigestOrExact()),
						},
						"quay.io/multi-signed": {
							xNewPRSignedByKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
							xNewPRSignedByKeyPath(t, "/2,3.pub", signature.NewPRMMatchRepoDigestOrExact()),
						},
					},
				},
			},
			[]*Policy{
				{
					Transport: "all",
					Name:      "* (default)",
					RepoName:  "default",
					Type:      "reject",
				},
				{
					Transport: "repository",
					Name:      "quay.io/accepted",
					RepoName:  "quay.io/accepted",
					Type:      "accept",
				},
				{
					Transport:      "repository",
					Name:           "quay.io/multi-signed",
					RepoName:       "quay.io/multi-signed",
					Type:           "signed",
					SignatureStore: "",
					GPGId:          "1, 2, 3",
				},
				{
					Transport:      "repository",
					Name:           "registry.redhat.io",
					RepoName:       "registry.redhat.io",
					Type:           "signed",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "redhat",
				},
			},
		},
	} {
		policyJSON, err := json.Marshal(c.policy)
		require.NoError(t, err)
		err = os.WriteFile(policyPath, policyJSON, 0600)
		require.NoError(t, err)

		res, err := policyDescriptionWithGPGIDReader(policyPath, "./testdata", idReader)
		require.NoError(t, err)
		assert.Equal(t, c.expected, res)
	}
}
