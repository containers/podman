package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/coreos/stream-metadata-go/fedoracoreos"
	"github.com/coreos/stream-metadata-go/stream"
	"github.com/sirupsen/logrus"
)

func GetDownload(vmType define.VMType) (string, error) {
	var (
		fcosstable           stream.Stream
		artifactType, format string
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

	switch vmType {
	case define.AppleHvVirt:
		artifactType = "applehv"
		format = "raw.gz"
	case define.HyperVVirt:
		artifactType = "hyperv"
		format = "vhdx.zip"
	default:
		artifactType = "qemu"
		format = "qcow2.xz"
	}

	arch, ok := fcosstable.Architectures[machine.GetFcosArch()]
	if !ok {
		return "", fmt.Errorf("unable to pull VM image: no targetArch in stream")
	}
	upstreamArtifacts := arch.Artifacts
	if upstreamArtifacts == nil {
		return "", fmt.Errorf("unable to pull VM image: no artifact in stream")
	}
	upstreamArtifact, ok := upstreamArtifacts[artifactType]
	if !ok {
		return "", fmt.Errorf("unable to pull VM image: no %s artifact in stream", artifactType)
	}
	formats := upstreamArtifact.Formats
	if formats == nil {
		return "", fmt.Errorf("unable to pull VM image: no formats in stream")
	}
	formatType, ok := formats[format]
	if !ok {
		return "", fmt.Errorf("unable to pull VM image: no %s format in stream", format)
	}
	disk := formatType.Disk
	return disk.Location, nil
}
