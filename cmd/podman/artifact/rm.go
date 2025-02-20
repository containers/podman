package artifact

import (
	"errors"
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	rmCmd = &cobra.Command{
		Use:     "rm [options] ARTIFACT",
		Short:   "Remove an OCI artifact",
		Long:    "Remove an OCI artifact from local storage",
		RunE:    rm,
		Aliases: []string{"remove"},
		Args: func(cmd *cobra.Command, args []string) error { //nolint: gocritic
			return checkAllAndArgs(cmd, args)
		},
		ValidArgsFunction: common.AutocompleteArtifacts,
		Example: `podman artifact rm quay.io/myimage/myartifact:latest
podman artifact rm -a`,
		Annotations: map[string]string{registry.EngineMode: registry.ABIMode},
	}

	rmOptions = entities.ArtifactRemoveOptions{}
)

func rmFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all artifacts")
}
func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  artifactCmd,
	})
	rmFlags(rmCmd)
}

func rm(cmd *cobra.Command, args []string) error {
	var nameOrID string
	if len(args) > 0 {
		nameOrID = args[0]
	}
	artifactRemoveReport, err := registry.ImageEngine().ArtifactRm(registry.Context(), nameOrID, rmOptions)
	if err != nil {
		return err
	}
	for _, d := range artifactRemoveReport.ArtifactDigests {
		fmt.Println(d.Encoded())
	}
	return nil
}

// checkAllAndArgs takes a cobra command and args and checks if
// all is used, then no args can be passed. note: this was created
// as an unexported local func for now and could be moved to pkg
// validate.  if we add "--latest" to the command, then perhaps
// one of the existing plg validate funcs would be appropriate.
func checkAllAndArgs(c *cobra.Command, args []string) error {
	all, _ := c.Flags().GetBool("all")
	if all && len(args) > 0 {
		return fmt.Errorf("when using the --all switch, you may not pass any artifact names or digests")
	}
	if !all {
		if len(args) < 1 {
			return errors.New("a single artifact name or digest must be specified")
		}
		if len(args) > 1 {
			return errors.New("too many arguments: only accepts one artifact name or digest ")
		}
	}
	return nil
}
