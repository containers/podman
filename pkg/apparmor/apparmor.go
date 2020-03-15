package apparmor

import (
	"errors"

	libpodVersion "github.com/containers/libpod/version"
)

var (
	// DefaultLipodProfilePrefix is used for version-independent presence checks.
	DefaultLipodProfilePrefix = "libpod-default" + "-"
	// DefaultLibpodProfile is the name of default libpod AppArmor profile.
	DefaultLibpodProfile = DefaultLipodProfilePrefix + libpodVersion.Version
	// ErrApparmorUnsupported indicates that AppArmor support is not supported.
	ErrApparmorUnsupported = errors.New("AppArmor is not supported")
	// ErrApparmorRootless indicates that AppArmor support is not supported in rootless mode.
	ErrApparmorRootless = errors.New("AppArmor is not supported in rootless mode")
)
