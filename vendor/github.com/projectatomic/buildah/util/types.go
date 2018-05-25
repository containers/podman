package util

const (
	// DefaultRuntime is the default command to use to run the container.
	DefaultRuntime = "runc"
	// DefaultCNIPluginPath is the default location of CNI plugin helpers.
	DefaultCNIPluginPath = "/usr/libexec/cni:/opt/cni/bin"
	// DefaultCNIConfigDir is the default location of CNI configuration files.
	DefaultCNIConfigDir = "/etc/cni/net.d"
)
