// +build amd64 arm64

package machine

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	url2 "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/storage/pkg/archive"
	"github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
	"github.com/vbauerster/mpb/v6"
	"github.com/vbauerster/mpb/v6/decor"
)

// GenericDownload is used when a user provides a URL
// or path for an image
type GenericDownload struct {
	Download
}

// NewGenericDownloader is used when the disk image is provided by the user
func NewGenericDownloader(vmType, vmName, pullPath string) (DistributionDownload, error) {
	var (
		imageName string
	)
	dataDir, err := GetDataDir(vmType)
	if err != nil {
		return nil, err
	}
	dl := Download{}
	// Is pullpath a file or url?
	getURL, err := url2.Parse(pullPath)
	if err != nil {
		return nil, err
	}
	if len(getURL.Scheme) > 0 {
		urlSplit := strings.Split(getURL.Path, "/")
		imageName = urlSplit[len(urlSplit)-1]
		dl.LocalUncompressedFile = filepath.Join(dataDir, imageName)
		dl.URL = getURL
		dl.LocalPath = filepath.Join(dataDir, imageName)
	} else {
		// Dealing with FilePath
		imageName = filepath.Base(pullPath)
		dl.LocalUncompressedFile = filepath.Join(dataDir, imageName)
		dl.LocalPath = pullPath
	}
	dl.VMName = vmName
	dl.ImageName = imageName
	// The download needs to be pulled into the datadir

	gd := GenericDownload{Download: dl}
	gd.LocalUncompressedFile = gd.getLocalUncompressedName()
	return gd, nil
}

func (d Download) getLocalUncompressedName() string {
	var (
		extension string
	)
	switch {
	case strings.HasSuffix(d.LocalPath, ".bz2"):
		extension = ".bz2"
	case strings.HasSuffix(d.LocalPath, ".gz"):
		extension = ".gz"
	case strings.HasSuffix(d.LocalPath, ".xz"):
		extension = ".xz"
	}
	uncompressedFilename := filepath.Join(filepath.Dir(d.LocalPath), d.VMName+"_"+d.ImageName)
	return strings.TrimSuffix(uncompressedFilename, extension)
}

func (g GenericDownload) Get() *Download {
	return &g.Download
}

func (g GenericDownload) HasUsableCache() (bool, error) {
	// If we have a URL for this "downloader", we now pull it
	return g.URL == nil, nil
}

func DownloadImage(d DistributionDownload) error {
	// check if the latest image is already present
	ok, err := d.HasUsableCache()
	if err != nil {
		return err
	}
	if !ok {
		if err := DownloadVMImage(d.Get().URL, d.Get().LocalPath); err != nil {
			return err
		}
	}
	return Decompress(d.Get().LocalPath, d.Get().getLocalUncompressedName())
}

// DownloadVMImage downloads a VM image from url to given path
// with download status
func DownloadVMImage(downloadURL *url2.URL, localImagePath string) error {
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
		return fmt.Errorf("error downloading VM image %s: %s", downloadURL, resp.Status)
	}
	size := resp.ContentLength
	urlSplit := strings.Split(downloadURL.Path, "/")
	prefix := "Downloading VM image: " + urlSplit[len(urlSplit)-1]
	onComplete := prefix + ": done"

	p := mpb.New(
		mpb.WithWidth(60),
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
	uncompressedFileWriter, err := os.OpenFile(uncompressedPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	sourceFile, err := ioutil.ReadFile(localPath)
	if err != nil {
		return err
	}

	compressionType := archive.DetectCompression(sourceFile)
	if compressionType != archive.Uncompressed {
		fmt.Println("Extracting compressed file")
	}
	if compressionType == archive.Xz {
		return decompressXZ(localPath, uncompressedFileWriter)
	}
	return decompressEverythingElse(localPath, uncompressedFileWriter)
}

// Will error out if file without .xz already exists
// Maybe extracting then renameing is a good idea here..
// depends on xz: not pre-installed on mac, so it becomes a brew dependency
func decompressXZ(src string, output io.WriteCloser) error {
	var read io.Reader
	var cmd *exec.Cmd
	// Prefer xz utils for fastest performance, fallback to go xi2 impl
	if _, err := exec.LookPath("xzcat"); err == nil {
		cmd = exec.Command("xzcat", "-k", src)
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
