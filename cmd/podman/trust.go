package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"

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

	var policyContentStruct trust.PolicyContent
	if err := json.Unmarshal(policyContent, &policyContentStruct); err != nil {
		return errors.Errorf("could not read trust policies")
	}
	policyJSON, err := trust.GetPolicyJSON(policyContentStruct, systemRegistriesDirPath)
	if err != nil {
		return errors.Wrapf(err, "error reading registry config file")
	}
	if c.Bool("json") {
		var outjson interface{}
		outjson = policyJSON
		out := formats.JSONStruct{Output: outjson}
		return formats.Writer(out).Out()
	}

	sortedRepos := sortPolicyJSONKey(policyJSON)
	type policydefault struct {
		Repo      string
		Trusttype string
		GPGid     string
		Sigstore  string
	}
	var policyoutput []policydefault
	for _, repo := range sortedRepos {
		repoval := policyJSON[repo]
		var defaultstruct policydefault
		defaultstruct.Repo = repo
		if repoval["type"] != nil {
			defaultstruct.Trusttype = trustTypeDescription(repoval["type"].(string))
		}
		if repoval["keys"] != nil && len(repoval["keys"].([]string)) > 0 {
			defaultstruct.GPGid = trust.GetGPGId(repoval["keys"].([]string))
		}
		if repoval["sigstore"] != nil {
			defaultstruct.Sigstore = repoval["sigstore"].(string)
		}
		policyoutput = append(policyoutput, defaultstruct)
	}
	var output []interface{}
	for _, ele := range policyoutput {
		output = append(output, interface{}(ele))
	}
	out := formats.StdoutTemplateArray{Output: output, Template: "{{.Repo}}\t{{.Trusttype}}\t{{.GPGid}}\t{{.Sigstore}}"}
	return formats.Writer(out).Out()
}

func setTrustCmd(c *cli.Context) error {
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}

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

	var policyPath string
	if c.IsSet("policypath") {
		policyPath = c.String("policypath")
	} else {
		policyPath = trust.DefaultPolicyPath(runtime.SystemContext())
	}
	var policyContentStruct trust.PolicyContent
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
	var newReposContent []trust.RepoContent
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
		exists := false
		for transport, transportval := range policyContentStruct.Transports {
			_, exists = transportval[args[0]]
			if exists {
				policyContentStruct.Transports[transport][args[0]] = newReposContent
				break
			}
		}
		if !exists {
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

var typeDescription = map[string]string{"insecureAcceptAnything": "accept", "signedBy": "signed", "reject": "reject"}

func trustTypeDescription(trustType string) string {
	trustDescription, exist := typeDescription[trustType]
	if !exist {
		logrus.Warnf("invalid trust type %s", trustType)
	}
	return trustDescription
}

func sortPolicyJSONKey(m map[string]map[string]interface{}) []string {
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
