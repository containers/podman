package types

const (
	// these are default path for run and graph root for rootful users
	// for rootless path is constructed via getRootlessStorageOpts
	defaultRunRoot   string = "/run/containers/storage"
	defaultGraphRoot string = "/var/lib/containers/storage"
)

// defaultConfigFile path to the system wide storage.conf file
var (
	defaultConfigFile         = "/usr/share/containers/storage.conf"
	defaultOverrideConfigFile = "/etc/containers/storage.conf"
	defaultConfigFileSet      = false
	// DefaultStoreOptions is a reasonable default set of options.
	defaultStoreOptions StoreOptions
)
