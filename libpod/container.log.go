package libpod

import (
	"os"

	"github.com/containers/libpod/libpod/logs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Log is a runtime function that can read one or more container logs.
func (r *Runtime) Log(containers []*Container, options *logs.LogOptions, logChannel chan *logs.LogLine) error {
	for _, ctr := range containers {
		if err := ctr.ReadLog(options, logChannel); err != nil {
			return err
		}
	}
	return nil
}

// ReadLog reads a containers log based on the input options and returns loglines over a channel
func (c *Container) ReadLog(options *logs.LogOptions, logChannel chan *logs.LogLine) error {
	// TODO Skip sending logs until journald logs can be read
	// TODO make this not a magic string
	if c.LogDriver() == JournaldLogging {
		return c.readFromJournal(options, logChannel)
	}
	return c.readFromLogFile(options, logChannel)
}

func (c *Container) readFromLogFile(options *logs.LogOptions, logChannel chan *logs.LogLine) error {
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
			if nll.Since(options.Since) {
				logChannel <- nll
			}
		}
	}

	go func() {
		var partial string
		for line := range t.Lines {
			nll, err := logs.NewLogLine(line.Text)
			if err != nil {
				logrus.Error(err)
				continue
			}
			if nll.Partial() {
				partial += nll.Msg
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
