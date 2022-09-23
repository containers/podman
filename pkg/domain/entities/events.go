package entities

import (
	"strconv"
	"time"

	libpodEvents "github.com/containers/podman/v4/libpod/events"
	dockerEvents "github.com/docker/docker/api/types/events"
)

// Event combines various event-related data such as time, event type, status
// and more.
type Event struct {
	// TODO: it would be nice to have full control over the types at some
	// point and fork such Docker types.
	dockerEvents.Message
	HealthStatus string `json:",omitempty"`
}

// ConvertToLibpodEvent converts an entities event to a libpod one.
func ConvertToLibpodEvent(e Event) *libpodEvents.Event {
	exitCode, err := strconv.Atoi(e.Actor.Attributes["containerExitCode"])
	if err != nil {
		return nil
	}
	status, err := libpodEvents.StringToStatus(e.Action)
	if err != nil {
		return nil
	}
	t, err := libpodEvents.StringToType(e.Type)
	if err != nil {
		return nil
	}
	image := e.Actor.Attributes["image"]
	name := e.Actor.Attributes["name"]
	details := e.Actor.Attributes
	podID := e.Actor.Attributes["podId"]
	delete(details, "image")
	delete(details, "name")
	delete(details, "containerExitCode")
	return &libpodEvents.Event{
		ContainerExitCode: exitCode,
		ID:                e.Actor.ID,
		Image:             image,
		Name:              name,
		Status:            status,
		Time:              time.Unix(0, e.TimeNano),
		Type:              t,
		HealthStatus:      e.HealthStatus,
		Details: libpodEvents.Details{
			PodID:      podID,
			Attributes: details,
		},
	}
}

// ConvertToEntitiesEvent converts a libpod event to an entities one.
func ConvertToEntitiesEvent(e libpodEvents.Event) *Event {
	attributes := e.Details.Attributes
	if attributes == nil {
		attributes = make(map[string]string)
	}
	attributes["image"] = e.Image
	attributes["name"] = e.Name
	attributes["containerExitCode"] = strconv.Itoa(e.ContainerExitCode)
	attributes["podId"] = e.PodID
	message := dockerEvents.Message{
		// Compatibility with clients that still look for deprecated API elements
		Status: e.Status.String(),
		ID:     e.ID,
		From:   e.Image,
		Type:   e.Type.String(),
		Action: e.Status.String(),
		Actor: dockerEvents.Actor{
			ID:         e.ID,
			Attributes: attributes,
		},
		Scope:    "local",
		Time:     e.Time.Unix(),
		TimeNano: e.Time.UnixNano(),
	}
	return &Event{
		message,
		e.HealthStatus,
	}
}
