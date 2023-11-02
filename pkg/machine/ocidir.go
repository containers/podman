package machine

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/pkg/machine/ocipull"
	"github.com/sirupsen/logrus"
)

type LocalBlobDir struct {
	blob            *types.BlobInfo
	blobDirPath     string
	ctx             context.Context
	imageName       string
	machineImageDir string
	vmName          string
}

func NewOCIDir(ctx context.Context, inputDir, machineImageDir, vmName string) *LocalBlobDir {
	strippedInputDir := strings.TrimPrefix(inputDir, fmt.Sprintf("%s:/", OCIDir.String()))
	l := LocalBlobDir{
		blob:            nil,
		blobDirPath:     strippedInputDir,
		ctx:             ctx,
		imageName:       "",
		machineImageDir: machineImageDir,
		vmName:          vmName,
	}
	return &l
}

func (l *LocalBlobDir) Pull() error {
	localBlob, err := ocipull.GetLocalBlob(l.ctx, l.DiskEndpoint())
	if err != nil {
		return err
	}
	l.blob = localBlob
	return nil
}

func (l *LocalBlobDir) Decompress(compressedFile *VMFile) (*VMFile, error) {
	finalName := finalFQImagePathName(l.vmName, l.imageName)
	if err := Decompress(compressedFile, finalName); err != nil {
		return nil, err
	}
	return NewMachineFile(finalName, nil)
}

func (l *LocalBlobDir) Unpack() (*VMFile, error) {
	tbPath := localOCIDiskImageDir(l.blobDirPath, l.blob)
	unPackedFile, err := unpackOCIDir(tbPath, l.machineImageDir)
	if err != nil {
		return nil, err
	}
	l.imageName = unPackedFile.GetPath()
	return unPackedFile, err
}

func (l *LocalBlobDir) DiskEndpoint() string {
	return l.blobDirPath
}

func (l *LocalBlobDir) LocalBlob() *types.BlobInfo {
	return l.blob
}

// findTarComponent returns a header and a reader matching componentPath within inputFile,
// or (nil, nil, nil) if not found.
func findTarComponent(pathToTar string) (string, error) {
	f, err := os.Open(pathToTar)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	uncompressedReader, _, err := compression.AutoDecompress(f)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := uncompressedReader.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	var (
		filename    string
		headerCount uint
	)
	t := tar.NewReader(uncompressedReader)
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		filename = h.Name
		headerCount++
	}
	if headerCount != 1 {
		return "", errors.New("invalid oci machine image")
	}
	return filename, nil
}
