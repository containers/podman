package logs

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/containers/podman/v2/libpod/logs/reversereader"
	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	Tail       int64
	Timestamps bool
	Multi      bool
	WaitGroup  *sync.WaitGroup
	UseName    bool
}

// LogLine describes the information for each line of a log
type LogLine struct {
	Device       string
	ParseLogType string
	Time         time.Time
	Msg          string
	CID          string
	CName        string
}

// GetLogFile returns an hp tail for a container given options
func GetLogFile(path string, options *LogOptions) (*tail.Tail, []*LogLine, error) {
	var (
		whence  int
		err     error
		logTail []*LogLine
	)
	// whence 0=origin, 2=end
	if options.Tail >= 0 {
		whence = 2
	}
	if options.Tail > 0 {
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
		nlls       []*LogLine
		nllCounter int
		leftover   string
		partial    string
		tailLog    []*LogLine
	)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	rr, err := reversereader.NewReverseReader(f)
	if err != nil {
		return nil, err
	}

	inputs := make(chan []string)
	go func() {
		for {
			s, err := rr.Read()
			if err != nil {
				if errors.Cause(err) == io.EOF {
					inputs <- []string{leftover}
				} else {
					logrus.Error(err)
				}
				close(inputs)
				if err := f.Close(); err != nil {
					logrus.Error(err)
				}
				break
			}
			line := strings.Split(s+leftover, "\n")
			if len(line) > 1 {
				inputs <- line[1:]
			}
			leftover = line[0]
		}
	}()

	for i := range inputs {
		// the incoming array is FIFO; we want FIFO so
		// reverse the slice read order
		for j := len(i) - 1; j >= 0; j-- {
			// lines that are "" are junk
			if len(i[j]) < 1 {
				continue
			}
			// read the content in reverse and add each nll until we have the same
			// number of F type messages as the desired tail
			nll, err := NewLogLine(i[j])
			if err != nil {
				return nil, err
			}
			nlls = append(nlls, nll)
			if !nll.Partial() {
				nllCounter++
			}
		}
		// if we have enough loglines, we can hangup
		if nllCounter >= tail {
			break
		}
	}

	// re-assemble the log lines and trim (if needed) to the
	// tail length
	for _, nll := range nlls {
		if nll.Partial() {
			partial += nll.Msg
		} else {
			nll.Msg += partial
			// prepend because we need to reverse the order again to FIFO
			tailLog = append([]*LogLine{nll}, tailLog...)
			partial = ""
		}
		if len(tailLog) == tail {
			break
		}
	}
	return tailLog, nil
}

// String converts a logline to a string for output given whether a detail
// bool is specified.
func (l *LogLine) String(options *LogOptions) string {
	var out string
	if options.Multi {
		if options.UseName {
			out = l.CName + " "
		} else {
			cid := l.CID
			if len(cid) > 12 {
				cid = cid[:12]
			}
			out = fmt.Sprintf("%s ", cid)
		}
	}
	if options.Timestamps {
		out += fmt.Sprintf("%s ", l.Time.Format(LogTimeFormat))
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
