//go:build (amd64 || arm64) && experimental
// +build amd64 arm64
// +build experimental

package machineos

import (
	"fmt"

	"github.com/containers/podman/v4/cmd/podman/machine"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	applyCmd = &cobra.Command{
		Use:               "apply",
		Short:             "Apply OCI image to existing VM",
		Long:              "Apply custom layers from a containerized Fedora CoreOS image on top of an existing VM",
		PersistentPreRunE: validate.NoOp,
		RunE:              apply,
		Example:           `podman machine os apply myimage`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: applyCmd,
		Parent:  machine.OSCmd,
	})

}

func apply(cmd *cobra.Command, args []string) error {
	fmt.Println("Applying..")
	return nil
}
