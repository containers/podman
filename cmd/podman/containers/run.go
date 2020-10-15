package containers

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/utils"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/errorhandling"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	runDescription = "Runs a command in a new container from the given image"
	runCommand     = &cobra.Command{
		Args:  cobra.MinimumNArgs(1),
		Use:   "run [options] IMAGE [COMMAND [ARG...]]",
		Short: "Run a command in a new container",
		Long:  runDescription,
		RunE:  run,
		Example: `podman run imageID ls -alF /etc
  podman run --network=host imageID dnf -y install java
  podman run --volume /var/hostdir:/var/ctrdir -i -t fedora /bin/bash`,
	}

	containerRunCommand = &cobra.Command{
		Args:  cobra.MinimumNArgs(1),
		Use:   runCommand.Use,
		Short: runCommand.Short,
		Long:  runCommand.Long,
		RunE:  runCommand.RunE,
		Example: `podman container run imageID ls -alF /etc
	podman container run --network=host imageID dnf -y install java
	podman container run --volume /var/hostdir:/var/ctrdir -i -t fedora /bin/bash`,
	}
)

var (
	runOpts = entities.ContainerRunOptions{
		OutputStream: os.Stdout,
		InputStream:  os.Stdin,
		ErrorStream:  os.Stderr,
	}
	runRmi bool
)

func runFlags(flags *pflag.FlagSet) {
	flags.SetInterspersed(false)
	flags.AddFlagSet(common.GetCreateFlags(&cliVals))
	flags.AddFlagSet(common.GetNetFlags())
	flags.SetNormalizeFunc(utils.AliasFlags)
	flags.BoolVar(&runOpts.SigProxy, "sig-proxy", true, "Proxy received signals to the process")
	flags.BoolVar(&runRmi, "rmi", false, "Remove container image unless used by other containers")
	flags.UintVar(&runOpts.PreserveFDs, "preserve-fds", 0, "Pass a number of additional file descriptors into the container")

	_ = flags.MarkHidden("signature-policy")
	if registry.IsRemote() {
		_ = flags.MarkHidden("http-proxy")
		_ = flags.MarkHidden("preserve-fds")
	}
}
func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: runCommand,
	})
	flags := runCommand.Flags()
	runFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerRunCommand,
		Parent:  containerCmd,
	})

	containerRunFlags := containerRunCommand.Flags()
	runFlags(containerRunFlags)
}

func run(cmd *cobra.Command, args []string) error {
	var err error
	cliVals.Net, err = common.NetFlagsToNetOptions(cmd)
	if err != nil {
		return err
	}

	if rootless.IsRootless() && !registry.IsRemote() {
		userspec := strings.SplitN(cliVals.User, ":", 2)[0]
		if uid, err := strconv.ParseInt(userspec, 10, 32); err == nil {
			if err := util.CheckRootlessUIDRange(int(uid)); err != nil {
				return err
			}
		}
	}

	if af := cliVals.Authfile; len(af) > 0 {
		if _, err := os.Stat(af); err != nil {
			return errors.Wrapf(err, "error checking authfile path %s", af)
		}
	}
	cidFile, err := openCidFile(cliVals.CIDFile)
	if err != nil {
		return err
	}

	if cidFile != nil {
		defer errorhandling.CloseQuiet(cidFile)
		defer errorhandling.SyncQuiet(cidFile)
	}
	runOpts.Rm = cliVals.Rm
	if err := createInit(cmd); err != nil {
		return err
	}
	for fd := 3; fd < int(3+runOpts.PreserveFDs); fd++ {
		if !rootless.IsFdInherited(fd) {
			return errors.Errorf("file descriptor %d is not available - the preserve-fds option requires that file descriptors must be passed", fd)
		}
	}

	imageName := args[0]
	rawImageName := ""
	if !cliVals.RootFS {
		rawImageName = args[0]
		name, err := pullImage(args[0])
		if err != nil {
			return err
		}
		imageName = name
	}

	if cliVals.Replace {
		if err := replaceContainer(cliVals.Name); err != nil {
			return err
		}
	}

	// If -i is not set, clear stdin
	if !cliVals.Interactive {
		runOpts.InputStream = nil
	}

	// If attach is set, clear stdin/stdout/stderr and only attach requested
	if cmd.Flag("attach").Changed {
		runOpts.OutputStream = nil
		runOpts.ErrorStream = nil
		if !cliVals.Interactive {
			runOpts.InputStream = nil
		}

		for _, stream := range cliVals.Attach {
			switch strings.ToLower(stream) {
			case "stdout":
				runOpts.OutputStream = os.Stdout
			case "stderr":
				runOpts.ErrorStream = os.Stderr
			case "stdin":
				runOpts.InputStream = os.Stdin
			default:
				return errors.Wrapf(define.ErrInvalidArg, "invalid stream %q for --attach - must be one of stdin, stdout, or stderr", stream)
			}
		}
	}
	runOpts.Detach = cliVals.Detach
	runOpts.DetachKeys = cliVals.DetachKeys
	cliVals.PreserveFDs = runOpts.PreserveFDs
	s := specgen.NewSpecGenerator(imageName, cliVals.RootFS)
	if err := common.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}
	s.RawImageName = rawImageName
	runOpts.Spec = s

	if _, err := createPodIfNecessary(s, cliVals.Net); err != nil {
		return err
	}

	report, err := registry.ContainerEngine().ContainerRun(registry.GetContext(), runOpts)
	// report.ExitCode is set by ContainerRun even it it returns an error
	if report != nil {
		registry.SetExitCode(report.ExitCode)
	}
	if err != nil {
		return err
	}
	if cidFile != nil {
		_, err = cidFile.WriteString(report.Id)
		if err != nil {
			logrus.Error(err)
		}
	}

	if cliVals.Detach {
		fmt.Println(report.Id)
		return nil
	}
	if runRmi {
		_, rmErrors := registry.ImageEngine().Remove(registry.GetContext(), []string{imageName}, entities.ImageRemoveOptions{})
		if len(rmErrors) > 0 {
			logrus.Errorf("%s", errors.Wrapf(errorhandling.JoinErrors(rmErrors), "failed removing image"))
		}
	}
	return nil
}
