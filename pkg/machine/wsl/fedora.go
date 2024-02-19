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
	"github.com/sirupsen/logrus"
)

const (
	githubX86ReleaseURL = "https://github.com/containers/podman-wsl-fedora/releases/latest/download/rootfs.tar.xz"
	githubArmReleaseURL = "https://github.com/containers/podman-wsl-fedora-arm/releases/latest/download/rootfs.tar.xz"
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
	var releaseURL string
	arch := machine.DetermineMachineArch()
	switch arch {
	case "arm64":
		releaseURL = githubArmReleaseURL
	case "amd64":
		releaseURL = githubX86ReleaseURL
	default:
		return nil, "", "", -1, fmt.Errorf("CPU architecture %q is not supported", arch)
	}

	downloadURL, err := url.Parse(releaseURL)
	if err != nil {
		return nil, "", "", -1, fmt.Errorf("invalid URL generated from discovered Fedora file: %s: %w", releaseURL, err)
	}

	resp, err := http.Head(releaseURL)
	if err != nil {
		return nil, "", "", -1, fmt.Errorf("head request failed: %s: %w", releaseURL, err)
	}
	_ = resp.Body.Close()
	contentLen := resp.ContentLength

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", -1, fmt.Errorf("head request failed: %s: %w", releaseURL, err)
	}

	verURL := *downloadURL
	verURL.Path = path.Join(path.Dir(downloadURL.Path), "version")

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
	return downloadURL, strings.TrimSpace(string(b)), arch, contentLen, nil
}
