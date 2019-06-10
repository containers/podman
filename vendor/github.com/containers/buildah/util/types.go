package util

const (
	// DefaultRuntime is the default command to use to run the container.
	DefaultRuntime = "runc"
	// DefaultCNIPluginPath is the default location of CNI plugin helpers.
	DefaultCNIPluginPath = "/usr/libexec/cni:/opt/cni/bin"
	// DefaultCNIConfigDir is the default location of CNI configuration files.
	DefaultCNIConfigDir = "/etc/cni/net.d"
)

var (
	// DefaultCapabilities is the list of capabilities which we grant by
	// default to containers which are running under UID 0.
	DefaultCapabilities = []string{
		"CAP_AUDIT_WRITE",
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_FOWNER",
		"CAP_FSETID",
		"CAP_KILL",
		"CAP_MKNOD",
		"CAP_NET_BIND_SERVICE",
		"CAP_SETFCAP",
		"CAP_SETGID",
		"CAP_SETPCAP",
		"CAP_SETUID",
		"CAP_SYS_CHROOT",
	}
	// DefaultNetworkSysctl is the list of Kernel parameters which we
	// grant by default to containers which are running under UID 0.
	DefaultNetworkSysctl = map[string]string{
		"net.ipv4.ping_group_range": "0 0",
	}
)
