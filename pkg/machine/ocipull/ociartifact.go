package ocipull

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/machine/compression"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/utils"
	crc "github.com/crc-org/crc/v2/pkg/os"
	"github.com/opencontainers/go-digest"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

const (
	artifactRegistry     = "quay.io"
	artifactRepo         = "podman"
	artifactImageName    = "machine-os"
	artifactImageNameWSL = "machine-os-wsl"
	artifactOriginalName = "org.opencontainers.image.title"
	machineOS            = "linux"
)

type OCIArtifactDisk struct {
	cache                    bool
	cachedCompressedDiskPath *define.VMFile
	name                     string
	ctx                      context.Context
	dirs                     *define.MachineDirs
	diskArtifactOpts         *DiskArtifactOpts
	finalPath                string
	imageEndpoint            string
	machineVersion           *OSVersion
	diskArtifactFileName     string
	pullOptions              *PullOptions
	vmType                   define.VMType
}

type DiskArtifactOpts struct {
	arch     string
	diskType string
	os       string
}

/*

	This interface is for automatically pulling a disk artifact(qcow2, raw, vhdx file) from a pre-determined
	image location.  The logic is tied to vmtypes (applehv, qemu, hyperv) and their understanding of the type of
	disk they require.  The process can be generally described as:

	* Determine the flavor of artifact we are looking for (arch, compression, type)
	* Grab the manifest list for the target
	* Walk the artifacts to find a match based on flavor
	* Check the hash of the artifact against the hash of our cached image
	* If the cached image does not exist or match, pull the latest into an OCI directory
		* Read the OCI blob's manifest to determine which blob is the artifact disk
		* Rename/move the blob in the OCI directory to the image cache dir and append the type and compression
		  i.e. 91d1e51ddfac9d4afb1f96df878089cfdb9ab9be5886f8bccac0f0557ed28974.qcow2.xz
		* Discard the OCI directory
	* Decompress the cached image to the image dir in the form of <vmname>-<arch>.<raw|vhdx|qcow2>

*/

func NewOCIArtifactPull(ctx context.Context, dirs *define.MachineDirs, endpoint string, vmName string, vmType define.VMType, finalPath *define.VMFile) (*OCIArtifactDisk, error) {
	var (
		arch string
	)

	artifactVersion := getVersion()
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	default:
		return nil, fmt.Errorf("unsupported machine arch: %s", runtime.GOARCH)
	}

	diskOpts := DiskArtifactOpts{
		arch:     arch,
		diskType: vmType.DiskType(),
		os:       machineOS,
	}

	cache := false
	if endpoint == "" {
		// The OCI artifact containing the OS image for WSL has a different
		// image name. This should be temporary and dropped as soon as the
		// OS image for WSL is built from fedora-coreos too (c.f. RUN-2178).
		imageName := artifactImageName
		if vmType == define.WSLVirt {
			imageName = artifactImageNameWSL
		}
		endpoint = fmt.Sprintf("docker://%s/%s/%s:%s", artifactRegistry, artifactRepo, imageName, artifactVersion.majorMinor())
		cache = true
	}

	ociDisk := OCIArtifactDisk{
		ctx:              ctx,
		cache:            cache,
		dirs:             dirs,
		diskArtifactOpts: &diskOpts,
		finalPath:        finalPath.GetPath(),
		imageEndpoint:    endpoint,
		machineVersion:   artifactVersion,
		name:             vmName,
		pullOptions:      &PullOptions{},
		vmType:           vmType,
	}
	return &ociDisk, nil
}

func (o *OCIArtifactDisk) OriginalFileName() (string, string) {
	return o.cachedCompressedDiskPath.GetPath(), o.diskArtifactFileName
}

func (o *OCIArtifactDisk) Get() error {
	cleanCache, err := o.get()
	if err != nil {
		return err
	}
	if cleanCache != nil {
		defer cleanCache()
	}
	return o.decompress()
}

func (o *OCIArtifactDisk) GetNoCompress() (func(), error) {
	return o.get()
}

