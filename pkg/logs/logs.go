/*
This package picks up CRI parsing and writer for the logs from the kubernetes
logs package. These two bits have been modified to fit the requirements of libpod.

Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logs

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// timeFormat is the time format used in the log.
	timeFormat = time.RFC3339Nano
)

// LogStreamType is the type of the stream in CRI container log.
type LogStreamType string

const (
	// Stdout is the stream type for stdout.
	Stdout LogStreamType = "stdout"
	// Stderr is the stream type for stderr.
	Stderr LogStreamType = "stderr"
)

// LogTag is the tag of a log line in CRI container log.
// Currently defined log tags:
// * First tag: Partial/Full - P/F.
// The field in the container log format can be extended to include multiple
// tags by using a delimiter, but changes should be rare.
type LogTag string

const (
	// LogTagPartial means the line is part of multiple lines.
	LogTagPartial LogTag = "P"
	// LogTagFull means the line is a single full line or the end of multiple lines.
	LogTagFull LogTag = "F"
	// LogTagDelimiter is the delimiter for different log tags.
	LogTagDelimiter = ":"
)

var (
	// eol is the end-of-line sign in the log.
	eol = []byte{'\n'}
	// delimiter is the delimiter for timestamp and stream type in log line.
	delimiter = []byte{' '}
	// tagDelimiter is the delimiter for log tags.
	tagDelimiter = []byte(LogTagDelimiter)
)

// logMessage is the CRI internal log type.
type logMessage struct {
	timestamp time.Time
	stream    LogStreamType
	log       []byte
}

// LogOptions is the options you can use for logs
type LogOptions struct {
	Details    bool
	Follow     bool
	Since      time.Time
	Tail       uint64
	Timestamps bool
	bytes      int64
}

// reset resets the log to nil.
func (l *logMessage) reset() {
	l.timestamp = time.Time{}
	l.stream = ""
	l.log = nil
}

// parseCRILog parses logs in CRI log format. CRI Log format example:
//   2016-10-06T00:17:09.669794202Z stdout P log content 1
//   2016-10-06T00:17:09.669794203Z stderr F log content 2
func parseCRILog(log []byte, msg *logMessage) error {
	var err error
	// Parse timestamp
	idx := bytes.Index(log, delimiter)
	if idx < 0 {
		return fmt.Errorf("timestamp is not found")
	}
	msg.timestamp, err = time.Parse(timeFormat, string(log[:idx]))
	if err != nil {
		return fmt.Errorf("unexpected timestamp format %q: %v", timeFormat, err)
	}

	// Parse stream type
	log = log[idx+1:]
	idx = bytes.Index(log, delimiter)
	if idx < 0 {
		return fmt.Errorf("stream type is not found")
	}
	msg.stream = LogStreamType(log[:idx])
	if msg.stream != Stdout && msg.stream != Stderr {
		return fmt.Errorf("unexpected stream type %q", msg.stream)
	}

	// Parse log tag
	log = log[idx+1:]
	idx = bytes.Index(log, delimiter)
	if idx < 0 {
		return fmt.Errorf("log tag is not found")
	}
	// Keep this forward compatible.
	tags := bytes.Split(log[:idx], tagDelimiter)
	partial := (LogTag(tags[0]) == LogTagPartial)
	// Trim the tailing new line if this is a partial line.
	if partial && len(log) > 0 && log[len(log)-1] == '\n' {
		log = log[:len(log)-1]
	}

	// Get log content
	msg.log = log[idx+1:]

	return nil
}

// ReadLogs reads in the logs from the logPath
func ReadLogs(logPath string, ctr *libpod.Container, opts *LogOptions) error {
	file, err := os.Open(logPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open log file %q", logPath)
	}
	defer file.Close()

	msg := &logMessage{}
	opts.bytes = -1
	writer := newLogWriter(opts)
	reader := bufio.NewReader(file)

	if opts.Follow {
		followLog(reader, writer, opts, ctr, msg, logPath)
	} else {
		dumpLog(reader, writer, opts, msg, logPath)
	}
	return err
}

func followLog(reader *bufio.Reader, writer *logWriter, opts *LogOptions, ctr *libpod.Container, msg *logMessage, logPath string) error {
	var cacheOutput []string
	firstPass := false
	if opts.Tail > 0 {
		firstPass = true
	}
	// We need to read the entire file in here until we reach EOF
	// and then dump it out in the case that the user also wants
	// tail output
	for {
		line, err := reader.ReadString(eol[0])
		if err == io.EOF && opts.Follow {
			if firstPass {
				firstPass = false
				cacheLen := int64(len(cacheOutput))
				start := int64(0)
				if cacheLen > int64(opts.Tail) {
					start = cacheLen - int64(opts.Tail)
				}
				for i := start; i < cacheLen; i++ {
					msg.reset()
					if err := parseCRILog([]byte(cacheOutput[i]), msg); err != nil {
						return errors.Wrapf(err, "error parsing log line")
					}
					// Write the log line into the stream.
					if err := writer.write(msg); err != nil {
						if err == errMaximumWrite {
							logrus.Infof("Finish parsing log file %q, hit bytes limit %d(bytes)", logPath, opts.bytes)
							return nil
						}
						logrus.Errorf("Failed with err %v when writing log for log file %q: %+v", err, logPath, msg)
						return err
					}
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
		msg.reset()
		if err := parseCRILog([]byte(line), msg); err != nil {
			return errors.Wrapf(err, "error parsing log line")
		}
		// Write the log line into the stream.
		if err := writer.write(msg); err != nil {
			if err == errMaximumWrite {
				logrus.Infof("Finish parsing log file %q, hit bytes limit %d(bytes)", logPath, opts.bytes)
				return nil
			}
			logrus.Errorf("Failed with err %v when writing log for log file %q: %+v", err, logPath, msg)
			return err
		}
	}
	return nil
}

func dumpLog(reader *bufio.Reader, writer *logWriter, opts *LogOptions, msg *logMessage, logPath string) error {
	output := readLog(reader, opts)
	for _, line := range output {
		msg.reset()
		if err := parseCRILog([]byte(line), msg); err != nil {
			return errors.Wrapf(err, "error parsing log line")
		}
		// Write the log line into the stream.
		if err := writer.write(msg); err != nil {
			if err == errMaximumWrite {
				logrus.Infof("Finish parsing log file %q, hit bytes limit %d(bytes)", logPath, opts.bytes)
				return nil
			}
			logrus.Errorf("Failed with err %v when writing log for log file %q: %+v", err, logPath, msg)
			return err
		}
	}

	return nil
}

func readLog(reader *bufio.Reader, opts *LogOptions) []string {
	var output []string
	for {
		line, err := reader.ReadString(eol[0])
		if err != nil {
			break
		}
		output = append(output, line)
	}
	start := 0
	if opts.Tail > 0 {
		if len(output) > int(opts.Tail) {
			start = len(output) - int(opts.Tail)
		}
	}
	return output[start:]
}

// logWriter controls the writing into the stream based on the log options.
type logWriter struct {
	stdout io.Writer
	stderr io.Writer
	opts   *LogOptions
	remain int64
}

// errMaximumWrite is returned when all bytes have been written.
var errMaximumWrite = errors.New("maximum write")

// errShortWrite is returned when the message is not fully written.
var errShortWrite = errors.New("short write")

func newLogWriter(opts *LogOptions) *logWriter {
	w := &logWriter{
		stdout: os.Stdout,
		stderr: os.Stderr,
		opts:   opts,
		remain: math.MaxInt64, // initialize it as infinity
	}
	if opts.bytes >= 0 {
		w.remain = opts.bytes
	}
	return w
}

// writeLogs writes logs into stdout, stderr.
func (w *logWriter) write(msg *logMessage) error {
	if msg.timestamp.Before(w.opts.Since) {
		// Skip the line because it's older than since
		return nil
	}
	line := msg.log
	if w.opts.Timestamps {
		prefix := append([]byte(msg.timestamp.Format(timeFormat)), delimiter[0])
		line = append(prefix, line...)
	}
	// If the line is longer than the remaining bytes, cut it.
	if int64(len(line)) > w.remain {
		line = line[:w.remain]
	}
	// Get the proper stream to write to.
	var stream io.Writer
	switch msg.stream {
	case Stdout:
		stream = w.stdout
	case Stderr:
		stream = w.stderr
	default:
		return fmt.Errorf("unexpected stream type %q", msg.stream)
	}
	n, err := stream.Write(line)
	w.remain -= int64(n)
	if err != nil {
		return err
	}
	// If the line has not been fully written, return errShortWrite
	if n < len(line) {
		return errShortWrite
	}
	// If there are no more bytes left, return errMaximumWrite
	if w.remain <= 0 {
		return errMaximumWrite
	}
	return nil
}
