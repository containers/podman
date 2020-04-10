package containers

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/cmd/podmanV2/common"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	runDescription = "Runs a command in a new container from the given image"
	runCommand     = &cobra.Command{
		Use:     "run [flags] IMAGE [COMMAND [ARG...]]",
		Short:   "Run a command in a new container",
		Long:    runDescription,
		PreRunE: preRunE,
		RunE:    run,
		Example: `podman run imageID ls -alF /etc
  podman run --network=host imageID dnf -y install java
  podman run --volume /var/hostdir:/var/ctrdir -i -t fedora /bin/bash`,
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

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: runCommand,
	})
	flags := runCommand.Flags()
	flags.SetInterspersed(false)
	flags.AddFlagSet(common.GetCreateFlags(&cliVals))
	flags.AddFlagSet(common.GetNetFlags())
	flags.SetNormalizeFunc(common.AliasFlags)
	flags.BoolVar(&runOpts.SigProxy, "sig-proxy", true, "Proxy received signals to the process")
	flags.BoolVar(&runRmi, "rmi", false, "Remove container image unless used by other containers")
	if registry.IsRemote() {
		_ = flags.MarkHidden("authfile")
	}
}

func run(cmd *cobra.Command, args []string) error {
	var (
		err error
	)
	cliVals.Net, err = common.NetFlagsToNetOptions(cmd)
	if err != nil {
		return err
	}
	if af := cliVals.Authfile; len(af) > 0 {
		if _, err := os.Stat(af); err != nil {
			return errors.Wrapf(err, "error checking authfile path %s", af)
		}
	}
	runOpts.Rm = cliVals.Rm
	if err := createInit(cmd); err != nil {
		return err
	}

	ie, err := registry.NewImageEngine(cmd, args)
	if err != nil {
		return err
	}
	br, err := ie.Exists(registry.GetContext(), args[0])
	if err != nil {
		return err
	}
	pullPolicy, err := config.ValidatePullPolicy(cliVals.Pull)
	if err != nil {
		return err
	}
	if !br.Value || pullPolicy == config.PullImageAlways {
		if pullPolicy == config.PullImageNever {
			return errors.New("unable to find a name and tag match for busybox in repotags: no such image")
		}
		_, pullErr := ie.Pull(registry.GetContext(), args[0], entities.ImagePullOptions{
			Authfile: cliVals.Authfile,
			Quiet:    cliVals.Quiet,
		})
		if pullErr != nil {
			return pullErr
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
	s := specgen.NewSpecGenerator(args[0])
	if err := common.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}
	runOpts.Spec = s
	report, err := registry.ContainerEngine().ContainerRun(registry.GetContext(), runOpts)
	// report.ExitCode is set by ContainerRun even it it returns an error
	if report != nil {
		registry.SetExitCode(report.ExitCode)
	}
	if err != nil {
		return err
	}
	if cliVals.Detach {
		fmt.Println(report.Id)
	}
	if runRmi {
		_, err := registry.ImageEngine().Delete(registry.GetContext(), []string{args[0]}, entities.ImageDeleteOptions{})
		if err != nil {
			logrus.Errorf("%s", errors.Wrapf(err, "failed removing image"))
		}
	}
	return nil
}
