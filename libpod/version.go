package libpod

import (
	"runtime"
	"strconv"

	podmanVersion "github.com/containers/libpod/version"
)

// Overwritten at build time
var (
	// GitCommit is the commit that the binary is being built from.
	// It will be populated by the Makefile.
	gitCommit string
	// BuildInfo is the time at which the binary was built
	// It will be populated by the Makefile.
	buildInfo string
	// InstallPrefix is the prefix where podman will be installed.
	// It will be populated by the Makefile.
	installPrefix string = "/usr/local"
	// EtcDir is the sysconfdir where podman should look for system config files.
	// It will be populated by the Makefile.
	etcDir string = "/etc"
)

//Version is an output struct for varlink
type Version struct {
	RemoteAPIVersion int64
	Version          string
	GoVersion        string
	GitCommit        string
	Built            int64
	OsArch           string
}

// GetVersion returns a VersionOutput struct for varlink and podman
func GetVersion() (Version, error) {
	var err error
	var buildTime int64
	if buildInfo != "" {
		// Converts unix time from string to int64
		buildTime, err = strconv.ParseInt(buildInfo, 10, 64)

		if err != nil {
			return Version{}, err
		}
	}
	return Version{
		RemoteAPIVersion: podmanVersion.RemoteAPIVersion,
		Version:          podmanVersion.Version,
		GoVersion:        runtime.Version(),
		GitCommit:        gitCommit,
		Built:            buildTime,
		OsArch:           runtime.GOOS + "/" + runtime.GOARCH,
	}, nil
}
