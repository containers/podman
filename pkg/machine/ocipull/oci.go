package ocipull

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/version"
	"github.com/containers/storage/pkg/archive"
	"github.com/sirupsen/logrus"
)

type OSVersion struct {
	*semver.Version
}

type Disker interface {
	Get() error
}

type OCIOpts struct {
	Scheme *OCIKind
	Dir    *string
}

type OCIKind string

var (
	OCIDir      OCIKind = "oci-dir"
	OCIRegistry OCIKind = "docker"
	OCIUnknown  OCIKind = "unknown"
)

func (o OCIKind) String() string {
	switch o {
	case OCIDir:
		return string(OCIDir)
	case OCIRegistry:
		return string(OCIRegistry)
	}
	return string(OCIUnknown)
}

func (o OCIKind) IsOCIDir() bool {
	return o == OCIDir
}

func StripOCIReference(input string) string {
	return strings.TrimPrefix(input, "docker://")
}

func getVersion() *OSVersion {
	v := version.Version
	return &OSVersion{&v}
}

func (o *OSVersion) majorMinor() string {
	return fmt.Sprintf("%d.%d", o.Major, o.Minor)
}

func unpackOCIDir(ociTb, machineImageDir string) (*define.VMFile, error) {
	imageFileName, err := findTarComponent(ociTb)
	if err != nil {
		return nil, err
	}

	unpackedFileName := filepath.Join(machineImageDir, imageFileName)

	f, err := os.Open(ociTb)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	uncompressedReader, _, err := compression.AutoDecompress(f)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := uncompressedReader.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	logrus.Debugf("untarring %q to %q", ociTb, machineImageDir)
	if err := archive.Untar(uncompressedReader, machineImageDir, &archive.TarOptions{
		NoLchown: true,
	}); err != nil {
		return nil, err
	}

	return define.NewMachineFile(unpackedFileName, nil)
}

func localOCIDiskImageDir(blobDirPath string, localBlob *types.BlobInfo) string {
	return filepath.Join(blobDirPath, "blobs", "sha256", localBlob.Digest.Hex())
}

func finalFQImagePathName(vmName, imageName string) string {
	// imageName here is fully qualified. we need to break
	// it apart and add the vmname
	baseDir, filename := filepath.Split(imageName)
	return filepath.Join(baseDir, fmt.Sprintf("%s-%s", vmName, filename))
}
