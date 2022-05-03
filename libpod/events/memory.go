package events

import (
	"context"
)

// EventMemory is the structure for event writing to a channel. It contains the eventer
// options and the event itself.  Methods for reading and writing are also defined from it.
type EventMemory struct {
	options  EventerOptions
	elements chan *Event
}

// Write event to memory queue
func (e EventMemory) Write(event Event) (err error) {
	e.elements <- &event
	return
}

// Read event(s) from memory queue
func (e EventMemory) Read(ctx context.Context, options ReadOptions) (err error) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	select {
	case event := <-e.elements:
		options.EventChannel <- event
	default:
	}
	return nil
}

// String returns eventer type
func (e EventMemory) String() string {
	return e.options.EventerType
}

// NewMemoryEventer returns configured MemoryEventer
func NewMemoryEventer() Eventer {
	return EventMemory{
		options: EventerOptions{
			EventerType: Memory.String(),
		},
		elements: make(chan *Event, 100),
	}
}
