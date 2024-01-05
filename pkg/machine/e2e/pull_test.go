package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/coreos/stream-metadata-go/fedoracoreos"
	"github.com/coreos/stream-metadata-go/stream"
	"github.com/sirupsen/logrus"
)

func GetDownload() (string, error) {
	var (
		fcosstable stream.Stream
	)
	url := fedoracoreos.GetStreamURL("testing")
	resp, err := http.Get(url.String())
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if err := json.Unmarshal(body, &fcosstable); err != nil {
		return "", err
	}
	arch, ok := fcosstable.Architectures[machine.GetFcosArch()]
	if !ok {
		return "", fmt.Errorf("unable to pull VM image: no targetArch in stream")
	}
	upstreamArtifacts := arch.Artifacts
	if upstreamArtifacts == nil {
		return "", fmt.Errorf("unable to pull VM image: no artifact in stream")
	}
	upstreamArtifact, ok := upstreamArtifacts["qemu"]
	if !ok {
		return "", fmt.Errorf("unable to pull VM image: no %s artifact in stream", "qemu")
	}
	formats := upstreamArtifact.Formats
	if formats == nil {
		return "", fmt.Errorf("unable to pull VM image: no formats in stream")
	}
	formatType, ok := formats["qcow2.xz"]
	if !ok {
		return "", fmt.Errorf("unable to pull VM image: no %s format in stream", "qcow2.xz")
	}
	disk := formatType.Disk
	return disk.Location, nil
}
