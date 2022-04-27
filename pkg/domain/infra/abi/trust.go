package abi

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/trust"
	"github.com/pkg/errors"
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
		return nil, errors.Wrapf(err, "could not read trust policies")
	}
	report.Policies, err = getPolicyShowOutput(policyContentStruct, report.SystemRegistriesDirPath)
	if err != nil {
		return nil, errors.Wrapf(err, "could not show trust policies")
	}
	return &report, nil
}

func (ir *ImageEngine) SetTrust(ctx context.Context, args []string, options entities.SetTrustOptions) error {
	var (
		policyContentStruct trust.PolicyContent
		newReposContent     []trust.RepoContent
	)
	trustType := options.Type
	if trustType == "accept" {
		trustType = "insecureAcceptAnything"
	}

	pubkeysfile := options.PubKeysFile
	if len(pubkeysfile) == 0 && trustType == "signedBy" {
		return errors.Errorf("At least one public key must be defined for type 'signedBy'")
	}

	policyPath := trust.DefaultPolicyPath(ir.Libpod.SystemContext())
	if len(options.PolicyPath) > 0 {
		policyPath = options.PolicyPath
	}
	_, err := os.Stat(policyPath)
	if !os.IsNotExist(err) {
		policyContent, err := ioutil.ReadFile(policyPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(policyContent, &policyContentStruct); err != nil {
			return errors.Errorf("could not read trust policies")
		}
	}
	if len(pubkeysfile) != 0 {
		for _, filepath := range pubkeysfile {
			newReposContent = append(newReposContent, trust.RepoContent{Type: trustType, KeyType: "GPGKeys", KeyPath: filepath})
		}
	} else {
		newReposContent = append(newReposContent, trust.RepoContent{Type: trustType})
	}
	if args[0] == "default" {
		policyContentStruct.Default = newReposContent
	} else {
		if len(policyContentStruct.Default) == 0 {
			return errors.Errorf("default trust policy must be set")
		}
		registryExists := false
		for transport, transportval := range policyContentStruct.Transports {
			_, registryExists = transportval[args[0]]
			if registryExists {
				policyContentStruct.Transports[transport][args[0]] = newReposContent
				break
			}
		}
		if !registryExists {
			if policyContentStruct.Transports == nil {
				policyContentStruct.Transports = make(map[string]trust.RepoMap)
			}
			if policyContentStruct.Transports["docker"] == nil {
				policyContentStruct.Transports["docker"] = make(map[string][]trust.RepoContent)
			}
			policyContentStruct.Transports["docker"][args[0]] = append(policyContentStruct.Transports["docker"][args[0]], newReposContent...)
		}
	}

	data, err := json.MarshalIndent(policyContentStruct, "", "    ")
	if err != nil {
		return errors.Wrapf(err, "error setting trust policy")
	}
	return ioutil.WriteFile(policyPath, data, 0644)
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
			// TODO - keyarr is not used and I don't know its intent; commenting out for now for someone to fix later
			// keyarr := []string{}
			uids := []string{}
			for _, repoele := range repoval {
				if len(repoele.KeyPath) > 0 {
					// keyarr = append(keyarr, repoele.KeyPath)
					uids = append(uids, trust.GetGPGIdFromKeyPath(repoele.KeyPath)...)
				}
				if len(repoele.KeyData) > 0 {
					// keyarr = append(keyarr, string(repoele.KeyData))
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
