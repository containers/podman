// +build varlink

package varlinkapi

import (
	"fmt"
	"time"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod/events"
)

// GetEvents is a remote endpoint to get events from the event log
func (i *LibpodAPI) GetEvents(call iopodman.VarlinkCall, filter []string, since string, until string) error {
	var (
		fromStart   bool
		eventsError error
		event       *events.Event
		stream      bool
	)
	if call.WantsMore() {
		stream = true
		call.Continues = true
	}
	filters, err := shared.GenerateEventOptions(filter, since, until)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if len(since) > 0 || len(until) > 0 {
		fromStart = true
	}
	eventChannel := make(chan *events.Event)
	go func() {
		eventsError = i.Runtime.Events(fromStart, stream, filters, eventChannel)
	}()
	if eventsError != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	for {
		event = <-eventChannel
		if event == nil {
			call.Continues = false
			break
		}
		call.ReplyGetEvents(iopodman.Event{
			Id:     event.ID,
			Image:  event.Image,
			Name:   event.Name,
			Status: fmt.Sprintf("%s", event.Status),
			Time:   event.Time.Format(time.RFC3339Nano),
			Type:   fmt.Sprintf("%s", event.Type),
		})
		if !call.Continues {
			// For a one-shot on events, we break out here
			break
		}
	}
	return nil
}
