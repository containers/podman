// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package machine

import (
	url2 "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

// These should eventually be moved into machine/qemu as
// they are specific to running qemu
var (
	artifact string = "qemu"
	Format   string = "qcow2.xz"
)

type FcosDownload struct {
	Download
}

func NewFcosDownloader(vmType, vmName string) (DistributionDownload, error) {
	info, err := getFCOSDownload()
	if err != nil {
		return nil, err
	}
	urlSplit := strings.Split(info.Location, "/")
	imageName := urlSplit[len(urlSplit)-1]
	url, err := url2.Parse(info.Location)
	if err != nil {
		return nil, err
	}

	dataDir, err := GetDataDir(vmType)
	if err != nil {
		return nil, err
	}

	fcd := FcosDownload{
		Download: Download{
			Arch:      getFcosArch(),
			Artifact:  artifact,
			Format:    Format,
			ImageName: imageName,
			LocalPath: filepath.Join(dataDir, imageName),
			Sha256sum: info.Sha256Sum,
			URL:       url,
			VMName:    vmName,
		},
	}
	fcd.Download.LocalUncompressedFile = fcd.getLocalUncompressedName()
	return fcd, nil
}

func (f FcosDownload) getLocalUncompressedName() string {
	uncompressedFilename := filepath.Join(filepath.Dir(f.LocalPath), f.VMName+"_"+f.ImageName)
	return strings.TrimSuffix(uncompressedFilename, ".xz")
}

func (f FcosDownload) DownloadImage() error {
	// check if the latest image is already present
	ok, err := UpdateAvailable(&f.Download)
	if err != nil {
		return err
	}
	if !ok {
		if err := DownloadVMImage(f.URL, f.LocalPath); err != nil {
			return err
		}
	}
	return Decompress(f.LocalPath, f.getLocalUncompressedName())
}

func (f FcosDownload) Get() *Download {
	return &f.Download
}

type fcosDownloadInfo struct {
	CompressionType string
	Location        string
	Release         string
	Sha256Sum       string
}

func UpdateAvailable(d *Download) (bool, error) {
	//	 check the sha of the local image if it exists
	//  get the sha of the remote image
	// == dont bother to pull
	if _, err := os.Stat(d.LocalPath); os.IsNotExist(err) {
		return false, nil
	}
	fd, err := os.Open(d.LocalPath)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := fd.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	sum, err := digest.SHA256.FromReader(fd)
	if err != nil {
		return false, err
	}
	return sum.Encoded() == d.Sha256sum, nil
}

func getFcosArch() string {
	var arch string
	// TODO fill in more architectures
	switch runtime.GOARCH {
	case "arm64":
		arch = "aarch64"
	default:
		arch = "x86_64"
	}
	return arch
}
