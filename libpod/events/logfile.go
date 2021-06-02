package events

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containers/podman/v3/pkg/util"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/pkg/errors"
)

// EventLogFile is the structure for event writing to a logfile. It contains the eventer
// options and the event itself.  Methods for reading and writing are also defined from it.
type EventLogFile struct {
	options EventerOptions
}

// Writes to the log file
func (e EventLogFile) Write(ee Event) error {
	// We need to lock events file
	lock, err := lockfile.GetLockfile(e.options.LogFilePath + ".lock")
	if err != nil {
		return err
	}
	lock.Lock()
	defer lock.Unlock()
	f, err := os.OpenFile(e.options.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	defer f.Close()
	eventJSONString, err := ee.ToJSONString()
	if err != nil {
		return err
	}
	if _, err := f.WriteString(fmt.Sprintf("%s\n", eventJSONString)); err != nil {
		return err
	}
	return nil
}

// Reads from the log file
func (e EventLogFile) Read(ctx context.Context, options ReadOptions) error {
	defer close(options.EventChannel)
	filterMap, err := generateEventFilters(options.Filters, options.Since, options.Until)
	if err != nil {
		return errors.Wrapf(err, "failed to parse event filters")
	}
	t, err := e.getTail(options)
	if err != nil {
		return err
	}
	if len(options.Until) > 0 {
		untilTime, err := util.ParseInputTime(options.Until)
		if err != nil {
			return err
		}
		go func() {
			time.Sleep(time.Until(untilTime))
			t.Stop()
		}()
	}
	funcDone := make(chan bool)
	copy := true
	go func() {
		select {
		case <-funcDone:
			// Do nothing
		case <-ctx.Done():
			copy = false
			t.Kill(errors.New("hangup by client"))
		}
	}()
	for line := range t.Lines {
		select {
		case <-ctx.Done():
			// the consumer has cancelled
			return nil
		default:
			// fallthrough
		}

		event, err := newEventFromJSONString(line.Text)
		if err != nil {
			return err
		}
		switch event.Type {
		case Image, Volume, Pod, System, Container, Network:
		//	no-op
		default:
			return errors.Errorf("event type %s is not valid in %s", event.Type.String(), e.options.LogFilePath)
		}
		if copy && applyFilters(event, filterMap) {
			options.EventChannel <- event
		}
	}
	funcDone <- true
	return nil
}

// String returns a string representation of the logger
func (e EventLogFile) String() string {
	return LogFile.String()
}
