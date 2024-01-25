package entities

import (
	"strconv"
	"time"

	libpodEvents "github.com/containers/podman/v4/libpod/events"
	types "github.com/containers/podman/v4/pkg/domain/entities/types"
	dockerEvents "github.com/docker/docker/api/types/events"
)

type Event = types.Event

// ConvertToLibpodEvent converts an entities event to a libpod one.
func ConvertToLibpodEvent(e Event) *libpodEvents.Event {
	var exitCode int
	if ec, ok := e.Actor.Attributes["containerExitCode"]; ok {
		var err error
		exitCode, err = strconv.Atoi(ec)
		if err != nil {
			return nil
		}
	}
	status, err := libpodEvents.StringToStatus(string(e.Action))
	if err != nil {
		return nil
	}
	t, err := libpodEvents.StringToType(string(e.Type))
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
		ContainerExitCode: &exitCode,
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
func ConvertToEntitiesEvent(e libpodEvents.Event) *types.Event {
	attributes := e.Details.Attributes
	if attributes == nil {
		attributes = make(map[string]string)
	}
	attributes["image"] = e.Image
	attributes["name"] = e.Name
	if e.ContainerExitCode != nil {
		attributes["containerExitCode"] = strconv.Itoa(*e.ContainerExitCode)
	}
	attributes["podId"] = e.PodID
	message := dockerEvents.Message{
		// Compatibility with clients that still look for deprecated API elements
		Status: e.Status.String(),
		ID:     e.ID,
		From:   e.Image,
		Type:   dockerEvents.Type(e.Type.String()),
		Action: dockerEvents.Action(e.Status.String()),
		Actor: dockerEvents.Actor{
			ID:         e.ID,
			Attributes: attributes,
		},
		Scope:    "local",
		Time:     e.Time.Unix(),
		TimeNano: e.Time.UnixNano(),
	}
	return &types.Event{
		Message:      message,
		HealthStatus: e.HealthStatus,
	}
}
