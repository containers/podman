package libkpod

import (
	"path"
	"strings"
	"time"

	"github.com/hpcloud/tail"
)

// LogOptions contains all of the options for displaying logs in kpod
type LogOptions struct {
	Details   bool
	Follow    bool
	SinceTime time.Time
	Tail      uint64
}

// GetLogs gets each line of a log file and, if it matches the criteria in logOptions, sends it down logChan
func (c *ContainerServer) GetLogs(container string, logChan chan string, opts LogOptions) error {
	defer close(logChan)
	// Get the full ID of the container
	ctr, err := c.LookupContainer(container)
	if err != nil {
		return err
	}

	containerID := ctr.ID()
	sandbox := ctr.Sandbox()
	if sandbox == "" {
		sandbox = containerID
	}
	// Read the log line by line and pass it into the pipe
	logsFile := path.Join(c.config.LogDir, sandbox, containerID+".log")

	seekInfo := &tail.SeekInfo{Offset: 0, Whence: 0}
	if opts.Tail > 0 {
		// seek to correct position in logs files
		seekInfo.Offset = int64(opts.Tail)
		seekInfo.Whence = 2
	}

	t, err := tail.TailFile(logsFile, tail.Config{Follow: false, ReOpen: false, Location: seekInfo})
	for line := range t.Lines {
		if since, err := logSinceTime(opts.SinceTime, line.Text); err != nil || !since {
			continue
		}
		logMessage := line.Text[secondSpaceIndex(line.Text):]
		if opts.Details {
			// add additional information to line
		}
		logChan <- logMessage
	}
	return err
}

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
