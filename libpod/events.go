//go:build !remote
// +build !remote

package libpod

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/containers/podman/v4/libpod/events"
	"github.com/sirupsen/logrus"
)

// newEventer returns an eventer that can be used to read/write events
func (r *Runtime) newEventer() (events.Eventer, error) {
	if r.config.Engine.EventsLogFilePath == "" {
		// default, use path under tmpdir when none was explicitly set by the user
		r.config.Engine.EventsLogFilePath = filepath.Join(r.config.Engine.TmpDir, "events", "events.log")
	}
	options := events.EventerOptions{
		EventerType:    r.config.Engine.EventsLogger,
		LogFilePath:    r.config.Engine.EventsLogFilePath,
		LogFileMaxSize: r.config.Engine.EventsLogMaxSize(),
	}
	return events.NewEventer(options)
}

// newContainerEvent creates a new event based on a container
func (c *Container) newContainerEvent(status events.Status) {
	if err := c.newContainerEventWithInspectData(status, false); err != nil {
		logrus.Errorf("Unable to write container event: %v", err)
	}
}

// newContainerEventWithInspectData creates a new event and sets the
// ContainerInspectData field if inspectData is set.
func (c *Container) newContainerEventWithInspectData(status events.Status, inspectData bool) error {
	e := events.NewEvent(status)
	e.ID = c.ID()
	e.Name = c.Name()
	e.Image = c.config.RootfsImageName
	e.Type = events.Container

	e.Details = events.Details{
		ID:         e.ID,
		PodID:      c.PodID(),
		Attributes: c.Labels(),
	}

	if inspectData {
		err := func() error {
			data, err := c.inspectLocked(true)
			if err != nil {
				return err
			}
			rawData, err := json.Marshal(data)
			if err != nil {
				return err
			}
			e.Details.ContainerInspectData = string(rawData)
			return nil
		}()
		if err != nil {
			return fmt.Errorf("adding inspect data to container-create event: %v", err)
		}
	}

	// if the current event is a HealthStatus event, we need to get the current
	// status of the container to pass to the event
	if status == events.HealthStatus {
		containerHealthStatus, err := c.healthCheckStatus()
		if err != nil {
			e.HealthStatus = err.Error()
		}
		e.HealthStatus = containerHealthStatus
	}

	return c.runtime.eventer.Write(e)
}

// newContainerExitedEvent creates a new event for a container's death
func (c *Container) newContainerExitedEvent(exitCode int32) {
	e := events.NewEvent(events.Exited)
	e.ID = c.ID()
	e.Name = c.Name()
	e.Image = c.config.RootfsImageName
	e.Type = events.Container
	e.PodID = c.PodID()
	e.ContainerExitCode = int(exitCode)

	e.Details = events.Details{
		ID:         e.ID,
		Attributes: c.Labels(),
	}

	if err := c.runtime.eventer.Write(e); err != nil {
		logrus.Errorf("Unable to write container exited event: %q", err)
	}
}

// newExecDiedEvent creates a new event for an exec session's death
func (c *Container) newExecDiedEvent(sessionID string, exitCode int) {
	e := events.NewEvent(events.ExecDied)
	e.ID = c.ID()
	e.Name = c.Name()
	e.Image = c.config.RootfsImageName
	e.Type = events.Container
	e.ContainerExitCode = exitCode
	e.Attributes = make(map[string]string)
	e.Attributes["execID"] = sessionID

	e.Details = events.Details{
		ID:         e.ID,
		Attributes: c.Labels(),
	}

	if err := c.runtime.eventer.Write(e); err != nil {
		logrus.Errorf("Unable to write exec died event: %q", err)
	}
}

// netNetworkEvent creates a new event based on a network connect/disconnect
func (c *Container) newNetworkEvent(status events.Status, netName string) {
	e := events.NewEvent(status)
	e.ID = c.ID()
	e.Name = c.Name()
	e.Type = events.Network
	e.Network = netName
	if err := c.runtime.eventer.Write(e); err != nil {
		logrus.Errorf("Unable to write pod event: %q", err)
	}
}

