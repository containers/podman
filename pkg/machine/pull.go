//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/storage/pkg/archive"
	"github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
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

func (d Download) GetLocalUncompressedFile(dataDir string) string {
	compressedFilename := d.VMName + "_" + d.ImageName
	extension := compressionFromFile(compressedFilename)
	uncompressedFile := strings.TrimSuffix(compressedFilename, fmt.Sprintf(".%s", extension.String()))
	d.LocalUncompressedFile = filepath.Join(dataDir, uncompressedFile)
	return d.LocalUncompressedFile
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
	return Decompress(d.Get().LocalPath, d.Get().LocalUncompressedFile)
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

	p := mpb.New(
		mpb.WithWidth(80), // Do not go below 80, see bug #17718
		mpb.WithRefreshRate(180*time.Millisecond),
	)

	bar := p.AddBar(size,
		mpb.BarFillerClearOnComplete(),
		mpb.PrependDecorators(
			decor.OnComplete(decor.Name(prefix), onComplete),
		),
		mpb.AppendDecorators(
			decor.OnComplete(decor.CountersKibiByte("%.1f / %.1f"), ""),
		),
	)

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

func Decompress(localPath, uncompressedPath string) error {
	var isZip bool
	uncompressedFileWriter, err := os.OpenFile(uncompressedPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	sourceFile, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	if strings.HasSuffix(localPath, ".zip") {
		isZip = true
	}
	compressionType := archive.DetectCompression(sourceFile)
	if compressionType != archive.Uncompressed || isZip {
		fmt.Println("Extracting compressed file")
	}
	if compressionType == archive.Xz {
		return decompressXZ(localPath, uncompressedFileWriter)
	}
	if isZip && runtime.GOOS == "windows" {
		return decompressZip(localPath, uncompressedFileWriter)
	}
	return decompressEverythingElse(localPath, uncompressedFileWriter)
}

// Will error out if file without .Xz already exists
// Maybe extracting then renaming is a good idea here..
// depends on Xz: not pre-installed on mac, so it becomes a brew dependency
func decompressXZ(src string, output io.WriteCloser) error {
	var read io.Reader
	var cmd *exec.Cmd
	// Prefer Xz utils for fastest performance, fallback to go xi2 impl
	if _, err := exec.LookPath("xz"); err == nil {
		cmd = exec.Command("xz", "-d", "-c", "-k", src)
		read, err = cmd.StdoutPipe()
		if err != nil {
			return err
		}
		cmd.Stderr = os.Stderr
	} else {
		file, err := os.Open(src)
		if err != nil {
			return err
		}
		defer file.Close()
		// This XZ implementation is reliant on buffering. It is also 3x+ slower than XZ utils.
		// Consider replacing with a faster implementation (e.g. xi2) if podman machine is
		// updated with a larger image for the distribution base.
		buf := bufio.NewReader(file)
		read, err = xz.NewReader(buf)
		if err != nil {
			return err
		}
	}

	done := make(chan bool)
	go func() {
		if _, err := io.Copy(output, read); err != nil {
			logrus.Error(err)
		}
		output.Close()
		done <- true
	}()

	if cmd != nil {
		return cmd.Run()
	}
	<-done
	return nil
}

func decompressEverythingElse(src string, output io.WriteCloser) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	uncompressStream, _, err := compression.AutoDecompress(f)
	if err != nil {
		return err
	}
	defer func() {
		if err := uncompressStream.Close(); err != nil {
			logrus.Error(err)
		}
		if err := output.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	_, err = io.Copy(output, uncompressStream)
	return err
}

func decompressZip(src string, output io.WriteCloser) error {
	zipReader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	if len(zipReader.File) != 1 {
		return errors.New("machine image files should consist of a single compressed file")
	}
	f, err := zipReader.File[0].Open()
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	defer func() {
		if err := output.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	_, err = io.Copy(output, f)
	return err
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
