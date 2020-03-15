// +build !linux

package events

import "github.com/pkg/errors"

// NewEventer creates an eventer based on the eventer type
func NewEventer(options EventerOptions) (Eventer, error) {
	return nil, errors.New("this function is not available for your platform")
}
