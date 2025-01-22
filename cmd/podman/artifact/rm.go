package artifact

import (
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	rmCmd = &cobra.Command{
		Use:               "rm ARTIFACT",
		Short:             "Remove an OCI artifact",
		Long:              "Remove an OCI from local storage",
		RunE:              rm,
		Aliases:           []string{"remove"},
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteArtifacts,
		Example:           `podman artifact rm quay.io/myimage/myartifact:latest`,
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
	}
	// The lint avoid here is because someday soon we will need flags for
	// this command
	rmFlag = rmFlagType{} //nolint:unused
)

// TODO at some point force will be a required option; but this cannot be
// until we have artifacts being consumed by other parts of libpod like
// volumes
type rmFlagType struct { //nolint:unused
	force bool
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  artifactCmd,
	})
}

func rm(cmd *cobra.Command, args []string) error {
	artifactRemoveReport, err := registry.ImageEngine().ArtifactRm(registry.GetContext(), args[0], entities.ArtifactRemoveOptions{})
	if err != nil {
		return err
	}
	fmt.Println(artifactRemoveReport.ArtfactDigest.Encoded())
	return nil
}
