package containers

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	runDescription = "Runs a command in a new container from the given image"
	runCommand     = &cobra.Command{
		Args:              cobra.MinimumNArgs(1),
		Use:               "run [options] IMAGE [COMMAND [ARG...]]",
		Short:             "Run a command in a new container",
		Long:              runDescription,
		RunE:              run,
		ValidArgsFunction: common.AutocompleteCreateRun,
		Example: `podman run imageID ls -alF /etc
  podman run --network=host imageID dnf -y install java
  podman run --volume /var/hostdir:/var/ctrdir -i -t fedora /bin/bash`,
	}

	containerRunCommand = &cobra.Command{
		Args:              cobra.MinimumNArgs(1),
		Use:               runCommand.Use,
		Short:             runCommand.Short,
		Long:              runCommand.Long,
		RunE:              runCommand.RunE,
		ValidArgsFunction: runCommand.ValidArgsFunction,
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

func runFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.SetInterspersed(false)
	common.DefineCreateDefaults(&cliVals)
	common.DefineCreateFlags(cmd, &cliVals, false, false)
	common.DefineNetFlags(cmd)

	flags.SetNormalizeFunc(utils.AliasFlags)
	flags.BoolVar(&runOpts.SigProxy, "sig-proxy", true, "Proxy received signals to the process")
	flags.BoolVar(&runRmi, "rmi", false, "Remove container image unless used by other containers")

	preserveFdsFlagName := "preserve-fds"
	flags.UintVar(&runOpts.PreserveFDs, "preserve-fds", 0, "Pass a number of additional file descriptors into the container")
	_ = cmd.RegisterFlagCompletionFunc(preserveFdsFlagName, completion.AutocompleteNone)

	flags.BoolVarP(&runOpts.Detach, "detach", "d", false, "Run container in background and print container ID")

	detachKeysFlagName := "detach-keys"
	flags.StringVar(&runOpts.DetachKeys, detachKeysFlagName, containerConfig.DetachKeys(), "Override the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-cf`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	_ = cmd.RegisterFlagCompletionFunc(detachKeysFlagName, common.AutocompleteDetachKeys)

	gpuFlagName := "gpus"
	flags.String(gpuFlagName, "", "This is a Docker specific option and is a NOOP")
	_ = cmd.RegisterFlagCompletionFunc(gpuFlagName, completion.AutocompleteNone)
	_ = flags.MarkHidden("gpus")

	passwdFlagName := "passwd"
	flags.BoolVar(&runOpts.Passwd, passwdFlagName, true, "add entries to /etc/passwd and /etc/group")

	if registry.IsRemote() {
		_ = flags.MarkHidden("preserve-fds")
		_ = flags.MarkHidden("conmon-pidfile")
		_ = flags.MarkHidden("pidfile")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: runCommand,
	})

	runFlags(runCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerRunCommand,
		Parent:  containerCmd,
	})

	runFlags(containerRunCommand)
}

func run(cmd *cobra.Command, args []string) error {
	if err := commonFlags(cmd); err != nil {
		return err
	}

	// TODO: Breaking change should be made fatal in next major Release
	if cliVals.TTY && cliVals.Interactive && !term.IsTerminal(int(os.Stdin.Fd())) {
		logrus.Warnf("The input device is not a TTY. The --tty and --interactive flags might not work properly")
	}

	if af := cliVals.Authfile; len(af) > 0 {
		if _, err := os.Stat(af); err != nil {
			return err
		}
	}

	runOpts.CIDFile = cliVals.CIDFile
	runOpts.Rm = cliVals.Rm
	cliVals, err := CreateInit(cmd, cliVals, false)
	if err != nil {
		return err
	}

	for fd := 3; fd < int(3+runOpts.PreserveFDs); fd++ {
		if !rootless.IsFdInherited(fd) {
			return fmt.Errorf("file descriptor %d is not available - the preserve-fds option requires that file descriptors must be passed", fd)
		}
	}

	imageName := args[0]
	rawImageName := ""
	if !cliVals.RootFS {
		rawImageName = args[0]
		name, err := PullImage(args[0], &cliVals)
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

	passthrough := cliVals.LogDriver == define.PassthroughLogging

	// If attach is set, clear stdin/stdout/stderr and only attach requested
	if cmd.Flag("attach").Changed {
		if passthrough {
			return fmt.Errorf("cannot specify --attach with --log-driver=passthrough: %w", define.ErrInvalidArg)
		}
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
				return fmt.Errorf("invalid stream %q for --attach - must be one of stdin, stdout, or stderr: %w", stream, define.ErrInvalidArg)
			}
		}
	}

	cliVals.PreserveFDs = runOpts.PreserveFDs
	s := specgen.NewSpecGenerator(imageName, cliVals.RootFS)
	if err := specgenutil.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}
	s.RawImageName = rawImageName
	s.ImageOS = cliVals.OS
	s.ImageArch = cliVals.Arch
	s.ImageVariant = cliVals.Variant
	s.Passwd = &runOpts.Passwd
	runOpts.Spec = s

	if err := createPodIfNecessary(cmd, s, cliVals.Net); err != nil {
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

	if runOpts.Detach && !passthrough {
		fmt.Println(report.Id)
		return nil
	}
	if runRmi {
		_, rmErrors := registry.ImageEngine().Remove(registry.GetContext(), []string{imageName}, entities.ImageRemoveOptions{})
		if len(rmErrors) > 0 {
			logrus.Errorf("%s", errorhandling.JoinErrors(rmErrors))
		}
	}
	if cmd.Flag("gpus").Changed {
		logrus.Info("--gpus is a Docker specific option and is a NOOP")
	}
	return nil
}
