package artifact

import (
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	addCmd = &cobra.Command{
		Use:               "add ARTIFACT PATH [...PATH]",
		Short:             "Add an OCI artifact to the local store",
		Long:              "Add an OCI artifact to the local store from the local filesystem",
		RunE:              add,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: common.AutocompleteArtifactAdd,
		Example:           `podman artifact add quay.io/myimage/myartifact:latest /tmp/foobar.txt`,
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: addCmd,
		Parent:  artifactCmd,
	})
}

func add(cmd *cobra.Command, args []string) error {
	report, err := registry.ImageEngine().ArtifactAdd(registry.Context(), args[0], args[1:], entities.ArtifactAddoptions{})
	if err != nil {
		return err
	}
	fmt.Println(report.ArtifactDigest.Encoded())
	return nil
}
