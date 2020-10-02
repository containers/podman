//+build linux
//+build systemd

package libpod

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/containers/podman/v2/libpod/logs"
	journal "github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// journaldLogOut is the journald priority signifying stdout
	journaldLogOut = "6"

	// journaldLogErr is the journald priority signifying stderr
	journaldLogErr = "3"

	// bufLen is the length of the buffer to read from a k8s-file
	// formatted log line
	// let's set it as 2k just to be safe if k8s-file format ever changes
	bufLen = 16384
)

func (c *Container) readFromJournal(ctx context.Context, options *logs.LogOptions, logChannel chan *logs.LogLine) error {
	var config journal.JournalReaderConfig
	if options.Tail < 0 {
		config.NumFromTail = 0
	} else {
		config.NumFromTail = uint64(options.Tail)
	}
	config.Formatter = journalFormatter
	defaultTime := time.Time{}
	if options.Since != defaultTime {
		// coreos/go-systemd/sdjournal doesn't correctly handle requests for data in the future
		// return nothing instead of falsely printing
		if time.Now().Before(options.Since) {
			return nil
		}
		config.Since = time.Since(options.Since)
	}
	config.Matches = append(config.Matches, journal.Match{
		Field: "CONTAINER_ID_FULL",
		Value: c.ID(),
	})
	options.WaitGroup.Add(1)

	r, err := journal.NewJournalReader(config)
	if err != nil {
		return err
	}
	if r == nil {
		return errors.Errorf("journal reader creation failed")
	}
	if options.Tail == math.MaxInt64 {
		r.Rewind()
	}

	if options.Follow {
		go func() {
			done := make(chan bool)
			until := make(chan time.Time)
			go func() {
				select {
				case <-ctx.Done():
					until <- time.Time{}
				case <-done:
					// nothing to do anymore
				}
			}()
			follower := FollowBuffer{logChannel}
			err := r.Follow(until, follower)
			if err != nil {
				logrus.Debugf(err.Error())
			}
			r.Close()
			options.WaitGroup.Done()
			done <- true
			return
		}()
		return nil
	}

	go func() {
		bytes := make([]byte, bufLen)
		// /me complains about no do-while in go
		ec, err := r.Read(bytes)
		for ec != 0 && err == nil {
			// because we are reusing bytes, we need to make
			// sure the old data doesn't get into the new line
			bytestr := string(bytes[:ec])
			logLine, err2 := logs.NewLogLine(bytestr)
			if err2 != nil {
				logrus.Error(err2)
				continue
			}
			logChannel <- logLine
			ec, err = r.Read(bytes)
		}
		if err != nil && err != io.EOF {
			logrus.Error(err)
		}
		r.Close()
		options.WaitGroup.Done()
	}()
	return nil
}

func journalFormatter(entry *journal.JournalEntry) (string, error) {
	usec := entry.RealtimeTimestamp
	tsString := time.Unix(0, int64(usec)*int64(time.Microsecond)).Format(logs.LogTimeFormat)
	output := fmt.Sprintf("%s ", tsString)
	priority, ok := entry.Fields["PRIORITY"]
	if !ok {
		return "", errors.Errorf("no PRIORITY field present in journal entry")
	}
	if priority == journaldLogOut {
		output += "stdout "
	} else if priority == journaldLogErr {
		output += "stderr "
	} else {
		return "", errors.Errorf("unexpected PRIORITY field in journal entry")
	}

	// if CONTAINER_PARTIAL_MESSAGE is defined, the log type is "P"
	if _, ok := entry.Fields["CONTAINER_PARTIAL_MESSAGE"]; ok {
		output += fmt.Sprintf("%s ", logs.PartialLogType)
	} else {
		output += fmt.Sprintf("%s ", logs.FullLogType)
	}

	// Finally, append the message
	msg, ok := entry.Fields["MESSAGE"]
	if !ok {
		return "", fmt.Errorf("no MESSAGE field present in journal entry")
	}
	output += strings.TrimSpace(msg)
	return output, nil
}

type FollowBuffer struct {
	logChannel chan *logs.LogLine
}

func (f FollowBuffer) Write(p []byte) (int, error) {
	bytestr := string(p)
	logLine, err := logs.NewLogLine(bytestr)
	if err != nil {
		return -1, err
	}
	f.logChannel <- logLine
	return len(p), nil
}
