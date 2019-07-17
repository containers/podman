package events

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NewEventer creates an eventer based on the eventer type
func NewEventer(options EventerOptions) (eventer Eventer, err error) {
	logrus.Debugf("Initializing event backend %s", options.EventerType)
	switch strings.ToUpper(options.EventerType) {
	case strings.ToUpper(Journald.String()):
		eventer, err = newEventJournalD(options)
		if err != nil {
			return nil, errors.Wrapf(err, "eventer creation")
		}
	case strings.ToUpper(LogFile.String()):
		eventer = EventLogFile{options}
	case strings.ToUpper(Null.String()):
		eventer = NewNullEventer()
	default:
		return nil, errors.Errorf("unknown event logger type: %s", strings.ToUpper(options.EventerType))
	}
	return eventer, nil
}
