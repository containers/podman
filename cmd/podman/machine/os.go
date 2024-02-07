//go:build amd64 || arm64

package machine

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	OSCmd = &cobra.Command{
		Use:               "os",
		Short:             "Manage a Podman virtual machine's OS",
		Long:              "Manage a Podman virtual machine's operating system",
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
