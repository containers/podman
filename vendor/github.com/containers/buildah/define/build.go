package define

import (
	"io"
	"time"

	"github.com/containers/image/v5/types"
	encconfig "github.com/containers/ocicrypt/config"
	"github.com/containers/storage/pkg/archive"
	"golang.org/x/sync/semaphore"
)

// CommonBuildOptions are resources that can be defined by flags for both buildah from and build
type CommonBuildOptions struct {
	// AddHost is the list of hostnames to add to the build container's /etc/hosts.
	AddHost []string
	// CgroupParent is the path to cgroups under which the cgroup for the container will be created.
	CgroupParent string
	// CPUPeriod limits the CPU CFS (Completely Fair Scheduler) period
	CPUPeriod uint64
	// CPUQuota limits the CPU CFS (Completely Fair Scheduler) quota
	CPUQuota int64
	// CPUShares (relative weight
	CPUShares uint64
	// CPUSetCPUs in which to allow execution (0-3, 0,1)
	CPUSetCPUs string
	// CPUSetMems memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.
	CPUSetMems string
	// HTTPProxy determines whether *_proxy env vars from the build host are passed into the container.
	HTTPProxy bool
	// Memory is the upper limit (in bytes) on how much memory running containers can use.
	Memory int64
	// DNSSearch is the list of DNS search domains to add to the build container's /etc/resolv.conf
	DNSSearch []string
	// DNSServers is the list of DNS servers to add to the build container's /etc/resolv.conf
	DNSServers []string
	// DNSOptions is the list of DNS
	DNSOptions []string
	// MemorySwap limits the amount of memory and swap together.
	MemorySwap int64
	// LabelOpts is the a slice of fields of an SELinux context, given in "field:pair" format, or "disable".
	// Recognized field names are "role", "type", and "level".
	LabelOpts []string
	// OmitTimestamp forces epoch 0 as created timestamp to allow for
	// deterministic, content-addressable builds.
	OmitTimestamp bool
	// SeccompProfilePath is the pathname of a seccomp profile.
	SeccompProfilePath string
	// ApparmorProfile is the name of an apparmor profile.
	ApparmorProfile string
	// ShmSize is the "size" value to use when mounting an shmfs on the container's /dev/shm directory.
	ShmSize string
	// Ulimit specifies resource limit options, in the form type:softlimit[:hardlimit].
	// These types are recognized:
	// "core": maximum core dump size (ulimit -c)
	// "cpu": maximum CPU time (ulimit -t)
	// "data": maximum size of a process's data segment (ulimit -d)
	// "fsize": maximum size of new files (ulimit -f)
	// "locks": maximum number of file locks (ulimit -x)
	// "memlock": maximum amount of locked memory (ulimit -l)
	// "msgqueue": maximum amount of data in message queues (ulimit -q)
	// "nice": niceness adjustment (nice -n, ulimit -e)
	// "nofile": maximum number of open files (ulimit -n)
	// "nproc": maximum number of processes (ulimit -u)
	// "rss": maximum size of a process's (ulimit -m)
	// "rtprio": maximum real-time scheduling priority (ulimit -r)
	// "rttime": maximum amount of real-time execution between blocking syscalls
	// "sigpending": maximum number of pending signals (ulimit -i)
	// "stack": maximum stack size (ulimit -s)
	Ulimit []string
	// Volumes to bind mount into the container
	Volumes []string
	// Secrets are the available secrets to use in a build
	Secrets []string
	// SSHSources is the available ssh agent connections to forward in the build
	SSHSources []string
}

