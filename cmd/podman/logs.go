package main

import (
	"os"
	"time"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/logs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	logsFlags = []cli.Flag{
		cli.BoolFlag{
			Name:   "details",
			Usage:  "Show extra details provided to the logs",
			Hidden: true,
		},
		cli.BoolFlag{
			Name:  "follow, f",
			Usage: "Follow log output.  The default is false",
		},
		cli.StringFlag{
			Name:  "since",
			Usage: "Show logs since TIMESTAMP",
		},
		cli.Uint64Flag{
			Name:  "tail",
			Usage: "Output the specified number of LINES at the end of the logs.  Defaults to 0, which prints all lines",
		},
		cli.BoolFlag{
			Name:  "timestamps, t",
			Usage: "Output the timestamps in the log",
		},
		LatestFlag,
	}
	logsDescription = "The podman logs command batch-retrieves whatever logs are present for a container at the time of execution.  This does not guarantee execution" +
		"order when combined with podman run (i.e. your run may not have generated any logs at the time you execute podman logs"
	logsCommand = cli.Command{
		Name:                   "logs",
		Usage:                  "Fetch the logs of a container",
		Description:            logsDescription,
		Flags:                  sortFlags(logsFlags),
		Action:                 logsCmd,
		ArgsUsage:              "CONTAINER",
		SkipArgReorder:         true,
		OnUsageError:           usageErrorHandler,
		UseShortOptionHandling: true,
	}
)

func logsCmd(c *cli.Context) error {
	var ctr *libpod.Container
	var err error
	if err := validateFlags(c, logsFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) != 1 && !c.Bool("latest") {
		return errors.Errorf("'podman logs' requires exactly one container name/ID")
	}

	sinceTime := time.Time{}
	if c.IsSet("since") {
		// parse time, error out if something is wrong
		since, err := parseInputTime(c.String("since"))
		if err != nil {
			return errors.Wrapf(err, "could not parse time: %q", c.String("since"))
		}
		sinceTime = since
	}

	opts := &logs.LogOptions{
		Details:    c.Bool("details"),
		Follow:     c.Bool("follow"),
		Since:      sinceTime,
		Tail:       c.Uint64("tail"),
		Timestamps: c.Bool("timestamps"),
	}

	if c.Bool("latest") {
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
