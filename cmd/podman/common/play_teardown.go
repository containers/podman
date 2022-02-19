package common

import (
	"fmt"
	"os"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

// TeardownPlayKube tears down a Kubernetes Play based on its YAML
// configuration. It is in the common package because it is shared
// by "podman play kube --down" and "podman down kube"
func TeardownPlayKube(yamlfile string, opts entities.PlayKubeDownOptions) error {
	var (
		podStopErrors utils.OutputErrors
		podRmErrors   utils.OutputErrors
	)

	reports, err := registry.ContainerEngine().PlayKubeDown(registry.GetContext(), yamlfile, opts)
	if err != nil {
		return err
	}

	// Output stopped pods
	fmt.Println("Pods stopped:")
	for _, stopped := range reports.StopReport {
		if len(stopped.Errs) == 0 {
			fmt.Println(stopped.Id)
		} else {
			podStopErrors = append(podStopErrors, stopped.Errs...)
		}
	}
	// Dump any stop errors
	lastStopError := podStopErrors.PrintErrors()
	if lastStopError != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", lastStopError)
	}

	// Output rm'd pods
	fmt.Println("Pods removed:")
	for _, removed := range reports.RmReport {
		if removed.Err == nil {
			fmt.Println(removed.Id)
		} else {
			podRmErrors = append(podRmErrors, removed.Err)
		}
	}
	return podRmErrors.PrintErrors()
}
