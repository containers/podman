package cli

// the cli package contains urfave/cli related structs that help make up
// the command line for buildah commands. it resides here so other projects
// that vendor in this code can use them too.

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/pkg/completion"
	"github.com/containers/buildah/pkg/parse"
	commonComp "github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/storage/pkg/unshare"
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

// BudResults represents the results for Build flags
type BudResults struct {
	Annotation          []string
	Authfile            string
	BuildArg            []string
	CacheFrom           string
	CertDir             string
	Compress            bool
	Creds               string
	DisableCompression  bool
	DisableContentTrust bool
	IgnoreFile          string
	File                []string
	Format              string
	From                string
	Iidfile             string
	Label               []string
	Logfile             string
	Manifest            string
	NoCache             bool
	Timestamp           int64
	Pull                bool
	PullAlways          bool
	PullNever           bool
	Quiet               bool
	Rm                  bool
	Runtime             string
	RuntimeFlags        []string
	Secrets             []string
	SSH                 []string
	SignaturePolicy     string
	SignBy              string
	Squash              bool
	Stdin               bool
	Tag                 []string
	Target              string
	TLSVerify           bool
	Jobs                int
	LogRusage           bool
	RusageLogFile       string
}

// FromAndBugResults represents the results for common flags
// in build and from
type FromAndBudResults struct {
	AddHost        []string
	BlobCache      string
	CapAdd         []string
	CapDrop        []string
	CgroupParent   string
	CPUPeriod      uint64
	CPUQuota       int64
	CPUSetCPUs     string
	CPUSetMems     string
	CPUShares      uint64
	DecryptionKeys []string
	Devices        []string
	DNSSearch      []string
	DNSServers     []string
	DNSOptions     []string
	HTTPProxy      bool
	Isolation      string
	Memory         string
	MemorySwap     string
	SecurityOpt    []string
	ShmSize        string
	Ulimit         []string
	Volumes        []string
}

// GetUserNSFlags returns the common flags for usernamespace
func GetUserNSFlags(flags *UserNSResults) pflag.FlagSet {
	usernsFlags := pflag.FlagSet{}
	usernsFlags.StringVar(&flags.UserNS, "userns", "", "'container', `path` of user namespace to join, or 'host'")
	usernsFlags.StringSliceVar(&flags.UserNSUIDMap, "userns-uid-map", []string{}, "`containerUID:hostUID:length` UID mapping to use in user namespace")
	usernsFlags.StringSliceVar(&flags.UserNSGIDMap, "userns-gid-map", []string{}, "`containerGID:hostGID:length` GID mapping to use in user namespace")
	usernsFlags.StringVar(&flags.UserNSUIDMapUser, "userns-uid-map-user", "", "`name` of entries from /etc/subuid to use to set user namespace UID mapping")
	usernsFlags.StringVar(&flags.UserNSGIDMapGroup, "userns-gid-map-group", "", "`name` of entries from /etc/subgid to use to set user namespace GID mapping")
	return usernsFlags
}

// GetUserNSFlagsCompletions returns the FlagCompletions for the userns flags
func GetUserNSFlagsCompletions() commonComp.FlagCompletions {
	flagCompletion := commonComp.FlagCompletions{}
	flagCompletion["userns"] = completion.AutocompleteNamespaceFlag
	flagCompletion["userns-uid-map"] = commonComp.AutocompleteNone
	flagCompletion["userns-gid-map"] = commonComp.AutocompleteNone
	flagCompletion["userns-uid-map-user"] = commonComp.AutocompleteSubuidName
	flagCompletion["userns-gid-map-group"] = commonComp.AutocompleteSubgidName
	return flagCompletion
}

// GetNameSpaceFlags returns the common flags for a namespace menu
func GetNameSpaceFlags(flags *NameSpaceResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.StringVar(&flags.IPC, string(specs.IPCNamespace), "", "'private', `path` of IPC namespace to join, or 'host'")
	fs.StringVar(&flags.Network, string(specs.NetworkNamespace), "", "'private', 'none', 'ns:path' of network namespace to join, or 'host'")
	fs.StringVar(&flags.CNIConfigDir, "cni-config-dir", define.DefaultCNIConfigDir, "`directory` of CNI configuration files")
	fs.StringVar(&flags.CNIPlugInPath, "cni-plugin-path", define.DefaultCNIPluginPath, "`path` of CNI network plugins")
	fs.StringVar(&flags.PID, string(specs.PIDNamespace), "", "private, `path` of PID namespace to join, or 'host'")
	fs.StringVar(&flags.UTS, string(specs.UTSNamespace), "", "private, :`path` of UTS namespace to join, or 'host'")
	return fs
}