func (o *OCIArtifactDisk) get() (func(), error) {
	cleanCache := func() {}

	destRef, artifactDigest, err := o.getDestArtifact()
	if err != nil {
		return nil, err
	}

	// Note: the artifactDigest here is the hash of the most recent disk image available
	cachedImagePath, err := o.dirs.ImageCacheDir.AppendToNewVMFile(fmt.Sprintf("%s.%s", artifactDigest.Encoded(), o.vmType.ImageFormat().KindWithCompression()), nil)
	if err != nil {
		return nil, err
	}

	// check if we have the latest and greatest disk image
	if _, err = os.Stat(cachedImagePath.GetPath()); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unable to access cached image path %q: %q", cachedImagePath.GetPath(), err)
		}

		// On cache misses, we clean out the cache
		cleanCache = o.cleanCache(cachedImagePath.GetPath())

		// pull the image down to our local filesystem
		if err := o.pull(destRef, artifactDigest); err != nil {
			return nil, fmt.Errorf("failed to pull %s: %w", destRef.DockerReference(), err)
		}
		// grab the artifact disk out of the cache and lay
		// it into our local cache in the format of
		// hash + disktype + compression
		//
		// in cache it will be used until it is "outdated"
		//
		// i.e. 91d1e51...d28974.qcow2.xz
		if err := o.unpack(artifactDigest); err != nil {
			return nil, err
		}
	} else {
		logrus.Debugf("cached image exists and is latest: %s", cachedImagePath.GetPath())
		o.cachedCompressedDiskPath = cachedImagePath
	}
	return cleanCache, nil
}

func (o *OCIArtifactDisk) cleanCache(cachedImagePath string) func() {
	// cache miss while using an image that we cache, ie the default image
	// clean out all old files from the cache dir
	if o.cache {
		files, err := os.ReadDir(o.dirs.ImageCacheDir.GetPath())
		if err != nil {
			logrus.Warn("failed to clean machine image cache: ", err)
			return nil
		}

		return func() {
			for _, file := range files {
				path := filepath.Join(o.dirs.ImageCacheDir.GetPath(), file.Name())
				logrus.Debugf("cleaning cached file: %s", path)
				err := utils.GuardedRemoveAll(path)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					logrus.Warn("failed to clean machine image cache: ", err)
				}
			}
		}
	} else {
		// using an image that we don't cache, ie not the default image
		// delete image after use and don't cache
		return func() {
			logrus.Debugf("cleaning cache: %s", o.dirs.ImageCacheDir.GetPath())
			err := os.Remove(cachedImagePath)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				logrus.Warn("failed to clean pulled machine image: ", err)
			}
		}
	}
}

func (o *OCIArtifactDisk) getDestArtifact() (types.ImageReference, digest.Digest, error) {
	imgRef, err := alltransports.ParseImageName(o.imageEndpoint)
	if err != nil {
		return nil, "", err
	}
	fmt.Printf("Looking up Podman Machine image at %s to create VM\n", imgRef.DockerReference())
	sysCtx := &types.SystemContext{
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(!o.pullOptions.TLSVerify),
	}
	imgSrc, err := imgRef.NewImageSource(o.ctx, sysCtx)
	if err != nil {
		return nil, "", err
	}

	defer func() {
		if err := imgSrc.Close(); err != nil {
			logrus.Warn(err)
		}
	}()

	diskArtifactDigest, err := GetDiskArtifactReference(o.ctx, imgSrc, o.diskArtifactOpts)
	if err != nil {
		return nil, "", err
	}
	// create a ref now and return
	named := imgRef.DockerReference()
	digestedRef, err := reference.WithDigest(reference.TrimNamed(named), diskArtifactDigest)
	if err != nil {
		return nil, "", err
	}

	// Get and "store" the original filename the disk artifact had
	originalFileName, err := getOriginalFileName(o.ctx, imgSrc, diskArtifactDigest)
	if err != nil {
		return nil, "", err
	}
	o.diskArtifactFileName = originalFileName

	newRef, err := docker.NewReference(digestedRef)
	if err != nil {
		return nil, "", err
	}
	return newRef, diskArtifactDigest, err
}

