//go:build !systemd

package events

// DefaultEventerType is logfile when systemd is not present
const DefaultEventerType = LogFile

// newJournalDEventer always returns an error if libsystemd not found
func newJournalDEventer(options EventerOptions) (Eventer, error) {
	return nil, ErrNoJournaldLogging
}
