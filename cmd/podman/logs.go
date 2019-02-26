package main

import (
	"os"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/logs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	logsCommand     cliconfig.LogsValues
	logsDescription = "The podman logs command batch-retrieves whatever logs are present for a container at the time of execution.  This does not guarantee execution" +
		"order when combined with podman run (i.e. your run may not have generated any logs at the time you execute podman logs"
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
		since, err := parseInputTime(c.Since)
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

// parseInputTime takes the users input and to determine if it is valid and
// returns a time format and error.  The input is compared to known time formats
// or a duration which implies no-duration
func parseInputTime(inputTime string) (time.Time, error) {
	timeFormats := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04:05.999999999",
		"2006-01-02Z07:00", "2006-01-02"}
	// iterate the supported time formats
	for _, tf := range timeFormats {
		t, err := time.Parse(tf, inputTime)
		if err == nil {
			return t, nil
		}
	}

	// input might be a duration
	duration, err := time.ParseDuration(inputTime)
	if err != nil {
		return time.Time{}, errors.Errorf("unable to interpret time value")
	}
	return time.Now().Add(-duration), nil
}
