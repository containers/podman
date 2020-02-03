// +build windows

package config

// Defaults for linux/unix if none are specified
const (
	cniConfigDir = "C:\\cni\\etc\\net.d\\"
)

var cniBinDir = []string{"C:\\cni\\bin\\"}
