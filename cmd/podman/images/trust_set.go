package images

import (
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	setTrustDescription = "Set default trust policy or add a new trust policy for a registry"
	setTrustCommand     = &cobra.Command{
		Use:     "set [options] REGISTRY",
		Short:   "Set default trust policy or a new trust policy for a registry",
		Long:    setTrustDescription,
		Example: "",
		RunE:    setTrust,
		Args:    cobra.ExactArgs(1),
	}
)

var (
	setOptions entities.SetTrustOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: setTrustCommand,
		Parent:  trustCmd,
	})
	setFlags := setTrustCommand.Flags()
	setFlags.StringVar(&setOptions.PolicyPath, "policypath", "", "")
	_ = setFlags.MarkHidden("policypath")
	setFlags.StringSliceVarP(&setOptions.PubKeysFile, "pubkeysfile", "f", []string{}, `Path of installed public key(s) to trust for TARGET.
Absolute path to keys is added to policy.json. May
used multiple times to define multiple public keys.
File(s) must exist before using this command`)
	setFlags.StringVarP(&setOptions.Type, "type", "t", "signedBy", "Trust type, accept values: signedBy(default), accept, reject")
}

func setTrust(cmd *cobra.Command, args []string) error {
	validTrustTypes := []string{"accept", "insecureAcceptAnything", "reject", "signedBy"}

	valid, err := image.IsValidImageURI(args[0])
	if err != nil || !valid {
		return errors.Wrapf(err, "invalid image uri %s", args[0])
	}

	if !util.StringInSlice(setOptions.Type, validTrustTypes) {
		return errors.Errorf("invalid choice: %s (choose from 'accept', 'reject', 'signedBy')", setOptions.Type)
	}
	return registry.ImageEngine().SetTrust(registry.Context(), args, setOptions)
}
