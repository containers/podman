// +build !windows

package config

// Defaults for linux/unix if none are specified
const (
	cniConfigDir = "/etc/cni/net.d/"
)

var cniBinDir = []string{
	"/usr/libexec/cni",
	"/usr/lib/cni",
	"/usr/local/lib/cni",
	"/opt/cni/bin",
}
