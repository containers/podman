// +build !systemd

package events

// DefaultEventerType is logfile when systemd is not present
const DefaultEventerType = LogFile

// newEventJournalD always returns an error if libsystemd not found
func newEventJournalD(options EventerOptions) (Eventer, error) {
	return nil, ErrNoJournaldLogging
}
