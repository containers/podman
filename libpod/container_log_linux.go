//+build linux
//+build systemd

package libpod

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containers/podman/v3/libpod/events"
	"github.com/containers/podman/v3/libpod/logs"
	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// journaldLogOut is the journald priority signifying stdout
	journaldLogOut = "6"

	// journaldLogErr is the journald priority signifying stderr
	journaldLogErr = "3"
)

func (c *Container) readFromJournal(ctx context.Context, options *logs.LogOptions, logChannel chan *logs.LogLine) error {
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
		return errors.Wrapf(err, "adding filter to journald logger: %v", match)
	}
	match = sdjournal.Match{Field: "PODMAN_ID", Value: c.ID()}
	if err := journal.AddMatch(match.String()); err != nil {
		return errors.Wrapf(err, "adding filter to journald logger: %v", match)
	}

	// Add the filter for logs.  Note the disjunction so that we match
	// either the events or the logs.
	if err := journal.AddDisjunction(); err != nil {
		return errors.Wrap(err, "adding filter disjunction to journald logger")
	}
	match = sdjournal.Match{Field: "CONTAINER_ID_FULL", Value: c.ID()}
	if err := journal.AddMatch(match.String()); err != nil {
		return errors.Wrapf(err, "adding filter to journald logger: %v", match)
	}

	if err := journal.SeekHead(); err != nil {
		return err
	}
	// API requires Next() immediately after SeekHead().
	if _, err := journal.Next(); err != nil {
		return errors.Wrap(err, "initial journal cursor")
	}

	// API requires a next|prev before getting a cursor.
	if _, err := journal.Previous(); err != nil {
		return errors.Wrap(err, "initial journal cursor")
	}

	// Note that the initial cursor may not yet be ready, so we'll do an
	// exponential backoff.
	var cursor string
	var cursorError error
	for i := 1; i <= 3; i++ {
		cursor, cursorError = journal.GetCursor()
		if err != nil {
			continue
		}
		time.Sleep(time.Duration(i*100) * time.Millisecond)
		break
	}
	if cursorError != nil {
		return errors.Wrap(cursorError, "inital journal cursor")
	}

	// We need the container's events in the same journal to guarantee
	// consistency, see #10323.
	if options.Follow && c.runtime.config.Engine.EventsLogger != "journald" {
		return errors.Errorf("using --follow with the journald --log-driver but without the journald --events-backend (%s) is not supported", c.runtime.config.Engine.EventsLogger)
	}

	options.WaitGroup.Add(1)
	go func() {
		defer func() {
			options.WaitGroup.Done()
			if err := journal.Close(); err != nil {
				logrus.Errorf("Unable to close journal: %v", err)
			}
		}()

		afterTimeStamp := false        // needed for options.Since
		tailQueue := []*logs.LogLine{} // needed for options.Tail
		doTail := options.Tail > 0
		for {
			select {
			case <-ctx.Done():
				// Remote client may have closed/lost the connection.
				return
			default:
				// Fallthrough
			}

			if _, err := journal.Next(); err != nil {
				logrus.Errorf("Failed to move journal cursor to next entry: %v", err)
				return
			}
			latestCursor, err := journal.GetCursor()
			if err != nil {
				logrus.Errorf("Failed to get journal cursor: %v", err)
				return
			}

			// Hit the end of the journal.
			if cursor == latestCursor {
				if doTail {
					// Flush *once* we hit the end of the journal.
					startIndex := int64(len(tailQueue)-1) - options.Tail
					if startIndex < 0 {
						startIndex = 0
					}
					for i := startIndex; i < int64(len(tailQueue)); i++ {
						logChannel <- tailQueue[i]
					}
					tailQueue = nil
					doTail = false
				}
				// Unless we follow, quit.
				if !options.Follow {
					return
				}
				// Sleep until something's happening on the journal.
				journal.Wait(sdjournal.IndefiniteWait)
				continue
			}
			cursor = latestCursor

			entry, err := journal.GetEntry()
			if err != nil {
				logrus.Errorf("Failed to get journal entry: %v", err)
				return
			}

			if !afterTimeStamp {
				entryTime := time.Unix(0, int64(entry.RealtimeTimestamp)*int64(time.Microsecond))
				if entryTime.Before(options.Since) {
					continue
				}
				afterTimeStamp = true
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
				if status == events.Exited {
					return
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
				logrus.Errorf("Failed to parse journald log entry: %v", err)
				return
			}

			logLine, err := logs.NewJournaldLogLine(message, options.Multi)
			if err != nil {
				logrus.Errorf("Failed parse log line: %v", err)
				return
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
		return "", fmt.Errorf("no CONTAINER_ID_FULL field present in journal entry")
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

	return output, nil
}

func formatterMessage(entry *sdjournal.JournalEntry) (string, error) {
	// Finally, append the message
	msg, ok := entry.Fields["MESSAGE"]
	if !ok {
		return "", fmt.Errorf("no MESSAGE field present in journal entry")
	}
	msg = strings.TrimSuffix(msg, "\n")
	return msg, nil
}
