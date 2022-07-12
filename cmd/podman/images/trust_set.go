package images

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/util"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	setTrustDescription = "Set default trust policy or add a new trust policy for a registry"
	setTrustCommand     = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "set [options] REGISTRY",
		Short:             "Set default trust policy or a new trust policy for a registry",
		Long:              setTrustDescription,
		Example:           "",
		RunE:              setTrust,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteRegistries,
	}
)

var (
	setOptions entities.SetTrustOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: setTrustCommand,
		Parent:  trustCmd,
	})
	setFlags := setTrustCommand.Flags()
	setFlags.StringVar(&setOptions.PolicyPath, "policypath", "", "")
	_ = setFlags.MarkHidden("policypath")

	pubkeysfileFlagName := "pubkeysfile"
	setFlags.StringSliceVarP(&setOptions.PubKeysFile, pubkeysfileFlagName, "f", []string{}, `Path of installed public key(s) to trust for TARGET.
Absolute path to keys is added to policy.json. May
used multiple times to define multiple public keys.
File(s) must exist before using this command`)
	_ = setTrustCommand.RegisterFlagCompletionFunc(pubkeysfileFlagName, completion.AutocompleteDefault)

	typeFlagName := "type"
	setFlags.StringVarP(&setOptions.Type, typeFlagName, "t", "signedBy", "Trust type, accept values: signedBy(default), accept, reject")
	_ = setTrustCommand.RegisterFlagCompletionFunc(typeFlagName, common.AutocompleteTrustType)
}

func setTrust(cmd *cobra.Command, args []string) error {
	validTrustTypes := []string{"accept", "insecureAcceptAnything", "reject", "signedBy"}

	valid, err := isValidImageURI(args[0])
	if err != nil || !valid {
		return err
	}

	if !util.StringInSlice(setOptions.Type, validTrustTypes) {
		return fmt.Errorf("invalid choice: %s (choose from 'accept', 'reject', 'signedBy')", setOptions.Type)
	}
	return registry.ImageEngine().SetTrust(registry.Context(), args, setOptions)
}

// isValidImageURI checks if image name has valid format
func isValidImageURI(imguri string) (bool, error) {
	uri := "http://" + imguri
	u, err := url.Parse(uri)
	if err != nil {
		return false, fmt.Errorf("invalid image uri: %s: %w", imguri, err)
	}
	reg := regexp.MustCompile(`^[a-zA-Z0-9-_\.]+\/?:?[0-9]*[a-z0-9-\/:]*$`)
	ret := reg.FindAllString(u.Host, -1)
	if len(ret) == 0 {
		return false, fmt.Errorf("invalid image uri: %s: %w", imguri, err)
	}
	reg = regexp.MustCompile(`^[a-z0-9-:\./]*$`)
	ret = reg.FindAllString(u.Fragment, -1)
	if len(ret) == 0 {
		return false, fmt.Errorf("invalid image uri: %s: %w", imguri, err)
	}
	return true, nil
}
