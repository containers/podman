// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package machine

import (
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
	"github.com/docker/docker/pkg/archive"
	"github.com/sirupsen/logrus"
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
		urlSplit := strings.Split(pullPath, "/")
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

func (g GenericDownload) getLocalUncompressedName() string {
	var (
		extension string
	)
	switch {
	case strings.HasSuffix(g.LocalPath, ".bz2"):
		extension = ".bz2"
	case strings.HasSuffix(g.LocalPath, ".gz"):
		extension = ".gz"
	case strings.HasSuffix(g.LocalPath, ".xz"):
		extension = ".xz"
	}
	uncompressedFilename := filepath.Join(filepath.Dir(g.LocalUncompressedFile), g.VMName+"_"+g.ImageName)
	return strings.TrimSuffix(uncompressedFilename, extension)
}

func (g GenericDownload) DownloadImage() error {
	// If we have a URL for this "downloader", we now pull it
	if g.URL != nil {
		if err := DownloadVMImage(g.URL, g.LocalPath); err != nil {
			return err
		}
	}
	return Decompress(g.LocalPath, g.getLocalUncompressedName())
}

func (g GenericDownload) Get() *Download {
	return &g.Download
}

// DownloadVMImage downloads a VM image from url to given path
// with download status
func DownloadVMImage(downloadURL fmt.Stringer, localImagePath string) error {
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
	urlSplit := strings.Split(downloadURL.String(), "/")
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
func decompressXZ(src string, output io.Writer) error {
	cmd := exec.Command("xzcat", "-k", src)
	//cmd := exec.Command("xz", "-d", "-k", "-v", src)
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	//cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		if _, err := io.Copy(output, stdOut); err != nil {
			logrus.Error(err)
		}
	}()
	return cmd.Run()
}

func decompressEverythingElse(src string, output io.Writer) error {
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
	}()

	_, err = io.Copy(output, uncompressStream)
	return err
}
