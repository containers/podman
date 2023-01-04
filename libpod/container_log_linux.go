//go:build linux && systemd
// +build linux,systemd

package libpod

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/libpod/logs"
	"github.com/coreos/go-systemd/v22/journal"
	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/sirupsen/logrus"
)

const (
	// journaldLogOut is the journald priority signifying stdout
	journaldLogOut = "6"

	// journaldLogErr is the journald priority signifying stderr
	journaldLogErr = "3"
)

func init() {
	logDrivers = append(logDrivers, define.JournaldLogging)
}

// initializeJournal will write an empty string to the journal
// when a journal is created. This solves a problem when people
// attempt to read logs from a container that has never had stdout/stderr
func (c *Container) initializeJournal(ctx context.Context) error {
	m := make(map[string]string)
	m["SYSLOG_IDENTIFIER"] = "podman"
	m["PODMAN_ID"] = c.ID()
	history := events.History
	m["PODMAN_EVENT"] = history.String()
	container := events.Container
	m["PODMAN_TYPE"] = container.String()
	m["PODMAN_TIME"] = time.Now().Format(time.RFC3339Nano)
	return journal.Send("", journal.PriInfo, m)
}

func (c *Container) readFromJournal(ctx context.Context, options *logs.LogOptions, logChannel chan *logs.LogLine, colorID int64) error {
	// We need the container's events in the same journal to guarantee
	// consistency, see #10323.
	if options.Follow && c.runtime.config.Engine.EventsLogger != "journald" {
		return fmt.Errorf("using --follow with the journald --log-driver but without the journald --events-backend (%s) is not supported", c.runtime.config.Engine.EventsLogger)
	}

	journal, err := sdjournal.NewJournal()
	if err != nil {
		return err
	}
	// While logs are written to the `logChannel`, we inspect each event
	// and stop once the container has died.  Having logs and events in one
	// stream prevents a race condition that we faced in #10323.

	// Add the filters for events.
	match := sdjournal.Match{Field: "SYSLOG_IDENTIFIER", Value: "podman"}
	if err := journal.AddMatch(match.String()); err != nil {
		return fmt.Errorf("adding filter to journald logger: %v: %w", match, err)
	}
	match = sdjournal.Match{Field: "PODMAN_ID", Value: c.ID()}
	if err := journal.AddMatch(match.String()); err != nil {
		return fmt.Errorf("adding filter to journald logger: %v: %w", match, err)
	}

	// Add the filter for logs.  Note the disjunction so that we match
	// either the events or the logs.
	if err := journal.AddDisjunction(); err != nil {
		return fmt.Errorf("adding filter disjunction to journald logger: %w", err)
	}
	match = sdjournal.Match{Field: "CONTAINER_ID_FULL", Value: c.ID()}
	if err := journal.AddMatch(match.String()); err != nil {
		return fmt.Errorf("adding filter to journald logger: %v: %w", match, err)
	}

	if options.Since.IsZero() {
		if err := journal.SeekHead(); err != nil {
			return err
		}
	} else {
		// seek based on time which helps to reduce unnecessary event reads
		if err := journal.SeekRealtimeUsec(uint64(options.Since.UnixMicro())); err != nil {
			return err
		}
	}

	c.lock.Lock()
	if err := c.syncContainer(); err != nil {
		c.lock.Unlock()
		return err
	}
	// The initial "containerCouldBeLogging" state must be correct, we cannot rely on the start event being still in the journal.
	// This can happen if the journal was rotated after the container was started or when --since is used.
	// https://github.com/containers/podman/issues/16950
	containerCouldBeLogging := c.ensureState(define.ContainerStateRunning, define.ContainerStateStopping)
	c.lock.Unlock()

	options.WaitGroup.Add(1)
	go func() {
		defer func() {
			options.WaitGroup.Done()
			if err := journal.Close(); err != nil {
				logrus.Errorf("Unable to close journal: %v", err)
			}
		}()

		tailQueue := []*logs.LogLine{} // needed for options.Tail
		doTail := options.Tail >= 0
		doTailFunc := func() {
			// Flush *once* we hit the end of the journal.
			startIndex := int64(len(tailQueue))
			outputLines := int64(0)
			for startIndex > 0 && outputLines < options.Tail {
				startIndex--
				for startIndex > 0 && tailQueue[startIndex].Partial() {
					startIndex--
				}
				outputLines++
			}
			for i := startIndex; i < int64(len(tailQueue)); i++ {
				logChannel <- tailQueue[i]
			}
			tailQueue = nil
			doTail = false
		}
		for {
			entry, err := events.GetNextEntry(ctx, journal, !doTail && options.Follow && containerCouldBeLogging, options.Until)
			if err != nil {
				logrus.Errorf("Failed to get journal entry: %v", err)
				return
			}
			// entry nil == EOF in journal
			if entry == nil {
				if doTail {
					doTailFunc()
					continue
				}
				return
			}

			entryTime := time.Unix(0, int64(entry.RealtimeTimestamp)*int64(time.Microsecond))
			if (entryTime.Before(options.Since) && !options.Since.IsZero()) || (entryTime.After(options.Until) && !options.Until.IsZero()) {
				continue
			}
			// If we're reading an event and the container exited/died,
			// then we're done and can return.
			event, ok := entry.Fields["PODMAN_EVENT"]
			if ok {
				status, err := events.StringToStatus(event)
				if err != nil {
					logrus.Errorf("Failed to translate event: %v", err)
					return
				}
				switch status {
				case events.History, events.Init, events.Start, events.Restart:
					containerCouldBeLogging = true
				case events.Exited:
					containerCouldBeLogging = false
				}
				continue
			}

			var message string
			var formatError error

			if options.Multi {
				message, formatError = journalFormatterWithID(entry)
			} else {
				message, formatError = journalFormatter(entry)
			}

			if formatError != nil {
				logrus.Errorf("Failed to parse journald log entry: %v", formatError)
				return
			}

			logLine, err := logs.NewJournaldLogLine(message, options.Multi)
			logLine.ColorID = colorID
			if err != nil {
				logrus.Errorf("Failed parse log line: %v", err)
				return
			}
			if options.UseName {
				logLine.CName = c.Name()
			}
			if doTail {
				tailQueue = append(tailQueue, logLine)
				continue
			}
			logChannel <- logLine
		}
	}()

	return nil
}

