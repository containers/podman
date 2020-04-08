package containers

import (
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/common"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	createDescription = `Creates a new container from the given image or storage and prepares it for running the specified command.

  The container ID is then printed to stdout. You can then start it at any time with the podman start <container_id> command. The container will be created with the initial state 'created'.`
	createCommand = &cobra.Command{
		Use:               "create [flags] IMAGE [COMMAND [ARG...]]",
		Short:             "Create but do not start a container",
		Long:              createDescription,
		RunE:              create,
		PersistentPreRunE: preRunE,
		Args:              cobra.MinimumNArgs(1),
		Example: `podman create alpine ls
  podman create --annotation HELLO=WORLD alpine ls
  podman create -t -i --name myctr alpine ls`,
	}
)

var (
	cliVals common.ContainerCLIOpts
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: createCommand,
	})
	//common.GetCreateFlags(createCommand)
	flags := createCommand.Flags()
	flags.SetInterspersed(false)
	flags.AddFlagSet(common.GetCreateFlags(&cliVals))
	flags.AddFlagSet(common.GetNetFlags())
	flags.SetNormalizeFunc(common.AliasFlags)
}

func create(cmd *cobra.Command, args []string) error {
	var (
		err           error
		rawImageInput string
	)
	cliVals.Net, err = common.NetFlagsToNetOptions(cmd)
	if err != nil {
		return err
	}
	if rfs := cliVals.RootFS; !rfs {
		rawImageInput = args[0]
	}

	if err := createInit(cmd); err != nil {
		return err
	}
	//TODO rootfs still
	s := specgen.NewSpecGenerator(rawImageInput)
	if err := common.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}

	report, err := registry.ContainerEngine().ContainerCreate(registry.GetContext(), s)
	if err != nil {
		return err
	}
	fmt.Println(report.Id)
	return nil
}

func createInit(c *cobra.Command) error {
	if c.Flag("privileged").Changed && c.Flag("security-opt").Changed {
		logrus.Warn("setting security options with --privileged has no effect")
	}

	if (c.Flag("dns").Changed || c.Flag("dns-opt").Changed || c.Flag("dns-search").Changed) && (cliVals.Net.Network.NSMode == specgen.NoNetwork || cliVals.Net.Network.IsContainer()) {
		return errors.Errorf("conflicting options: dns and the network mode.")
	}

	if c.Flag("cpu-period").Changed && c.Flag("cpus").Changed {
		return errors.Errorf("--cpu-period and --cpus cannot be set together")
	}
	if c.Flag("cpu-quota").Changed && c.Flag("cpus").Changed {
		return errors.Errorf("--cpu-quota and --cpus cannot be set together")
	}

	if c.Flag("no-hosts").Changed && c.Flag("add-host").Changed {
		return errors.Errorf("--no-hosts and --add-host cannot be set together")
	}

	// Docker-compatibility: the "-h" flag for run/create is reserved for
	// the hostname (see https://github.com/containers/libpod/issues/1367).

	return nil
}
