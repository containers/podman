package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	containerKubeFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "service, s",
			Usage: "only generate YAML for kubernetes service object",
		},
		LatestFlag,
	}
	containerKubeDescription = "Generate Kubernetes Pod YAML"
	containerKubeCommand     = cli.Command{
		Name:                   "generate",
		Usage:                  "Generate Kubernetes pod YAML for a container",
		Description:            containerKubeDescription,
		Flags:                  sortFlags(containerKubeFlags),
		Action:                 generateKubeYAMLCmd,
		ArgsUsage:              "CONTAINER-NAME",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

// generateKubeYAMLCmdgenerates or replays kube
func generateKubeYAMLCmd(c *cli.Context) error {
	var (
		container *libpod.Container
		err       error
		output    []byte
	)

	if rootless.IsRootless() {
		return errors.Wrapf(libpod.ErrNotImplemented, "rootless users")
	}
	args := c.Args()
	if len(args) > 1 || (len(args) < 1 && !c.Bool("latest")) {
		return errors.Errorf("you must provide one container ID or name or --latest")
	}
	if c.Bool("service") {
		return errors.Wrapf(libpod.ErrNotImplemented, "service generation")
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	// Get the container in question
	if c.Bool("latest") {
		container, err = runtime.GetLatestContainer()
	} else {
		container, err = runtime.LookupContainer(args[0])
	}
	if err != nil {
		return err
	}

	if len(container.Dependencies()) > 0 {
		return errors.Wrapf(libpod.ErrNotImplemented, "containers with dependencies")
	}

	podYAML, err := container.InspectForKube()
	if err != nil {
		return err
	}

	developmentComment := []byte("# Generation of Kubenetes YAML is still under development!\n")
	logrus.Warn("This function is still under heavy development.")
	// Marshall the results
	b, err := yaml.Marshal(podYAML)
	if err != nil {
		return err
	}
	output = append(output, developmentComment...)
	output = append(output, b...)
	// Output the v1.Pod with the v1.Container
	fmt.Println(string(output))

	return nil
}
