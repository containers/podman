package wsl

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/version"
	"github.com/sirupsen/logrus"
)

const (
	latestReleaseURL = "https://github.com/containers/podman-machine-wsl-os/releases/latest/download"
)

type FedoraDownload struct {
	machine.Download
}

func (f FedoraDownload) Get() *machine.Download {
	return &f.Download
}

func (f FedoraDownload) HasUsableCache() (bool, error) {
	info, err := os.Stat(f.LocalPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return info.Size() == f.Size, nil
}

func (f FedoraDownload) CleanCache() error {
	// Set cached image to expire after 2 weeks
	expire := 14 * 24 * time.Hour
	return machine.RemoveImageAfterExpire(f.CacheDir, expire)
}

func GetFedoraDownloadForWSL() (*url.URL, string, string, int64, error) {
	arch := machine.DetermineMachineArch()
	if arch != "amd64" && arch != "arm64" {
		return nil, "", "", -1, fmt.Errorf("CPU architecture %q is not supported", arch)
	}

	releaseURL, err := url.Parse(latestReleaseURL)
	if err != nil {
		return nil, "", "", -1, fmt.Errorf("could not parse release URL: %s: %w", releaseURL, err)
	}

	rootFs := fmt.Sprintf("%d.%d-rootfs-%s.tar.zst", version.Version.Major, version.Version.Minor, arch)
	rootFsURL := appendToURL(releaseURL, rootFs)

	resp, err := http.Head(rootFsURL.String())
	if err != nil {
		return nil, "", "", -1, fmt.Errorf("head request failed: %s: %w", releaseURL, err)
	}
	_ = resp.Body.Close()
	contentLen := resp.ContentLength

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", -1, fmt.Errorf("head request failed: %s: %w", rootFsURL, err)
	}

	verURL := appendToURL(releaseURL, "version")
	resp, err = http.Get(verURL.String())
	if err != nil {
		return nil, "", "", -1, fmt.Errorf("get request failed: %s: %w", verURL.String(), err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Errorf("error closing http body: %q", err)
		}
	}()
	b, err := io.ReadAll(&io.LimitedReader{R: resp.Body, N: 1024})
	if err != nil {
		return nil, "", "", -1, fmt.Errorf("failed reading: %s: %w", verURL.String(), err)
	}
	return rootFsURL, strings.TrimSpace(string(b)), arch, contentLen, nil
}

func appendToURL(url *url.URL, elem string) *url.URL {
	newURL := *url
	newURL.Path = path.Join(url.Path, elem)
	return &newURL
}
