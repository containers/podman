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

type ImageCompression int64
type Artifact int64
type ImageFormat int64

const (
	// Used for testing the latest podman in fcos
	// special builds
	podmanTesting     = "podman-testing"
	PodmanTestingHost = "fedorapeople.org"
	PodmanTestingURL  = "groups/podman/testing"

	Xz ImageCompression = iota
	Zip
	Gz
	Bz2

	Qemu Artifact = iota
	HyperV
	Metal
	None

	Qcow ImageFormat = iota
	Vhdx
	Tar
	Raw
)

//
// TODO artifact, imageformat, and imagecompression should be probably combined into some sort
// of object which can "produce" the correct output we are looking for bc things like
// image format contain both the image type AND the compression.  This work can be done before
// or after the hyperv work.  For now, my preference is to NOT change things and just get things
// typed strongly
//

func (a Artifact) String() string {
	switch a {
	case HyperV:
		return "hyperv"
	case Metal:
		return "metal"
	}
	return "qemu"
}

func (imf ImageFormat) String() string {
	switch imf {
	case Vhdx:
		return "vhdx.zip"
	case Tar:
		return "tar.xz"
	case Raw:
		return "raw.xz"
	}
	return "qcow2.xz"
}

func (c ImageCompression) String() string {
	switch c {
	case Gz:
		return "gz"
	case Zip:
		return "zip"
	case Bz2:
		return "bz2"
	}
	return "xz"
}

func compressionFromFile(path string) ImageCompression {
	switch {
	case strings.HasSuffix(path, Bz2.String()):
		return Bz2
	case strings.HasSuffix(path, Gz.String()):
		return Gz
	case strings.HasSuffix(path, Zip.String()):
		return Zip
	}
	return Xz
}

type FcosDownload struct {
	Download
}

func NewFcosDownloader(vmType VMType, vmName string, imageStream FCOSStream, vp VirtProvider) (DistributionDownload, error) {
	info, err := GetFCOSDownload(vp, imageStream)
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
			Arch:      GetFcosArch(),
			Artifact:  Qemu,
			CacheDir:  cacheDir,
			Format:    Qcow,
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
	fcd.Download.LocalUncompressedFile = fcd.GetLocalUncompressedFile(dataDir)
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
	return RemoveImageAfterExpire(f.CacheDir, expire)
}

func GetFcosArch() string {
	var arch string
	// TODO fill in more architectures
	switch runtime.GOARCH {
	case "arm64":
		arch = "aarch64"
	case "riscv64":
		arch = "riscv64"
	default:
		arch = "x86_64"
	}
	return arch
}

// getStreamURL is a wrapper for the fcos.GetStream URL
// so that we can inject a special stream and url for
// testing podman before it merges into fcos builds
func getStreamURL(streamType FCOSStream) url2.URL {
	// For the podmanTesting stream type, we point to
	// a custom url on fedorapeople.org
	if streamType == PodmanTesting {
		return url2.URL{
			Scheme: "https",
			Host:   PodmanTestingHost,
			Path:   fmt.Sprintf("%s/%s.json", PodmanTestingURL, "podman4"),
		}
	}
	return fedoracoreos.GetStreamURL(streamType.String())
}

// This should get Exported and stay put as it will apply to all fcos downloads
// getFCOS parses fedoraCoreOS's stream and returns the image download URL and the release version
func GetFCOSDownload(vp VirtProvider, imageStream FCOSStream) (*FcosDownloadInfo, error) {
	var (
		fcosstable stream.Stream
		altMeta    release.Release
	)

	streamurl := getStreamURL(imageStream)
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
	if imageStream == PodmanTesting {
		if err := json.Unmarshal(body, &altMeta); err != nil {
			return nil, err
		}

		arches, ok := altMeta.Architectures[GetFcosArch()]
		if !ok {
			return nil, fmt.Errorf("unable to pull VM image: no targetArch in stream")
		}
		qcow2, ok := arches.Media.Qemu.Artifacts[Qcow.String()]
		if !ok {
			return nil, fmt.Errorf("unable to pull VM image: no qcow2.xz format in stream")
		}
		disk := qcow2.Disk

		return &FcosDownloadInfo{
			Location:        disk.Location,
			Sha256Sum:       disk.Sha256,
			CompressionType: vp.Compression().String(),
		}, nil
	}

	if err := json.Unmarshal(body, &fcosstable); err != nil {
		return nil, err
	}
	arch, ok := fcosstable.Architectures[GetFcosArch()]
	if !ok {
		return nil, fmt.Errorf("unable to pull VM image: no targetArch in stream")
	}
	upstreamArtifacts := arch.Artifacts
	if upstreamArtifacts == nil {
		return nil, fmt.Errorf("unable to pull VM image: no artifact in stream")
	}
	upstreamArtifact, ok := upstreamArtifacts[vp.Artifact().String()]
	if !ok {
		return nil, fmt.Errorf("unable to pull VM image: no %s artifact in stream", vp.Artifact().String())
	}
	formats := upstreamArtifact.Formats
	if formats == nil {
		return nil, fmt.Errorf("unable to pull VM image: no formats in stream")
	}
	formatType, ok := formats[vp.Format().String()]
	if !ok {
		return nil, fmt.Errorf("unable to pull VM image: no %s format in stream", vp.Format().String())
	}
	disk := formatType.Disk
	if disk == nil {
		return nil, fmt.Errorf("unable to pull VM image: no disk in stream")
	}
	return &FcosDownloadInfo{
		Location:        disk.Location,
		Release:         upstreamArtifact.Release,
		Sha256Sum:       disk.Sha256,
		CompressionType: vp.Compression().String(),
	}, nil
}

type FCOSStream int64

const (
	// FCOS streams
	// Testing FCOS stream
	Testing FCOSStream = iota
	// Next FCOS stream
	Next
	// Stable FCOS stream
	Stable
	// Podman-Testing
	PodmanTesting
)

// String is a helper func for fcos streams
func (st FCOSStream) String() string {
	switch st {
	case Testing:
		return "testing"
	case Next:
		return "next"
	case PodmanTesting:
		return "podman-testing"
	}
	return "stable"
}

func FCOSStreamFromString(s string) FCOSStream {
	switch s {
	case Testing.String():
		return Testing
	case Next.String():
		return Next
	case PodmanTesting.String():
		return PodmanTesting
	}
	return Stable
}
