package events

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/containers/storage"
	"github.com/pkg/errors"
)

// Event describes the attributes of a libpod event
type Event struct {
	// ContainerExitCode is for storing the exit code of a container which can
	// be used for "internal" event notification
	ContainerExitCode int
	// ID can be for the container, image, volume, etc
	ID string
	// Image used where applicable
	Image string
	// Name where applicable
	Name string
	// Status describes the event that occurred
	Status Status
	// Time the event occurred
	Time time.Time
	// Type of event that occurred
	Type Type
}

// Type of event that occurred (container, volume, image, pod, etc)
type Type string

// Status describes the actual event action (stop, start, create, kill)
type Status string

const (
	// If you add or subtract any values to the following lists, make sure you also update
	// the switch statements below and the enums for EventType or EventStatus in the
	// varlink description file.

	// Container - event is related to containers
	Container Type = "container"
	// Image - event is related to images
	Image Type = "image"
	// Pod - event is related to pods
	Pod Type = "pod"
	// Volume - event is related to volumes
	Volume Type = "volume"

	// Attach ...
	Attach Status = "attach"
	// Checkpoint ...
	Checkpoint Status = "checkpoint"
	// Cleanup ...
	Cleanup Status = "cleanup"
	// Commit ...
	Commit Status = "commit"
	// Create ...
	Create Status = "create"
	// Exec ...
	Exec Status = "exec"
	// Exited indicates that a container's process died
	Exited Status = "died"
	// Export ...
	Export Status = "export"
	// History ...
	History Status = "history"
	// Import ...
	Import Status = "import"
	// Init ...
	Init Status = "init"
	// Kill ...
	Kill Status = "kill"
	// LoadFromArchive ...
	LoadFromArchive Status = "status"
	// Mount ...
	Mount Status = "mount"
	// Pause ...
	Pause Status = "pause"
	// Prune ...
	Prune Status = "prune"
	// Pull ...
	Pull Status = "pull"
	// Push ...
	Push Status = "push"
	// Remove ...
	Remove Status = "remove"
	// Restore ...
	Restore Status = "restore"
	// Save ...
	Save Status = "save"
	// Start ...
	Start Status = "start"
	// Stop ...
	Stop Status = "stop"
	// Sync ...
	Sync Status = "sync"
	// Tag ...
	Tag Status = "tag"
	// Unmount ...
	Unmount Status = "unmount"
	// Unpause ...
	Unpause Status = "unpause"
	// Untag ...
	Untag Status = "untag"
)

// EventFilter for filtering events
type EventFilter func(*Event) bool

// NewEvent creates a event struct and populates with
// the given status and time.
func NewEvent(status Status) Event {
	return Event{
		Status: status,
		Time:   time.Now(),
	}
}

// Write will record the event to the given path
func (e *Event) Write(path string) error {
	// We need to lock events file
	lock, err := storage.GetLockfile(path + ".lock")
	if err != nil {
		return err
	}
	lock.Lock()
	defer lock.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	defer f.Close()
	eventJSONString, err := e.ToJSONString()
	if err != nil {
		return err
	}
	if _, err := f.WriteString(fmt.Sprintf("%s\n", eventJSONString)); err != nil {
		return err
	}
	return nil
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
		humanFormat = fmt.Sprintf("%s %s %s %s (image=%s, name=%s)", e.Time, e.Type, e.Status, e.ID, e.Image, e.Name)
	case Image:
		humanFormat = fmt.Sprintf("%s %s %s %s %s", e.Time, e.Type, e.Status, e.ID, e.Name)
	case Volume:
		humanFormat = fmt.Sprintf("%s %s %s %s", e.Time, e.Type, e.Status, e.Name)
	}
	return humanFormat
}

// NewEventFromString takes stringified json and converts
// it to an event
func NewEventFromString(event string) (*Event, error) {
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
	case Pod.String():
		return Pod, nil
	case Volume.String():
		return Volume, nil
	}
	return "", errors.Errorf("unknown event type %s", name)
}

// StringToStatus converts a string to an Event Status
// TODO if we add more events, we might consider a go-generator to
// create the switch statement
func StringToStatus(name string) (Status, error) {
	switch name {
	case Attach.String():
		return Attach, nil
	case Checkpoint.String():
		return Checkpoint, nil
	case Restore.String():
		return Restore, nil
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
	case Pause.String():
		return Pause, nil
	case Prune.String():
		return Prune, nil
	case Pull.String():
		return Pull, nil
	case Push.String():
		return Push, nil
	case Remove.String():
		return Remove, nil
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
	return "", errors.Errorf("unknown event status %s", name)
}
