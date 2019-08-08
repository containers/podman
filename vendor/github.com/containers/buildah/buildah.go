package buildah

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/buildah/docker"
	"github.com/containers/buildah/util"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/ioutils"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// Package is the name of this package, used in help output and to
	// identify working containers.
	Package = "buildah"
	// Version for the Package.  Bump version in contrib/rpm/buildah.spec
	// too.
	Version = "1.10.1"
	// The value we use to identify what type of information, currently a
	// serialized Builder structure, we are using as per-container state.
	// This should only be changed when we make incompatible changes to
	// that data structure, as it's used to distinguish containers which
	// are "ours" from ones that aren't.
	containerType = Package + " 0.0.1"
	// The file in the per-container directory which we use to store our
	// per-container state.  If it isn't there, then the container isn't
	// one of our build containers.
	stateFile = Package + ".json"
)

// PullPolicy takes the value PullIfMissing, PullAlways, or PullNever.
type PullPolicy int

const (
	// PullIfMissing is one of the values that BuilderOptions.PullPolicy
	// can take, signalling that the source image should be pulled from a
	// registry if a local copy of it is not already present.
	PullIfMissing PullPolicy = iota
	// PullAlways is one of the values that BuilderOptions.PullPolicy can
	// take, signalling that a fresh, possibly updated, copy of the image
	// should be pulled from a registry before the build proceeds.
	PullAlways
	// PullNever is one of the values that BuilderOptions.PullPolicy can
	// take, signalling that the source image should not be pulled from a
	// registry if a local copy of it is not already present.
	PullNever
)

// String converts a PullPolicy into a string.
func (p PullPolicy) String() string {
	switch p {
	case PullIfMissing:
		return "PullIfMissing"
	case PullAlways:
		return "PullAlways"
	case PullNever:
		return "PullNever"
	}
	return fmt.Sprintf("unrecognized policy %d", p)
}

// NetworkConfigurationPolicy takes the value NetworkDefault, NetworkDisabled,
// or NetworkEnabled.
type NetworkConfigurationPolicy int

const (
	// NetworkDefault is one of the values that BuilderOptions.ConfigureNetwork
	// can take, signalling that the default behavior should be used.
	NetworkDefault NetworkConfigurationPolicy = iota
	// NetworkDisabled is one of the values that BuilderOptions.ConfigureNetwork
	// can take, signalling that network interfaces should NOT be configured for
	// newly-created network namespaces.
	NetworkDisabled
	// NetworkEnabled is one of the values that BuilderOptions.ConfigureNetwork
	// can take, signalling that network interfaces should be configured for
	// newly-created network namespaces.
	NetworkEnabled
)

// String formats a NetworkConfigurationPolicy as a string.
func (p NetworkConfigurationPolicy) String() string {
	switch p {
	case NetworkDefault:
		return "NetworkDefault"
	case NetworkDisabled:
		return "NetworkDisabled"
	case NetworkEnabled:
		return "NetworkEnabled"
	}
	return fmt.Sprintf("unknown NetworkConfigurationPolicy %d", p)
}

