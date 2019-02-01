package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/trust"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	setTrustFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Usage: "Trust type, accept values: signedBy(default), accept, reject.",
			Value: "signedBy",
		},
		cli.StringSliceFlag{
			Name: "pubkeysfile, f",
			Usage: `Path of installed public key(s) to trust for TARGET.
	Absolute path to keys is added to policy.json. May
	used multiple times to define multiple public keys.
	File(s) must exist before using this command.`,
		},
		cli.StringFlag{
			Name:   "policypath",
			Hidden: true,
		},
	}
	showTrustFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "raw",
			Usage: "Output raw policy file",
		},
		cli.BoolFlag{
			Name:  "json, j",
			Usage: "Output as json",
		},
		cli.StringFlag{
			Name:   "policypath",
			Hidden: true,
		},
		cli.StringFlag{
			Name:   "registrypath",
			Hidden: true,
		},
	}

	setTrustDescription = "Set default trust policy or add a new trust policy for a registry"
	setTrustCommand     = cli.Command{
		Name:         "set",
		Usage:        "Set default trust policy or a new trust policy for a registry",
		Description:  setTrustDescription,
		Flags:        sortFlags(setTrustFlags),
		ArgsUsage:    "default | REGISTRY[/REPOSITORY]",
		Action:       setTrustCmd,
		OnUsageError: usageErrorHandler,
	}

	showTrustDescription = "Display trust policy for the system"
	showTrustCommand     = cli.Command{
		Name:                   "show",
		Usage:                  "Display trust policy for the system",
		Description:            showTrustDescription,
		Flags:                  sortFlags(showTrustFlags),
		Action:                 showTrustCmd,
		ArgsUsage:              "",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}

	trustSubCommands = []cli.Command{
		setTrustCommand,
		showTrustCommand,
	}

	trustDescription = fmt.Sprintf(`Manages the trust policy of the host system. (%s)
	 Trust policy describes a registry scope that must be signed by public keys.`, getDefaultPolicyPath())
	trustCommand = cli.Command{
		Name:         "trust",
		Usage:        "Manage container image trust policy",
		Description:  trustDescription,
		ArgsUsage:    "{set,show} ...",
		Subcommands:  trustSubCommands,
		OnUsageError: usageErrorHandler,
	}
)

func showTrustCmd(c *cli.Context) error {
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}

	var (
		policyPath              string
		systemRegistriesDirPath string
		outjson                 interface{}
	)
	if c.IsSet("policypath") {
		policyPath = c.String("policypath")
	} else {
		policyPath = trust.DefaultPolicyPath(runtime.SystemContext())
	}
	policyContent, err := ioutil.ReadFile(policyPath)
	if err != nil {
		return errors.Wrapf(err, "unable to read %s", policyPath)
	}
	if c.IsSet("registrypath") {
		systemRegistriesDirPath = c.String("registrypath")
	} else {
		systemRegistriesDirPath = trust.RegistriesDirPath(runtime.SystemContext())
	}

	if c.Bool("raw") {
		_, err := os.Stdout.Write(policyContent)
		if err != nil {
			return errors.Wrap(err, "could not read trust policies")
		}
		return nil
	}

	policyContentStruct, err := trust.GetPolicy(policyPath)
	if err != nil {
		return errors.Wrapf(err, "could not read trust policies")
	}

	if c.Bool("json") {
		policyJSON, err := getPolicyJSON(policyContentStruct, systemRegistriesDirPath)
		if err != nil {
			return errors.Wrapf(err, "could not show trust policies in JSON format")
		}
		outjson = policyJSON
		out := formats.JSONStruct{Output: outjson}
		return formats.Writer(out).Out()
	}

	showOutputMap, err := getPolicyShowOutput(policyContentStruct, systemRegistriesDirPath)
	if err != nil {
		return errors.Wrapf(err, "could not show trust policies")
	}
	out := formats.StdoutTemplateArray{Output: showOutputMap, Template: "{{.Repo}}\t{{.Trusttype}}\t{{.GPGid}}\t{{.Sigstore}}"}
	return formats.Writer(out).Out()
}