// GetNameSpaceFlagsCompletions returns the FlagCompletions for the namespace flags
func GetNameSpaceFlagsCompletions() commonComp.FlagCompletions {
	flagCompletion := commonComp.FlagCompletions{}
	flagCompletion[string(specs.IPCNamespace)] = completion.AutocompleteNamespaceFlag
	flagCompletion[string(specs.NetworkNamespace)] = completion.AutocompleteNamespaceFlag
	flagCompletion["cni-config-dir"] = commonComp.AutocompleteDefault
	flagCompletion["cni-plugin-path"] = commonComp.AutocompleteDefault
	flagCompletion[string(specs.PIDNamespace)] = completion.AutocompleteNamespaceFlag
	flagCompletion[string(specs.UTSNamespace)] = completion.AutocompleteNamespaceFlag
	return flagCompletion
}

// GetLayerFlags returns the common flags for layers
func GetLayerFlags(flags *LayerResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.BoolVar(&flags.ForceRm, "force-rm", false, "Always remove intermediate containers after a build, even if the build is unsuccessful.")
	fs.BoolVar(&flags.Layers, "layers", UseLayers(), fmt.Sprintf("cache intermediate layers during build. Use BUILDAH_LAYERS environment variable to override."))
	return fs
}

// Note: GetLayerFlagsCompletion is not needed since GetLayerFlags only contains bool flags

