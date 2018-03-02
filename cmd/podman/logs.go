package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"bufio"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type logOptions struct {
	details        bool
	follow         bool
	sinceTime      time.Time
	tail           uint64
	showTimestamps bool
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
		cli.BoolFlag{
			Name:  "timestamps, t",
			Usage: "Output the timestamps in the log",
		},
		LatestFlag,
	}
	logsDescription = "The podman logs command batch-retrieves whatever logs are present for a container at the time of execution.  This does not guarantee execution" +
		"order when combined with podman run (i.e. your run may not have generated any logs at the time you execute podman logs"
	logsCommand = cli.Command{
		Name:           "logs",
		Usage:          "Fetch the logs of a container",
		Description:    logsDescription,
		Flags:          logsFlags,
		Action:         logsCmd,
		ArgsUsage:      "CONTAINER",
		SkipArgReorder: true,
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
		since, err := parseInputTime(c.String("since"))
		if err != nil {
			return errors.Wrapf(err, "could not parse time: %q", c.String("since"))
		}
		sinceTime = since
	}

	opts := logOptions{
		details:        c.Bool("details"),
		follow:         c.Bool("follow"),
		sinceTime:      sinceTime,
		tail:           c.Uint64("tail"),
		showTimestamps: c.Bool("timestamps"),
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
			logrus.Debugf("Container has not been created, no logs exist yet")
			return nil
		}
	}

	file, err := os.Open(logPath)
	if err != nil {
		return errors.Wrapf(err, "unable to read container log file")
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	if opts.follow {
		followLog(reader, opts, ctr)
	} else {
		dumpLog(reader, opts)
	}
	return err
}

func followLog(reader *bufio.Reader, opts logOptions, ctr *libpod.Container) error {
	var cacheOutput []string
	firstPass := false
	if opts.tail > 0 {
		firstPass = true
	}
	// We need to read the entire file in here until we reach EOF
	// and then dump it out in the case that the user also wants
	// tail output
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF && opts.follow {
			if firstPass {
				firstPass = false
				cacheLen := int64(len(cacheOutput))
				start := int64(0)
				if cacheLen > int64(opts.tail) {
					start = cacheLen - int64(opts.tail)
				}
				for i := start; i < cacheLen; i++ {
					printLine(cacheOutput[i], opts)
				}
				continue
			}
			time.Sleep(1 * time.Second)
			// Check if container is still running or paused
			state, err := ctr.State()
			if err != nil {
				return err
			}
			if state != libpod.ContainerStateRunning && state != libpod.ContainerStatePaused {
				break
			}
			continue
		}
		// exits
		if err != nil {
			break
		}
		if firstPass {
			cacheOutput = append(cacheOutput, line)
			continue
		}
		printLine(line, opts)
	}
	return nil
}

func dumpLog(reader *bufio.Reader, opts logOptions) error {
	output := readLog(reader, opts)
	for _, line := range output {
		printLine(line, opts)
	}

	return nil
}

func readLog(reader *bufio.Reader, opts logOptions) []string {
	var output []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		output = append(output, line)
	}
	start := 0
	if opts.tail > 0 {
		if len(output) > int(opts.tail) {
			start = len(output) - int(opts.tail)
		}
	}
	return output[start:]
}

func printLine(line string, opts logOptions) {
	start := 3
	fields := strings.Fields(line)
	if opts.showTimestamps || !isStringTimestamp(fields[0]) {
		start = 0
	}
	if opts.sinceTime.IsZero() || logSinceTime(opts.sinceTime, fields[0]) {
		output := strings.Join(fields[start:], " ")
		fmt.Printf("%s\n", output)
	}
}

func isStringTimestamp(t string) bool {
	_, err := time.Parse("2006-01-02T15:04:05.999999999-07:00", t)
	if err != nil {
		return false
	}
	return true
}

// returns true if the time stamps of the logs are equal to or after the
// timestamp comparing to
func logSinceTime(sinceTime time.Time, logStr string) bool {
	timestamp := strings.Split(logStr, " ")[0]
	logTime, err := time.Parse("2006-01-02T15:04:05.999999999-07:00", timestamp)
	if err != nil {
		return false
	}
	return logTime.After(sinceTime) || logTime.Equal(sinceTime)
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
