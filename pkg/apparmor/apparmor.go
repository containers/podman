package apparmor

import (
	"errors"
)

var (
	// DefaultLibpodProfile is the name of default libpod AppArmor profile.
	DefaultLibpodProfile = "libpod-default"
	// ErrApparmorUnsupported indicates that AppArmor support is not supported.
	ErrApparmorUnsupported = errors.New("AppArmor is not supported")
)
