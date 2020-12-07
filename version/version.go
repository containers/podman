package version

import (
	"github.com/blang/semver"
)

// Version is the version of the build.
// NOTE: remember to bump the version at the top
// of the top-level README.md file when this is
// bumped.
var Version = semver.MustParse("2.2.1")

// APIVersion is the version for the remote
// client API.  It is used to determine compatibility
// between a remote podman client and its backend
var APIVersion = semver.MustParse("2.1.0")
