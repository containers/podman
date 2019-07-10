package logs

import (
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
)

const (
	// LogTimeFormat is the time format used in the log.
	// It is a modified version of RFC3339Nano that guarantees trailing
	// zeroes are not trimmed, taken from
	// https://github.com/golang/go/issues/19635
	LogTimeFormat = "2006-01-02T15:04:05.000000000Z07:00"

	// PartialLogType signifies a log line that exceeded the buffer
	// length and needed to spill into a new line
	PartialLogType = "P"

	// FullLogType signifies a log line is full
	FullLogType = "F"
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

// GetLogFile returns an hp tail for a container given options
func GetLogFile(path string, options *LogOptions) (*tail.Tail, []*LogLine, error) {
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
		nll, err := NewLogLine(splitContent[i])
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
		out = out + fmt.Sprintf("%s ", l.Time.Format(LogTimeFormat))
	}
	return out + l.Msg
}

// Since returns a bool as to whether a log line occurred after a given time
func (l *LogLine) Since(since time.Time) bool {
	return l.Time.After(since)
}

// NewLogLine creates a logLine struct from a container log string
func NewLogLine(line string) (*LogLine, error) {
	splitLine := strings.Split(line, " ")
	if len(splitLine) < 4 {
		return nil, errors.Errorf("'%s' is not a valid container log line", line)
	}
	logTime, err := time.Parse(LogTimeFormat, splitLine[0])
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
	return l.ParseLogType == PartialLogType
}
