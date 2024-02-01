package kube

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _kube_
	kubeCmd = &cobra.Command{
		Use:   "kube",
		Short: "Play containers, pods or volumes from a structured file",
		Long:  "Play structured data (e.g., Kubernetes YAML) based on containers, pods or volumes.",
		RunE:  validate.SubCommandExists,
	}
	// Command: podman _play_
	playKubeParentCmd = &cobra.Command{
		Use:    "play",
		Short:  "Play containers, pods or volumes from a structured file",
		Long:   "Play structured data (e.g., Kubernetes YAML) based on containers, pods or volumes.",
		Hidden: true,
		RunE:   validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: kubeCmd,
	})

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: playKubeParentCmd,
	})
}
