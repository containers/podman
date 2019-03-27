// +build varlink

package varlinkapi

import (
	"fmt"
	"time"

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
	if len(since) > 0 || len(until) > 0 {
		fromStart = true
	}
	eventChannel := make(chan *events.Event)
	go func() {
		readOpts := events.ReadOptions{FromStart: fromStart, Stream: stream, Filters: filter, EventChannel: eventChannel}
		eventsError = i.Runtime.Events(readOpts)
	}()
	if eventsError != nil {
		return call.ReplyErrorOccurred(eventsError.Error())
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
