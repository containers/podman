//go:build (amd64 || arm64) && experimental
// +build amd64 arm64
// +build experimental

package machine

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	OSCmd = &cobra.Command{
		Use:               "os",
		Short:             "Manage a virtual machine's os",
		Long:              "Manage a virtual machine's operating system",
		PersistentPreRunE: validate.NoOp,
		RunE:              validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: OSCmd,
		Parent:  machineCmd,
	})
}
