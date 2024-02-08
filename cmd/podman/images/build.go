package images

import (
	"errors"
	"os"
	"os/exec"

	buildahCLI "github.com/containers/buildah/pkg/cli"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _diff_ Object_ID
	buildDescription = "Builds an OCI or Docker image using instructions from one or more Containerfiles and a specified build context directory."
	buildCmd         = &cobra.Command{
		Use:               "build [options] [CONTEXT]",
		Short:             "Build an image using instructions from Containerfiles",
		Long:              buildDescription,
		Args:              cobra.MaximumNArgs(1),
		RunE:              build,
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman build .
  podman build --creds=username:password -t imageName -f Containerfile.simple .
  podman build --layers --force-rm --tag imageName .`,
	}

	imageBuildCmd = &cobra.Command{
		Args:              buildCmd.Args,
		Use:               buildCmd.Use,
		Short:             buildCmd.Short,
		Long:              buildCmd.Long,
		RunE:              buildCmd.RunE,
		ValidArgsFunction: buildCmd.ValidArgsFunction,
		Example: `podman image build .
  podman image build --creds=username:password -t imageName -f Containerfile.simple .
  podman image build --layers --force-rm --tag imageName .`,
	}

	buildxBuildCmd = &cobra.Command{
		Args:              buildCmd.Args,
		Use:               buildCmd.Use,
		Short:             buildCmd.Short,
		Long:              buildCmd.Long,
		RunE:              buildCmd.RunE,
		ValidArgsFunction: buildCmd.ValidArgsFunction,
		Example: `podman buildx build .
  podman buildx build --creds=username:password -t imageName -f Containerfile.simple .
  podman buildx build --layers --force-rm --tag imageName .`,
	}

	buildOpts = common.BuildFlagsWrapper{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildCmd,
	})
	buildFlags(buildCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageBuildCmd,
		Parent:  imageCmd,
	})
	buildFlags(imageBuildCmd)
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildxBuildCmd,
		Parent:  buildxCmd,
	})
	buildFlags(buildxBuildCmd)
}

func buildFlags(cmd *cobra.Command) {
	common.DefineBuildFlags(cmd, &buildOpts, false)
}

// build executes the build command.
func build(cmd *cobra.Command, args []string) error {
	apiBuildOpts, err := common.ParseBuildOpts(cmd, args, &buildOpts)
	if err != nil {
		return err
	}
	// Close the logFile if one was created based on the flag
	if apiBuildOpts.LogFileToClose != nil {
		defer apiBuildOpts.LogFileToClose.Close()
	}
	if apiBuildOpts.TmpDirToClose != "" {
		// We had to download the context directory.
		// Delete it later.
		defer func() {
			if err = os.RemoveAll(apiBuildOpts.TmpDirToClose); err != nil {
				logrus.Errorf("Removing temporary directory %q: %v", apiBuildOpts.ContextDirectory, err)
			}
		}()
	}
	report, err := registry.ImageEngine().Build(registry.GetContext(), apiBuildOpts.ContainerFiles, *apiBuildOpts)

	if err != nil {
		exitCode := buildahCLI.ExecErrorCodeGeneric
		if registry.IsRemote() {
			// errors from server does not contain ExitCode
			// so parse exit code from error message
			remoteExitCode, parseErr := utils.ExitCodeFromBuildError(err.Error())
			if parseErr == nil {
				exitCode = remoteExitCode
			}
		}

		exitError := &exec.ExitError{}
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}

		registry.SetExitCode(exitCode)
		return err
	}

	if cmd.Flag("iidfile").Changed {
		f, err := os.Create(buildOpts.Iidfile)
		if err != nil {
			return err
		}
		if _, err := f.WriteString("sha256:" + report.ID); err != nil {
			return err
		}
	}

	return nil
}