// Builder objects are used to represent containers which are being used to
// build images.  They also carry potential updates which will be applied to
// the image's configuration when the container's contents are used to build an
// image.
type Builder struct {
	store storage.Store

	// Args define variables that users can pass at build-time to the builder
	Args map[string]string
	// Type is used to help identify a build container's metadata.  It
	// should not be modified.
	Type string `json:"type"`
	// FromImage is the name of the source image which was used to create
	// the container, if one was used.  It should not be modified.
	FromImage string `json:"image,omitempty"`
	// FromImageID is the ID of the source image which was used to create
	// the container, if one was used.  It should not be modified.
	FromImageID string `json:"image-id"`
	// FromImageDigest is the digest of the source image which was used to
	// create the container, if one was used.  It should not be modified.
	FromImageDigest string `json:"image-digest"`
	// Config is the source image's configuration.  It should not be
	// modified.
	Config []byte `json:"config,omitempty"`
	// Manifest is the source image's manifest.  It should not be modified.
	Manifest []byte `json:"manifest,omitempty"`

	// Container is the name of the build container.  It should not be modified.
	Container string `json:"container-name,omitempty"`
	// ContainerID is the ID of the build container.  It should not be modified.
	ContainerID string `json:"container-id,omitempty"`
	// MountPoint is the last location where the container's root
	// filesystem was mounted.  It should not be modified.
	MountPoint string `json:"mountpoint,omitempty"`
	// ProcessLabel is the SELinux process label associated with the container
	ProcessLabel string `json:"process-label,omitempty"`
	// MountLabel is the SELinux mount label associated with the container
	MountLabel string `json:"mount-label,omitempty"`

	// ImageAnnotations is a set of key-value pairs which is stored in the
	// image's manifest.
	ImageAnnotations map[string]string `json:"annotations,omitempty"`
	// ImageCreatedBy is a description of how this container was built.
	ImageCreatedBy string `json:"created-by,omitempty"`
	// ImageHistoryComment is a description of how our added layers were built.
	ImageHistoryComment string `json:"history-comment,omitempty"`

	// Image metadata and runtime settings, in multiple formats.
	OCIv1  v1.Image       `json:"ociv1,omitempty"`
	Docker docker.V2Image `json:"docker,omitempty"`
	// DefaultMountsFilePath is the file path holding the mounts to be mounted in "host-path:container-path" format.
	DefaultMountsFilePath string `json:"defaultMountsFilePath,omitempty"`

	// Isolation controls how we handle "RUN" statements and the Run() method.
	Isolation Isolation
	// NamespaceOptions controls how we set up the namespaces for processes that we run in the container.
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
	// ID mapping options to use when running processes in the container with non-host user namespaces.
	IDMappingOptions IDMappingOptions
	// AddCapabilities is a list of capabilities to add to the default set when running
	// commands in the container.
	AddCapabilities []string
	// DropCapabilities is a list of capabilities to remove from the default set,
	// after processing the AddCapabilities set, when running commands in the container.
	// If a capability appears in both lists, it will be dropped.
	DropCapabilities []string
	// PrependedEmptyLayers are history entries that we'll add to a
	// committed image, after any history items that we inherit from a base
	// image, but before the history item for the layer that we're
	// committing.
	PrependedEmptyLayers []v1.History
	// AppendedEmptyLayers are history entries that we'll add to a
	// committed image after the history item for the layer that we're
	// committing.
	AppendedEmptyLayers []v1.History

	CommonBuildOpts *CommonBuildOptions
	// TopLayer is the top layer of the image
	TopLayer string
	// Format for the build Image
	Format string
	// TempVolumes are temporary mount points created during container runs
	TempVolumes map[string]bool
}

// BuilderInfo are used as objects to display container information
type BuilderInfo struct {
	Type                  string
	FromImage             string
	FromImageID           string
	FromImageDigest       string
	Config                string
	Manifest              string
	Container             string
	ContainerID           string
	MountPoint            string
	ProcessLabel          string
	MountLabel            string
	ImageAnnotations      map[string]string
	ImageCreatedBy        string
	OCIv1                 v1.Image
	Docker                docker.V2Image
	DefaultMountsFilePath string
	Isolation             string
	NamespaceOptions      NamespaceOptions
	ConfigureNetwork      string
	CNIPluginPath         string
	CNIConfigDir          string
	IDMappingOptions      IDMappingOptions
	DefaultCapabilities   []string
	AddCapabilities       []string
	DropCapabilities      []string
	History               []v1.History
}

// GetBuildInfo gets a pointer to a Builder object and returns a BuilderInfo object from it.
// This is used in the inspect command to display Manifest and Config as string and not []byte.
func GetBuildInfo(b *Builder) BuilderInfo {
	history := copyHistory(b.OCIv1.History)
	history = append(history, copyHistory(b.PrependedEmptyLayers)...)
	now := time.Now().UTC()
	created := &now
	history = append(history, v1.History{
		Created:    created,
		CreatedBy:  b.ImageCreatedBy,
		Author:     b.Maintainer(),
		Comment:    b.ImageHistoryComment,
		EmptyLayer: false,
	})
	history = append(history, copyHistory(b.AppendedEmptyLayers)...)
	return BuilderInfo{
		Type:                  b.Type,
		FromImage:             b.FromImage,
		FromImageID:           b.FromImageID,
		FromImageDigest:       b.FromImageDigest,
		Config:                string(b.Config),
		Manifest:              string(b.Manifest),
		Container:             b.Container,
		ContainerID:           b.ContainerID,
		MountPoint:            b.MountPoint,
		ProcessLabel:          b.ProcessLabel,
		MountLabel:            b.MountLabel,
		ImageAnnotations:      b.ImageAnnotations,
		ImageCreatedBy:        b.ImageCreatedBy,
		OCIv1:                 b.OCIv1,
		Docker:                b.Docker,
		DefaultMountsFilePath: b.DefaultMountsFilePath,
		Isolation:             b.Isolation.String(),
		NamespaceOptions:      b.NamespaceOptions,
		ConfigureNetwork:      fmt.Sprintf("%v", b.ConfigureNetwork),
		CNIPluginPath:         b.CNIPluginPath,
		CNIConfigDir:          b.CNIConfigDir,
		IDMappingOptions:      b.IDMappingOptions,
		DefaultCapabilities:   append([]string{}, util.DefaultCapabilities...),
		AddCapabilities:       append([]string{}, b.AddCapabilities...),
		DropCapabilities:      append([]string{}, b.DropCapabilities...),
		History:               history,
	}
}

