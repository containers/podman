package libpod

import (
	"os"

	"github.com/containers/libpod/libpod/events"
	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// newContainerEvent creates a new event based on a container
func (c *Container) newContainerEvent(status events.Status) {
	e := events.NewEvent(status)
	e.ID = c.ID()
	e.Name = c.Name()
	e.Image = c.config.RootfsImageName
	e.Type = events.Container
	if err := e.Write(c.runtime.config.EventsLogFilePath); err != nil {
		logrus.Errorf("unable to write event to %s", c.runtime.config.EventsLogFilePath)
	}
}

// newContainerExitedEvent creates a new event for a container's death
func (c *Container) newContainerExitedEvent(exitCode int32) {
	e := events.NewEvent(events.Exited)
	e.ID = c.ID()
	e.Name = c.Name()
	e.Image = c.config.RootfsImageName
	e.Type = events.Container
	e.ContainerExitCode = int(exitCode)
	if err := e.Write(c.runtime.config.EventsLogFilePath); err != nil {
		logrus.Errorf("unable to write event to %s", c.runtime.config.EventsLogFilePath)
	}
}

// newPodEvent creates a new event for a libpod pod
func (p *Pod) newPodEvent(status events.Status) {
	e := events.NewEvent(status)
	e.ID = p.ID()
	e.Name = p.Name()
	e.Type = events.Pod
	if err := e.Write(p.runtime.config.EventsLogFilePath); err != nil {
		logrus.Errorf("unable to write event to %s", p.runtime.config.EventsLogFilePath)
	}
}

// newVolumeEvent creates a new event for a libpod volume
func (v *Volume) newVolumeEvent(status events.Status) {
	e := events.NewEvent(status)
	e.Name = v.Name()
	e.Type = events.Volume
	if err := e.Write(v.runtime.config.EventsLogFilePath); err != nil {
		logrus.Errorf("unable to write event to %s", v.runtime.config.EventsLogFilePath)
	}
}

// Events is a wrapper function for everyone to begin tailing the events log
// with options
func (r *Runtime) Events(fromStart, stream bool, options []events.EventFilter, eventChannel chan *events.Event) error {
	if !r.valid {
		return ErrRuntimeStopped
	}

	t, err := r.getTail(fromStart, stream)
	if err != nil {
		return err
	}
	for line := range t.Lines {
		event, err := events.NewEventFromString(line.Text)
		if err != nil {
			return err
		}
		switch event.Type {
		case events.Image, events.Volume, events.Pod, events.Container:
		//	no-op
		default:
			return errors.Errorf("event type %s is not valid in %s", event.Type.String(), r.config.EventsLogFilePath)
		}
		include := true
		for _, filter := range options {
			include = include && filter(event)
		}
		if include {
			eventChannel <- event
		}
	}
	close(eventChannel)
	return nil
}

func (r *Runtime) getTail(fromStart, stream bool) (*tail.Tail, error) {
	reopen := true
	seek := tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
	if fromStart || !stream {
		seek.Whence = 0
		reopen = false
	}
	return tail.TailFile(r.config.EventsLogFilePath, tail.Config{ReOpen: reopen, Follow: stream, Location: &seek, Logger: tail.DiscardingLogger})
}
