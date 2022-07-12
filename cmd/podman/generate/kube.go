package pods

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	kubeOptions     = entities.GenerateKubeOptions{}
	kubeFile        = ""
	kubeDescription = `Command generates Kubernetes Pod, Service or PersistenVolumeClaim YAML (v1 specification) from Podman containers, pods or volumes.

  Whether the input is for a container or pod, Podman will always generate the specification as a pod.`

	kubeCmd = &cobra.Command{
		Use:               "kube [options] {CONTAINER...|POD...|VOLUME...}",
		Short:             "Generate Kubernetes YAML from containers, pods or volumes.",
		Long:              kubeDescription,
		RunE:              kube,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteForKube,
		Example: `podman generate kube ctrID
  podman generate kube podID
  podman generate kube --service podID
  podman generate kube volumeName
  podman generate kube ctrID podID volumeName --service`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
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
	if r, ok := report.Reader.(io.ReadCloser); ok {
		defer r.Close()
	}

	if cmd.Flags().Changed("filename") {
		if _, err := os.Stat(kubeFile); err == nil {
			return fmt.Errorf("cannot write to %q; file exists", kubeFile)
		}
		if err := ioutil.WriteFile(kubeFile, content, 0644); err != nil {
			return fmt.Errorf("cannot write to %q: %w", kubeFile, err)
		}
		return nil
	}

	fmt.Println(string(content))
	return nil
}
