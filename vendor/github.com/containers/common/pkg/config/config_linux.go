package config

import (
	"github.com/containers/common/pkg/capabilities"
	selinux "github.com/opencontainers/selinux/go-selinux"
)

const (
	// OverrideContainersConfig holds the default config path overridden by the root user
	OverrideContainersConfig = "/etc/" + _configPath

	// DefaultContainersConfig holds the default containers config path
	DefaultContainersConfig = "/usr/share/" + _configPath

	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/etc/containers/policy.json"
)

func selinuxEnabled() bool {
	return selinux.GetEnabled()
}

var defaultHelperBinariesDir = []string{
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
	"/usr/libexec/podman",
	"/usr/lib/podman",
}

// Capabilities returns the capabilities parses the Add and Drop capability
// list from the default capabilities for the container
func (c *Config) Capabilities(user string, addCapabilities, dropCapabilities []string) ([]string, error) {
	userNotRoot := func(user string) bool {
		if user == "" || user == "root" || user == "0" {
			return false
		}
		return true
	}

	defaultCapabilities := c.Containers.DefaultCapabilities.Get()
	if userNotRoot(user) {
		defaultCapabilities = []string{}
	}

	return capabilities.MergeCapabilities(defaultCapabilities, addCapabilities, dropCapabilities)
}
