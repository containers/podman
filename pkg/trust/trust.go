package trust

import (
	"fmt"
	"strings"
)

// Policy describes a basic trust policy configuration
type Policy struct {
	Transport      string   `json:"transport"`
	Name           string   `json:"name,omitempty"`
	RepoName       string   `json:"repo_name,omitempty"`
	Keys           []string `json:"keys,omitempty"`
	SignatureStore string   `json:"sigstore,omitempty"`
	Type           string   `json:"type"`
	GPGId          string   `json:"gpg_id,omitempty"`
}

// PolicyDescription returns an user-focused description of the policy in policyPath and registries.d data from registriesDirPath.
func PolicyDescription(policyPath, registriesDirPath string) ([]*Policy, error) {
	policyContentStruct, err := GetPolicy(policyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read trust policies: %w", err)
	}
	res, err := getPolicyShowOutput(policyContentStruct, registriesDirPath)
	if err != nil {
		return nil, fmt.Errorf("could not show trust policies: %w", err)
	}
	return res, nil
}

func getPolicyShowOutput(policyContentStruct PolicyContent, systemRegistriesDirPath string) ([]*Policy, error) {
	var output []*Policy

	registryConfigs, err := LoadAndMergeConfig(systemRegistriesDirPath)
	if err != nil {
		return nil, err
	}

	if len(policyContentStruct.Default) > 0 {
		defaultPolicyStruct := Policy{
			Transport: "all",
			Name:      "* (default)",
			RepoName:  "default",
			Type:      trustTypeDescription(policyContentStruct.Default[0].Type),
		}
		output = append(output, &defaultPolicyStruct)
	}
	for transport, transval := range policyContentStruct.Transports {
		if transport == "docker" {
			transport = "repository"
		}

		for repo, repoval := range transval {
			tempTrustShowOutput := Policy{
				Name:      repo,
				RepoName:  repo,
				Transport: transport,
				Type:      trustTypeDescription(repoval[0].Type),
			}
			uids := []string{}
			for _, repoele := range repoval {
				if len(repoele.KeyPath) > 0 {
					uids = append(uids, GetGPGIdFromKeyPath(repoele.KeyPath)...)
				}
				if len(repoele.KeyData) > 0 {
					uids = append(uids, GetGPGIdFromKeyData(repoele.KeyData)...)
				}
			}
			tempTrustShowOutput.GPGId = strings.Join(uids, ", ")

			registryNamespace := HaveMatchRegistry(repo, registryConfigs)
			if registryNamespace != nil {
				tempTrustShowOutput.SignatureStore = registryNamespace.SigStore
			}
			output = append(output, &tempTrustShowOutput)
		}
	}
	return output, nil
}
