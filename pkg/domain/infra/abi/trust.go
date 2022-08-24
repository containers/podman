package abi

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/trust"
	"github.com/sirupsen/logrus"
)

func (ir *ImageEngine) ShowTrust(ctx context.Context, args []string, options entities.ShowTrustOptions) (*entities.ShowTrustReport, error) {
	var (
		err    error
		report entities.ShowTrustReport
	)
	policyPath := trust.DefaultPolicyPath(ir.Libpod.SystemContext())
	if len(options.PolicyPath) > 0 {
		policyPath = options.PolicyPath
	}
	report.Raw, err = ioutil.ReadFile(policyPath)
	if err != nil {
		return nil, err
	}
	if options.Raw {
		return &report, nil
	}
	report.SystemRegistriesDirPath = trust.RegistriesDirPath(ir.Libpod.SystemContext())
	if len(options.RegistryPath) > 0 {
		report.SystemRegistriesDirPath = options.RegistryPath
	}
	policyContentStruct, err := trust.GetPolicy(policyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read trust policies: %w", err)
	}
	report.Policies, err = getPolicyShowOutput(policyContentStruct, report.SystemRegistriesDirPath)
	if err != nil {
		return nil, fmt.Errorf("could not show trust policies: %w", err)
	}
	return &report, nil
}

func (ir *ImageEngine) SetTrust(ctx context.Context, args []string, options entities.SetTrustOptions) error {
	if len(args) != 1 {
		return fmt.Errorf("SetTrust called with unexpected %d args", len(args))
	}
	scope := args[0]

	policyPath := trust.DefaultPolicyPath(ir.Libpod.SystemContext())
	if len(options.PolicyPath) > 0 {
		policyPath = options.PolicyPath
	}

	return trust.AddPolicyEntries(policyPath, trust.AddPolicyEntriesInput{
		Scope:       scope,
		Type:        options.Type,
		PubKeyFiles: options.PubKeysFile,
	})
}

func getPolicyShowOutput(policyContentStruct trust.PolicyContent, systemRegistriesDirPath string) ([]*trust.Policy, error) {
	var output []*trust.Policy

	registryConfigs, err := trust.LoadAndMergeConfig(systemRegistriesDirPath)
	if err != nil {
		return nil, err
	}

	if len(policyContentStruct.Default) > 0 {
		defaultPolicyStruct := trust.Policy{
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
			tempTrustShowOutput := trust.Policy{
				Name:      repo,
				RepoName:  repo,
				Transport: transport,
				Type:      trustTypeDescription(repoval[0].Type),
			}
			uids := []string{}
			for _, repoele := range repoval {
				if len(repoele.KeyPath) > 0 {
					uids = append(uids, trust.GetGPGIdFromKeyPath(repoele.KeyPath)...)
				}
				if len(repoele.KeyData) > 0 {
					uids = append(uids, trust.GetGPGIdFromKeyData(repoele.KeyData)...)
				}
			}
			tempTrustShowOutput.GPGId = strings.Join(uids, ", ")

			registryNamespace := trust.HaveMatchRegistry(repo, registryConfigs)
			if registryNamespace != nil {
				tempTrustShowOutput.SignatureStore = registryNamespace.SigStore
			}
			output = append(output, &tempTrustShowOutput)
		}
	}
	return output, nil
}

var typeDescription = map[string]string{"insecureAcceptAnything": "accept", "signedBy": "signed", "reject": "reject"}

func trustTypeDescription(trustType string) string {
	trustDescription, exist := typeDescription[trustType]
	if !exist {
		logrus.Warnf("Invalid trust type %s", trustType)
	}
	return trustDescription
}
