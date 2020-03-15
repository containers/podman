package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	podmanVersion "github.com/containers/libpod/version"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	containerKubeCommand     cliconfig.GenerateKubeValues
	containerKubeDescription = `Command generates Kubernetes Pod YAML (v1 specification) from a podman container or pod.

  Whether the input is for a container or pod, Podman will always generate the specification as a Pod. The input may be in the form of a pod or container name or ID.`
	_containerKubeCommand = &cobra.Command{
		Use:   "kube [flags] CONTAINER | POD",
		Short: "Generate Kubernetes pod YAML from a container or pod",
		Long:  containerKubeDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			containerKubeCommand.InputArgs = args
			containerKubeCommand.GlobalFlags = MainGlobalOpts
			containerKubeCommand.Remote = remoteclient
			return generateKubeYAMLCmd(&containerKubeCommand)
		},
		Example: `podman generate kube ctrID
  podman generate kube podID
  podman generate kube --service podID`,
	}
)

func init() {
	containerKubeCommand.Command = _containerKubeCommand
	containerKubeCommand.SetHelpTemplate(HelpTemplate())
	containerKubeCommand.SetUsageTemplate(UsageTemplate())
	flags := containerKubeCommand.Flags()
	flags.BoolVarP(&containerKubeCommand.Service, "service", "s", false, "Generate YAML for kubernetes service object")
	flags.StringVarP(&containerKubeCommand.Filename, "filename", "f", "", "Filename to output to")
}

func generateKubeYAMLCmd(c *cliconfig.GenerateKubeValues) error {
	var (
		//podYAML           *v1.Pod
		err    error
		output []byte
		//pod               *libpod.Pod
		marshalledPod     []byte
		marshalledService []byte
	)

	args := c.InputArgs
	if len(args) != 1 {
		return errors.Errorf("you must provide exactly one container|pod ID or name")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	podYAML, serviceYAML, err := runtime.GenerateKube(c)
	if err != nil {
		return err
	}
	// Marshall the results
	marshalledPod, err = yaml.Marshal(podYAML)
	if err != nil {
		return err
	}
	if c.Service {
		marshalledService, err = yaml.Marshal(serviceYAML)
		if err != nil {
			return err
		}
	}
	header := `# Generation of Kubernetes YAML is still under development!
#
# Save the output of this file and use kubectl create -f to import
# it into Kubernetes.
#
# Created with podman-%s
`
	output = append(output, []byte(fmt.Sprintf(header, podmanVersion.Version))...)
	output = append(output, marshalledPod...)
	if c.Bool("service") {
		output = append(output, []byte("---\n")...)
		output = append(output, marshalledService...)
	}

	if c.Filename != "" {
		if _, err := os.Stat(c.Filename); err == nil {
			return errors.Errorf("cannot write to %q - file exists", c.Filename)
		}

		if err := ioutil.WriteFile(c.Filename, output, 0644); err != nil {
			return err
		}
	} else {
		// Output the v1.Pod with the v1.Container
		fmt.Println(string(output))
	}

	return nil
}
