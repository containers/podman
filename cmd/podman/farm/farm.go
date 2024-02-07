package farm

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _farm_
	farmCmd = &cobra.Command{
		Use:   "farm",
		Short: "Farm out builds to remote machines",
		Long:  "Farm out builds to remote machines that podman can connect to via podman system connection",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: farmCmd,
	})
}
