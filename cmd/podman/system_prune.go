package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	pruneSystemCommand     cliconfig.SystemPruneValues
	pruneSystemDescription = `
	podman system prune

        Remove unused data
`
	_pruneSystemCommand = &cobra.Command{
		Use:   "prune",
		Args:  noSubArgs,
		Short: "Remove unused data",
		Long:  pruneSystemDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			pruneSystemCommand.InputArgs = args
			pruneSystemCommand.GlobalFlags = MainGlobalOpts
			pruneSystemCommand.Remote = remoteclient
			return pruneSystemCmd(&pruneSystemCommand)
		},
	}
)

func init() {
	pruneSystemCommand.Command = _pruneSystemCommand
	pruneSystemCommand.SetHelpTemplate(HelpTemplate())
	pruneSystemCommand.SetUsageTemplate(UsageTemplate())
	flags := pruneSystemCommand.Flags()
	flags.BoolVarP(&pruneSystemCommand.All, "all", "a", false, "Remove all unused data")
	flags.BoolVarP(&pruneSystemCommand.Force, "force", "f", false, "Do not prompt for confirmation")
	flags.BoolVar(&pruneSystemCommand.Volume, "volumes", false, "Prune volumes")

}

func pruneSystemCmd(c *cliconfig.SystemPruneValues) error {

	// Prompt for confirmation if --force is not set
	if !c.Force {
		reader := bufio.NewReader(os.Stdin)
		volumeString := ""
		if c.Volume {
			volumeString = `
        - all volumes not used by at least one container`
		}
		fmt.Printf(`
WARNING! This will remove:
        - all stopped containers%s
        - all stopped pods
        - all dangling images
        - all build cache
Are you sure you want to continue? [y/N] `, volumeString)
		ans, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "error reading input")
		}
		if strings.ToLower(ans)[0] != 'y' {
			return nil
		}
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	// We must clean out pods first because if they may have infra containers
	fmt.Println("Deleted Pods")
	pruneValues := cliconfig.PodPruneValues{
		PodmanCommand: c.PodmanCommand,
	}
	ctx := getContext()
	ok, failures, lasterr := runtime.PrunePods(ctx, &pruneValues)

	if err := printCmdResults(ok, failures); err != nil {
		return err
	}

	rmWorkers := shared.Parallelize("rm")
	fmt.Println("Deleted Containers")
	ok, failures, err = runtime.Prune(ctx, rmWorkers, []string{})
	if err != nil {
		if lasterr != nil {
			logrus.Errorf("%q", err)
		}
		lasterr = err
	}
	if err := printCmdResults(ok, failures); err != nil {
		return err
	}

	if c.Bool("volumes") {
		fmt.Println("Deleted Volumes")
		err := volumePrune(runtime, getContext())
		if err != nil {
			if lasterr != nil {
				logrus.Errorf("%q", lasterr)
			}
			lasterr = err
		}
	}

	// Call prune; if any cids are returned, print them and then
	// return err in case an error also came up
	// TODO: support for filters in system prune
	pruneCids, err := runtime.PruneImages(ctx, c.All, []string{})
	if len(pruneCids) > 0 {
		fmt.Println("Deleted Images")
		for _, cid := range pruneCids {
			fmt.Println(cid)
		}
	}
	if err != nil {
		if lasterr != nil {
			logrus.Errorf("%q", lasterr)
		}
		lasterr = err
	}
	return lasterr
}
