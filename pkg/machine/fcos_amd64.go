package machine

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/coreos/stream-metadata-go/fedoracoreos"
	"github.com/coreos/stream-metadata-go/stream"
	"github.com/sirupsen/logrus"
)

// This should get Exported and stay put as it will apply to all fcos downloads
// getFCOS parses fedoraCoreOS's stream and returns the image download URL and the release version
func getFCOSDownload() (*fcosDownloadInfo, error) {
	var (
		fcosstable stream.Stream
	)
	streamurl := fedoracoreos.GetStreamURL(fedoracoreos.StreamNext)
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
