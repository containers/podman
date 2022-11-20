//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/stream-metadata-go/fedoracoreos"
	"github.com/coreos/stream-metadata-go/release"
	"github.com/coreos/stream-metadata-go/stream"

	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

// These should eventually be moved into machine/qemu as
// they are specific to running qemu
var (
	artifact = "qemu"
	Format   = "qcow2.xz"
)

const (
	// Used for testing the latest podman in fcos
	// special builds
	podmanTesting     = "podman-testing"
	PodmanTestingHost = "fedorapeople.org"
	PodmanTestingURL  = "groups/podman/testing"
)

type FcosDownload struct {
	Download
}

func NewFcosDownloader(vmType, vmName, imageStream string) (DistributionDownload, error) {
	info, err := GetFCOSDownload(imageStream)
	if err != nil {
		return nil, err
	}
	urlSplit := strings.Split(info.Location, "/")
	imageName := urlSplit[len(urlSplit)-1]
	url, err := url2.Parse(info.Location)
	if err != nil {
		return nil, err
	}

	cacheDir, err := GetCacheDir(vmType)
	if err != nil {
		return nil, err
	}

	fcd := FcosDownload{
		Download: Download{
			Arch:      getFcosArch(),
			Artifact:  artifact,
			CacheDir:  cacheDir,
			Format:    Format,
			ImageName: imageName,
			LocalPath: filepath.Join(cacheDir, imageName),
			Sha256sum: info.Sha256Sum,
			URL:       url,
			VMName:    vmName,
		},
	}
	dataDir, err := GetDataDir(vmType)
	if err != nil {
		return nil, err
	}
	fcd.Download.LocalUncompressedFile = fcd.getLocalUncompressedFile(dataDir)
	return fcd, nil
}

func (f FcosDownload) Get() *Download {
	return &f.Download
}

type FcosDownloadInfo struct {
	CompressionType string
	Location        string
	Release         string
	Sha256Sum       string
}

func (f FcosDownload) HasUsableCache() (bool, error) {
	//	 check the sha of the local image if it exists
	//  get the sha of the remote image
	// == do not bother to pull
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

func (f FcosDownload) CleanCache() error {
	// Set cached image to expire after 2 weeks
	// FCOS refreshes around every 2 weeks, assume old images aren't needed
	expire := 14 * 24 * time.Hour
	return removeImageAfterExpire(f.CacheDir, expire)
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

// getStreamURL is a wrapper for the fcos.GetStream URL
// so that we can inject a special stream and url for
// testing podman before it merges into fcos builds
func getStreamURL(streamType string) url2.URL {
	// For the podmanTesting stream type, we point to
	// a custom url on fedorapeople.org
	if streamType == podmanTesting {
		return url2.URL{
			Scheme: "https",
			Host:   PodmanTestingHost,
			Path:   fmt.Sprintf("%s/%s.json", PodmanTestingURL, "podman4"),
		}
	}
	return fedoracoreos.GetStreamURL(streamType)
}

// This should get Exported and stay put as it will apply to all fcos downloads
// getFCOS parses fedoraCoreOS's stream and returns the image download URL and the release version
func GetFCOSDownload(imageStream string) (*FcosDownloadInfo, error) {
	var (
		fcosstable stream.Stream
		altMeta    release.Release
		streamType string
	)

	switch imageStream {
	case "podman-testing":
		streamType = "podman-testing"
	case "testing", "":
		streamType = fedoracoreos.StreamTesting
	case "next":
		streamType = fedoracoreos.StreamNext
	case "stable":
		streamType = fedoracoreos.StreamStable
	default:
		return nil, fmt.Errorf("invalid stream %s: valid streams are `testing` and `stable`", imageStream)
	}
	streamurl := getStreamURL(streamType)
	resp, err := http.Get(streamurl.String())
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	if imageStream == podmanTesting {
		if err := json.Unmarshal(body, &altMeta); err != nil {
			return nil, err
		}

		arches, ok := altMeta.Architectures[getFcosArch()]
		if !ok {
			return nil, fmt.Errorf("unable to pull VM image: no targetArch in stream")
		}
		qcow2, ok := arches.Media.Qemu.Artifacts["qcow2.xz"]
		if !ok {
			return nil, fmt.Errorf("unable to pull VM image: no qcow2.xz format in stream")
		}
		disk := qcow2.Disk

		return &FcosDownloadInfo{
			Location:        disk.Location,
			Sha256Sum:       disk.Sha256,
			CompressionType: "xz",
		}, nil
	}

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
	return &FcosDownloadInfo{
		Location:        disk.Location,
		Release:         qemu.Release,
		Sha256Sum:       disk.Sha256,
		CompressionType: "xz",
	}, nil
}