func journalFormatterWithID(entry *sdjournal.JournalEntry) (string, error) {
	output, err := formatterPrefix(entry)
	if err != nil {
		return "", err
	}

	id, ok := entry.Fields["CONTAINER_ID_FULL"]
	if !ok {
		return "", errors.New("no CONTAINER_ID_FULL field present in journal entry")
	}
	if len(id) > 12 {
		id = id[:12]
	}
	output += fmt.Sprintf("%s ", id)
	// Append message
	msg, err := formatterMessage(entry)
	if err != nil {
		return "", err
	}
	output += msg
	return output, nil
}

func journalFormatter(entry *sdjournal.JournalEntry) (string, error) {
	output, err := formatterPrefix(entry)
	if err != nil {
		return "", err
	}
	// Append message
	msg, err := formatterMessage(entry)
	if err != nil {
		return "", err
	}
	output += msg
	return output, nil
}

func formatterPrefix(entry *sdjournal.JournalEntry) (string, error) {
	usec := entry.RealtimeTimestamp
	tsString := time.Unix(0, int64(usec)*int64(time.Microsecond)).Format(logs.LogTimeFormat)
	output := fmt.Sprintf("%s ", tsString)
	priority, ok := entry.Fields["PRIORITY"]
	if !ok {
		return "", errors.New("no PRIORITY field present in journal entry")
	}
	switch priority {
	case journaldLogOut:
		output += "stdout "
	case journaldLogErr:
		output += "stderr "
	default:
		return "", errors.New("unexpected PRIORITY field in journal entry")
	}

	// if CONTAINER_PARTIAL_MESSAGE is defined, the log type is "P"
	if _, ok := entry.Fields["CONTAINER_PARTIAL_MESSAGE"]; ok {
		output += fmt.Sprintf("%s ", logs.PartialLogType)
	} else {
		output += fmt.Sprintf("%s ", logs.FullLogType)
	}

	return output, nil
}

func formatterMessage(entry *sdjournal.JournalEntry) (string, error) {
	// Finally, append the message
	msg, ok := entry.Fields["MESSAGE"]
	if !ok {
		return "", errors.New("no MESSAGE field present in journal entry")
	}
	msg = strings.TrimSuffix(msg, "\n")
	return msg, nil
}
