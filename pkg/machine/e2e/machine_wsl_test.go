package e2e_test

import (
	"errors"
	"fmt"
	url2 "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/compression"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/wsl"
	. "github.com/onsi/ginkgo/v2"
)

func pullWSLDisk() error {
	downloadLocation := os.Getenv("MACHINE_IMAGE")
	if downloadLocation == "" {
		dl, _, _, _, err := wsl.GetFedoraDownloadForWSL()
		if err != nil {
			return errors.New("unable to determine WSL download")
		}
		downloadLocation = dl.String()
	}

	if downloadLocation == "" {
		return errors.New("machine tests require a file reference to a disk image right now")
	}
	compressionExtension := ".xz"
	suiteImageName = strings.TrimSuffix(path.Base(downloadLocation), compressionExtension)
	fqImageName = filepath.Join(tmpDir, suiteImageName)
	getMe, err := url2.Parse(downloadLocation)
	if err != nil {
		return fmt.Errorf("unable to create url for download: %q", err)
	}
	now := time.Now()
	if err := machine.DownloadVMImage(getMe, suiteImageName, fqImageName+compressionExtension); err != nil {
		return fmt.Errorf("unable to download machine image: %q", err)
	}
	GinkgoWriter.Println("Download took: ", time.Since(now).String())
	diskImage, err := define.NewMachineFile(fqImageName+compressionExtension, nil)
	if err != nil {
		return fmt.Errorf("unable to create vmfile %q: %v", fqImageName+compressionExtension, err)
	}
	compressionStart := time.Now()
	if err := compression.Decompress(diskImage, fqImageName); err != nil {
		return fmt.Errorf("unable to decompress image file: %q", err)
	}
	GinkgoWriter.Println("compression took: ", time.Since(compressionStart))
	return nil
}
