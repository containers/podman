package libpod

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// logTimeFormat is the time format used in the log.
	// It is a modified version of RFC3339Nano that guarantees trailing
	// zeroes are not trimmed, taken from
	// https://github.com/golang/go/issues/19635
	logTimeFormat = "2006-01-02T15:04:05.000000000Z07:00"
)

// LogOptions is the options you can use for logs
type LogOptions struct {
	Details    bool
	Follow     bool
	Since      time.Time
	Tail       uint64
	Timestamps bool
	Multi      bool
	WaitGroup  *sync.WaitGroup
}

// LogLine describes the information for each line of a log
type LogLine struct {
	Device       string
	ParseLogType string
	Time         time.Time
	Msg          string
	CID          string
}

// Log is a runtime function that can read one or more container logs.
func (r *Runtime) Log(containers []*Container, options *LogOptions, logChannel chan *LogLine) error {
	for _, ctr := range containers {
		if err := ctr.ReadLog(options, logChannel); err != nil {
			return err
		}
	}
	return nil
}

// ReadLog reads a containers log based on the input options and returns loglines over a channel
func (c *Container) ReadLog(options *LogOptions, logChannel chan *LogLine) error {
	t, tailLog, err := getLogFile(c.LogPath(), options)
	if err != nil {
		// If the log file does not exist, this is not fatal.
		if os.IsNotExist(errors.Cause(err)) {
			return nil
		}
		return errors.Wrapf(err, "unable to read log file %s for %s ", c.ID(), c.LogPath())
	}
	options.WaitGroup.Add(1)
	if len(tailLog) > 0 {
		for _, nll := range tailLog {
			nll.CID = c.ID()
			if nll.Since(options.Since) {
				logChannel <- nll
			}
		}
	}

	go func() {
		var partial string
		for line := range t.Lines {
			nll, err := newLogLine(line.Text)
			if err != nil {
				logrus.Error(err)
				continue
			}
			if nll.Partial() {
				partial = partial + nll.Msg
				continue
			} else if !nll.Partial() && len(partial) > 1 {
				nll.Msg = partial
				partial = ""
			}
			nll.CID = c.ID()
			if nll.Since(options.Since) {
				logChannel <- nll
			}
		}
		options.WaitGroup.Done()
	}()
	return nil
}

// getLogFile returns an hp tail for a container given options
func getLogFile(path string, options *LogOptions) (*tail.Tail, []*LogLine, error) {
	var (
		whence  int
		err     error
		logTail []*LogLine
	)
	// whence 0=origin, 2=end
	if options.Tail > 0 {
		whence = 2
		logTail, err = getTailLog(path, int(options.Tail))
		if err != nil {
			return nil, nil, err
		}
	}
	seek := tail.SeekInfo{
		Offset: 0,
		Whence: whence,
	}

	t, err := tail.TailFile(path, tail.Config{MustExist: true, Poll: true, Follow: options.Follow, Location: &seek, Logger: tail.DiscardingLogger})
	return t, logTail, err
}

func getTailLog(path string, tail int) ([]*LogLine, error) {
	var (
		tailLog     []*LogLine
		nlls        []*LogLine
		tailCounter int
		partial     string
	)
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	splitContent := strings.Split(string(content), "\n")
	// We read the content in reverse and add each nll until we have the same
	// number of F type messages as the desired tail
	for i := len(splitContent) - 1; i >= 0; i-- {
		if len(splitContent[i]) == 0 {
			continue
		}
		nll, err := newLogLine(splitContent[i])
		if err != nil {
			return nil, err
		}
		nlls = append(nlls, nll)
		if !nll.Partial() {
			tailCounter = tailCounter + 1
		}
		if tailCounter == tail {
			break
		}
	}
	// Now we iterate the results and assemble partial messages to become full messages
	for _, nll := range nlls {
		if nll.Partial() {
			partial = partial + nll.Msg
		} else {
			nll.Msg = nll.Msg + partial
			tailLog = append(tailLog, nll)
			partial = ""
		}
	}
	return tailLog, nil
}

// String converts a logline to a string for output given whether a detail
// bool is specified.
func (l *LogLine) String(options *LogOptions) string {
	var out string
	if options.Multi {
		cid := l.CID
		if len(cid) > 12 {
			cid = cid[:12]
		}
		out = fmt.Sprintf("%s ", cid)
	}
	if options.Timestamps {
		out = out + fmt.Sprintf("%s ", l.Time.Format(logTimeFormat))
	}
	return out + l.Msg
}

// Since returns a bool as to whether a log line occurred after a given time
func (l *LogLine) Since(since time.Time) bool {
	return l.Time.After(since)
}

// newLogLine creates a logLine struct from a container log string
func newLogLine(line string) (*LogLine, error) {
	splitLine := strings.Split(line, " ")
	if len(splitLine) < 4 {
		return nil, errors.Errorf("'%s' is not a valid container log line", line)
	}
	logTime, err := time.Parse(time.RFC3339Nano, splitLine[0])
	if err != nil {
		return nil, errors.Wrapf(err, "unable to convert time %s from container log", splitLine[0])
	}
	l := LogLine{
		Time:         logTime,
		Device:       splitLine[1],
		ParseLogType: splitLine[2],
		Msg:          strings.Join(splitLine[3:], " "),
	}
	return &l, nil
}

// Partial returns a bool if the log line is a partial log type
func (l *LogLine) Partial() bool {
	if l.ParseLogType == "P" {
		return true
	}
	return false
}
