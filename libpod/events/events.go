package events

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
)

// ErrNoJournaldLogging indicates that there is no journald logging
// supported (requires libsystemd)
var ErrNoJournaldLogging = errors.New("No support for journald logging")

// String returns a string representation of EventerType
func (et EventerType) String() string {
	switch et {
	case LogFile:
		return "file"
	case Journald:
		return "journald"
	case Null:
		return "none"
	default:
		return "invalid"
	}
}

// IsValidEventer checks if the given string is a valid eventer type.
func IsValidEventer(eventer string) bool {
	switch eventer {
	case LogFile.String():
		return true
	case Journald.String():
		return true
	case Null.String():
		return true
	default:
		return false
	}
}

// NewEvent creates a event struct and populates with
// the given status and time.
func NewEvent(status Status) Event {
	return Event{
		Status: status,
		Time:   time.Now(),
	}
}

// Recycle checks if the event log has reach a limit and if so
// renames the current log and starts a new one.  The remove bool
// indicates the old log file should be deleted.
func (e *Event) Recycle(path string, remove bool) error {
	return errors.New("not implemented")
}

// ToJSONString returns the event as a json'ified string
func (e *Event) ToJSONString() (string, error) {
	b, err := json.Marshal(e)
	return string(b), err
}

// ToHumanReadable returns human readable event as a formatted string
func (e *Event) ToHumanReadable() string {
	var humanFormat string
	switch e.Type {
	case Container, Pod:
		humanFormat = fmt.Sprintf("%s %s %s %s (image=%s, name=%s", e.Time, e.Type, e.Status, e.ID, e.Image, e.Name)
		// check if the container has labels and add it to the output
		if len(e.Attributes) > 0 {
			for k, v := range e.Attributes {
				humanFormat += fmt.Sprintf(", %s=%s", k, v)
			}
		}
		humanFormat += ")"
	case Network:
		humanFormat = fmt.Sprintf("%s %s %s %s (container=%s, name=%s)", e.Time, e.Type, e.Status, e.ID, e.ID, e.Network)
	case Image:
		humanFormat = fmt.Sprintf("%s %s %s %s %s", e.Time, e.Type, e.Status, e.ID, e.Name)
	case System:
		humanFormat = fmt.Sprintf("%s %s %s", e.Time, e.Type, e.Status)
	case Volume:
		humanFormat = fmt.Sprintf("%s %s %s %s", e.Time, e.Type, e.Status, e.Name)
	}
	return humanFormat
}

// NewEventFromString takes stringified json and converts
// it to an event
func newEventFromJSONString(event string) (*Event, error) {
	e := Event{}
	if err := json.Unmarshal([]byte(event), &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// ToString converts a Type to a string
func (t Type) String() string {
	return string(t)
}

// ToString converts a status to a string
func (s Status) String() string {
	return string(s)
}

// StringToType converts string to an EventType
func StringToType(name string) (Type, error) {
	switch name {
	case Container.String():
		return Container, nil
	case Image.String():
		return Image, nil
	case Network.String():
		return Network, nil
	case Pod.String():
		return Pod, nil
	case System.String():
		return System, nil
	case Volume.String():
		return Volume, nil
	case "":
		return "", ErrEventTypeBlank
	}
	return "", errors.Errorf("unknown event type %q", name)
}

// StringToStatus converts a string to an Event Status
// TODO if we add more events, we might consider a go-generator to
// create the switch statement
func StringToStatus(name string) (Status, error) {
	switch name {
	case Attach.String():
		return Attach, nil
	case Build.String():
		return Build, nil
	case Checkpoint.String():
		return Checkpoint, nil
	case Cleanup.String():
		return Cleanup, nil
	case Commit.String():
		return Commit, nil
	case Create.String():
		return Create, nil
	case Exec.String():
		return Exec, nil
	case Exited.String():
		return Exited, nil
	case Export.String():
		return Export, nil
	case History.String():
		return History, nil
	case Import.String():
		return Import, nil
	case Init.String():
		return Init, nil
	case Kill.String():
		return Kill, nil
	case LoadFromArchive.String():
		return LoadFromArchive, nil
	case Mount.String():
		return Mount, nil
	case NetworkConnect.String():
		return NetworkConnect, nil
	case NetworkDisconnect.String():
		return NetworkDisconnect, nil
	case Pause.String():
		return Pause, nil
	case Prune.String():
		return Prune, nil
	case Pull.String():
		return Pull, nil
	case Push.String():
		return Push, nil
	case Refresh.String():
		return Refresh, nil
	case Remove.String():
		return Remove, nil
	case Renumber.String():
		return Renumber, nil
	case Restart.String():
		return Restart, nil
	case Restore.String():
		return Restore, nil
	case Save.String():
		return Save, nil
	case Start.String():
		return Start, nil
	case Stop.String():
		return Stop, nil
	case Sync.String():
		return Sync, nil
	case Tag.String():
		return Tag, nil
	case Unmount.String():
		return Unmount, nil
	case Unpause.String():
		return Unpause, nil
	case Untag.String():
		return Untag, nil
	}
	return "", errors.Errorf("unknown event status %q", name)
}

func (e EventLogFile) getTail(options ReadOptions) (*tail.Tail, error) {
	reopen := true
	seek := tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
	if options.FromStart || !options.Stream {
		seek.Whence = 0
		reopen = false
	}
	stream := options.Stream
	return tail.TailFile(e.options.LogFilePath, tail.Config{ReOpen: reopen, Follow: stream, Location: &seek, Logger: tail.DiscardingLogger, Poll: true})
}
