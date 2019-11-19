package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	initCommand     cliconfig.InitValues
	initDescription = `Initialize one or more containers, creating the OCI spec and mounts for inspection. Container names or IDs can be used.`

	_initCommand = &cobra.Command{
		Use:   "init [flags] CONTAINER [CONTAINER...]",
		Short: "Initialize one or more containers",
		Long:  initDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			initCommand.InputArgs = args
			initCommand.GlobalFlags = MainGlobalOpts
			initCommand.Remote = remoteclient
			return initCmd(&initCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman init --latest
  podman init 3c45ef19d893
  podman init test1`,
	}
)

func init() {
	initCommand.Command = _initCommand
	initCommand.SetHelpTemplate(HelpTemplate())
	initCommand.SetUsageTemplate(UsageTemplate())
	flags := initCommand.Flags()
	flags.BoolVarP(&initCommand.All, "all", "a", false, "Initialize all containers")
	flags.BoolVarP(&initCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
}

// initCmd initializes a container
func initCmd(c *cliconfig.InitValues) error {
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "initCmd")
		defer span.Finish()
	}

	ctx := getContext()

	runtime, err := adapter.GetRuntime(ctx, &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	ok, failures, err := runtime.InitContainers(ctx, c)
	if err != nil {
		return err
	}
	return printCmdResults(ok, failures)
}
