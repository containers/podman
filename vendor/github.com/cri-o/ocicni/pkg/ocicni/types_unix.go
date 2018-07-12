// +build !windows

package ocicni

const (
	// DefaultConfDir is the default place to look for CNI Network
	DefaultConfDir = "/etc/cni/net.d"
	// DefaultBinDir is the default place to look for CNI config files
	DefaultBinDir = "/opt/cni/bin"
)
