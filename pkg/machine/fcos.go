//go:build amd64 || arm64

package machine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"os"
	"runtime"
	"time"

	"github.com/containers/podman/v4/pkg/machine/compression"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/coreos/stream-metadata-go/fedoracoreos"
	"github.com/coreos/stream-metadata-go/release"
	"github.com/coreos/stream-metadata-go/stream"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

const (
	// Used for testing the latest podman in fcos
	// special builds
	podmanTesting     = "podman-testing"
	PodmanTestingHost = "fedorapeople.org"
	PodmanTestingURL  = "groups/podman/testing"
)

//
// TODO artifact, imageformat, and imagecompression should be probably combined into some sort
// of object which can "produce" the correct output we are looking for bc things like
// image format contain both the image type AND the compression.  This work can be done before
// or after the hyperv work.  For now, my preference is to NOT change things and just get things
// typed strongly
//

type FcosDownload struct {
	Download
}

func (f FcosDownload) Get() *Download {
	return &f.Download
}

type FcosDownloadInfo struct {
	CompressionType compression.ImageCompression
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

// GetFCOSDownload parses fedoraCoreOS's stream and returns the image download URL and the release version
func (dl Download) GetFCOSDownload(imageStream FCOSStream) (*FcosDownloadInfo, error) {
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
		qcow2, ok := arches.Media.Qemu.Artifacts[define.Qcow.KindWithCompression()]
		if !ok {
			return nil, fmt.Errorf("unable to pull VM image: no qcow2.xz format in stream")
		}
		disk := qcow2.Disk

		return &FcosDownloadInfo{
			Location:        disk.Location,
			Sha256Sum:       disk.Sha256,
			CompressionType: dl.CompressionType,
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
	upstreamArtifact, ok := upstreamArtifacts[dl.Artifact.String()]
	if !ok {
		return nil, fmt.Errorf("unable to pull VM image: no %s artifact in stream", dl.Artifact.String())
	}
	formats := upstreamArtifact.Formats
	if formats == nil {
		return nil, fmt.Errorf("unable to pull VM image: no formats in stream")
	}
	formatType, ok := formats[dl.Format.KindWithCompression()]
	if !ok {
		return nil, fmt.Errorf("unable to pull VM image: no %s format in stream", dl.Format.KindWithCompression())
	}
	disk := formatType.Disk
	if disk == nil {
		return nil, fmt.Errorf("unable to pull VM image: no disk in stream")
	}
	return &FcosDownloadInfo{
		Location:        disk.Location,
		Release:         upstreamArtifact.Release,
		Sha256Sum:       disk.Sha256,
		CompressionType: dl.CompressionType,
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
	// Unknown
	UnknownStream
	// Custom
	CustomStream
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
	case Stable:
		return "stable"
	}
	return "custom"
}

func FCOSStreamFromString(s string) (FCOSStream, error) {
	switch s {
	case Testing.String():
		return Testing, nil
	case Next.String():
		return Next, nil
	case PodmanTesting.String():
		return PodmanTesting, nil
	case Stable.String():
		return Stable, nil
	case CustomStream.String():
		return CustomStream, nil
	}
	return UnknownStream, fmt.Errorf("unknown fcos stream: %s", s)
}

func IsValidFCOSStreamString(s string) bool {
	switch s {
	case Testing.String():
		fallthrough
	case Next.String():
		fallthrough
	case PodmanTesting.String():
		fallthrough
	case Stable.String():
		return true
	case CustomStream.String():
		return true
	}

	return false
}
