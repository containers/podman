package down

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// playKubeOptionsWrapper allows for separating CLI-only fields from API-only
// fields.
type playKubeOptionsWrapper struct {
	entities.PlayKubeDownOptions
}

var (
	kubeOptions     = playKubeOptionsWrapper{}
	kubeDescription = `Command stops pods or volumes defined in Kubernetes YAML.

  It stops pods or volumes created from a Kubernetes YAML. Supported kinds are Pods, Deployments and PersistentVolumeClaims.`

	kubeCmd = &cobra.Command{
		Use:               "kube [options] KUBEFILE|-",
		Short:             "Stop a pod or volume based on Kubernetes YAML.",
		Long:              kubeDescription,
		RunE:              kube,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman down kube nginx.yml
  cat nginx.yml | podman down kube -
  podman down kube apache.yml`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: kubeCmd,
		Parent:  downCmd,
	})

	flags := kubeCmd.Flags()
	flags.SetNormalizeFunc(utils.AliasFlags)
}

func kube(cmd *cobra.Command, args []string) error {
	yamlfile := args[0]
	if yamlfile == "-" {
		yamlfile = "/dev/stdin"
	}

	return common.TeardownPlayKube(yamlfile, kubeOptions.PlayKubeDownOptions)
}
