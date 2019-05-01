package buildah

import (
	"fmt"
	"io"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	// runUsingRuntimeCommand is a command we use as a key for reexec
	runUsingRuntimeCommand = Package + "-oci-runtime"
)

// TerminalPolicy takes the value DefaultTerminal, WithoutTerminal, or WithTerminal.
type TerminalPolicy int

const (
	// DefaultTerminal indicates that this Run invocation should be
	// connected to a pseudoterminal if we're connected to a terminal.
	DefaultTerminal TerminalPolicy = iota
	// WithoutTerminal indicates that this Run invocation should NOT be
	// connected to a pseudoterminal.
	WithoutTerminal
	// WithTerminal indicates that this Run invocation should be connected
	// to a pseudoterminal.
	WithTerminal
)

// String converts a TerminalPoliicy into a string.
func (t TerminalPolicy) String() string {
	switch t {
	case DefaultTerminal:
		return "DefaultTerminal"
	case WithoutTerminal:
		return "WithoutTerminal"
	case WithTerminal:
		return "WithTerminal"
	}
	return fmt.Sprintf("unrecognized terminal setting %d", t)
}

// NamespaceOption controls how we set up a namespace when launching processes.
type NamespaceOption struct {
	// Name specifies the type of namespace, typically matching one of the
	// ...Namespace constants defined in
	// github.com/opencontainers/runtime-spec/specs-go.
	Name string
	// Host is used to force our processes to use the host's namespace of
	// this type.
	Host bool
	// Path is the path of the namespace to attach our process to, if Host
	// is not set.  If Host is not set and Path is also empty, a new
	// namespace will be created for the process that we're starting.
	// If Name is specs.NetworkNamespace, if Path doesn't look like an
	// absolute path, it is treated as a comma-separated list of CNI
	// configuration names which will be selected from among all of the CNI
	// network configurations which we find.
	Path string
}

// NamespaceOptions provides some helper methods for a slice of NamespaceOption
// structs.
type NamespaceOptions []NamespaceOption

// IDMappingOptions controls how we set up UID/GID mapping when we set up a
// user namespace.
type IDMappingOptions struct {
	HostUIDMapping bool
	HostGIDMapping bool
	UIDMap         []specs.LinuxIDMapping
	GIDMap         []specs.LinuxIDMapping
}

// Isolation provides a way to specify whether we're supposed to use a proper
// OCI runtime, or some other method for running commands.
type Isolation int

const (
	// IsolationDefault is whatever we think will work best.
	IsolationDefault Isolation = iota
	// IsolationOCI is a proper OCI runtime.
	IsolationOCI
	// IsolationChroot is a more chroot-like environment: less isolation,
	// but with fewer requirements.
	IsolationChroot
	// IsolationOCIRootless is a proper OCI runtime in rootless mode.
	IsolationOCIRootless
)

// String converts a Isolation into a string.
func (i Isolation) String() string {
	switch i {
	case IsolationDefault:
		return "IsolationDefault"
	case IsolationOCI:
		return "IsolationOCI"
	case IsolationChroot:
		return "IsolationChroot"
	case IsolationOCIRootless:
		return "IsolationOCIRootless"
	}
	return fmt.Sprintf("unrecognized isolation type %d", i)
}

// RunOptions can be used to alter how a command is run in the container.
type RunOptions struct {
	// Hostname is the hostname we set for the running container.
	Hostname string
	// Isolation is either IsolationDefault, IsolationOCI, IsolationChroot, or IsolationOCIRootless.
	Isolation Isolation
	// Runtime is the name of the runtime to run.  It should accept the
	// same arguments that runc does, and produce similar output.
	Runtime string
	// Args adds global arguments for the runtime.
	Args []string
	// NoPivot adds the --no-pivot runtime flag.
	NoPivot bool
	// Mounts are additional mount points which we want to provide.
	Mounts []specs.Mount
	// Env is additional environment variables to set.
	Env []string
	// User is the user as whom to run the command.
	User string
	// WorkingDir is an override for the working directory.
	WorkingDir string
	// Shell is default shell to run in a container.
	Shell string
	// Cmd is an override for the configured default command.
	Cmd []string
	// Entrypoint is an override for the configured entry point.
	Entrypoint []string
	// NamespaceOptions controls how we set up the namespaces for the process.
	NamespaceOptions NamespaceOptions
	// ConfigureNetwork controls whether or not network interfaces and
	// routing are configured for a new network namespace (i.e., when not
	// joining another's namespace and not just using the host's
	// namespace), effectively deciding whether or not the process has a
	// usable network.
	ConfigureNetwork NetworkConfigurationPolicy
	// CNIPluginPath is the location of CNI plugin helpers, if they should be
	// run from a location other than the default location.
	CNIPluginPath string
	// CNIConfigDir is the location of CNI configuration files, if the files in
	// the default configuration directory shouldn't be used.
	CNIConfigDir string
	// Terminal provides a way to specify whether or not the command should
	// be run with a pseudoterminal.  By default (DefaultTerminal), a
	// terminal is used if os.Stdout is connected to a terminal, but that
	// decision can be overridden by specifying either WithTerminal or
	// WithoutTerminal.
	Terminal TerminalPolicy
	// TerminalSize provides a way to set the number of rows and columns in
	// a pseudo-terminal, if we create one, and Stdin/Stdout/Stderr aren't
	// connected to a terminal.
	TerminalSize *specs.Box
	// The stdin/stdout/stderr descriptors to use.  If set to nil, the
	// corresponding files in the "os" package are used as defaults.
	Stdin  io.Reader `json:"-"`
	Stdout io.Writer `json:"-"`
	Stderr io.Writer `json:"-"`
	// Quiet tells the run to turn off output to stdout.
	Quiet bool
	// AddCapabilities is a list of capabilities to add to the default set.
	AddCapabilities []string
	// DropCapabilities is a list of capabilities to remove from the default set,
	// after processing the AddCapabilities set.  If a capability appears in both
	// lists, it will be dropped.
	DropCapabilities []string
}

// Find the configuration for the namespace of the given type.  If there are
// duplicates, find the _last_ one of the type, since we assume it was appended
// more recently.
func (n *NamespaceOptions) Find(namespace string) *NamespaceOption {
	for i := range *n {
		j := len(*n) - 1 - i
		if (*n)[j].Name == namespace {
			return &((*n)[j])
		}
	}
	return nil
}

// AddOrReplace either adds or replaces the configuration for a given namespace.
func (n *NamespaceOptions) AddOrReplace(options ...NamespaceOption) {
nextOption:
	for _, option := range options {
		for i := range *n {
			j := len(*n) - 1 - i
			if (*n)[j].Name == option.Name {
				(*n)[j] = option
				continue nextOption
			}
		}
		*n = append(*n, option)
	}
}
