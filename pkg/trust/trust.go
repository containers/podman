package trust

import (
	"fmt"
	"sort"
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
	return policyDescriptionWithGPGIDReader(policyPath, registriesDirPath, getGPGIdFromKeyPath)
}

// policyDescriptionWithGPGIDReader is PolicyDescription with a gpgIDReader parameter. It exists only to make testing easier.
func policyDescriptionWithGPGIDReader(policyPath, registriesDirPath string, idReader gpgIDReader) ([]*Policy, error) {
	policyContentStruct, err := getPolicy(policyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read trust policies: %w", err)
	}
	res, err := getPolicyShowOutput(policyContentStruct, registriesDirPath, idReader)
	if err != nil {
		return nil, fmt.Errorf("could not show trust policies: %w", err)
	}
	return res, nil
}

func getPolicyShowOutput(policyContentStruct policyContent, systemRegistriesDirPath string, idReader gpgIDReader) ([]*Policy, error) {
	var output []*Policy

	registryConfigs, err := loadAndMergeConfig(systemRegistriesDirPath)
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
	// FIXME: This should use x/exp/maps.Keys after we update to Go 1.18.
	transports := []string{}
	for t := range policyContentStruct.Transports {
		transports = append(transports, t)
	}
	sort.Strings(transports)
	for _, transport := range transports {
		transval := policyContentStruct.Transports[transport]
		if transport == "docker" {
			transport = "repository"
		}

		// FIXME: This should use x/exp/maps.Keys after we update to Go 1.18.
		scopes := []string{}
		for s := range transval {
			scopes = append(scopes, s)
		}
		sort.Strings(scopes)
		for _, repo := range scopes {
			repoval := transval[repo]
			template := Policy{
				Transport: transport,
				Name:      repo,
				RepoName:  repo,
			}
			output = append(output, descriptionsOfPolicyRequirements(repoval, template, registryConfigs, repo, idReader)...)
		}
	}
	return output, nil
}

// descriptionsOfPolicyRequirements turns reqs into user-readable policy entries, with Transport/Name/Reponame coming from template, potentially looking up scope in registryConfigs.
func descriptionsOfPolicyRequirements(reqs []repoContent, template Policy, registryConfigs *registryConfiguration, scope string, idReader gpgIDReader) []*Policy {
	tempTrustShowOutput := template
	tempTrustShowOutput.Type = trustTypeDescription(reqs[0].Type)
	uids := []string{}
	for _, repoele := range reqs {
		if len(repoele.KeyPath) > 0 {
			uids = append(uids, idReader(repoele.KeyPath)...)
		}
		if len(repoele.KeyData) > 0 {
			uids = append(uids, getGPGIdFromKeyData(idReader, repoele.KeyData)...)
		}
	}
	tempTrustShowOutput.GPGId = strings.Join(uids, ", ")

	registryNamespace := haveMatchRegistry(scope, registryConfigs)
	if registryNamespace != nil {
		if registryNamespace.Lookaside != "" {
			tempTrustShowOutput.SignatureStore = registryNamespace.Lookaside
		} else { // incl. registryNamespace.SigStore == ""
			tempTrustShowOutput.SignatureStore = registryNamespace.SigStore
		}
	}
	return []*Policy{&tempTrustShowOutput}
}
