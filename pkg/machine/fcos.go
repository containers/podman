// +build amd64 arm64

package machine

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	url2 "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/coreos/stream-metadata-go/fedoracoreos"
	"github.com/coreos/stream-metadata-go/stream"
	"github.com/pkg/errors"

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

func NewFcosDownloader(vmType, vmName, imageStream string) (DistributionDownload, error) {
	info, err := getFCOSDownload(imageStream)
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

func (f FcosDownload) Get() *Download {
	return &f.Download
}

type fcosDownloadInfo struct {
	CompressionType string
	Location        string
	Release         string
	Sha256Sum       string
}

func (f FcosDownload) HasUsableCache() (bool, error) {
	//	 check the sha of the local image if it exists
	//  get the sha of the remote image
	// == dont bother to pull
	if _, err := os.Stat(f.LocalPath); os.IsNotExist(err) {
		return false, nil
	}
	fd, err := os.Open(f.LocalPath)
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
	return sum.Encoded() == f.Sha256sum, nil
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

// This should get Exported and stay put as it will apply to all fcos downloads
// getFCOS parses fedoraCoreOS's stream and returns the image download URL and the release version
func getFCOSDownload(imageStream string) (*fcosDownloadInfo, error) {
	var (
		fcosstable stream.Stream
		streamType string
	)
	switch imageStream {
	case "testing", "":
		streamType = fedoracoreos.StreamTesting
	case "next":
		streamType = fedoracoreos.StreamNext
	case "stable":
		streamType = fedoracoreos.StreamStable
	default:
		return nil, errors.Errorf("invalid stream %s: valid streams are `testing` and `stable`", imageStream)
	}
	streamurl := fedoracoreos.GetStreamURL(streamType)
	resp, err := http.Get(streamurl.String())
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if err := json.Unmarshal(body, &fcosstable); err != nil {
		return nil, err
	}
	arch, ok := fcosstable.Architectures[getFcosArch()]
	if !ok {
		return nil, fmt.Errorf("unable to pull VM image: no targetArch in stream")
	}
	artifacts := arch.Artifacts
	if artifacts == nil {
		return nil, fmt.Errorf("unable to pull VM image: no artifact in stream")
	}
	qemu, ok := artifacts[artifact]
	if !ok {
		return nil, fmt.Errorf("unable to pull VM image: no qemu artifact in stream")
	}
	formats := qemu.Formats
	if formats == nil {
		return nil, fmt.Errorf("unable to pull VM image: no formats in stream")
	}
	qcow, ok := formats[Format]
	if !ok {
		return nil, fmt.Errorf("unable to pull VM image: no qcow2.xz format in stream")
	}
	disk := qcow.Disk
	if disk == nil {
		return nil, fmt.Errorf("unable to pull VM image: no disk in stream")
	}
	return &fcosDownloadInfo{
		Location:        disk.Location,
		Release:         qemu.Release,
		Sha256Sum:       disk.Sha256,
		CompressionType: "xz",
	}, nil
}
