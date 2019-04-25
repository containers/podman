package events

// EventToNull is an eventer type that only performs write operations
// and only writes to /dev/null. It is meant for unittests only
type EventToNull struct{}

// Write eats the event and always returns nil
func (e EventToNull) Write(ee Event) error {
	return nil
}

// Read does nothing. Do not use it.
func (e EventToNull) Read(options ReadOptions) error {
	return nil
}

// NewNullEventer returns a new null eventer.  You should only do this for
// the purposes on internal libpod testing.
func NewNullEventer() Eventer {
	var e Eventer
	e = EventToNull{}
	return e
}
