package version

import "fmt"

const (
	// VersionMajor is for an API incompatible changes
	VersionMajor = 5
	// VersionMinor is for functionality in a backwards-compatible manner
	VersionMinor = 10
	// VersionPatch is for backwards-compatible bug fixes
	VersionPatch = 5

	// VersionDev indicates development branch. Releases will be empty string.
	VersionDev = ""
)

// Version is the specification version that the package types support.
var Version = fmt.Sprintf("%d.%d.%d%s", VersionMajor, VersionMinor, VersionPatch, VersionDev)
