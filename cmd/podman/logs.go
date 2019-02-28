package main

import (
	"os"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/logs"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	logsCommand     cliconfig.LogsValues
	logsDescription = `Retrieves logs for a container.

  This does not guarantee execution order when combined with podman run (i.e. your run may not have generated any logs at the time you execute podman logs.
`
	_logsCommand = &cobra.Command{
		Use:   "logs [flags] CONTAINER",
		Short: "Fetch the logs of a container",
		Long:  logsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			logsCommand.InputArgs = args
			logsCommand.GlobalFlags = MainGlobalOpts
			return logsCmd(&logsCommand)
		},
		Example: `podman logs ctrID
  podman logs --tail 2 mywebserver
  podman logs --follow=true --since 10m ctrID`,
	}
)

func init() {
	logsCommand.Command = _logsCommand
	logsCommand.SetHelpTemplate(HelpTemplate())
	logsCommand.SetUsageTemplate(UsageTemplate())
	flags := logsCommand.Flags()
	flags.BoolVar(&logsCommand.Details, "details", false, "Show extra details provided to the logs")
	flags.BoolVarP(&logsCommand.Follow, "follow", "f", false, "Follow log output.  The default is false")
	flags.BoolVarP(&logsCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.StringVar(&logsCommand.Since, "since", "", "Show logs since TIMESTAMP")
	flags.Uint64Var(&logsCommand.Tail, "tail", 0, "Output the specified number of LINES at the end of the logs.  Defaults to 0, which prints all lines")
	flags.BoolVarP(&logsCommand.Timestamps, "timestamps", "t", false, "Output the timestamps in the log")
	flags.MarkHidden("details")

	flags.SetInterspersed(false)

	markFlagHiddenForRemoteClient("latest", flags)
}

func logsCmd(c *cliconfig.LogsValues) error {
	var ctr *libpod.Container
	var err error

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.InputArgs
	if len(args) != 1 && !c.Latest {
		return errors.Errorf("'podman logs' requires exactly one container name/ID")
	}

	sinceTime := time.Time{}
	if c.Flag("since").Changed {
		// parse time, error out if something is wrong
		since, err := util.ParseInputTime(c.Since)
		if err != nil {
			return errors.Wrapf(err, "could not parse time: %q", c.Since)
		}
		sinceTime = since
	}

	opts := &logs.LogOptions{
		Details:    c.Details,
		Follow:     c.Follow,
		Since:      sinceTime,
		Tail:       c.Tail,
		Timestamps: c.Timestamps,
	}

	if c.Latest {
		ctr, err = runtime.GetLatestContainer()
	} else {
		ctr, err = runtime.LookupContainer(args[0])
	}
	if err != nil {
		return err
	}

	logPath := ctr.LogPath()

	state, err := ctr.State()
	if err != nil {
		return err
	}

	// If the log file does not exist yet and the container is in the
	// Configured state, it has never been started before and no logs exist
	// Exit cleanly in this case
	if _, err := os.Stat(logPath); err != nil {
		if state == libpod.ContainerStateConfigured {
			logrus.Debugf("Container has not been started, no logs exist yet")
			return nil
		}
	}
	return logs.ReadLogs(logPath, ctr, opts)
}
