package libpod

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/libpod/logs"
	"github.com/nxadm/tail/watch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// logDrivers stores the currently available log drivers, do not modify
var logDrivers []string

func init() {
	logDrivers = append(logDrivers, define.KubernetesLogging, define.NoLogging, define.PassthroughLogging)
}

// Log is a runtime function that can read one or more container logs.
func (r *Runtime) Log(ctx context.Context, containers []*Container, options *logs.LogOptions, logChannel chan *logs.LogLine) error {
	for c, ctr := range containers {
		if err := ctr.ReadLog(ctx, options, logChannel, int64(c)); err != nil {
			return err
		}
	}
	return nil
}

// ReadLog reads a containers log based on the input options and returns log lines over a channel.
func (c *Container) ReadLog(ctx context.Context, options *logs.LogOptions, logChannel chan *logs.LogLine, colorID int64) error {
	switch c.LogDriver() {
	case define.PassthroughLogging:
		return errors.Wrapf(define.ErrNoLogs, "this container is using the 'passthrough' log driver, cannot read logs")
	case define.NoLogging:
		return errors.Wrapf(define.ErrNoLogs, "this container is using the 'none' log driver, cannot read logs")
	case define.JournaldLogging:
		return c.readFromJournal(ctx, options, logChannel, colorID)
	case define.JSONLogging:
		// TODO provide a separate implementation of this when Conmon
		// has support.
		fallthrough
	case define.KubernetesLogging, "":
		return c.readFromLogFile(ctx, options, logChannel, colorID)
	default:
		return errors.Wrapf(define.ErrInternal, "unrecognized log driver %q, cannot read logs", c.LogDriver())
	}
}

func (c *Container) readFromLogFile(ctx context.Context, options *logs.LogOptions, logChannel chan *logs.LogLine, colorID int64) error {
	t, tailLog, err := logs.GetLogFile(c.LogPath(), options)
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
			nll.CName = c.Name()
			nll.ColorID = colorID
			if nll.Since(options.Since) && nll.Until(options.Until) {
				logChannel <- nll
			}
		}
	}

	go func() {
		defer options.WaitGroup.Done()

		var partial string
		for line := range t.Lines {
			select {
			case <-ctx.Done():
				// the consumer has cancelled
				return
			default:
				// fallthrough
			}
			nll, err := logs.NewLogLine(line.Text)
			if err != nil {
				logrus.Errorf("Getting new log line: %v", err)
				continue
			}
			if nll.Partial() {
				partial += nll.Msg
				continue
			} else if !nll.Partial() && len(partial) > 0 {
				nll.Msg = partial + nll.Msg
				partial = ""
			}
			nll.CID = c.ID()
			nll.CName = c.Name()
			nll.ColorID = colorID
			if nll.Since(options.Since) && nll.Until(options.Until) {
				logChannel <- nll
			}
		}
	}()
	// Check if container is still running or paused
	if options.Follow {
		// If the container isn't running or if we encountered an error
		// getting its state, instruct the logger to read the file
		// until EOF.
		state, err := c.State()
		if err != nil || state != define.ContainerStateRunning {
			if err != nil && errors.Cause(err) != define.ErrNoSuchCtr {
				logrus.Errorf("Getting container state: %v", err)
			}
			go func() {
				// Make sure to wait at least for the poll duration
				// before stopping the file logger (see #10675).
				time.Sleep(watch.POLL_DURATION)
				tailError := t.StopAtEOF()
				if tailError != nil && tailError.Error() != "tail: stop at eof" {
					logrus.Errorf("Stopping logger: %v", tailError)
				}
			}()
			return nil
		}

		// The container is running, so we need to wait until the container exited
		go func() {
			eventChannel := make(chan *events.Event)
			eventOptions := events.ReadOptions{
				EventChannel: eventChannel,
				Filters:      []string{"event=died", "container=" + c.ID()},
				Stream:       true,
			}
			go func() {
				if err := c.runtime.Events(ctx, eventOptions); err != nil {
					logrus.Errorf("Waiting for container to exit: %v", err)
				}
			}()
			// Now wait for the died event and signal to finish
			// reading the log until EOF.
			<-eventChannel
			// Make sure to wait at least for the poll duration
			// before stopping the file logger (see #10675).
			time.Sleep(watch.POLL_DURATION)
			tailError := t.StopAtEOF()
			if tailError != nil && fmt.Sprintf("%v", tailError) != "tail: stop at eof" {
				logrus.Errorf("Stopping logger: %v", tailError)
			}
		}()
	}
	return nil
}
