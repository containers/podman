package containers

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/common"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	runDescription = "Runs a command in a new container from the given image"
	runCommand     = &cobra.Command{
		Args:  cobra.MinimumNArgs(1),
		Use:   "run [flags] IMAGE [COMMAND [ARG...]]",
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
	flags.SetNormalizeFunc(common.AliasFlags)
	flags.BoolVar(&runOpts.SigProxy, "sig-proxy", true, "Proxy received signals to the process")
	flags.BoolVar(&runRmi, "rmi", false, "Remove container image unless used by other containers")
	if registry.IsRemote() {
		_ = flags.MarkHidden("authfile")
		_ = flags.MarkHidden("env-host")
		_ = flags.MarkHidden("http-proxy")
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

	if !cliVals.RootFS {
		if err := pullImage(args[0]); err != nil {
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
	s := specgen.NewSpecGenerator(args[0], cliVals.RootFS)
	if err := common.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}
	runOpts.Spec = s

	if _, err := createPodIfNecessary(s); err != nil {
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
		_, rmErrors := registry.ImageEngine().Remove(registry.GetContext(), []string{args[0]}, entities.ImageRemoveOptions{})
		if len(rmErrors) > 0 {
			logrus.Errorf("%s", errors.Wrapf(errorhandling.JoinErrors(rmErrors), "failed removing image"))
		}
	}
	return nil
}
