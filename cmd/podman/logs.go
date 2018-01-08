package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

type logOptions struct {
	details   bool
	follow    bool
	sinceTime time.Time
	tail      uint64
}

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
		LatestFlag,
	}
	logsDescription = "The podman logs command batch-retrieves whatever logs are present for a container at the time of execution.  This does not guarantee execution" +
		"order when combined with podman run (i.e. your run may not have generated any logs at the time you execute podman logs"
	logsCommand = cli.Command{
		Name:        "logs",
		Usage:       "Fetch the logs of a container",
		Description: logsDescription,
		Flags:       logsFlags,
		Action:      logsCmd,
		ArgsUsage:   "CONTAINER",
	}
)

func logsCmd(c *cli.Context) error {
	var ctr *libpod.Container
	var err error
	if err := validateFlags(c, logsFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
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
		since, err := time.Parse("2006-01-02T15:04:05.999999999-07:00", c.String("since"))
		if err != nil {
			return errors.Wrapf(err, "could not parse time: %q", c.String("since"))
		}
		sinceTime = since
	}

	opts := logOptions{
		details:   c.Bool("details"),
		follow:    c.Bool("follow"),
		sinceTime: sinceTime,
		tail:      c.Uint64("tail"),
	}

	if c.Bool("latest") {
		ctr, err = runtime.GetLatestContainer()
	} else {
		ctr, err = runtime.LookupContainer(args[0])
	}
	if err != nil {
		return err
	}

	logs := make(chan string)
	go func() {
		err = getLogs(ctr, logs, opts)
	}()
	printLogs(logs)
	return err
}

// getLogs returns the logs of a container from the log file
// log file is created when the container is started/ran
func getLogs(container *libpod.Container, logChan chan string, opts logOptions) error {
	defer close(logChan)

	seekInfo := &tail.SeekInfo{Offset: 0, Whence: 0}
	if opts.tail > 0 {
		// seek to correct position in log files
		seekInfo.Offset = int64(opts.tail)
		seekInfo.Whence = 2
	}

	t, err := tail.TailFile(container.LogPath(), tail.Config{Follow: opts.follow, ReOpen: false, Location: seekInfo})
	for line := range t.Lines {
		if since, err := logSinceTime(opts.sinceTime, line.Text); err != nil || !since {
			continue
		}
		logMessage := line.Text[secondSpaceIndex(line.Text):]
		logChan <- logMessage
	}
	return err
}

func printLogs(logs chan string) {
	for line := range logs {
		fmt.Println(line)
	}
}

// returns true if the time stamps of the logs are equal to or after the
// timestamp comparing to
func logSinceTime(sinceTime time.Time, logStr string) (bool, error) {
	timestamp := strings.Split(logStr, " ")[0]
	logTime, err := time.Parse("2006-01-02T15:04:05.999999999-07:00", timestamp)
	if err != nil {
		return false, err
	}
	return logTime.After(sinceTime) || logTime.Equal(sinceTime), nil
}

// secondSpaceIndex returns the index of the second space in a string
// In a line of the logs, the first two tokens are a timestamp and stdout/stderr,
// followed by the message itself.  This allows us to get the index of the message
// and avoid sending the other information back to the caller of GetLogs()
func secondSpaceIndex(line string) int {
	index := strings.Index(line, " ")
	if index == -1 {
		return 0
	}
	index = strings.Index(line[index:], " ")
	if index == -1 {
		return 0
	}
	return index
}
