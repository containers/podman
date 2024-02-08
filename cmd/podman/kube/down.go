package kube

import (
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

type downKubeOptions struct {
	Force bool
}

var (
	downDescription = `Reads in a structured file of Kubernetes YAML.

  Removes pods that have been based on the Kubernetes kind described in the YAML.`

	downCmd = &cobra.Command{
		Use:               "down [options] KUBEFILE|-",
		Short:             "Remove pods based on Kubernetes YAML",
		Long:              downDescription,
		RunE:              down,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman kube down nginx.yml
   cat nginx.yml | podman kube down -
   podman kube down https://example.com/nginx.yml`,
	}

	downOptions = downKubeOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: downCmd,
		Parent:  kubeCmd,
	})
	downFlags(downCmd)
}

func downFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.SetNormalizeFunc(utils.AliasFlags)

	flags.BoolVar(&downOptions.Force, "force", false, "remove volumes")
}

func down(cmd *cobra.Command, args []string) error {
	reader, err := readerFromArg(args[0])
	if err != nil {
		return err
	}
	return teardown(reader, entities.PlayKubeDownOptions{Force: downOptions.Force})
}
