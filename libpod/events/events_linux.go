package events

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// NewEventer creates an eventer based on the eventer type
func NewEventer(options EventerOptions) (Eventer, error) {
	logrus.Debugf("Initializing event backend %s", options.EventerType)
	switch strings.ToUpper(options.EventerType) {
	case strings.ToUpper(Journald.String()):
		eventer, err := newEventJournalD(options)
		if err != nil {
			return nil, fmt.Errorf("eventer creation: %w", err)
		}
		return eventer, nil
	case strings.ToUpper(LogFile.String()):
		return newLogFileEventer(options)
	case strings.ToUpper(Null.String()):
		return newNullEventer(), nil
	case strings.ToUpper(Memory.String()):
		return NewMemoryEventer(), nil
	default:
		return nil, fmt.Errorf("unknown event logger type: %s", strings.ToUpper(options.EventerType))
	}
}
