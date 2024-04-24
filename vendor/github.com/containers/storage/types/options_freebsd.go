package types

const (
	// these are default path for run and graph root for rootful users
	// for rootless path is constructed via getRootlessStorageOpts
	defaultRunRoot   string = "/var/run/containers/storage"
	defaultGraphRoot string = "/var/db/containers/storage"
	SystemConfigFile        = "/usr/local/share/containers/storage.conf"
)

// canUseRootlessOverlay returns true if the overlay driver can be used for rootless containers
func canUseRootlessOverlay(home, runhome string) bool {
	return false
}
