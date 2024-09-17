package pods

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	// Command: podman _pod_
	podCmd = &cobra.Command{
		Use:   "pod",
		Short: "Manage pods",
		Long:  "Pods are a group of one or more containers sharing the same network, pid and ipc namespaces.",
		RunE:  validate.SubCommandExists,
	}
	containerConfig = registry.PodmanConfig().ContainersConfDefaultsRO
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: podCmd,
	})
}
