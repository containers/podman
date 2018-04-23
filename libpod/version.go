package libpod

import (
	"runtime"
	"strconv"

	podmanVersion "github.com/projectatomic/libpod/version"
)

// Overwritten at build time
var (
	// GitCommit is the commit that the binary is being built from.
	// It will be populated by the Makefile.
	GitCommit string
	// BuildInfo is the time at which the binary was built
	// It will be populated by the Makefile.
	BuildInfo string
)

//Version is an output struct for varlink
type Version struct {
	Version   string
	GoVersion string
	GitCommit string
	Built     int64
	OsArch    string
}

// GetVersion returns a VersionOutput struct for varlink and podman
func GetVersion() (Version, error) {
	var err error
	var buildTime int64
	if BuildInfo != "" {
		// Converts unix time from string to int64
		buildTime, err = strconv.ParseInt(BuildInfo, 10, 64)

		if err != nil {
			return Version{}, err
		}
	}
	return Version{
		Version:   podmanVersion.Version,
		GoVersion: runtime.Version(),
		GitCommit: GitCommit,
		Built:     buildTime,
		OsArch:    runtime.GOOS + "/" + runtime.GOARCH,
	}, nil
}
