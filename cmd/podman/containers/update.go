package containers

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/containers/podman/v5/pkg/specgenutil"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/cobra"
)

var (
	updateDescription = `Updates the configuration of an existing container, allowing changes to resource limits and healthchecks`

	updateCommand = &cobra.Command{
		Use:               "update [options] CONTAINER",
		Short:             "Update an existing container",
		Long:              updateDescription,
		RunE:              update,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteContainers,
		Example:           `podman update --cpus=5 foobar_container`,
	}

	containerUpdateCommand = &cobra.Command{
		Args:              updateCommand.Args,
		Use:               updateCommand.Use,
		Short:             updateCommand.Short,
		Long:              updateCommand.Long,
		RunE:              updateCommand.RunE,
		ValidArgsFunction: updateCommand.ValidArgsFunction,
		Example:           `podman container update --cpus=5 foobar_container`,
	}
)
var (
	updateOpts entities.ContainerCreateOptions
)

func updateFlags(cmd *cobra.Command) {
	common.DefineCreateDefaults(&updateOpts)
	common.DefineCreateFlags(cmd, &updateOpts, entities.UpdateMode)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: updateCommand,
	})
	updateFlags(updateCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerUpdateCommand,
		Parent:  containerCmd,
	})
	updateFlags(containerUpdateCommand)
}

func GetChangedHealthCheckConfiguration(cmd *cobra.Command, vals *entities.ContainerCreateOptions) define.UpdateHealthCheckConfig {
	updateHealthCheckConfig := define.UpdateHealthCheckConfig{}

	if cmd.Flags().Changed("health-log-destination") {
		updateHealthCheckConfig.HealthLogDestination = &vals.HealthLogDestination
	}
	if cmd.Flags().Changed("health-max-log-size") {
		updateHealthCheckConfig.HealthMaxLogSize = &vals.HealthMaxLogSize
	}
	if cmd.Flags().Changed("health-max-log-count") {
		updateHealthCheckConfig.HealthMaxLogCount = &vals.HealthMaxLogCount
	}
	if cmd.Flags().Changed("health-on-failure") {
		updateHealthCheckConfig.HealthOnFailure = &vals.HealthOnFailure
	}
	if cmd.Flags().Changed("no-healthcheck") {
		updateHealthCheckConfig.NoHealthCheck = &vals.NoHealthCheck
	}
	if cmd.Flags().Changed("health-cmd") {
		updateHealthCheckConfig.HealthCmd = &vals.HealthCmd
	}
	if cmd.Flags().Changed("health-interval") {
		updateHealthCheckConfig.HealthInterval = &vals.HealthInterval
	}
	if cmd.Flags().Changed("health-retries") {
		updateHealthCheckConfig.HealthRetries = &vals.HealthRetries
	}
	if cmd.Flags().Changed("health-timeout") {
		updateHealthCheckConfig.HealthTimeout = &vals.HealthTimeout
	}
	if cmd.Flags().Changed("health-start-period") {
		updateHealthCheckConfig.HealthStartPeriod = &vals.HealthStartPeriod
	}
	if cmd.Flags().Changed("health-startup-cmd") {
		updateHealthCheckConfig.HealthStartupCmd = &vals.StartupHCCmd
	}
	if cmd.Flags().Changed("health-startup-interval") {
		updateHealthCheckConfig.HealthStartupInterval = &vals.StartupHCInterval
	}
	if cmd.Flags().Changed("health-startup-retries") {
		updateHealthCheckConfig.HealthStartupRetries = &vals.StartupHCRetries
	}
	if cmd.Flags().Changed("health-startup-timeout") {
		updateHealthCheckConfig.HealthStartupTimeout = &vals.StartupHCTimeout
	}
	if cmd.Flags().Changed("health-startup-success") {
		updateHealthCheckConfig.HealthStartupSuccess = &vals.StartupHCSuccesses
	}

	return updateHealthCheckConfig
}

func GetChangedDeviceLimits(s *specgen.SpecGenerator) *define.UpdateContainerDevicesLimits {
	updateDevicesLimits := define.UpdateContainerDevicesLimits{}
	updateDevicesLimits.SetBlkIOWeightDevice(s.WeightDevice)
	updateDevicesLimits.SetDeviceReadBPs(s.ThrottleReadBpsDevice)
	updateDevicesLimits.SetDeviceWriteBPs(s.ThrottleWriteBpsDevice)
	updateDevicesLimits.SetDeviceReadIOPs(s.ThrottleReadIOPSDevice)
	updateDevicesLimits.SetDeviceWriteIOPs(s.ThrottleWriteIOPSDevice)
	return &updateDevicesLimits
}

func update(cmd *cobra.Command, args []string) error {
	var err error
	// use a specgen since this is the easiest way to hold resource info
	s := &specgen.SpecGenerator{}
	s.ResourceLimits = &specs.LinuxResources{}

	err = createOrUpdateFlags(cmd, &updateOpts)
	if err != nil {
		return err
	}

	s.ResourceLimits, err = specgenutil.GetResources(s, &updateOpts)
	if err != nil {
		return err
	}

	if s.ResourceLimits == nil {
		s.ResourceLimits = &specs.LinuxResources{}
	}

	healthCheckConfig := GetChangedHealthCheckConfiguration(cmd, &updateOpts)

	opts := &entities.ContainerUpdateOptions{
		NameOrID:                        strings.TrimPrefix(args[0], "/"),
		Resources:                       s.ResourceLimits,
		ChangedHealthCheckConfiguration: &healthCheckConfig,
		DevicesLimits:                   GetChangedDeviceLimits(s),
	}

	if cmd.Flags().Changed("restart") {
		policy, retries, err := util.ParseRestartPolicy(updateOpts.Restart)
		if err != nil {
			return err
		}
		opts.RestartPolicy = &policy
		if policy == define.RestartPolicyOnFailure {
			opts.RestartRetries = &retries
		}
	}

	rep, err := registry.ContainerEngine().ContainerUpdate(context.Background(), opts)
	if err != nil {
		return err
	}
	fmt.Println(rep)
	return nil
}
