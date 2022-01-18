package healthcheck

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	healthCmd = &cobra.Command{
		Use:   "healthcheck",
		Short: "Manage health checks on containers",
		Long:  "Run health checks on containers",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: healthCmd,
	})
}
