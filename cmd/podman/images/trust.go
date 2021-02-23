package images

import (
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	trustDescription = `Manages which registries you trust as a source of container images based on their location.
  The location is determined by the transport and the registry host of the image.  Using this container image docker://quay.io/podman/stable as an example, docker is the transport and quay.io is the registry host.`
	trustCmd = &cobra.Command{
		Use:   "trust",
		Short: "Manage container image trust policy",
		Long:  trustDescription,
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: trustCmd,
		Parent:  imageCmd,
	})
}
