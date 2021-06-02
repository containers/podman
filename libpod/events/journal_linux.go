// +build systemd

package events

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/containers/podman/v3/pkg/util"
	"github.com/coreos/go-systemd/v22/journal"
	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// DefaultEventerType is journald when systemd is available
const DefaultEventerType = Journald

// EventJournalD is the journald implementation of an eventer
type EventJournalD struct {
	options EventerOptions
}

// newEventJournalD creates a new journald Eventer
func newEventJournalD(options EventerOptions) (Eventer, error) {
	return EventJournalD{options}, nil
}

// Write to journald
func (e EventJournalD) Write(ee Event) error {
	m := make(map[string]string)
	m["SYSLOG_IDENTIFIER"] = "podman"
	m["PODMAN_EVENT"] = ee.Status.String()
	m["PODMAN_TYPE"] = ee.Type.String()
	m["PODMAN_TIME"] = ee.Time.Format(time.RFC3339Nano)

	// Add specialized information based on the podman type
	switch ee.Type {
	case Image:
		m["PODMAN_NAME"] = ee.Name
		m["PODMAN_ID"] = ee.ID
	case Container, Pod:
		m["PODMAN_IMAGE"] = ee.Image
		m["PODMAN_NAME"] = ee.Name
		m["PODMAN_ID"] = ee.ID
		if ee.ContainerExitCode != 0 {
			m["PODMAN_EXIT_CODE"] = strconv.Itoa(ee.ContainerExitCode)
		}
		// If we have container labels, we need to convert them to a string so they
		// can be recorded with the event
		if len(ee.Details.Attributes) > 0 {
			b, err := json.Marshal(ee.Details.Attributes)
			if err != nil {
				return err
			}
			m["PODMAN_LABELS"] = string(b)
		}
	case Network:
		m["PODMAN_ID"] = ee.ID
		m["PODMAN_NETWORK_NAME"] = ee.Network
	case Volume:
		m["PODMAN_NAME"] = ee.Name
	}
	return journal.Send(string(ee.ToHumanReadable()), journal.PriInfo, m)
}

// Read reads events from the journal and sends qualified events to the event channel
func (e EventJournalD) Read(ctx context.Context, options ReadOptions) error {
	defer close(options.EventChannel)
	filterMap, err := generateEventFilters(options.Filters, options.Since, options.Until)
	if err != nil {
		return errors.Wrapf(err, "failed to parse event filters")
	}
	var untilTime time.Time
	if len(options.Until) > 0 {
		untilTime, err = util.ParseInputTime(options.Until)
		if err != nil {
			return err
		}
	}
	j, err := sdjournal.NewJournal()
	if err != nil {
		return err
	}
	defer func() {
		if err := j.Close(); err != nil {
			logrus.Errorf("Unable to close journal :%v", err)
		}
	}()
	// match only podman journal entries
	podmanJournal := sdjournal.Match{Field: "SYSLOG_IDENTIFIER", Value: "podman"}
	if err := j.AddMatch(podmanJournal.String()); err != nil {
		return errors.Wrap(err, "failed to add journal filter for event log")
	}

	if len(options.Since) == 0 && len(options.Until) == 0 && options.Stream {
		if err := j.SeekTail(); err != nil {
			return errors.Wrap(err, "failed to seek end of journal")
		}
		// After SeekTail calling Next moves to a random entry.
		// To prevent this we have to call Previous first.
		// see: https://bugs.freedesktop.org/show_bug.cgi?id=64614
		if _, err := j.Previous(); err != nil {
			return errors.Wrap(err, "failed to move journal cursor to previous entry")
		}
	}

	// the api requires a next|prev before getting a cursor
	if _, err := j.Next(); err != nil {
		return errors.Wrap(err, "failed to move journal cursor to next entry")
	}

	prevCursor, err := j.GetCursor()
	if err != nil {
		return errors.Wrap(err, "failed to get journal cursor")
	}
	for {
		select {
		case <-ctx.Done():
			// the consumer has cancelled
			return nil
		default:
			// fallthrough
		}

		if _, err := j.Next(); err != nil {
			return errors.Wrap(err, "failed to move journal cursor to next entry")
		}
		newCursor, err := j.GetCursor()
		if err != nil {
			return errors.Wrap(err, "failed to get journal cursor")
		}
		if prevCursor == newCursor {
			if !options.Stream || (len(options.Until) > 0 && time.Now().After(untilTime)) {
				break
			}
			t := sdjournal.IndefiniteWait
			if len(options.Until) > 0 {
				t = time.Until(untilTime)
			}
			_ = j.Wait(t)
			continue
		}
		prevCursor = newCursor

		entry, err := j.GetEntry()
		if err != nil {
			return errors.Wrap(err, "failed to read journal entry")
		}
		newEvent, err := newEventFromJournalEntry(entry)
		if err != nil {
			// We can't decode this event.
			// Don't fail hard - that would make events unusable.
			// Instead, log and continue.
			if errors.Cause(err) != ErrEventTypeBlank {
				logrus.Errorf("Unable to decode event: %v", err)
			}
			continue
		}
		if applyFilters(newEvent, filterMap) {
			options.EventChannel <- newEvent
		}
	}
	return nil

}

func newEventFromJournalEntry(entry *sdjournal.JournalEntry) (*Event, error) { //nolint
	newEvent := Event{}
	eventType, err := StringToType(entry.Fields["PODMAN_TYPE"])
	if err != nil {
		return nil, err
	}
	eventTime, err := time.Parse(time.RFC3339Nano, entry.Fields["PODMAN_TIME"])
	if err != nil {
		return nil, err
	}
	eventStatus, err := StringToStatus(entry.Fields["PODMAN_EVENT"])
	if err != nil {
		return nil, err
	}
	newEvent.Type = eventType
	newEvent.Time = eventTime
	newEvent.Status = eventStatus
	newEvent.Name = entry.Fields["PODMAN_NAME"]

	switch eventType {
	case Container, Pod:
		newEvent.ID = entry.Fields["PODMAN_ID"]
		newEvent.Image = entry.Fields["PODMAN_IMAGE"]
		if code, ok := entry.Fields["PODMAN_EXIT_CODE"]; ok {
			intCode, err := strconv.Atoi(code)
			if err != nil {
				logrus.Errorf("Error parsing event exit code %s", code)
			} else {
				newEvent.ContainerExitCode = intCode
			}
		}

		// we need to check for the presence of labels recorded to a container event
		if stringLabels, ok := entry.Fields["PODMAN_LABELS"]; ok && len(stringLabels) > 0 {
			labels := make(map[string]string, 0)
			if err := json.Unmarshal([]byte(stringLabels), &labels); err != nil {
				return nil, err
			}

			// if we have labels, add them to the event
			if len(labels) > 0 {
				newEvent.Details = Details{Attributes: labels}
			}
		}
	case Network:
		newEvent.ID = entry.Fields["PODMAN_ID"]
		newEvent.Network = entry.Fields["PODMAN_NETWORK_NAME"]
	case Image:
		newEvent.ID = entry.Fields["PODMAN_ID"]
	}
	return &newEvent, nil
}

// String returns a string representation of the logger
func (e EventJournalD) String() string {
	return Journald.String()
}
