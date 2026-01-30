package config

const (
	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/etc/containers/policy.json"
)

var defaultHelperBinariesDir = []string{
	// Relative to the binary directory
	"$BINDIR/../libexec/podman",
	// Homebrew install paths
	"/usr/local/opt/podman/libexec/podman",
	"/opt/homebrew/opt/podman/libexec/podman",
	"/opt/homebrew/bin",
	"/usr/local/bin",
	// default paths
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
	"/usr/libexec/podman",
	"/usr/lib/podman",
}
