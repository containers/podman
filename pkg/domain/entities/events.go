package entities

import (
	"strconv"
	"time"

	libpodEvents "github.com/containers/podman/v2/libpod/events"
	dockerEvents "github.com/docker/docker/api/types/events"
)

// Event combines various event-related data such as time, event type, status
// and more.
type Event struct {
	// TODO: it would be nice to have full control over the types at some
	// point and fork such Docker types.
	dockerEvents.Message
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
	return &libpodEvents.Event{
		ContainerExitCode: exitCode,
		ID:                e.Actor.ID,
		Image:             e.Actor.Attributes["image"],
		Name:              e.Actor.Attributes["name"],
		Status:            status,
		Time:              time.Unix(e.Time, e.TimeNano),
		Type:              t,
	}
}

// ConvertToEntitiesEvent converts a libpod event to an entities one.
func ConvertToEntitiesEvent(e libpodEvents.Event) *Event {
	return &Event{dockerEvents.Message{
		Type:   e.Type.String(),
		Action: e.Status.String(),
		Actor: dockerEvents.Actor{
			ID: e.ID,
			Attributes: map[string]string{
				"image":             e.Image,
				"name":              e.Name,
				"containerExitCode": strconv.Itoa(e.ContainerExitCode),
			},
		},
		Scope:    "local",
		Time:     e.Time.Unix(),
		TimeNano: e.Time.UnixNano(),
	}}
}
