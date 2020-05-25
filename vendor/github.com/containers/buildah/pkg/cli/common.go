package cli

// the cli package contains urfave/cli related structs that help make up
// the command line for buildah commands. it resides here so other projects
// that vendor in this code can use them too.

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/util"
	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/config"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

// LayerResults represents the results of the layer flags
type LayerResults struct {
	ForceRm bool
	Layers  bool
}

// UserNSResults represents the results for the UserNS flags
type UserNSResults struct {
	UserNS            string
	UserNSUIDMap      []string
	UserNSGIDMap      []string
	UserNSUIDMapUser  string
	UserNSGIDMapGroup string
}

// NameSpaceResults represents the results for Namespace flags
type NameSpaceResults struct {
	IPC           string
	Network       string
	CNIConfigDir  string
	CNIPlugInPath string
	PID           string
	UTS           string
}

// BudResults represents the results for Bud flags
type BudResults struct {
	Annotation          []string
	Arch                string
	Authfile            string
	BuildArg            []string
	CacheFrom           string
	CertDir             string
	Compress            bool
	Creds               string
	DisableCompression  bool
	DisableContentTrust bool
	DecryptionKeys      []string
	File                []string
	Format              string
	Iidfile             string
	Label               []string
	Logfile             string
	Loglevel            int
	NoCache             bool
	OS                  string
	Platform            string
	Pull                bool
	PullAlways          bool
	PullNever           bool
	Quiet               bool
	Rm                  bool
	Runtime             string
	RuntimeFlags        []string
	SignaturePolicy     string
	SignBy              string
	Squash              bool
	Tag                 []string
	Target              string
	TLSVerify           bool
}

// FromAndBugResults represents the results for common flags
// in bud and from
type FromAndBudResults struct {
	AddHost      []string
	BlobCache    string
	CapAdd       []string
	CapDrop      []string
	CgroupParent string
	CPUPeriod    uint64
	CPUQuota     int64
	CPUSetCPUs   string
	CPUSetMems   string
	CPUShares    uint64
	Devices      []string
	DNSSearch    []string
	DNSServers   []string
	DNSOptions   []string
	HTTPProxy    bool
	Isolation    string
	Memory       string
	MemorySwap   string
	OverrideArch string
	OverrideOS   string
	SecurityOpt  []string
	ShmSize      string
	Ulimit       []string
	Volumes      []string
}

// GetUserNSFlags returns the common flags for usernamespace
func GetUserNSFlags(flags *UserNSResults) pflag.FlagSet {
	usernsFlags := pflag.FlagSet{}
	usernsFlags.StringVar(&flags.UserNS, "userns", "", "'container', `path` of user namespace to join, or 'host'")
	usernsFlags.StringSliceVar(&flags.UserNSUIDMap, "userns-uid-map", []string{}, "`containerID:hostID:length` UID mapping to use in user namespace")
	usernsFlags.StringSliceVar(&flags.UserNSGIDMap, "userns-gid-map", []string{}, "`containerID:hostID:length` GID mapping to use in user namespace")
	usernsFlags.StringVar(&flags.UserNSUIDMapUser, "userns-uid-map-user", "", "`name` of entries from /etc/subuid to use to set user namespace UID mapping")
	usernsFlags.StringVar(&flags.UserNSGIDMapGroup, "userns-gid-map-group", "", "`name` of entries from /etc/subgid to use to set user namespace GID mapping")
	return usernsFlags
}

// GetNameSpaceFlags returns the common flags for a namespace menu
func GetNameSpaceFlags(flags *NameSpaceResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.StringVar(&flags.IPC, string(specs.IPCNamespace), "", "'container', `path` of IPC namespace to join, or 'host'")
	fs.StringVar(&flags.Network, string(specs.NetworkNamespace), "", "'container', `path` of network namespace to join, or 'host'")
	// TODO How do we alias net and network?
	fs.StringVar(&flags.Network, "net", "", "'container', `path` of network namespace to join, or 'host'")
	if err := fs.MarkHidden("net"); err != nil {
		panic(fmt.Sprintf("error marking net flag as hidden: %v", err))
	}
	fs.StringVar(&flags.CNIConfigDir, "cni-config-dir", util.DefaultCNIConfigDir, "`directory` of CNI configuration files")
	fs.StringVar(&flags.CNIPlugInPath, "cni-plugin-path", util.DefaultCNIPluginPath, "`path` of CNI network plugins")
	fs.StringVar(&flags.PID, string(specs.PIDNamespace), "", "container, `path` of PID namespace to join, or 'host'")
	fs.StringVar(&flags.UTS, string(specs.UTSNamespace), "", "container, :`path` of UTS namespace to join, or 'host'")
	return fs
}