func (o *OCIArtifactDisk) pull(destRef types.ImageReference, artifactDigest digest.Digest) error {
	destFileName := artifactDigest.Encoded()
	destFile, err := o.dirs.ImageCacheDir.AppendToNewVMFile(destFileName, nil)
	if err != nil {
		return err
	}
	return Pull(o.ctx, destRef, destFile, o.pullOptions)
}

func (o *OCIArtifactDisk) unpack(diskArtifactHash digest.Digest) error {
	finalSuffix := extractKindAndCompression(o.diskArtifactFileName)
	blobDir, err := o.dirs.ImageCacheDir.AppendToNewVMFile(diskArtifactHash.Encoded(), nil)
	if err != nil {
		return err
	}
	cachedCompressedPath, err := o.dirs.ImageCacheDir.AppendToNewVMFile(diskArtifactHash.Encoded()+finalSuffix, nil)
	if err != nil {
		return err
	}

	o.cachedCompressedDiskPath = cachedCompressedPath

	blobInfo, err := GetLocalBlob(o.ctx, blobDir.GetPath())
	if err != nil {
		return fmt.Errorf("unable to get local manifest for %s: %q", blobDir.GetPath(), err)
	}

	diskBlobPath := filepath.Join(blobDir.GetPath(), "blobs", "sha256", blobInfo.Digest.Encoded())

	// Rename and move the hashed blob file to the cache dir.
	// If the rename fails, we do a sparsecopy instead
	if err := os.Rename(diskBlobPath, cachedCompressedPath.GetPath()); err != nil {
		logrus.Errorf("renaming compressed image %q failed: %q", cachedCompressedPath.GetPath(), err)
		logrus.Error("trying again using copy")
		if err := crc.CopyFileSparse(diskBlobPath, cachedCompressedPath.GetPath()); err != nil {
			return err
		}
	}

	// Clean up the oci dir which is no longer needed
	return utils.GuardedRemoveAll(blobDir.GetPath())
}

func (o *OCIArtifactDisk) decompress() error {
	return compression.Decompress(o.cachedCompressedDiskPath, o.finalPath)
}

func getOriginalFileName(ctx context.Context, imgSrc types.ImageSource, artifactDigest digest.Digest) (string, error) {
	v1RawMannyfest, _, err := imgSrc.GetManifest(ctx, &artifactDigest)
	if err != nil {
		return "", err
	}
	v1MannyFest := specV1.Manifest{}
	if err := json.Unmarshal(v1RawMannyfest, &v1MannyFest); err != nil {
		return "", err
	}
	if layerLen := len(v1MannyFest.Layers); layerLen > 1 {
		return "", fmt.Errorf("podman-machine images should only have 1 layer: %d found", layerLen)
	}

	// podman-machine-images should have an original file name
	// stored in the annotations under org.opencontainers.image.title
	// i.e. fedora-coreos-39.20240128.2.2-qemu.x86_64.qcow2.xz
	originalFileName, ok := v1MannyFest.Layers[0].Annotations[artifactOriginalName]
	if !ok {
		return "", fmt.Errorf("unable to determine original artifact name: missing required annotation 'org.opencontainers.image.title'")
	}
	logrus.Debugf("original artifact file name: %s", originalFileName)
	return originalFileName, nil
}

// extractKindAndCompression extracts the vmimage type and the compression type
// this is used for when we rename the blob from its hash to something real
// i.e. fedora-coreos-39.20240128.2.2-qemu.x86_64.qcow2.xz would return qcow2.xz
func extractKindAndCompression(name string) string {
	compressAlgo := filepath.Ext(name)
	compressStrippedName := strings.TrimSuffix(name, compressAlgo)
	kind := filepath.Ext(compressStrippedName)
	return kind + compressAlgo
}