// newPodEvent creates a new event for a libpod pod
func (p *Pod) newPodEvent(status events.Status) {
	e := events.NewEvent(status)
	e.ID = p.ID()
	e.Name = p.Name()
	e.Type = events.Pod
	if err := p.runtime.eventer.Write(e); err != nil {
		logrus.Errorf("Unable to write pod event: %q", err)
	}
}

// NewSystemEvent creates a new event for libpod as a whole.
func (r *Runtime) NewSystemEvent(status events.Status) {
	e := events.NewEvent(status)
	e.Type = events.System

	if err := r.eventer.Write(e); err != nil {
		logrus.Errorf("Unable to write system event: %q", err)
	}
}

// newVolumeEvent creates a new event for a libpod volume
func (v *Volume) newVolumeEvent(status events.Status) {
	e := events.NewEvent(status)
	e.Name = v.Name()
	e.Type = events.Volume
	if err := v.runtime.eventer.Write(e); err != nil {
		logrus.Errorf("Unable to write volume event: %q", err)
	}
}

// Events is a wrapper function for everyone to begin tailing the events log
// with options
func (r *Runtime) Events(ctx context.Context, options events.ReadOptions) error {
	return r.eventer.Read(ctx, options)
}

// GetEvents reads the event log and returns events based on input filters
func (r *Runtime) GetEvents(ctx context.Context, filters []string) ([]*events.Event, error) {
	eventChannel := make(chan *events.Event)
	options := events.ReadOptions{
		EventChannel: eventChannel,
		Filters:      filters,
		FromStart:    true,
		Stream:       false,
	}

	logEvents := make([]*events.Event, 0, len(eventChannel))
	readLock := sync.Mutex{}
	readLock.Lock()
	go func() {
		for e := range eventChannel {
			logEvents = append(logEvents, e)
		}
		readLock.Unlock()
	}()

	readErr := r.eventer.Read(ctx, options)
	readLock.Lock() // Wait for the events to be consumed.
	return logEvents, readErr
}

// GetLastContainerEvent takes a container name or ID and an event status and returns
// the last occurrence of the container event
func (r *Runtime) GetLastContainerEvent(ctx context.Context, nameOrID string, containerEvent events.Status) (*events.Event, error) {
	// FIXME: events should be read in reverse order!
	// https://github.com/containers/podman/issues/14579

	// check to make sure the event.Status is valid
	if _, err := events.StringToStatus(containerEvent.String()); err != nil {
		return nil, err
	}
	filters := []string{
		fmt.Sprintf("container=%s", nameOrID),
		fmt.Sprintf("event=%s", containerEvent),
		"type=container",
	}
	containerEvents, err := r.GetEvents(ctx, filters)
	if err != nil {
		return nil, err
	}
	if len(containerEvents) < 1 {
		return nil, fmt.Errorf("%s not found: %w", containerEvent.String(), events.ErrEventNotFound)
	}
	// return the last element in the slice
	return containerEvents[len(containerEvents)-1], nil
}

// GetExecDiedEvent takes a container name or ID, exec session ID, and returns
// that exec session's Died event (if it has already occurred).
func (r *Runtime) GetExecDiedEvent(ctx context.Context, nameOrID, execSessionID string) (*events.Event, error) {
	filters := []string{
		fmt.Sprintf("container=%s", nameOrID),
		"event=exec_died",
		"type=container",
		fmt.Sprintf("label=execID=%s", execSessionID),
	}

	containerEvents, err := r.GetEvents(ctx, filters)
	if err != nil {
		return nil, err
	}
	// There *should* only be one event maximum.
	// But... just in case... let's not blow up if there's more than one.
	if len(containerEvents) < 1 {
		return nil, fmt.Errorf("exec died event for session %s (container %s) not found: %w", execSessionID, nameOrID, events.ErrEventNotFound)
	}
	return containerEvents[len(containerEvents)-1], nil
}