// GetLayerFlags returns the common flags for layers
func GetLayerFlags(flags *LayerResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.BoolVar(&flags.ForceRm, "force-rm", false, "Always remove intermediate containers after a build, even if the build is unsuccessful.")
	fs.BoolVar(&flags.Layers, "layers", UseLayers(), fmt.Sprintf("cache intermediate layers during build. Use BUILDAH_LAYERS environment variable to override."))
	return fs
}

// GetBudFlags returns common bud flags
func GetBudFlags(flags *BudResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.StringVar(&flags.Arch, "arch", runtime.GOARCH, "set the ARCH of the image to the provided value instead of the architecture of the host")
	fs.StringArrayVar(&flags.Annotation, "annotation", []string{}, "Set metadata for an image (default [])")
	fs.StringVar(&flags.Authfile, "authfile", auth.GetDefaultAuthFile(), "path of the authentication file.")
	fs.StringArrayVar(&flags.BuildArg, "build-arg", []string{}, "`argument=value` to supply to the builder")
	fs.StringVar(&flags.CacheFrom, "cache-from", "", "Images to utilise as potential cache sources. The build process does not currently support caching so this is a NOOP.")
	fs.StringVar(&flags.CertDir, "cert-dir", "", "use certificates at the specified path to access the registry")
	fs.BoolVar(&flags.Compress, "compress", false, "This is legacy option, which has no effect on the image")
	fs.StringVar(&flags.Creds, "creds", "", "use `[username[:password]]` for accessing the registry")
	fs.BoolVarP(&flags.DisableCompression, "disable-compression", "D", true, "don't compress layers by default")
	fs.BoolVar(&flags.DisableContentTrust, "disable-content-trust", false, "This is a Docker specific option and is a NOOP")
	fs.StringSliceVarP(&flags.File, "file", "f", []string{}, "`pathname or URL` of a Dockerfile")
	fs.StringVar(&flags.Format, "format", DefaultFormat(), "`format` of the built image's manifest and metadata. Use BUILDAH_FORMAT environment variable to override.")
	fs.StringVar(&flags.Iidfile, "iidfile", "", "`file` to write the image ID to")
	fs.StringArrayVar(&flags.Label, "label", []string{}, "Set metadata for an image (default [])")
	fs.BoolVar(&flags.NoCache, "no-cache", false, "Do not use existing cached images for the container build. Build from the start with a new set of cached layers.")
	fs.StringVar(&flags.Logfile, "logfile", "", "log to `file` instead of stdout/stderr")
	fs.IntVar(&flags.Loglevel, "loglevel", 0, "adjust logging level (range from -2 to 3)")
	fs.StringVar(&flags.OS, "os", runtime.GOOS, "set the OS to the provided value instead of the current operating system of the host")
	fs.StringVar(&flags.Platform, "platform", parse.DefaultPlatform(), "set the OS/ARCH to the provided value instead of the current operating system and architecture of the host (for example `linux/arm`)")
	fs.BoolVar(&flags.Pull, "pull", true, "pull the image from the registry if newer or not present in store, if false, only pull the image if not present")
	fs.BoolVar(&flags.PullAlways, "pull-always", false, "pull the image even if the named image is present in store")
	fs.BoolVar(&flags.PullNever, "pull-never", false, "do not pull the image, use the image present in store if available")
	fs.BoolVarP(&flags.Quiet, "quiet", "q", false, "refrain from announcing build instructions and image read/write progress")
	fs.BoolVar(&flags.Rm, "rm", true, "Remove intermediate containers after a successful build")
	// "runtime" definition moved to avoid name collision in podman build.  Defined in cmd/buildah/bud.go.
	fs.StringSliceVar(&flags.RuntimeFlags, "runtime-flag", []string{}, "add global flags for the container runtime")
	fs.StringVar(&flags.SignBy, "sign-by", "", "sign the image using a GPG key with the specified `FINGERPRINT`")
	fs.StringVar(&flags.SignaturePolicy, "signature-policy", "", "`pathname` of signature policy file (not usually used)")
	fs.BoolVar(&flags.Squash, "squash", false, "squash newly built layers into a single new layer")
	fs.StringArrayVarP(&flags.Tag, "tag", "t", []string{}, "tagged `name` to apply to the built image")
	fs.StringVar(&flags.Target, "target", "", "set the target build stage to build")
	fs.BoolVar(&flags.TLSVerify, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
	return fs
}

func GetFromAndBudFlags(flags *FromAndBudResults, usernsResults *UserNSResults, namespaceResults *NameSpaceResults) (pflag.FlagSet, error) {
	fs := pflag.FlagSet{}
	defaultContainerConfig, err := config.Default()
	if err != nil {
		return fs, errors.Wrapf(err, "failed to get container config")
	}

	fs.StringSliceVar(&flags.AddHost, "add-host", []string{}, "add a custom host-to-IP mapping (`host:ip`) (default [])")
	fs.StringVar(&flags.BlobCache, "blob-cache", "", "assume image blobs in the specified directory will be available for pushing")
	if err := fs.MarkHidden("blob-cache"); err != nil {
		panic(fmt.Sprintf("error marking net flag as hidden: %v", err))
	}
	fs.StringSliceVar(&flags.CapAdd, "cap-add", []string{}, "add the specified capability when running (default [])")
	fs.StringSliceVar(&flags.CapDrop, "cap-drop", []string{}, "drop the specified capability when running (default [])")
	fs.StringVar(&flags.CgroupParent, "cgroup-parent", "", "optional parent cgroup for the container")
	fs.Uint64Var(&flags.CPUPeriod, "cpu-period", 0, "limit the CPU CFS (Completely Fair Scheduler) period")
	fs.Int64Var(&flags.CPUQuota, "cpu-quota", 0, "limit the CPU CFS (Completely Fair Scheduler) quota")
	fs.Uint64VarP(&flags.CPUShares, "cpu-shares", "c", 0, "CPU shares (relative weight)")
	fs.StringVar(&flags.CPUSetCPUs, "cpuset-cpus", "", "CPUs in which to allow execution (0-3, 0,1)")
	fs.StringVar(&flags.CPUSetMems, "cpuset-mems", "", "memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.")
	fs.StringArrayVar(&flags.Devices, "device", defaultContainerConfig.Containers.Devices, "Additional devices to be used within containers (default [])")
	fs.StringSliceVar(&flags.DNSSearch, "dns-search", defaultContainerConfig.Containers.DNSSearches, "Set custom DNS search domains")
	fs.StringSliceVar(&flags.DNSServers, "dns", defaultContainerConfig.Containers.DNSServers, "Set custom DNS servers or disable it completely by setting it to 'none', which prevents the automatic creation of `/etc/resolv.conf`.")
	fs.StringSliceVar(&flags.DNSOptions, "dns-option", defaultContainerConfig.Containers.DNSOptions, "Set custom DNS options")
	fs.BoolVar(&flags.HTTPProxy, "http-proxy", true, "pass through HTTP Proxy environment variables")
	fs.StringVar(&flags.Isolation, "isolation", DefaultIsolation(), "`type` of process isolation to use. Use BUILDAH_ISOLATION environment variable to override.")
	fs.StringVarP(&flags.Memory, "memory", "m", "", "memory limit (format: <number>[<unit>], where unit = b, k, m or g)")
	fs.StringVar(&flags.MemorySwap, "memory-swap", "", "swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	fs.StringVar(&flags.OverrideOS, "override-os", runtime.GOOS, "prefer `OS` instead of the running OS when pulling images")
	if err := fs.MarkHidden("override-os"); err != nil {
		panic(fmt.Sprintf("error marking override-os as hidden: %v", err))
	}
	fs.StringVar(&flags.OverrideArch, "override-arch", runtime.GOARCH, "prefer `ARCH` instead of the architecture of the machine when pulling images")
	if err := fs.MarkHidden("override-arch"); err != nil {
		panic(fmt.Sprintf("error marking override-arch as hidden: %v", err))
	}
	fs.StringArrayVar(&flags.SecurityOpt, "security-opt", []string{}, "security options (default [])")
	fs.StringVar(&flags.ShmSize, "shm-size", defaultContainerConfig.Containers.ShmSize, "size of '/dev/shm'. The format is `<number><unit>`.")
	fs.StringSliceVar(&flags.Ulimit, "ulimit", defaultContainerConfig.Containers.DefaultUlimits, "ulimit options")
	fs.StringArrayVarP(&flags.Volumes, "volume", "v", defaultContainerConfig.Containers.Volumes, "bind mount a volume into the container")

	// Add in the usernamespace and namespaceflags
	usernsFlags := GetUserNSFlags(usernsResults)
	namespaceFlags := GetNameSpaceFlags(namespaceResults)
	fs.AddFlagSet(&usernsFlags)
	fs.AddFlagSet(&namespaceFlags)

	return fs, nil
}

// UseLayers returns true if BUILDAH_LAYERS is set to "1" or "true"
// otherwise it returns false
func UseLayers() bool {
	layers := os.Getenv("BUILDAH_LAYERS")
	if strings.ToLower(layers) == "true" || layers == "1" {
		return true
	}
	return false
}

// DefaultFormat returns the default image format
func DefaultFormat() string {
	format := os.Getenv("BUILDAH_FORMAT")
	if format != "" {
		return format
	}
	return buildah.OCI
}

// DefaultIsolation returns the default image format
func DefaultIsolation() string {
	isolation := os.Getenv("BUILDAH_ISOLATION")
	if isolation != "" {
		return isolation
	}
	return buildah.OCI
}

// DefaultHistory returns the default add-history setting
func DefaultHistory() bool {
	history := os.Getenv("BUILDAH_HISTORY")
	if strings.ToLower(history) == "true" || history == "1" {
		return true
	}
	return false
}

func VerifyFlagsArgsOrder(args []string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return errors.Errorf("No options (%s) can be specified after the image or container name", arg)
		}
	}
	return nil
}
