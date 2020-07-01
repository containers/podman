package events

import (
	"context"
)

// EventToNull is an eventer type that only performs write operations
// and only writes to /dev/null. It is meant for unittests only
type EventToNull struct{}

// Write eats the event and always returns nil
func (e EventToNull) Write(ee Event) error {
	return nil
}

// Read does nothing. Do not use it.
func (e EventToNull) Read(ctx context.Context, options ReadOptions) error {
	return nil
}

// NewNullEventer returns a new null eventer.  You should only do this for
// the purposes on internal libpod testing.
func NewNullEventer() Eventer {
	return EventToNull{}
}

// String returns a string representation of the logger
func (e EventToNull) String() string {
	return "none"
}
