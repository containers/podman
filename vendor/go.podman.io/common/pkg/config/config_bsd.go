//go:build freebsd || netbsd || openbsd

package config

const (
	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/usr/local/etc/containers/policy.json"
)

var defaultHelperBinariesDir = []string{
	"/usr/local/bin",
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
}