// BuildOptions can be used to alter how an image is built.
type BuildOptions struct {
	// ContextDirectory is the default source location for COPY and ADD
	// commands.
	ContextDirectory string
	// PullPolicy controls whether or not we pull images.  It should be one
	// of PullIfMissing, PullAlways, PullIfNewer, or PullNever.
	PullPolicy PullPolicy
	// Registry is a value which is prepended to the image's name, if it
	// needs to be pulled and the image name alone can not be resolved to a
	// reference to a source image.  No separator is implicitly added.
	Registry string
	// IgnoreUnrecognizedInstructions tells us to just log instructions we
	// don't recognize, and try to keep going.
	IgnoreUnrecognizedInstructions bool
	// Manifest Name to which the image will be added.
	Manifest string
	// Quiet tells us whether or not to announce steps as we go through them.
	Quiet bool
	// Isolation controls how Run() runs things.
	Isolation Isolation
	// Runtime is the name of the command to run for RUN instructions when
	// Isolation is either IsolationDefault or IsolationOCI.  It should
	// accept the same arguments and flags that runc does.
	Runtime string
	// RuntimeArgs adds global arguments for the runtime.
	RuntimeArgs []string
	// TransientMounts is a list of mounts that won't be kept in the image.
	TransientMounts []string
	// Compression specifies the type of compression which is applied to
	// layer blobs.  The default is to not use compression, but
	// archive.Gzip is recommended.
	Compression archive.Compression
	// Arguments which can be interpolated into Dockerfiles
	Args map[string]string
	// Name of the image to write to.
	Output string
	// Additional tags to add to the image that we write, if we know of a
	// way to add them.
	AdditionalTags []string
	// Log is a callback that will print a progress message.  If no value
	// is supplied, the message will be sent to Err (or os.Stderr, if Err
	// is nil) by default.
	Log func(format string, args ...interface{})
	// In is connected to stdin for RUN instructions.
	In io.Reader
	// Out is a place where non-error log messages are sent.
	Out io.Writer
	// Err is a place where error log messages should be sent.
	Err io.Writer
	// SignaturePolicyPath specifies an override location for the signature
	// policy which should be used for verifying the new image as it is
	// being written.  Except in specific circumstances, no value should be
	// specified, indicating that the shared, system-wide default policy
	// should be used.
	SignaturePolicyPath string
	// ReportWriter is an io.Writer which will be used to report the
	// progress of the (possible) pulling of the source image and the
	// writing of the new image.
	ReportWriter io.Writer
	// OutputFormat is the format of the output image's manifest and
	// configuration data.
	// Accepted values are buildah.OCIv1ImageManifest and buildah.Dockerv2ImageManifest.
	OutputFormat string
	// SystemContext holds parameters used for authentication.
	SystemContext *types.SystemContext
	// NamespaceOptions controls how we set up namespaces processes that we
	// might need when handling RUN instructions.
	NamespaceOptions []NamespaceOption
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
	// ID mapping options to use if we're setting up our own user namespace
	// when handling RUN instructions.
	IDMappingOptions *IDMappingOptions
	// AddCapabilities is a list of capabilities to add to the default set when
	// handling RUN instructions.
	AddCapabilities []string
	// DropCapabilities is a list of capabilities to remove from the default set
	// when handling RUN instructions. If a capability appears in both lists, it
	// will be dropped.
	DropCapabilities []string
	// CommonBuildOpts is *required*.
	CommonBuildOpts *CommonBuildOptions
	// DefaultMountsFilePath is the file path holding the mounts to be mounted in "host-path:container-path" format
	DefaultMountsFilePath string
	// IIDFile tells the builder to write the image ID to the specified file
	IIDFile string
	// Squash tells the builder to produce an image with a single layer
	// instead of with possibly more than one layer.
	Squash bool
	// Labels metadata for an image
	Labels []string
	// Annotation metadata for an image
	Annotations []string
	// OnBuild commands to be run by images based on this image
	OnBuild []string
	// Layers tells the builder to create a cache of images for each step in the Dockerfile
	Layers bool
	// NoCache tells the builder to build the image from scratch without checking for a cache.
	// It creates a new set of cached images for the build.
	NoCache bool
	// RemoveIntermediateCtrs tells the builder whether to remove intermediate containers used
	// during the build process. Default is true.
	RemoveIntermediateCtrs bool
	// ForceRmIntermediateCtrs tells the builder to remove all intermediate containers even if
	// the build was unsuccessful.
	ForceRmIntermediateCtrs bool
	// BlobDirectory is a directory which we'll use for caching layer blobs.
	BlobDirectory string
	// Target the targeted FROM in the Dockerfile to build.
	Target string
	// Devices are the additional devices to add to the containers.
	Devices []string
	// SignBy is the fingerprint of a GPG key to use for signing images.
	SignBy string
	// Architecture specifies the target architecture of the image to be built.
	Architecture string
	// Timestamp sets the created timestamp to the specified time, allowing
	// for deterministic, content-addressable builds.
	Timestamp *time.Time
	// OS is the specifies the operating system of the image to be built.
	OS string
	// MaxPullPushRetries is the maximum number of attempts we'll make to pull or push any one
	// image from or to an external registry if the first attempt fails.
	MaxPullPushRetries int
	// PullPushRetryDelay is how long to wait before retrying a pull or push attempt.
	PullPushRetryDelay time.Duration
	// OciDecryptConfig contains the config that can be used to decrypt an image if it is
	// encrypted if non-nil. If nil, it does not attempt to decrypt an image.
	OciDecryptConfig *encconfig.DecryptConfig
	// Jobs is the number of stages to run in parallel.  If not specified it defaults to 1.
	// Ignored if a JobSemaphore is provided.
	Jobs *int
	// JobSemaphore, for when you want Jobs to be shared with more than just this build.
	JobSemaphore *semaphore.Weighted
	// LogRusage logs resource usage for each step.
	LogRusage bool
	// File to which the Rusage logs will be saved to instead of stdout
	RusageLogFile string
	// Excludes is a list of excludes to be used instead of the .dockerignore file.
	Excludes []string
	// From is the image name to use to replace the value specified in the first
	// FROM instruction in the Containerfile
	From string
	// Platforms is the list of parsed OS/Arch/Variant triples that we want
	// to build the image for.  If this slice has items in it, the OS and
	// Architecture fields above are ignored.
	Platforms []struct{ OS, Arch, Variant string }
}
