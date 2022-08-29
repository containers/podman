package kube

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/spf13/cobra"
)

var (
	downDescription = `Reads in a structured file of Kubernetes YAML.

  Removes pods that have been based on the Kubernetes kind described in the YAML.`

	downCmd = &cobra.Command{
		Use:               "down KUBEFILE|-",
		Short:             "Remove pods based on Kubernetes YAML.",
		Long:              downDescription,
		RunE:              down,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman kube down nginx.yml
   cat nginx.yml | podman kube down -
   podman kube down https://example.com/nginx.yml`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: downCmd,
		Parent:  kubeCmd,
	})
}

func down(cmd *cobra.Command, args []string) error {
	reader, err := readerFromArg(args[0])
	if err != nil {
		return err
	}
	return teardown(reader)
}
