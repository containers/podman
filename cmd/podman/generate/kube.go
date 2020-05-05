package pods

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	kubeOptions     = entities.GenerateKubeOptions{}
	kubeFile        = ""
	kubeDescription = `Command generates Kubernetes pod and service YAML (v1 specification) from a podman container or pod.

Whether the input is for a container or pod, Podman will always generate the specification as a pod.`

	kubeCmd = &cobra.Command{
		Use:   "kube [flags] CONTAINER | POD",
		Short: "Generate Kubernetes YAML from a container or pod.",
		Long:  kubeDescription,
		RunE:  kube,
		Args:  cobra.ExactArgs(1),
		Example: `podman generate kube ctrID
  podman generate kube podID
  podman generate kube --service podID`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: kubeCmd,
		Parent:  generateCmd,
	})
	flags := kubeCmd.Flags()
	flags.BoolVarP(&kubeOptions.Service, "service", "s", false, "Generate YAML for a Kubernetes service object")
	flags.StringVarP(&kubeFile, "filename", "f", "", "Write output to the specified path")
	flags.SetNormalizeFunc(utils.AliasFlags)
}

func kube(cmd *cobra.Command, args []string) error {
	report, err := registry.ContainerEngine().GenerateKube(registry.GetContext(), args[0], kubeOptions)
	if err != nil {
		return err
	}

	if cmd.Flags().Changed("filename") {
		if _, err := os.Stat(kubeFile); err == nil {
			return errors.Errorf("cannot write to %q - file exists", kubeFile)
		}

		if err := ioutil.WriteFile(kubeFile, []byte(report.Output), 0644); err != nil {
			return err
		}
		return nil
	}

	fmt.Println(report.Output)
	return nil
}
