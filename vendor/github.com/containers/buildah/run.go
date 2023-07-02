package buildah

import (
	"fmt"
	"io"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/internal"
	"github.com/containers/buildah/pkg/sshagent"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const (
	// runUsingRuntimeCommand is a command we use as a key for reexec
	runUsingRuntimeCommand = define.Package + "-oci-runtime"
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

// String converts a TerminalPolicy into a string.
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
type NamespaceOption = define.NamespaceOption

// NamespaceOptions provides some helper methods for a slice of NamespaceOption
// structs.
type NamespaceOptions = define.NamespaceOptions

// IDMappingOptions controls how we set up UID/GID mapping when we set up a
// user namespace.
type IDMappingOptions = define.IDMappingOptions

// Isolation provides a way to specify whether we're supposed to use a proper
// OCI runtime, or some other method for running commands.
type Isolation = define.Isolation

const (
	// IsolationDefault is whatever we think will work best.
	IsolationDefault = define.IsolationDefault
	// IsolationOCI is a proper OCI runtime.
	IsolationOCI = define.IsolationOCI
	// IsolationChroot is a more chroot-like environment: less isolation,
	// but with fewer requirements.
	IsolationChroot = define.IsolationChroot
	// IsolationOCIRootless is a proper OCI runtime in rootless mode.
	IsolationOCIRootless = define.IsolationOCIRootless
)

// RunOptions can be used to alter how a command is run in the container.
type RunOptions struct {
	// Logger is the logrus logger to write log messages with
	Logger *logrus.Logger `json:"-"`
	// Hostname is the hostname we set for the running container.
	Hostname string
	// Isolation is either IsolationDefault, IsolationOCI, IsolationChroot, or IsolationOCIRootless.
	Isolation define.Isolation
	// Runtime is the name of the runtime to run.  It should accept the
	// same arguments that runc does, and produce similar output.
	Runtime string
	// Args adds global arguments for the runtime.
	Args []string
	// NoHosts use the images /etc/hosts file
	NoHosts bool
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
	// ContextDir is used as the root directory for the source location for mounts that are of type "bind".
	ContextDir string
	// Shell is default shell to run in a container.
	Shell string
	// Cmd is an override for the configured default command.
	Cmd []string
	// Entrypoint is an override for the configured entry point.
	Entrypoint []string
	// NamespaceOptions controls how we set up the namespaces for the process.
	NamespaceOptions define.NamespaceOptions
	// ConfigureNetwork controls whether or not network interfaces and
	// routing are configured for a new network namespace (i.e., when not
	// joining another's namespace and not just using the host's
	// namespace), effectively deciding whether or not the process has a
	// usable network.
	ConfigureNetwork define.NetworkConfigurationPolicy
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
	// Devices are the additional devices to add to the containers
	Devices define.ContainerDevices
	// Secrets are the available secrets to use in a RUN
	Secrets map[string]define.Secret
	// SSHSources is the available ssh agents to use in a RUN
	SSHSources map[string]*sshagent.Source `json:"-"`
	// RunMounts are mounts for this run. RunMounts for this run
	// will not show up in subsequent runs.
	RunMounts []string
	// Map of stages and container mountpoint if any from stage executor
	StageMountPoints map[string]internal.StageMountDetails
	// External Image mounts to be cleaned up.
	// Buildah run --mount could mount image before RUN calls, RUN could cleanup
	// them up as well
	ExternalImageMounts []string
	// System context of current build
	SystemContext *types.SystemContext
	// CgroupManager to use for running OCI containers
	CgroupManager string
}

// RunMountArtifacts are the artifacts created when using a run mount.
type runMountArtifacts struct {
	// RunMountTargets are the run mount targets inside the container
	RunMountTargets []string
	// TmpFiles are artifacts that need to be removed outside the container
	TmpFiles []string
	// Any external images which were mounted inside container
	MountedImages []string
	// Agents are the ssh agents started
	Agents []*sshagent.AgentServer
	// SSHAuthSock is the path to the ssh auth sock inside the container
	SSHAuthSock string
	// TargetLocks to be unlocked if there are any.
	TargetLocks []*lockfile.LockFile
}

// RunMountInfo are the available run mounts for this run
type runMountInfo struct {
	// WorkDir is the current working directory inside the container.
	WorkDir string
	// ContextDir is the root directory for the source location for bind mounts.
	ContextDir string
	// Secrets are the available secrets to use in a RUN
	Secrets map[string]define.Secret
	// SSHSources is the available ssh agents to use in a RUN
	SSHSources map[string]*sshagent.Source `json:"-"`
	// Map of stages and container mountpoint if any from stage executor
	StageMountPoints map[string]internal.StageMountDetails
	// System context of current build
	SystemContext *types.SystemContext
}

// IDMaps are the UIDs, GID, and maps for the run
type IDMaps struct {
	uidmap     []specs.LinuxIDMapping
	gidmap     []specs.LinuxIDMapping
	rootUID    int
	rootGID    int
	processUID int
	processGID int
}
