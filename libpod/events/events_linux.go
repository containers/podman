package events

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// NewEventer creates an eventer based on the eventer type
func NewEventer(options EventerOptions) (Eventer, error) {
	logrus.Debugf("Initializing event backend %s", options.EventerType)
	switch EventerType(strings.ToLower(options.EventerType)) {
	case Journald:
		return newJournalDEventer(options)
	case LogFile:
		return newLogFileEventer(options)
	case Null:
		return newNullEventer(), nil
	default:
		return nil, fmt.Errorf("unknown event logger type: %s", strings.ToLower(options.EventerType))
	}
}
