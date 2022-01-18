package images

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	trustDescription = `Manages which registries you trust as a source of container images based on their location.
  The location is determined by the transport and the registry host of the image.  Using this container image docker://quay.io/podman/stable as an example, docker is the transport and quay.io is the registry host.`
	trustCmd = &cobra.Command{
		Annotations: map[string]string{registry.EngineMode: registry.ABIMode},
		Use:         "trust",
		Short:       "Manage container image trust policy",
		Long:        trustDescription,
		RunE:        validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: trustCmd,
		Parent:  imageCmd,
	})
}
