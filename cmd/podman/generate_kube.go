package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	podmanVersion "github.com/containers/libpod/version"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
)

var (
	containerKubeCommand     cliconfig.GenerateKubeValues
	containerKubeDescription = "Generate Kubernetes Pod YAML"
	_containerKubeCommand    = &cobra.Command{
		Use:   "kube [flags] CONTAINER | POD",
		Short: "Generate Kubernetes pod YAML for a container or pod",
		Long:  containerKubeDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			containerKubeCommand.InputArgs = args
			containerKubeCommand.GlobalFlags = MainGlobalOpts
			return generateKubeYAMLCmd(&containerKubeCommand)
		},
		Example: `podman generate kube ctrID
  podman generate kube podID
  podman generate kube --service podID`,
	}
)

func init() {
	containerKubeCommand.Command = _containerKubeCommand
	containerKubeCommand.SetUsageTemplate(UsageTemplate())
	flags := containerKubeCommand.Flags()
	flags.BoolVarP(&containerKubeCommand.Service, "service", "s", false, "Generate YAML for kubernetes service object")
}

func generateKubeYAMLCmd(c *cliconfig.GenerateKubeValues) error {
	var (
		podYAML           *v1.Pod
		container         *libpod.Container
		err               error
		output            []byte
		pod               *libpod.Pod
		marshalledPod     []byte
		marshalledService []byte
		servicePorts      []v1.ServicePort
	)

	if rootless.IsRootless() {
		return errors.Wrapf(libpod.ErrNotImplemented, "rootless users")
	}
	args := c.InputArgs
	if len(args) > 1 || (len(args) < 1 && !c.Bool("latest")) {
		return errors.Errorf("you must provide one container|pod ID or name or --latest")
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	// Get the container in question
	container, err = runtime.LookupContainer(args[0])
	if err != nil {
		pod, err = runtime.LookupPod(args[0])
		if err != nil {
			return err
		}
		podYAML, servicePorts, err = pod.GenerateForKube()
	} else {
		if len(container.Dependencies()) > 0 {
			return errors.Wrapf(libpod.ErrNotImplemented, "containers with dependencies")
		}
		podYAML, err = container.GenerateForKube()
	}
	if err != nil {
		return err
	}

	if c.Service {
		serviceYAML := libpod.GenerateKubeServiceFromV1Pod(podYAML, servicePorts)
		marshalledService, err = yaml.Marshal(serviceYAML)
		if err != nil {
			return err
		}
	}
	// Marshall the results
	marshalledPod, err = yaml.Marshal(podYAML)
	if err != nil {
		return err
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
	// Output the v1.Pod with the v1.Container
	fmt.Println(string(output))

	return nil
}