func setTrustCmd(c *cli.Context) error {
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	var (
		policyPath          string
		policyContentStruct trust.PolicyContent
		newReposContent     []trust.RepoContent
	)
	args := c.Args()
	if len(args) != 1 {
		return errors.Errorf("default or a registry name must be specified")
	}
	valid, err := image.IsValidImageURI(args[0])
	if err != nil || !valid {
		return errors.Wrapf(err, "invalid image uri %s", args[0])
	}

	trusttype := c.String("type")
	if !isValidTrustType(trusttype) {
		return errors.Errorf("invalid choice: %s (choose from 'accept', 'reject', 'signedBy')", trusttype)
	}
	if trusttype == "accept" {
		trusttype = "insecureAcceptAnything"
	}

	pubkeysfile := c.StringSlice("pubkeysfile")
	if len(pubkeysfile) == 0 && trusttype == "signedBy" {
		return errors.Errorf("At least one public key must be defined for type 'signedBy'")
	}

	if c.IsSet("policypath") {
		policyPath = c.String("policypath")
	} else {
		policyPath = trust.DefaultPolicyPath(runtime.SystemContext())
	}
	_, err = os.Stat(policyPath)
	if !os.IsNotExist(err) {
		policyContent, err := ioutil.ReadFile(policyPath)
		if err != nil {
			return errors.Wrapf(err, "unable to read %s", policyPath)
		}
		if err := json.Unmarshal(policyContent, &policyContentStruct); err != nil {
			return errors.Errorf("could not read trust policies")
		}
	}
	if len(pubkeysfile) != 0 {
		for _, filepath := range pubkeysfile {
			newReposContent = append(newReposContent, trust.RepoContent{Type: trusttype, KeyType: "GPGKeys", KeyPath: filepath})
		}
	} else {
		newReposContent = append(newReposContent, trust.RepoContent{Type: trusttype})
	}
	if args[0] == "default" {
		policyContentStruct.Default = newReposContent
	} else {
		if len(policyContentStruct.Default) == 0 {
			return errors.Errorf("Default trust policy must be set.")
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
	err = ioutil.WriteFile(policyPath, data, 0644)
	if err != nil {
		return errors.Wrapf(err, "error setting trust policy")
	}
	return nil
}

func sortShowOutputMapKey(m map[string]trust.ShowOutput) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func isValidTrustType(t string) bool {
	if t == "accept" || t == "insecureAcceptAnything" || t == "reject" || t == "signedBy" {
		return true
	}
	return false
}

func getDefaultPolicyPath() string {
	return trust.DefaultPolicyPath(&types.SystemContext{})
}

func getPolicyJSON(policyContentStruct trust.PolicyContent, systemRegistriesDirPath string) (map[string]map[string]interface{}, error) {
	registryConfigs, err := trust.LoadAndMergeConfig(systemRegistriesDirPath)
	if err != nil {
		return nil, err
	}

	policyJSON := make(map[string]map[string]interface{})
	if len(policyContentStruct.Default) > 0 {
		policyJSON["* (default)"] = make(map[string]interface{})
		policyJSON["* (default)"]["type"] = policyContentStruct.Default[0].Type
	}
	for transname, transval := range policyContentStruct.Transports {
		for repo, repoval := range transval {
			policyJSON[repo] = make(map[string]interface{})
			policyJSON[repo]["type"] = repoval[0].Type
			policyJSON[repo]["transport"] = transname
			keyarr := []string{}
			uids := []string{}
			for _, repoele := range repoval {
				if len(repoele.KeyPath) > 0 {
					keyarr = append(keyarr, repoele.KeyPath)
					uids = append(uids, trust.GetGPGIdFromKeyPath(repoele.KeyPath)...)
				}
				if len(repoele.KeyData) > 0 {
					keyarr = append(keyarr, string(repoele.KeyData))
					uids = append(uids, trust.GetGPGIdFromKeyData(string(repoele.KeyData))...)
				}
			}
			policyJSON[repo]["keys"] = keyarr
			policyJSON[repo]["sigstore"] = ""
			registryNamespace := trust.HaveMatchRegistry(repo, registryConfigs)
			if registryNamespace != nil {
				policyJSON[repo]["sigstore"] = registryNamespace.SigStore
			}
		}
	}
	return policyJSON, nil
}

var typeDescription = map[string]string{"insecureAcceptAnything": "accept", "signedBy": "signed", "reject": "reject"}

func trustTypeDescription(trustType string) string {
	trustDescription, exist := typeDescription[trustType]
	if !exist {
		logrus.Warnf("invalid trust type %s", trustType)
	}
	return trustDescription
}

func getPolicyShowOutput(policyContentStruct trust.PolicyContent, systemRegistriesDirPath string) ([]interface{}, error) {
	var output []interface{}

	registryConfigs, err := trust.LoadAndMergeConfig(systemRegistriesDirPath)
	if err != nil {
		return nil, err
	}

	trustShowOutputMap := make(map[string]trust.ShowOutput)
	if len(policyContentStruct.Default) > 0 {
		defaultPolicyStruct := trust.ShowOutput{
			Repo:      "default",
			Trusttype: trustTypeDescription(policyContentStruct.Default[0].Type),
		}
		trustShowOutputMap["* (default)"] = defaultPolicyStruct
	}
	for _, transval := range policyContentStruct.Transports {
		for repo, repoval := range transval {
			tempTrustShowOutput := trust.ShowOutput{
				Repo:      repo,
				Trusttype: repoval[0].Type,
			}
			keyarr := []string{}
			uids := []string{}
			for _, repoele := range repoval {
				if len(repoele.KeyPath) > 0 {
					keyarr = append(keyarr, repoele.KeyPath)
					uids = append(uids, trust.GetGPGIdFromKeyPath(repoele.KeyPath)...)
				}
				if len(repoele.KeyData) > 0 {
					keyarr = append(keyarr, string(repoele.KeyData))
					uids = append(uids, trust.GetGPGIdFromKeyData(string(repoele.KeyData))...)
				}
			}
			tempTrustShowOutput.GPGid = strings.Join(uids, ", ")

			registryNamespace := trust.HaveMatchRegistry(repo, registryConfigs)
			if registryNamespace != nil {
				tempTrustShowOutput.Sigstore = registryNamespace.SigStore
			}
			trustShowOutputMap[repo] = tempTrustShowOutput
		}
	}

	sortedRepos := sortShowOutputMapKey(trustShowOutputMap)
	for _, reponame := range sortedRepos {
		showOutput, exists := trustShowOutputMap[reponame]
		if exists {
			output = append(output, interface{}(showOutput))
		}
	}
	return output, nil
}