// CommonBuildOptions are resources that can be defined by flags for both buildah from and build-using-dockerfile
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
	// SeccompProfilePath is the pathname of a seccomp profile.
	SeccompProfilePath string
	// ApparmorProfile is the name of an apparmor profile.
	ApparmorProfile string
	// ShmSize is the "size" value to use when mounting an shmfs on the container's /dev/shm directory.
	ShmSize string
	// Ulimit specifies resource limit options, in the form type:softlimit[:hardlimit].
	// These types are recognized:
	// "core": maximimum core dump size (ulimit -c)
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
}

// BuilderOptions are used to initialize a new Builder.
type BuilderOptions struct {
	// Args define variables that users can pass at build-time to the builder
	Args map[string]string
	// FromImage is the name of the image which should be used as the
	// starting point for the container.  It can be set to an empty value
	// or "scratch" to indicate that the container should not be based on
	// an image.
	FromImage string
	// Container is a desired name for the build container.
	Container string
	// PullPolicy decides whether or not we should pull the image that
	// we're using as a base image.  It should be PullIfMissing,
	// PullAlways, or PullNever.
	PullPolicy PullPolicy
	// Registry is a value which is prepended to the image's name, if it
	// needs to be pulled and the image name alone can not be resolved to a
	// reference to a source image.  No separator is implicitly added.
	Registry string
	// BlobDirectory is the name of a directory in which we'll attempt
	// to store copies of layer blobs that we pull down, if any.  It should
	// already exist.
	BlobDirectory string
	// Mount signals to NewBuilder() that the container should be mounted
	// immediately.
	Mount bool
	// SignaturePolicyPath specifies an override location for the signature
	// policy which should be used for verifying the new image as it is
	// being written.  Except in specific circumstances, no value should be
	// specified, indicating that the shared, system-wide default policy
	// should be used.
	SignaturePolicyPath string
	// ReportWriter is an io.Writer which will be used to log the reading
	// of the source image from a registry, if we end up pulling the image.
	ReportWriter io.Writer
	// github.com/containers/image/types SystemContext to hold credentials
	// and other authentication/authorization information.
	SystemContext *types.SystemContext
	// DefaultMountsFilePath is the file path holding the mounts to be
	// mounted in "host-path:container-path" format
	DefaultMountsFilePath string
	// Isolation controls how we handle "RUN" statements and the Run()
	// method.
	Isolation Isolation
	// NamespaceOptions controls how we set up namespaces for processes that
	// we might need to run using the container's root filesystem.
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
	// ID mapping options to use if we're setting up our own user namespace.
	IDMappingOptions *IDMappingOptions
	// AddCapabilities is a list of capabilities to add to the default set when
	// running commands in the container.
	AddCapabilities []string
	// DropCapabilities is a list of capabilities to remove from the default set,
	// after processing the AddCapabilities set, when running commands in the
	// container.  If a capability appears in both lists, it will be dropped.
	DropCapabilities []string

	CommonBuildOpts *CommonBuildOptions
	// Format for the container image
	Format string
}

// ImportOptions are used to initialize a Builder from an existing container
// which was created elsewhere.
type ImportOptions struct {
	// Container is the name of the build container.
	Container string
	// SignaturePolicyPath specifies an override location for the signature
	// policy which should be used for verifying the new image as it is
	// being written.  Except in specific circumstances, no value should be
	// specified, indicating that the shared, system-wide default policy
	// should be used.
	SignaturePolicyPath string
}

// ImportFromImageOptions are used to initialize a Builder from an image.
type ImportFromImageOptions struct {
	// Image is the name or ID of the image we'd like to examine.
	Image string
	// SignaturePolicyPath specifies an override location for the signature
	// policy which should be used for verifying the new image as it is
	// being written.  Except in specific circumstances, no value should be
	// specified, indicating that the shared, system-wide default policy
	// should be used.
	SignaturePolicyPath string
	// github.com/containers/image/types SystemContext to hold information
	// about which registries we should check for completing image names
	// that don't include a domain portion.
	SystemContext *types.SystemContext
}

// NewBuilder creates a new build container.
func NewBuilder(ctx context.Context, store storage.Store, options BuilderOptions) (*Builder, error) {
	return newBuilder(ctx, store, options)
}

