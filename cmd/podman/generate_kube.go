package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	podmanVersion "github.com/containers/libpod/version"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"k8s.io/api/core/v1"
)

var (
	containerKubeFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "service, s",
			Usage: "only generate YAML for kubernetes service object",
		},
	}
	containerKubeDescription = "Generate Kubernetes Pod YAML"
	containerKubeCommand     = cli.Command{
		Name:                   "kube",
		Usage:                  "Generate Kubernetes pod YAML for a container or pod",
		Description:            containerKubeDescription,
		Flags:                  sortFlags(containerKubeFlags),
		Action:                 generateKubeYAMLCmd,
		ArgsUsage:              "CONTAINER|POD-NAME",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

// generateKubeYAMLCmdgenerates or replays kube
func generateKubeYAMLCmd(c *cli.Context) error {
	var (
		podYAML        *v1.Pod
		container      *libpod.Container
		err            error
		output         []byte
		pod            *libpod.Pod
		mashalledBytes []byte
		servicePorts   []v1.ServicePort
	)

	if rootless.IsRootless() {
		return errors.Wrapf(libpod.ErrNotImplemented, "rootless users")
	}
	args := c.Args()
	if len(args) > 1 || (len(args) < 1 && !c.Bool("latest")) {
		return errors.Errorf("you must provide one container|pod ID or name or --latest")
	}

	runtime, err := libpodruntime.GetRuntime(c)
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

	if c.Bool("service") {
		serviceYAML := libpod.GenerateKubeServiceFromV1Pod(podYAML, servicePorts)
		mashalledBytes, err = yaml.Marshal(serviceYAML)
	} else {
		// Marshall the results
		mashalledBytes, err = yaml.Marshal(podYAML)
	}
	if err != nil {
		return err
	}

	header := `# Generation of Kubenetes YAML is still under development!
#
# Save the output of this file and use kubectl create -f to import
# it into Kubernetes.
#
# Created with podman-%s
`
	output = append(output, []byte(fmt.Sprintf(header, podmanVersion.Version))...)
	output = append(output, mashalledBytes...)
	// Output the v1.Pod with the v1.Container
	fmt.Println(string(output))

	return nil
}
