package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	createCommand     cliconfig.CreateValues
	createDescription = `Creates a new container from the given image or storage and prepares it for running the specified command.

  The container ID is then printed to stdout. You can then start it at any time with the podman start <container_id> command. The container will be created with the initial state 'created'.`
	_createCommand = &cobra.Command{
		Use:   "create [flags] IMAGE [COMMAND [ARG...]]",
		Short: "Create but do not start a container",
		Long:  createDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			createCommand.InputArgs = args
			createCommand.GlobalFlags = MainGlobalOpts
			createCommand.Remote = remoteclient
			return createCmd(&createCommand)
		},
		Example: `podman create alpine ls
  podman create --annotation HELLO=WORLD alpine ls
  podman create -t -i --name myctr alpine ls`,
	}
)

func init() {
	createCommand.PodmanCommand.Command = _createCommand
	createCommand.SetHelpTemplate(HelpTemplate())
	createCommand.SetUsageTemplate(UsageTemplate())

	getCreateFlags(&createCommand.PodmanCommand)
	flags := createCommand.Flags()
	flags.SetInterspersed(false)

}

func createCmd(c *cliconfig.CreateValues) error {
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "createCmd")
		defer span.Finish()
	}

	if err := createInit(&c.PodmanCommand); err != nil {
		return err
	}

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	cid, err := runtime.CreateContainer(getContext(), c)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", cid)
	return nil
}

func createInit(c *cliconfig.PodmanCommand) error {
	if !remote && c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "createInit")
		defer span.Finish()
	}

	// Docker-compatibility: the "-h" flag for run/create is reserved for
	// the hostname (see https://github.com/containers/libpod/issues/1367).

	if len(c.InputArgs) < 1 {
		return errors.Errorf("image name or ID is required")
	}

	return nil
}