// ImportBuilder creates a new build configuration using an already-present
// container.
func ImportBuilder(ctx context.Context, store storage.Store, options ImportOptions) (*Builder, error) {
	return importBuilder(ctx, store, options)
}

// ImportBuilderFromImage creates a new builder configuration using an image.
// The returned object can be modified and examined, but it can not be saved
// or committed because it is not associated with a working container.
func ImportBuilderFromImage(ctx context.Context, store storage.Store, options ImportFromImageOptions) (*Builder, error) {
	return importBuilderFromImage(ctx, store, options)
}

// OpenBuilder loads information about a build container given its name or ID.
func OpenBuilder(store storage.Store, container string) (*Builder, error) {
	cdir, err := store.ContainerDirectory(container)
	if err != nil {
		return nil, err
	}
	buildstate, err := ioutil.ReadFile(filepath.Join(cdir, stateFile))
	if err != nil {
		return nil, errors.Wrapf(err, "error reading %q", filepath.Join(cdir, stateFile))
	}
	b := &Builder{}
	if err = json.Unmarshal(buildstate, &b); err != nil {
		return nil, errors.Wrapf(err, "error parsing %q, read from %q", string(buildstate), filepath.Join(cdir, stateFile))
	}
	if b.Type != containerType {
		return nil, errors.Errorf("container %q is not a %s container (is a %q container)", container, Package, b.Type)
	}
	b.store = store
	b.fixupConfig()
	return b, nil
}

// OpenBuilderByPath loads information about a build container given a
// path to the container's root filesystem
func OpenBuilderByPath(store storage.Store, path string) (*Builder, error) {
	containers, err := store.Containers()
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error turning %q into an absolute path", path)
	}
	builderMatchesPath := func(b *Builder, path string) bool {
		return (b.MountPoint == path)
	}
	for _, container := range containers {
		cdir, err := store.ContainerDirectory(container.ID)
		if err != nil {
			return nil, err
		}
		buildstate, err := ioutil.ReadFile(filepath.Join(cdir, stateFile))
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Debugf("error reading %q: %v, ignoring container %q", filepath.Join(cdir, stateFile), err, container.ID)
				continue
			}
			return nil, errors.Wrapf(err, "error reading %q", filepath.Join(cdir, stateFile))
		}
		b := &Builder{}
		err = json.Unmarshal(buildstate, &b)
		if err == nil && b.Type == containerType && builderMatchesPath(b, abs) {
			b.store = store
			b.fixupConfig()
			return b, nil
		}
		if err != nil {
			logrus.Debugf("error parsing %q, read from %q: %v", string(buildstate), filepath.Join(cdir, stateFile), err)
		} else if b.Type != containerType {
			logrus.Debugf("container %q is not a %s container (is a %q container)", container.ID, Package, b.Type)
		}
	}
	return nil, storage.ErrContainerUnknown
}

// OpenAllBuilders loads all containers which have a state file that we use in
// their data directory, typically so that they can be listed.
func OpenAllBuilders(store storage.Store) (builders []*Builder, err error) {
	containers, err := store.Containers()
	if err != nil {
		return nil, err
	}
	for _, container := range containers {
		cdir, err := store.ContainerDirectory(container.ID)
		if err != nil {
			return nil, err
		}
		buildstate, err := ioutil.ReadFile(filepath.Join(cdir, stateFile))
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Debugf("error reading %q: %v, ignoring container %q", filepath.Join(cdir, stateFile), err, container.ID)
				continue
			}
			return nil, errors.Wrapf(err, "error reading %q", filepath.Join(cdir, stateFile))
		}
		b := &Builder{}
		err = json.Unmarshal(buildstate, &b)
		if err == nil && b.Type == containerType {
			b.store = store
			b.fixupConfig()
			builders = append(builders, b)
			continue
		}
		if err != nil {
			logrus.Debugf("error parsing %q, read from %q: %v", string(buildstate), filepath.Join(cdir, stateFile), err)
		} else if b.Type != containerType {
			logrus.Debugf("container %q is not a %s container (is a %q container)", container.ID, Package, b.Type)
		}
	}
	return builders, nil
}

// Save saves the builder's current state to the build container's metadata.
// This should not need to be called directly, as other methods of the Builder
// object take care of saving their state.
func (b *Builder) Save() error {
	buildstate, err := json.Marshal(b)
	if err != nil {
		return err
	}
	cdir, err := b.store.ContainerDirectory(b.ContainerID)
	if err != nil {
		return err
	}
	if err = ioutils.AtomicWriteFile(filepath.Join(cdir, stateFile), buildstate, 0600); err != nil {
		return errors.Wrapf(err, "error saving builder state to %q", filepath.Join(cdir, stateFile))
	}
	return nil
}
