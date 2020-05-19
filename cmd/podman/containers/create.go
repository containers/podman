package containers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/libpod/define"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/cmd/podman/common"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	createDescription = `Creates a new container from the given image or storage and prepares it for running the specified command.

  The container ID is then printed to stdout. You can then start it at any time with the podman start <container_id> command. The container will be created with the initial state 'created'.`
	createCommand = &cobra.Command{
		Use:   "create [flags] IMAGE [COMMAND [ARG...]]",
		Short: "Create but do not start a container",
		Long:  createDescription,
		RunE:  create,
		Args:  cobra.MinimumNArgs(1),
		Example: `podman create alpine ls
  podman create --annotation HELLO=WORLD alpine ls
  podman create -t -i --name myctr alpine ls`,
	}

	containerCreateCommand = &cobra.Command{
		Args:  cobra.MinimumNArgs(1),
		Use:   createCommand.Use,
		Short: createCommand.Short,
		Long:  createCommand.Long,
		RunE:  createCommand.RunE,
		Example: `podman container create alpine ls
  podman container create --annotation HELLO=WORLD alpine ls
  podman container create -t -i --name myctr alpine ls`,
	}
)

var (
	cliVals common.ContainerCLIOpts
)

func createFlags(flags *pflag.FlagSet) {
	flags.SetInterspersed(false)
	flags.AddFlagSet(common.GetCreateFlags(&cliVals))
	flags.AddFlagSet(common.GetNetFlags())
	flags.SetNormalizeFunc(common.AliasFlags)
	if registry.IsRemote() {
		_ = flags.MarkHidden("authfile")
		_ = flags.MarkHidden("env-host")
		_ = flags.MarkHidden("http-proxy")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: createCommand,
	})
	// common.GetCreateFlags(createCommand)
	flags := createCommand.Flags()
	createFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerCreateCommand,
		Parent:  containerCmd,
	})

	containerCreateFlags := containerCreateCommand.Flags()
	createFlags(containerCreateFlags)
}

func create(cmd *cobra.Command, args []string) error {
	var (
		err error
	)
	cliVals.Net, err = common.NetFlagsToNetOptions(cmd)
	if err != nil {
		return err
	}
	cidFile, err := openCidFile(cliVals.CIDFile)
	if err != nil {
		return err
	}

	if cidFile != nil {
		defer errorhandling.CloseQuiet(cidFile)
		defer errorhandling.SyncQuiet(cidFile)
	}

	if err := createInit(cmd); err != nil {
		return err
	}

	if !cliVals.RootFS {
		if err := pullImage(args[0]); err != nil {
			return err
		}
	}
	s := specgen.NewSpecGenerator(args[0], cliVals.RootFS)
	if err := common.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}

	if _, err := createPodIfNecessary(s); err != nil {
		return err
	}

	report, err := registry.ContainerEngine().ContainerCreate(registry.GetContext(), s)
	if err != nil {
		return err
	}

	if cidFile != nil {
		_, err = cidFile.WriteString(report.Id)
		if err != nil {
			logrus.Error(err)
		}
	}

	fmt.Println(report.Id)
	return nil
}

func createInit(c *cobra.Command) error {
	if c.Flag("privileged").Changed && c.Flag("security-opt").Changed {
		logrus.Warn("setting security options with --privileged has no effect")
	}

	if c.Flag("shm-size").Changed {
		cliVals.ShmSize = c.Flag("shm-size").Value.String()
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
	if c.Flag("userns").Changed {
		cliVals.UserNS = c.Flag("userns").Value.String()
	}
	if c.Flag("ipc").Changed {
		cliVals.IPC = c.Flag("ipc").Value.String()
	}
	if c.Flag("uts").Changed {
		cliVals.UTS = c.Flag("uts").Value.String()
	}
	if c.Flag("pid").Changed {
		cliVals.PID = c.Flag("pid").Value.String()
	}
	if !c.Flag("pids-limit").Changed {
		cliVals.PIDsLimit = -1
	}
	if c.Flag("cgroupns").Changed {
		cliVals.CGroupsNS = c.Flag("cgroupns").Value.String()
	}
	if c.Flag("entrypoint").Changed {
		val := c.Flag("entrypoint").Value.String()
		cliVals.Entrypoint = &val
	}
	if c.Flags().Changed("env") {
		env, err := c.Flags().GetStringArray("env")
		if err != nil {
			return errors.Wrapf(err, "retrieve env flag")
		}
		cliVals.Env = env
	}

	// Docker-compatibility: the "-h" flag for run/create is reserved for
	// the hostname (see https://github.com/containers/libpod/issues/1367).

	return nil
}

func pullImage(imageName string) error {
	br, err := registry.ImageEngine().Exists(registry.GetContext(), imageName)
	if err != nil {
		return err
	}
	pullPolicy, err := config.ValidatePullPolicy(cliVals.Pull)
	if err != nil {
		return err
	}
	if !br.Value || pullPolicy == config.PullImageAlways {
		if pullPolicy == config.PullImageNever {
			return errors.Wrapf(define.ErrNoSuchImage, "unable to find a name and tag match for %s in repotags", imageName)
		}
		_, pullErr := registry.ImageEngine().Pull(registry.GetContext(), imageName, entities.ImagePullOptions{
			Authfile:     cliVals.Authfile,
			Quiet:        cliVals.Quiet,
			OverrideArch: cliVals.OverrideArch,
			OverrideOS:   cliVals.OverrideOS,
		})
		if pullErr != nil {
			return pullErr
		}
	}
	return nil
}

func openCidFile(cidfile string) (*os.File, error) {
	if cidfile == "" {
		return nil, nil
	}
	cidFile, err := util.OpenExclusiveFile(cidfile)
	if err != nil && os.IsExist(err) {
		return nil, errors.Errorf("container id file exists. Ensure another container is not using it or delete %s", cidfile)
	}
	if err != nil {
		return nil, errors.Errorf("error opening cidfile %s", cidfile)
	}
	return cidFile, nil
}

// createPodIfNecessary automatically creates a pod when requested.  if the pod name
// has the form new:ID, the pod ID is created and the name in the spec generator is replaced
// with ID.
func createPodIfNecessary(s *specgen.SpecGenerator) (*entities.PodCreateReport, error) {
	if !strings.HasPrefix(s.Pod, "new:") {
		return nil, nil
	}
	podName := strings.Replace(s.Pod, "new:", "", 1)
	if len(podName) < 1 {
		return nil, errors.Errorf("new pod name must be at least one character")
	}
	createOptions := entities.PodCreateOptions{
		Name:  podName,
		Infra: true,
		Net: &entities.NetOptions{
			PublishPorts: s.PortMappings,
		},
	}
	s.Pod = podName
	return registry.ContainerEngine().PodCreate(context.Background(), createOptions)
}
