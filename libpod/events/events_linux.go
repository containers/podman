package events

import (
	"github.com/pkg/errors"
	"strings"
)

// NewEventer creates an eventer based on the eventer type
func NewEventer(options EventerOptions) (Eventer, error) {
	var eventer Eventer
	switch strings.ToUpper(options.EventerType) {
	case strings.ToUpper(Journald.String()):
		eventer = EventJournalD{options}
	case strings.ToUpper(LogFile.String()):
		eventer = EventLogFile{options}
	default:
		return eventer, errors.Errorf("unknown event logger type: %s", strings.ToUpper(options.EventerType))
	}
	return eventer, nil
}
