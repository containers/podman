// +build !systemd

package events

// newEventJournalD always returns an error if libsystemd not found
func newEventJournalD(options EventerOptions) (Eventer, error) {
	return nil, ErrNoJournaldLogging
}
