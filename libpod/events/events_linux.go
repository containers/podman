package events

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NewEventer creates an eventer based on the eventer type
func NewEventer(options EventerOptions) (Eventer, error) {
	var eventer Eventer
	logrus.Debugf("Initializing event backend %s", options.EventerType)
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
