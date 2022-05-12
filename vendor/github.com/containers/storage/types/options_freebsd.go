package types

const (
	// these are default path for run and graph root for rootful users
	// for rootless path is constructed via getRootlessStorageOpts
	defaultRunRoot   string = "/var/run/containers/storage"
	defaultGraphRoot string = "/var/db/containers/storage"
)

// defaultConfigFile path to the system wide storage.conf file
var (
	defaultConfigFile         = "/usr/local/share/containers/storage.conf"
	defaultOverrideConfigFile = "/usr/local/etc/containers/storage.conf"
	defaultConfigFileSet      = false
	// DefaultStoreOptions is a reasonable default set of options.
	defaultStoreOptions StoreOptions
)
