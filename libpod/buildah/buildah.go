package buildah

import (
	"encoding/json"
	"path/filepath"

	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/docker"
)

const (
	// Package is the name of this package, used in help output and to
	// identify working containers.
	Package = "buildah"
	// Version for the Package.  Bump version in contrib/rpm/buildah.spec
	// too.
	Version = "0.15"
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

// Builder objects are used to represent containers which are being used to
// build images.  They also carry potential updates which will be applied to
// the image's configuration when the container's contents are used to build an
// image.
type Builder struct {
	store storage.Store

	// Type is used to help identify a build container's metadata.  It
	// should not be modified.
	Type string `json:"type"`
	// FromImage is the name of the source image which was used to create
	// the container, if one was used.  It should not be modified.
	FromImage string `json:"image,omitempty"`
	// FromImageID is the ID of the source image which was used to create
	// the container, if one was used.  It should not be modified.
	FromImageID string `json:"image-id"`
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

	// Image metadata and runtime settings, in multiple formats.
	OCIv1  v1.Image       `json:"ociv1,omitempty"`
	Docker docker.V2Image `json:"docker,omitempty"`
	// DefaultMountsFilePath is the file path holding the mounts to be mounted in "host-path:container-path" format
	DefaultMountsFilePath string `json:"defaultMountsFilePath,omitempty"`
	CommonBuildOpts       *CommonBuildOptions
}

// CommonBuildOptions are reseources that can be defined by flags for both buildah from and bud
type CommonBuildOptions struct {
	// AddHost is the list of hostnames to add to the resolv.conf
	AddHost []string
	//CgroupParent it the path to cgroups under which the cgroup for the container will be created.
	CgroupParent string
	//CPUPeriod limits the CPU CFS (Completely Fair Scheduler) period
	CPUPeriod uint64
	//CPUQuota limits the CPU CFS (Completely Fair Scheduler) quota
	CPUQuota int64
	//CPUShares (relative weight
	CPUShares uint64
	//CPUSetCPUs in which to allow execution (0-3, 0,1)
	CPUSetCPUs string
	//CPUSetMems memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.
	CPUSetMems string
	//Memory limit
	Memory int64
	//MemorySwap limit value equal to memory plus swap.
	MemorySwap int64
	//SecruityOpts modify the way container security is running
	LabelOpts          []string
	SeccompProfilePath string
	ApparmorProfile    string
	//ShmSize is the shared memory size
	ShmSize string
	//Ulimit options
	Ulimit []string
	//Volumes to bind mount into the container
	Volumes []string
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

// ImportBuilder creates a new build configuration using an already-present
// container.
func ImportBuilder(store storage.Store, options ImportOptions) (*Builder, error) {
	return importBuilder(store, options)
}

func importBuilder(store storage.Store, options ImportOptions) (*Builder, error) {
	if options.Container == "" {
		return nil, errors.Errorf("container name must be specified")
	}

	c, err := store.Container(options.Container)
	if err != nil {
		return nil, err
	}

	systemContext := getSystemContext(&types.SystemContext{}, options.SignaturePolicyPath)

	builder, err := importBuilderDataFromImage(store, systemContext, c.ImageID, options.Container, c.ID)
	if err != nil {
		return nil, err
	}

	if builder.FromImageID != "" {
		if d, err2 := digest.Parse(builder.FromImageID); err2 == nil {
			builder.Docker.Parent = docker.ID(d)
		} else {
			builder.Docker.Parent = docker.ID(digest.NewDigestFromHex(digest.Canonical.String(), builder.FromImageID))
		}
	}
	if builder.FromImage != "" {
		builder.Docker.ContainerConfig.Image = builder.FromImage
	}

	err = builder.Save()
	if err != nil {
		return nil, errors.Wrapf(err, "error saving builder state")
	}

	return builder, nil
}

func importBuilderDataFromImage(store storage.Store, systemContext *types.SystemContext, imageID, containerName, containerID string) (*Builder, error) {
	manifest := []byte{}
	config := []byte{}
	imageName := ""

	if imageID != "" {
		ref, err := is.Transport.ParseStoreReference(store, imageID)
		if err != nil {
			return nil, errors.Wrapf(err, "no such image %q", imageID)
		}
		src, err2 := ref.NewImage(systemContext)
		if err2 != nil {
			return nil, errors.Wrapf(err2, "error instantiating image")
		}
		defer src.Close()
		config, err = src.ConfigBlob()
		if err != nil {
			return nil, errors.Wrapf(err, "error reading image configuration")
		}
		manifest, _, err = src.Manifest()
		if err != nil {
			return nil, errors.Wrapf(err, "error reading image manifest")
		}
		if img, err3 := store.Image(imageID); err3 == nil {
			if len(img.Names) > 0 {
				imageName = img.Names[0]
			}
		}
	}

	builder := &Builder{
		store:            store,
		Type:             containerType,
		FromImage:        imageName,
		FromImageID:      imageID,
		Config:           config,
		Manifest:         manifest,
		Container:        containerName,
		ContainerID:      containerID,
		ImageAnnotations: map[string]string{},
		ImageCreatedBy:   "",
	}

	builder.initConfig()

	return builder, nil
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
	return ioutils.AtomicWriteFile(filepath.Join(cdir, stateFile), buildstate, 0600)
}
