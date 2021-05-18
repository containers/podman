package volumes

import (
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	// Command: podman _volume_
	volumeCmd = &cobra.Command{
		Use:   "volume",
		Short: "Manage volumes",
		Long:  "Volumes are created in and can be shared between containers",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: volumeCmd,
	})
}
