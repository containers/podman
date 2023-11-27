//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman/v4/pkg/machine/compression"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/ocipull"
	"github.com/containers/podman/v4/utils"
	"github.com/sirupsen/logrus"
)

// GenericDownload is used when a user provides a URL
// or path for an image
type GenericDownload struct {
	Download
}

// NewGenericDownloader is used when the disk image is provided by the user
func NewGenericDownloader(vmType VMType, vmName, pullPath string) (DistributionDownload, error) {
	var (
		imageName string
	)
	dataDir, err := GetDataDir(vmType)
	if err != nil {
		return nil, err
	}
	cacheDir, err := GetCacheDir(vmType)
	if err != nil {
		return nil, err
	}
	dl := Download{}
	// Is pullpath a file or url?
	if getURL := supportedURL(pullPath); getURL != nil {
		urlSplit := strings.Split(getURL.Path, "/")
		imageName = urlSplit[len(urlSplit)-1]
		dl.URL = getURL
		dl.LocalPath = filepath.Join(cacheDir, imageName)
	} else {
		// Dealing with FilePath
		imageName = filepath.Base(pullPath)
		dl.LocalPath = pullPath
	}
	dl.VMName = vmName
	dl.ImageName = imageName
	dl.LocalUncompressedFile = dl.GetLocalUncompressedFile(dataDir)
	// The download needs to be pulled into the datadir

	gd := GenericDownload{Download: dl}
	return gd, nil
}

func supportedURL(path string) (url *url2.URL) {
	getURL, err := url2.Parse(path)
	if err != nil {
		// ignore error, probably not a URL, fallback & treat as file path
		return nil
	}

	// Check supported scheme. Since URL is passed to net.http, only http
	// schemes are supported. Also, windows drive paths can resemble a
	// URL, but with a single letter scheme. These values should be
	// passed through for interpretation as a file path.
	switch getURL.Scheme {
	case "http":
		fallthrough
	case "https":
		return getURL
	default:
		return nil
	}
}

func (dl Download) GetLocalUncompressedFile(dataDir string) string {
	compressedFilename := dl.VMName + "_" + dl.ImageName
	extension := compression.KindFromFile(compressedFilename)
	uncompressedFile := strings.TrimSuffix(compressedFilename, fmt.Sprintf(".%s", extension.String()))
	dl.LocalUncompressedFile = filepath.Join(dataDir, uncompressedFile)
	return dl.LocalUncompressedFile
}

func (g GenericDownload) Get() *Download {
	return &g.Download
}

func (g GenericDownload) HasUsableCache() (bool, error) {
	// If we have a URL for this "downloader", we now pull it
	return g.URL == nil, nil
}

// CleanCache cleans out downloaded uncompressed image files
func (g GenericDownload) CleanCache() error {
	// Remove any image that has been downloaded via URL
	// We never read from cache for generic downloads
	if g.URL != nil {
		if err := os.Remove(g.LocalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func DownloadImage(d DistributionDownload) error {
	// check if the latest image is already present
	ok, err := d.HasUsableCache()
	if err != nil {
		return err
	}
	if !ok {
		if err := DownloadVMImage(d.Get().URL, d.Get().ImageName, d.Get().LocalPath); err != nil {
			return err
		}
		// Clean out old cached images, since we didn't find needed image in cache
		defer func() {
			if err = d.CleanCache(); err != nil {
				logrus.Warnf("error cleaning machine image cache: %s", err)
			}
		}()
	}
	localPath, err := define.NewMachineFile(d.Get().LocalPath, nil)
	if err != nil {
		return err
	}
	return compression.Decompress(localPath, d.Get().LocalUncompressedFile)
}

// DownloadVMImage downloads a VM image from url to given path
// with download status
func DownloadVMImage(downloadURL *url2.URL, imageName string, localImagePath string) error {
	out, err := os.Create(localImagePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	resp, err := http.Get(downloadURL.String())
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading VM image %s: %s", downloadURL, resp.Status)
	}
	size := resp.ContentLength
	prefix := "Downloading VM image: " + imageName
	onComplete := prefix + ": done"

	p, bar := utils.ProgressBar(prefix, size, onComplete)

	proxyReader := bar.ProxyReader(resp.Body)
	defer func() {
		if err := proxyReader.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if _, err := io.Copy(out, proxyReader); err != nil {
		return err
	}

	p.Wait()
	return nil
}

func RemoveImageAfterExpire(dir string, expire time.Duration) error {
	now := time.Now()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// Delete any cache files that are older than expiry date
		if !info.IsDir() && (now.Sub(info.ModTime()) > expire) {
			err := os.Remove(path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				logrus.Warnf("unable to clean up cached image: %s", path)
			} else {
				logrus.Debugf("cleaning up cached image: %s", path)
			}
		}
		return nil
	})
	return err
}

// AcquireAlternateImage downloads the alternate image the user provided, which
// can be a file path or URL
func (dl Download) AcquireAlternateImage(inputPath string) (*define.VMFile, error) {
	g, err := NewGenericDownloader(dl.VMKind, dl.VMName, inputPath)
	if err != nil {
		return nil, err
	}

	imagePath, err := define.NewMachineFile(g.Get().LocalUncompressedFile, nil)
	if err != nil {
		return nil, err
	}

	if err := DownloadImage(g); err != nil {
		return nil, err
	}

	return imagePath, nil
}

func isOci(input string) (bool, *ocipull.OCIKind, error) {
	inputURL, err := url2.Parse(input)
	if err != nil {
		return false, nil, err
	}
	switch inputURL.Scheme {
	case ocipull.OCIDir.String():
		return true, &ocipull.OCIDir, nil
	case ocipull.OCIRegistry.String():
		return true, &ocipull.OCIRegistry, nil
	}
	return false, nil, nil
}

func Pull(input, machineName string, vp VirtProvider) (*define.VMFile, FCOSStream, error) {
	var (
		disk ocipull.Disker
	)

	ociBased, ociScheme, err := isOci(input)
	if err != nil {
		return nil, 0, err
	}
	if !ociBased {
		// Business as usual
		dl, err := vp.NewDownload(machineName)
		if err != nil {
			return nil, 0, err
		}
		return dl.AcquireVMImage(input)
	}
	oopts := ocipull.OCIOpts{
		Scheme: ociScheme,
	}
	dataDir, err := GetDataDir(vp.VMType())
	if err != nil {
		return nil, 0, err
	}
	if ociScheme.IsOCIDir() {
		strippedOCIDir := ocipull.StripOCIReference(input)
		oopts.Dir = &strippedOCIDir
		disk = ocipull.NewOCIDir(context.Background(), input, dataDir, machineName)
	} else {
		// a use of a containers image type here might be
		// tighter
		strippedInput := strings.TrimPrefix(input, "docker://")
		// this is the next piece of work
		if len(strippedInput) > 0 {
			return nil, 0, errors.New("image names are not supported yet")
		}
		disk, err = ocipull.NewVersioned(context.Background(), dataDir, machineName, vp.VMType().String())
		if err != nil {
			return nil, 0, err
		}
	}
	if err := disk.Pull(); err != nil {
		return nil, 0, err
	}
	unpacked, err := disk.Unpack()
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		logrus.Debugf("cleaning up %q", unpacked.GetPath())
		if err := unpacked.Delete(); err != nil {
			logrus.Errorf("unable to delete local compressed file %q:%v", unpacked.GetPath(), err)
		}
	}()
	imagePath, err := disk.Decompress(unpacked)
	return imagePath, UnknownStream, err
}
