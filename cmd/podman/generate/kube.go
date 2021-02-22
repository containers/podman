package pods

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	kubeOptions     = entities.GenerateKubeOptions{}
	kubeFile        = ""
	kubeDescription = `Command generates Kubernetes pod and service YAML (v1 specification) from Podman containers or a pod.

Whether the input is for a container or pod, Podman will always generate the specification as a pod.`

	kubeCmd = &cobra.Command{
		Use:               "kube [options] {CONTAINER...|POD}",
		Short:             "Generate Kubernetes YAML from a container or pod.",
		Long:              kubeDescription,
		RunE:              kube,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteContainersAndPods,
		Example: `podman generate kube ctrID
  podman generate kube podID
  podman generate kube --service podID`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: kubeCmd,
		Parent:  generateCmd,
	})
	flags := kubeCmd.Flags()
	flags.BoolVarP(&kubeOptions.Service, "service", "s", false, "Generate YAML for a Kubernetes service object")

	filenameFlagName := "filename"
	flags.StringVarP(&kubeFile, filenameFlagName, "f", "", "Write output to the specified path")
	_ = kubeCmd.RegisterFlagCompletionFunc(filenameFlagName, completion.AutocompleteDefault)

	flags.SetNormalizeFunc(utils.AliasFlags)
}

func kube(cmd *cobra.Command, args []string) error {
	report, err := registry.ContainerEngine().GenerateKube(registry.GetContext(), args, kubeOptions)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadAll(report.Reader)
	if err != nil {
		return err
	}
	if cmd.Flags().Changed("filename") {
		if _, err := os.Stat(kubeFile); err == nil {
			return errors.Errorf("cannot write to %q; file exists", kubeFile)
		}
		if err := ioutil.WriteFile(kubeFile, content, 0644); err != nil {
			return errors.Wrapf(err, "cannot write to %q", kubeFile)
		}
		return nil
	}

	fmt.Println(string(content))
	return nil
}
