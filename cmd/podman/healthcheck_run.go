package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	healthcheckRunCommand     cliconfig.HealthCheckValues
	healthcheckRunDescription = "run the health check of a container"
	_healthcheckrunCommand    = &cobra.Command{
		Use:     "run CONTAINER",
		Short:   "run the health check of a container",
		Long:    healthcheckRunDescription,
		Example: `podman healthcheck run mywebapp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			healthcheckRunCommand.InputArgs = args
			healthcheckRunCommand.GlobalFlags = MainGlobalOpts
			return healthCheckCmd(&healthcheckRunCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || len(args) > 1 {
				return errors.New("must provide the name or ID of one container")
			}
			return nil
		},
	}
)

func init() {
	healthcheckRunCommand.Command = _healthcheckrunCommand
	healthcheckRunCommand.SetUsageTemplate(UsageTemplate())
}

func healthCheckCmd(c *cliconfig.HealthCheckValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrap(err, "could not get runtime")
	}
	status, err := runtime.HealthCheck(c)
	if err != nil {
		if status == libpod.HealthCheckFailure {
			fmt.Println("\nunhealthy")
		}
		return err
	}
	fmt.Println("\nhealthy")
	return nil
}
