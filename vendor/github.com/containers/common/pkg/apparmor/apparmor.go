package apparmor

import (
	"errors"
)

const (
	// ProfilePrefix is used for version-independent presence checks.
	ProfilePrefix = "apparmor_profile"

	// Profile default name
	Profile = "container-default"
)

var (

	// ErrApparmorUnsupported indicates that AppArmor support is not supported.
	ErrApparmorUnsupported = errors.New("AppArmor is not supported")
	// ErrApparmorRootless indicates that AppArmor support is not supported in rootless mode.
	ErrApparmorRootless = errors.New("AppArmor is not supported in rootless mode")
)
