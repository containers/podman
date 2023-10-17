package machine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/pkg/machine/ocipull"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

type Versioned struct {
	blob            *types.BlobInfo
	blobDirPath     string
	cacheDir        string
	ctx             context.Context
	imageFormat     ImageFormat
	imageName       string
	machineImageDir string
	machineVersion  *OSVersion
	vmName          string
}

func newVersioned(ctx context.Context, machineImageDir, vmName string) (*Versioned, error) {
	imageCacheDir := filepath.Join(machineImageDir, "cache")
	if err := os.MkdirAll(imageCacheDir, 0777); err != nil {
		return nil, err
	}
	o := getVersion()
	return &Versioned{ctx: ctx, cacheDir: imageCacheDir, machineImageDir: machineImageDir, machineVersion: o, vmName: vmName}, nil
}

func (d *Versioned) LocalBlob() *types.BlobInfo {
	return d.blob
}

func (d *Versioned) DiskEndpoint() string {
	return d.machineVersion.diskImage(d.imageFormat)
}

func (d *Versioned) versionedOCICacheDir() string {
	return filepath.Join(d.cacheDir, d.machineVersion.majorMinor())
}

func (d *Versioned) identifyImageNameFromOCIDir() (string, error) {
	imageManifest, err := ocipull.ReadImageManifestFromOCIPath(d.ctx, d.versionedOCICacheDir())
	if err != nil {
		return "", err
	}
	if len(imageManifest.Layers) > 1 {
		return "", fmt.Errorf("podman machine images can have only one layer: %d found", len(imageManifest.Layers))
	}
	path := filepath.Join(d.versionedOCICacheDir(), "blobs", "sha256", imageManifest.Layers[0].Digest.Hex())
	return findTarComponent(path)
}

func (d *Versioned) pull(path string) error {
	fmt.Printf("Pulling %s\n", d.DiskEndpoint())
	logrus.Debugf("pulling %s to %s", d.DiskEndpoint(), path)
	return ocipull.Pull(d.ctx, d.DiskEndpoint(), path, ocipull.PullOptions{})
}

func (d *Versioned) Pull() error {
	var (
		err              error
		isUpdatable      bool
		localBlob        *types.BlobInfo
		remoteDescriptor *v1.Descriptor
	)

	remoteDiskImage := d.machineVersion.diskImage(Qcow)
	logrus.Debugf("podman disk image name: %s", remoteDiskImage)

	// is there a valid oci dir in our cache
	hasCache := d.localOCIDirExists()

	if hasCache {
		logrus.Debug("checking remote registry")
		remoteDescriptor, err = ocipull.GetRemoteDescriptor(d.ctx, remoteDiskImage)
		if err != nil {
			return err
		}
		logrus.Debugf("working with local cache: %s", d.versionedOCICacheDir())
		localBlob, err = ocipull.GetLocalBlob(d.ctx, d.versionedOCICacheDir())
		if err != nil {
			return err
		}
		// determine if the local is same as remote
		if remoteDescriptor.Digest.Hex() != localBlob.Digest.Hex() {
			logrus.Debugf("new image is available: %s", remoteDescriptor.Digest.Hex())
			isUpdatable = true
		}
	}
	if !hasCache || isUpdatable {
		if hasCache {
			if err := GuardedRemoveAll(d.versionedOCICacheDir()); err != nil {
				return err
			}
		}
		if err := d.pull(d.versionedOCICacheDir()); err != nil {
			return err
		}
	}
	imageName, err := d.identifyImageNameFromOCIDir()
	if err != nil {
		return err
	}
	logrus.Debugf("image name: %s", imageName)
	d.imageName = imageName

	if localBlob == nil {
		localBlob, err = ocipull.GetLocalBlob(d.ctx, d.versionedOCICacheDir())
		if err != nil {
			return err
		}
	}
	d.blob = localBlob
	d.blobDirPath = d.versionedOCICacheDir()
	logrus.Debugf("local oci disk image blob: %s", d.localOCIDiskImageDir(localBlob))
	return nil
}

func (d *Versioned) Unpack() (*VMFile, error) {
	tbPath := localOCIDiskImageDir(d.blobDirPath, d.blob)
	unpackedFile, err := unpackOCIDir(tbPath, d.machineImageDir)
	if err != nil {
		return nil, err
	}
	d.imageName = unpackedFile.GetPath()
	return unpackedFile, nil
}

func (d *Versioned) Decompress(compressedFile *VMFile) (*VMFile, error) {
	imageCompression := compressionFromFile(d.imageName)
	strippedImageName := strings.TrimSuffix(d.imageName, fmt.Sprintf(".%s", imageCompression.String()))
	finalName := finalFQImagePathName(d.vmName, strippedImageName)
	if err := Decompress(compressedFile, finalName); err != nil {
		return nil, err
	}
	return NewMachineFile(finalName, nil)
}

func (d *Versioned) localOCIDiskImageDir(localBlob *types.BlobInfo) string {
	return filepath.Join(d.versionedOCICacheDir(), "blobs", "sha256", localBlob.Digest.Hex())
}

func (d *Versioned) localOCIDirExists() bool {
	_, indexErr := os.Stat(filepath.Join(d.versionedOCICacheDir(), "index.json"))
	return indexErr == nil
}