// GetBudFlags returns common build flags
func GetBudFlags(flags *BudResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.String("arch", runtime.GOARCH, "set the ARCH of the image to the provided value instead of the architecture of the host")
	fs.StringArrayVar(&flags.Annotation, "annotation", []string{}, "Set metadata for an image (default [])")
	fs.StringVar(&flags.Authfile, "authfile", "", "path of the authentication file.")
	fs.StringArrayVar(&flags.BuildArg, "build-arg", []string{}, "`argument=value` to supply to the builder")
	fs.StringVar(&flags.CacheFrom, "cache-from", "", "Images to utilise as potential cache sources. The build process does not currently support caching so this is a NOOP.")
	fs.StringVar(&flags.CertDir, "cert-dir", "", "use certificates at the specified path to access the registry")
	fs.BoolVar(&flags.Compress, "compress", false, "This is legacy option, which has no effect on the image")
	fs.StringVar(&flags.Creds, "creds", "", "use `[username[:password]]` for accessing the registry")
	fs.BoolVarP(&flags.DisableCompression, "disable-compression", "D", true, "don't compress layers by default")
	fs.BoolVar(&flags.DisableContentTrust, "disable-content-trust", false, "This is a Docker specific option and is a NOOP")
	fs.StringVar(&flags.From, "from", "", "image name used to replace the value in the first FROM instruction in the Containerfile")
	fs.StringVar(&flags.IgnoreFile, "ignorefile", "", "path to an alternate .dockerignore file")
	fs.StringSliceVarP(&flags.File, "file", "f", []string{}, "`pathname or URL` of a Dockerfile")
	fs.StringVar(&flags.Format, "format", DefaultFormat(), "`format` of the built image's manifest and metadata. Use BUILDAH_FORMAT environment variable to override.")
	fs.StringVar(&flags.Iidfile, "iidfile", "", "`file` to write the image ID to")
	fs.IntVar(&flags.Jobs, "jobs", 1, "how many stages to run in parallel")
	fs.StringArrayVar(&flags.Label, "label", []string{}, "Set metadata for an image (default [])")
	fs.StringVar(&flags.Logfile, "logfile", "", "log to `file` instead of stdout/stderr")
	fs.Int("loglevel", 0, "NO LONGER USED, flag ignored, and hidden")
	if err := fs.MarkHidden("loglevel"); err != nil {
		panic(fmt.Sprintf("error marking the loglevel flag as hidden: %v", err))
	}
	fs.BoolVar(&flags.LogRusage, "log-rusage", false, "log resource usage at each build step")
	if err := fs.MarkHidden("log-rusage"); err != nil {
		panic(fmt.Sprintf("error marking the log-rusage flag as hidden: %v", err))
	}
	fs.StringVar(&flags.RusageLogFile, "rusage-logfile", "", "destination file to which rusage should be logged to instead of stdout (= the default).")
	if err := fs.MarkHidden("rusage-logfile"); err != nil {
		panic(fmt.Sprintf("error marking the rusage-logfile flag as hidden: %v", err))
	}
	fs.StringVar(&flags.Manifest, "manifest", "", "add the image to the specified manifest list. Creates manifest list if it does not exist")
	fs.BoolVar(&flags.NoCache, "no-cache", false, "Do not use existing cached images for the container build. Build from the start with a new set of cached layers.")
	fs.String("os", runtime.GOOS, "set the OS to the provided value instead of the current operating system of the host")
	fs.BoolVar(&flags.Pull, "pull", true, "pull the image from the registry if newer or not present in store, if false, only pull the image if not present")
	fs.BoolVar(&flags.PullAlways, "pull-always", false, "pull the image even if the named image is present in store")
	fs.BoolVar(&flags.PullNever, "pull-never", false, "do not pull the image, use the image present in store if available")
	fs.BoolVarP(&flags.Quiet, "quiet", "q", false, "refrain from announcing build instructions and image read/write progress")
	fs.BoolVar(&flags.Rm, "rm", true, "Remove intermediate containers after a successful build")
	// "runtime" definition moved to avoid name collision in podman build.  Defined in cmd/buildah/build.go.
	fs.StringSliceVar(&flags.RuntimeFlags, "runtime-flag", []string{}, "add global flags for the container runtime")
	fs.StringArrayVar(&flags.Secrets, "secret", []string{}, "secret file to expose to the build")
	fs.StringVar(&flags.SignBy, "sign-by", "", "sign the image using a GPG key with the specified `FINGERPRINT`")
	fs.StringVar(&flags.SignaturePolicy, "signature-policy", "", "`pathname` of signature policy file (not usually used)")
	if err := fs.MarkHidden("signature-policy"); err != nil {
		panic(fmt.Sprintf("error marking the signature-policy flag as hidden: %v", err))
	}
	fs.BoolVar(&flags.Squash, "squash", false, "squash newly built layers into a single new layer")
	fs.StringArrayVar(&flags.SSH, "ssh", []string{}, "SSH agent socket or keys to expose to the build. (format: default|<id>[=<socket>|<key>[,<key>]])")
	fs.BoolVar(&flags.Stdin, "stdin", false, "pass stdin into containers")
	fs.StringArrayVarP(&flags.Tag, "tag", "t", []string{}, "tagged `name` to apply to the built image")
	fs.StringVar(&flags.Target, "target", "", "set the target build stage to build")
	fs.Int64Var(&flags.Timestamp, "timestamp", 0, "set created timestamp to the specified epoch seconds to allow for deterministic builds, defaults to current time")
	fs.BoolVar(&flags.TLSVerify, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
	fs.String("variant", "", "override the `variant` of the specified image")
	return fs
}

// GetBudFlagsCompletions returns the FlagCompletions for the common build flags
func GetBudFlagsCompletions() commonComp.FlagCompletions {
	flagCompletion := commonComp.FlagCompletions{}
	flagCompletion["arch"] = commonComp.AutocompleteNone
	flagCompletion["annotation"] = commonComp.AutocompleteNone
	flagCompletion["authfile"] = commonComp.AutocompleteDefault
	flagCompletion["build-arg"] = commonComp.AutocompleteNone
	flagCompletion["cache-from"] = commonComp.AutocompleteNone
	flagCompletion["cert-dir"] = commonComp.AutocompleteDefault
	flagCompletion["creds"] = commonComp.AutocompleteNone
	flagCompletion["file"] = commonComp.AutocompleteDefault
	flagCompletion["from"] = commonComp.AutocompleteDefault
	flagCompletion["format"] = commonComp.AutocompleteNone
	flagCompletion["ignorefile"] = commonComp.AutocompleteDefault
	flagCompletion["iidfile"] = commonComp.AutocompleteDefault
	flagCompletion["jobs"] = commonComp.AutocompleteNone
	flagCompletion["label"] = commonComp.AutocompleteNone
	flagCompletion["logfile"] = commonComp.AutocompleteDefault
	flagCompletion["manifest"] = commonComp.AutocompleteDefault
	flagCompletion["os"] = commonComp.AutocompleteNone
	flagCompletion["runtime-flag"] = commonComp.AutocompleteNone
	flagCompletion["secret"] = commonComp.AutocompleteNone
	flagCompletion["ssh"] = commonComp.AutocompleteNone
	flagCompletion["sign-by"] = commonComp.AutocompleteNone
	flagCompletion["signature-policy"] = commonComp.AutocompleteNone
	flagCompletion["tag"] = commonComp.AutocompleteNone
	flagCompletion["target"] = commonComp.AutocompleteNone
	flagCompletion["timestamp"] = commonComp.AutocompleteNone
	flagCompletion["variant"] = commonComp.AutocompleteNone
	return flagCompletion
}

// GetFromAndBudFlags returns from and build flags
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
	fs.StringSliceVar(&flags.DecryptionKeys, "decryption-key", nil, "key needed to decrypt the image")
	fs.StringArrayVar(&flags.Devices, "device", defaultContainerConfig.Containers.Devices, "Additional devices to be used within containers (default [])")
	fs.StringSliceVar(&flags.DNSSearch, "dns-search", defaultContainerConfig.Containers.DNSSearches, "Set custom DNS search domains")
	fs.StringSliceVar(&flags.DNSServers, "dns", defaultContainerConfig.Containers.DNSServers, "Set custom DNS servers or disable it completely by setting it to 'none', which prevents the automatic creation of `/etc/resolv.conf`.")
	fs.StringSliceVar(&flags.DNSOptions, "dns-option", defaultContainerConfig.Containers.DNSOptions, "Set custom DNS options")
	fs.BoolVar(&flags.HTTPProxy, "http-proxy", true, "pass through HTTP Proxy environment variables")
	fs.StringVar(&flags.Isolation, "isolation", DefaultIsolation(), "`type` of process isolation to use. Use BUILDAH_ISOLATION environment variable to override.")
	fs.StringVarP(&flags.Memory, "memory", "m", "", "memory limit (format: <number>[<unit>], where unit = b, k, m or g)")
	fs.StringVar(&flags.MemorySwap, "memory-swap", "", "swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	fs.String("arch", runtime.GOARCH, "set the ARCH of the image to the provided value instead of the architecture of the host")
	fs.String("os", runtime.GOOS, "prefer `OS` instead of the running OS when pulling images")
	fs.StringSlice("platform", []string{parse.DefaultPlatform()}, "set the OS/ARCH/VARIANT of the image to the provided value instead of the current operating system and architecture of the host (for example `linux/arm`)")
	fs.String("variant", "", "override the `variant` of the specified image")
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

// GetFromAndBudFlagsCompletions returns the FlagCompletions for the from and build flags
func GetFromAndBudFlagsCompletions() commonComp.FlagCompletions {
	flagCompletion := commonComp.FlagCompletions{}
	flagCompletion["arch"] = commonComp.AutocompleteNone
	flagCompletion["add-host"] = commonComp.AutocompleteNone
	flagCompletion["blob-cache"] = commonComp.AutocompleteNone
	flagCompletion["cap-add"] = commonComp.AutocompleteCapabilities
	flagCompletion["cap-drop"] = commonComp.AutocompleteCapabilities
	flagCompletion["cgroup-parent"] = commonComp.AutocompleteDefault // FIXME: This would be a path right?!
	flagCompletion["cpu-period"] = commonComp.AutocompleteNone
	flagCompletion["cpu-quota"] = commonComp.AutocompleteNone
	flagCompletion["cpu-shares"] = commonComp.AutocompleteNone
	flagCompletion["cpuset-cpus"] = commonComp.AutocompleteNone
	flagCompletion["cpuset-mems"] = commonComp.AutocompleteNone
	flagCompletion["decryption-key"] = commonComp.AutocompleteNone
	flagCompletion["device"] = commonComp.AutocompleteDefault
	flagCompletion["dns-search"] = commonComp.AutocompleteNone
	flagCompletion["dns"] = commonComp.AutocompleteNone
	flagCompletion["dns-option"] = commonComp.AutocompleteNone
	flagCompletion["isolation"] = commonComp.AutocompleteNone
	flagCompletion["memory"] = commonComp.AutocompleteNone
	flagCompletion["memory-swap"] = commonComp.AutocompleteNone
	flagCompletion["os"] = commonComp.AutocompleteNone
	flagCompletion["platform"] = commonComp.AutocompleteNone
	flagCompletion["security-opt"] = commonComp.AutocompleteNone
	flagCompletion["shm-size"] = commonComp.AutocompleteNone
	flagCompletion["ulimit"] = commonComp.AutocompleteNone
	flagCompletion["volume"] = commonComp.AutocompleteDefault
	flagCompletion["variant"] = commonComp.AutocompleteNone

	// Add in the usernamespace and namespace flag completions
	userNsComp := GetUserNSFlagsCompletions()
	for name, comp := range userNsComp {
		flagCompletion[name] = comp
	}
	namespaceComp := GetNameSpaceFlagsCompletions()
	for name, comp := range namespaceComp {
		flagCompletion[name] = comp
	}

	return flagCompletion
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
	return define.OCI
}

// DefaultIsolation returns the default image format
func DefaultIsolation() string {
	isolation := os.Getenv("BUILDAH_ISOLATION")
	if isolation != "" {
		return isolation
	}
	if unshare.IsRootless() {
		return "rootless"
	}
	return define.OCI
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

// aliasFlags is a function to handle backwards compatibility with old flags
func AliasFlags(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "net":
		name = "network"
	case "override-arch":
		name = "arch"
	case "override-os":
		name = "os"
	case "purge":
		name = "rm"
	case "tty":
		name = "terminal"
	}
	return pflag.NormalizedName(name)
}
